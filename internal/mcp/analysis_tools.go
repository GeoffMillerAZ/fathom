// Package mcp — analysis tool registrations backed by callgraph.Graph.
//
// registerAnalysisTools wires test_impact into the server's tool registry.
// reachable was unregistered by SR-12 (F-22) and dead_code by SR-19 (F-77,
// both surface reckoning). rename_impact was unregistered by SR-24 (F-13,
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
	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// registerAnalysisTools is called from registerTools (tools.go) and adds the
// one surviving analysis tool. backend may be nil; tools that require it
// return ErrCodeInternal when backend == nil.
func registerAnalysisTools(s *Server, backend QueryInterface) {
	registerTestImpact(s, backend)
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
		Description: "Every Test* function reaching symbol via the call graph; rank:\"risk\" instead ranks untested, dangerous symbols, factors cited. Reach for it when you changed a symbol, to see what exercises it. Required: service. symbol too, unless rank is set.",
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
				"rank": map[string]any{
					"type":        "string",
					"enum":        []string{"risk"},
					"description": "\"risk\": rank the service's untested, dangerous symbols (each factor cited) instead of one symbol's tests",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "rank:\"risk\" only: rows returned (default 20, max 500)",
				},
			},
			"required": []string{"service"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
			Symbol  string `json:"symbol"`
			Rank    string `json:"rank"`
			Limit   int    `json:"limit"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "test_impact: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "test_impact: service required"}
		}
		switch p.Rank {
		case "":
		case "risk":
			return testImpactRiskRank(ctx, backend, p.Service, p.Limit)
		default:
			return nil, &rpcError{Code: ErrCodeParams, Message: fmt.Sprintf("test_impact: unknown rank %q (want \"risk\")", p.Rank)}
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

// testImpactRiskRank is test_impact's rank:"risk" answer (TQ-5,
// docs/plans/test-intelligence.md; design doc §3 T1): the service's
// risk-weighted coverage list from certify.RiskWeightedCoverage — a sort
// over factors stored at ingest, never a graph walk (I48) — served
// through the existing tool rather than a new one. tests stays present
// and empty, so the payload keeps test_impact's shape. Coverage comes from
// the sources the service's own repository names (TQ-26:
// certify.CoverageSources over its root, `[tests] results_dir`), read as
// files and never from a store; risk.coverage_source names what was read,
// and with none every uncovered_share is 1 and its citation says so. The
// tool's input schema is unchanged.
func testImpactRiskRank(ctx context.Context, backend QueryInterface, service string, limit int) (any, *rpcError) {
	st, ok := storeOf(backend)
	if !ok {
		return nil, &rpcError{Code: ErrCodeInternal, Message: "test_impact: rank risk needs a store-backed backend"}
	}
	root, _, _, err := backend.ServiceMeta(ctx, service)
	if err != nil {
		return nil, &rpcError{Code: ErrCodeInternal, Message: "test_impact: service meta: " + err.Error()}
	}
	rep, err := certify.RiskWeightedCoverage(ctx, st, service, root, certify.RiskOptions{Limit: limit, Sources: certify.CoverageSources(root)})
	if err != nil {
		return nil, &rpcError{Code: ErrCodeInternal, Message: "test_impact: " + err.Error()}
	}
	result := map[string]any{"tests": []any{}, "resolution": resolutionMix{}, "risk": rep}
	if ic := indexCommitFor(ctx, backend, service); ic != "" {
		result["index_commit"] = ic
	}
	return withSummary(testImpactRiskSummary(rep), result), nil
}

// testImpactRiskSummary is rank:"risk"'s answer-first sentence: the top
// symbol and its score, or why the list is empty.
func testImpactRiskSummary(rep certify.RiskReport) string {
	if len(rep.Rows) == 0 {
		if rep.Watermark == "" {
			return "no risk rank: centrality has not been computed for this service (re-ingest it)"
		}
		return "no risk rank: the service has no non-test symbols"
	}
	top := rep.Rows[0]
	cov := "no coverage source named"
	if rep.CoverageSource != "" {
		cov = "coverage from " + rep.CoverageSource
	}
	return capSummary(fmt.Sprintf("%d of %d symbols ranked by risk; highest: %s (%.3g); %s", len(rep.Rows), rep.Considered, top.Symbol, top.Risk, cov))
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

// dead_code (F-77) was unregistered by SR-19 (surface reckoning): 0
// lifetime calls; callgraph.DeadCode stays (only its own test calls it
// now).

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
