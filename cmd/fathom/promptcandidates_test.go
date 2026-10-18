package main

import (
	"os"
	"path/filepath"
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
			want:   []promptCandidate{{candidatePath, "internal/store/backend.go", false}},
		},
		{
			name:   "path wrapped in backticks and trailing punctuation",
			prompt: "look at `cmd/fathom/init.go`, it needs a fix.",
			want:   []promptCandidate{{candidatePath, "cmd/fathom/init.go", false}},
		},
		{
			name:   "bare package mention, no extension",
			prompt: "the rules governing internal/mcp look stale",
			want:   []promptCandidate{{candidatePackage, "internal/mcp", false}},
		},
		{
			name:   "camelCase symbol",
			prompt: "add a case to registerTool for the new verb",
			want:   []promptCandidate{{candidateSymbol, "registerTool", false}},
		},
		{
			name:   "PascalCase and dotted symbols",
			prompt: "ObligationsFor and Store.Get both need review",
			want:   []promptCandidate{{candidateSymbol, "ObligationsFor", false}, {candidateSymbol, "Store.Get", false}},
		},
		{
			name:   "snake_case symbol",
			prompt: "the max_rules argument is ignored",
			want:   []promptCandidate{{candidateSymbol, "max_rules", false}},
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
			want:   []promptCandidate{{candidatePath, "internal/store/backend.go", false}},
		},
		{
			name:   "a plain capitalized sentence word is not a symbol; only the backticked, dotted reference is",
			prompt: "Update the Edge struct in `core.Edge`",
			want:   []promptCandidate{{candidateSymbol, "core.Edge", true}},
		},
		{
			name:   "a bare filename with no directory prefix is a path candidate, not a symbol",
			prompt: "does README.md still describe this correctly?",
			want:   []promptCandidate{{candidatePath, "README.md", false}},
		},
		{
			name: "capped at maxPromptCandidates",
			prompt: "internal/a/a.go internal/b/b.go internal/c/c.go internal/d/d.go " +
				"internal/e/e.go internal/f/f.go",
			want: []promptCandidate{
				{candidatePath, "internal/a/a.go", false},
				{candidatePath, "internal/b/b.go", false},
				{candidatePath, "internal/c/c.go", false},
				{candidatePath, "internal/d/d.go", false},
				{candidatePath, "internal/e/e.go", false},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractPromptCandidates(tc.prompt, nil)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("extractPromptCandidates(%q) = %+v, want %+v", tc.prompt, got, tc.want)
			}
		})
	}
}

// touchTree writes an empty file at every rel under root, creating the
// directories, and returns root.
func touchTree(t *testing.T, root string, rels ...string) string {
	t.Helper()
	for _, rel := range rels {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// EW-21 / O126: a path or package token that names nothing in the working
// tree is dropped BEFORE it takes a candidate slot: a build artifact
// (bin/fathom, a file, never a directory), a scratch path outside the tree,
// a `~/.claude` file, a path that climbs out, and a file or directory that
// does not exist. Each of these used to reach obligations, which answers
// every path with the root-scope rules.
func TestPromptCandidatesDropNonexistentPaths(t *testing.T) {
	root := touchTree(t, t.TempDir(),
		"internal/store/backend.go", "docs/plans/README.md", "README.md", "bin/fathom", "cmd/fathom/main.go")
	names := workingTreeNames([]string{root})

	cases := []struct {
		name   string
		prompt string
		want   []promptCandidate
	}{
		{
			name:   "an existing file and directory are kept",
			prompt: "read internal/store/backend.go and the rules for internal/store",
			want:   []promptCandidate{{candidatePath, "internal/store/backend.go", false}, {candidatePackage, "internal/store", false}},
		},
		{
			name:   "a file that does not exist is dropped",
			prompt: "fix internal/store/ghost.go and schema/x.json",
			want:   nil,
		},
		{
			name:   "a directory that does not exist is dropped",
			prompt: "the rules for internal/ghost and internal/store/nothing",
			want:   nil,
		},
		{
			name:   "a build artifact that is a file is never a package (bin/fathom/__prompt__.go)",
			prompt: "run bin/fathom verify --strict",
			want:   nil,
		},
		{
			name:   "a scratch path outside every root is dropped",
			prompt: "see /private/tmp/claude-501/scratchpad/genv-ew.sh and /tmp/x/y.go",
			want:   nil,
		},
		{
			name:   "a home-relative file is dropped",
			prompt: "append to ~/.claude/projects/p/memory/MEMORY.md",
			want:   nil,
		},
		{
			name:   "a path that climbs out of the root is dropped",
			prompt: "compare with ../other/repo/internal/store/backend.go",
			want:   nil,
		},
		{
			name:   "an absolute path inside the root is kept, repo-relative, and deduplicated with its relative spelling",
			prompt: "open " + filepath.Join(root, "internal", "store", "backend.go") + " and internal/store/backend.go",
			want:   []promptCandidate{{candidatePath, "internal/store/backend.go", false}},
		},
		{
			name:   "a bare filename at the root is kept when it exists there",
			prompt: "does README.md still describe this, and CHANGELOG.md?",
			want:   []promptCandidate{{candidatePath, "README.md", false}},
		},
		{
			name:   "symbols are not filtered by the working tree",
			prompt: "add a case to registerTool for the new verb",
			want:   []promptCandidate{{candidateSymbol, "registerTool", false}},
		},
		{
			name: "tokens that name nothing do not take a slot: the real file after six ghosts is still found",
			prompt: "internal/a/a.go internal/b/b.go internal/c/c.go internal/d/d.go " +
				"internal/e/e.go internal/f/f.go and finally internal/store/backend.go",
			want: []promptCandidate{{candidatePath, "internal/store/backend.go", false}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractPromptCandidates(tc.prompt, names)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("extractPromptCandidates(%q) = %+v, want %+v", tc.prompt, got, tc.want)
			}
		})
	}
}
