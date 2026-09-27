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
	"github.com/geoffmilleraz/fathom/internal/memo"
	"github.com/geoffmilleraz/fathom/internal/scope"
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
	// Nodes is the shared snapshot's slice: read it, never sort or edit
	// it (serviceSymbols hands out a copy).
	Nodes    []core.SymbolNode
	Snapshot *memo.Snapshot
	Typed    []core.Edge
	// Syntax is the syntax-only graph (no typed edges) — what
	// symbol_context's "callers"/"callees" read.
	Syntax *callgraph.Graph
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
	// TypedErr is set when this call could not read the typed edges: the
	// graphs are the syntax edges only, for this call alone (never cached).
	TypedErr error
}

// storeIdentifier is the OPTIONAL backend capability naming the store a
// backend reads (pkg/client's Client): the graph store and snapshot keys
// carry it, so two stores holding a same-named service never share an
// entry, and the MCP tools and the console — one client — share one.
type storeIdentifier interface {
	StoreID() string
}

// freshnessEnsurer is the OPTIONAL backend capability that runs the
// auto-refresh freshness check a symbol load would have run (autorefresh.go
// ensure) before a snapshot is taken: a cached snapshot must not also skip
// the refresh the uncached load triggered.
type freshnessEnsurer interface {
	EnsureFresh(ctx context.Context, service string)
}

// navScope is service's (store, service, watermark) scope; the watermark
// is "" when the backend cannot name one (never cached, I21).
func navScope(ctx context.Context, backend QueryInterface, service string) memo.Scope {
	if e, ok := backend.(freshnessEnsurer); ok {
		e.EnsureFresh(ctx, service)
	}
	return navScopeNoEnsure(ctx, backend, service)
}

// navScopeNoEnsure is navScope without the freshness check (the refresh
// path patching the store's own state).
func navScopeNoEnsure(ctx context.Context, backend QueryInterface, service string) memo.Scope {
	id, ok := backend.(storeIdentifier)
	if !ok || id.StoreID() == "" {
		// A backend that cannot name its store is never cached: an address
		// is not an identity (a freed backend's address is reused).
		return memo.Scope{Service: service}
	}
	wm := serviceWatermark(ctx, backend, service)
	if wm != "" {
		if t, ok := backend.(suppressionTokener); ok {
			// A suppression change is a new key: a cached snapshot never
			// serves rows a suppression now hides (SP-2).
			if tok := t.SuppressionToken(service); tok != "" {
				wm += "|suppressed:" + tok
			}
		}
	}
	return memo.Scope{Store: id.StoreID(), Service: service, Watermark: wm}
}

// suppressionTokener is the OPTIONAL backend capability naming the
// suppressions currently applied to a service's reads (pkg/client).
type suppressionTokener interface {
	SuppressionToken(service string) string
}

// serviceSnapshot is service's symbol snapshot (RT-75): one immutable load
// per (store, service, watermark), shared by every iterating tool, the
// navigation graphs and the console.
func serviceSnapshot(ctx context.Context, backend QueryInterface, service string) (*memo.Snapshot, error) {
	sc := navScope(ctx, backend, service)
	snap, err := memo.SnapshotForExtra(ctx, sc, func(lctx context.Context) ([]core.SymbolNode, any, error) {
		// The load's suppression disclosure is kept on the snapshot and
		// replayed to every reader below: a cached snapshot still says
		// what it hides (SP-3).
		cctx, col := scope.WithCollector(lctx)
		//fathom:tenancy-allow sources the nav memo's loader: the one shared snapshot per watermark (single flight, memo-budgeted)
		nodes, err := backend.ListAllSymbols(cctx, service)
		return nodes, col.For(service), err
	})
	if err != nil {
		return nil, err
	}
	if d, ok := snap.Extra.(*scope.Disclosure); ok {
		scope.Record(ctx, service, d)
	}
	return snap, nil
}

// serviceSymbols is the service's symbols as a slice the caller may sort
// or edit (a copy of the shared snapshot's): the iterating tools' load
// (RT-75), never a per-call store read at a cached watermark.
func serviceSymbols(ctx context.Context, backend QueryInterface, service string) ([]core.SymbolNode, error) {
	snap, err := serviceSnapshot(ctx, backend, service)
	if err != nil {
		return nil, err
	}
	return snap.NodesCopy(), nil
}

// typedLoader loads the service's typed edges through the backend. A read
// failure is returned, never read as "none": the graph store does not
// cache a failure, so it cannot stand for the whole watermark (I38); the
// callers degrade that one call (memo.GraphForTyped).
func typedLoader(backend QueryInterface, service string) memo.TypedLoader {
	return func(ctx context.Context) ([]core.Edge, error) {
		return backend.TypedEdges(ctx, service)
	}
}

// loadNavGraphs returns service's snapshot and its three graphs, each
// from the graph store (RT-74): one build per (store, service, watermark,
// kind), shared by every tool and the console, single-flighted with
// ctx-aware waiters. Returns nil when the backend cannot list symbols (or
// the caller left); an index with no symbols yields empty graphs.
func (s *Server) loadNavGraphs(ctx context.Context, backend QueryInterface, service string) *navGraphs {
	g, err := loadNavGraphsErr(ctx, backend, service)
	if err != nil {
		return nil
	}
	return g
}

// loadNavGraphsErr is loadNavGraphs with the load error.
func loadNavGraphsErr(ctx context.Context, backend QueryInterface, service string) (*navGraphs, error) {
	snap, err := serviceSnapshot(ctx, backend, service)
	if err != nil {
		return nil, err
	}
	typed := typedLoader(backend, service)
	g := &navGraphs{Nodes: snap.Nodes, Snapshot: snap}
	if g.Syntax, err = memo.GraphFor(ctx, snap, memo.GraphSyntax, nil); err != nil {
		return nil, err
	}
	// A typed-edge read failure degrades this call to the syntax edges
	// (TypedAvailable false, TypedErr naming it) and is never cached: the
	// next call reads the typed edges again (I38).
	if g.Both, g.Typed, g.TypedErr, err = memo.GraphForTyped(ctx, snap, memo.GraphBoth, typed); err != nil {
		return nil, err
	}
	if g.TypedErr == nil {
		if g.Resolved, _, g.TypedErr, err = memo.GraphForTyped(ctx, snap, memo.GraphResolved, typed); err != nil {
			return nil, err
		}
	}
	if g.TypedErr != nil {
		g.Typed, g.Both, g.Resolved = nil, g.Syntax, memo.EmptyGraph()
	}
	g.TypedAvailable = g.Resolved.HasEdges()
	return g, nil
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
