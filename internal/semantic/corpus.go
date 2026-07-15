package semantic

import (
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/filter"
)

// ObligationsInclude lists the repo-relative globs that select the
// obligations corpus. Mirror of schema/semantic/schema.cue
// obligations.corpus.include — keep in sync when the schema changes.
var ObligationsInclude = []string{
	"CLAUDE.md",
	"ADR.md",
	"docs/plans/README.md",
	// Deliberately NOT docs/design/**: the design essays are program specs
	// full of aspirational/metric statements ("achieve precision ≥0.90"),
	// which the first fleet compile turned into noisy every-edit rules —
	// the exact trust-fatal failure ADR-016 §4.6 warns about. Obligations
	// compile only from documents that BIND the editor of a file.
}

// SelectCorpus returns the subset of files whose repo-relative path is
// matched by at least one include glob. Results are sorted deterministically
// by path. files not matched by any glob are omitted.
//
// glob semantics are gitignore-style (internal/filter.New): "**" in
// "docs/design/**" matches any path under that directory.
func SelectCorpus(files []core.SourceFile, include []string) []core.SourceFile {
	m := filter.New(include)
	var selected []core.SourceFile
	for _, f := range files {
		if m.Match(f.Path) {
			selected = append(selected, f)
		}
	}
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].Path < selected[j].Path
	})
	return selected
}
