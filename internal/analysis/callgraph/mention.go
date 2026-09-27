package callgraph

import (
	"sort"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// edgeConfidenceBetween reads the confidence of the (caller → callee) edge
// in g: resolved when the typed pass confirmed that exact pair, heuristic
// for a syntax-only edge (EdgeInfo.Kind == "").
func edgeConfidenceBetween(g *Graph, caller, callee string) core.EdgeConfidence {
	for _, d := range CallersDetail(g, callee) {
		if d.Key == caller {
			return core.EdgeConfidenceOf(core.Edge{FromSymbol: caller, ToSymbol: callee, Kind: d.Kind})
		}
	}
	return core.EdgeConfidenceHeuristic
}

// MentionIndex is the query-independent part of mention confirmation: the
// combined syntax+typed call graph and the per-file, line-sorted symbol
// lists used to find a mention's enclosing symbol. Building it costs one
// pass over every node and edge, so BuildMentionIndex is meant to run ONCE
// per (nodes, typed) pair — a caller answering many queries over the same
// service (the L0 index oracle's SB-45 exact-mode scoring, hundreds of
// queries per subject) builds it once and calls Confidence per query,
// rather than paying the graph-construction cost on every query the way a
// single-shot MentionConfidence call would.
type MentionIndex struct {
	g     *Graph
	nodes []core.SymbolNode
	// byFile holds, per file, indexes into nodes sorted by (line, key) —
	// built on the first lookup, so an index nobody asks costs nothing and
	// the nodes are never copied.
	once   sync.Once
	byFile map[string][]int32
}

// BuildMentionIndex builds the index once for (nodes, typed).
func BuildMentionIndex(nodes []core.SymbolNode, typed []core.Edge) *MentionIndex {
	g := Build(nodes)
	AddTypedEdges(g, typed)
	return &MentionIndex{g: g, nodes: nodes}
}

// NewMentionIndex is BuildMentionIndex over an already-built graph of
// nodes plus their typed edges (BuildBoth's graph — the shared memo's
// GraphBoth), so a caller holding one pays no graph build. nodes is read,
// never copied or reordered.
func NewMentionIndex(both *Graph, nodes []core.SymbolNode) *MentionIndex {
	return &MentionIndex{g: both, nodes: nodes}
}

func (m *MentionIndex) buildByFile() {
	m.byFile = map[string][]int32{}
	for i := range m.nodes {
		if m.nodes[i].Line > 0 {
			f := m.nodes[i].FilePath
			m.byFile[f] = append(m.byFile[f], int32(i))
		}
	}
	for _, ix := range m.byFile {
		sort.Slice(ix, func(a, b int) bool {
			na, nb := &m.nodes[ix[a]], &m.nodes[ix[b]]
			if na.Line != nb.Line {
				return na.Line < nb.Line
			}
			return na.Key() < nb.Key()
		})
	}
}

// Confidence returns a lookup that classifies a text mention at (file,
// line) against defKeys — the current query's definitions in this service:
// resolved when the symbol enclosing that line has a typed call edge to
// any of them, heuristic otherwise. The enclosing symbol is the last
// indexed symbol in the file that starts at or before the line (most Go
// definitions carry no end_line, so "last start before" is the honest
// approximation). Deterministic: per-file symbol lists are sorted by line,
// then key.
func (m *MentionIndex) Confidence(defKeys []string) func(file string, line int) core.EdgeConfidence {
	return func(file string, line int) core.EdgeConfidence {
		m.once.Do(m.buildByFile)
		ns := m.byFile[file]
		idx := sort.Search(len(ns), func(i int) bool { return m.nodes[ns[i]].Line > line }) - 1
		if idx < 0 {
			return core.EdgeConfidenceHeuristic
		}
		enclosing := m.nodes[ns[idx]].Key()
		for _, def := range defKeys {
			if edgeConfidenceBetween(m.g, enclosing, def) == core.EdgeConfidenceResolved {
				return core.EdgeConfidenceResolved
			}
		}
		return core.EdgeConfidenceHeuristic
	}
}

// MentionConfidence is the single-shot convenience form — shared by the
// `references` tool's per-occurrence labeling (BD-2), which answers one
// query per call and so has no index to reuse. A caller scoring MANY
// queries over the same (nodes, typed) pair (SB-45's L0 oracle) should use
// BuildMentionIndex once and call Confidence per query instead.
func MentionConfidence(nodes []core.SymbolNode, typed []core.Edge, defKeys []string) func(file string, line int) core.EdgeConfidence {
	return BuildMentionIndex(nodes, typed).Confidence(defKeys)
}
