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
// Over-approximations (a reached node is not proof the product runs it):
// every func value a linked package references counts as called — `var _
// = f` included; an interface call reaches every implementation the typed
// pass knows, whether or not the product ever constructs that type; and a
// `package main` excluded by a build constraint (`//go:build ignore`)
// still counts as an entry point.
//
// Deterministic: entries are sorted, the walk is a multi-source BFS over
// sorted adjacency, and each reached node records the entry whose walk
// reached it first in BFS order (the nearest; ties go to the entry that
// sorts first) — not necessarily the only entry that reaches it.

import (
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
}

// ProductEntryReach computes g's product entry points and everything they
// reach. O(V+E) per init fixpoint round (in practice two or three).
func ProductEntryReach(g *Graph) *ProductReach {
	r := &ProductReach{g: g, reached: map[string]string{}}
	if g == nil {
		return r
	}
	var mains []string
	initsByPkg := map[string][]string{}
	for key, n := range g.nodes {
		if !isGoProductSource(n) {
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
// newly reached node the source whose walk got there first.
func (r *ProductReach) walk(sources []string) {
	type item struct{ key, entry string }
	queue := make([]item, 0, len(sources))
	for _, s := range sources {
		if _, seen := r.reached[s]; !seen {
			r.reached[s] = s
			queue = append(queue, item{s, s})
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range r.g.callees[cur.key] {
			if _, seen := r.reached[next]; seen {
				continue
			}
			r.reached[next] = cur.entry
			queue = append(queue, item{next, cur.entry})
		}
	}
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
