// Package graph provides a typed edge graph and BFS traversal for sextant.
// All functions are pure, deterministic, and require no I/O.
package graph

import "sort"

// Kind constants for edge types.
const (
	KindCalls   = "calls"
	KindImports = "imports"
	KindSpecRef = "spec-ref" // file → spec_id
	KindRoute   = "route"    // route → handler
	KindEnvKey  = "env-key"  // file → env key name
)

// Edge is a single typed edge in the graph.
type Edge struct {
	From    string
	To      string
	Kind    string
	Service string
	File    string
	Line    int
}

// Path is an ordered sequence of edges forming a walk.
type Path struct {
	Edges []Edge
}

// TraverseResult is returned by Traverse.
type TraverseResult struct {
	Paths      []Path
	Truncated  bool
	EdgeCounts map[string]int
}

const maxPaths = 200

// Traverse performs BFS from start over edges up to depth hops in direction.
// direction: "out" (default), "in", or "both".
// kinds: filter to these edge kinds; nil/empty means all.
// Returns at most 200 paths; Truncated=true when capped.
// Results are deterministic.
func Traverse(edges []Edge, start string, kinds []string, depth int, direction string) TraverseResult {
	if depth <= 0 {
		depth = 3
	}
	if direction == "" {
		direction = "out"
	}

	kindSet := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		kindSet[k] = true
	}
	filterKind := func(k string) bool {
		return len(kindSet) == 0 || kindSet[k]
	}

	// Count all edges per kind (before kind filtering, for informational edge_counts).
	edgeCounts := make(map[string]int)
	for _, e := range edges {
		edgeCounts[e.Kind]++
	}

	// Build adjacency lists.
	type adj struct {
		edge Edge
		next string
	}
	outAdj := make(map[string][]adj)
	inAdj := make(map[string][]adj)
	for _, e := range edges {
		if !filterKind(e.Kind) {
			continue
		}
		outAdj[e.From] = append(outAdj[e.From], adj{e, e.To})
		inAdj[e.To] = append(inAdj[e.To], adj{e, e.From})
	}
	// Sort adjacency for determinism.
	for _, list := range []map[string][]adj{outAdj, inAdj} {
		for k := range list {
			l := list[k]
			sort.Slice(l, func(i, j int) bool {
				if l[i].edge.Kind != l[j].edge.Kind {
					return l[i].edge.Kind < l[j].edge.Kind
				}
				return l[i].next < l[j].next
			})
			list[k] = l
		}
	}

	nextFn := func(node string) []adj {
		switch direction {
		case "in":
			return inAdj[node]
		case "both":
			out := make([]adj, 0, len(outAdj[node])+len(inAdj[node]))
			out = append(out, outAdj[node]...)
			out = append(out, inAdj[node]...)
			sort.Slice(out, func(i, j int) bool {
				if out[i].edge.Kind != out[j].edge.Kind {
					return out[i].edge.Kind < out[j].edge.Kind
				}
				return out[i].next < out[j].next
			})
			return out
		default: // "out"
			return outAdj[node]
		}
	}

	type qitem struct {
		path    []Edge
		tail    string
		visited map[string]bool
	}

	var paths []Path
	truncated := false

	// Resolve start node: check if it exists as a From or To in any edge.
	// (If not, the BFS will just produce no paths — that's OK.)
	startNode := start

	queue := []qitem{{
		path:    nil,
		tail:    startNode,
		visited: map[string]bool{startNode: true},
	}}

	for d := 0; d < depth && len(queue) > 0; d++ {
		var next []qitem
		for _, item := range queue {
			for _, a := range nextFn(item.tail) {
				if item.visited[a.next] {
					continue
				}
				newEdges := make([]Edge, len(item.path)+1)
				copy(newEdges, item.path)
				newEdges[len(item.path)] = a.edge

				newVis := make(map[string]bool, len(item.visited)+1)
				for k, v := range item.visited {
					newVis[k] = v
				}
				newVis[a.next] = true

				if len(paths) >= maxPaths {
					truncated = true
					goto done
				}
				paths = append(paths, Path{Edges: newEdges})
				next = append(next, qitem{
					path:    newEdges,
					tail:    a.next,
					visited: newVis,
				})
			}
		}
		queue = next
	}
done:
	return TraverseResult{
		Paths:      paths,
		Truncated:  truncated,
		EdgeCounts: edgeCounts,
	}
}
