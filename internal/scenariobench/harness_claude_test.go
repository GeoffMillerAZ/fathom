package scenariobench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildClaudeArgs_MirrorsContextBenchShape(t *testing.T) {
	req := SessionRequest{
		Prompt:          "do the thing",
		SystemAppend:    "keep notes in NOTES.md",
		AllowedTools:    []string{"mcp__fathom__find_symbol", "mcp__fathom__references"},
		DisallowedTools: []string{"Write", "Bash"},
		MaxTurns:        25,
		Model:           "sonnet",
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

// writeFakeClaudeScript writes a `/bin/sh` script standing in for `claude`:
// it records its own arguments (space-joined) to argsOut and emits one
// minimal valid stream-json result event, so RunSession's own plumbing
// (never a real session) can be exercised with no spend.
func writeFakeClaudeScript(t *testing.T, dir, argsOut string) string {
	t.Helper()
	path := filepath.Join(dir, "fake-claude.sh")
	script := "#!/bin/sh\n" +
		"echo \"$@\" > \"" + argsOut + "\"\n" +
		`echo '{"type":"result","is_error":false,"num_turns":1,"duration_ms":1,"total_cost_usd":0,"usage":{"input_tokens":1,"output_tokens":1}}'` + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestClaudeHarness_RunSession_PropagatesRenderedMCPConfig is the DEFECT
// fix's own proof: RunSession must copy req.MCPConfigPath (RenderArm's
// rendered config, carrying alwaysLoad/fathom-merged entries no
// RunSession-authored config would) into RawDir under the existing
// provenance filename, and pass THAT copy's path via --mcp-config — never
// re-derive a config from req.Arm.MCP, which a wired cell's caller never
// updates with the merged fathom entry (subject.go's renderedArm is a
// local copy PrepareSubject never hands back).
func TestClaudeHarness_RunSession_PropagatesRenderedMCPConfig(t *testing.T) {
	dir := t.TempDir()
	worktree := t.TempDir()
	rawDir := filepath.Join(dir, "raw")

	renderedPath := filepath.Join(dir, "scenariobench-mcp.json")
	renderedContent := `{"mcpServers":{"fathom":{"alwaysLoad":true,"type":"http","url":"http://127.0.0.1:1234/rpc/agent"}}}` + "\n"
	if err := os.WriteFile(renderedPath, []byte(renderedContent), 0o644); err != nil {
		t.Fatal(err)
	}

	argsOut := filepath.Join(dir, "args.txt")
	fakeBin := writeFakeClaudeScript(t, dir, argsOut)

	h := ClaudeHarness{Bin: fakeBin}
	req := SessionRequest{
		Worktree:      worktree,
		Prompt:        "do the thing",
		RawDir:        rawDir,
		Index:         0,
		MCPConfigPath: renderedPath,
	}
	if _, err := h.RunSession(req); err != nil {
		t.Fatalf("RunSession: %v", err)
	}

	copyPath := filepath.Join(rawDir, "session-0-mcp.json")
	copied, err := os.ReadFile(copyPath)
	if err != nil {
		t.Fatalf("expected the rendered config copied into RawDir: %v", err)
	}
	if string(copied) != renderedContent {
		t.Errorf("copied MCP config = %q, want %q", copied, renderedContent)
	}

	argsRaw, err := os.ReadFile(argsOut)
	if err != nil {
		t.Fatalf("reading recorded args: %v", err)
	}
	if !strings.Contains(string(argsRaw), "--mcp-config "+copyPath) {
		t.Errorf("expected --mcp-config to point at the copied path %s, got %s", copyPath, argsRaw)
	}
}

// TestClaudeHarness_RunSession_FallsBackToArmMCPWhenConfigPathEmpty covers
// the pre-existing behaviour for a SessionRequest with no rendered config
// (e.g. built by hand in a test): RunSession still writes req.Arm.MCP
// verbatim, as before this change.
func TestClaudeHarness_RunSession_FallsBackToArmMCPWhenConfigPathEmpty(t *testing.T) {
	dir := t.TempDir()
	worktree := t.TempDir()
	rawDir := filepath.Join(dir, "raw")
	argsOut := filepath.Join(dir, "args.txt")
	fakeBin := writeFakeClaudeScript(t, dir, argsOut)

	h := ClaudeHarness{Bin: fakeBin}
	req := SessionRequest{
		Worktree: worktree,
		Prompt:   "do the thing",
		RawDir:   rawDir,
		Index:    0,
		Arm:      Arm{MCP: map[string]any{"fathom": map[string]any{"type": "http"}}},
	}
	if _, err := h.RunSession(req); err != nil {
		t.Fatalf("RunSession: %v", err)
	}

	copyPath := filepath.Join(rawDir, "session-0-mcp.json")
	copied, err := os.ReadFile(copyPath)
	if err != nil {
		t.Fatalf("expected an mcp config written from req.Arm.MCP: %v", err)
	}
	if !strings.Contains(string(copied), `"fathom"`) {
		t.Errorf("expected the fallback config to carry req.Arm.MCP, got %s", copied)
	}
}
