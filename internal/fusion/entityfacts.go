package fusion

// entityfacts.go — dialect facts stored (WAE-4, ADR-069): the facts a
// dialect's doc spec declares (DocResult.Facts, which DocExtractor.Extract
// drops) become EntityFact nodes beside the entities, written with the
// file's other observations and purged with it (the ADR-064 precedent): no
// table, no pass. Ingest's extract sink, ReingestFile and the embedded
// documents' reader (k8sEmbedExtractor) store them alike.

import (
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// factsDeclarer is a dialect whose doc spec declares facts
// (extractor.DocExtractor.DeclaresFacts).
type factsDeclarer interface {
	DeclaresFacts() bool
}

// extractWithFacts is x.Extract, plus the facts x declares as EntityFact
// nodes; a dialect that declares none is read by Extract alone.
func extractWithFacts(x extractor.Extractor, service, relPath string, src []byte) ([]core.SymbolNode, error) {
	doc, isDoc := x.(extractor.DocResulter)
	if fd, ok := x.(factsDeclarer); !isDoc || !ok || !fd.DeclaresFacts() {
		return x.Extract(service, relPath, src)
	}
	res, err := doc.ExtractDoc(service, relPath, src)
	if err != nil {
		return res.Nodes, err
	}
	return append(res.Nodes, storeEntityFacts(res)...), nil
}

// storeEntityFacts turns a result's facts into EntityFact nodes, in the
// result's fact order: `<entity>#<key>`, the entity's service, package,
// file and tool kind, the value as written, at the fact's line, tier
// verified. A fact whose entity the result carries no node for is dropped:
// it has no package to stand in.
func storeEntityFacts(res extractor.DocResult) []core.SymbolNode {
	if len(res.Facts) == 0 {
		return nil
	}
	entity := make(map[string]core.SymbolNode, len(res.Nodes))
	for _, n := range res.Nodes {
		if _, seen := entity[n.Name]; !seen {
			entity[n.Name] = n
		}
	}
	out := make([]core.SymbolNode, 0, len(res.Facts))
	for _, f := range res.Facts {
		e, ok := entity[f.Entity]
		if !ok {
			continue
		}
		out = append(out, core.SymbolNode{
			Service:   e.Service,
			Package:   e.Package,
			Name:      f.Entity + "#" + f.Key,
			Kind:      core.KindEntityFact,
			FilePath:  e.FilePath,
			Line:      f.Line,
			EndLine:   f.Line,
			Signature: f.Value,
			Tier:      core.TierVerified,
			ToolKind:  e.ToolKind,
		})
	}
	return out
}
