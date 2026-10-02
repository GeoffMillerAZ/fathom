package connector

import (
	"strings"
	"testing"
)

// sniffRules models §5's ambiguous-extension table with the manifests'
// match blocks (docs/design/language-breadth.md §5): each dialect claims
// its ambiguous extension through match.sniff.
func sniffRules() []DialectRule {
	return []DialectRule{
		{ID: "objective-c", Extensions: []string{".mm"}, Sniff: &DialectSniffRule{
			Extensions: []string{".m"}, Pattern: `@interface|@implementation|#import`, Priority: 0}},
		{ID: "matlab", Sniff: &DialectSniffRule{
			Extensions: []string{".m"}, Pattern: `(?m)^\s*function\b|^\s*%`, Priority: 1}},
		{ID: "perl", Extensions: []string{".pm"}, Shebangs: []string{"perl"}, Sniff: &DialectSniffRule{
			Extensions: []string{".pl"}, Pattern: `^#!.*perl|\buse\s+(strict|warnings)\b|\bsub\s+\w+`, Priority: 0}},
		{ID: "prolog", Extensions: []string{".pro"}, Sniff: &DialectSniffRule{
			Extensions: []string{".pl"}, Pattern: `:-`, Priority: 1}},
		{ID: "fsharp", Extensions: []string{".fsi", ".fsx"}, Sniff: &DialectSniffRule{
			Extensions: []string{".fs"}, Pattern: `\b(let|module|open)\b`, Priority: 0}},
		{ID: "glsl", Extensions: []string{".glsl", ".frag"}, Sniff: &DialectSniffRule{
			Extensions: []string{".fs"}, Pattern: `#version|gl_FragColor`, Priority: 1}},
		{ID: "python", Extensions: []string{".py"}, Shebangs: []string{"python"}},
		{ID: "gitlab-ci", Basenames: []string{".gitlab-ci.yml"}, Globs: []string{".gitlab/ci/**/*.yml"}},
		{ID: "yaml-generic", Extensions: []string{".yml"}},
	}
}

// TestClassifyAmbiguousExtensions pins LB-F9: basename, glob, extension,
// shebang, then the priority-ordered sniff for an extension more than one
// dialect claims; an ambiguous file no sniff settles is left unclassified
// and named as such (never guessed).
func TestClassifyAmbiguousExtensions(t *testing.T) {
	rules := sniffRules()
	cases := []struct {
		path, head  string
		wantDialect string
		wantBy      string
	}{
		{"src/Foo.m", "#import <Foundation/Foundation.h>\n@interface Foo\n@end\n", "objective-c", ClassifiedBySniff},
		{"lab/solve.m", "function y = solve(x)\n  y = x;\nend\n", "matlab", ClassifiedBySniff},
		{"lab/notes.m", "% a MATLAB comment\nx = 1;\n", "matlab", ClassifiedBySniff},
		{"odd/blank.m", "x = 1\n", "", ClassifiedAmbiguous},
		// both sniffs match: the lower priority number wins.
		{"odd/both.m", "@interface X\n% comment\n", "objective-c", ClassifiedBySniff},
		{"bin/tool.pl", "#!/usr/bin/perl\nuse strict;\n", "perl", ClassifiedBySniff},
		{"kb/facts.pl", "parent(a, b).\nancestor(X, Y) :- parent(X, Y).\n", "prolog", ClassifiedBySniff},
		{"app/Main.fs", "module Main\nlet x = 1\n", "fsharp", ClassifiedBySniff},
		{"gfx/shader.fs", "#version 330\nvoid main() { gl_FragColor = vec4(1); }\n", "glsl", ClassifiedBySniff},
		{"app/Lib.fsi", "", "fsharp", ClassifiedByExtension},
		{"x/y.PY", "", "python", ClassifiedByExtension},
		{"scripts/run", "#!/usr/bin/env python3\nprint(1)\n", "python", ClassifiedByShebang},
		{"scripts/tidy", "#!/usr/bin/perl -w\n", "perl", ClassifiedByShebang},
		{"scripts/plain", "echo hi\n", "", ClassifiedNone},
		{".gitlab-ci.yml", "", "gitlab-ci", ClassifiedByBasename},
		{".gitlab/ci/jobs/build.yml", "", "gitlab-ci", ClassifiedByGlob},
		{"other/ci.yml", "", "yaml-generic", ClassifiedByExtension},
		{"README.txt", "", "", ClassifiedNone},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			v := ClassifyDialect(rules, c.path, []byte(c.head))
			if v.Dialect != c.wantDialect || v.By != c.wantBy {
				t.Fatalf("ClassifyDialect(%q) = {%q %q}, want {%q %q}", c.path, v.Dialect, v.By, c.wantDialect, c.wantBy)
			}
		})
	}
}

// TestClassifyDialect_SniffReadsOnly1KiB: the sniff sees the first 1 KiB.
func TestClassifyDialect_SniffReadsOnly1KiB(t *testing.T) {
	head := make([]byte, 0, 2048)
	for len(head) < 1100 {
		head = append(head, "x = 1\n"...)
	}
	head = append(head, "@interface Late\n"...)
	if v := ClassifyDialect(sniffRules(), "a.m", head); v.By != ClassifiedAmbiguous {
		t.Fatalf("marker past 1 KiB classified the file: %+v", v)
	}
}

// TestClassifyDialect_BadSniffPatternIsNoMatch: a pattern that does not
// compile never matches (the manifest test refuses it; classification must
// not panic on it).
func TestClassifyDialect_BadSniffPatternIsNoMatch(t *testing.T) {
	rules := []DialectRule{
		{ID: "a", Sniff: &DialectSniffRule{Extensions: []string{".q"}, Pattern: `(`, Priority: 0}},
		{ID: "b", Sniff: &DialectSniffRule{Extensions: []string{".q"}, Pattern: `x`, Priority: 1}},
	}
	if v := ClassifyDialect(rules, "f.q", []byte("x")); v.Dialect != "b" {
		t.Fatalf("got %+v, want b by sniff", v)
	}
}

// globMatchRef is globMatch before LB-F210: the split-first reference.
func globMatchRef(pattern, p string) bool {
	return globSegs(strings.Split(pattern, "/"), strings.Split(p, "/"))
}

// TestGlobMatchBaseFirstEqualsSplitFirst (LB-F210): refusing a path on its base name before
// splitting never changes an answer, over the shapes manifests use (a "**" in the middle, at
// either end, a bare segment, a malformed bracket) crossed with paths of every depth.
func TestGlobMatchBaseFirstEqualsSplitFirst(t *testing.T) {
	patterns := []string{
		"**/*.yaml", "**/.github/workflows/*.yml", "docs/**", "**", "*.json", "a/*/c", "a/**/c", "a/**/**/c",
		"charts/*/Chart.yaml", "**/k8s/**/*.y*ml", "Dockerfile", "[", "a/[b", "", "/", "a//b", "**/", "x/**/",
		"*", "?", "src/*/main.go", "**/vendor/**/LICENSE*",
	}
	paths := []string{
		"", "a", "a/b", "a/b/c", "a/x/y/c", "a/c", "x.yaml", "d/e/x.yaml", ".github/workflows/ci.yml",
		"r/.github/workflows/ci.yml", "docs", "docs/a/b.md", "charts/x/Chart.yaml", "k8s/a/b/c.yaml",
		"k8s/a.yml", "Dockerfile", "d/Dockerfile", "src/m/main.go", "a//b", "/a", "a/", "vendor/x/y/LICENSE.txt",
		"a/b/", "b",
	}
	for _, g := range patterns {
		for _, p := range paths {
			if got, want := globMatch(g, p), globMatchRef(g, p); got != want {
				t.Errorf("globMatch(%q, %q) = %v, the split-first reference says %v", g, p, got, want)
			}
		}
	}
}
