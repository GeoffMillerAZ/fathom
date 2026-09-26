package main

// research_bundle_retros_test.go — IR-5 (docs/design/assess-lanes.md §5):
// the retro corpus feeds the research flywheel only through the sanitized
// bundle, only when a human passes --include-retros, and only as counts.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/research/bundle"
)

func writeRetroFixture(t *testing.T, dir, name string, m retroModel) {
	t.Helper()
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func runBundleDry(t *testing.T, root string, extra ...string) (bundle.Bundle, string, string) {
	t.Helper()
	empty := t.TempDir()
	args := append([]string{"--root", root, "--dry-run", "--since", "168h",
		"--call-log", filepath.Join(empty, "no-calls.jsonl"),
		"--hooks-log", filepath.Join(empty, "no-hooks.jsonl"),
		"--bench-dir", empty}, extra...)
	var stdout, stderr bytes.Buffer
	if code := cmdResearchBundle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	// --dry-run prints the bundle JSON first; decode just that value.
	var b bundle.Bundle
	if err := json.NewDecoder(strings.NewReader(stdout.String())).Decode(&b); err != nil {
		t.Fatalf("decode bundle: %v\n%s", err, stdout.String())
	}
	return b, stdout.String(), stderr.String()
}

func TestResearchBundle_RetroCorpusIsOptInAndSanitized(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	state := t.TempDir()
	t.Setenv("FATHOM_STATE_DIR", state)
	root := t.TempDir()
	dir := filepath.Join(state, "artifacts", "retros")
	now := time.Now().UTC()

	writeRetroFixture(t, dir, "sess-one.json", retroModel{
		Session: "SESSION-SENTINEL-1", ProjectRoot: "/Users/someone/PATH-SENTINEL",
		WindowStart: now.Add(-2 * time.Hour).Format(time.RFC3339), WindowEnd: now.Add(-time.Hour).Format(time.RFC3339),
		Firings: []retroFiringOutcome{
			{Channel: "guard", File: "internal/SECRET-FILE-SENTINEL.go", Outcome: "committed"},
			{Channel: "obligations", File: "b.go", Outcome: "modified-uncommitted"},
			{Channel: "custodian", Outcome: "unremedied — 2 violation(s) still standing"},
		},
		GuardFires: 1, Injections: 1, SilentClean: 3, Unjoinable: 1,
	})
	writeRetroFixture(t, dir, "sess-two.json", retroModel{
		Session: "SESSION-SENTINEL-2", Arm: "off",
		WindowEnd:  now.Add(-2 * time.Hour).Format(time.RFC3339),
		Firings:    []retroFiringOutcome{{Channel: "guard", File: "c.go", Outcome: "untouched"}},
		GuardFires: 1,
	})
	writeRetroFixture(t, dir, "sess-old.json", retroModel{
		Session: "old", WindowEnd: now.Add(-30 * 24 * time.Hour).Format(time.RFC3339), GuardFires: 9,
	})
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Default: the corpus is never read — the group is absent, not zero.
	b, out, _ := runBundleDry(t, root)
	if b.Retros != nil || strings.Contains(out, `"retros"`) {
		t.Fatalf("the retro group appeared without --include-retros:\n%s", out)
	}

	b, out, _ = runBundleDry(t, root, "--include-retros")
	r := b.Retros
	if r == nil {
		t.Fatalf("--include-retros produced no retros group:\n%s", out)
	}
	if r.Retros != 2 || r.Unreadable != 1 || r.OffArmRetros != 1 || r.GuardFires != 2 || r.ObligationInjections != 1 ||
		r.SilentOnClean != 3 || r.Unjoinable != 1 || r.CustodianUnremedied != 1 || r.CustodianRemedied != 0 {
		t.Errorf("retro counts = %+v", *r)
	}
	cells := map[string]int{}
	for _, o := range r.Outcomes {
		cells[o.Channel+"/"+o.Outcome] = o.Firings
	}
	for k, want := range map[string]int{"guard/committed": 1, "guard/untouched": 1, "obligations/modified_uncommitted": 1} {
		if cells[k] != want {
			t.Errorf("outcome cell %s = %d, want %d (cells %v)", k, cells[k], want, cells)
		}
	}
	for _, leak := range []string{"SESSION-SENTINEL", "PATH-SENTINEL", "SECRET-FILE-SENTINEL", "sess-one", "violation(s)"} {
		if strings.Contains(out, leak) {
			t.Errorf("the shareable bundle leaked %q:\n%s", leak, out)
		}
	}

	// Opted in but no corpus: absent (not read), and stderr says why.
	t.Setenv("FATHOM_STATE_DIR", t.TempDir())
	b, _, errOut := runBundleDry(t, root, "--include-retros")
	if b.Retros != nil {
		t.Errorf("a missing corpus produced a group: %+v", b.Retros)
	}
	if !strings.Contains(errOut, "retro corpus not found") {
		t.Errorf("stderr must name the missing corpus: %s", errOut)
	}
}
