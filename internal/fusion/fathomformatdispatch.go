package fusion

// fathomformatdispatch.go — ingest's claim of fathom's own formats
// (LB-F219, language-breadth wave fx item fmt): a project's
// .fathom/config.toml (the fathom-config doc-engine dialect) and a plan
// under docs/plans/ (the fathom-plan-rows native dialect). Each dialect's
// manifest names the path (a glob), so classification stays the manifest's;
// a plan is also a plan by content: a checkbox row whose annotation block
// carries an author-owned id, so docs/plans/README.md and any other Markdown
// file in that directory read exactly as before.
//
// resolveExtractor (ingest and ExtractForBench) and ReingestFile both call
// fathomFormatDispatch before the extension and basename maps, so the
// language-breadth harness measures the dispatch ingest runs.

import (
	"sync"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

const (
	// ToolFathomConfig: a project's .fathom/config.toml (LB-Q-fathom-config).
	ToolFathomConfig ToolKind = "fathom-config"
	// ToolFathomPlanRows: a plan grammar v2 file under docs/plans/ (LB-Q-fathom-plan-rows).
	ToolFathomPlanRows ToolKind = "fathom-plan-rows"
)

var (
	fathomFormatOnce sync.Once
	fathomConfigExt  *extractor.DocExtractor
	fathomPlansExt   *extractor.PlanRowsExtractor
)

// fathomFormatExtractors builds the two extractors once; a config dialect
// that does not load is absent (TestDialectManifests fails the build's tests
// on that, and dispatch then claims no config file).
func fathomFormatExtractors() (*extractor.DocExtractor, *extractor.PlanRowsExtractor) {
	fathomFormatOnce.Do(func() {
		if x, err := extractor.NewDocDialectExtractor("fathom-config"); err == nil {
			fathomConfigExt = x
		}
		fathomPlansExt = extractor.NewPlanRowsExtractor()
	})
	return fathomConfigExt, fathomPlansExt
}

// fathomFormatDispatch returns the extractor and ToolKind of a file in one
// of fathom's own formats: relPath matches the dialect's manifest glob and,
// for a plan, the content is a plan (extractor.LooksLikePlan).
func fathomFormatDispatch(relPath string, content []byte) (extractor.Extractor, ToolKind, bool) {
	v := connector.ClassifyDialect(dialectRules(), relPath, nil)
	if v.By != connector.ClassifiedByGlob {
		return nil, "", false
	}
	cfg, plans := fathomFormatExtractors()
	switch v.Dialect {
	case "fathom-config":
		if cfg != nil {
			return cfg, ToolFathomConfig, true
		}
	case "fathom-plan-rows":
		if extractor.LooksLikePlan(content) {
			return plans, ToolFathomPlanRows, true
		}
	}
	return nil, "", false
}
