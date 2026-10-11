package callgraph

// typedreach.go — RF-5 (ADR-067 D3; I57 extended to call edges): verify's
// reach never rests on a guess. The `reachable_from` criterion
// (edge_wired) reads ReachableTyped, and the `wired` criterion reads
// ProductReach (product.go), both over edges of resolution tier typed
// only (core.ResolutionTierOf: a typed pass confirmed the pair). A reach
// that exists only through an edge of tier heuristic (a syntax-only call,
// a reference to a type, a file-resolved import match) is refused, and
// the refusal names the edge it would have rested on. The edge itself is
// never removed: Reachable, TestImpact, CallersDetail and blast_radius
// still walk it, and serve it with its tier named.

import "github.com/geoffmilleraz/fathom/internal/core"

// HeuristicEdge is one call edge of resolution tier heuristic that a reach
// rested on: the first edge, along a path from where the typed reach ends,
// that no typed pass confirmed. Kind is the edge's label (edgeLabel): ""
// for a syntax-only call, core.EdgeTypeRef, core.EdgeFileResolved.
type HeuristicEdge struct {
	From, To string
	Kind     core.EdgeKind
}

// Tier is the edge's resolution tier (core.ResolutionTierOf): heuristic.
func (h HeuristicEdge) Tier() core.ResolutionTier {
	return core.ResolutionTierOf(core.Edge{FromSymbol: h.From, ToSymbol: h.To, Kind: h.Kind})
}

// typedEdge reports whether the caller→callee edge is of resolution tier
// typed: a typed pass added the pair (AddTypedEdges) with a kind
// core.ResolutionTierOf calls typed. A syntax-only edge, an
// EdgeFileResolved edge and a reference to a type are heuristic.
func (g *Graph) typedEdge(caller, callee string) bool {
	k, ok := g.edgeKind[caller+"\x00"+callee]
	return ok && core.ResolutionTierOf(core.Edge{Kind: k}).Kind == core.TierTyped
}

// ReachableTyped is Reachable over edges of resolution tier typed only:
// verify's edge_wired criterion (RF-5). A node trivially reaches itself.
// When from does not reach to that way, via names the heuristic edge a
// reach over every edge would rest on — the first edge leaving the typed
// reach on a shortest path from it — or is nil when no path exists at
// all. Deterministic: both walks are BFS over sorted adjacency.
func ReachableTyped(g *Graph, from, to string) (ok bool, via *HeuristicEdge) {
	fromKey, toKey := g.canonicalize(from), g.canonicalize(to)
	if fromKey == "" || toKey == "" {
		return false, nil
	}
	if fromKey == toKey {
		return true, nil
	}
	seen := map[string]bool{fromKey: true}
	order := []string{fromKey}
	for i := 0; i < len(order); i++ {
		cur := order[i]
		for _, next := range g.callees[cur] {
			if seen[next] || !g.typedEdge(cur, next) {
				continue
			}
			if next == toKey {
				return true, nil
			}
			seen[next] = true
			order = append(order, next)
		}
	}
	// Every edge out of the typed reach to a node outside it is heuristic
	// (a typed one would have been followed); carry the first one crossed.
	viaOf := map[string]HeuristicEdge{}
	queue := order
	for i := 0; i < len(queue); i++ {
		cur := queue[i]
		v, behind := viaOf[cur]
		for _, next := range g.callees[cur] {
			if seen[next] {
				continue
			}
			seen[next] = true
			nv := v
			if !behind {
				nv = HeuristicEdge{From: cur, To: next, Kind: g.edgeLabel(cur, next)}
			}
			if next == toKey {
				return false, &nv
			}
			viaOf[next] = nv
			queue = append(queue, next)
		}
	}
	return false, nil
}
