package langroute

// extract.go — Router.ExtractFile, ingest's gates and dispatch over one file
// with no store: what the language-breadth harness (indexbench) and the
// readout's outline run. Moved out of internal/fusion's extractForBench
// unchanged (ADR-092).

import (
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// Extraction is the result of extracting one file outside a store.
type Extraction struct {
	Nodes    []core.SymbolNode
	ToolKind ToolKind
	// Skipped names why no extractor ran: "binary", "oversize", "generated",
	// "ambiguous", "unclassified" or "no-extractor"; "" when one did.
	Skipped string
	// TypeRefs is the type references the tags engine captured in this
	// file (design §3.4: counted in the reading, never stored).
	TypeRefs int
}

// ExtractFile runs ingest's gates (binary, size cap, the dialect gate) and
// dispatch on one file and returns the extractor's nodes, touching no store.
// index returns the sibling index the YAML classification reads (nil reads the
// file alone); it is called only for a file the gates let through, so a caller
// that memoizes the index builds it for those files alone. with, when not nil, is the extractor to run in place of dispatch's
// choice, the retirement protocol's parity run (LB-F18, design §11).
func (r *Router) ExtractFile(with extractor.Extractor, serviceID string, index func() *SiblingIndex, relPath string, content []byte) (Extraction, error) {
	if BinaryExcluded(relPath, content) {
		return Extraction{Skipped: "binary"}, nil
	}
	if len(content) > MaxExtractSize {
		return Extraction{Skipped: "oversize"}, nil
	}
	switch GateDialectFile(relPath, content).Excluded {
	case ExcludedSkippedGenerated:
		return Extraction{Skipped: "generated"}, nil
	case ExcludedAmbiguous:
		return Extraction{Skipped: "ambiguous"}, nil
	}
	var idx *SiblingIndex
	if index != nil {
		idx = index()
	}
	e, tk, ok, unclassified := r.ResolveExtractor(relPath, content, idx)
	switch {
	case with != nil:
		e = with
	case unclassified:
		return Extraction{ToolKind: tk, Skipped: "unclassified"}, nil
	case !ok:
		return Extraction{Skipped: "no-extractor"}, nil
	}
	tr, counts := e.(interface{ TypeRefs() int64 })
	var before int64
	if counts {
		before = tr.TypeRefs()
	}
	nodes, err := e.Extract(serviceID, relPath, content)
	out := Extraction{Nodes: nodes, ToolKind: tk}
	if counts {
		out.TypeRefs = int(tr.TypeRefs() - before)
	}
	return out, err
}
