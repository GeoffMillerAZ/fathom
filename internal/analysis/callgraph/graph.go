// Package callgraph builds a directed call graph from []core.SymbolNode and
// provides pure, deterministic impact analyses on top of it.
//
// All public analysis functions return sorted, deterministic output so results
// are safe to compare in tests and stable across repeated calls.
//
// Tolerant matching: callee entries in SymbolNode.Calls may be stored as a
// full SymbolKey ("svc::pkg.Name") or as a bare function name ("Name").  Build
// resolves each callee to the correct node when the match is unambiguous.
package callgraph

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// Graph is a directed call graph.  Nodes are identified by their canonical
// SymbolKey; edges point from caller to callee.
//
// The zero value is not useful; always obtain a Graph via Build.
type Graph struct {
	// nodes maps canonical key -> SymbolNode (for membership tests).
	nodes map[string]core.SymbolNode

	// callees maps caller key -> sorted slice of callee keys.
	callees map[string][]string

	// callers maps callee key -> sorted slice of caller keys (reverse index).
	callers map[string][]string

	// byName maps bare function name -> canonical key, used for tolerant
	// matching.  Only populated when the bare name is unambiguous (exactly
	// one node with that Name).
	byName map[string]string

	// byPkgName maps "package.Name" -> canonical key. It resolves cross-package
	// selector calls ("pkg.Name", e.g. an external "pkg_test" package calling
	// "pkg.Name") precisely: the left side must be a real indexed package, so a
	// receiver-method call ("recv.Name") is not mistaken for a package selector.
	// Only populated when "package.Name" is unambiguous.
	byPkgName map[string]string
}

// NodeCount returns the number of nodes in the graph.
func (g *Graph) NodeCount() int { return len(g.nodes) }

// EdgeCount returns the total number of directed edges in the graph.
func (g *Graph) EdgeCount() int {
	n := 0
	for _, cs := range g.callees {
		n += len(cs)
	}
	return n
}

// ─── Build ───────────────────────────────────────────────────────────────────

// Build constructs a Graph from a slice of SymbolNodes.  The slice may be in
// any order.  Unknown callees (those not present in nodes) are silently dropped
// — they represent external dependencies.
func Build(nodes []core.SymbolNode) *Graph {
	g := &Graph{
		nodes:     make(map[string]core.SymbolNode, len(nodes)),
		callees:   make(map[string][]string, len(nodes)),
		callers:   make(map[string][]string, len(nodes)),
		byName:    make(map[string]string),
		byPkgName: make(map[string]string),
	}

	// First pass: register all nodes and build the byName / byPkgName indexes.
	// Count how many nodes share each bare name and each "package.Name"; keep
	// only the unambiguous ones.
	nameCounts := make(map[string]int, len(nodes))
	pkgNameCounts := make(map[string]int, len(nodes))
	for _, n := range nodes {
		key := n.Key()
		g.nodes[key] = n
		nameCounts[n.Name]++
		if n.Package != "" {
			pkgNameCounts[n.Package+"."+n.Name]++
		}
	}
	for _, n := range nodes {
		if nameCounts[n.Name] == 1 {
			g.byName[n.Name] = n.Key()
		}
		if n.Package != "" && pkgNameCounts[n.Package+"."+n.Name] == 1 {
			g.byPkgName[n.Package+"."+n.Name] = n.Key()
		}
	}

	// Second pass: resolve call edges.
	for _, n := range nodes {
		callerKey := n.Key()
		seen := make(map[string]struct{}) // deduplicate edges from this caller
		for _, raw := range n.Calls {
			calleeKey := g.resolve(raw)
			if calleeKey == "" {
				continue // unknown external symbol
			}
			if _, dup := seen[calleeKey]; dup {
				continue
			}
			seen[calleeKey] = struct{}{}
			g.callees[callerKey] = append(g.callees[callerKey], calleeKey)
			g.callers[calleeKey] = append(g.callers[calleeKey], callerKey)
		}
	}

	// Sort all adjacency lists for determinism.
	for k := range g.callees {
		sort.Strings(g.callees[k])
	}
	for k := range g.callers {
		sort.Strings(g.callers[k])
	}

	return g
}

// resolve converts a raw callee reference (full key or bare name) to a
// canonical key present in g.nodes.  Returns "" when unresolvable.
func (g *Graph) resolve(raw string) string {
	// Exact match (full SymbolKey already in nodes).
	if _, ok := g.nodes[raw]; ok {
		return raw
	}
	// Unambiguous bare-name lookup.
	if key, ok := g.byName[raw]; ok {
		return key
	}
	// Precise package-qualified lookup: raw "pkg.Name" -> "svc::pkg.Name".
	if key, ok := g.byPkgName[raw]; ok {
		return key
	}
	// Suffix match: raw might be "pkg.Name" and the key is "svc::pkg.Name".
	// Accept only if exactly one node matches.
	suffix := "." + raw
	var match string
	for key := range g.nodes {
		if strings.HasSuffix(key, suffix) {
			if match != "" {
				return "" // ambiguous
			}
			match = key
		}
	}
	return match
}

// ─── RenameImpact ────────────────────────────────────────────────────────────

// RenameImpact returns the sorted slice of canonical keys for all direct
// callers of symbol.  These are the call sites that would need updating if
// symbol were renamed.
//
// symbol may be a full SymbolKey or a bare name (tolerant matching applies).
func RenameImpact(g *Graph, symbol string) []string {
	key := g.canonicalize(symbol)
	if key == "" {
		return []string{}
	}
	cs := g.callers[key]
	if len(cs) == 0 {
		return []string{}
	}
	out := make([]string, len(cs))
	copy(out, cs)
	sort.Strings(out)
	return out
}

// ─── Reachable ───────────────────────────────────────────────────────────────

// Reachable reports whether there is any directed path from the node identified
// by from to the node identified by to (following caller→callee edges).
//
// A node trivially reaches itself.  Both from and to may be full keys or bare
// names.
func Reachable(g *Graph, from, to string) bool {
	fromKey := g.canonicalize(from)
	toKey := g.canonicalize(to)
	if fromKey == "" || toKey == "" {
		return false
	}
	if fromKey == toKey {
		return true
	}
	visited := make(map[string]bool)
	return bfsReachable(g, fromKey, toKey, visited)
}

func bfsReachable(g *Graph, from, to string, visited map[string]bool) bool {
	queue := []string{from}
	visited[from] = true
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range g.callees[cur] {
			if next == to {
				return true
			}
			if !visited[next] {
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}
	return false
}

// ─── CallPath ────────────────────────────────────────────────────────────────

// CallPath returns the shortest directed path (sequence of canonical keys)
// from from to to.  Returns nil when no path exists.  When from == to, returns
// a single-element slice.
//
// Among paths of equal length, the lexicographically smallest path is returned
// (deterministic BFS over sorted adjacency lists).
func CallPath(g *Graph, from, to string) []string {
	fromKey := g.canonicalize(from)
	toKey := g.canonicalize(to)
	if fromKey == "" || toKey == "" {
		return nil
	}
	if fromKey == toKey {
		return []string{fromKey}
	}

	// BFS with predecessor map to reconstruct path.
	prev := map[string]string{fromKey: ""}
	queue := []string{fromKey}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range g.callees[cur] {
			if _, seen := prev[next]; seen {
				continue
			}
			prev[next] = cur
			if next == toKey {
				return reconstructPath(prev, fromKey, toKey)
			}
			queue = append(queue, next)
		}
	}
	return nil
}

func reconstructPath(prev map[string]string, from, to string) []string {
	var path []string
	for cur := to; cur != ""; cur = prev[cur] {
		path = append(path, cur)
	}
	// Reverse.
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

// ─── TestImpact ──────────────────────────────────────────────────────────────

// TestImpact returns the sorted canonical keys of all test functions (nodes
// whose Name starts with "Test") that can transitively reach symbol via the
// call graph.
//
// symbol may be a full key or bare name.
func TestImpact(g *Graph, symbol string) []string {
	key := g.canonicalize(symbol)
	if key == "" {
		return []string{}
	}

	// Walk the reverse graph (callers) from symbol upward; collect test nodes.
	visited := make(map[string]bool)
	queue := []string{key}
	visited[key] = true
	var tests []string

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, caller := range g.callers[cur] {
			if visited[caller] {
				continue
			}
			visited[caller] = true
			node := g.nodes[caller]
			if strings.HasPrefix(node.Name, "Test") {
				tests = append(tests, caller)
			}
			queue = append(queue, caller)
		}
	}

	sort.Strings(tests)
	return tests
}

// ─── DeadCode ────────────────────────────────────────────────────────────────

// DeadCode returns the sorted canonical keys of all nodes that are both:
//   - not reachable from any of the provided entrypoints, AND
//   - have no callers in the graph (i.e., nothing in the graph calls them).
//
// Entrypoints may be full keys or bare names.  The result is sorted for
// determinism.
func DeadCode(g *Graph, entrypoints []string) []string {
	// Resolve entrypoints.
	epKeys := make([]string, 0, len(entrypoints))
	for _, ep := range entrypoints {
		if k := g.canonicalize(ep); k != "" {
			epKeys = append(epKeys, k)
		}
	}

	// BFS from all entrypoints to find reachable set.
	reachable := make(map[string]bool, len(g.nodes))
	queue := make([]string, 0, len(epKeys))
	for _, k := range epKeys {
		if !reachable[k] {
			reachable[k] = true
			queue = append(queue, k)
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range g.callees[cur] {
			if !reachable[next] {
				reachable[next] = true
				queue = append(queue, next)
			}
		}
	}

	// Collect nodes that are unreachable AND have no callers.
	var dead []string
	for key := range g.nodes {
		if reachable[key] {
			continue
		}
		if len(g.callers[key]) == 0 {
			dead = append(dead, key)
		}
	}
	sort.Strings(dead)
	return dead
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// canonicalize resolves symbol to the canonical key present in g.nodes.
// Accepts full keys or bare names.  Returns "" when unresolvable.
func (g *Graph) canonicalize(symbol string) string {
	if _, ok := g.nodes[symbol]; ok {
		return symbol
	}
	if key, ok := g.byName[symbol]; ok {
		return key
	}
	// Precise package-qualified lookup: "pkg.Name" -> "svc::pkg.Name".
	if key, ok := g.byPkgName[symbol]; ok {
		return key
	}
	// Try suffix match ".<symbol>" for dotted partial keys like "pkg.Name".
	suffix := "." + symbol
	var match string
	for key := range g.nodes {
		if strings.HasSuffix(key, suffix) {
			if match != "" {
				return "" // ambiguous
			}
			match = key
		}
	}
	return match
}
