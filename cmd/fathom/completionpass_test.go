package main

// completionpass_test.go — coverage for the 2026-08-14 completion pass:
// SD-1 (verify ledger), CC-8 (custodian flywheel loop), DO-1 (derived
// outcome), CC-10 (scaffold loads empty).

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/custodian"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

func TestVerifyLedgerAppendsAndReads(t *testing.T) {
	p := filepath.Join(t.TempDir(), "verify-ledger.jsonl")
	t.Setenv("FATHOM_VERIFY_LEDGER", p)

	rep := verify.Report{Tasks: []verify.TaskVerdict{
		{TaskID: "T-1", Claimed: true, Status: verify.Pass},
		{TaskID: "T-2", Claimed: true, Status: verify.Fail},
		{TaskID: "T-3"},
	}}
	appendVerifyLedger([]string{"docs/plans/x.md"}, "svc", []verify.Report{rep}, verifyProvenance{})

	rows := readVerifyLedger()
	if len(rows) != 1 {
		t.Fatalf("want 1 ledger row, got %d", len(rows))
	}
	r := rows[0]
	if r.Total != 3 || r.Claimed != 2 || r.Verified != 1 || r.Failing != 1 {
		t.Fatalf("counts wrong: %+v", r)
	}
	if r.Plan != "docs/plans/x.md" || r.Service != "svc" || r.At == "" {
		t.Fatalf("identity fields wrong: %+v", r)
	}
}

// CC-8: IDLE with no concerns (named), OPEN when violations stand unread,
// CLOSED when the audit has looked.
func TestCustodianLoopStates(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))

	clean := t.TempDir()
	t.Chdir(clean)
	c := checkLoopCustodian(time.Now())
	if c.Status != loopIdle || !strings.Contains(c.Evidence, "no concerns registered") {
		t.Fatalf("no concerns must be IDLE and named, got %+v", c)
	}

	violated := t.TempDir()
	writeConcernFixture(t, violated) // max_lines 2 vs a 4-line CLAUDE.md
	t.Chdir(violated)
	c = checkLoopCustodian(time.Now())
	if c.Status != loopOpen || !strings.Contains(c.Evidence, "accumulating unread") {
		t.Fatalf("standing violations with no audit beat must be OPEN, got %+v", c)
	}

	// An audit beat inside the week flips it to CLOSED ("being consumed").
	beat := retroBeatLine(time.Now(), "custodian", "audit", map[string]string{
		"violated": "1", "project_dir": violated,
	})
	if err := os.WriteFile(os.Getenv("FATHOM_HOOKS_LOG"), []byte(beat+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c = checkLoopCustodian(time.Now())
	if c.Status != loopClosed || !strings.Contains(c.Evidence, "being consumed") {
		t.Fatalf("looked-at violations must be CLOSED, got %+v", c)
	}
}

// DO-1: commits inside the unit's beat-window count; a session with no
// stamped beats yields ok=false (absence, never a fabricated zero).
func TestDerivedCommitsForSession(t *testing.T) {
	root := t.TempDir()
	retroGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	retroGit(t, root, "add", "-A")
	retroGit(t, root, "commit", "-q", "-m", "in-window")

	now := time.Now()
	beats := []retroBeat{
		{
			Channel: "edit-hook", Kind: "answered", At: now.Add(-time.Hour),
			Meta: map[string]string{"session": "s1", "project_dir": root},
		},
		{
			Channel: "edit-hook", Kind: "answered", At: now,
			Meta: map[string]string{"session": "s1", "project_dir": root},
		},
	}
	n, ok := derivedCommitsForSession(beats, "s1", root)
	if !ok || n != 1 {
		t.Fatalf("want 1 commit in window, got n=%d ok=%v", n, ok)
	}
	if _, ok := derivedCommitsForSession(beats, "unstamped", root); ok {
		t.Fatal("a session with no stamped beats must be absence, not zero")
	}
}

// CC-10: the scaffold teaches and loads as ZERO concerns (the three-state
// loader's scaffold path) — an init-created file must never audit anything
// by surprise.
func TestConcernsScaffoldLoadsEmpty(t *testing.T) {
	p := filepath.Join(t.TempDir(), "concerns.cue")
	if err := os.WriteFile(p, custodian.ExampleTemplate(), 0o644); err != nil {
		t.Fatal(err)
	}
	cs, err := custodian.LoadFile(p, custodian.ScopeProject)
	if err != nil || len(cs) != 0 {
		t.Fatalf("scaffold must load as zero concerns, got %d, err %v", len(cs), err)
	}
	if !bytes.Contains(custodian.ExampleTemplate(), []byte("forbidden_pattern")) {
		t.Fatal("scaffold must teach the check kinds")
	}
}
