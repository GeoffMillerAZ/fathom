package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/wiring"
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

// ─── INC-12: reconcile --apply goes through internal/wiring ──────────────

// TestReconcileStopsOnDriftedRegion pins law 15 on the production path: a
// region fathom is TRACKING (marker format v3, digest recorded) that a
// human edited is reported BLOCKED, the file is left byte-identical, the
// verb exits 1 — and the other files in the same repository still apply,
// under one receipt.
func TestReconcileStopsOnDriftedRegion(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	svc := strings.ToLower(filepath.Base(root))

	// CLAUDE.md: written by fathom (tracked), then edited by hand INSIDE
	// fathom's region.
	claudePath := filepath.Join(root, "CLAUDE.md")
	if err := os.WriteFile(claudePath, []byte("# repo\n\nhouse prose\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyStanza(claudePath, fathomStanza(svc, false), &strings.Builder{}); err != nil {
		t.Fatalf("applyStanza: %v", err)
	}
	written, _ := os.ReadFile(claudePath)
	if !strings.Contains(string(written), fathomStanzaBeginPrefix+" "+fathomStanzaVersion+" sha256:") {
		t.Fatalf("applyStanza did not write a tracked (digest-bearing) marker:\n%s", written)
	}
	edited := strings.Replace(string(written), "Read files to *edit* them", "Read files to *edit* them (house rule: read everything first)", 1)
	if edited == string(written) {
		t.Fatal("fixture: the hand edit changed nothing")
	}
	if err := os.WriteFile(claudePath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	// AGENTS.md: a pre-v3 region behind the template — still applies.
	agentsPath := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("# agents\n\n"+fathomStanzaBegin+"\nold wording\n"+fathomStanzaEnd+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr strings.Builder
	code := cmdFleetReconcile([]string{"--apply", "--repo", root}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (a BLOCKED file is a non-zero exit):\n%s%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "BLOCKED: CLAUDE.md") || !strings.Contains(stdout.String(), "edited by hand") {
		t.Errorf("the drift is not named:\n%s", stdout.String())
	}
	got, _ := os.ReadFile(claudePath)
	if string(got) != edited {
		t.Fatalf("fathom MERGED over a hand-edited tracked region:\n--- want\n%s\n--- got\n%s", edited, got)
	}

	agents, _ := os.ReadFile(agentsPath)
	if strings.Contains(string(agents), "old wording") || !strings.Contains(string(agents), fathomStanzaBody(svc, false)) {
		t.Errorf("the un-drifted AGENTS.md was not refreshed alongside the BLOCKED file:\n%s", agents)
	}
	settings, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	if err != nil || !strings.Contains(string(settings), "fathom session-hook") {
		t.Errorf("missing hooks were not added alongside the BLOCKED file (err=%v):\n%s", err, settings)
	}

	receipts, err := wiring.LoadReceipts(root)
	if err != nil {
		t.Fatalf("LoadReceipts: %v", err)
	}
	if len(receipts) != 2 {
		t.Fatalf("receipts = %d, want 2 (the fixture's applyStanza, then reconcile)", len(receipts))
	}
	var paths []string
	for _, f := range receipts[0].Files {
		paths = append(paths, f.Path)
	}
	joined := strings.Join(paths, ",")
	if strings.Contains(joined, "CLAUDE.md") {
		t.Errorf("the receipt claims a write to the BLOCKED file: %v", paths)
	}
	if !strings.Contains(joined, "AGENTS.md") || !strings.Contains(joined, ".claude/settings.json") {
		t.Errorf("the receipt does not cover the files reconcile wrote: %v", paths)
	}
}

// TestReconcileUpgradesUntrackedRegion pins the compatibility rule: a v1
// region (marker pair, no digest, no receipt — what every repository wired
// before INC-12 carries) is fathom's by its markers: reported stale, never
// drift; --apply rewrites it to the template under a digest-bearing marker,
// keeps every byte outside it, and from then on a hand edit IS drift.
func TestReconcileUpgradesUntrackedRegion(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	svc := strings.ToLower(filepath.Base(root))
	claudePath := filepath.Join(root, "CLAUDE.md")
	prefix, suffix := "# repo\n\nhouse prose\n\n", "\n\ntrailing notes\n"
	if err := os.WriteFile(claudePath, []byte(prefix+fathomStanzaBegin+"\nold wording nobody shipped\n"+fathomStanzaEnd+suffix), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr strings.Builder
	if code := cmdFleetReconcile([]string{"--repo", root}, &stdout, &stderr); code != 0 {
		t.Fatalf("dry-run exit = %d:\n%s%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "CLAUDE.md behind the current template") {
		t.Errorf("dry-run does not report the v1 region as stale:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "BLOCKED") {
		t.Errorf("a v1 region was reported as drift — unknown provenance is not evidence of an edit:\n%s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := cmdFleetReconcile([]string{"--apply", "--repo", root}, &stdout, &stderr); code != 0 {
		t.Fatalf("apply exit = %d:\n%s%s", code, stdout.String(), stderr.String())
	}
	got, _ := os.ReadFile(claudePath)
	s := string(got)
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		t.Errorf("bytes outside the region changed:\n%s", s)
	}
	if !strings.Contains(s, fathomStanzaBeginPrefix+" "+fathomStanzaVersion+" sha256:") {
		t.Errorf("the upgraded region carries no digest — the next hand edit would be invisible:\n%s", s)
	}
	if strings.Contains(s, "old wording") || !strings.Contains(s, fathomStanzaBody(svc, false)) {
		t.Errorf("the region was not rewritten to the template:\n%s", s)
	}
	if receipts, err := wiring.LoadReceipts(root); err != nil || len(receipts) != 1 {
		t.Errorf("receipts = %d (err=%v), want 1", len(receipts), err)
	}

	d := analyzeWiring(root)
	if len(d.StaleStanzas) != 0 || len(d.DriftedStanzas) != 0 {
		t.Errorf("after the upgrade the stanza is not current: stale=%v drifted=%v", d.StaleStanzas, d.DriftedStanzas)
	}

	// Now tracked: a hand edit inside the region is drift, and stays so.
	if err := os.WriteFile(claudePath, []byte(strings.Replace(s, "the index replaces exploratory grep", "the index replaces exploratory grep (mostly)", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	d = analyzeWiring(root)
	if len(d.DriftedStanzas) != 1 || !strings.HasPrefix(d.DriftedStanzas[0], "CLAUDE.md — ") {
		t.Errorf("a hand edit inside the tracked region was not reported as drift: %v", d.DriftedStanzas)
	}
}

// TestReconcileRemarkIsNotStale pins the honest reading of a v1 region
// whose prose already equals the current template: it is UP TO DATE, and
// --apply only re-marks it with a digest — it is not "behind the template",
// which promised a rewrite that would change no prose. A v1 region whose
// prose differs is still stale, and still refreshed.
func TestReconcileRemarkIsNotStale(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	svc := strings.ToLower(filepath.Base(root))
	claudePath := filepath.Join(root, "CLAUDE.md")
	prefix, suffix := "# repo\n\nhouse prose\n\n", "\n\ntrailing notes\n"
	if err := os.WriteFile(claudePath, []byte(prefix+fathomStanza(svc, false)+suffix), 0o644); err != nil {
		t.Fatal(err)
	}
	agentsPath := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("# agents\n\n"+fathomStanzaBegin+"\nold wording\n"+fathomStanzaEnd+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	d := analyzeWiring(root)
	if len(d.RemarkStanzas) != 1 || d.RemarkStanzas[0] != "CLAUDE.md" {
		t.Errorf("remark = %v, want [CLAUDE.md] (prose equals the template; only the marker changes)", d.RemarkStanzas)
	}
	if len(d.StaleStanzas) != 1 || d.StaleStanzas[0] != "AGENTS.md" {
		t.Errorf("stale = %v, want [AGENTS.md] (truly behind the template)", d.StaleStanzas)
	}
	if len(d.DriftedStanzas) != 0 {
		t.Errorf("drifted = %v, want none", d.DriftedStanzas)
	}

	var stdout, stderr strings.Builder
	if code := cmdFleetReconcile([]string{"--repo", root}, &stdout, &stderr); code != 0 {
		t.Fatalf("dry-run exit = %d:\n%s%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "CLAUDE.md up to date (will be re-marked with a digest on apply") {
		t.Errorf("dry-run does not report the up-to-date v1 region as a re-mark:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "CLAUDE.md behind the current template") {
		t.Errorf("an up-to-date region was reported as stale:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "AGENTS.md behind the current template") {
		t.Errorf("the truly stale region was not reported:\n%s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := cmdFleetReconcile([]string{"--apply", "--repo", root}, &stdout, &stderr); code != 0 {
		t.Fatalf("apply exit = %d:\n%s%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "CLAUDE.md stanza re-marked with a digest (prose unchanged)") {
		t.Errorf("apply does not name the re-mark as such:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "AGENTS.md stanza refreshed to the current template") {
		t.Errorf("apply does not name the refresh:\n%s", stdout.String())
	}
	got, _ := os.ReadFile(claudePath)
	s := string(got)
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		t.Errorf("bytes outside the region changed:\n%s", s)
	}
	if !strings.Contains(s, fathomStanzaBeginPrefix+" "+fathomStanzaVersion+" sha256:") || !strings.Contains(s, fathomStanzaBody(svc, false)) {
		t.Errorf("the region was not re-marked with a digest around unchanged prose:\n%s", s)
	}
	if want := prefix + fathomStanzaBeginPrefix; !strings.HasPrefix(s, want) {
		t.Errorf("the re-mark moved the region:\n%s", s)
	}
	d = analyzeWiring(root)
	if len(d.StaleStanzas) != 0 || len(d.RemarkStanzas) != 0 || len(d.DriftedStanzas) != 0 {
		t.Errorf("after apply: stale=%v remark=%v drifted=%v, want none", d.StaleStanzas, d.RemarkStanzas, d.DriftedStanzas)
	}
}
