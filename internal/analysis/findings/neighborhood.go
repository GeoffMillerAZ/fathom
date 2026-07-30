package findings

// neighborhood.go — the §3.6 differentiating verb (docs/design/findings-
// as-evidence.md): "findings near <symbol|file>", the query only fathom can
// answer because it already has the call graph. Reuses
// internal/analysis/callgraph's existing traversal primitives
// (CallersDetail/CalleesDetail) — adds NO new storage, exactly as the
// design specifies. A pure function over already-fetched inputs, like every
// other file in this package.

import (
	"sort"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// NeighborhoodEntry is one finding within hops of the query subject, ranked
// (by the caller, RankNear) by proximity then recency.
type NeighborhoodEntry struct {
	Claim      core.SemanticClaim
	Hops       int  // 0 = the subject symbol/file itself
	Superseded bool // true for a MemberSuperseded/refuted-shaped claim — see Status
}

// symbolNeighborhood computes each symbol's minimum call-graph hop distance
// from seeds, bidirectionally (a finding is relevant whether the subject
// calls it or is called by it), out to maxHops. seeds start at hop 0.
func symbolNeighborhood(nodes []core.SymbolNode, typed []core.Edge, seeds []string, maxHops int) map[string]int {
	g := callgraph.Build(nodes)
	if len(typed) > 0 {
		callgraph.AddTypedEdges(g, typed)
	}

	hops := make(map[string]int, len(seeds))
	type item struct {
		key string
		hop int
	}
	var queue []item
	for _, s := range seeds {
		if _, seen := hops[s]; !seen {
			hops[s] = 0
			queue = append(queue, item{s, 0})
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.hop >= maxHops {
			continue
		}
		var neighbors []string
		for _, e := range callgraph.CallersDetail(g, cur.key) {
			neighbors = append(neighbors, e.Key)
		}
		for _, e := range callgraph.CalleesDetail(g, cur.key) {
			neighbors = append(neighbors, e.Key)
		}
		sort.Strings(neighbors)
		for _, n := range neighbors {
			if _, seen := hops[n]; seen {
				continue
			}
			hops[n] = cur.hop + 1
			queue = append(queue, item{n, cur.hop + 1})
		}
	}
	return hops
}

// FileNeighborhood computes each FILE's minimum hop distance from subject
// (a canonical symbol key, or a repo-relative file path — a file path
// expands to every symbol it defines as hop-0 seeds), out to maxHops. A
// file containing multiple symbols at different hop distances gets the
// MINIMUM (closest) distance. subject itself is included at hop 0 whenever
// it resolves to at least one known symbol/file; an unresolvable subject
// (no matching node, no matching file) returns an empty map — the caller
// renders that as "subject not found in the index", never a fabricated
// empty-but-valid neighborhood.
func FileNeighborhood(nodes []core.SymbolNode, typed []core.Edge, subject string, maxHops int) map[string]int {
	symbolFile := make(map[string]string, len(nodes))
	fileSymbols := map[string][]string{}
	for _, n := range nodes {
		symbolFile[n.Key()] = n.FilePath
		fileSymbols[n.FilePath] = append(fileSymbols[n.FilePath], n.Key())
	}

	var seeds []string
	if _, ok := symbolFile[subject]; ok {
		seeds = []string{subject}
	} else if syms, ok := fileSymbols[subject]; ok {
		seeds = append(seeds, syms...)
	}
	if len(seeds) == 0 {
		return nil
	}

	symHops := symbolNeighborhood(nodes, typed, seeds, maxHops)
	fileHops := map[string]int{}
	for sym, hop := range symHops {
		fp, ok := symbolFile[sym]
		if !ok {
			continue
		}
		if prior, seen := fileHops[fp]; !seen || hop < prior {
			fileHops[fp] = hop
		}
	}
	return fileHops
}

// RankNear filters claims to those whose Scope intersects fileHops (within
// the caller's requested hop range — fileHops is already maxHops-bounded by
// FileNeighborhood) and ranks them by proximity (ascending hops) then
// recency (descending CompiledAt) — the design's own stated order (§3.6).
// includeSuperseded controls whether a superseded/refuted claim is included
// at all; when included, every such entry is labeled (Superseded true) —
// never silently indistinguishable from a live finding (§4 honesty rule 4).
func RankNear(claims []core.SemanticClaim, fileHops map[string]int, includeSuperseded bool) []NeighborhoodEntry {
	var entries []NeighborhoodEntry
	for _, cl := range claims {
		superseded := cl.Status == core.ClaimSuperseded || cl.Status == core.ClaimRefuted
		if superseded && !includeSuperseded {
			continue
		}
		minHop := -1
		for _, s := range cl.Scope {
			if h, ok := fileHops[s]; ok && (minHop < 0 || h < minHop) {
				minHop = h
			}
		}
		if minHop < 0 {
			continue // no scoped file lies within the neighborhood
		}
		entries = append(entries, NeighborhoodEntry{Claim: cl, Hops: minHop, Superseded: superseded})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Hops != entries[j].Hops {
			return entries[i].Hops < entries[j].Hops
		}
		if !entries[i].Claim.CompiledAt.Equal(entries[j].Claim.CompiledAt) {
			return entries[i].Claim.CompiledAt.After(entries[j].Claim.CompiledAt)
		}
		return entries[i].Claim.ClaimID < entries[j].Claim.ClaimID
	})
	return entries
}
