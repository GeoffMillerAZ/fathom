package fusion

import (
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"path/filepath"
	"regexp"
	"strings"
)

const ToolAzurePipelines ToolKind = "azure-pipelines"

var azurePipelineRootRe = regexp.MustCompile(`(?m)^(stages|jobs|steps|extends|parameters)[ \t]*:`)

// azurePipelineDispatch claims pipeline basenames and literal stage/job templates
// only after earlier dispatchers and the YAML classifier decline.
// Content claims also need a top-level pipeline key.
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
	if !named && (!azurePipelineRootRe.Match(src) || !x.DetectsContent(src)) {
		return nil, "", false
	}
	r, err := x.ExtractDoc("", rel, src)
	if err != nil || len(r.Roots) == 0 || (!named && len(r.Nodes) < 2) {
		return nil, "", false
	}
	if Classify(rel, src, idx) != ToolUnclassified {
		return nil, "", false
	}
	return x, ToolAzurePipelines, true
}
