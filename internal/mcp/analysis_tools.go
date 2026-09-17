// Package mcp — analysis tool registrations backed by callgraph.Graph.
//
// registerAnalysisTools wires rename_impact, reachable, test_impact, and
// dead_code into the server's tool registry.
//
// The call-graph tools build a *callgraph.Graph on each invocation by calling
// backend.ListAllSymbols and passing the result to callgraph.Build.  This is
// intentionally simple: caching can be added later without changing the public
// tool API.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// registerAnalysisTools is called from registerTools (tools.go) and adds the
// four analysis tools.  backend may be nil; tools that require it return
// ErrCodeInternal when backend == nil.
func registerAnalysisTools(s *Server, backend QueryInterface) {
	registerRenameImpact(s, backend)
	registerReachable(s, backend)
	registerTestImpact(s, backend)
	registerDeadCode(s, backend)
}

// ─── buildGraph is a shared helper ──────────────────────────────────────────

// buildGraph fetches all symbols for service from backend and constructs a
// callgraph.Graph.  Returns (nil, rpcError) on any failure.
func buildGraph(ctx context.Context, backend QueryInterface, service string) (*callgraph.Graph, *rpcError) {
	nodes, err := backend.ListAllSymbols(ctx, service)
	if err != nil {
		return nil, &rpcError{Code: ErrCodeInternal, Message: "list symbols: " + err.Error()}
	}
	return callgraph.Build(nodes), nil
}

// ─── rename_impact ────────────────────────────────────────────────────────────

func registerRenameImpact(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "rename_impact",
		Description: "The answer grep cannot give: every call site that would need updating if symbol were renamed (direct callers only) — grep's text match cannot resolve identity across a bare name shared by unrelated symbols. Reach for it when you are about to rename a symbol and need every call site a text search would miss or over-match. Required: service, symbol.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{
					"type":        "string",
					"description": "Service / repo identifier",
				},
				"symbol": map[string]any{
					"type":        "string",
					"description": "Symbol name (bare or fully-qualified SymbolKey)",
				},
			},
			"required": []string{"service", "symbol"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
			Symbol  string `json:"symbol"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "rename_impact: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "rename_impact: service required"}
		}
		if p.Symbol == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "rename_impact: symbol required"}
		}
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "list symbols: " + err.Error()}
		}
		g := callgraph.Build(nodes)
		callers := callgraph.RenameImpact(g, p.Symbol)

		// resolution/index_commit (VB-19, docs/vision/29-verbs-not-lookups.md
		// §3 wave E): tally each RETURNED caller's tier against a SEPARATE
		// typed-edge-augmented graph — never the one RenameImpact itself
		// walked, since AddTypedEdges can introduce callers the syntax pass
		// never had (BP-26) and rename_impact's caller SET must stay
		// exactly what it always was; only the tier annotation is additive.
		typed, _ := backend.TypedEdges(ctx, p.Service)
		tg := callgraph.Build(nodes)
		callgraph.AddTypedEdges(tg, typed)
		kinds := make(map[string]core.EdgeKind, len(callers))
		for _, d := range callgraph.CallersDetail(tg, p.Symbol) {
			kinds[d.Key] = d.Kind
		}
		var mix resolutionMix
		for _, c := range callers {
			mix.addKind(kinds[c])
		}

		// Return as []any so JSON serialises to an array of strings.
		result := map[string]any{"callers": stringsToAny(callers), "resolution": mix}
		if ic := indexCommitFor(ctx, backend, p.Service); ic != "" {
			result["index_commit"] = ic
		}
		return withSummary(renameImpactSummary(p.Symbol, callers), result), nil
	})
}

// renameImpactSummary renders rename_impact's answer-first sentence (wave
// L1): the count and the first three call sites, built from callers —
// exactly the list the "callers" field returns, so the summary can never
// claim a site the body does not carry.
func renameImpactSummary(symbol string, callers []string) string {
	if len(callers) == 0 {
		return fmt.Sprintf("0 call sites for %s — nothing to rename", symbol)
	}
	n := len(callers)
	if n > 3 {
		n = 3
	}
	summary := fmt.Sprintf("%d call site%s for %s: %s", len(callers), pluralS(len(callers)), symbol, strings.Join(callers[:n], ", "))
	if len(callers) > n {
		summary += fmt.Sprintf(" (+%d more)", len(callers)-n)
	}
	return capSummary(summary)
}

// ─── reachable ────────────────────────────────────────────────────────────────

func registerReachable(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "reachable",
		Description: "Report whether there is any directed call path from one symbol to another.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{
					"type":        "string",
					"description": "Service / repo identifier",
				},
				"from": map[string]any{
					"type":        "string",
					"description": "Starting symbol (bare name or fully-qualified SymbolKey)",
				},
				"to": map[string]any{
					"type":        "string",
					"description": "Target symbol (bare name or fully-qualified SymbolKey)",
				},
			},
			"required": []string{"service", "from", "to"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
			From    string `json:"from"`
			To      string `json:"to"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "reachable: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "reachable: service required"}
		}
		if p.From == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "reachable: from required"}
		}
		if p.To == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "reachable: to required"}
		}
		g, rpcErr := buildGraph(ctx, backend, p.Service)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return map[string]any{"reachable": callgraph.Reachable(g, p.From, p.To)}, nil
	})
}

// ─── test_impact ──────────────────────────────────────────────────────────────

func registerTestImpact(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "test_impact",
		Description: "The answer grep cannot give: every test function (names prefixed with 'Test') that transitively reaches the given symbol via the call graph — no text search walks a call graph to a Test* root. Reach for it when you changed a symbol and need to know which tests actually exercise it, not which tests merely mention its name. Required: service, symbol.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{
					"type":        "string",
					"description": "Service / repo identifier",
				},
				"symbol": map[string]any{
					"type":        "string",
					"description": "Symbol name (bare or fully-qualified SymbolKey)",
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
			return nil, &rpcError{Code: ErrCodeParams, Message: "test_impact: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "test_impact: service required"}
		}
		if p.Symbol == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "test_impact: symbol required"}
		}
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "list symbols: " + err.Error()}
		}
		g := callgraph.Build(nodes)
		tests := callgraph.TestImpact(g, p.Symbol)

		// resolution/index_commit (VB-19, docs/vision/29-verbs-not-lookups.md
		// §3 wave E): tally each RETURNED test's tier — the confidence of
		// the edge it was first reached through, callerDepths' own BD-2
		// semantics — against a SEPARATE typed-edge-augmented graph, never
		// the one TestImpact itself walked (AddTypedEdges can introduce
		// reachability the syntax pass never had, BP-26); test_impact's
		// returned SET must stay exactly what it always was.
		typed, _ := backend.TypedEdges(ctx, p.Service)
		tg := callgraph.Build(nodes)
		callgraph.AddTypedEdges(tg, typed)
		infos := callerDepths(tg, p.Symbol, 0)
		var mix resolutionMix
		for _, t := range tests {
			if info, ok := infos[t]; ok {
				mix.addConfidence(info.Confidence)
			} else {
				mix.Heuristic++
			}
		}

		result := map[string]any{"tests": stringsToAny(tests), "resolution": mix}
		if ic := indexCommitFor(ctx, backend, p.Service); ic != "" {
			result["index_commit"] = ic
		}
		return withSummary(testImpactSummary(p.Symbol, tests, infos), result), nil
	})
}

// testImpactSummary renders test_impact's answer-first sentence (wave L1):
// how many tests reach symbol, and the nearest one — built from tests (the
// exact list the "tests" field returns) and infos (the same per-test depth
// tally the resolution mix above already summed), so "nearest" can never
// name a test that is not actually in the body.
func testImpactSummary(symbol string, tests []string, infos map[string]depthInfo) string {
	if len(tests) == 0 {
		return fmt.Sprintf("no tests reach %s", symbol)
	}
	nearest := ""
	nearestDepth := -1
	for _, t := range tests {
		d := 0
		if info, ok := infos[t]; ok {
			d = info.Depth
		}
		if nearestDepth == -1 || d < nearestDepth || (d == nearestDepth && t < nearest) {
			nearestDepth, nearest = d, t
		}
	}
	verb := "reach"
	if len(tests) == 1 {
		verb = "reaches"
	}
	summary := fmt.Sprintf("%d test%s %s %s", len(tests), pluralS(len(tests)), verb, symbol)
	if nearest != "" {
		summary += "; nearest: " + nearest
	}
	return capSummary(summary)
}

// ─── dead_code ────────────────────────────────────────────────────────────────

func registerDeadCode(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "dead_code",
		Description: "Find symbols that are neither reachable from any entrypoint nor called by anything else in the graph.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{
					"type":        "string",
					"description": "Service / repo identifier",
				},
				"entrypoints": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Entrypoint symbol names (bare or fully-qualified SymbolKeys); typically main functions or exported handlers",
				},
			},
			"required": []string{"service"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service     string   `json:"service"`
			Entrypoints []string `json:"entrypoints"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "dead_code: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "dead_code: service required"}
		}
		g, rpcErr := buildGraph(ctx, backend, p.Service)
		if rpcErr != nil {
			return nil, rpcErr
		}
		dead := callgraph.DeadCode(g, p.Entrypoints)
		return map[string]any{"dead": stringsToAny(dead)}, nil
	})
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// stringsToAny converts []string to []any so json.Marshal always produces a
// JSON array (nil []string would marshal as null).
func stringsToAny(ss []string) []any {
	if ss == nil {
		return []any{}
	}
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}
