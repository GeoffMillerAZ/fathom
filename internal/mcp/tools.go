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

	"github.com/geoffmilleraz/sextant/internal/core"
)

// registerTools registers all MCP tools into s. backend may be nil, in which
// case query tools return ErrCodeInternal when called.
func registerTools(s *Server, backend QueryInterface) {
	// set_profile — always available on stdio; rejected over HTTP, where the
	// daemon is shared and the profile would leak across sessions.
	s.registerTool(toolDescriptor{
		Name:        "set_profile",
		Description: "Switch the active tool profile to filter tools/list results (stdio only — over HTTP register a path-scoped surface such as POST /rpc/companion instead). Valid profiles: companion, coding, investigation, shared, platform.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"profile": map[string]any{
					"type":        "string",
					"description": "Profile name: companion | coding | investigation | shared | platform",
					"enum":        []string{"companion", "coding", "investigation", "shared", "platform"},
				},
			},
			"required": []string{"profile"},
		},
		group: GroupShared,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if scopeFrom(ctx).viaHTTP {
			return nil, &rpcError{Code: ErrCodeInvalid, Message: "set_profile is disabled over HTTP: the daemon is shared and the profile is process-global. Register a path-scoped surface instead, e.g. POST /rpc/companion (valid surfaces: companion, coding, investigation, shared, platform; POST /rpc is the full surface)."}
		}
		var p struct {
			Profile string `json:"profile"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "set_profile: invalid args"}
		}
		if rpcErr := s.setProfile(p.Profile); rpcErr != nil {
			return nil, rpcErr
		}
		return map[string]any{"profile": p.Profile, "ok": true}, nil
	})

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
					"description": "Concern ID to scope the query (e.g. \"authentication\")",
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
		group: GroupCoding,
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
		return map[string]any{"callers": toNodeDTOs(nodes)}, nil
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
					"description": "Symbol name (bare or fully-qualified; exact spelling not required)",
				},
			},
			"required": []string{"service", "name"},
		},
		group: GroupShared,
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
	registerNavTools(s, backend)

	// Register source-scanning analyzer tools.
	registerSourceTools(s, backend)

	// Register meta-tools, prompts, and resources.
	registerMeta(s, backend)

	// Register extra analyzer tools (impact, spec, arch, coding, etc.).
	registerExtraTools(s, backend)
}

// ─── DTOs ────────────────────────────────────────────────────────────────────

// symbolNodeDTO is the JSON-serialisable representation of a SymbolNode
// returned to MCP clients.
type symbolNodeDTO struct {
	Service   string `json:"service"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	FilePath  string `json:"file_path"`
	Line      int    `json:"line,omitempty"`
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
			Signature: n.Signature,
			Tier:      string(n.Tier),
		}
	}
	return dtos
}
