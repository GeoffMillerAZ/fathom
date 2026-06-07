// Package dataflow provides a call-edge based proxy for data-flow analysis.
//
// Trace performs a forward BFS over SymbolNode.Calls edges starting from the
// named symbol and returns the sorted set of all transitively reachable symbol
// keys (excluding the start node itself).
//
// All functions are pure, deterministic, and safe for concurrent use.
package dataflow

import (
	"sort"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// Trace returns the sorted slice of canonical symbol keys reachable from the
// symbol named by from via forward call edges (BFS).
//
// from is matched against node keys first (service::pkg.Name), then as a bare
// symbol name (unambiguous match only).  The start node itself is not included
// in the result.  Unknown callees (not present in nodes) are silently skipped.
//
// The returned slice is sorted and deterministic; nil input returns nil.
func Trace(nodes []core.SymbolNode, from string) []string {
	if len(nodes) == 0 {
		return nil
	}

	// Build index: key -> SymbolNode.
	byKey := make(map[string]core.SymbolNode, len(nodes))
	nameCounts := make(map[string]int, len(nodes))
	for _, n := range nodes {
		byKey[n.Key()] = n
		nameCounts[n.Name]++
	}

	// Build unambiguous bare-name index.
	byName := make(map[string]string, len(nodes))
	for _, n := range nodes {
		if nameCounts[n.Name] == 1 {
			byName[n.Name] = n.Key()
		}
	}

	// Resolve the start node.
	startKey := resolve(from, byKey, byName)
	if startKey == "" {
		return nil
	}

	// Forward BFS.
	visited := make(map[string]bool)
	visited[startKey] = true
	queue := []string{startKey}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		node, ok := byKey[cur]
		if !ok {
			continue
		}
		for _, rawCallee := range node.Calls {
			calleeKey := resolve(rawCallee, byKey, byName)
			if calleeKey == "" || visited[calleeKey] {
				continue
			}
			visited[calleeKey] = true
			queue = append(queue, calleeKey)
		}
	}

	// Collect reachable set (excluding start).
	delete(visited, startKey)
	result := make([]string, 0, len(visited))
	for k := range visited {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

// resolve converts a raw symbol reference to a canonical key present in byKey.
// Accepts full keys, bare names (unambiguous), or "pkg.Name" suffix matches.
func resolve(raw string, byKey map[string]core.SymbolNode, byName map[string]string) string {
	if _, ok := byKey[raw]; ok {
		return raw
	}
	if key, ok := byName[raw]; ok {
		return key
	}
	// Suffix match: raw is "pkg.Name"; accept only if exactly one node matches.
	suffix := "." + raw
	var match string
	for key := range byKey {
		if len(key) > len(suffix) && key[len(key)-len(suffix):] == suffix {
			if match != "" {
				return "" // ambiguous
			}
			match = key
		}
	}
	return match
}
