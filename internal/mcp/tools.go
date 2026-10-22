// Package mcp — MCP tool registrations backed by the QueryInterface.
//
// registerTools wires query_symbols, find_callers, find_symbol, and
// set_profile into the server's tool registry. The set_profile tool is
// always registered (even when backend == nil) so clients can switch
// profiles before any query.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// registerTools registers all MCP tools into s. backend may be nil, in which
// case query tools return ErrCodeInternal when called. fleet may also be
// nil (non-fleet stdio serve-mcp); when set, find_callers and
// symbol_context (registerNavTools) additionally consult it for
// cross-service call edges (P1-2). modules may be nil (no --module store
// mounted); when set, the obligations tool (registerSemanticTools) additionally
// unions in rules from every platform module a service's own
// .fathom/modules.cue binds (docs/design/platform-modules.md §3.3).
func registerTools(s *Server, backend QueryInterface, fleet FleetBackend, modules moduleBackend) {
	// query_symbols — concern-scoped symbol search.
	s.registerTool(toolDescriptor{
		Name:        "query_symbols",
		Description: "Concern-scoped symbol search: ranked symbols for a concern plus keywords. Reach for it when you have a topic, not a name. Required: service, concern.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{
					"type":        "string",
					"description": "Service / repo identifier",
				},
				"concern": map[string]any{
					"type":        "string",
					"description": "Concern ID to scope the query. Alias: \"symbol\".",
				},
				"symbol": map[string]any{
					"type":        "string",
					"description": "Alias for \"concern\" — same field.",
				},
				"keywords": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Optional keyword hints for relevance scoring",
				},
				"top": map[string]any{
					"type":        "integer",
					"description": "Maximum results to return (default 20)",
				},
			},
			"required": []string{"service", "concern"},
		},
		group:   GroupCoding,
		aliases: map[string]string{"symbol": "concern"},
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service  string   `json:"service"`
			Concern  string   `json:"concern"`
			Keywords []string `json:"keywords"`
			Top      int      `json:"top"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "query_symbols: invalid args"}
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(args, &raw); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "query_symbols: invalid args"}
		}
		concern, aliasErr := resolveAliasedString(raw, "query_symbols", "concern", "symbol")
		if aliasErr != nil {
			return nil, aliasErr
		}
		p.Concern = concern
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "query_symbols: service required"}
		}
		if p.Concern == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "query_symbols: concern required"}
		}
		topN := p.Top
		if topN <= 0 {
			// SR-43 (F-24, design §5): cut from 20 — query_symbols is a
			// ranked shortlist, not a listing; a caller after more
			// re-calls with an explicit "top".
			topN = querySymbolsDefaultTopN
		}
		result, err := backend.QuerySymbols(ctx, core.Query{
			ServiceID: p.Service,
			ConcernID: p.Concern,
			Keywords:  p.Keywords,
			TopN:      topN,
		})
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		nodes := toNodeDTOs(result.Nodes)
		nodes, el := applyBudget(nodes, budgetDefaults("query_symbols", s.budgets),
			"relevance score, descending",
			"re-call with a larger budget, or narrow concern/keywords")
		truncated := el != nil || len(result.Nodes) >= topN
		return withSummary(querySymbolsAnswerLine(nodes, p.Concern, truncated), symbolQueryResult{
			Nodes:     nodes,
			Staleness: result.Staleness,
			Truncated: truncated,
			Elided:    el,
		}), nil
	})

	// find_callers — k-hop callers of a symbol.
	s.registerTool(toolDescriptor{
		Name:        "find_callers",
		Description: "Finds callers of symbol; resolved edges by default. This result IS the answer for who calls it; a follow-up call only widens depth. Reach for it when about to change a signature. Required: service, symbol.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{
					"type":        "string",
					"description": "Service / repo identifier",
				},
				"symbol": map[string]any{
					"type":        "string",
					"description": "Symbol name (bare or fully-qualified)",
				},
				"depth": map[string]any{
					"type":        "integer",
					"description": "Reverse-graph depth to walk (default 1); 0 = unbounded",
				},
				"include_heuristic": map[string]any{
					"type":        "boolean",
					"description": "Add syntax-only callers. Default false.",
				},
				"with_signatures": map[string]any{
					"type":        "boolean",
					"description": "Include each caller's full signature. Default false.",
				},
			},
			"required": []string{"service", "symbol"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service          string `json:"service"`
			Symbol           string `json:"symbol"`
			Depth            *int   `json:"depth"`
			IncludeHeuristic bool   `json:"include_heuristic"`
			WithSignatures   bool   `json:"with_signatures"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "find_callers: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "find_callers: service required"}
		}
		if p.Symbol == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "find_callers: symbol required"}
		}
		maxDepth := 1
		if p.Depth != nil && *p.Depth >= 0 {
			maxDepth = *p.Depth
		}
		nodes, err := backend.FindCallers(ctx, p.Service, p.Symbol)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}

		// BD-2 (docs/plans/blast-depth.md): depth + edge_confidence per row
		// and a by_depth summary, read from the reverse call graph (syntax
		// edges plus the typed pass's edges). The graph is best-effort — a
		// backend that cannot list symbols leaves every direct caller at
		// depth 1 / heuristic, which is exactly what the syntax engine's
		// FindCallers proves on its own — and the summary names the failed
		// load, so the shorter answer never reads as the whole one (EW-78).
		navs, navErr := loadNavGraphsErr(ctx, backend, p.Service)
		var g *callgraph.Graph
		if navErr == nil {
			g = navs.Both
		}
		var depths map[string]depthInfo
		if g != nil {
			depths = callerDepths(g, p.Symbol, maxDepth)
		}

		// BP-26: resolved by default. The resolved graph both DROPS the
		// syntax pass's name-collision callers and ADDS the ones reachable
		// only through typed interface dispatch, which never had a Calls
		// entry to be found by — so the default answer is a different set,
		// not a subset.
		edgeFilter := "all"
		if !p.IncludeHeuristic {
			if keep, ok := resolvedCallersOnly(navs, p.Symbol); ok {
				edgeFilter = "resolved"
				kept := nodes[:0:0]
				for _, n := range nodes {
					if keep[n.Key()] {
						kept = append(kept, n)
						delete(keep, n.Key())
					}
				}
				// A resolved caller the syntax engine never listed still
				// belongs in the answer; find it in the index by key.
				if len(keep) > 0 && navs != nil {
					fromFiles := typedCallerFiles(navs, p.Symbol, keep)
					for _, n := range navs.Nodes {
						if !keep[n.Key()] {
							continue
						}
						// Every init() of a package shares one key (O1124):
						// the call is the init in the file its typed edge
						// was made in, not each of its siblings.
						if files := fromFiles[n.Key()]; n.Name == "init" && len(files) > 0 && !files[n.FilePath] {
							continue
						}
						kept = append(kept, n)
					}
				}
				sort.Slice(kept, func(i, j int) bool {
					if kept[i].Name != kept[j].Name {
						return kept[i].Name < kept[j].Name
					}
					return kept[i].Key() < kept[j].Key()
				})
				nodes = kept
				if depths != nil {
					for k := range depths {
						if d := depths[k]; d.Depth == 1 && d.Confidence != core.EdgeConfidenceResolved {
							delete(depths, k)
						}
					}
				}
			}
		}
		// RC-20: a fan-in this large would never fit even a paged answer
		// (fitToolBudget below already handles the ordinary "busy
		// function" case) — a generic name's callers spread across a
		// whole fleet. Return the shape of the answer instead of rows:
		// the count and the packages driving it.
		byPkg := make(map[string]int, len(nodes))
		for _, n := range nodes {
			byPkg[n.Package]++
		}
		if need, sum := scopeRequired(len(nodes), scopeRequiredBudget, byPkg); need {
			msg := fmt.Sprintf(
				"%d callers exceed the budget — too many to page through; scope with a more specific symbol, or inspect top_packages below.",
				sum.Count,
			)
			return withSummary(capSummary(fmt.Sprintf("%d callers of %s exceed the budget — too many to list; scope required", sum.Count, p.Symbol)), map[string]any{
				"callers":        []any{},
				"edge_filter":    edgeFilter,
				"by_depth":       []any{},
				"resolution":     resolutionMix{},
				"scope_required": true,
				"count":          sum.Count,
				"top_packages":   sum.TopPackages,
				"message":        msg,
			}), nil
		}

		callers := toNodeDTOs(nodes)
		infos := make([]depthInfo, 0, len(callers))
		listed := make(map[string]bool, len(callers))
		for i, n := range nodes {
			info := depthInfo{Depth: 1, Confidence: core.EdgeConfidenceHeuristic}
			if edgeFilter == "resolved" {
				info.Confidence = core.EdgeConfidenceResolved
			}
			if d, ok := depths[n.Key()]; ok && d.Depth == 1 {
				info = d
			}
			callers[i].Depth = info.Depth
			callers[i].EdgeConfidence = string(info.Confidence)
			infos = append(infos, info)
			listed[n.Key()] = true
		}

		// SB-47: render each caller compact for the wire — "file:line
		// name" plus a one-character confidence code, never a signature
		// unless with_signatures asked for it — and lift service/tier/
		// depth/edge_confidence to the envelope whenever every listed
		// caller shares one (mirroring references' SB-1 occurrence_*
		// lift). The direct "callers" list is depth-1 and single-service
		// by construction, so these are almost always uniform.
		wireCallers := make([]callerEntry, len(callers))
		for i, n := range callers {
			wireCallers[i] = renderCallerEntry(n, p.WithSignatures)
		}
		env := uniformCallerFields(callers)

		// RC-19: keep whole rows up to budget, then a one-line elision
		// naming the refinement — the direct "callers" list had no
		// budget line at all before this row.
		var elidedCount int
		wireCallers, elidedCount = fitToolBudget(wireCallers, defaultCharBudget)
		// infos and wireCallers were built 1:1 from the SAME "callers"
		// order above, and fitToBudget's own contract is a deterministic
		// PREFIX — never a reordering or arbitrary subset — so trimming
		// infos to the survivors' length keeps by_depth/resolution
		// describing exactly the depth-1 rows this response returns,
		// never a wider set the budget already cut (the same discipline
		// blastBucketsOf's `include` filter and references_tool.go's own
		// post-elision infos already apply). transitive_callers (added
		// below) is never itself budgeted, so its own infos entries
		// already match what it returns.
		infos = infos[:len(wireCallers)]
		result := map[string]any{"callers": wireCallers, "edge_filter": edgeFilter}
		if line := elisionLine(elidedCount, "a package-qualified symbol (pkg.Name)"); line != "" {
			result["elision"] = line
		}
		if env.ServiceUniform {
			result["caller_service"] = env.Service
		}
		if env.TierUniform {
			result["caller_tier"] = env.Tier
		}
		if env.DepthUniform {
			result["caller_depth"] = env.Depth
		}
		if env.ConfUniform {
			result["caller_edge_confidence"] = env.EdgeConfidence
		}
		var transitiveRows []depthRow
		if maxDepth != 1 {
			transitive := map[string]depthInfo{}
			for k, d := range depths {
				if d.Depth > 1 || (d.Depth == 1 && !listed[k]) {
					transitive[k] = d
					infos = append(infos, d)
				}
			}
			transitiveRows = sortedDepthRows(transitive)
			result["transitive_callers"] = transitiveRows
		}
		byDepth := withDepthBuckets(infos)
		result["by_depth"] = byDepth

		// Typed Go call edges (ADR-021 §3): kind-labeled callers, including
		// any reachable ONLY via typed dispatch — never visible in the
		// syntax-only "callers" list above (an ambiguous method name across
		// receivers, graph.go:218-240, has no Calls entry at all). Additive
		// and best-effort: a non-Go service, or one with no typed edges yet,
		// simply omits this field.
		typedCallers := typedCallersFor(g, p.Symbol)
		if len(typedCallers) > 0 {
			result["typed_callers"] = typedCallers
		}

		// resolution/index_commit (VB-19, docs/vision/29-verbs-not-lookups.md
		// §3 wave E): the typed/heuristic mix is the same tally by_depth
		// above already carries (infos) — every fact returned in
		// "callers"/"transitive_callers" — plus the daemon's watermark for
		// the service, when the backend can name one.
		result["resolution"] = resolutionFromDepthBuckets(byDepth)
		if ic := indexCommitFor(ctx, backend, p.Service); ic != "" {
			result["index_commit"] = ic
		}

		// Cross-service callers (P1-2, closing O12(a)): the QueryInterface
		// backend above only ever sees p.Service's own symbols, so a caller
		// defined in a DIFFERENT fleet service — e.g. fathom::main.
		// cmdInstrument calling agentic-kit's CheckLiveness — is invisible
		// to it. When a fleet backend is wired (fleet stores; see
		// NewServerWithFleet), crossServiceCallers reuses cross_repo_
		// callers' own fleet-wide matching (platform_tools.go's
		// fleetCallers/matchesSymbol) filtered to services other than
		// p.Service, and every hit is placed in its OWN field — "callers"
		// stays exactly the same-service shape it always was — so the two
		// scopes are never silently merged; each cross-service entry also
		// carries an explicit "scope":"cross-service" tag. nil fleet or no
		// cross-service hits omit the field entirely.
		if cross := crossServiceCallers(ctx, fleet, p.Service, p.Symbol); len(cross) > 0 {
			result["cross_service_callers"] = cross
		}
		summary := findCallersSummary(p.Symbol, callers[:len(wireCallers)], transitiveRows, elidedCount, maxDepth, len(typedCallers))
		if navErr != nil {
			summary += "; " + navLoadNote(p.Service, navErr) + " (direct syntax callers only)"
		}
		return withSummary(summary, result), nil
	})

	// find_symbol — locate a symbol definition.
	s.registerTool(toolDescriptor{
		Name:        "find_symbol",
		Description: "Locates symbol definitions: exact, then case-insensitive, then substring. This result IS the answer once exact; a follow-up call only adds neighbourhood/call sites. Reach for it when you need a file/line. Required: service, name.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{
					"type":        "string",
					"description": "Service / repo identifier",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Symbol name (exact spelling not required). Alias: \"symbol\".",
				},
				"symbol": map[string]any{
					"type":        "string",
					"description": "Alias for \"name\" — same field.",
				},
			},
			"required": []string{"service", "name"},
		},
		group:   GroupShared,
		aliases: map[string]string{"symbol": "name"},
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
			Name    string `json:"name"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "find_symbol: invalid args"}
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(args, &raw); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "find_symbol: invalid args"}
		}
		name, aliasErr := resolveAliasedString(raw, "find_symbol", "name", "symbol")
		if aliasErr != nil {
			return nil, aliasErr
		}
		p.Name = name
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "find_symbol: service required"}
		}
		if p.Name == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "find_symbol: name required"}
		}
		nodes, err := backend.FindSymbol(ctx, p.Service, p.Name)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		if len(nodes) > 0 {
			resp := map[string]any{"symbols": markSymbolDTOs(ctx, backend, p.Service, toNodeDTOs(nodes)), "matched_via": "exact"}
			// SB-5: an exact hit on a single Go interface additionally
			// carries its implementers (H1-T06's question in one call).
			if len(nodes) == 1 && nodes[0].Kind == core.KindGoInterface {
				if all, allErr := serviceSymbols(ctx, backend, p.Service); allErr == nil {
					if impls := implementationsOf(ctx, backend, p.Service, nodes[0], all); len(impls) > 0 {
						resp["implementations"] = impls
					}
				}
			}
			return resp, nil
		}

		// Exact tier missed — degrade over the service's symbol list. Fallback
		// errors never fail the lookup; the answer is just "no match".
		var all []core.SymbolNode
		if listed, listErr := serviceSymbols(ctx, backend, p.Service); listErr == nil {
			all = listed
			if ci, ciTotal := caseFoldMatches(all, p.Name); len(ci) > 0 {
				resp := map[string]any{"symbols": markSymbolDTOs(ctx, backend, p.Service, toNodeDTOs(ci)), "matched_via": "case_insensitive"}
				if el := elisionFor(len(ci), ciTotal, "name match before package match, then name/file/line/kind", "narrow with the exact name, or a package-qualified query (pkg.Name)"); el != nil {
					resp["truncated"] = true
					resp["elided"] = el
				}
				return resp, nil
			}
			if sub, total := substringMatches(all, p.Name); len(sub) > 0 {
				resp := map[string]any{"symbols": markSymbolDTOs(ctx, backend, p.Service, toNodeDTOs(sub)), "matched_via": "substring"}
				if el := elisionFor(len(sub), total, "shortest name first, then name/file/line/kind", "narrow the query, or ask for the exact/qualified name"); el != nil {
					resp["truncated"] = true
					resp["elided"] = el
				}
				return resp, nil
			}
		}

		// Full miss: did_you_mean suggestions, never matches. Edit-distance
		// over the symbol list catches typos ("authentiction"); the
		// concern-ranked query is the last resort for conceptual near-misses.
		resp := map[string]any{"symbols": []any{}, "matched_via": "none"}
		if fuzzy := fuzzySuggestions(all, p.Name); len(fuzzy) > 0 {
			resp["did_you_mean"] = suggestionDTOs(ctx, backend, p.Service, fuzzy)
			return resp, nil
		}
		ranked, qErr := backend.QuerySymbols(ctx, core.Query{
			ServiceID: p.Service,
			ConcernID: p.Name,
			Keywords:  nameTokens(p.Name),
			TopN:      maxSuggestions,
		})
		if qErr == nil && len(ranked.Nodes) > 0 {
			resp["did_you_mean"] = suggestionDTOs(ctx, backend, p.Service, ranked.Nodes)
		}
		return resp, nil
	})

	// Register analysis tools (call-graph and import-cycle analysis).
	// safe_delete_check and rename_impact (ADR-035 decision 1, plan VB-2)
	// were unregistered as standalone tools by SR-24 (F-13, design §4);
	// their answers are reached through blast_radius's kind argument now
	// (blastRadiusByKind, extra_tools.go).
	registerAnalysisTools(s, backend)

	// Register localize — the bounded, budgeted, logged localization walk
	// (ADR-035 decision 1 follow-on, vision 29 Wave G, plan VB-26).
	registerLocalize(s, backend)

	// Register navigation and structural analysis tools.
	registerNavTools(s, backend, fleet)

	// registerSourceTools (secret_scan, shell_out, http_client_calls, cloc)
	// was retired by SR-12/SR-19 (surface reckoning): every tool it wired
	// is unregistered, 0 lifetime calls each; fetchSources stays as a
	// plain helper (meta.go still calls it).

	// Register the bounded raw-text read tool (T5-4).
	registerReadSource(s, backend)

	// Register meta-tools, prompts, and resources.
	registerMeta(s, backend)

	// Register extra analyzer tools (impact, spec, arch, coding, etc.).
	registerExtraTools(s, backend)

	// Register semantic-layer tools (obligations, compile_layer).
	registerSemanticTools(s, backend, modules)

	// registerDiscoveries (RC-27, docs/vision/28-receipts.md §4) was
	// unregistered by SR-29 (surface reckoning F-47): 0 lifetime calls; the
	// SubagentStart hook's push-form render (subagentDiscoveries,
	// cmd/fathom/discoveries.go) already covers the need.
}

// ─── DTOs ────────────────────────────────────────────────────────────────────

// symbolNodeDTO is the JSON-serialisable representation of a SymbolNode
// returned to MCP clients.
type symbolNodeDTO struct {
	CitationStatus *core.CitationStatus `json:"citation_status,omitempty"`
	Service        string               `json:"service"`
	Name           string               `json:"name"`
	Kind           string               `json:"kind"`
	FilePath       string               `json:"file_path"`
	Line           int                  `json:"line,omitempty"`
	// EndLine mirrors core.SymbolNode.EndLine (T5-5): the last line covered
	// by this symbol, when the extractor computed one — DocChunk headings,
	// Go func/method/type/const/var, and most tree-sitter/AST-backed
	// extractors; 0/omitted for the extractors that don't yet (see
	// core.SymbolNode.EndLine's doc comment).
	EndLine   int    `json:"end_line,omitempty"`
	Signature string `json:"signature,omitempty"`
	Tier      string `json:"tier,omitempty"`
	// Depth and EdgeConfidence (BD-2, docs/plans/blast-depth.md) are set
	// only on find_callers rows: the caller's reverse-graph distance from
	// the queried symbol (direct caller = 1) and whether the edge that
	// reached it was confirmed by the typed pass ("resolved") or is
	// syntax-only ("heuristic"). Omitted on every other tool's node rows.
	Depth          int    `json:"depth,omitempty"`
	EdgeConfidence string `json:"edge_confidence,omitempty"`
}

// symbolQueryResult is the top-level payload for query_symbols.
type symbolQueryResult struct {
	Nodes     []symbolNodeDTO      `json:"nodes"`
	Staleness core.StalenessReport `json:"staleness"`
	Truncated bool                 `json:"truncated"`
	// Elided (BA-1) discloses what applyBudget cut, when it cut anything —
	// nil (omitted) for a complete answer.
	Elided *elided `json:"elided,omitempty"`
}

// querySymbolsDefaultTopN (SR-43, F-24, design §5) is query_symbols'
// default result count when the caller omits "top" — cut from 20: a
// ranked shortlist earns its place by being short, and a caller after
// more re-calls with an explicit "top".
const querySymbolsDefaultTopN = 10

// querySymbolsAnswerLine renders query_symbols' answer-first sentence
// (wave L1, SR-43): the count for concern, and the top hit's name, kind
// and location — built from nodes, the exact (post-budget) rows the
// "nodes" field returns, so the summary can never name a row the body
// does not carry. truncated notes when more exist than the response kept.
func querySymbolsAnswerLine(nodes []symbolNodeDTO, concern string, truncated bool) string {
	if len(nodes) == 0 {
		return fmt.Sprintf("no symbols for concern %q", concern)
	}
	first := nodes[0]
	summary := fmt.Sprintf("%d symbol%s for concern %q; top: %s (%s) %s:%d",
		len(nodes), pluralS(len(nodes)), concern, first.Name, first.Kind, first.FilePath, first.Line)
	if truncated {
		summary += " (more available)"
	}
	return capSummary(summary)
}

func toNodeDTOs(nodes []core.SymbolNode) []symbolNodeDTO {
	dtos := make([]symbolNodeDTO, len(nodes))
	for i, n := range nodes {
		dtos[i] = symbolNodeDTO{
			Service:   n.Service,
			Name:      n.Name,
			Kind:      string(n.Kind),
			FilePath:  n.FilePath,
			Line:      n.Line,
			EndLine:   n.EndLine,
			Signature: n.Signature,
			Tier:      string(n.Tier),
		}
	}
	return dtos
}

// callerGraph builds the service's reverse-walkable call graph — syntax
// edges from every indexed symbol plus the typed pass's edges (ADR-021 §3)
// — for find_callers' depth/confidence annotation and typed_callers.
// Best-effort: a backend that cannot list symbols (or lists none) yields
// nil, and the caller degrades to the syntax engine's own direct callers.
func callerGraph(ctx context.Context, backend QueryInterface, service string) *callgraph.Graph {
	ng, err := loadNavGraphsErr(ctx, backend, service)
	if err != nil || len(ng.Nodes) == 0 {
		return nil
	}
	return ng.Both // RT-74: the graph store's, shared and read-only
}

// findCallersSummary renders find_callers' answer-first sentence (wave L1,
// H1-T05): the direct (depth-1) callers by name and location, built from
// direct — the exact prefix of symbolNodeDTOs the "callers" field renders
// on the wire, post-budget — never from transitive (depth 2+), so the
// summary can never name a deeper caller as direct (the exact defect
// H1-T05 found: find_callers listed the direct caller correctly and the
// model's final answer named a depth-2 caller instead). elidedDirect is
// how many further direct callers the budget cut; transitive is the
// (already depth/key-sorted) rows beyond depth 1, present only when the
// caller asked for depth > 1.
func findCallersSummary(symbol string, direct []symbolNodeDTO, transitive []depthRow, elidedDirect, maxDepth, typedCallers int) string {
	var summary string
	if len(direct) == 0 && elidedDirect == 0 {
		if typedCallers > 0 {
			// callers is the syntax read; typed_callers lists the typed
			// (file-resolved included) edges beside it (O742).
			summary = fmt.Sprintf("%d typed caller%s of %s (see typed_callers); no syntax-matched callers", typedCallers, pluralS(typedCallers), symbol)
		} else {
			summary = fmt.Sprintf("no direct callers of %s found", symbol)
		}
	} else {
		n := len(direct)
		if n > 3 {
			n = 3
		}
		names := make([]string, n)
		for i, c := range direct[:n] {
			names[i] = fmt.Sprintf("%s (%s:%d)", c.Name, c.FilePath, c.Line)
		}
		summary = fmt.Sprintf("%d direct caller%s of %s: %s", len(direct)+elidedDirect, pluralS(len(direct)+elidedDirect), symbol, strings.Join(names, ", "))
		if len(direct) > n {
			summary += fmt.Sprintf(" (+%d more)", len(direct)-n)
		}
		if elidedDirect > 0 {
			summary += fmt.Sprintf(" (+%d not shown)", elidedDirect)
		}
	}
	if len(transitive) > 0 {
		depthLabel := "depth 2"
		switch {
		case maxDepth == 0:
			depthLabel = "deeper depths"
		case maxDepth > 2:
			depthLabel = fmt.Sprintf("depths 2-%d", maxDepth)
		}
		summary += fmt.Sprintf("; %d more at %s", len(transitive), depthLabel)
	}
	return capSummary(summary)
}

// typedCallersFor returns symbol's typed-edge callers (ADR-021 §3), each
// kind-labeled ("direct" or "iface-dispatch"), as JSON-ready maps —
// syntax-only edges (Kind == "") are excluded, since those are already
// covered by the caller's own syntax-based result. Best-effort: a nil graph
// (unsupported backend, no symbols) yields nil, not a tool-level error.
func typedCallersFor(g *callgraph.Graph, symbol string) []any {
	if g == nil {
		return nil
	}
	details := callgraph.CallersDetail(g, symbol)
	var out []any
	for _, d := range details {
		if d.Kind == "" {
			continue
		}
		out = append(out, map[string]any{"key": d.Key, "kind": string(d.Kind)})
	}
	return out
}
