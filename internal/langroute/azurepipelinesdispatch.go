package langroute

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/langclass"
)

const ToolAzurePipelines ToolKind = "azure-pipelines"

var azurePipelineRootRe = regexp.MustCompile(`(?m)^(stages|jobs|steps|extends|parameters)[ \t]*:`)

// AzurePipelineDispatch claims pipeline basenames and literal stage/job templates
// only after earlier dispatchers and the YAML classifier decline.
// Content claims also need a top-level pipeline key.
func AzurePipelineDispatch(rel string, src []byte, idx *SiblingIndex) (extractor.Extractor, ToolKind, bool) {
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
	v := langclass.ClassifyDialect(DialectRules(), rel, src)
	if v.Dialect != "azure-pipelines" && (v.By == langclass.ClassifiedByBasename || (v.By == langclass.ClassifiedByGlob && v.Dialect != "helm")) {
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
