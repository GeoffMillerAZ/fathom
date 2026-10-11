package callgraph

// product.go — "done means proven AND wired" (ADR-018 amendment 2026-09-28):
// which symbols a PRODUCT entry point reaches, as opposed to only a test.
// internal/verify's `wired` criterion reads it; a claimed plan row whose
// symbol a Test* reaches but no entry point does is library-only.
//
// An entry point is where a built program starts running code of this
// service:
//
//   - every `func main` of a Go `package main` outside a testdata/ tree
//     (every cmd/ binary, bench drivers included — a harness package a
//     bench binary calls is wired, because that binary is a product);
//   - every `func init` of a package the first set links, iterated to a
//     fixpoint — the runtime runs an init whenever its package is linked,
//     so what it registers is live.
//
// Registered MCP tool handlers, CLI verbs, console routes and hook
// entries need no rule of their own: a handler registered by reference
// (mux.HandleFunc(p, h), a route-table literal, a method value) is a
// func-value edge from the registering function (VS-6, I23), and a call
// inside a closure counts for its enclosing function (VS-7), so they are
// reached through the binary that registers them. A table built in a
// package-level var initializer is attributed to "<pkg>.init" by the
// typed pass (TypedEdgeEpoch 3), so it is reached with its package.
//
// Hardened against the 2026-09-29 adversarial corpus (EW-15,
// testdata/hardening):
//
//   - a bare reference assigned to the blank identifier (`var _ = f`,
//     `_ = f`) is no edge at all (the extractors drop it);
//   - a `func main` or `func init` in a file no default build compiles
//     (core.ToolKindGoBuildExcluded: `//go:build ignore`, a custom tag) is
//     not an entry point;
//   - a method matches only its own receiver type: a syntax-only edge into
//     a method — a receiver-variable call the syntax pass matched by name
//     ("r.Len" to the one first-party Len) — is not followed from a file
//     the typed pass resolved, which gave that call site its own answer;
//   - interface dispatch is cross-checked against the types the reached
//     code instantiates (rapid type analysis): an EdgeIfaceDispatch edge is
//     followed into T's method only once some reached caller holds a T
//     (core.EdgeInstantiates). A service whose typed edges carry no
//     instantiation facts (written before TypedEdgeEpoch 4), or a method
//     in a package the typed pass could not load, keeps the old reading:
//     every implementation the typed pass knows.
//
// Over-approximations that remain (a reached node is not proof the
// product runs it): every func value a linked package references counts
// as called, a func literal held in a package variable included — its
// calls are attributed to the package's init whether or not anything
// invokes it; and a type counts as instantiated wherever a reached caller
// holds a value of it, not only where the value is converted to the
// interface.
//
// Typed edges only (RF-5, ADR-067 D3, I57): the walk follows an edge only
// when its resolution tier is typed (typedreach.go), so `wired` never
// passes on a guess, and a package counts as linked only when typed
// reach touches it. What the entry points reach only through a heuristic
// edge is walked second and kept apart (HeuristicReachedBy), naming the
// edge the reach would rest on, so a refusal can say why.
//
// Deterministic: entries are sorted, the walk is a multi-source BFS over
// sorted adjacency, and each reached node records the entry whose walk
// reached it first in BFS order (the nearest; ties go to the entry that
// sorts first) — not necessarily the only entry that reaches it.

import (
	"path"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// ProductReach is the set of nodes some product entry point reaches.
// Immutable once built; safe for concurrent readers.
type ProductReach struct {
	g       *Graph
	entries []string
	reached map[string]string // node key -> the entry whose typed walk reached it first
	// heuristic holds what the entry points reach only through an edge of
	// tier heuristic: the entry, and the first such edge on the path.
	heuristic map[string]heuristicReach

	// The walk's rapid-type-analysis state, dropped when the walk ends:
	// the receiver types (typeID) some reached caller instantiates, and the
	// interface dispatch targets waiting on a type not instantiated yet.
	rta          bool
	instantiated map[string]bool
	pending      map[string][]walkItem
}

// walkItem is one node on the walk's queue, the entry whose walk reached
// it, and — in the heuristic walk — the heuristic edge its path crossed
// (nil: reached over typed edges only).
type walkItem struct {
	key, entry string
	via        *HeuristicEdge
}

// heuristicReach is a node reached only through a heuristic edge.
type heuristicReach struct {
	entry string
	via   HeuristicEdge
}

// ProductEntryReach computes g's product entry points and everything they
// reach over typed edges, then what they reach only through a heuristic
// edge. O(V+E) per init fixpoint round (in practice two or three) per walk.
func ProductEntryReach(g *Graph) *ProductReach {
	r := &ProductReach{g: g, reached: map[string]string{}, heuristic: map[string]heuristicReach{}}
	if g == nil {
		return r
	}
	r.rta = len(g.instantiates) > 0
	r.instantiated, r.pending = map[string]bool{}, map[string][]walkItem{}
	defer func() { r.instantiated, r.pending = nil, nil }()
	var mains []string
	initsByPkg := map[string][]string{}
	for key, n := range g.nodes {
		if !isGoProductSource(n) || n.ToolKind == core.ToolKindGoBuildExcluded {
			continue
		}
		switch {
		case n.Name == "main" && n.Package == "main":
			mains = append(mains, key)
		case n.Name == "init":
			initsByPkg[linkUnit(n)] = append(initsByPkg[linkUnit(n)], key)
		}
	}
	sort.Strings(mains)
	for _, keys := range initsByPkg {
		sort.Strings(keys)
	}

	// The typed walk: what the product reaches over typed edges.
	linked := map[string]bool{}
	frontier := make([]walkItem, len(mains))
	for i, m := range mains {
		frontier[i] = walkItem{key: m, entry: m}
	}
	for len(frontier) > 0 {
		for _, it := range frontier {
			r.entries = append(r.entries, it.key)
		}
		r.walk(frontier, true)
		// Every package the walk touched is linked into some binary; its
		// init functions run at start-up and become entry points too.
		frontier = r.linkInits(sortedKeys(r.reached), linked, initsByPkg, func(string) walkItem { return walkItem{} })
	}
	sort.Strings(r.entries)

	// The heuristic walk: from everything the typed walk reached, over
	// every edge, recording for each newly reached node the first
	// heuristic edge its path crossed. A package it links runs its init
	// behind the edge that linked it.
	seeds := make([]walkItem, 0, len(r.reached))
	for _, k := range sortedKeys(r.reached) {
		seeds = append(seeds, walkItem{key: k, entry: r.reached[k]})
	}
	for len(seeds) > 0 {
		r.walk(seeds, false)
		heur := make([]string, 0, len(r.heuristic))
		for k := range r.heuristic {
			heur = append(heur, k)
		}
		sort.Strings(heur)
		seeds = r.linkInits(heur, linked, initsByPkg, func(k string) walkItem {
			h := r.heuristic[k]
			via := h.via
			return walkItem{entry: h.entry, via: &via}
		})
	}
	return r
}

// linkInits marks the packages of keys (sorted) linked and returns the
// init functions of each newly linked package not yet reached, sorted,
// each walkItem's entry and via taken from link(the key that linked it)
// (the typed walk's items are their own entries).
func (r *ProductReach) linkInits(keys []string, linked map[string]bool, initsByPkg map[string][]string, link func(string) walkItem) []walkItem {
	var next []walkItem
	for _, key := range keys {
		pkg := linkUnit(r.g.nodes[key])
		if linked[pkg] {
			continue
		}
		linked[pkg] = true
		for _, ik := range initsByPkg[pkg] {
			if r.seen(ik) {
				continue
			}
			it := link(key)
			it.key = ik
			if it.entry == "" {
				it.entry = ik
			}
			next = append(next, it)
		}
	}
	sort.Slice(next, func(i, j int) bool { return next[i].key < next[j].key })
	return next
}

// linkUnit is the unit the Go linker links a node with: its package,
// named by its directory (one non-test package per directory). Not the
// node's PackagePath: a method's names its receiver type, so a package
// reached only through a method (main calls reg.T's Run) would otherwise
// be linked only by a reference to the type, an edge of tier heuristic.
func linkUnit(n core.SymbolNode) string { return path.Dir(n.FilePath) }

// seen reports whether either walk has reached key.
func (r *ProductReach) seen(key string) bool {
	if _, ok := r.reached[key]; ok {
		return true
	}
	_, ok := r.heuristic[key]
	return ok
}

// walk is a multi-source BFS from sources (sorted), recording for each
// newly reached node the source whose walk got there first. The typed walk
// (typedOnly) follows only edges of resolution tier typed and records into
// reached; the heuristic walk follows every edge, re-walks its sources
// (already reached) and records into heuristic, with the first heuristic
// edge the path crossed. An interface dispatch target whose receiver type
// no reached caller instantiates yet waits in r.pending until one does
// (EW-15), in the order the walk met it.
func (r *ProductReach) walk(sources []walkItem, typedOnly bool) {
	queue := make([]walkItem, 0, len(sources))
	reach := func(it walkItem) {
		if r.seen(it.key) {
			return
		}
		switch {
		case typedOnly:
			r.reached[it.key] = it.entry
		case it.via == nil:
			// Unreachable by construction (every typed edge out of the
			// typed reach was followed; a dispatch target still pending
			// is released only by a node behind a heuristic edge): an
			// item with no edge to name is not reached at all.
			return
		default:
			r.heuristic[it.key] = heuristicReach{entry: it.entry, via: *it.via}
		}
		queue = append(queue, it)
	}
	for _, s := range sources {
		if !typedOnly && r.seen(s.key) {
			queue = append(queue, s)
			continue
		}
		reach(s)
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if r.rta {
			for _, tk := range r.g.instantiates[cur.key] {
				id := typeKeyID(tk)
				if r.instantiated[id] {
					continue
				}
				r.instantiated[id] = true
				for _, w := range r.pending[id] {
					if w.via == nil {
						w.via = cur.via
					}
					reach(w)
				}
				delete(r.pending, id)
			}
		}
		callerFile := r.g.nodes[cur.key].FilePath
		for _, next := range r.g.callees[cur.key] {
			if r.seen(next) {
				continue
			}
			typed := r.g.typedEdge(cur.key, next)
			if typedOnly && !typed {
				continue
			}
			via := cur.via
			if via == nil && !typed {
				via = &HeuristicEdge{From: cur.key, To: next, Kind: r.g.edgeLabel(cur.key, next)}
			}
			if n := r.g.nodes[next]; n.Kind == core.KindGoMethod {
				kind, marked := r.g.edgeKind[cur.key+"\x00"+next]
				if !marked && r.g.typedFiles[callerFile] {
					// The syntax pass's guess at a receiver the typed pass
					// resolved. AddTypedEdges drops these from the graph
					// itself since wave lg (dropTypedFileGuesses), so this
					// never fires on a graph it built; kept so the walk's
					// rule does not depend on how the graph was assembled.
					continue
				}
				if r.rta && kind == core.EdgeIfaceDispatch && r.g.typedDirs[path.Dir(n.FilePath)] {
					if id := methodReceiverID(n); !r.instantiated[id] {
						r.pending[id] = append(r.pending[id], walkItem{key: next, entry: cur.entry, via: via})
						continue
					}
				}
			}
			reach(walkItem{key: next, entry: cur.entry, via: via})
		}
	}
}

// sortedKeys returns m's keys, sorted.
func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// typeKeyID is a type's identity across the two ways the graph names it:
// its directory and its declared name, from a type key
// ("svc::<dir>/<pkg>.<Name>").
func typeKeyID(key string) string {
	if i := strings.Index(key, "::"); i >= 0 {
		key = key[i+2:]
	}
	dir, tail := "", key
	if i := strings.LastIndex(key, "/"); i >= 0 {
		dir, tail = key[:i], key[i+1:]
	}
	if i := strings.LastIndex(tail, "."); i >= 0 {
		tail = tail[i+1:]
	}
	return dir + "\x00" + tail
}

// methodReceiverID is typeKeyID for a method node's receiver type: the
// method's directory and its Package field, which holds the receiver
// type's name.
func methodReceiverID(n core.SymbolNode) string {
	dir := path.Dir(n.FilePath)
	if dir == "." || dir == "/" {
		dir = ""
	}
	return dir + "\x00" + n.Package
}

// isGoProductSource reports whether n is Go code a built program can run:
// a non-test .go file outside any testdata/ tree (the go tool ignores
// testdata, so a fixture's `package main` is never this service's binary).
func isGoProductSource(n core.SymbolNode) bool {
	if n.Kind != core.KindGoFunc {
		return false
	}
	p := n.FilePath
	if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
		return false
	}
	return !strings.HasPrefix(p, "testdata/") && !strings.Contains(p, "/testdata/")
}

// Entries returns the sorted canonical keys of the product entry points.
func (r *ProductReach) Entries() []string { return append([]string(nil), r.entries...) }

// HasEntries reports whether the service has any product entry point. A
// service with none (a library) has no product to be wired into.
func (r *ProductReach) HasEntries() bool { return len(r.entries) > 0 }

// Len is the number of nodes the entry points reach (entries included).
func (r *ProductReach) Len() int { return len(r.reached) }

// ReachedBy reports whether a product entry point reaches symbol (a full
// key or any name Canonical accepts) over typed edges, naming the entry
// that does. An unresolvable or ambiguous symbol is not reached; neither is
// one reached only through a heuristic edge (HeuristicReachedBy).
func (r *ProductReach) ReachedBy(symbol string) (entry string, ok bool) {
	if r.g == nil {
		return "", false
	}
	key := r.g.canonicalize(symbol)
	if key == "" {
		return "", false
	}
	entry, ok = r.reached[key]
	return entry, ok
}

// HeuristicReachedBy reports whether a product entry point reaches symbol
// only through an edge of resolution tier heuristic, naming the entry and
// the first such edge on its path (RF-5): the reach `wired` refuses, and
// the reason it refuses it.
func (r *ProductReach) HeuristicReachedBy(symbol string) (entry string, via HeuristicEdge, ok bool) {
	if r.g == nil {
		return "", HeuristicEdge{}, false
	}
	key := r.g.canonicalize(symbol)
	if key == "" {
		return "", HeuristicEdge{}, false
	}
	h, ok := r.heuristic[key]
	return h.entry, h.via, ok
}

// EstimatedBytes is the reach set's resident size beyond the graph it
// reads (the memo registry's byte budget, I51): one map entry per reached
// node, its key string shared with the graph.
func (r *ProductReach) EstimatedBytes() int64 {
	if r == nil {
		return 0
	}
	// A heuristic entry holds an entry and an edge: four strings and a kind.
	return int64(len(r.reached))*(2*strHdr+mapEntryOverhead) + int64(len(r.heuristic))*(6*strHdr+mapEntryOverhead) +
		int64(len(r.entries))*strHdr
}
