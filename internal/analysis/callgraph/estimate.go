package callgraph

// estimate.go — a graph's resident size, for the memo registry's byte
// budget (docs/design/responsible-tenant.md §5, I51). The budget is only
// as good as the builders' estimates: a flat per-node guess undercounted a
// graph about 3× (review finding), so this sums the graph's own maps.

import (
	"unsafe"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// mapEntryOverhead is a Go map's per-entry cost beyond its key and value
// (bucket slots at a typical load factor, tophash, overflow): calibrated
// against the heap by internal/memo's TestByteEstimatesTrackTheHeap.
const mapEntryOverhead = 96

const strHdr = int64(unsafe.Sizeof(""))

// EstimatedBytes is the graph's approximate resident size in bytes: every
// index map's entries, their keys (counted as owned — a canonical key is
// built per node) and the adjacency lists. Node strings shared with the
// snapshot the graph was built from are not counted again.
func (g *Graph) EstimatedBytes() int64 {
	if g == nil {
		return 0
	}
	nodeSize := int64(unsafe.Sizeof(core.SymbolNode{}))
	var n int64
	for k := range g.nodes {
		n += strHdr + int64(len(k)) + nodeSize + mapEntryOverhead
	}
	adj := func(m map[string][]string) {
		for _, vs := range m {
			n += strHdr + 24 + mapEntryOverhead + int64(cap(vs))*strHdr
		}
	}
	adj(g.callees)
	adj(g.callers)
	adj(g.byNameAll)
	for _, m := range []map[string]string{g.byName, g.byPkgName, g.byDirName} {
		for k := range m {
			n += 2*strHdr + int64(len(k)) + mapEntryOverhead
		}
	}
	for _, m := range []map[string]suffixCandidate{g.bySuffix, g.byDotSuffix} {
		n += int64(len(m)) * (strHdr + int64(unsafe.Sizeof(suffixCandidate{})) + mapEntryOverhead)
	}
	for k := range g.edgeKind {
		n += strHdr + int64(len(k)) + 8 + mapEntryOverhead
	}
	n += int64(len(g.dispatchOut)) * (strHdr + 8 + mapEntryOverhead)
	return n
}
