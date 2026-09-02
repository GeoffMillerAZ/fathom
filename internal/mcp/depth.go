// Package mcp — depth buckets and edge confidence for the blast-radius
// family (docs/vision/26-proof-credibility.md §3, docs/plans/blast-depth.md
// BD-2/BD-3).
//
// The facts were always in the index — typed edges vs syntax-only edges,
// the reverse call graph — and the tools did not say them. These helpers
// add ADDITIVE response fields: `depth` and `edge_confidence` per row, a
// `by_depth` summary per response. No tool is added or renamed (law 4).
package mcp

import (
	"sort"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// depthInfo is one row's annotation: its reverse-graph distance from the
// target and the confidence of the edge it was reached through.
type depthInfo struct {
	Depth      int
	Confidence core.EdgeConfidence
}

// depthBucket is one `by_depth` summary row.
type depthBucket struct {
	Depth     int `json:"depth"`
	Count     int `json:"count"`
	Resolved  int `json:"resolved"`
	Heuristic int `json:"heuristic"`
}

// withDepthBuckets folds per-row annotations into the sorted `by_depth`
// summary (one bucket per distinct depth, ascending; counts split by
// confidence). Always a non-nil slice so the field marshals as `[]`, never
// `null`.
func withDepthBuckets(infos []depthInfo) []depthBucket {
	byDepth := map[int]*depthBucket{}
	for _, d := range infos {
		b := byDepth[d.Depth]
		if b == nil {
			b = &depthBucket{Depth: d.Depth}
			byDepth[d.Depth] = b
		}
		b.Count++
		switch d.Confidence {
		case core.EdgeConfidenceResolved:
			b.Resolved++
		case core.EdgeConfidenceHeuristic:
			b.Heuristic++
		}
	}
	out := make([]depthBucket, 0, len(byDepth))
	for _, b := range byDepth {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Depth < out[j].Depth })
	return out
}

// edgeConfidence reads the confidence of the (caller → callee) edge in g:
// resolved when the typed pass confirmed that exact pair, heuristic for a
// syntax-only edge (callgraph.EdgeInfo.Kind == "").
func edgeConfidence(g *callgraph.Graph, caller, callee string) core.EdgeConfidence {
	for _, d := range callgraph.CallersDetail(g, callee) {
		if d.Key == caller {
			return core.EdgeConfidenceOf(core.Edge{FromSymbol: caller, ToSymbol: callee, Kind: d.Kind})
		}
	}
	return core.EdgeConfidenceHeuristic
}

// callerDepths walks the reverse call graph from symbol (full key or bare
// name) breadth-first and returns every transitive caller keyed by
// canonical key, with its shortest-path depth (direct caller = 1) and the
// confidence of the edge it was first reached through. maxDepth 0 means
// unbounded. Deterministic: callgraph keeps every adjacency list sorted, so
// BFS discovery order — and therefore which edge "reached" a caller first —
// is stable across runs. Returns nil when symbol does not resolve.
func callerDepths(g *callgraph.Graph, symbol string, maxDepth int) map[string]depthInfo {
	direct := callgraph.CallersDetail(g, symbol)
	if direct == nil {
		return nil
	}
	out := map[string]depthInfo{}
	type item struct {
		key   string
		depth int
	}
	var queue []item
	for _, d := range direct {
		if _, seen := out[d.Key]; seen {
			continue
		}
		out[d.Key] = depthInfo{Depth: 1, Confidence: core.EdgeConfidenceOf(core.Edge{Kind: d.Kind})}
		queue = append(queue, item{d.Key, 1})
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if maxDepth > 0 && cur.depth >= maxDepth {
			continue
		}
		for _, d := range callgraph.CallersDetail(g, cur.key) {
			if _, seen := out[d.Key]; seen {
				continue
			}
			out[d.Key] = depthInfo{Depth: cur.depth + 1, Confidence: core.EdgeConfidenceOf(core.Edge{Kind: d.Kind})}
			queue = append(queue, item{d.Key, cur.depth + 1})
		}
	}
	return out
}

// depthRow is one `transitive_callers` / by-depth symbol entry.
type depthRow struct {
	Key            string `json:"key"`
	Depth          int    `json:"depth"`
	EdgeConfidence string `json:"edge_confidence,omitempty"`
}

// sortedDepthRows renders infos as rows ordered by depth, then key.
func sortedDepthRows(infos map[string]depthInfo) []depthRow {
	rows := make([]depthRow, 0, len(infos))
	for k, d := range infos {
		rows = append(rows, depthRow{Key: k, Depth: d.Depth, EdgeConfidence: string(d.Confidence)})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Depth != rows[j].Depth {
			return rows[i].Depth < rows[j].Depth
		}
		return rows[i].Key < rows[j].Key
	})
	return rows
}

// blastBucket is one `by_depth` entry of blast_radius / diff_impact: the
// symbols at that distance from the changed files, each with the
// confidence of the edge that pulled it in (absent at depth 0 — the changed
// symbols themselves are index facts, not edges).
type blastBucket struct {
	Depth     int        `json:"depth"`
	Count     int        `json:"count"`
	Resolved  int        `json:"resolved"`
	Heuristic int        `json:"heuristic"`
	Symbols   []depthRow `json:"symbols"`
}

// blastRadiusByDepth groups a blast radius by distance from the changed
// files: depth 0 is every symbol defined in a changed file, depth n+1 the
// callers of depth n first reached there, each labeled resolved/heuristic
// from the edge's own provenance (BD-3: "depth 1 WILL BREAK / depth 2
// LIKELY AFFECTED" needs the depth AND the confidence to be honest). The
// walk follows the same graph impact.BlastRadiusWithTypedEdges walks
// (syntax edges plus typed edges), so the union of the buckets equals that
// result's Symbols. Buckets are sorted by depth, symbols by key; the result
// is never nil.
func blastRadiusByDepth(nodes []core.SymbolNode, changedFiles []string, typed []core.Edge) []blastBucket {
	changed := make(map[string]bool, len(changedFiles))
	for _, f := range changedFiles {
		changed[f] = true
	}
	g := callgraph.Build(nodes)
	callgraph.AddTypedEdges(g, typed)

	seen := map[string]depthInfo{}
	var queue []string
	for _, n := range nodes {
		if changed[n.FilePath] {
			k := n.Key()
			if _, ok := seen[k]; !ok {
				seen[k] = depthInfo{Depth: 0}
				queue = append(queue, k)
			}
		}
	}
	sort.Strings(queue)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, d := range callgraph.CallersDetail(g, cur) {
			if _, ok := seen[d.Key]; ok {
				continue
			}
			seen[d.Key] = depthInfo{Depth: seen[cur].Depth + 1, Confidence: core.EdgeConfidenceOf(core.Edge{Kind: d.Kind})}
			queue = append(queue, d.Key)
		}
	}

	byDepth := map[int]*blastBucket{}
	for k, d := range seen {
		b := byDepth[d.Depth]
		if b == nil {
			b = &blastBucket{Depth: d.Depth, Symbols: []depthRow{}}
			byDepth[d.Depth] = b
		}
		b.Count++
		row := depthRow{Key: k, Depth: d.Depth}
		if d.Depth > 0 {
			row.EdgeConfidence = string(d.Confidence)
			switch d.Confidence {
			case core.EdgeConfidenceResolved:
				b.Resolved++
			case core.EdgeConfidenceHeuristic:
				b.Heuristic++
			}
		}
		b.Symbols = append(b.Symbols, row)
	}
	out := make([]blastBucket, 0, len(byDepth))
	for _, b := range byDepth {
		sort.Slice(b.Symbols, func(i, j int) bool { return b.Symbols[i].Key < b.Symbols[j].Key })
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Depth < out[j].Depth })
	return out
}

// mentionConfidence returns a lookup that classifies a text mention at
// (file, line) for the references tool (BD-2): resolved when the symbol
// enclosing that line has a typed call edge to any of defKeys — the
// query's definitions in this service — heuristic otherwise. The enclosing
// symbol is the last indexed symbol in the file that starts at or before
// the line (most Go definitions carry no end_line, so "last start before"
// is the honest approximation). Deterministic: per-file symbol lists are
// sorted by line, then key.
func mentionConfidence(nodes []core.SymbolNode, typed []core.Edge, defKeys []string) func(file string, line int) core.EdgeConfidence {
	g := callgraph.Build(nodes)
	callgraph.AddTypedEdges(g, typed)
	byFile := map[string][]core.SymbolNode{}
	for _, n := range nodes {
		if n.Line > 0 {
			byFile[n.FilePath] = append(byFile[n.FilePath], n)
		}
	}
	for f := range byFile {
		ns := byFile[f]
		sort.Slice(ns, func(i, j int) bool {
			if ns[i].Line != ns[j].Line {
				return ns[i].Line < ns[j].Line
			}
			return ns[i].Key() < ns[j].Key()
		})
	}
	return func(file string, line int) core.EdgeConfidence {
		ns := byFile[file]
		idx := sort.Search(len(ns), func(i int) bool { return ns[i].Line > line }) - 1
		if idx < 0 {
			return core.EdgeConfidenceHeuristic
		}
		enclosing := ns[idx].Key()
		for _, def := range defKeys {
			if edgeConfidence(g, enclosing, def) == core.EdgeConfidenceResolved {
				return core.EdgeConfidenceResolved
			}
		}
		return core.EdgeConfidenceHeuristic
	}
}
