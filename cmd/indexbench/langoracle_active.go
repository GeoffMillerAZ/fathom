package main

import (
	"fmt"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// noteFileEdgeCallersInactive names the file edges whose caller the build's own
// preprocessor discards, which the file-edge scoring leaves out (LB-F194, O368).
const noteFileEdgeCallersInactive = "file-resolved edges come from a caller on lines the build's own preprocessor discards (an inactive #if branch of the one configuration scip-clang indexes); the index holds no reference there, so they are left out of file_edge_precision as L1 and L2 leave out the same lines"

// scopeFileEdgesToActive drops the edges whose caller definition starts on a
// line the build's preprocessor discards (active: per file, sorted kept
// [first,last] ranges). A caller in a file the oracle gives no ranges for, or
// an edge with no mapped caller, is kept, and so is a file's module symbol
// (K18, LB-F434): it starts on line 1 by construction, and its edges are the
// file's includes and file-level references, whose lines an edge does not
// carry. It returns the kept edges and the count left out.
func scopeFileEdgesToActive(edges []core.Edge, nodes []core.SymbolNode, active map[string][][2]int) ([]core.Edge, int) {
	if len(active) == 0 {
		return edges, 0
	}
	line := make(map[string]int, len(nodes))
	for _, n := range nodes {
		if n.IsModuleSymbol() {
			continue
		}
		line[n.Key()] = n.Line
	}
	kept := edges[:0:0]
	for _, e := range edges {
		if lines, ok := active[e.FromFile]; ok {
			if l, found := line[e.FromSymbol]; found && !lineActive(lines, l) {
				continue
			}
		}
		kept = append(kept, e)
	}
	return kept, len(edges) - len(kept)
}

// scopeFileEdgesNote scopes edges and, when any are left out, discloses the count.
func scopeFileEdgesNote(edges []core.Edge, nodes []core.SymbolNode, active map[string][][2]int, name string, notes *[]string) []core.Edge {
	kept, n := scopeFileEdgesToActive(edges, nodes, active)
	if n > 0 {
		*notes = append(*notes, fmt.Sprintf("%s: %d %s", name, n, noteFileEdgeCallersInactive))
	}
	return kept
}
