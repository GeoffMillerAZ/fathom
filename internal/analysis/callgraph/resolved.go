package callgraph

import (
	"sort"
	"strings"

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

// CandidateKeys returns EVERY node key the symbol could name, sorted:
// an exact key, a package-qualified key, or — the case canonicalize
// deliberately refuses — every node whose key ends in the symbol.
//
// canonicalize's refusal is right for a question that must have ONE answer
// ("does A reach B"): first-match-wins on an ambiguous name is how a graph
// starts lying. It is wrong for "who calls X", which has a perfectly good
// answer when X names three functions — the union, with every candidate
// named. Before this, `find_callers Reachable` on a repository with both
// `callgraph.Reachable` and `impact.Reachable` resolved to nothing and the
// handler fell back to the syntax answer, which is exactly the imprecise
// answer BP-26 set out to stop returning (bench-pyramid BP-31: 19 of the
// 327 truth callers the L0 self oracle scored were lost this way, every one
// of them PRESENT in the typed graph under its own canonical key).
//
// Returns nil when the symbol names no node at all.
func CandidateKeys(g *Graph, symbol string) []string {
	if g == nil || symbol == "" {
		return nil
	}
	if _, ok := g.nodes[symbol]; ok {
		return []string{symbol}
	}
	// A BARE name (no dot) matches only nodes whose Name is exactly that.
	// A key-suffix match would cross the receiver boundary — "Reachable"
	// would pull in the method "residueEvidence.Reachable", whose callers
	// answer a different question and which the go/types truth spells as
	// its own separate name. Exact-Name is both the precise reading and the
	// one the truth shares.
	// The same holds for a method asked by its own spelling, "Recv.Method",
	// which IS a node's Name: it matches the nodes that carry that name,
	// not every key ending in ".Method".
	//
	// byNameAll is checked BEFORE canonicalize on purpose: canonicalize's
	// last resort is an O(nodes) suffix scan it runs on every name it
	// cannot place before returning "", which cost the L0 self oracle 48ms
	// a query for an answer one map lookup already had.
	if keys := g.byNameAll[symbol]; len(keys) > 0 {
		return append([]string(nil), keys...)
	}
	if !strings.Contains(symbol, ".") {
		return nil
	}
	if key := g.canonicalize(symbol); key != "" {
		return []string{key}
	}
	// A DOTTED partial ("pkg.Name", "Recv.Method") keeps the component-
	// boundary suffix match canonicalize uses, minus its refusal to name
	// more than one.
	var out []string
	for key := range g.nodes {
		if keyEndsWith(key, symbol) {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

// ResolvedCallersAcross unions ResolvedCallers over every candidate
// CandidateKeys reports, deduplicated by caller key and sorted. An edge that
// is resolved for ANY candidate is resolved: the ambiguity is in the
// QUESTION, never in the evidence.
//
// Returns nil (not an empty slice) when the symbol names no node, so a
// caller can still tell "this name is not in the index" — which is not an
// answer — from "this name has no resolved callers", which is.
func ResolvedCallersAcross(g *Graph, symbol string) []EdgeInfo {
	keys := CandidateKeys(g, symbol)
	if keys == nil {
		return nil
	}
	seen := map[string]EdgeInfo{}
	for _, k := range keys {
		for _, e := range ResolvedCallers(g, k) {
			if _, dup := seen[e.Key]; !dup {
				seen[e.Key] = e
			}
		}
	}
	out := make([]EdgeInfo, 0, len(seen))
	for _, e := range seen {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}
