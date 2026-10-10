package fusion

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestResolveImportSpecs_HTMLLinks (wave lf5, item html; L3): an HTML link is
// a URL, so its query and fragment never name a file, also after a "./" or
// "../" (a relative-path dialect with relative set; every other dialect's
// specs keep their text), and a link to a directory names its index page
// (the html manifest's index_files).
func TestResolveImportSpecs_HTMLLinks(t *testing.T) {
	files := []core.SourceFile{
		{Path: "site/index.html", Content: []byte(`<!DOCTYPE html>
<a href="./about.html#team">a</a>
<a href="../top.html?x=1">t</a>
<a href="docs/">d</a>
<a href="about.html?y#z">a2</a>
<a href="missing.html">m</a>
`)},
		{Path: "site/about.html", Content: []byte("<p>about</p>\n")},
		{Path: "site/docs/index.html", Content: []byte("<p>docs</p>\n")},
		{Path: "top.html", Content: []byte("<p>top</p>\n")},
	}
	ris, rep := ResolveImportSpecs(context.Background(), "html", t.TempDir(), files, nil)
	if rep.Skipped {
		t.Fatalf("skipped: %s", rep.SkipReason)
	}
	got := map[string]string{}
	for _, ri := range ris {
		if ri.File != "site/index.html" {
			continue
		}
		ts := append([]string(nil), ri.Targets...)
		sort.Strings(ts)
		got[ri.Spec.Module] = strings.Join(ts, ",")
	}
	want := map[string]string{
		"./about.html#team": "site/about.html",
		"../top.html?x=1":   "top.html",
		"docs/":             "site/docs/index.html",
		"about.html?y#z":    "site/about.html",
		"missing.html":      "",
	}
	for spec, w := range want {
		if g, ok := got[spec]; !ok || g != w {
			t.Errorf("%q -> %q (present %v), want %q", spec, g, ok, w)
		}
	}
}
