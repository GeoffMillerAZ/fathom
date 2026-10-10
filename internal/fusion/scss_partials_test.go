package fusion

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// TestResolveRelativePath_SassPartials (LB-F465): with partial_prefix "_"
// relative-path follows Sass's load rule. `@use "x"` and `@import "dir/x"`
// name `x.scss` or the partial `_x.scss` beside the importer, or a directory's
// `_index.scss` or `index.scss`; a plain file wins over its partial only when
// the partial is absent (Sass refuses a load both name), and without the
// parameter a partial is never tried.
func TestResolveRelativePath_SassPartials(t *testing.T) {
	sass := extractor.ImportParams{Relative: true, Suffixes: []string{".scss", ".css"}, IndexFiles: []string{"_index.scss", "index.scss"}, PartialPrefix: "_"}
	tree := []string{
		"scss/bootstrap.scss", "scss/_variables.scss", "scss/mixins/_banner.scss",
		"scss/forms/_index.scss", "scss/utilities/index.scss", "scss/plain.scss",
		"src/support/index.scss", "src/base/base.scss",
	}
	for _, c := range []struct {
		name     string
		params   extractor.ImportParams
		importer string
		module   string
		want     []string
	}{
		{"partial beside", sass, "scss/bootstrap.scss", "variables", []string{"scss/_variables.scss"}},
		{"partial in a subdirectory", sass, "scss/bootstrap.scss", "mixins/banner", []string{"scss/mixins/_banner.scss"}},
		{"partial written with its extension", sass, "scss/bootstrap.scss", "mixins/banner.scss", []string{"scss/mixins/_banner.scss"}},
		{"the _index partial", sass, "scss/bootstrap.scss", "forms", []string{"scss/forms/_index.scss"}},
		{"an index file", sass, "scss/bootstrap.scss", "utilities", []string{"scss/utilities/index.scss"}},
		{"a plain file", sass, "scss/bootstrap.scss", "plain", []string{"scss/plain.scss"}},
		{"dot-dot up a level", sass, "src/base/base.scss", "../support", []string{"src/support/index.scss"}},
		{"a built-in module names no file", sass, "scss/bootstrap.scss", "sass:math", nil},
		{"no partial rule without the parameter", extractor.ImportParams{Relative: true, Suffixes: []string{".scss", ".css"}}, "scss/bootstrap.scss", "variables", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			env := newTestEnv("/repo", tree, nil)
			if got := resolveByStrategy("relative-path", c.params, env, c.importer, c.module); !reflect.DeepEqual(got, c.want) {
				t.Errorf("resolve %q from %s = %q, want %q", c.module, c.importer, got, c.want)
			}
		})
	}
}
