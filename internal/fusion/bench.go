package fusion

// bench.go — ExtractForBench and ExtractForBenchWith, the indexbench entry points that run
// one file through the live dispatch or a named extractor with no store. Split out of
// pipeline.go unchanged.

import (
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// BenchExtraction is the result of extracting one file outside a store.
type BenchExtraction struct {
	Nodes    []core.SymbolNode
	ToolKind ToolKind
	// Skipped names why no extractor ran: "binary", "oversize",
	// "unclassified" or "no-extractor"; "" when one did.
	Skipped string
	// TypeRefs is the type references the tags engine captured in this
	// file (design §3.4: counted in the reading, never stored).
	TypeRefs int
}

// ExtractForBench runs ingest's gates (binary, size cap) and dispatch on
// one file and returns the extractor's nodes, touching no store — the
// language-breadth harness's in-process extraction (LB-F23, design §10).
// refs is the service's Discover list (the sibling index YAML
// classification reads).
func (p *Pipeline) ExtractForBench(serviceID string, refs []core.SourceRef, relPath string, content []byte) (BenchExtraction, error) {
	return p.extractForBench(nil, serviceID, refs, relPath, content)
}

// ExtractForBenchWith runs ingest's gates on one file and then e, not
// dispatch's choice — the retirement protocol's parity run (LB-F18, design
// §11), which measures a dialect while its incumbent is still dispatched.
func (p *Pipeline) ExtractForBenchWith(e extractor.Extractor, serviceID string, refs []core.SourceRef, relPath string, content []byte) (BenchExtraction, error) {
	return p.extractForBench(e, serviceID, refs, relPath, content)
}

func (p *Pipeline) extractForBench(with extractor.Extractor, serviceID string, refs []core.SourceRef, relPath string, content []byte) (BenchExtraction, error) {
	if binaryExcluded(relPath, content) {
		return BenchExtraction{Skipped: "binary"}, nil
	}
	if len(content) > MaxExtractSize {
		return BenchExtraction{Skipped: "oversize"}, nil
	}
	switch gateDialectFile(relPath, content).excluded {
	case ExcludedSkippedGenerated:
		return BenchExtraction{Skipped: "generated"}, nil
	case ExcludedAmbiguous:
		return BenchExtraction{Skipped: "ambiguous"}, nil
	}
	idx, ok := p.siblingIdx[serviceID]
	if !ok {
		idx = buildSiblingIndex(refs)
		p.siblingIdx[serviceID] = idx
	}
	e, tk, ok, unclassified := p.resolveExtractor(relPath, content, idx)
	switch {
	case with != nil:
		e = with
	case unclassified:
		return BenchExtraction{ToolKind: tk, Skipped: "unclassified"}, nil
	case !ok:
		return BenchExtraction{Skipped: "no-extractor"}, nil
	}
	tr, counts := e.(interface{ TypeRefs() int64 })
	var before int64
	if counts {
		before = tr.TypeRefs()
	}
	nodes, err := e.Extract(serviceID, relPath, content)
	out := BenchExtraction{Nodes: nodes, ToolKind: tk}
	if counts {
		out.TypeRefs = int(tr.TypeRefs() - before)
	}
	return out, err
}
