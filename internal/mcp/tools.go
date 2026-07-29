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
		Description: "Concern-scoped symbol search. Returns ranked SymbolNodes matching the given concern and optional keywords.",
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
		nodes, budgetTrunc := fitToBudget(nodes, 0)
		return symbolQueryResult{
			Nodes:     nodes,
			Staleness: result.Staleness,
			Truncated: budgetTrunc || len(result.Nodes) >= topN,
		}, nil
	})

	// find_callers — k-hop callers of a symbol.
	s.registerTool(toolDescriptor{
		Name:        "find_callers",
		Description: "Find all symbols that call the named symbol in the given service.",
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
			},
			"required": []string{"service", "symbol"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
			Symbol  string `json:"symbol"`
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
		nodes, err := backend.FindCallers(ctx, p.Service, p.Symbol)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		result := map[string]any{"callers": toNodeDTOs(nodes)}

		// Typed Go call edges (ADR-021 §3): kind-labeled callers, including
		// any reachable ONLY via typed dispatch — never visible in the
		// syntax-only "callers" list above (an ambiguous method name across
		// receivers, graph.go:218-240, has no Calls entry at all). Additive
		// and best-effort: a non-Go service, or one with no typed edges yet,
		// simply omits this field.
		if typedCallers := typedCallersFor(ctx, backend, p.Service, p.Symbol); len(typedCallers) > 0 {
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
		Description: "Locate symbol definitions by name, with tiered matching: exact/key-suffix first, then case-insensitive, then substring; a full miss returns did_you_mean suggestions (edit-distance typo candidates, else concern-ranked) when any exist. matched_via reports which tier answered — treat the result as proof the named symbol exists only when it is exact or case_insensitive.",
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
			if sub, truncated := substringMatches(all, p.Name); len(sub) > 0 {
				resp := map[string]any{"symbols": toNodeDTOs(sub), "matched_via": "substring"}
				if truncated {
					resp["truncated"] = true
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
}

// symbolQueryResult is the top-level payload for query_symbols.
type symbolQueryResult struct {
	Nodes     []symbolNodeDTO      `json:"nodes"`
	Staleness core.StalenessReport `json:"staleness"`
	Truncated bool                 `json:"truncated"`
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

// typedCallersFor returns symbol's typed-edge callers (ADR-021 §3), each
// kind-labeled ("direct" or "iface-dispatch"), as JSON-ready maps —
// syntax-only edges (Kind == "") are excluded, since those are already
// covered by the caller's own syntax-based result. Best-effort: any error
// (unsupported backend, no typed edges yet) yields an empty/nil result, not
// a tool-level error.
func typedCallersFor(ctx context.Context, backend QueryInterface, service, symbol string) []any {
	typed, err := backend.TypedEdges(ctx, service)
	if err != nil || len(typed) == 0 {
		return nil
	}
	nodes, err := backend.ListAllSymbols(ctx, service)
	if err != nil || len(nodes) == 0 {
		return nil
	}
	g := callgraph.Build(nodes)
	callgraph.AddTypedEdges(g, typed)
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
