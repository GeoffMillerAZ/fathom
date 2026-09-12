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
	"sort"

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
		Description: "Concern-scoped symbol search. Returns ranked SymbolNodes matching the given concern and optional keywords. Reach for it when you have a topic rather than a name: the concern scopes the index to what is relevant and returns the ranked slice (ADR-006 — the concern is this engine's parameter, not every tool's). Required: service, concern.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{
					"type":        "string",
					"description": "Service / repo identifier (e.g. \"payment-service\")",
				},
				"concern": map[string]any{
					"type":        "string",
					"description": "Concern ID to scope the query (e.g. \"authentication\"). Alias: \"symbol\" (additive — same field, either name works; both may be given only if they agree).",
				},
				"symbol": map[string]any{
					"type":        "string",
					"description": "Alias for \"concern\" (P1-1: the hot-tool quartet's primary argument is named differently per tool; this lets a caller reuse the same field name across all four).",
				},
				"keywords": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Optional keyword hints for relevance scoring",
				},
				"top": map[string]any{
					"type":        "integer",
					"description": "Maximum number of results to return (default 20)",
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
			topN = 20
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
		return symbolQueryResult{
			Nodes:     nodes,
			Staleness: result.Staleness,
			Truncated: el != nil || len(result.Nodes) >= topN,
			Elided:    el,
		}, nil
	})

	// find_callers — k-hop callers of a symbol.
	s.registerTool(toolDescriptor{
		Name:        "find_callers",
		Description: "Find every symbol that calls the named symbol in one service. Returns only RESOLVED callers by default (edges the Go typed pass confirmed); include_heuristic: true adds the syntax-only ones back. Rows carry depth (direct = 1) and edge_confidence; edge_filter names which evidence answered, and a corpus with no typed edges falls back to \"all\". depth > 1 adds transitive_callers; by_depth summarises. Reach for it when you need who-calls-this before changing a signature. Required: service, symbol.",
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
					"description": "Reverse-graph depth to walk (default 1 = direct callers only). Values above 1 add transitive_callers (depth 2..N) and extend by_depth; 0 = unbounded.",
				},
				"include_heuristic": map[string]any{
					"type":        "boolean",
					"description": "Add syntax-only (heuristic) callers, which come from bare-name matching. Default false.",
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
		// FindCallers proves on its own.
		navs := s.loadNavGraphs(ctx, backend, p.Service)
		var g *callgraph.Graph
		if navs != nil {
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
					for _, n := range navs.Nodes {
						if keep[n.Key()] {
							kept = append(kept, n)
						}
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
		result := map[string]any{"callers": callers, "edge_filter": edgeFilter}
		if maxDepth != 1 {
			transitive := map[string]depthInfo{}
			for k, d := range depths {
				if d.Depth > 1 || (d.Depth == 1 && !listed[k]) {
					transitive[k] = d
					infos = append(infos, d)
				}
			}
			result["transitive_callers"] = sortedDepthRows(transitive)
		}
		result["by_depth"] = withDepthBuckets(infos)

		// Typed Go call edges (ADR-021 §3): kind-labeled callers, including
		// any reachable ONLY via typed dispatch — never visible in the
		// syntax-only "callers" list above (an ambiguous method name across
		// receivers, graph.go:218-240, has no Calls entry at all). Additive
		// and best-effort: a non-Go service, or one with no typed edges yet,
		// simply omits this field.
		if typedCallers := typedCallersFor(g, p.Symbol); len(typedCallers) > 0 {
			result["typed_callers"] = typedCallers
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
		if cross := crossServiceCallers(fleet, p.Service, p.Symbol); len(cross) > 0 {
			result["cross_service_callers"] = cross
		}
		return result, nil
	})

	// find_symbol — locate a symbol definition.
	s.registerTool(toolDescriptor{
		Name:        "find_symbol",
		Description: "Locate symbol definitions by name, with tiered matching: exact/key-suffix first, then case-insensitive, then substring; a full miss returns did_you_mean suggestions (edit-distance typo candidates, else concern-ranked) when any exist. matched_via reports which tier answered — treat the result as proof the named symbol exists only when it is exact or case_insensitive. Reach for it when you know (roughly) a name and need its definition's file and line before reading or editing — the first move of almost every task. Required: service, name.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{
					"type":        "string",
					"description": "Service / repo identifier",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Symbol name (bare or fully-qualified; exact spelling not required). Alias: \"symbol\" (additive — same field, either name works; both may be given only if they agree).",
				},
				"symbol": map[string]any{
					"type":        "string",
					"description": "Alias for \"name\" (P1-1: the hot-tool quartet's primary argument is named differently per tool; this lets a caller reuse the same field name across all four).",
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
			return map[string]any{"symbols": toNodeDTOs(nodes), "matched_via": "exact"}, nil
		}

		// Exact tier missed — degrade over the service's symbol list. Fallback
		// errors never fail the lookup; the answer is just "no match".
		var all []core.SymbolNode
		if listed, listErr := backend.ListAllSymbols(ctx, p.Service); listErr == nil {
			all = listed
			if ci := caseFoldMatches(all, p.Name); len(ci) > 0 {
				return map[string]any{"symbols": toNodeDTOs(ci), "matched_via": "case_insensitive"}, nil
			}
			if sub, total := substringMatches(all, p.Name); len(sub) > 0 {
				resp := map[string]any{"symbols": toNodeDTOs(sub), "matched_via": "substring"}
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
			resp["did_you_mean"] = suggestionDTOs(fuzzy)
			return resp, nil
		}
		ranked, qErr := backend.QuerySymbols(ctx, core.Query{
			ServiceID: p.Service,
			ConcernID: p.Name,
			Keywords:  nameTokens(p.Name),
			TopN:      maxSuggestions,
		})
		if qErr == nil && len(ranked.Nodes) > 0 {
			resp["did_you_mean"] = suggestionDTOs(ranked.Nodes)
		}
		return resp, nil
	})

	// Register analysis tools (call-graph and import-cycle analysis).
	registerAnalysisTools(s, backend)

	// Register navigation and structural analysis tools.
	registerNavTools(s, backend, fleet)

	// Register source-scanning analyzer tools.
	registerSourceTools(s, backend)

	// Register the bounded raw-text read tool (T5-4).
	registerReadSource(s, backend)

	// Register meta-tools, prompts, and resources.
	registerMeta(s, backend)

	// Register extra analyzer tools (impact, spec, arch, coding, etc.).
	registerExtraTools(s, backend)

	// Register semantic-layer tools (obligations, compile_layer).
	registerSemanticTools(s, backend, modules)
}

// ─── DTOs ────────────────────────────────────────────────────────────────────

// symbolNodeDTO is the JSON-serialisable representation of a SymbolNode
// returned to MCP clients.
type symbolNodeDTO struct {
	Service  string `json:"service"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	FilePath string `json:"file_path"`
	Line     int    `json:"line,omitempty"`
	// EndLine mirrors core.SymbolNode.EndLine (T5-5): the last line covered
	// by this symbol, when the extractor computed one (currently DocChunk
	// headings only) — 0/omitted otherwise.
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
	nodes, err := backend.ListAllSymbols(ctx, service)
	if err != nil || len(nodes) == 0 {
		return nil
	}
	g := callgraph.Build(nodes)
	if typed, tErr := backend.TypedEdges(ctx, service); tErr == nil && len(typed) > 0 {
		callgraph.AddTypedEdges(g, typed)
	}
	return g
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
