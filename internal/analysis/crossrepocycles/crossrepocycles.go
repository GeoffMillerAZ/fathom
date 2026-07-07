// Package crossrepocycles detects import cycles across service/repo boundaries.
//
// Each service is treated as a node; the edges are the explicit import
// relationships declared by the caller (serviceImports).  Cycles are detected
// using the depgraph package's Kosaraju-based SCC algorithm, which is
// deterministic.
//
// All public functions are pure, deterministic, and safe for concurrent use.
package crossrepocycles

import (
	"sort"

	"github.com/geoffmilleraz/fathom/internal/depgraph"
)

// Detect finds all import cycles in the cross-repo service dependency graph.
//
// serviceImports maps each service name to the list of services it imports.
// All services mentioned (either as keys or as values) are registered as nodes,
// so isolated services with no imports appear in the graph but will not be
// reported as cycles.
//
// The returned value is a slice of cycles, where each cycle is a sorted slice
// of service names that form a strongly-connected component.  The outer slice
// is sorted by the first element of each cycle for full determinism.
// Returns an empty (non-nil) slice when no cycles are found.
func Detect(serviceImports map[string][]string) [][]string {
	g := depgraph.NewGraph()

	// Register all services as nodes first (ensures isolated services appear).
	for svc, imports := range serviceImports {
		g.AddNode(svc)
		for _, dep := range imports {
			g.AddNode(dep)
		}
	}

	// Add directed edges svc → imported-service.
	for svc, imports := range serviceImports {
		// Deduplicate imports before adding to avoid redundant edges.
		seen := make(map[string]bool, len(imports))
		for _, dep := range imports {
			if seen[dep] || dep == svc {
				// self-imports are legitimate cycles but depgraph handles them;
				// add the self-edge so the SCC detector can find it.
				if dep == svc {
					g.AddEdge(svc, svc)
				}
				seen[dep] = true
				continue
			}
			seen[dep] = true
			g.AddEdge(svc, dep)
		}
	}

	raw := g.Cycles()
	if len(raw) == 0 {
		return [][]string{}
	}

	// depgraph.Cycles already returns sorted inner slices and a sorted outer
	// slice, but we normalise here to be defensive.
	result := make([][]string, len(raw))
	for i, c := range raw {
		cp := make([]string, len(c))
		copy(cp, c)
		sort.Strings(cp)
		result[i] = cp
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i][0] < result[j][0]
	})
	return result
}
