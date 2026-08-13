package main

// flywheel_test.go — the two 2026-08-08 measurement failures, as fixtures.
//
// Both were silent. Every dashboard looked fine because every surface asked
// "what do the numbers say?" and none asked "is the pipe connected?". These
// tests fail if `fathom flywheel` ever loses the ability to see either one.

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFlywheelSeesTheArmCollectingNothing(t *testing.T) {
	now := time.Date(2026, 8, 8, 22, 0, 0, 0, time.UTC)
	setAt := now.Add(-14 * time.Hour) // the real gap: armed 07:11Z, checked 21:00Z
	armSetAt(t, setAt)

	// Failure 1, verbatim: armed for fourteen hours, not one row recorded.
	got := checkLoopArm(nil, obsImpact{}, now)
	if got.Status != loopOpen {
		t.Fatalf("status = %q, want OPEN — an arm that has recorded nothing for 14h is burning its budget for no data", got.Status)
	}
	if !strings.Contains(got.Evidence, "collecting NOTHING") {
		t.Errorf("the evidence must name the failure plainly: %q", got.Evidence)
	}
	if got.Remedy == "" {
		t.Error("an OPEN loop must carry a remedy")
	}
}

func TestFlywheelSeesRowsArrivingButInadmissible(t *testing.T) {
	now := time.Date(2026, 8, 8, 22, 0, 0, 0, time.UTC)
	setAt := now.Add(-14 * time.Hour)
	armSetAt(t, setAt)

	// Failure 2, verbatim: rows ARE arriving, and not one of them counts.
	// This is the subtle case — the producing side looks alive.
	rows := []ledgerRow{
		{At: now.Add(-time.Hour).Format(time.RFC3339), Arm: "off", ArmCoverage: "partial"},
		{At: now.Add(-30 * time.Minute).Format(time.RFC3339), Arm: "off", ArmCoverage: "partial"},
	}
	got := checkLoopArm(rows, obsImpact{}, now)
	if got.Status != loopOpen {
		t.Fatalf("status = %q, want OPEN — rows arriving that none of which are admissible is still zero data", got.Status)
	}
	if !strings.Contains(got.Evidence, "NONE are admissible") {
		t.Errorf("the evidence must distinguish 'arriving' from 'usable': %q", got.Evidence)
	}
	if !strings.Contains(got.Remedy, "ARM-COVERED WINDOW") {
		t.Errorf("the remedy must teach the amended unit — a resumed session's window counts once it clears the floor: %q", got.Remedy)
	}
}

func TestFlywheelClosesWhenAdmissibleRowsExist(t *testing.T) {
	now := time.Date(2026, 8, 8, 22, 0, 0, 0, time.UTC)
	setAt := now.Add(-14 * time.Hour)
	armSetAt(t, setAt)

	rows := []ledgerRow{
		{At: now.Add(-time.Hour).Format(time.RFC3339), Arm: "off", ArmCoverage: "partial"},
		{At: now.Add(-20 * time.Minute).Format(time.RFC3339), Arm: "off", ArmCoverage: "full"},
	}
	got := checkLoopArm(rows, obsImpact{}, now)
	if got.Status != loopClosed {
		t.Fatalf("status = %q, want CLOSED — one admissible row is data", got.Status)
	}
	if !strings.Contains(got.Evidence, "1 admissible") {
		t.Errorf("the evidence must report the admissible count: %q", got.Evidence)
	}
}

// TestFlywheelIdleIsNotFailure pins the three-valued discipline: a loop
// with nothing to measure yet must not fail the exit code, or people learn
// to ignore it.
func TestFlywheelIdleIsNotFailure(t *testing.T) {
	idle := []flywheelCheck{
		{Loop: "a", Status: loopIdle},
		{Loop: "b", Status: loopClosed},
	}
	if code := flywheelExit(idle); code != 0 {
		t.Errorf("exit = %d, want 0 — IDLE is 'nothing to measure yet', not a defect", code)
	}
	withOpen := append(idle, flywheelCheck{Loop: "c", Status: loopOpen})
	if code := flywheelExit(withOpen); code != 1 {
		t.Errorf("exit = %d, want 1 — an OPEN loop is a defect", code)
	}
}

// TestFlywheelEveryCheckCarriesEvidence pins the honesty contract: a
// verdict with no numbers behind it is an assertion, and an OPEN or IDLE
// loop with no remedy leaves a reader stuck.
func TestFlywheelEveryCheckCarriesEvidence(t *testing.T) {
	now := time.Date(2026, 8, 8, 22, 0, 0, 0, time.UTC)
	armSetAt(t, now.Add(-time.Hour))
	hermeticObsEnv(t)
	armSetAt(t, now.Add(-time.Hour)) // hermeticObsEnv resets FATHOM_ARM_STATE

	for _, c := range runFlywheelChecks("http://127.0.0.1:1", now) {
		if c.Purpose == "" {
			t.Errorf("%q has no stated purpose — a health check nobody can interpret is not one", c.Loop)
		}
		if strings.TrimSpace(c.Evidence) == "" {
			t.Errorf("%q reports %q with no evidence", c.Loop, c.Status)
		}
		if c.Status != loopClosed && c.Remedy == "" {
			t.Errorf("%q is %q with no remedy — a reader is left stuck", c.Loop, c.Status)
		}
	}
}

// TestFlywheelSessionLedgerStall pins the first failure's other signature:
// the ledger itself going quiet while sessions are plainly happening.
func TestFlywheelSessionLedgerStall(t *testing.T) {
	now := time.Date(2026, 8, 8, 22, 0, 0, 0, time.UTC)
	stale := []ledgerRow{{At: now.Add(-72 * time.Hour).Format(time.RFC3339)}}
	got := checkLoopSessionLedger(stale, now)
	if got.Status != loopOpen {
		t.Fatalf("status = %q, want OPEN — a ledger silent for three days is the denominator failing", got.Status)
	}
	fresh := []ledgerRow{{At: now.Add(-time.Hour).Format(time.RFC3339)}}
	if got := checkLoopSessionLedger(fresh, now); got.Status != loopClosed {
		t.Errorf("status = %q, want CLOSED", got.Status)
	}
}

// TestFathomServicePathToleratesNilClient is the regression guard for a
// SIGSEGV that reached main and was invisible here for hours.
//
// `fathom flywheel` passed a nil *http.Client into
// resolveCallLogCandidates. On any machine WITHOUT a local
// cmd/fathom/workspace/logs/mcp-calls.jsonl — i.e. every clean checkout —
// that reached fetchServicesOnce and dereferenced nil. The author's own
// gate stayed green only because that gitignored artifact happened to
// exist locally.
//
// The second-order damage is the reason this test asserts at the boundary
// rather than through the flywheel: a panic aborts the WHOLE package test
// binary, so every test file sorting after the panicking one never runs.
// The gate does not merely go red — it goes BLIND, and reports on work it
// never exercised.
//
// This test does not depend on any artifact, so it cannot be masked the
// same way.
func TestFathomServicePathToleratesNilClient(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("a nil client must not panic — a diagnostic runs precisely when things are broken: %v", r)
		}
	}()
	// 127.0.0.1:1 refuses instantly; the point is reaching the request at
	// all without dereferencing nil.
	if _, ok := fathomServicePath(nil, "http://127.0.0.1:1"); ok {
		t.Error("no daemon is listening; ok must be false")
	}
}

// TestFlywheelSurvivesWithoutALocalCallLog pins the end-to-end shape: the
// verb must produce a report on a machine that has never run the daemon.
func TestFlywheelSurvivesWithoutALocalCallLog(t *testing.T) {
	hermeticObsEnv(t)
	// Point the call log at a path that does not exist, rather than
	// chdir-ing into an empty directory.
	//
	// The first version of this test called os.Chdir. That is a
	// PROCESS-GLOBAL mutation, and Go runs a package's tests in one
	// process — so every later test resolving a relative path (the report
	// and remap suites) broke, and the failures pointed at them rather
	// than here. A test that changes the working directory is a landmine
	// for every test after it.
	t.Setenv("FATHOM_CALL_LOG", filepath.Join(t.TempDir(), "absent", "mcp-calls.jsonl"))

	checks := runFlywheelChecks("http://127.0.0.1:1", time.Date(2026, 8, 8, 22, 0, 0, 0, time.UTC))
	if len(checks) == 0 {
		t.Fatal("the flywheel must still report every loop with no local state")
	}
	for _, c := range checks {
		if strings.TrimSpace(c.Evidence) == "" {
			t.Errorf("%q has no evidence even in the degraded case (law 3)", c.Loop)
		}
	}
}
