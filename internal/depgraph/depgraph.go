// Package depgraph provides a generic directed graph over string nodes and
// utilities for dependency analysis: cycle detection, topological sort,
// transitive closure, and go.mod / import graph construction.
package depgraph

import (
	"errors"
	"sort"
)

// Graph is a directed graph whose nodes are strings.
// The zero value is not ready for use; allocate with NewGraph.
type Graph struct {
	// nodes tracks membership (true = present).
	nodes map[string]bool
	// edges[u] is the set of v such that there is an edge u→v.
	edges map[string]map[string]bool
	// revEdges[v] is the set of u such that there is an edge u→v.
	revEdges map[string]map[string]bool
}

// NewGraph allocates and returns an empty Graph.
func NewGraph() *Graph {
	return &Graph{
		nodes:    make(map[string]bool),
		edges:    make(map[string]map[string]bool),
		revEdges: make(map[string]map[string]bool),
	}
}

// AddNode registers n as a node. Idempotent.
func (g *Graph) AddNode(n string) {
	if !g.nodes[n] {
		g.nodes[n] = true
		if g.edges[n] == nil {
			g.edges[n] = make(map[string]bool)
		}
		if g.revEdges[n] == nil {
			g.revEdges[n] = make(map[string]bool)
		}
	}
}

// AddEdge adds a directed edge from → to, registering both endpoints as nodes.
// Idempotent.
func (g *Graph) AddEdge(from, to string) {
	g.AddNode(from)
	g.AddNode(to)
	g.edges[from][to] = true
	g.revEdges[to][from] = true
}

// Nodes returns a sorted slice of all node names.
func (g *Graph) Nodes() []string {
	out := make([]string, 0, len(g.nodes))
	for n := range g.nodes {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Deps returns the direct dependencies of n (nodes reachable in one hop from
// n). Returns nil if n is not in the graph, a sorted non-nil slice otherwise.
func (g *Graph) Deps(n string) []string {
	adj, ok := g.edges[n]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(adj))
	for v := range adj {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// Dependents returns the nodes that directly depend on n (reverse edges).
// Returns nil if n is not in the graph.
func (g *Graph) Dependents(n string) []string {
	rev, ok := g.revEdges[n]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(rev))
	for u := range rev {
		out = append(out, u)
	}
	sort.Strings(out)
	return out
}

// HasCycle returns true if the graph contains at least one cycle.
func (g *Graph) HasCycle() bool {
	return len(g.Cycles()) > 0
}

// Cycles returns the strongly-connected components (SCCs) that have size > 1
// or are self-loops (size == 1 with a self-edge).  Each SCC is returned as a
// sorted slice of node names; the outer slice is sorted by the first element of
// each SCC so the result is deterministic.
func (g *Graph) Cycles() [][]string {
	// Kosaraju's algorithm — two DFS passes.
	nodes := g.Nodes() // deterministic sorted order

	// Pass 1: fill finish order using the forward graph.
	visited := make(map[string]bool)
	finishStack := make([]string, 0, len(nodes))
	var dfs1 func(n string)
	dfs1 = func(n string) {
		visited[n] = true
		// iterate over sorted neighbours for determinism
		nbrs := g.Deps(n) // already sorted
		for _, v := range nbrs {
			if !visited[v] {
				dfs1(v)
			}
		}
		finishStack = append(finishStack, n)
	}
	for _, n := range nodes {
		if !visited[n] {
			dfs1(n)
		}
	}

	// Pass 2: DFS on the reverse graph in reverse-finish order.
	visited2 := make(map[string]bool)
	var sccs [][]string
	var dfs2 func(n string, comp *[]string)
	dfs2 = func(n string, comp *[]string) {
		visited2[n] = true
		*comp = append(*comp, n)
		// iterate over sorted reverse neighbours for determinism
		revNbrs := g.Dependents(n) // already sorted
		for _, u := range revNbrs {
			if !visited2[u] {
				dfs2(u, comp)
			}
		}
	}
	for i := len(finishStack) - 1; i >= 0; i-- {
		n := finishStack[i]
		if !visited2[n] {
			comp := []string{}
			dfs2(n, &comp)
			// Only report as a cycle if SCC has >1 node OR it is a self-loop.
			isCycle := len(comp) > 1
			if !isCycle && len(comp) == 1 {
				// self-loop?
				if g.edges[comp[0]] != nil && g.edges[comp[0]][comp[0]] {
					isCycle = true
				}
			}
			if isCycle {
				sort.Strings(comp)
				sccs = append(sccs, comp)
			}
		}
	}

	// Sort outer slice by first element for full determinism.
	sort.Slice(sccs, func(i, j int) bool {
		return sccs[i][0] < sccs[j][0]
	})
	return sccs
}

// TopoSort returns a topological ordering of all nodes such that for every
// edge u→v, u appears before v. Returns an error if the graph is cyclic.
// The order is deterministic: among nodes with equal in-degree, alphabetical
// order is used (Kahn's algorithm with a sorted-queue).
func (g *Graph) TopoSort() ([]string, error) {
	if g.HasCycle() {
		return nil, errors.New("depgraph: graph contains a cycle, topological sort not possible")
	}
	nodes := g.Nodes()

	// Compute in-degrees.
	inDeg := make(map[string]int, len(nodes))
	for _, n := range nodes {
		if _, ok := inDeg[n]; !ok {
			inDeg[n] = 0
		}
		for v := range g.edges[n] {
			inDeg[v]++
		}
	}

	// Collect roots (in-degree == 0).
	var queue []string
	for _, n := range nodes {
		if inDeg[n] == 0 {
			queue = append(queue, n)
		}
	}
	sort.Strings(queue)

	result := make([]string, 0, len(nodes))
	for len(queue) > 0 {
		// pop front
		n := queue[0]
		queue = queue[1:]
		result = append(result, n)

		// reduce in-degree of neighbours, collect new roots
		var newRoots []string
		for v := range g.edges[n] {
			inDeg[v]--
			if inDeg[v] == 0 {
				newRoots = append(newRoots, v)
			}
		}
		sort.Strings(newRoots)
		queue = append(queue, newRoots...)
		sort.Strings(queue) // keep sorted for determinism
	}

	return result, nil
}

// Transitive returns all nodes transitively reachable from n (i.e., the
// transitive closure of n's dependency relation), excluding n itself.
// The result is sorted. Returns nil if n is not in the graph.
func (g *Graph) Transitive(n string) []string {
	if !g.nodes[n] {
		return nil
	}
	visited := make(map[string]bool)
	var dfs func(cur string)
	dfs = func(cur string) {
		for v := range g.edges[cur] {
			if !visited[v] && v != n {
				visited[v] = true
				dfs(v)
			}
		}
	}
	dfs(n)

	out := make([]string, 0, len(visited))
	for v := range visited {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
