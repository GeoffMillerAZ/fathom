package semantic

import (
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/filter"
)

// SelectCorpus returns the subset of files whose repo-relative path is
// matched by at least one include glob AND no exclude glob (LC-6,
// docs/plans/layer-catalog.md: #Layer.corpus.exclude was decoded from the
// manifest — LayerDef.CorpusExclude — but never wired into any caller before
// this; spec-bindings' corpus, "docs/plans/**" excluding "README.md", is the
// motivating case). Results are sorted deterministically by path. exclude
// may be nil — a plain include-only selection, unchanged from before this
// parameter existed.
//
// glob semantics are gitignore-style (internal/filter.New): "**" in
// "docs/design/**" matches any path under that directory. include and
// exclude are independent Matchers (not one negation-combined pattern list)
// so a caller's exclude list can never accidentally re-include a path an
// include glob didn't already select.
func SelectCorpus(files []core.SourceFile, include, exclude []string) []core.SourceFile {
	inc := filter.New(include)
	var excl *filter.Matcher
	if len(exclude) > 0 {
		excl = filter.New(exclude)
	}
	var selected []core.SourceFile
	for _, f := range files {
		if !inc.Match(f.Path) {
			continue
		}
		if excl != nil && excl.Match(f.Path) {
			continue
		}
		selected = append(selected, f)
	}
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].Path < selected[j].Path
	})
	return selected
}

// BuildCompileCorpus selects a layer's source files and adds the governing
// rules and root guides the connector does not index. Both init's first-run
// estimate and CompileLayer use this corpus, including identical exclusions.
func BuildCompileCorpus(root string, files []core.SourceFile, layer LayerDef) []core.SourceFile {
	corpus := SelectCorpus(files, layer.CorpusInclude, layer.CorpusExclude)
	if layer.Name == LayerObligations {
		corpus = WithRulesSources(corpus, root, layer.CorpusExclude, nil)
		have := make(map[string]bool, len(corpus))
		for _, f := range corpus {
			have[f.Path] = true
		}
		corpus = append(corpus, RootGuideExtras(root, layer.CorpusInclude, layer.CorpusExclude, have)...)
	}
	sort.Slice(corpus, func(i, j int) bool { return corpus[i].Path < corpus[j].Path })
	return corpus
}
