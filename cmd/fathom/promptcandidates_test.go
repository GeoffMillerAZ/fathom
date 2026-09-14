package main

import (
	"reflect"
	"testing"
)

func TestExtractPromptCandidates(t *testing.T) {
	cases := []struct {
		name   string
		prompt string
		want   []promptCandidate
	}{
		{
			name:   "path with extension",
			prompt: "please refactor internal/store/backend.go to remove CGO",
			want:   []promptCandidate{{candidatePath, "internal/store/backend.go"}},
		},
		{
			name:   "path wrapped in backticks and trailing punctuation",
			prompt: "look at `cmd/fathom/init.go`, it needs a fix.",
			want:   []promptCandidate{{candidatePath, "cmd/fathom/init.go"}},
		},
		{
			name:   "bare package mention, no extension",
			prompt: "the rules governing internal/mcp look stale",
			want:   []promptCandidate{{candidatePackage, "internal/mcp"}},
		},
		{
			name:   "camelCase symbol",
			prompt: "add a case to registerTool for the new verb",
			want:   []promptCandidate{{candidateSymbol, "registerTool"}},
		},
		{
			name:   "PascalCase and dotted symbols",
			prompt: "ObligationsFor and Store.Get both need review",
			want:   []promptCandidate{{candidateSymbol, "ObligationsFor"}, {candidateSymbol, "Store.Get"}},
		},
		{
			name:   "snake_case symbol",
			prompt: "the max_rules argument is ignored",
			want:   []promptCandidate{{candidateSymbol, "max_rules"}},
		},
		{
			name:   "plain English prose has no candidates",
			prompt: "please make the tests pass and fix the bug",
			want:   nil,
		},
		{
			name:   "a URL is never a path or package candidate",
			prompt: "see https://example.com/docs/setup.md for context",
			want:   nil,
		},
		{
			name:   "duplicates collapse to one",
			prompt: "internal/store/backend.go and internal/store/backend.go again",
			want:   []promptCandidate{{candidatePath, "internal/store/backend.go"}},
		},
		{
			name: "capped at maxPromptCandidates",
			prompt: "internal/a/a.go internal/b/b.go internal/c/c.go internal/d/d.go " +
				"internal/e/e.go internal/f/f.go",
			want: []promptCandidate{
				{candidatePath, "internal/a/a.go"},
				{candidatePath, "internal/b/b.go"},
				{candidatePath, "internal/c/c.go"},
				{candidatePath, "internal/d/d.go"},
				{candidatePath, "internal/e/e.go"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractPromptCandidates(tc.prompt)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("extractPromptCandidates(%q) = %+v, want %+v", tc.prompt, got, tc.want)
			}
		})
	}
}
