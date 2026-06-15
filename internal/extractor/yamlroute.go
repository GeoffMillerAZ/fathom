package extractor

import (
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// YAMLExtractor routes YAML files to the appropriate extractor:
// files under .github/workflows/ are sent to the GHA extractor;
// all other YAML files are sent to the Kubernetes extractor.
type YAMLExtractor struct {
	k8s Extractor
	gha Extractor
}

// NewYAMLExtractor returns a YAMLExtractor that uses k8s for Kubernetes YAML
// and gha for GitHub Actions workflows.
func NewYAMLExtractor(k8s, gha Extractor) *YAMLExtractor {
	return &YAMLExtractor{k8s: k8s, gha: gha}
}

// Extract routes the file to the appropriate extractor based on its path.
func (y *YAMLExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	if strings.Contains(filepath.ToSlash(relPath), ".github/workflows/") {
		return y.gha.Extract(service, relPath, src)
	}
	return y.k8s.Extract(service, relPath, src)
}
