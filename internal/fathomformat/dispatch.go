// Package fathomformat claims fathom's own formats for the language router: a
// project's .fathom/config.toml, its CUE packs and a plan under docs/plans/
// (ADR-092). They are product formats, so the language library does not link
// them; the product and the readout register Dispatch on a langroute.Router.
package fathomformat

// dispatch.go — ingest's claim of fathom's own formats
// (LB-F219, language-breadth wave fx item fmt): a project's
// .fathom/config.toml (the fathom-config doc-engine dialect), the CUE formats
// (.fathom/concerns.cue, a layer.cue, a rule pack: the native fathom-concerns and
// fathom-layer-packs dialects over the CUE extraction) and a plan
// under docs/plans/ (the fathom-plan-rows native dialect). Each dialect's
// manifest names the path (a glob), so classification stays the manifest's;
// a plan is also a plan by content: a checkbox row whose annotation block
// carries an author-owned id, so docs/plans/README.md and any other Markdown
// file in that directory read exactly as before.
//
// langroute.Router.ResolveExtractor (ingest and ExtractForBench) and Resolve
// (ReingestFile) both call Dispatch before the extension and basename maps, so the
// language-breadth harness measures the dispatch ingest runs.

import (
	"sync"

	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/extractor/cuelang"
	"github.com/geoffmilleraz/fathom/internal/langclass"
	"github.com/geoffmilleraz/fathom/internal/langroute"
	"github.com/geoffmilleraz/fathom/internal/planrowsdialect"
)

const (
	// ToolFathomConfig: a project's .fathom/config.toml (LB-Q-fathom-config).
	ToolFathomConfig langroute.ToolKind = "fathom-config"
	// ToolFathomPlanRows: a plan grammar v2 file under docs/plans/ (LB-Q-fathom-plan-rows).
	ToolFathomPlanRows langroute.ToolKind = "fathom-plan-rows"
	// ToolFathomConcerns: a project's .fathom/concerns.cue (LB-Q-fathom-concerns).
	ToolFathomConcerns langroute.ToolKind = "fathom-concerns"
	// ToolFathomLayerPacks: a layer pack's layer.cue or a rule pack under .fathom/rules/ (LB-Q-fathom-layer-packs).
	ToolFathomLayerPacks langroute.ToolKind = "fathom-layer-packs"
)

var (
	fathomFormatOnce sync.Once
	fathomConfigExt  *extractor.DocExtractor
	fathomPlansExt   *planrowsdialect.PlanRowsExtractor
	fathomCUEExt     = map[string]*cuelang.FathomCUEExtractor{}
)

// fathomFormatExtractors builds the two extractors once; a config dialect
// that does not load is absent (TestDialectManifests fails the build's tests
// on that, and dispatch then claims no config file).
func fathomFormatExtractors() (*extractor.DocExtractor, *planrowsdialect.PlanRowsExtractor) {
	fathomFormatOnce.Do(func() {
		if x, err := extractor.NewDocDialectExtractor("fathom-config"); err == nil {
			fathomConfigExt = x
		}
		fathomPlansExt = planrowsdialect.NewPlanRowsExtractor()
		for _, id := range []string{"fathom-concerns", "fathom-layer-packs"} {
			fathomCUEExt[id] = cuelang.NewFathomCUEExtractor(id)
		}
	})
	return fathomConfigExt, fathomPlansExt
}

// Dispatch returns the extractor and tool kind of a file in one
// of fathom's own formats: relPath matches the dialect's manifest glob and,
// for a plan, the content is a plan (planrowsdialect.LooksLikePlan).
func Dispatch(relPath string, content []byte) (extractor.Extractor, langroute.ToolKind, bool) {
	v := langclass.ClassifyDialect(langroute.DialectRules(), relPath, nil)
	if v.By != langclass.ClassifiedByGlob {
		return nil, "", false
	}
	cfg, plans := fathomFormatExtractors()
	switch v.Dialect {
	case "fathom-config":
		if cfg != nil {
			return cfg, ToolFathomConfig, true
		}
	case "fathom-concerns":
		return fathomCUEExt[v.Dialect], ToolFathomConcerns, true
	case "fathom-layer-packs":
		return fathomCUEExt[v.Dialect], ToolFathomLayerPacks, true
	case "fathom-plan-rows":
		if planrowsdialect.LooksLikePlan(content) {
			return plans, ToolFathomPlanRows, true
		}
	}
	return nil, "", false
}

// RouterOptions is the registration list every product router carries beyond
// the language library's own: the CUE extractor, the Timoni extractor the
// caller supplies (fusion shares one across pipelines) and fathom's own
// formats. fusion.New and the readout's outline router both build from it, so
// a format added here reaches both.
func RouterOptions(timoni extractor.Extractor) []langroute.Option {
	return []langroute.Option{
		langroute.WithCUE(cuelang.NewCUEExtractor()),
		langroute.WithTimoni(timoni),
		langroute.WithFormats(Dispatch),
	}
}
