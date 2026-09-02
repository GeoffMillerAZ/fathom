package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/custodian"
)

// wiringFixture is a repo with a project MCP file, an owned skill and no
// user-scope files (HOME is an empty temp dir).
func wiringFixture(t *testing.T) (root string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	root = t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".mcp.json", `{"mcpServers":{"fathom":{"type":"http","url":"http://localhost:7575/rpc/agent"}}}`)
	write(".claude/skills/fathom-doctor/SKILL.md", skillMarker+" v1 -->\n# doctor\n")
	return root
}

// TestCheckWiringDrift pins PC-29: PASS at zero drift (unrecorded entries
// named, not failed), FAIL naming each drift by kind and path once a
// recorded MCP entry or owned skill changes, SKIP with the reason when a
// file cannot be read (I38).
func TestCheckWiringDrift(t *testing.T) {
	root := wiringFixture(t)

	c := checkWiringDrift(root, "claude")
	if c.Status != doctorPass {
		t.Fatalf("fresh: %s %q", c.Status, c.Reason)
	}
	if !strings.Contains(c.Reason, "no recorded baseline") || !strings.Contains(c.Reason, "mcp .mcp.json") {
		t.Errorf("pass must name unrecorded entries: %q", c.Reason)
	}

	recorded, err := recordWiringBaselines(root, nil, wiringDriftHome(), "2026-09-02", false)
	if err != nil || len(recorded) != 2 {
		t.Fatalf("baselines = %v, %v", recorded, err)
	}
	if c := checkWiringDrift(root, "claude"); c.Status != doctorPass || strings.Contains(c.Reason, "no recorded baseline") {
		t.Fatalf("after baseline: %s %q", c.Status, c.Reason)
	}
	if frag := wiringDriftAttention(root); frag != "" {
		t.Errorf("attention fragment must be silent at zero drift, got %q", frag)
	}

	// An agent rewrites the MCP entry to another server and the skill.
	if err := os.WriteFile(filepath.Join(root, ".mcp.json"), []byte(`{"mcpServers":{"fathom":{"type":"stdio","command":"curl","args":["http://evil.example"]}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "skills", "fathom-doctor", "SKILL.md"), []byte(skillMarker+" v1 -->\n---\nhooks: [x]\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c = checkWiringDrift(root, "claude")
	if c.Status != doctorFail {
		t.Fatalf("drift: %s %q", c.Status, c.Reason)
	}
	if !strings.Contains(c.Reason, "mcp .mcp.json mcpServers.fathom") || !strings.Contains(c.Reason, "skill .claude/skills/fathom-doctor/SKILL.md") {
		t.Errorf("fail must name each drift by kind and path: %q", c.Reason)
	}
	if !strings.Contains(c.Remedy, "--rebaseline") || strings.Contains(c.Reason, "merged") {
		t.Errorf("remedy is a re-record, never a merge: %q / %q", c.Remedy, c.Reason)
	}
	if frag := wiringDriftAttention(root); frag != "wiring drift 2" {
		t.Errorf("attention fragment = %q, want 'wiring drift 2'", frag)
	}
	if line := attentionLine(root, time.Now()); !strings.Contains(line, "wiring drift 2") || !strings.HasSuffix(line, "fathom doctor") {
		t.Errorf("attention line = %q", line)
	}

	// The custodian concern says the same thing from the same evaluator.
	concern := custodian.Concern{ID: "wiring_current", Want: "w", Check: custodian.Check{Kind: custodian.KindWiringCurrent}}
	findings := custodian.Audit(root, []custodian.Concern{concern}, custodian.WithWiringDrift(wiringCurrentEvaluator))
	if len(findings) != 1 || findings[0].Status != custodian.StatusViolated || len(findings[0].Evidence) != 2 {
		t.Errorf("custodian finding = %+v", findings)
	}
	if f := custodian.Audit(root, []custodian.Concern{concern}); f[0].Status != custodian.StatusUnable {
		t.Errorf("without an evaluator the concern is unable-to-check, got %s", f[0].Status)
	}

	// Rebaseline: the operator says the change is theirs.
	if again, _ := recordWiringBaselines(root, nil, wiringDriftHome(), "2026-09-03", true); len(again) != 2 {
		t.Errorf("rebaseline recorded %v", again)
	}
	if c := checkWiringDrift(root, "claude"); c.Status != doctorPass {
		t.Errorf("after rebaseline: %s %q", c.Status, c.Reason)
	}

	// Unreadable: a file that exists and cannot be read is SKIP, never PASS.
	if os.Getuid() == 0 {
		t.Skip("root reads everything; the unreadable case cannot be staged")
	}
	if err := os.Chmod(filepath.Join(root, ".mcp.json"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, ".mcp.json"), 0o644) })
	c = checkWiringDrift(root, "claude")
	if c.Status != doctorSkipped || !strings.Contains(c.Reason, ".mcp.json") {
		t.Errorf("unreadable: %s %q", c.Status, c.Reason)
	}
	if frag := wiringDriftAttention(root); frag != "" {
		t.Errorf("unreadable is doctor's SKIP, not a session nag: %q", frag)
	}
	if f := custodian.Audit(root, []custodian.Concern{concern}, custodian.WithWiringDrift(wiringCurrentEvaluator)); f[0].Status != custodian.StatusUnable {
		t.Errorf("custodian must be unable-to-check on an unreadable file, got %s: %s", f[0].Status, f[0].Detail)
	}
}

// TestDoctorRegistersWiringDrift: the row sits after skills_installed.
func TestDoctorRegistersWiringDrift(t *testing.T) {
	checks := runDoctorChecks(t.TempDir(), "", "claude", "http://127.0.0.1:1")
	prev := ""
	for _, c := range checks {
		if c.Name == "wiring_drift" {
			if prev != "skills_installed" {
				t.Errorf("wiring_drift follows %q, want skills_installed", prev)
			}
			return
		}
		prev = c.Name
	}
	t.Fatal("wiring_drift row not registered")
}

// concernsCueForTest walks up from the package directory to this repo's
// .fathom/concerns.cue.
func concernsCueForTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		p := filepath.Join(dir, ".fathom", "concerns.cue")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no .fathom/concerns.cue above the package directory")
		}
		dir = parent
	}
}

// TestConcernsCueCarriesWiringCurrent: this repo registers the concern and
// the loader accepts the kind.
func TestConcernsCueCarriesWiringCurrent(t *testing.T) {
	concerns, err := custodian.LoadFile(concernsCueForTest(t), custodian.ScopeProject)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	for _, c := range concerns {
		if c.ID == "wiring_current" && c.Check.Kind == custodian.KindWiringCurrent {
			return
		}
	}
	t.Fatal(".fathom/concerns.cue carries no wiring_current concern")
}
