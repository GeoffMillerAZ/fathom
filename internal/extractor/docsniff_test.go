package extractor

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestSniffLiterals(t *testing.T) {
	cases := []struct {
		pattern string
		want    []string // nil: no promise
	}{
		{`(?m)^apiVersion:[ \t]*x`, []string{"apiVersion:"}},
		{`foo|bar`, []string{"bar", "foo"}},
		{`foo|b.r`, []string{"b", "foo"}}, // b.r still needs its b
		{`foo|.*`, nil},                   // an alternative with no literal breaks the promise
		{`(?i)foo`, nil},                  // case folding: bytes.Contains would miss FOO
		{`a?foo`, []string{"foo"}},        // the optional part is not required, foo is
		{`(?:foo)?`, nil},                 // nothing is required
		{`x*`, nil},
		{`(?s)(?m:^policies[ \t]*:).*(?m:^[ \t]+resource[ \t]*:)`, []string{"resource", "policies"}},
		{`"(?:openapi|swagger)"`, []string{"openapi", "swagger"}},
		{`[ab]cde`, []string{"cde"}},
		{`(ab){2}`, []string{"ab"}},
		{`(ab){0,2}x`, []string{"x"}},
	}
	for _, c := range cases {
		got := sniffLiterals(c.pattern)
		if c.pattern == `(?s)(?m:^policies[ \t]*:).*(?m:^[ \t]+resource[ \t]*:)` {
			// the sharper of two equal-length-free literals: "policies" (8) over "resource" (8) is a tie by
			// length, so the first wins; either is a sound promise.
			if len(got) != 1 || (got[0] != "policies" && got[0] != "resource") {
				t.Errorf("%s: got %q", c.pattern, got)
			}
			continue
		}
		sort.Strings(got)
		want := append([]string(nil), c.want...)
		sort.Strings(want)
		if strings.Join(got, "\x00") != strings.Join(want, "\x00") || (got == nil) != (c.want == nil) {
			t.Errorf("%s: got %q want %q", c.pattern, got, c.want)
		}
	}
}

// TestSniffPrefilterNeverChangesAnAnswer (LB-F209) runs every doc dialect's sniff with and
// without the literal prefilter over every .json, .yaml and .yml file of the repository
// (the dialects' own fixtures and goldens, the docs) and a set of synthetic documents:
// the answers are equal, and the prefilter is present where the plan says it is needed.
func TestSniffPrefilterNeverChangesAnAnswer(t *testing.T) {
	dialects, err := LoadDialects()
	if err != nil {
		t.Fatal(err)
	}
	root := "../.."
	var files [][]byte
	for _, sub := range []string{"internal", "docs", "calibration", "testdata"} {
		_ = filepath.Walk(filepath.Join(root, sub), func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || info.Size() > 1<<20 {
				return nil
			}
			switch filepath.Ext(p) {
			case ".json", ".yaml", ".yml", ".tf", ".txt":
				if b, err := os.ReadFile(p); err == nil {
					files = append(files, b)
				}
			}
			return nil
		})
	}
	if len(files) > 250 { // every k-th: the check is a regexp run per (dialect, file)
		k := len(files) / 250
		var kept [][]byte
		for i := 0; i < len(files); i += k {
			kept = append(kept, files[i])
		}
		files = kept
	}
	files = append(files,
		[]byte(""), []byte("{}"), []byte("AWSTemplateFormatVersion: '2010-09-09'\nResources: {}\n"),
		[]byte(`{"openapi":"3.0.0"}`), []byte("swagger: '2.0'\n"), []byte(`{"Version":"2012-10-17","Statement":[]}`),
		[]byte("Version: 2012-10-17\nStatement:\n  - Effect: Allow\n"), []byte(`{"policyRule":{"if":{},"then":{"effect":"deny"}}}`),
		[]byte("policies:\n  - name: x\n    resource: ec2\n"), []byte("kind: Deployment\n"),
		[]byte("APIVERSION: v1 OPENAPI SWAGGER POLICIES RESOURCE"))
	if len(files) < 100 {
		t.Fatalf("corpus too small to mean anything: %d files", len(files))
	}
	guarded, sniffed, decided := 0, 0, 0
	for _, d := range dialects {
		if d.Engine != "doc" && d.Engine != "tags+doc" {
			continue
		}
		x, err := LoadDocExtractor(d)
		if err != nil || x.sniff == nil {
			continue
		}
		sniffed++
		if x.sniffLits != nil {
			guarded++
		}
		for _, src := range files {
			want := x.sniff.Match(src)
			if got := x.DetectsContent(src); got != want {
				t.Fatalf("%s: DetectsContent = %v, the pattern says %v", d.ID, got, want)
			}
			if x.sniffLits != nil && !containsAny(src, x.sniffLits) {
				decided++
			}
		}
	}
	t.Logf("%d of %d sniffing dialects carry a prefilter, it decided %d file checks", guarded, sniffed, decided)
	if sniffed < 10 || guarded < sniffed*3/4 {
		t.Errorf("only %d of %d sniffing dialects carry a prefilter: the analyzer has lost the shapes the manifests use", guarded, sniffed)
	}
	if decided == 0 {
		t.Error("the prefilter decided no file: the corpus does not exercise it")
	}
}
