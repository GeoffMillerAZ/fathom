package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// TestResolveHookHarness pins HP-7: absent ⇒ claude-code; a known
// descriptor without a codec and an unknown id are both I9 fail-open —
// one stderr line, ok=false — never an error the harness could see.
func TestResolveHookHarness(t *testing.T) {
	var errb bytes.Buffer
	c, ok := resolveHookHarness("", &errb)
	if !ok || c.ID() != harness.ClaudeCode || errb.Len() != 0 {
		t.Fatalf("absent flag: codec=%v ok=%v stderr=%q", c, ok, errb.String())
	}
	if c, ok := resolveHookHarness(harness.ClaudeCode, &errb); !ok || c.ID() != harness.ClaudeCode {
		t.Fatalf("explicit claude-code: ok=%v", ok)
	}

	for _, id := range []string{harness.Codex, harness.GeminiCLI} {
		errb.Reset()
		if c, ok := resolveHookHarness(id, &errb); !ok || c.ID() != id || errb.Len() != 0 {
			t.Fatalf("%s: wave-2 codec must resolve silently: ok=%v stderr=%q", id, ok, errb.String())
		}
	}

	errb.Reset()
	if _, ok := resolveHookHarness("cursor", &errb); ok {
		t.Fatal("cursor is a data row with no codec; must not resolve")
	}
	if lines := strings.Count(strings.TrimSpace(errb.String()), "\n") + 1; lines != 1 || !strings.Contains(errb.String(), "not wired") {
		t.Errorf("cursor: want ONE stderr line naming the unwired codec, got %q", errb.String())
	}

	errb.Reset()
	if _, ok := resolveHookHarness("bogus", &errb); ok {
		t.Fatal("unknown id must not resolve")
	}
	if lines := strings.Count(strings.TrimSpace(errb.String()), "\n") + 1; lines != 1 || !strings.Contains(errb.String(), "unknown --harness") {
		t.Errorf("bogus: want ONE stderr line, got %q", errb.String())
	}
}

// TestHookVerbsFailOpenOnUnknownHarness drives every hook verb with
// `--harness bogus`: exit 0, empty stdout, one stderr line (I9).
func TestHookVerbsFailOpenOnUnknownHarness(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FATHOM_HOOKS_LOG", "")
	t.Setenv("FATHOM_CHANNELS", "")
	for _, verb := range []string{"session-hook", "edit-hook", "stop-hook", "session-end-hook", "pre-compact-hook"} {
		t.Run(verb, func(t *testing.T) {
			origStdin := os.Stdin
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			os.Stdin = r
			_, _ = w.WriteString(`{"session_id":"s","cwd":"/tmp"}`)
			_ = w.Close()
			defer func() { os.Stdin = origStdin; _ = r.Close() }()

			var out, errb bytes.Buffer
			code := runWithStdin([]string{verb, "--harness", "bogus"}, r, &out, &errb)
			if code != 0 {
				t.Fatalf("%s --harness bogus exit = %d, want 0 (fail-open)", verb, code)
			}
			if out.Len() != 0 {
				t.Errorf("%s: stdout must be empty, got %q", verb, out.String())
			}
			if n := strings.Count(strings.TrimSpace(errb.String()), "\n") + 1; n != 1 || !strings.Contains(errb.String(), "bogus") {
				t.Errorf("%s: want exactly one stderr line naming the id, got %q", verb, errb.String())
			}
		})
	}
}

// TestHarnessLabelFromRegistry pins HP-14: the lease label and session id
// come from the descriptor's env contract, aliases excluded (the pre-seam
// leases rule: $CLAUDE_SESSION_ID only).
func TestHarnessLabelFromRegistry(t *testing.T) {
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("GEMINI_SESSION_ID", "")
	if label, sid := harnessLabel(); label != "cli" || sid != "" {
		t.Errorf("no session var: got %q/%q, want cli/\"\"", label, sid)
	}
	t.Setenv("CLAUDE_SESSION_ID", "sess-1")
	if label, sid := harnessLabel(); label != "claude" || sid != "sess-1" {
		t.Errorf("CLAUDE_SESSION_ID: got %q/%q, want claude/sess-1", label, sid)
	}
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("GEMINI_SESSION_ID", "g-1")
	if label, sid := harnessLabel(); label != "gemini" || sid != "g-1" {
		t.Errorf("GEMINI_SESSION_ID: got %q/%q, want gemini/g-1", label, sid)
	}
}

func TestHarnessSessionIDAndProjectDir(t *testing.T) {
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "alias-1")
	t.Setenv("GEMINI_SESSION_ID", "")
	if got := harnessSessionID(); got != "alias-1" {
		t.Errorf("alias must resolve: got %q", got)
	}
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CLAUDECODE", "")
	if got := harnessSessionID(); got != "" {
		t.Errorf("no session var: got %q", got)
	}
	if harnessPresent() {
		t.Error("no harness marker set: must not read as present")
	}
	t.Setenv("CLAUDECODE", "1")
	if !harnessPresent() {
		t.Error("CLAUDECODE set: must read as present")
	}

	t.Setenv("CLAUDE_PROJECT_DIR", "/proj")
	if got := harnessProjectDir(harness.ClaudeCode); got != "/proj" {
		t.Errorf("claude-code project dir = %q", got)
	}
	if got := harnessProjectDir(harness.Agents); got != "" {
		t.Errorf("agents declares no project-dir variable, got %q", got)
	}
	if got := anyHarnessProjectDir(); got != "/proj" {
		t.Errorf("anyHarnessProjectDir = %q", got)
	}
	if got := resolveProjDirFor(harness.Agents, "/cwd"); got != "/cwd" {
		t.Errorf("resolveProjDirFor(agents) must fall back to cwd, got %q", got)
	}
}
