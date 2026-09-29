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
	reached map[string]string // node key -> the entry whose walk reached it first

	// The walk's rapid-type-analysis state, dropped when the walk ends:
	// the receiver types (typeID) some reached caller instantiates, and the
	// interface dispatch targets waiting on a type not instantiated yet.
	rta          bool
	instantiated map[string]bool
	pending      map[string][]walkItem
}

// walkItem is one node on the walk's queue and the entry whose walk
// reached it.
type walkItem struct{ key, entry string }

// ProductEntryReach computes g's product entry points and everything they
// reach. O(V+E) per init fixpoint round (in practice two or three).
func ProductEntryReach(g *Graph) *ProductReach {
	r := &ProductReach{g: g, reached: map[string]string{}}
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
			initsByPkg[n.PackagePath()] = append(initsByPkg[n.PackagePath()], key)
		}
	}
	sort.Strings(mains)
	for _, keys := range initsByPkg {
		sort.Strings(keys)
	}

	frontier := mains
	linked := map[string]bool{}
	for len(frontier) > 0 {
		r.entries = append(r.entries, frontier...)
		r.walk(frontier)
		// Every package the walk touched is linked into some binary; its
		// init functions run at start-up and become entry points too.
		var next []string
		for key := range r.reached {
			pkg := g.nodes[key].PackagePath()
			if linked[pkg] {
				continue
			}
			linked[pkg] = true
			for _, ik := range initsByPkg[pkg] {
				if _, done := r.reached[ik]; !done {
					next = append(next, ik)
				}
			}
		}
		sort.Strings(next)
		frontier = next
	}
	sort.Strings(r.entries)
	return r
}

// walk is a multi-source BFS from sources (sorted), recording for each
// newly reached node the source whose walk got there first. An interface
// dispatch target whose receiver type no reached caller instantiates yet
// waits in r.pending until one does (EW-15), in the order the walk met it.
func (r *ProductReach) walk(sources []string) {
	queue := make([]walkItem, 0, len(sources))
	reach := func(key, entry string) {
		if _, seen := r.reached[key]; !seen {
			r.reached[key] = entry
			queue = append(queue, walkItem{key, entry})
		}
	}
	for _, s := range sources {
		reach(s, s)
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
					reach(w.key, w.entry)
				}
				delete(r.pending, id)
			}
		}
		callerFile := r.g.nodes[cur.key].FilePath
		for _, next := range r.g.callees[cur.key] {
			if _, seen := r.reached[next]; seen {
				continue
			}
			if n := r.g.nodes[next]; n.Kind == core.KindGoMethod {
				kind, typed := r.g.edgeKind[cur.key+"\x00"+next]
				if !typed && r.g.typedFiles[callerFile] {
					continue // the syntax pass's guess at a receiver the typed pass resolved
				}
				if r.rta && kind == core.EdgeIfaceDispatch && r.g.typedDirs[path.Dir(n.FilePath)] {
					if id := methodReceiverID(n); !r.instantiated[id] {
						r.pending[id] = append(r.pending[id], walkItem{next, cur.entry})
						continue
					}
				}
			}
			reach(next, cur.entry)
		}
	}
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
// key or any name Canonical accepts), naming the entry that does. An
// unresolvable or ambiguous symbol is not reached.
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

// EstimatedBytes is the reach set's resident size beyond the graph it
// reads (the memo registry's byte budget, I51): one map entry per reached
// node, its key string shared with the graph.
func (r *ProductReach) EstimatedBytes() int64 {
	if r == nil {
		return 0
	}
	return int64(len(r.reached))*(2*strHdr+mapEntryOverhead) + int64(len(r.entries))*strHdr
}
