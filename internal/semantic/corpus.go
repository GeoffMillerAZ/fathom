package semantic

import (
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/filter"
)

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
