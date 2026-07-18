// Package workspacematerialization computes the minimal transitive dependency
// set required to materialize a target node from a live-graph topology.
//
// Given a Graph (from the live-graph package) and a target node ID, Materialize
// returns the sorted, deduplicated set of all nodes that must be present in
// order to deploy the target — i.e. the transitive closure of the target's
// dependencies, including the target itself.
//
// No third-party dependencies are imported.
//
// Concurrency: Materialize is a pure function and safe for concurrent use; callers are responsible for the concurrency safety of the Graph they supply.
package workspacematerialization

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Graph is the minimal read interface that workspacematerialization requires
// from a live-graph topology. It is satisfied by *livegraph.Graph and by the
// test doubles in this package.
type Graph interface {
	// Nodes returns the sorted list of all node IDs in the graph.
	Nodes() []string
	// Predecessors returns the sorted list of node IDs that have a direct
	// edge pointing to id (i.e. direct dependencies of id).
	Predecessors(id string) []string
}

// Materialize returns the minimal set of node IDs that must be materialized
// to satisfy target, including target itself. The result is sorted
// lexicographically and contains no duplicates.
//
// The function performs a breadth-first traversal from target following
// predecessor (dependency) edges. It returns an error if target is empty or if
// target does not exist in the graph.
func Materialize(g Graph, target string) ([]string, error) {
	if strings.TrimSpace(target) == "" {
		return nil, errors.New("workspacematerialization: target must be non-empty")
	}

	// Check target exists.
	found := false
	for _, id := range g.Nodes() {
		if id == target {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("workspacematerialization: target %q not found in graph", target)
	}

	// BFS over predecessor edges.
	visited := make(map[string]struct{})
	queue := []string{target}
	visited[target] = struct{}{}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, pred := range g.Predecessors(cur) {
			if _, seen := visited[pred]; !seen {
				visited[pred] = struct{}{}
				queue = append(queue, pred)
			}
		}
	}

	result := make([]string, 0, len(visited))
	for id := range visited {
		result = append(result, id)
	}
	sort.Strings(result)
	return result, nil
}
