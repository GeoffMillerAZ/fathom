package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReconcileDiff pins IO-17: the dry-run names exactly what drifted —
// missing template entries and stale absolute binary paths — per repo.
func TestReconcileDiff(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home) // user scope empty
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".claude"), 0o755)
	// The audit's old shape: Edit-only PostToolUse, absolute stale path,
	// no Grep|Glob|Read observer, no Stop.
	os.WriteFile(filepath.Join(root, ".claude", "settings.json"), []byte(`{
  "hooks": {
    "PostToolUse": [{"matcher": "Edit|Write|MultiEdit", "hooks": [{"type": "command", "command": "/Volumes/x/sextant/bin/fathom edit-hook", "timeout": 5}]}]
  }
}`), 0o644)

	d := analyzeWiring(root)
	if len(d.StaleCmds) != 1 || !strings.Contains(d.StaleCmds[0], "/Volumes/x/sextant/bin/fathom") {
		t.Errorf("stale = %v", d.StaleCmds)
	}
	joined := strings.Join(d.Missing, "\n")
	for _, want := range []string{"SessionStart", "Grep|Glob|Read", "Stop"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing should include %s, got:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "PostToolUse[Edit|Write|MultiEdit]") {
		t.Errorf("present entry misreported missing:\n%s", joined)
	}
}

// TestReconcileApply pins IO-18: --apply repoints stale commands to the
// shim and merges the missing template entries — then a second analyze
// reports clean (reconciled, not scaffolded).
func TestReconcileApply(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".claude"), 0o755)
	settings := filepath.Join(root, ".claude", "settings.json")
	os.WriteFile(settings, []byte(`{
  "hooks": {
    "PostToolUse": [{"matcher": "Edit|Write|MultiEdit", "hooks": [{"type": "command", "command": "/Volumes/x/sextant/bin/fathom edit-hook", "timeout": 5}]}]
  }
}`), 0o644)

	var stdout, stderr strings.Builder
	code := cmdFleetReconcile([]string{"--apply", "--repo", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s\n%s", code, stdout.String(), stderr.String())
	}

	data, _ := os.ReadFile(settings)
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("settings unparsable after apply: %v", err)
	}
	if strings.Contains(string(data), "/Volumes/x/sextant") {
		t.Error("stale path survived apply")
	}
	if !strings.Contains(string(data), `"command": "fathom edit-hook"`) {
		t.Errorf("repointed command missing:\n%s", data)
	}

	d := analyzeWiring(root)
	if len(d.Missing) != 0 || len(d.StaleCmds) != 0 {
		t.Errorf("second analyze not clean: missing=%v stale=%v", d.Missing, d.StaleCmds)
	}
}
