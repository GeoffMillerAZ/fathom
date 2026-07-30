package main

// userguard_test.go — scoped config (docs/design/scoped-config.md
// deliverable 2): ~/.fathom/guard.cue's command-guard consultation and
// [user]-tagged rendering.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeUserGuardCue(t *testing.T, home, content string) {
	t.Helper()
	dir := filepath.Join(home, ".fathom")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "guard.cue"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadUserGuardSet_Absent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, ok := loadUserGuardSet(); ok {
		t.Fatal("expected no user guard set when ~/.fathom/guard.cue is absent")
	}
}

func TestLoadUserGuardSet_Valid(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeUserGuardCue(t, home, `guards: [
	{
		note: "confirm before an apply"
		command_scopes: ["terraform apply*"]
	},
]`)
	gs, ok := loadUserGuardSet()
	if !ok {
		t.Fatal("expected the user guard set to load")
	}
	if len(gs.Guards) != 1 || gs.Guards[0].Note != "[user] confirm before an apply" {
		t.Fatalf("gs = %+v", gs)
	}
}

func TestLoadUserGuardSet_Malformed_FailsOpen(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeUserGuardCue(t, home, `guards: [
	{
		scopes: ["internal/store/**"]
	},
]`)
	if _, ok := loadUserGuardSet(); ok {
		t.Fatal("expected loadUserGuardSet to fail open on a governance-field violation")
	}
}

// TestEditHookPre_UserScopeCommandGuard_Matches proves the user-scope
// guard fires for a Bash command with no project guard.cue at all — zero
// daemon calls (command guards never make one — O-9's decision, unchanged
// here), note rendered with its "[user] " tag.
func TestEditHookPre_UserScopeCommandGuard_Matches(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeUserGuardCue(t, home, `guards: [
	{
		note: "double-check this destroys real infra"
		command_scopes: ["terraform apply*"]
	},
]`)
	dir := t.TempDir()

	hookInput := fmt.Sprintf(`{"tool_name":"Bash","tool_input":{"command":"terraform apply -auto-approve"},"cwd":%q}`, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--pre", "--url=http://127.0.0.1:1"}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}

	var payload struct {
		HookSpecificOutput struct {
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("output = %q, unmarshal: %v", out.String(), err)
	}
	got := payload.HookSpecificOutput.AdditionalContext
	if want := "[user] double-check this destroys real infra"; !strings.Contains(got, want) {
		t.Errorf("additionalContext = %q, want it to contain %q", got, want)
	}
}

// TestEditHookPre_ProjectGuardWinsOverUserScope proves law 13: when BOTH a
// project guard and a user-scope guard match the same command, the project
// guard's note is what renders (project wins on conflict) and the
// user-scope loader is never even consulted.
func TestEditHookPre_ProjectGuardWinsOverUserScope(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeUserGuardCue(t, home, `guards: [
	{
		note: "user note — must not appear"
		command_scopes: ["terraform apply*"]
	},
]`)
	dir := t.TempDir()
	writeGuardCue(t, dir, `guards: [
	{
		note: "project note wins"
		command_scopes: ["terraform apply*"]
	},
]`)

	hookInput := fmt.Sprintf(`{"tool_name":"Bash","tool_input":{"command":"terraform apply -auto-approve"},"cwd":%q}`, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--pre", "--url=http://127.0.0.1:1"}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if strings.Contains(out.String(), "user note") {
		t.Errorf("project guard must win — got %q", out.String())
	}
	if !strings.Contains(out.String(), "project note wins") {
		t.Errorf("expected the project guard's note, got %q", out.String())
	}
}
