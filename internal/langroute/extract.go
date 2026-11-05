package langroute

// extract.go — Router.ExtractFile, ingest's gates and dispatch over one file
// with no store: what the language-breadth harness (indexbench) and the
// readout's outline run. Moved out of internal/fusion's extractForBench
// unchanged (ADR-092).

import (
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/langclass"
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
	// Extractor is the extractor that ran, nil when none did.
	Extractor extractor.Extractor
	// Doc is the document engine's full result, set only by ExtractFileDoc for
	// an extractor that has one.
	Doc *extractor.DocResult
	// PackDialect is the id of the pack dialect that read the file, "" for a
	// built-in one.
	PackDialect string
	// RouteDialect is the dialect id dispatch settled on by sniff, glob or
	// shebang, for a file whose extractor, tool kind and nodes may name none (a
	// file with nothing to emit): "" for every other route. The door reads it.
	RouteDialect string
	// Incomplete is the extractor's one-line reason it could not fully read the
	// file, "" when it did. Only ExtractFileDoc sets it; the product's
	// ExtractFile never does, so its behavior and node output are unchanged.
	Incomplete string
}

// ExtractFile runs ingest's gates (binary, size cap, the dialect gate) and
// dispatch on one file and returns the extractor's nodes, touching no store.
// index returns the sibling index the YAML classification reads (nil reads the
// file alone); it is called only for a file the gates let through, so a caller
// that memoizes the index builds it for those files alone. with, when not nil, is the extractor to run in place of dispatch's
// choice, the retirement protocol's parity run (LB-F18, design §11).
func (r *Router) ExtractFile(with extractor.Extractor, serviceID string, index func() *SiblingIndex, relPath string, content []byte) (Extraction, error) {
	return r.extractFile(nil, with, serviceID, index, relPath, content, false)
}

// ExtractFileDoc is ExtractFile for the language library's door: the same gates
// and dispatch, and for an extractor with a document-engine result (the nodes
// are that result's nodes) the whole result in Extraction.Doc, so facts,
// cross-file references and refused references are not dropped.
func (r *Router) ExtractFileDoc(serviceID string, index func() *SiblingIndex, relPath string, content []byte) (Extraction, error) {
	return r.extractFile(nil, nil, serviceID, index, relPath, content, true)
}

// ExtractFileDocSteps is ExtractFileDoc that calls step between its stages:
// after the gates and dispatch settle on an extractor and before it runs, and
// again after it returns. A step that returns an error stops the file there,
// with that error and no Extraction. The language library passes its context's
// Err, so a cancelled or expired context is honored at a stage boundary; a
// stage already running is not interrupted (the engine's own deadline bounds it).
func (r *Router) ExtractFileDocSteps(step func() error, serviceID string, index func() *SiblingIndex, relPath string, content []byte) (Extraction, error) {
	return r.extractFile(step, nil, serviceID, index, relPath, content, true)
}

func (r *Router) extractFile(step func() error, with extractor.Extractor, serviceID string, index func() *SiblingIndex, relPath string, content []byte, wantDoc bool) (Extraction, error) {
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
	e, tk, ok, unclassified, routeDialect := r.resolveRoute(relPath, content, idx)
	switch {
	case with != nil:
		e = with
	case unclassified:
		return Extraction{ToolKind: tk, Skipped: "unclassified"}, nil
	case !ok:
		return Extraction{Skipped: "no-extractor"}, nil
	}
	packDialect := PackDialectOf(tk)
	if packDialect != "" && langclass.IsGeneratedOrMinified(relPath, content) {
		return Extraction{Skipped: "generated"}, nil // the dialect gate's rule, for a pack dialect's file
	}
	if step != nil {
		if err := step(); err != nil {
			return Extraction{}, err
		}
	}
	tr, counts := e.(interface{ TypeRefs() int64 })
	var before int64
	if counts {
		before = tr.TypeRefs()
	}
	var nodes []core.SymbolNode
	var err error
	var doc *extractor.DocResult
	var incomplete string
	if dr, isDoc := e.(extractor.DocResulter); wantDoc && isDoc && ownsDocResult(e, relPath) {
		var res extractor.DocResult
		res, err = dr.ExtractDoc(serviceID, relPath, content)
		nodes, doc, incomplete = res.Nodes, &res, res.Incomplete
	} else if re, reports := e.(extractor.ReportingExtractor); wantDoc && reports {
		nodes, incomplete, err = re.ExtractReport(serviceID, relPath, content)
	} else {
		nodes, err = e.Extract(serviceID, relPath, content)
	}
	if step != nil {
		if serr := step(); serr != nil {
			return Extraction{}, serr
		}
	}
	out := Extraction{Nodes: nodes, ToolKind: tk, Extractor: e, Doc: doc, PackDialect: packDialect, Incomplete: incomplete, RouteDialect: routeDialect}
	if counts {
		out.TypeRefs = int(tr.TypeRefs() - before)
	}
	return out, err
}

// ownsDocResult is false for a tags+doc dialect's file the tags engine owns,
// which has no document result and is read by Extract.
func ownsDocResult(e extractor.Extractor, relPath string) bool {
	if td, ok := e.(*extractor.TagsDocExtractor); ok {
		return td.IsDocFile(relPath)
	}
	return true
}
