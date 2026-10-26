package fusion

import (
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"path/filepath"
	"strings"
)

const ToolAzurePipelines ToolKind = "azure-pipelines"

// azurePipelineDispatch claims pipeline basenames and literal stage/job templates.
// Existing workflow paths and Kubernetes objects retain their earlier ownership.
func azurePipelineDispatch(rel string, src []byte, idx *siblingIndex) (extractor.Extractor, ToolKind, bool) {
	ext := strings.ToLower(filepath.Ext(rel))
	if ext != ".yml" && ext != ".yaml" {
		return nil, "", false
	}
	if strings.Contains(filepath.ToSlash(rel), ".github/workflows/") || (apiVersionTopRe.Match(src) && kindTopRe.Match(src)) {
		return nil, "", false
	}
	x := lateYAMLLoad("azure-pipelines")
	if x == nil {
		return nil, "", false
	}
	v := connector.ClassifyDialect(dialectRules(), rel, src)
	if v.Dialect != "azure-pipelines" && (v.By == connector.ClassifiedByBasename || (v.By == connector.ClassifiedByGlob && v.Dialect != "helm")) {
		return nil, "", false
	}
	named := v.Dialect == "azure-pipelines"
	if !named && !x.DetectsContent(src) {
		return nil, "", false
	}
	r, err := x.ExtractDoc("", rel, src)
	if err != nil || len(r.Roots) == 0 || (!named && len(r.Nodes) < 2) {
		return nil, "", false
	}
	if Classify(rel, src, idx) == ToolHelmTemplateYAML {
		return nil, "", false
	}
	return x, ToolAzurePipelines, true
}
