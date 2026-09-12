// Package mcp — resolved-by-default navigation (bench-pyramid BP-26/BP-27/
// BP-28, ADR-032).
//
// The L0 index oracle (internal/indexbench) measured what the nav tools
// were actually returning on this repository at 10,651 symbols:
// find_callers precision 0.607, blast_radius precision 0.086 over 10,158
// predicted symbols per query, at ~10.9s a query. Recall was high in both.
// That shape — near-perfect recall, terrible precision — is the signature of
// the syntax pass's tolerant name matching: every bare-name collision in the
// fleet becomes an edge, and a transitive walk multiplies them.
//
// So the DEFAULT changed (docs/plans/bench-pyramid.md BP-26/BP-27): the nav
// tools answer from typed-pass (resolved) edges, and the syntax-only
// (heuristic) edges are opt-in via `include_heuristic: true`. Every row
// still carries edge_confidence (BD-2) — the label did not go away, the
// default selection did.
//
// What did NOT change: the verify engine's reachability. It reads
// impact.BlastRadiusWithTypedEdges / callgraph on both edge kinds exactly as
// before (pinned by TestVerifyReachabilityUsesBothEdgeKinds and
// internal/fusion's TestReachabilityFixtureEndToEnd). A gate whose verdict
// moved because a display default moved would be a silent regression.
package mcp

import (
	"context"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// Default bounds for blast_radius (BP-27). Depth 2 is "what breaks and what
// probably breaks"; beyond that the answer on a real repository is most of
// the repository, which is not an answer. Both are overridable per call.
const (
	defaultBlastDepth   = 2
	defaultBlastSymbols = 500
)

// storeWatermarker is the OPTIONAL backend capability that makes the graph
// memo safe: a token that changes on every ingest and only on an ingest
// (*pkg/client.Client implements it from store.ServiceMeta). A backend
// without it — every test fake, and any future transport that cannot name
// the store's state — simply never hits the cache, which costs time and can
// never serve a stale graph. Capability, never requirement: adding this to
// QueryInterface would have broken thirteen fakes to buy nothing.
type storeWatermarker interface {
	StoreWatermark(ctx context.Context, serviceID string) (string, error)
}

// serviceWatermark returns the backend's store watermark for service, or ""
// when the backend cannot name one. "" disables memoization for that call.
func serviceWatermark(ctx context.Context, backend QueryInterface, service string) string {
	w, ok := backend.(storeWatermarker)
	if !ok {
		return ""
	}
	mark, err := w.StoreWatermark(ctx, service)
	if err != nil {
		return ""
	}
	return mark
}

// navGraphs is one service's two graphs plus the facts a handler needs to
// report honestly about them.
type navGraphs struct {
	Nodes []core.SymbolNode
	Typed []core.Edge
	// Resolved carries only typed-pass edges; Both adds the syntax edges.
	Resolved *callgraph.Graph
	Both     *callgraph.Graph
	// TypedAvailable is false when the service has no resolved edges at all
	// (a non-Go corpus, or a Go corpus whose typed pass has not run). A
	// resolved-by-default answer MUST fall back to the heuristic edges in
	// that case: an empty answer off an empty graph is not evidence of no
	// callers, and silently returning it would trade bad precision for
	// invented recall.
	TypedAvailable bool
}

// loadNavGraphs builds (or reuses) the two graphs for service. Both are
// memoized under the store watermark (BP-28, I21) in the server's cache, so
// a session's second nav call on an unchanged index pays nothing to build.
// Returns nil when the backend cannot list symbols.
func (s *Server) loadNavGraphs(ctx context.Context, backend QueryInterface, service string) *navGraphs {
	nodes, err := backend.ListAllSymbols(ctx, service)
	if err != nil || len(nodes) == 0 {
		return nil
	}
	typed, _ := backend.TypedEdges(ctx, service)
	mark := serviceWatermark(ctx, backend, service)

	g := &navGraphs{Nodes: nodes, Typed: typed}
	g.Both = s.graphCache.Graph(service+"\x00both", mark, func() *callgraph.Graph {
		return callgraph.BuildBoth(nodes, typed)
	})
	g.Resolved = s.graphCache.Graph(service+"\x00resolved", mark, func() *callgraph.Graph {
		return callgraph.BuildResolved(nodes, typed)
	})
	g.TypedAvailable = g.Resolved.HasEdges()
	return g
}

// pick returns the graph a handler should walk given the caller's
// include_heuristic choice, plus the edge_filter label to report. The
// fallback when a service has no typed edges is deliberate and is reported
// as "all": the tool says which evidence answered, every time.
func (g *navGraphs) pick(includeHeuristic bool) (*callgraph.Graph, string) {
	if includeHeuristic || !g.TypedAvailable {
		return g.Both, "all"
	}
	return g.Resolved, "resolved"
}

// resolvedCallersOnly filters a service's direct-caller rows to the ones the
// typed pass resolved (BP-26).
//
// It answers off the RESOLVED graph rather than by filtering the syntax
// engine's rows, because the two disagree in both directions: the syntax
// pass invents callers through bare-name collisions (the precision loss the
// oracle measured), and it MISSES callers reachable only through interface
// dispatch, which have no Calls entry at all. keep is the set of canonical
// keys to report; ok is false when the service has no typed edges and the
// caller must fall back to the heuristic answer rather than report nothing.
func resolvedCallersOnly(g *navGraphs, symbol string) (keep map[string]bool, ok bool) {
	if g == nil || !g.TypedAvailable {
		return nil, false
	}
	rows := callgraph.ResolvedCallersAcross(g.Resolved, symbol)
	keep = make(map[string]bool, len(rows))
	for _, r := range rows {
		keep[r.Key] = true
	}
	// A symbol that names NO node in the graph (a name the index does not
	// carry) is not a "no callers" answer — it is a question the resolved
	// graph cannot take. Fall back. An AMBIGUOUS name is a question it can
	// take: ResolvedCallersAcross unions the candidates rather than
	// refusing, so `find_callers Reachable` no longer degrades to the
	// syntax answer just because two packages declare that name (BP-31).
	if rows == nil {
		return nil, false
	}
	return keep, true
}

// blastRadiusResolved computes a blast radius at resolved confidence, under
// the default bounds (BP-27): depth 2 and 500 symbols unless the caller says
// otherwise, resolved edges unless include_heuristic is set.
//
// This is the agent-facing path ONLY. impact.BlastRadiusWithTypedEdges —
// unbounded, both edge kinds — is untouched and is still what the verify
// engine's reachability walks.
func blastRadiusResolved(g *navGraphs, changedFiles []string, includeHeuristic bool, maxDepth, maxSymbols int) boundedBlast {
	if g == nil {
		return boundedBlast{BoundedResult: impact.BoundedResult{
			Result: impact.Result{Symbols: []string{}, Tests: []string{}, Files: []string{}}, EdgeFilter: "all",
		}, Buckets: []blastBucket{}}
	}
	if maxDepth < 0 {
		maxDepth = defaultBlastDepth
	}
	if maxSymbols < 0 {
		maxSymbols = defaultBlastSymbols
	}
	graph, filter := g.pick(includeHeuristic)
	res := impact.BlastRadiusBounded(graph, g.Nodes, changedFiles, impact.Options{
		MaxDepth:   maxDepth,
		MaxSymbols: maxSymbols,
	})
	res.EdgeFilter = filter
	listed := make(map[string]bool, len(res.Symbols))
	for _, k := range res.Symbols {
		listed[k] = true
	}
	return boundedBlast{
		BoundedResult: res,
		Buckets:       blastBucketsOf(graph, g.Nodes, changedFiles, maxDepth, listed),
	}
}

// boundedBlast is a bounded blast radius plus its by_depth summary, computed
// over the same graph and the same bounds — never a wider walk.
type boundedBlast struct {
	impact.BoundedResult
	Buckets []blastBucket
}
