package fusion

// checkovdispatch.go — ingest's claim of Checkov custom policies
// (LB-Q-checkov-policies, language-breadth: new — a tags+doc tool format). A Python
// check (checks/{resource,data,provider,module}/**/*.py) is read by the tags
// engine on the python grammar, a YAML graph check (checks/graph_checks/**/*.y*ml)
// by the doc engine. The manifest names the files by path glob alone, and a
// glob-claimed .py or .yaml would otherwise reach the python dialect's ".py" key
// or the YAML classifier first, so this hook sits at the top of resolveExtractor
// and ReingestFile: the classifier's glob verdict (connector.ClassifyDialect,
// the same one the ingest gate takes) is the whole decision. A file the globs do
// not name stays with python, the YAML classifier or no one.

import (
	"fmt"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// ToolCheckovPolicies: a Checkov Python check or YAML graph check
// (LB-Q-checkov-policies).
const ToolCheckovPolicies ToolKind = "checkov-policies"

const checkovDialectID = "checkov-policies"

// registerCheckov builds the dialect's one extractor (a tags engine and a doc
// engine) and keeps it with the pipeline's sniff-claimed dialects, so Close and
// the hosted-parser link see it.
func registerCheckov(p *Pipeline) {
	x, err := extractor.NewTagsDocExtractor(checkovDialectID)
	if err != nil {
		panic(fmt.Sprintf("fusion: tags+doc dialect %s: %v", checkovDialectID, err))
	}
	p.sniffExts[checkovDialectID] = x
}

// checkovDispatch returns the dialect's extractor and tool kind when a manifest
// glob names relPath; ok is false for every other file.
func (p *Pipeline) checkovDispatch(relPath string) (extractor.Extractor, ToolKind, bool) {
	e, ok := p.sniffExts[checkovDialectID]
	if !ok {
		return nil, "", false
	}
	if v := connector.ClassifyDialect(dialectRules(), relPath, nil); v.By != connector.ClassifiedByGlob || v.Dialect != checkovDialectID {
		return nil, "", false
	}
	return e, ToolCheckovPolicies, true
}
