// learning_test.go — GL-14 (docs/plans/governance-learning.md, design doc
// §6 "Rules that learn"): TestLearningNeverSilent is this row's proof
// anchor (reachable_from) — GL-X1 refuses silent learning, so every
// promotion/retirement applyLearning (GL-13) makes must come out the other
// end as an announcement, with no decision dropped.
package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

func TestLearningNeverSilent(t *testing.T) {
	t.Setenv("FATHOM_INTENT_LEDGER", filepath.Join(t.TempDir(), "l.jsonl"))
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	root := "/repo"

	decisions := []semantic.LearningDecision{
		{
			ClaimID: "cl-1", From: core.LifecycleCandidate, To: core.LifecycleActive,
			Reason:         "3 corroborations from 2 distinct sessions, 0 refutes over 30d",
			Corroborations: 3, DistinctSessions: 2,
		},
		{
			ClaimID: "cl-2", From: core.LifecycleActive, To: core.LifecycleRetired,
			Reason: "4 refutes > 1 corroborations", Refutes: 4, Corroborations: 1,
		},
	}

	lines := learningAnnouncements(root, decisions, now)

	// Never silent: every decision produces exactly one announced line.
	if len(lines) != len(decisions) {
		t.Fatalf("learningAnnouncements returned %d line(s) for %d decision(s), want 1:1 (none silent)",
			len(lines), len(decisions))
	}
	for i, want := range decisions {
		if lines[i] == "" {
			t.Fatalf("line %d is empty for decision %+v", i, want)
		}
		for _, substr := range []string{want.ClaimID, string(want.To), want.Reason} {
			if !strings.Contains(lines[i], substr) {
				t.Errorf("line %q missing %q (the signal must be cited, not just the verdict)", lines[i], substr)
			}
		}
	}

	// Durably recorded: `fathom intent audit` and the attention line both
	// read the SAME intent ledger (extended, not doubled) — this is the
	// "announced ... in fathom intent audit" half of GL-14's row text.
	rows := intent.RecentLearning(intent.ReadLedger(), root, now.Add(-24*time.Hour))
	if len(rows) != 2 {
		t.Fatalf("ledger rows = %+v, want 2 (one per decision)", rows)
	}
	byID := map[string]intent.LedgerRow{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	if r, ok := byID["cl-1"]; !ok || r.Verdict != "active" || r.Evidence != decisions[0].Reason {
		t.Errorf("cl-1 ledger row = %+v, want verdict=active evidence=%q", r, decisions[0].Reason)
	}
	if r, ok := byID["cl-2"]; !ok || r.Verdict != "retired" || r.Evidence != decisions[1].Reason {
		t.Errorf("cl-2 ledger row = %+v, want verdict=retired evidence=%q", r, decisions[1].Reason)
	}

	// Re-announcing the SAME already-recorded decisions must not duplicate
	// the ledger (a claim promotes/retires exactly once in applyLearning's
	// own state machine — a caller that recomputes must not fill the
	// ledger with repeats) but must still announce (never silent, even on
	// a repeat — the caller decides whether to print it again).
	lines2 := learningAnnouncements(root, decisions, now.Add(time.Minute))
	if len(lines2) != len(decisions) {
		t.Fatalf("second call returned %d line(s), want %d (still never silent)", len(lines2), len(decisions))
	}
	rows2 := intent.RecentLearning(intent.ReadLedger(), root, now.Add(-24*time.Hour))
	if len(rows2) != 2 {
		t.Fatalf("ledger rows after a repeat call = %+v, want still 2 (deduped)", rows2)
	}

	// Zero decisions: zero lines, nothing written — correctly nothing to
	// say is not the same as silently dropping something there was.
	if lines := learningAnnouncements(root, nil, now); len(lines) != 0 {
		t.Errorf("learningAnnouncements(nil) = %v, want empty", lines)
	}
}
