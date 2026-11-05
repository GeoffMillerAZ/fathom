package fusion

// bench.go — ExtractForBench and ExtractForBenchWith, the indexbench entry points that run
// one file through the live dispatch or a named extractor with no store. Split out of
// pipeline.go unchanged.

import (
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/langroute"
)

// BenchExtraction is langroute.Extraction: the result of extracting one file
// outside a store.
type BenchExtraction = langroute.Extraction

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
	index := func() *siblingIndex {
		idx, ok := p.siblingIdx[serviceID]
		if !ok {
			idx = buildSiblingIndex(refs)
			p.siblingIdx[serviceID] = idx
		}
		return idx
	}
	return p.Router.ExtractFile(with, serviceID, index, relPath, content)
}
