package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestConsoleFleetReconcileTrigger is SR-55's anchor test (docs/plans/
// surface-reckoning.md, design §5 row SR-55, F-57): the fleet page's
// reconcile trigger (consoleReconcileAction) runs the SAME read-only
// wiring analysis (analyzeWiringFor) `fathom fleet reconcile` runs before
// --apply, across every project root given, and reports drift per repo —
// deterministic (sorted by root) and strictly read-only: the console
// trigger never applies, only shows.
func TestConsoleFleetReconcileTrigger(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home) // user scope empty — drift is decided by the repo alone

	drifted := t.TempDir()
	if err := os.MkdirAll(filepath.Join(drifted, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(drifted, ".claude", "settings.json")
	settings := []byte(`{
  "hooks": {
    "PostToolUse": [{"matcher": "Edit|Write|MultiEdit", "hooks": [{"type": "command", "command": "/Volumes/x/sextant/bin/fathom edit-hook", "timeout": 5}]}]
  }
}`)
	if err := os.WriteFile(settingsPath, settings, 0o644); err != nil {
		t.Fatal(err)
	}

	roots := []string{drifted}
	svcByRoot := map[string]string{drifted: "widgetco"}

	diffs := consoleReconcileAction(roots, svcByRoot)

	// Read-only: the trigger must never write — no --apply from the console.
	after, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(settings) {
		t.Fatalf("consoleReconcileAction modified %s — the console trigger must be read-only (dry-run), never --apply", settingsPath)
	}

	if len(diffs) != 1 {
		t.Fatalf("got %d diffs, want 1", len(diffs))
	}
	d := diffs[0]
	if d.Repo != drifted || d.Service != "widgetco" {
		t.Errorf("diff = %+v, want Repo=%q Service=%q", d, drifted, "widgetco")
	}
	if !wiringDiffHasDrift(d) {
		t.Errorf("wiringDiffHasDrift(%+v) = false, want true (a stale absolute hook path)", d)
	}
	if len(d.StaleCmds) != 1 || !strings.Contains(d.StaleCmds[0], "/Volumes/x/sextant/bin/fathom") {
		t.Errorf("StaleCmds = %v, want the stale absolute path named", d.StaleCmds)
	}

	// Determinism: sorted by root, whatever order the caller passed them in.
	a, b := t.TempDir(), t.TempDir()
	order1 := consoleReconcileAction([]string{b, a}, nil)
	order2 := consoleReconcileAction([]string{a, b}, nil)
	if len(order1) != 2 || len(order2) != 2 {
		t.Fatalf("got %d/%d diffs, want 2/2", len(order1), len(order2))
	}
	if order1[0].Repo != order2[0].Repo || order1[1].Repo != order2[1].Repo {
		t.Errorf("consoleReconcileAction result order depends on input order: %+v vs %+v", order1, order2)
	}
	if !sort.StringsAreSorted([]string{order1[0].Repo, order1[1].Repo}) {
		t.Errorf("consoleReconcileAction results are not sorted by root: %v", []string{order1[0].Repo, order1[1].Repo})
	}

	// No roots at all: an empty, non-nil-panicking result.
	if got := consoleReconcileAction(nil, nil); len(got) != 0 {
		t.Errorf("consoleReconcileAction(nil, nil) = %v, want empty", got)
	}
}
