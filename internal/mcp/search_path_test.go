package mcp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/filter"
)

func searchPathFiles(hits []searchRawHit) []string {
	seen := map[string]bool{}
	for _, h := range hits {
		seen[h.File] = true
	}
	var out []string
	for f := range seen {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

func searchPathFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFlagFixture(t, root, ".gitignore", "pkg/.cache/\n")
	writeFlagFixture(t, root, "pkg/src/a.js", "needle\n")
	writeFlagFixture(t, root, "pkg/src/deep/b.js", "needle\n")
	writeFlagFixture(t, root, "pkg/.cache/c.js", "needle\n")
	writeFlagFixture(t, root, "other/d.js", "needle\n")
	return root
}

// O1448: a directory path is the search root, a glob path is applied to
// returned paths, and a glob never reaches a gitignored tree.
func TestSearchPathDirectoryIsTheRoot(t *testing.T) {
	root := searchPathFixture(t)
	ignore := filter.New([]string{"pkg/.cache/"})
	cases := []struct {
		path string
		want []string
	}{
		{"pkg", []string{"pkg/src/a.js", "pkg/src/deep/b.js"}},
		{"pkg/", []string{"pkg/src/a.js", "pkg/src/deep/b.js"}},
		{"pkg/src", []string{"pkg/src/a.js", "pkg/src/deep/b.js"}},
		{"pkg/**", []string{"pkg/src/a.js", "pkg/src/deep/b.js"}},
		{"pkg/src/*.js", []string{"pkg/src/a.js"}},
		{"pkg/src/a.js", []string{"pkg/src/a.js"}},
		{"../other", nil},
	}
	engines := map[string]func(string, searchOpts) ([]searchRawHit, error){
		"ripgrep": func(r string, o searchOpts) ([]searchRawHit, error) {
			h, _, err := searchRipgrep(context.Background(), r, o)
			return h, err
		},
		"walk": func(r string, o searchOpts) ([]searchRawHit, error) {
			o.ignore = ignore
			h, _, err := searchWalk(context.Background(), r, o)
			return h, err
		},
	}
	for name, run := range engines {
		if name == "ripgrep" {
			if _, err := exec.LookPath("rg"); err != nil {
				continue
			}
		}
		for _, c := range cases {
			hits, err := run(root, searchOpts{pattern: "needle", caseSensitive: true, pathGlob: c.path})
			if err != nil {
				t.Fatalf("%s %q: %v", name, c.path, err)
			}
			got := searchPathFiles(hits)
			if len(got) != len(c.want) {
				t.Fatalf("%s path %q: got %v want %v", name, c.path, got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("%s path %q: got %v want %v", name, c.path, got, c.want)
				}
			}
		}
	}
}

func TestSplitSearchPath(t *testing.T) {
	cases := []struct{ in, dir, pat string }{
		{"", "", ""},
		{"src", "src", ""},
		{"src/", "src", ""},
		{"./src/x", "src/x", ""},
		{"src/**", "src", "src/**"},
		{"*.go", "", "*.go"},
		{"a/b/*.go", "a/b", "a/b/*.go"},
		{"../x", "", "../x"},
		{"/abs", "", "/abs"},
	}
	for _, c := range cases {
		d, p := splitSearchPath(c.in)
		if d != c.dir || p != c.pat {
			t.Errorf("%q: got (%q,%q) want (%q,%q)", c.in, d, p, c.dir, c.pat)
		}
	}
}
