package main

// armcoverage_test.go — a row's arm label must not outrun what it measured.
//
// The bug this pins produced real corrupt data on 2026-08-08, hours after
// the OFF-arm measurement fix landed: two sessions whose transcripts began
// TEN DAYS before the arm was set were stamped arm="off", carrying token
// and discovery totals that were overwhelmingly ASSISTED activity. Fed to
// a paired comparison, rows like that drag the OFF arm toward the ON arm
// and manufacture a false null — the single conclusion ("fathom saves
// nothing") that would end the program, on evidence that is merely
// mislabelled.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func armSetAt(t *testing.T, when time.Time) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("FATHOM_ARM_STATE", filepath.Join(dir, "arm.json"))
	st := map[string]any{
		"schema_version":     1,
		"fathom_version":     "test",
		"arm":                "off",
		"set_at":             when.UTC().Format(time.RFC3339),
		"sessions_remaining": 5,
	}
	raw, _ := json.Marshal(st)
	if err := os.WriteFile(filepath.Join(dir, "arm.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestArmCoverage(t *testing.T) {
	now := time.Date(2026, 8, 8, 22, 0, 0, 0, time.UTC)
	setAt := time.Date(2026, 8, 8, 7, 11, 47, 0, time.UTC)

	t.Run("a session that began before the arm is PARTIAL", func(t *testing.T) {
		armSetAt(t, setAt)
		start := setAt.Add(-10 * 24 * time.Hour) // the real case: ten days earlier
		cov, note := armCoverage(&start, now)
		if cov != "partial" {
			t.Fatalf("coverage = %q, want partial — this row's totals include assisted activity", cov)
		}
		if !strings.Contains(note, "BEFORE the arm was set") || !strings.Contains(note, "excluded from paired comparison") {
			t.Errorf("the note must state the boundary and the consequence: %q", note)
		}
	})

	t.Run("a session that began after the arm is FULL", func(t *testing.T) {
		armSetAt(t, setAt)
		start := setAt.Add(time.Minute)
		cov, note := armCoverage(&start, now)
		if cov != "full" || note != "" {
			t.Errorf("got (%q, %q), want (full, \"\")", cov, note)
		}
	})

	t.Run("no start timestamp is UNKNOWN, never assumed clean", func(t *testing.T) {
		armSetAt(t, setAt)
		cov, note := armCoverage(nil, now)
		if cov != "unknown" {
			t.Fatalf("coverage = %q, want unknown — an unknowable start is not a clean one (law 3)", cov)
		}
		if !strings.Contains(note, "not assumed clean") {
			t.Errorf("the note must say so: %q", note)
		}
	})

	t.Run("no active arm asks no question", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("FATHOM_ARM_STATE", filepath.Join(dir, "absent.json"))
		start := now.Add(-time.Hour)
		if cov, note := armCoverage(&start, now); cov != "" || note != "" {
			t.Errorf("got (%q, %q), want empty — with no arm there is no window to be inside", cov, note)
		}
	})
}

// TestUncleanArmRowsAreExcludedAndNamed pins the consumer side. A dirty row
// must be neither used nor silently dropped.
func TestUncleanArmRowsAreExcludedAndNamed(t *testing.T) {
	dir := hermeticObsEnv(t)
	rows := []string{
		// Clean OFF row: begins inside the window, carries a split.
		`{"schema_version":2,"session_id":"clean","arm":"off","arm_coverage":"full","at":"2026-08-08T20:00:00Z","tokens_note":"","tokens_total":300,"tokens_in":100,"tokens_out":200}`,
		// Contaminated: began before the arm. The real 2026-08-08 shape.
		`{"schema_version":2,"session_id":"spans","arm":"off","arm_coverage":"partial","arm_coverage_note":"began before the arm","at":"2026-08-08T22:53:15Z","tokens_note":"","tokens_total":933232702,"tokens_in":900000000,"tokens_out":33232702}`,
		// Legacy: written by a binary that never stamped coverage.
		`{"schema_version":2,"session_id":"legacy","arm":"off","at":"2026-08-07T10:00:00Z","tokens_note":"","tokens_total":500,"tokens_in":200,"tokens_out":300}`,
		// An ON row needs no stamp: "on" is not a bounded window.
		`{"schema_version":2,"session_id":"onrow","arm":"on","at":"2026-08-08T21:00:00Z","tokens_note":"","tokens_total":150,"tokens_in":50,"tokens_out":100}`,
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), []byte(strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	imp := buildImpactModel(time.Date(2026, 8, 8, 23, 0, 0, 0, time.UTC))

	if imp.TokensOffSessions != 1 {
		t.Errorf("TokensOffSessions = %d, want 1 — only the clean row counts", imp.TokensOffSessions)
	}
	if imp.TokensOnSessions != 1 {
		t.Errorf("TokensOnSessions = %d, want 1 — an on-row needs no coverage stamp", imp.TokensOnSessions)
	}
	if imp.UncleanArmRows != 2 {
		t.Fatalf("UncleanArmRows = %d, want 2 (the partial and the unstamped legacy row)", imp.UncleanArmRows)
	}
	if imp.UncleanArmNote == "" {
		t.Error("excluded rows must be NAMED, never silently dropped (law 3)")
	}
	// The decisive assertion: the 933M contaminated total must not reach
	// the OFF median. If it does, the OFF arm inherits assisted activity.
	if imp.TokensOffMedian == nil {
		t.Fatal("the clean row should still produce a median")
	}
	if *imp.TokensOffMedian != 300 {
		t.Errorf("TokensOffMedian = %d, want 300 — a contaminated row leaked into the comparison", *imp.TokensOffMedian)
	}
}

// TestLedgerRowCarriesCoverage pins the producer side end to end.
func TestLedgerRowCarriesCoverage(t *testing.T) {
	setAt := time.Date(2026, 8, 8, 7, 11, 47, 0, time.UTC)
	armSetAt(t, setAt)
	now := time.Date(2026, 8, 8, 22, 0, 0, 0, time.UTC)

	before := setAt.Add(-48 * time.Hour)
	got := buildSessionLedgerEntry("s1", "repo", transcriptSummary{FirstTS: &before}, nil, now)
	if got.Arm != "off" {
		t.Fatalf("arm = %q, want off", got.Arm)
	}
	if got.ArmCoverage != "partial" {
		t.Errorf("arm_coverage = %q, want partial — the row must disclose that it spans the boundary", got.ArmCoverage)
	}
	if got.ArmCoverageNote == "" {
		t.Error("a non-full coverage must state the boundary in words")
	}

	after := setAt.Add(time.Hour)
	clean := buildSessionLedgerEntry("s2", "repo", transcriptSummary{FirstTS: &after}, nil, now)
	if clean.ArmCoverage != "full" {
		t.Errorf("arm_coverage = %q, want full", clean.ArmCoverage)
	}
}
