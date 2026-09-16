package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitInitTempRepo makes dir a real (if empty) git repo, so gitToplevel
// (serviceident.go) can find it as the enclosing work tree for a
// subdirectory cwd — exactly what a Bash command's "cwd" resolves against
// in a real checkout, where the project directory and the git root are
// usually the same thing but a command can still run from a subdirectory.
func gitInitTempRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", "-q", dir)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v\n%s", dir, err, out)
	}
}

func bashPostToolUseStdin(t *testing.T, cwd, command string) {
	t.Helper()
	body := fmt.Sprintf(`{"session_id":"s1","cwd":%q,"tool_name":"Bash","tool_input":{"command":%q}}`, cwd, command)
	hookStdin(t, body)
}

// TestEditHookBashWrite_ConfidentTargetServesGoverningRule pins O57 item
// 4/5(b)'s main case: a Bash command that writes a file via cp — never
// matching Edit|Write|MultiEdit — still serves the rule that governs the
// written path.
func TestEditHookBashWrite_ConfidentTargetServesGoverningRule(t *testing.T) {
	dir := t.TempDir()
	svc := strings.ToLower(filepath.Base(dir))
	srv := fakePromptDaemon(t, map[string]any{
		"internal/store/backend.go": map[string]any{
			"service": svc,
			"file":    "internal/store/backend.go",
			"rules": []any{
				map[string]any{
					"rule": "internal/store stays pure Go — the DuckDB backend was deleted outright (ADR-025); never reintroduce CGO or DuckDB there.",
					"src":  "ADR.md#L814-L814", "tier": "doc-claimed", "layer": "adr-rules",
				},
			},
		},
	}, nil)
	defer srv.Close()

	bashPostToolUseStdin(t, dir, "cp /tmp/draft.go internal/store/backend.go")

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("edit-hook exit = %d, want 0; stderr=%s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "ADR-025") {
		t.Fatalf("output missing ADR-025's rule: %s", out.String())
	}
	if !strings.Contains(out.String(), "internal/store/backend.go") {
		t.Fatalf("output does not name the written path: %s", out.String())
	}
}

// TestEditHookBashWrite_DoubtServesCwdNearestGuide pins O57 item 4/5(b)'s
// doubt path: `git apply` writes something, but the parser cannot name
// what — edit-hook falls back to the cwd's nearest guide rather than
// staying silent.
func TestEditHookBashWrite_DoubtServesCwdNearestGuide(t *testing.T) {
	dir := t.TempDir()
	gitInitTempRepo(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, "internal", "mcp"), 0o755); err != nil {
		t.Fatal(err)
	}
	svc := strings.ToLower(filepath.Base(dir))
	srv := fakePromptDaemon(t, map[string]any{
		"internal/mcp/__bash_write__.go": map[string]any{
			"service": svc,
			"file":    "internal/mcp/__bash_write__.go",
			"rules": []any{
				map[string]any{"rule": "Register every tool in the mcp registry.", "src": "internal/CLAUDE.md#L4-L4", "tier": "doc-claimed", "layer": "obligations"},
			},
		},
	}, nil)
	defer srv.Close()

	bashPostToolUseStdin(t, filepath.Join(dir, "internal", "mcp"), "git apply /tmp/fix.patch")

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("edit-hook exit = %d, want 0; stderr=%s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "Register every tool in the mcp registry.") {
		t.Fatalf("output missing the cwd's nearest-guide rule: %s", out.String())
	}
	if !strings.Contains(out.String(), "nearest guide") {
		t.Fatalf("output does not say it fell back to the nearest guide: %s", out.String())
	}
}

// TestEditHookBashWrite_SilentOnReadOnlyCommand: a Bash command that
// merely reads must stay silent — no beat-worthy output for `go test`.
func TestEditHookBashWrite_SilentOnReadOnlyCommand(t *testing.T) {
	dir := t.TempDir()
	srv := fakePromptDaemon(t, nil, nil)
	defer srv.Close()

	bashPostToolUseStdin(t, dir, "go test ./...")

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("edit-hook exit = %d, want 0", code)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for a read-only Bash command, got %q", out.String())
	}
}
