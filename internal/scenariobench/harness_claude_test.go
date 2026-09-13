package scenariobench

import (
	"strings"
	"testing"
)

func TestBuildClaudeArgs_MirrorsContextBenchShape(t *testing.T) {
	req := SessionRequest{
		Prompt:       "do the thing",
		SystemAppend: "keep notes in NOTES.md",
		Arm: Arm{
			AllowedTools:    []string{"mcp__fathom__find_symbol", "mcp__fathom__references"},
			DisallowedTools: []string{"Write", "Bash"},
		},
		MaxTurns: 25,
		Model:    "sonnet",
	}
	args := buildClaudeArgs(req, "/tmp/mcp-config.json")
	joined := strings.Join(args, " ")

	for _, want := range []string{
		"-p do the thing",
		"--output-format stream-json",
		"--verbose",
		"--mcp-config /tmp/mcp-config.json",
		"--strict-mcp-config",
		"--append-system-prompt keep notes in NOTES.md",
		"--allowedTools mcp__fathom__find_symbol mcp__fathom__references",
		"--disallowedTools Bash Write",
		"--max-turns 25",
		"--model sonnet",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("buildClaudeArgs missing %q in %q", want, joined)
		}
	}
}

func TestBuildClaudeArgs_OmitsEmptyOptionalFlags(t *testing.T) {
	req := SessionRequest{Prompt: "p"}
	args := buildClaudeArgs(req, "/tmp/mcp.json")
	joined := strings.Join(args, " ")
	for _, absent := range []string{"--append-system-prompt", "--allowedTools", "--disallowedTools", "--max-turns", "--model"} {
		if strings.Contains(joined, absent) {
			t.Errorf("expected %q to be omitted when unset, got %q", absent, joined)
		}
	}
}

func TestRelativizeSessionPaths(t *testing.T) {
	root := "/repo"
	got := relativizeSessionPaths(root, []string{"/repo/b/file.go", "/repo/a/file.go", "a/already-relative.go", "/repo/a/file.go"})
	want := []string{"a/already-relative.go", "a/file.go", "b/file.go"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRelativizeSessionPaths_KeepsPathsOutsideRoot(t *testing.T) {
	got := relativizeSessionPaths("/repo", []string{"/elsewhere/file.go"})
	if len(got) != 1 || got[0] != "/elsewhere/file.go" {
		t.Errorf("expected an out-of-root path to be kept as-is, got %v", got)
	}
}

func TestRelativizeSessionPaths_Empty(t *testing.T) {
	if got := relativizeSessionPaths("/repo", nil); got != nil {
		t.Errorf("expected nil for no paths, got %v", got)
	}
}
