// Package impact performs blast-radius analysis over a call graph and a
// file→symbol index derived from []core.SymbolNode.
//
// All exported functions are pure, deterministic, and safe for concurrent use.
// Output slices are always sorted for stable results.
package impact

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// Result holds the output of a blast-radius analysis.
type Result struct {
	// Symbols are the canonical SymbolKeys of all symbols directly defined in
	// the changed files plus every symbol that transitively calls them.
	Symbols []string

	// Tests is the subset of Symbols whose names start with "Test".
	Tests []string

	// Files is the sorted set of unique file paths that contain any symbol in
	// Symbols.
	Files []string
}

// fileSymbols builds a map from file path → sorted slice of canonical keys for
// all symbols defined in that file.
func fileSymbols(nodes []core.SymbolNode) map[string][]string {
	m := make(map[string][]string)
	for _, n := range nodes {
		fp := n.FilePath
		m[fp] = append(m[fp], n.Key())
	}
	for fp := range m {
		sort.Strings(m[fp])
	}
	return m
}

// symbolFile builds the reverse map: canonical key → file path.
func symbolFile(nodes []core.SymbolNode) map[string]string {
	m := make(map[string]string, len(nodes))
	for _, n := range nodes {
		m[n.Key()] = n.FilePath
	}
	return m
}

// BlastRadius computes the full blast radius of the given changed files:
//
//  1. Collect all symbols whose FilePath appears in changedFiles (seed set).
//  2. Walk the reverse call graph transitively to gather all callers.
//  3. Identify test functions (Name prefix "Test") in the combined set.
//  4. Return sorted Symbols, Tests, and Files.
//
// changedFiles is matched against SymbolNode.FilePath by exact string equality;
// callers should normalise paths before passing them in.
func BlastRadius(nodes []core.SymbolNode, changedFiles []string) Result {
	return blastRadius(nodes, changedFiles, nil)
}

// BlastRadiusWithTypedEdges is BlastRadius plus the Go typed call edges
// (ADR-021 §3, internal/fusion.EnrichTypedEdges): the transitive-caller walk
// additionally follows EdgeDirect/EdgeIfaceDispatch edges, so a caller
// reachable only through an interface-dispatch edge (previously refused by
// the syntax-only resolver for an ambiguous method name) is now included in
// the blast radius. typed may be nil (identical to BlastRadius).
func BlastRadiusWithTypedEdges(nodes []core.SymbolNode, changedFiles []string, typed []core.Edge) Result {
	return blastRadius(nodes, changedFiles, typed)
}

func blastRadius(nodes []core.SymbolNode, changedFiles []string, typed []core.Edge) Result {
	if len(nodes) == 0 || len(changedFiles) == 0 {
		return Result{Symbols: []string{}, Tests: []string{}, Files: []string{}}
	}

	changed := make(map[string]bool, len(changedFiles))
	for _, f := range changedFiles {
		changed[f] = true
	}

	fs := fileSymbols(nodes)
	sf := symbolFile(nodes)

	// Seed: all symbols in the changed files.
	seed := map[string]bool{}
	for fp, keys := range fs {
		if changed[fp] {
			for _, k := range keys {
				seed[k] = true
			}
		}
	}

	if len(seed) == 0 {
		return Result{Symbols: []string{}, Tests: []string{}, Files: []string{}}
	}

	// Build call graph once, then walk callers transitively from seed.
	g := callgraph.Build(nodes)
	if len(typed) > 0 {
		callgraph.AddTypedEdges(g, typed)
	}
	return blastRadiusOn(g, nodes, seed, sf)
}

// BlastRadiusOn is BlastRadiusWithTypedEdges over a graph the caller
// already holds (the serving daemon's graph store, RT-74) instead of one
// built per call: g must be callgraph.BuildBoth(nodes, typed).
func BlastRadiusOn(g *callgraph.Graph, nodes []core.SymbolNode, changedFiles []string) Result {
	if g == nil || len(nodes) == 0 || len(changedFiles) == 0 {
		return Result{Symbols: []string{}, Tests: []string{}, Files: []string{}}
	}
	changed := make(map[string]bool, len(changedFiles))
	for _, f := range changedFiles {
		changed[f] = true
	}
	seed := map[string]bool{}
	for fp, keys := range fileSymbols(nodes) {
		if changed[fp] {
			for _, k := range keys {
				seed[k] = true
			}
		}
	}
	if len(seed) == 0 {
		return Result{Symbols: []string{}, Tests: []string{}, Files: []string{}}
	}
	return blastRadiusOn(g, nodes, seed, symbolFile(nodes))
}

func blastRadiusOn(g *callgraph.Graph, nodes []core.SymbolNode, seed map[string]bool, sf map[string]string) Result {
	// BFS over callers.
	visited := make(map[string]bool, len(seed))
	queue := make([]string, 0, len(seed))
	for k := range seed {
		visited[k] = true
		queue = append(queue, k)
	}

	// We need access to the callers relation; use callgraph.RenameImpact which
	// returns direct callers, and repeat until the frontier is empty.
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, caller := range callgraph.RenameImpact(g, cur) {
			if !visited[caller] {
				visited[caller] = true
				queue = append(queue, caller)
			}
		}
	}

	// Build node name map for test detection.
	nameOf := make(map[string]string, len(nodes))
	for _, n := range nodes {
		nameOf[n.Key()] = n.Name
	}

	// Collect results.
	syms := make([]string, 0, len(visited))
	tests := []string{}
	fileSet := map[string]bool{}

	for k := range visited {
		syms = append(syms, k)
		if strings.HasPrefix(nameOf[k], "Test") {
			tests = append(tests, k)
		}
		if fp, ok := sf[k]; ok {
			fileSet[fp] = true
		}
	}

	sort.Strings(syms)
	sort.Strings(tests)

	files := make([]string, 0, len(fileSet))
	for fp := range fileSet {
		files = append(files, fp)
	}
	sort.Strings(files)

	return Result{Symbols: syms, Tests: tests, Files: files}
}

// DiffImpact is an alias for BlastRadius that emphasises its use in diff/PR
// workflows.  The semantics are identical.
func DiffImpact(nodes []core.SymbolNode, changedFiles []string) Result {
	return BlastRadius(nodes, changedFiles)
}

// DiffImpactWithTypedEdges is an alias for BlastRadiusWithTypedEdges (ADR-021 §3).
func DiffImpactWithTypedEdges(nodes []core.SymbolNode, changedFiles []string, typed []core.Edge) Result {
	return BlastRadiusWithTypedEdges(nodes, changedFiles, typed)
}

// Reachable reports whether any path exists in the call graph from any symbol
// in changedFiles to the target symbol (full key or bare name).
//
// It returns true when at least one symbol in changedFiles can transitively
// reach target via caller→callee edges.
func Reachable(nodes []core.SymbolNode, changedFiles []string, target string) bool {
	if len(nodes) == 0 {
		return false
	}
	g := callgraph.Build(nodes)
	changed := make(map[string]bool, len(changedFiles))
	for _, f := range changedFiles {
		changed[f] = true
	}
	for _, n := range nodes {
		if changed[n.FilePath] {
			if callgraph.Reachable(g, n.Key(), target) {
				return true
			}
		}
	}
	return false
}

// AffectedTests returns the sorted canonical keys of test functions (Name
// prefix "Test") that appear in the blast radius of changedFiles.
func AffectedTests(nodes []core.SymbolNode, changedFiles []string) []string {
	r := BlastRadius(nodes, changedFiles)
	return r.Tests
}
