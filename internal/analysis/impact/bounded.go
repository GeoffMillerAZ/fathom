package impact

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// Options bound a blast-radius walk (bench-pyramid BP-27).
//
// The unbounded, both-kinds walk BlastRadius/BlastRadiusWithTypedEdges
// performs is NOT deprecated and is not changed by this file: the verify
// engine's reachability reads it, and a reachability question whose answer
// shrinks because a display default changed would be a silent correctness
// regression. Options exist for the agent-facing tools, where an answer of
// 9,978 symbols at precision 0.075 is worse than no answer at all.
type Options struct {
	// MaxDepth stops the reverse walk that many hops from the changed
	// files. Zero means unbounded. Depth 1 is "direct callers"; the
	// blast_radius tool defaults to 2.
	MaxDepth int
	// MaxSymbols caps how many symbols are RETURNED. The walk still runs to
	// completion so Truncated and TotalSymbols are honest; only the listing
	// is cut, shallowest-first then by key, so the cap never hides a direct
	// caller in order to show a distant one.
	MaxSymbols int
}

// BoundedResult is Result plus what the bounds did to it. A truncated answer
// that does not say so is a lie by omission, so every field a cap touched
// has a counterpart here.
type BoundedResult struct {
	Result
	// TotalSymbols is how many symbols the walk reached before MaxSymbols
	// was applied. Equal to len(Symbols) when nothing was cut.
	TotalSymbols int
	// Truncated is true when MaxSymbols cut the listing.
	Truncated bool
	// MaxDepthReached is the greatest depth any returned symbol sits at.
	MaxDepthReached int
	// DepthCapped is true when the walk stopped at MaxDepth with callers
	// still unexplored — "there is more radius beyond this", said out loud.
	DepthCapped bool
	// EdgeFilter is "resolved" when the walk followed only typed-pass edges,
	// "all" when it followed syntax edges too.
	EdgeFilter string
	// References lists, sorted, the returned symbols the walk pulled in
	// through a reference to a Go type rather than a call (wave lg,
	// LB-F200: a syntax-only edge to a type, core.EdgeTypeRef): a type's
	// consumers stay in the radius, labelled as what they are. Empty on a
	// resolved-only walk, whose edges are all calls.
	References []string
}

// BlastRadiusBounded walks the reverse call graph from the symbols defined
// in changedFiles, following the edges of g, under opts.
//
// g decides the evidence: pass callgraph.BuildResolved(nodes, typed) for a
// resolved-only answer, callgraph.BuildBoth(nodes, typed) to include the
// syntax-derived edges. Passing the graph in (rather than nodes + typed) is
// what lets a caller hand in a memoized graph — the whole point of
// callgraph.CachedCallGraph, since building it was ~9.7s of every query.
func BlastRadiusBounded(g *callgraph.Graph, nodes []core.SymbolNode, changedFiles []string, opts Options) BoundedResult {
	out := BoundedResult{
		Result:     Result{Symbols: []string{}, Tests: []string{}, Files: []string{}},
		EdgeFilter: "all",
		References: []string{},
	}
	if g == nil || len(nodes) == 0 || len(changedFiles) == 0 {
		return out
	}
	changed := make(map[string]bool, len(changedFiles))
	for _, f := range changedFiles {
		changed[f] = true
	}

	depth := map[string]int{}
	viaRef := map[string]bool{} // first reached through a reference to a Go type
	var queue []string
	for _, n := range nodes {
		if !changed[n.FilePath] {
			continue
		}
		k := n.Key()
		if _, ok := depth[k]; !ok {
			depth[k] = 0
			queue = append(queue, k)
		}
	}
	if len(queue) == 0 {
		return out
	}
	sort.Strings(queue)

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		d := depth[cur]
		if opts.MaxDepth > 0 && d >= opts.MaxDepth {
			// Only report a cap when there was actually something past it.
			for _, caller := range callgraph.RenameImpact(g, cur) {
				if _, seen := depth[caller]; !seen {
					out.DepthCapped = true
					break
				}
			}
			continue
		}
		// DependentsDetail is RenameImpact's caller set, in the same order,
		// with each edge's kind: a reference to a Go type is walked like a
		// call (a type's consumers break when it changes) and marked.
		for _, dep := range callgraph.DependentsDetail(g, cur) {
			if _, seen := depth[dep.Key]; seen {
				continue
			}
			depth[dep.Key] = d + 1
			if dep.Kind == core.EdgeTypeRef {
				viaRef[dep.Key] = true
			}
			queue = append(queue, dep.Key)
		}
	}

	nameOf := make(map[string]string, len(nodes))
	fileOf := make(map[string]string, len(nodes))
	for _, n := range nodes {
		nameOf[n.Key()] = n.Name
		fileOf[n.Key()] = n.FilePath
	}

	keys := make([]string, 0, len(depth))
	for k, d := range depth {
		keys = append(keys, k)
		if d > out.MaxDepthReached {
			out.MaxDepthReached = d
		}
	}
	out.TotalSymbols = len(keys)
	// Shallowest first, then by key: a cap must never drop a direct caller
	// to make room for a transitive one.
	sort.Slice(keys, func(i, j int) bool {
		if depth[keys[i]] != depth[keys[j]] {
			return depth[keys[i]] < depth[keys[j]]
		}
		return keys[i] < keys[j]
	})
	if opts.MaxSymbols > 0 && len(keys) > opts.MaxSymbols {
		keys = keys[:opts.MaxSymbols]
		out.Truncated = true
		out.MaxDepthReached = 0
		for _, k := range keys {
			if depth[k] > out.MaxDepthReached {
				out.MaxDepthReached = depth[k]
			}
		}
	}

	syms := append([]string(nil), keys...)
	sort.Strings(syms)
	for _, k := range syms {
		if viaRef[k] {
			out.References = append(out.References, k)
		}
	}
	tests := []string{}
	fileSet := map[string]bool{}
	for _, k := range syms {
		if strings.HasPrefix(nameOf[k], "Test") {
			tests = append(tests, k)
		}
		if fp, ok := fileOf[k]; ok {
			fileSet[fp] = true
		}
	}
	files := make([]string, 0, len(fileSet))
	for fp := range fileSet {
		files = append(files, fp)
	}
	sort.Strings(files)

	out.Result = Result{Symbols: syms, Tests: tests, Files: files}
	return out
}
