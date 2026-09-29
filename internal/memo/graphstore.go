package memo

// graphstore.go — one symbol snapshot and one graph store per (service,
// watermark) (docs/design/responsible-tenant.md §5, RT-74, RT-75, I51).
//
// Before this, a service's symbols were loaded per call by every iterating
// tool, the navigation memo and the console palette each kept a private
// copy, and three caches (callgraph.CachedCallGraph, the console's project
// index and its plans graphs) could each hold the same service's graph.
// Now every caller asks here: SnapshotFor loads the service's symbols once
// per (store, service, watermark) into an immutable Snapshot with a name
// index and a callee index, and GraphFor builds each graph kind once per
// snapshot — both in memo groups, so they are single-flighted, byte-
// accounted, trimmed at shed, dropped at refuse and evicted when the
// service's watermark moves. callgraph.Build/BuildBoth/BuildResolved have
// no caller in the serving paths (internal/mcp, the console) but this.
//
// A Scope with no watermark (a backend that cannot name its store's state:
// test fakes, a service never ingested) is built on every call and never
// cached (I21).

import (
	"context"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// Scope names one service's state: the store it lives in (so two stores
// holding a same-named service never share an entry), the service, and
// the store watermark.
type Scope struct {
	Store, Service, Watermark string
}

type snapKey struct{ Scope }

func (k snapKey) MemoScope() (string, string) { return k.Store + "\x00" + k.Service, k.Watermark }

// GraphKind names a graph built from a snapshot.
type GraphKind string

// Graph kinds.
const (
	// GraphSyntax is the syntax-derived call graph (callgraph.Build).
	GraphSyntax GraphKind = "syntax"
	// GraphBoth adds the typed pass's edges (callgraph.BuildBoth).
	GraphBoth GraphKind = "both"
	// GraphResolved carries only typed-pass resolved edges
	// (callgraph.BuildResolved).
	GraphResolved GraphKind = "resolved"
)

type graphKey struct {
	Scope
	Kind GraphKind
}

func (k graphKey) MemoScope() (string, string) { return k.Store + "\x00" + k.Service, k.Watermark }

type typedKey struct{ Scope }

func (k typedKey) MemoScope() (string, string) { return k.Store + "\x00" + k.Service, k.Watermark }

// Expected costs of a cold build on a large service (the RT-74/75
// measurements: a 145k-observation fleet store, fathom's own service): a
// flight past half of it outlives a caller that gave up (a hook's short
// deadline), so the next caller finds it built; one short of that is
// cancelled with its last waiter.
const (
	snapshotCost = 2 * time.Second
	typedCost    = time.Second
	graphCost    = 2 * time.Second
)

var (
	snapshotGroup = NewGroup[snapKey, *Snapshot]("snapshot", Options{WatermarkScoped: true, Keep: true, ExpectedCost: snapshotCost})
	typedGroup    = NewGroup[typedKey, []core.Edge]("typed_edges", Options{WatermarkScoped: true, Keep: true, ExpectedCost: typedCost})
	graphGroup    = NewGroup[graphKey, *callgraph.Graph]("graphstore", Options{WatermarkScoped: true, Keep: true, ExpectedCost: graphCost})
	// productGroup holds each graph's product-entry reach set (verify's
	// `wired` criterion, ADR-018 amendment 2026-09-28): one O(V+E) walk
	// per (store, service, watermark, kind), dropped with its graph. Its
	// build nests the graph's (GraphFor) when that is not built yet, so it
	// is kept like one: a walk a caller gave up on finishes for the next
	// caller instead of being cancelled with it (EW-22, O130).
	productGroup = NewGroup[graphKey, *callgraph.ProductReach]("product_reach", Options{WatermarkScoped: true, Keep: true, ExpectedCost: graphCost})
)

// Snapshot is one service's symbols at one watermark. Immutable: callers
// read Nodes and never modify the slice or its elements (NodesCopy hands
// out a copy for callers that sort or edit).
type Snapshot struct {
	Scope Scope
	Nodes []core.SymbolNode
	// lower holds each node's lowercased search fields, a field already
	// lower case sharing its original string.
	lower   []Lowered
	names   map[string][]int32 // lowercased name -> node indexes
	callees map[string][]int32 // callee key and its final name -> caller node indexes
	// Extra is what the loader returned beside the nodes (SnapshotForExtra):
	// the MCP tools keep the load's suppression disclosure here and replay
	// it to every reader, so a cached snapshot still says what it hides.
	Extra any
	// palette lists each distinct (name, kind, package, ref) symbol once,
	// doc chunks and nameless rows excluded — the console palette's set.
	palette []int32
}

// Lowered is a node's lowercased search fields (the console palette).
type Lowered struct {
	Name, Kind, Package, Ref string
}

// Named returns the nodes whose Name equals name, case-insensitively, in
// snapshot order — a point lookup, never a scan.
func (s *Snapshot) Named(name string) []core.SymbolNode {
	idx := s.names[strings.ToLower(name)]
	out := make([]core.SymbolNode, len(idx))
	for i, j := range idx {
		out[i] = s.Nodes[j]
	}
	return out
}

// CallersOf returns the nodes whose Calls carry key, or a key whose final
// name segment is key (a candidate superset callers narrow with their own
// match), in snapshot order.
func (s *Snapshot) CallersOf(key string) []core.SymbolNode {
	idx := s.callees[key]
	out := make([]core.SymbolNode, len(idx))
	for i, j := range idx {
		out[i] = s.Nodes[j]
	}
	return out
}

// Palette returns the indexes of the distinct named, non-doc-chunk
// symbols, in snapshot order (the console palette's rows).
func (s *Snapshot) Palette() []int32 { return s.palette }

// Ref is node i's place: its file, with ":line" when the file names none.
func (s *Snapshot) Ref(i int) string { return nodeRef(&s.Nodes[i]) }

func nodeRef(n *core.SymbolNode) string {
	if n.Line > 0 && !strings.Contains(n.FilePath, ":") {
		return n.FilePath + ":" + itoa(n.Line)
	}
	return n.FilePath
}

// Lower returns node i's lowercased search fields.
func (s *Snapshot) Lower(i int) Lowered { return s.lower[i] }

// NodesCopy returns a copy of the node slice a caller may sort or edit.
func (s *Snapshot) NodesCopy() []core.SymbolNode {
	return append([]core.SymbolNode(nil), s.Nodes...)
}

// Len is the node count.
func (s *Snapshot) Len() int { return len(s.Nodes) }

// SnapshotLoader loads a service's symbols (the backend's ListAllSymbols).
type SnapshotLoader func(ctx context.Context) ([]core.SymbolNode, error)

// SnapshotFor returns the service's symbol snapshot at sc, loading it once
// per (store, service, watermark) and sharing it with every concurrent
// caller.
func SnapshotFor(ctx context.Context, sc Scope, load SnapshotLoader) (*Snapshot, error) {
	return snapshotGroup.Get(ctx, snapKey{sc}, func(bctx context.Context) (*Snapshot, int64, error) {
		nodes, err := load(bctx)
		if err != nil {
			return nil, 0, err
		}
		s := newSnapshot(sc, nodes)
		return s, snapshotBytes(s), nil
	})
}

// NewSnapshot indexes nodes into an unscoped snapshot: GraphFor builds
// its graphs on every call and never caches them (no watermark) — for
// callers holding a node set that is not a service's stored state (test
// fixtures, a filtered subset).
func NewSnapshot(nodes []core.SymbolNode) *Snapshot { return newSnapshot(Scope{}, nodes) }

// SnapshotForExtra is SnapshotFor with a loader that also returns a value
// kept on the snapshot (Snapshot.Extra).
func SnapshotForExtra(ctx context.Context, sc Scope, load func(ctx context.Context) ([]core.SymbolNode, any, error)) (*Snapshot, error) {
	return snapshotGroup.Get(ctx, snapKey{sc}, func(bctx context.Context) (*Snapshot, int64, error) {
		nodes, extra, err := load(bctx)
		if err != nil {
			return nil, 0, err
		}
		s := newSnapshot(sc, nodes)
		s.Extra = extra
		return s, snapshotBytes(s), nil
	})
}

// newSnapshot indexes nodes (taken over, not copied).
func newSnapshot(sc Scope, nodes []core.SymbolNode) *Snapshot {
	s := &Snapshot{Scope: sc, Nodes: nodes, lower: make([]Lowered, len(nodes)),
		names: make(map[string][]int32, len(nodes)), callees: make(map[string][]int32)}
	intern := map[string]string{}
	low := func(v string) string {
		if l, ok := intern[v]; ok {
			return l
		}
		l := strings.ToLower(v)
		if l == v {
			l = v
		}
		intern[v] = l
		return l
	}
	seen := make(map[[4]string]struct{}, len(nodes))
	for i := range nodes {
		n := &nodes[i]
		ref := nodeRef(n)
		if n.Name != "" && n.Kind != core.KindDocChunk {
			k := [4]string{n.Name, string(n.Kind), n.Package, ref}
			if _, dup := seen[k]; !dup {
				seen[k] = struct{}{}
				s.palette = append(s.palette, int32(i))
			}
		}
		lname := strings.ToLower(n.Name)
		if lname == n.Name {
			lname = n.Name
		}
		lref := strings.ToLower(ref)
		if lref == ref {
			lref = ref
		}
		s.lower[i] = Lowered{Name: lname, Kind: low(string(n.Kind)), Package: low(n.Package), Ref: lref}
		s.names[lname] = append(s.names[lname], int32(i))
		seenCall := map[string]bool{}
		for _, c := range n.Calls {
			for _, k := range [2]string{c, calleeName(c)} {
				if k == "" || seenCall[k] {
					continue
				}
				seenCall[k] = true
				s.callees[k] = append(s.callees[k], int32(i))
			}
		}
	}
	return s
}

// calleeName is a call key's final name segment ("svc::pkg.Name" → "Name").
func calleeName(key string) string {
	if i := strings.LastIndex(key, ":"); i >= 0 {
		key = key[i+1:]
	}
	if i := strings.LastIndex(key, "."); i >= 0 {
		key = key[i+1:]
	}
	return key
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// snapshotBytes estimates a snapshot's resident size: the node structs and
// their strings, the lowered fields and both indexes.
func snapshotBytes(s *Snapshot) int64 {
	const nodeOverhead, indexEntry = 320, 64 // calibrated: TestByteEstimatesTrackTheHeap
	var n int64
	for i := range s.Nodes {
		nd := &s.Nodes[i]
		n += nodeOverhead + int64(len(nd.Name)+len(nd.Package)+len(nd.FilePath)+len(nd.Signature)+len(nd.Service))
		for _, c := range nd.Calls {
			n += 16 + int64(len(c))
		}
	}
	n += int64(len(s.lower))*64 + int64(len(s.palette))*4
	n += int64(len(s.names)+len(s.callees)) * indexEntry
	return n
}

// TypedLoader loads a service's typed call edges (the backend's
// TypedEdges); nil means the service has none.
type TypedLoader func(ctx context.Context) ([]core.Edge, error)

// TypedFor returns the service's typed edges at sc, loaded once per
// (store, service, watermark).
func TypedFor(ctx context.Context, sc Scope, load TypedLoader) ([]core.Edge, error) {
	if load == nil {
		return nil, nil
	}
	return typedGroup.Get(ctx, typedKey{sc}, func(bctx context.Context) ([]core.Edge, int64, error) {
		edges, err := load(bctx)
		if err != nil {
			return nil, 0, err
		}
		var n int64
		for _, e := range edges {
			n += 96 + int64(len(e.FromSymbol)+len(e.ToSymbol)+len(e.FromFile)+len(e.FromService))
		}
		return edges, n, nil
	})
}

// GraphFor returns snap's graph of kind — built once per (store, service,
// watermark, kind) and shared: callers read it and never add edges to it.
// Both and resolved graphs take the typed edges from typed (TypedFor).
func GraphFor(ctx context.Context, snap *Snapshot, kind GraphKind, typed TypedLoader) (*callgraph.Graph, error) {
	return graphGroup.Get(ctx, graphKey{snap.Scope, kind}, func(bctx context.Context) (*callgraph.Graph, int64, error) {
		var edges []core.Edge
		if kind != GraphSyntax {
			var err error
			if edges, err = TypedFor(bctx, snap.Scope, typed); err != nil {
				return nil, 0, err
			}
		}
		var g *callgraph.Graph
		switch kind {
		case GraphBoth:
			g = callgraph.BuildBoth(snap.Nodes, edges)
		case GraphResolved:
			g = callgraph.BuildResolved(snap.Nodes, edges)
		default:
			g = callgraph.Build(snap.Nodes)
		}
		// The graph's own maps, summed (callgraph.EstimatedBytes): a flat
		// per-node guess undercounted about 3× and let the cache fraction
		// admit several times its budget.
		return g, g.EstimatedBytes(), nil
	})
}

// ProductReachFor returns what the product entry points of snap's graph
// of kind reach (callgraph.ProductEntryReach) — computed once per (store,
// service, watermark, kind) from the shared graph GraphFor serves, and
// dropped whenever that graph is patched or dropped.
func ProductReachFor(ctx context.Context, snap *Snapshot, kind GraphKind, typed TypedLoader) (*callgraph.ProductReach, error) {
	return productGroup.Get(ctx, graphKey{snap.Scope, kind}, func(bctx context.Context) (*callgraph.ProductReach, int64, error) {
		g, err := GraphFor(bctx, snap, kind, typed)
		if err != nil {
			return nil, 0, err
		}
		r := callgraph.ProductEntryReach(g)
		return r, r.EstimatedBytes(), nil
	})
}

// HasGraphs reports whether anything a refresh must patch is cached for
// sc — the snapshot (the iterating tools read it) or any graph — so a
// refresh asks before paying to list a package's nodes.
func HasGraphs(sc Scope) bool {
	if _, ok := snapshotGroup.Peek(snapKey{sc}); ok {
		return true
	}
	for _, k := range []GraphKind{GraphSyntax, GraphBoth, GraphResolved} {
		if _, ok := graphGroup.Peek(graphKey{sc, k}); ok {
			return true
		}
	}
	return false
}

// PatchPackage is SU-4's in-place refresh at the graph store: a single-
// file re-ingest replaced package pkg's rows without moving the watermark,
// so the cached snapshot and graphs at sc take pkg's complete new node set
// (and, when typed is non-nil, its typed edges) instead of being rebuilt —
// a full build costs seconds on a large service. The syntax and both
// graphs are patched copy-on-write (callgraph.Graph.ReplacePackage: a
// reader holding the old graph keeps a consistent one); the resolved graph
// is dropped (ReplacePackage re-derives syntax edges it must never carry)
// and rebuilds on its next use.
func PatchPackage(sc Scope, pkg string, nodes []core.SymbolNode, typed []core.Edge) {
	snapshotGroup.Update(snapKey{sc}, func(old *Snapshot) (*Snapshot, int64) {
		kept := make([]core.SymbolNode, 0, len(old.Nodes)+len(nodes))
		for _, n := range old.Nodes {
			if n.Package != pkg {
				kept = append(kept, n)
			}
		}
		kept = append(kept, nodes...)
		s := newSnapshot(sc, kept)
		s.Extra = old.Extra // the load's disclosure still describes it
		return s, snapshotBytes(s)
	})
	for _, k := range []GraphKind{GraphSyntax, GraphBoth} {
		t := typed
		if k == GraphSyntax {
			t = nil
		}
		graphGroup.Update(graphKey{sc, k}, func(old *callgraph.Graph) (*callgraph.Graph, int64) {
			ng := old.ReplacePackage(pkg, nodes, t)
			return ng, ng.EstimatedBytes()
		})
	}
	graphGroup.Remove(graphKey{sc, GraphResolved}, ReasonWatermark)
	for _, k := range []GraphKind{GraphSyntax, GraphBoth, GraphResolved} {
		productGroup.Remove(graphKey{sc, k}, ReasonWatermark)
	}
	if typed != nil {
		typedGroup.Remove(typedKey{sc}, ReasonWatermark)
	}
}

// EvictStoreService drops every entry of service in store — its snapshot,
// typed edges and graphs at any watermark, and entries keyed by the
// service alone (layer coverage) — the moment an ingest of it completes
// (§5: eager eviction on WatermarkBumped), instead of leaving superseded
// ones resident until the service is next read at its new watermark.
func EvictStoreService(store, service string) {
	reg := graphGroup.reg
	reg.EvictService(store + "\x00" + service)
	reg.EvictService(service)
}

// DropScope forgets sc's snapshot, typed edges and graphs — the honest
// fallback when a refresh cannot name the package to patch.
func DropScope(sc Scope) {
	snapshotGroup.Remove(snapKey{sc}, ReasonWatermark)
	typedGroup.Remove(typedKey{sc}, ReasonWatermark)
	for _, k := range []GraphKind{GraphSyntax, GraphBoth, GraphResolved} {
		graphGroup.Remove(graphKey{sc, k}, ReasonWatermark)
		productGroup.Remove(graphKey{sc, k}, ReasonWatermark)
	}
}

// GraphForTyped is GraphFor that also returns the typed edges and never
// caches a typed-edge read failure as "no typed edges" (I38). When the
// read fails for any reason but the caller's end, this call degrades — the
// syntax graph for GraphSyntax and GraphBoth (both with no typed edges is
// the syntax graph), an empty graph for GraphResolved — and typedErr names
// the failure; nothing built without the typed edges is cached, so the
// next call reads them again. err is the snapshot's graph build failing or
// the caller's ctx ending.
func GraphForTyped(ctx context.Context, snap *Snapshot, kind GraphKind, typed TypedLoader) (g *callgraph.Graph, edges []core.Edge, typedErr, err error) {
	if kind == GraphSyntax {
		g, err = GraphFor(ctx, snap, GraphSyntax, nil)
		return g, nil, nil, err
	}
	edges, terr := TypedFor(ctx, snap.Scope, typed)
	if terr != nil {
		if ctx.Err() != nil {
			return nil, nil, nil, ctx.Err()
		}
		if kind == GraphResolved {
			return EmptyGraph(), nil, terr, nil
		}
		g, err = GraphFor(ctx, snap, GraphSyntax, nil)
		return g, nil, terr, err
	}
	g, err = GraphFor(ctx, snap, kind, typed)
	return g, edges, nil, err
}

// EmptyGraph is a graph with no nodes and no edges: the resolved graph of
// a call that could not read the typed edges (GraphForTyped).
func EmptyGraph() *callgraph.Graph { return callgraph.BuildResolved(nil, nil) }
