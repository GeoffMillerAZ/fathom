// Package livegraph provides a deployment/dependency graph with cycle
// detection, critical-path analysis, and wave-based rollout planning.
// It is a pure planner: it computes waves and critical paths but does NOT
// execute rollouts or spawn any work.
//
// Plan-only status (D7): this package ships only as the wave-planner half of
// the future kits/worker-pool. The executor (kits/worker-pool with
// agent-harness as the worker unit) is not yet built; it will be added when a
// real fan-out workload subscribes.
//
// CUE is authoritative for the valid value space (schema.cue); Go is
// authoritative for runtime behaviour. No third-party dependencies are
// imported.
//
// Invariants:
//   - Node IDs must be non-empty.
//   - Duplicate node IDs are silently ignored (AddNode is idempotent).
//   - AddEdge returns an error for unknown node IDs.
//   - CriticalPath and Waves reject graphs containing cycles.
//   - All operations that return slices are deterministic (sorted).
//
// Concurrency: Graph is not safe for concurrent use; guard with your own synchronisation.
package livegraph

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Node is a single vertex in the deployment/dependency graph. ID must be
// non-empty.
type Node struct {
	// ID uniquely identifies this node in the graph.
	ID string
}

// Edge is a directed dependency from From to To, meaning "To depends on From"
// (From must be deployed before To).
type Edge struct {
	// From is the dependency (must be deployed first).
	From string
	// To is the dependent (deployed after From).
	To string
}

// Graph is an in-memory directed graph of Node and Edge values. The zero
// value is ready to use.
type Graph struct {
	nodes map[string]struct{}
	// adj[from] = set of "to" nodes (forward adjacency — from → to)
	adj map[string]map[string]struct{}
}

// AddNode registers a node in the graph. If a node with the same ID already
// exists the call is a no-op. It returns an error when the node ID is empty.
func (g *Graph) AddNode(n Node) error {
	if strings.TrimSpace(n.ID) == "" {
		return errors.New("livegraph: node ID must be non-empty")
	}
	if g.nodes == nil {
		g.nodes = make(map[string]struct{})
		g.adj = make(map[string]map[string]struct{})
	}
	if _, exists := g.nodes[n.ID]; !exists {
		g.nodes[n.ID] = struct{}{}
		g.adj[n.ID] = make(map[string]struct{})
	}
	return nil
}

// AddEdge adds a directed edge From→To. Both node IDs must already be present
// in the graph. Duplicate edges are silently ignored.
func (g *Graph) AddEdge(e Edge) error {
	if g.nodes == nil {
		return fmt.Errorf("livegraph: node %q not found", e.From)
	}
	if _, ok := g.nodes[e.From]; !ok {
		return fmt.Errorf("livegraph: node %q not found", e.From)
	}
	if _, ok := g.nodes[e.To]; !ok {
		return fmt.Errorf("livegraph: node %q not found", e.To)
	}
	g.adj[e.From][e.To] = struct{}{}
	return nil
}

// Nodes returns a sorted list of all node IDs currently in the graph.
func (g *Graph) Nodes() []string {
	out := make([]string, 0, len(g.nodes))
	for id := range g.nodes {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Predecessors returns the sorted list of node IDs that have a direct edge
// pointing to id (i.e. the direct dependencies of id). If id is not in the
// graph or has no predecessors, an empty slice is returned.
func (g *Graph) Predecessors(id string) []string {
	if g.adj == nil {
		return []string{}
	}
	out := make([]string, 0)
	for from, tos := range g.adj {
		if _, ok := tos[id]; ok {
			out = append(out, from)
		}
	}
	sort.Strings(out)
	return out
}

// Cycles reports whether the graph contains at least one directed cycle.
// It uses an iterative DFS with three-colour marking (white/grey/black).
func (g *Graph) Cycles() bool {
	// colour: 0=white (unvisited), 1=grey (in-stack), 2=black (done)
	colour := make(map[string]int, len(g.nodes))

	var hasCycle bool

	// Deterministic traversal: sort nodes.
	ids := g.Nodes()

	var dfs func(id string)
	dfs = func(id string) {
		if hasCycle {
			return
		}
		colour[id] = 1 // grey — on current DFS stack
		neighbours := sortedKeys(g.adj[id])
		for _, nb := range neighbours {
			if colour[nb] == 1 {
				hasCycle = true
				return
			}
			if colour[nb] == 0 {
				dfs(nb)
			}
		}
		colour[id] = 2 // black — fully processed
	}

	for _, id := range ids {
		if colour[id] == 0 {
			dfs(id)
		}
	}
	return hasCycle
}

// CriticalPath returns the longest dependency chain (most nodes) in the graph
// as an ordered slice of node IDs, from the earliest dependency to the final
// target. If multiple paths share the maximum length the lexicographically
// first one is returned (deterministic). Returns an error if the graph
// contains a cycle.
func (g *Graph) CriticalPath() ([]string, error) {
	if g.Cycles() {
		return nil, errors.New("livegraph: CriticalPath requires a DAG (cycle detected)")
	}
	if len(g.nodes) == 0 {
		return []string{}, nil
	}

	// For each node compute the length (in nodes) of the longest path ending
	// at that node using a topological sort + DP.
	// dp[id] = (length, predecessor) for the longest path ending at id.
	type entry struct {
		length int
		prev   string // "" means no predecessor
	}
	dp := make(map[string]entry, len(g.nodes))
	for id := range g.nodes {
		dp[id] = entry{length: 1, prev: ""}
	}

	// Topological order via Kahn's algorithm (deterministic: use sorted queues).
	inDegree := make(map[string]int, len(g.nodes))
	for id := range g.nodes {
		inDegree[id] = 0
	}
	for from, tos := range g.adj {
		_ = from
		for to := range tos {
			inDegree[to]++
		}
	}

	// Collect zero-indegree nodes, sorted for determinism.
	queue := make([]string, 0, len(g.nodes))
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)

	topoOrder := make([]string, 0, len(g.nodes))
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		topoOrder = append(topoOrder, cur)
		neighbours := sortedKeys(g.adj[cur])
		for _, nb := range neighbours {
			inDegree[nb]--
			if inDegree[nb] == 0 {
				// Insert in sorted order to maintain determinism.
				queue = insertSorted(queue, nb)
			}
		}
	}

	// DP in topological order.
	for _, cur := range topoOrder {
		neighbours := sortedKeys(g.adj[cur])
		for _, nb := range neighbours {
			candidate := dp[cur].length + 1
			if candidate > dp[nb].length ||
				(candidate == dp[nb].length && cur < dp[nb].prev) {
				dp[nb] = entry{length: candidate, prev: cur}
			}
		}
	}

	// Find the node with the maximum path length; break ties lexicographically.
	best := ""
	for _, id := range g.Nodes() {
		if best == "" ||
			dp[id].length > dp[best].length ||
			(dp[id].length == dp[best].length && id < best) {
			best = id
		}
	}

	// Reconstruct path by walking backwards through predecessors.
	path := []string{}
	cur := best
	for cur != "" {
		path = append(path, cur)
		cur = dp[cur].prev
	}
	// Reverse to get dependency → target order.
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path, nil
}

// Waves returns the topological levels (wave/level planning) of the graph as
// a slice of sorted string slices. Wave 0 contains nodes with no dependencies;
// each subsequent wave contains nodes whose dependencies have all been
// satisfied by prior waves. Returns an error if the graph contains a cycle.
//
// The ordering within each wave is deterministic (sorted). This gives safe
// rollout order: deploy wave[0] first, then wave[1], and so on.
func (g *Graph) Waves() ([][]string, error) {
	if g.Cycles() {
		return nil, errors.New("livegraph: Waves requires a DAG (cycle detected)")
	}
	if len(g.nodes) == 0 {
		return [][]string{}, nil
	}

	// Kahn's algorithm, collecting nodes into levels.
	inDegree := make(map[string]int, len(g.nodes))
	for id := range g.nodes {
		inDegree[id] = 0
	}
	for _, tos := range g.adj {
		for to := range tos {
			inDegree[to]++
		}
	}

	current := make([]string, 0)
	for id, deg := range inDegree {
		if deg == 0 {
			current = append(current, id)
		}
	}
	sort.Strings(current)

	var waves [][]string
	processed := make(map[string]struct{})

	for len(current) > 0 {
		waves = append(waves, append([]string(nil), current...))
		next := make([]string, 0)
		for _, cur := range current {
			processed[cur] = struct{}{}
			for nb := range g.adj[cur] {
				inDegree[nb]--
				if inDegree[nb] == 0 {
					next = append(next, nb)
				}
			}
		}
		sort.Strings(next)
		current = next
	}

	_ = processed
	return waves, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func insertSorted(s []string, val string) []string {
	i := sort.SearchStrings(s, val)
	s = append(s, "")
	copy(s[i+1:], s[i:])
	s[i] = val
	return s
}
