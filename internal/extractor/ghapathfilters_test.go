package extractor

import (
	"slices"
	"testing"
)

// Cases adapted from GitHub's path examples and filter pattern cheat sheet:
// https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#filter-pattern-cheat-sheet
// https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#onpushpull_requestpull_request_targetpathspaths-ignore
func TestGitHubPathPatternConformance(t *testing.T) {
	for _, tt := range []struct {
		pattern string
		yes, no []string
	}{
		{"*", []string{"README.md", "server.rb"}, []string{"docs/README.md"}},
		{"**", []string{"README.md", "all/the/files.md"}, nil},
		{"*.jsx?", []string{"page.js", "page.jsx"}, []string{"page.j", "page.jsxx", "src/page.js"}},
		{"**.js", []string{"index.js", "src/js/app.js"}, []string{"index.jsx"}},
		{"docs/*", []string{"docs/file.txt"}, []string{"a/docs/file.txt", "docs/a/file.txt"}},
		{"docs/**", []string{"docs/a/file.txt"}, []string{"a/docs/file.txt"}},
		{"docs/**/*.md", []string{"docs/README.md", "docs/a/markdown/file.md"}, []string{"a/docs/README.md"}},
		{"**/docs/**", []string{"docs/hello.md", "space/docs/plan/space.doc"}, []string{"documentation/hello.md"}},
		{"**/README.md", []string{"README.md", "js/README.md"}, []string{"otherREADME.md"}},
		{"**/*src/**", []string{"a/src/app.js", "my-src/code/js/app.js"}, []string{"a/srcapp.js"}},
		{"v[12].[0-9]+.[0-9]+", []string{"v1.10.1", "v2.0.0"}, []string{"v3.0.0", "v1..0", "v1.0.a"}},
		{"[CB]at[0-9a-zA-Z]", []string{"Cat0", "BatZ"}, []string{"Rat0", "Cat_"}},
		{`literal\*\?\+\[\]\!`, []string{"literal*?+[]!"}, nil},
		{`\!README.md`, []string{"!README.md"}, []string{"README.md"}},
		{"file!.txt", []string{"file!.txt"}, []string{"file.txt"}},
	} {
		t.Run(tt.pattern, func(t *testing.T) {
			p, err := compileGitHubPathPattern(tt.pattern)
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range tt.yes {
				if !p.Match(path) {
					t.Errorf("%q should match", path)
				}
			}
			for _, path := range tt.no {
				if p.Match(path) {
					t.Errorf("%q should not match", path)
				}
			}
		})
	}
}

func TestReadGitHubPathFilters(t *testing.T) {
	got, err := ReadGitHubPathFilters([]byte(`on:
  pull_request:
    paths-ignore: ['docs/**']
  push:
    paths: ['*.md', '!README.md', 'README*']
jobs:
  normal: {runs-on: ubuntu-latest}
  enabled: {if: true}
  disabled: {if: false}
  explicit: {if: '${{ github.event_name == "push" }}'}
  implicit: {if: github.ref == 'refs/heads/main'}
  upstream: {needs: normal}
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Events) != 2 || got.Events[0].Name != "pull_request" || got.Events[1].Name != "push" {
		t.Fatalf("event order: %+v", got.Events)
	}
	ignore, paths := got.Events[0], got.Events[1]
	if ignore.Includes("docs/file.md") || !ignore.Includes("app.go") ||
		!paths.Includes("README.md") || !paths.Includes("notes.md") || paths.Includes("docs/notes.md") {
		t.Fatal("paths ordering or paths-ignore inversion is wrong")
	}
	if names := []string{paths.Patterns[0].Text, paths.Patterns[1].Text, paths.Patterns[2].Text}; !slices.Equal(names, []string{"*.md", "!README.md", "README*"}) {
		t.Fatalf("pattern order: %v", names)
	}
	for _, job := range []string{"normal", "enabled", "disabled", "explicit", "implicit", "upstream"} {
		condition, ok := got.Jobs[job]
		if !ok || (condition == "") != (job == "normal" || job == "enabled") {
			t.Errorf("job %q: condition=%q present=%t", job, condition, ok)
		}
	}
	for _, src := range []string{"push", "[push, workflow_dispatch, pull_request]", "{push: null}", "{push: {}}", "{push: }"} {
		workflow, err := ReadGitHubPathFilters([]byte("on: " + src + "\n"))
		if err != nil || len(workflow.Events) == 0 || !workflow.Events[0].Includes("anything/here") {
			t.Errorf("on %s: %+v, %v", src, workflow, err)
		}
	}
}

func TestGitHubPathFiltersRefuseAmbiguity(t *testing.T) {
	for _, pattern := range []string{"", "!", "/tmp/**", "C:/tmp/**", "[", "[]", "[a-Z]", "[z-a]", "[a-]", "[!x]", "+a", "x??", "x++", "x***", "{a,b}", `abc\`, "${{ inputs.paths }}"} {
		if _, err := compileGitHubPathPattern(pattern); err == nil {
			t.Errorf("accepted %q", pattern)
		}
	}
	for _, src := range []string{
		"",
		"on: null",
		"on: [push, push]",
		"on: [push, {}]",
		"on: {push: [paths]}",
		"on: {push: {paths: true}}",
		"on: {push: {paths: [true]}}",
		"on: {push: {paths: []}}",
		"on: {push: {paths: ['!*.md']}}",
		"on: {push: {paths-ignore: ['!*.md']}}",
		"on: {push: {paths: ['**'], paths-ignore: ['docs/**']}}",
		"on: {push: {paths: ['**'], paths: ['*.js']}}",
		"on: push\non: pull_request\n",
		"on: push\n---\non: pull_request\n",
		"on: push\njobs: {build: null}",
		"on: push\njobs: {build: {if: [true]}}",
	} {
		if _, err := ReadGitHubPathFilters([]byte(src)); err == nil {
			t.Errorf("accepted %q", src)
		}
	}
}
