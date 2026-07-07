// Package packagedeps builds an internal package dependency graph from a
// []core.SymbolNode and provides pure, deterministic queries on top of it.
//
// An edge pkg A → pkg B is emitted whenever a symbol in package A has a Calls
// entry whose canonical SymbolKey resolves to a symbol whose Package field is B
// (and B != A).  Calls entries follow the same tolerant-matching convention used
// by internal/analysis/callgraph: a full key "svc::pkg.Name" or a bare
// "pkg.Name" are both accepted; bare "Name" entries without a package prefix
// are skipped (they cannot be attributed to a foreign package).
//
// All public functions are pure, deterministic, and safe for concurrent use.
package packagedeps

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/depgraph"
)

// Build constructs a package-level dependency graph from nodes.
//
// For each symbol node, every entry in its Calls slice is parsed to extract the
// target package.  Recognised formats:
//
//	"svc::pkg.Name"  → target package is "pkg"
//	"pkg.Name"       → target package is "pkg"
//	"Name"           → no package information; skipped
//
// A directed edge from-package → to-package is added when they differ.
// All packages that appear as a source (even those with no outgoing deps) are
// registered as nodes so Deps returns an empty non-nil slice for them.
func Build(nodes []core.SymbolNode) *depgraph.Graph {
	g := depgraph.NewGraph()

	// Build a lookup: callee key fragments → package.
	// We also register every node's own package.
	for _, n := range nodes {
		if n.Package != "" {
			g.AddNode(n.Package)
		}
	}

	for _, n := range nodes {
		if n.Package == "" {
			continue
		}
		for _, callee := range n.Calls {
			toPkg := calleePackage(callee)
			if toPkg == "" {
				continue
			}
			g.AddEdge(n.Package, toPkg)
		}
	}
	return g
}

// calleePackage extracts the package segment from a callee key.
//
// Formats handled:
//
//	"svc::pkg.Name"  → "pkg"
//	"pkg.Name"       → "pkg"
//	"Name"           → ""  (no package info)
func calleePackage(key string) string {
	// Strip optional "svc::" prefix.
	if idx := strings.Index(key, "::"); idx >= 0 {
		key = key[idx+2:]
	}
	// Now key is either "pkg.Name" or "Name".
	if dot := strings.LastIndex(key, "."); dot > 0 {
		return key[:dot]
	}
	return ""
}

// Deps returns the direct package dependencies of pkg in sorted order.
// It delegates to g.Deps, returning nil when pkg is absent and a non-nil
// (possibly empty) sorted slice otherwise.
func Deps(g *depgraph.Graph, pkg string) []string {
	return g.Deps(pkg)
}

// Cycles returns all cyclic strongly-connected components in the package graph
// (components with more than one node, or a single node with a self-loop).
// Each component is a sorted slice of package names; the outer slice is sorted
// by the first element of each component for full determinism.
func Cycles(g *depgraph.Graph) [][]string {
	cycles := g.Cycles()
	if cycles == nil {
		return [][]string{}
	}
	// Ensure each inner slice is sorted (depgraph already does this, but be
	// defensive), then sort the outer slice.
	for _, c := range cycles {
		sort.Strings(c)
	}
	sort.Slice(cycles, func(i, j int) bool {
		return cycles[i][0] < cycles[j][0]
	})
	return cycles
}
