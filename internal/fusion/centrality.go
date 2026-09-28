package fusion

// centrality.go — TQ-2 (docs/plans/test-intelligence.md; design doc
// docs/design/test-intelligence.md §1 owner decision, §3 T1, §4): per
// symbol, its transitive caller count over the typed call graph, normalized
// to [0, 1] within the service, computed once at ingest over the graph's
// SCC condensation (callgraph.TransitiveCallerCounts — never a BFS per
// symbol) and stored through store.SetSymbolCentrality, memoized per
// watermark: an ingest whose watermark matches the stored one and that
// changed no input file leaves the stored rows alone. A T1 risk read is
// then a sort over stored rows (I48), never a graph walk.

import (
	"context"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// computeCentrality returns one row per graph node (the syntax graph plus
// typed edges, the same graph blast_radius walks), sorted by symbol.
// Centrality is Callers divided by the service's maximum Callers; every
// row is 0 when no symbol has a caller.
func computeCentrality(nodes []core.SymbolNode, typed []core.Edge) []store.SymbolCentrality {
	if len(nodes) == 0 {
		return nil
	}
	g := callgraph.BuildBoth(nodes, typed)
	counts := callgraph.TransitiveCallerCounts(g)
	files := make(map[string]string, len(nodes))
	for _, n := range nodes {
		if _, ok := files[n.Key()]; !ok {
			files[n.Key()] = n.FilePath
		}
	}
	max := 0
	for _, c := range counts {
		if c > max {
			max = c
		}
	}
	out := make([]store.SymbolCentrality, 0, len(counts))
	for key, c := range counts {
		r := store.SymbolCentrality{Symbol: key, File: files[key], Callers: c}
		if max > 0 {
			r.Centrality = float64(c) / float64(max)
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out
}

// recordCentrality is the ingest-time step: skip when the stored rows were
// computed at this same watermark and this ingest changed no input (the
// memo), otherwise recompute from the stored symbols and typed edges.
// Best-effort like the rest of recordIngestMeta: a failed read leaves the
// previous rows in place rather than aborting the ingest.
func (p *Pipeline) recordCentrality(ctx context.Context, serviceID, watermark string, changed int) {
	if watermark != "" && changed == 0 {
		if stored, rows, err := p.st.SymbolCentrality(ctx, serviceID); err == nil && stored == watermark && len(rows) > 0 {
			return
		}
	}
	obs, err := p.st.QueryObservations(ctx, serviceID, "")
	if err != nil {
		return
	}
	nodes := make([]core.SymbolNode, 0, len(obs))
	for _, o := range obs {
		nodes = append(nodes, o.ToSymbolNode())
	}
	typed, err := p.st.TypedEdges(ctx, serviceID)
	if err != nil {
		return
	}
	_ = p.st.SetSymbolCentrality(ctx, serviceID, watermark, computeCentrality(nodes, typed))
}
