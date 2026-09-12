package callgraph

import (
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// BuildResolved constructs a Graph over the same node universe Build uses,
// carrying ONLY the edges the Go typed pass resolved (core.EdgeConfidenceOf
// == resolved: EdgeDirect, EdgeIfaceDispatch, EdgeFuncValue — never the
// syntax-derived edges Build reads off SymbolNode.Calls, and never
// EdgeUnresolved, whose ToSymbol is a call-site marker rather than a key).
//
// Why a second graph rather than a filter over the first: the syntax pass's
// tolerant name matching is what a resolved-only answer is trying to leave
// behind (bench-pyramid BP-26/BP-27 measured find_callers at precision 0.649
// and blast_radius at 0.075 with it in). Sharing buildIndexes keeps
// canonicalization — the way "Foo" becomes "svc::pkg.Foo" — identical
// between the two, so a caller can swap graphs without the QUERY changing
// meaning, only the evidence behind the answer.
//
// A service with no typed edges (a non-Go corpus, or a Go corpus whose typed
// pass has not run) yields a graph with nodes and no edges. Callers MUST
// treat that case as "cannot answer at resolved confidence" and fall back —
// an empty answer from an empty graph is not evidence of no callers.
func BuildResolved(nodes []core.SymbolNode, typed []core.Edge) *Graph {
	g := buildIndexes(nodes)
	resolved := make([]core.Edge, 0, len(typed))
	for _, e := range typed {
		if core.EdgeConfidenceOf(e) == core.EdgeConfidenceResolved {
			resolved = append(resolved, e)
		}
	}
	AddTypedEdges(g, resolved)
	return g
}

// HasEdges reports whether the graph carries at least one edge. It is the
// guard a resolved-by-default caller needs: a graph with no edges cannot
// distinguish "no callers" from "no typed pass for this service".
func (g *Graph) HasEdges() bool {
	for _, cs := range g.callees {
		if len(cs) > 0 {
			return true
		}
	}
	return false
}

// ResolvedCallers returns the callers of symbol whose edge the typed pass
// resolved, sorted by key. On a graph from BuildResolved every edge already
// qualifies; on a mixed graph (Build + AddTypedEdges) this is the filter
// that drops the syntax-only rows. Returns nil when symbol does not resolve
// to a node.
func ResolvedCallers(g *Graph, symbol string) []EdgeInfo {
	all := CallersDetail(g, symbol)
	if all == nil {
		return nil
	}
	out := make([]EdgeInfo, 0, len(all))
	for _, e := range all {
		if core.EdgeConfidenceOf(core.Edge{Kind: e.Kind}) == core.EdgeConfidenceResolved {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}
