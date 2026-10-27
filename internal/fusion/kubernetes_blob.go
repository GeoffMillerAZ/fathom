package fusion

import (
	"fmt"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// ExtractKubernetesBlob is a store-free, filesystem-free projection of ingest's
// dispatch for a historical blob. refs supplies that commit's sibling paths.
// policy applies the connector's skip rules using that commit's project config
// and the caller's scoped exclusions, keeping I/O outside this pure extraction.
// Only Kubernetes extractors run.
func (p *Pipeline) ExtractKubernetesBlob(serviceID string, refs []core.SourceRef, relPath string, content []byte, policy connector.BlobCorpusPolicy) (BenchExtraction, error) {
	if reason := policy.Exclusion(relPath); reason != "" {
		return BenchExtraction{Skipped: reason}, nil
	}
	if len(content) > connector.MaxFetchSize {
		return BenchExtraction{Skipped: "oversize"}, nil
	}
	if binaryExcluded(relPath, content) {
		return BenchExtraction{Skipped: "binary"}, nil
	}
	switch gateDialectFile(relPath, content).excluded {
	case ExcludedSkippedGenerated:
		return BenchExtraction{Skipped: "generated"}, nil
	case ExcludedAmbiguous:
		return BenchExtraction{Skipped: "ambiguous"}, nil
	}
	e, tk, ok, _ := p.resolveExtractor(relPath, content, buildSiblingIndex(refs))
	if !ok || (tk != ToolK8sManifest && tk != ToolK8sCRD) {
		return BenchExtraction{ToolKind: tk, Skipped: fmt.Sprintf("ingest does not read as Kubernetes (tool_kind=%q)", tk)}, nil
	}
	nodes, err := e.Extract(serviceID, relPath, content)
	return BenchExtraction{Nodes: nodes, ToolKind: tk}, err
}
