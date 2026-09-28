// Package mcp — analysis tool registrations backed by callgraph.Graph.
//
// registerAnalysisTools wires reachable, test_impact, and dead_code into the
// server's tool registry. rename_impact was unregistered by SR-24 (F-13,
// design §4); renameImpactAnswer below is now reached through
// blast_radius's kind:"rename" (blastRadiusByKind, extra_tools.go).
//
// The call-graph tools walk the service's syntax-plus-typed graph through the
// server's watermark-keyed cache (loadNavGraphs, BP-28/I21) — the SAME graph
// verify_plan and the nav tools walk (O98), so a method value, an interface
// dispatch, or a bare call on a receiver that shares a method name with
// another type resolves identically on every surface. renameImpactAnswer
// alone keeps its syntax-only caller SET by decision (BP-26) and annotates
// tiers from the typed graph.
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
// three surviving analysis tools. backend may be nil; tools that require it
// return ErrCodeInternal when backend == nil.
func registerAnalysisTools(s *Server, backend QueryInterface) {
	registerTestImpact(s, backend)
	registerDeadCode(s, backend)
}

// ─── buildGraph is a shared helper ──────────────────────────────────────────

// bothGraph returns the service's symbols and its syntax-plus-typed call
// graph, memoized under the store watermark (O98: one graph for every
// reachability answer the daemon gives). An index with no symbols yields an
// empty graph, as it always did — only a backend failure is an error.
func bothGraph(ctx context.Context, s *Server, backend QueryInterface, service string) ([]core.SymbolNode, *callgraph.Graph, *rpcError) {
	ng, err := loadNavGraphsErr(ctx, backend, service)
	if err != nil {
		return nil, nil, &rpcError{Code: ErrCodeInternal, Message: "list symbols: " + err.Error()}
	}
	return ng.Nodes, ng.Both, nil
}

// buildGraph is bothGraph for callers that need only the graph.
func buildGraph(ctx context.Context, s *Server, backend QueryInterface, service string) (*callgraph.Graph, *rpcError) {
	_, g, rpcErr := bothGraph(ctx, s, backend, service)
	return g, rpcErr
}

// claimsAnchoredBackend is the optional backend capability for SU-21's
// governance-on-rename receipt (ADR-041 decision 7): claims whose "anchors"
// claim link names a given core.SymbolKey. pkg/client implements it; a
// backend that omits it (an older module, a minimal test fake) leaves
// rename_impact's response exactly as it was before this field existed —
// no claims_anchored key at all, never a fabricated zero.
type claimsAnchoredBackend interface {
	ClaimsAnchoredTo(ctx context.Context, serviceID, symbolKey string) ([]string, error)
}

// ─── rename_impact (retired as a standalone tool, SR-24 — see
// blastRadiusByKind, extra_tools.go, kind:"rename") ──────────────────────────

// renameImpactAnswer is the retired rename_impact tool's exact answer,
// unregistered by SR-24 (F-13, design §4) and now reached through
// blast_radius's kind:"rename" (blastRadiusByKind, extra_tools.go) —
// byte-compatible with what rename_impact itself used to return.
func renameImpactAnswer(ctx context.Context, backend QueryInterface, service, symbol string) (any, *rpcError) {
	if backend == nil {
		return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
	}
	if service == "" {
		return nil, &rpcError{Code: ErrCodeParams, Message: "blast_radius: service required"}
	}
	if symbol == "" {
		return nil, &rpcError{Code: ErrCodeParams, Message: `blast_radius: kind "rename" requires symbol`}
	}
	ng, err := loadNavGraphsErr(ctx, backend, service)
	if err != nil {
		return nil, &rpcError{Code: ErrCodeInternal, Message: "list symbols: " + err.Error()}
	}
	g := ng.Syntax
	callers := callgraph.RenameImpact(g, symbol)

	// resolution/index_commit (VB-19, docs/vision/29-verbs-not-lookups.md
	// §3 wave E): tally each RETURNED caller's tier against a SEPARATE
	// typed-edge-augmented graph — never the one RenameImpact itself
	// walked, since AddTypedEdges can introduce callers the syntax pass
	// never had (BP-26) and rename_impact's caller SET must stay
	// exactly what it always was; only the tier annotation is additive.
	tg := ng.Both
	kinds := make(map[string]core.EdgeKind, len(callers))
	for _, d := range callgraph.CallersDetail(tg, symbol) {
		kinds[d.Key] = d.Kind
	}
	var mix resolutionMix
	for _, c := range callers {
		mix.addKind(kinds[c])
	}

	// Return as []any so JSON serialises to an array of strings.
	result := map[string]any{"callers": stringsToAny(callers), "resolution": mix}
	if ic := indexCommitFor(ctx, backend, service); ic != "" {
		result["index_commit"] = ic
	}
	// SU-21 (ADR-041 decision 7): a rename is never silent for
	// governance — any claim anchored to the renamed symbol (via an
	// "anchors" claim link) is named here, not just its call sites.
	// canonicalize through the SAME graph RenameImpact itself just
	// walked, so the key this looks up is the identical symbol
	// RenameImpact resolved symbol to (bare name or full key alike).
	if key := callgraph.Canonical(g, symbol); key != "" {
		if cab, ok := backend.(claimsAnchoredBackend); ok {
			if ids, aerr := cab.ClaimsAnchoredTo(ctx, service, key); aerr == nil && len(ids) > 0 {
				result["claims_anchored"] = len(ids)
				result["claims_anchored_ids"] = stringsToAny(ids)
			}
		}
	}
	return withSummary(renameImpactSummary(symbol, callers), result), nil
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

// ─── test_impact ──────────────────────────────────────────────────────────────

func registerTestImpact(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "test_impact",
		Description: "Every Test* function reaching symbol via the call graph — no text search walks it. Reach for it when you changed a symbol, to see what exercises it. Required: service, symbol.",
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
			return nil, &rpcError{Code: ErrCodeParams, Message: "test_impact: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "test_impact: service required"}
		}
		if p.Symbol == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "test_impact: symbol required"}
		}
		_, g, rpcErr := bothGraph(ctx, s, backend, p.Service)
		if rpcErr != nil {
			return nil, rpcErr
		}
		tests := callgraph.TestImpact(g, p.Symbol)

		// resolution/index_commit (VB-19, docs/vision/29-verbs-not-lookups.md
		// §3 wave E): tally each RETURNED test's tier — the confidence of
		// the edge it was first reached through, callerDepths' own BD-2
		// semantics — against a SEPARATE typed-edge-augmented graph, never
		// the one TestImpact itself walked (AddTypedEdges can introduce
		// reachability the syntax pass never had, BP-26); test_impact's
		// returned SET must stay exactly what it always was.
		tg := g // bothGraph's graph already carries the typed edges
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
		g, rpcErr := buildGraph(ctx, s, backend, p.Service)
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
