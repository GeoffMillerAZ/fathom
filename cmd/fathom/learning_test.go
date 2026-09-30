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
	"github.com/geoffmilleraz/fathom/internal/ledgerroots"
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

	// GL-14 wired: the verb that persists decisions announces each one.
	t.Run("through fathom claim learn --apply", learningNeverSilentThroughTheVerb)
}

// learningNeverSilentThroughTheVerb is GL-14 end to end (a subtest of
// the row's anchor, TestLearningNeverSilent): `fathom
// claim learn --apply` announces every decision it persists — one line
// each on stdout and one intent-ledger row each, which the session
// attention line counts and `fathom intent audit` lists — and a dry run
// writes no ledger row.
func learningNeverSilentThroughTheVerb(t *testing.T) {
	const svc = "learnsvc"
	f := newClaimLearnFixture(t, svc,
		[]claimLearnSeed{{"cl-cand", core.LifecycleCandidate}, {"cl-refuted", core.LifecycleActive}},
		[]claimLearnRow{
			{200 * learnDay, "cl-cand", "served", "s0"},
			{5 * learnDay, "cl-cand", "corroborated", "sess-a"},
			{4 * learnDay, "cl-cand", "corroborated", "sess-b"},
			{3 * learnDay, "cl-cand", "served", "sess-b"},
			{20 * learnDay, "cl-refuted", "refuted", "s1"},
			{19 * learnDay, "cl-refuted", "refuted", "s2"},
			{18 * learnDay, "cl-refuted", "refuted", "s3"},
		})

	if _, _, code := f.run(t, "--service", svc); code != 0 {
		t.Fatal("dry run failed")
	}
	if rows := intent.RecentLearning(intent.ReadLedger(), f.root, f.now.Add(-time.Hour)); len(rows) != 0 {
		t.Fatalf("a dry run announced %d decision(s) in the ledger; only --apply changes state", len(rows))
	}

	out, errb, code := f.run(t, "--service", svc, "--apply")
	if code != 0 {
		t.Fatalf("--apply: exit %d: %s", code, errb)
	}
	for _, want := range []string{
		"announced: learning: cl-cand candidate → active (2 corroborations from 2 distinct sessions, 0 refutes over 30d)",
		"announced: learning: cl-refuted active → retired (3 refutes > 0 corroborations)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("--apply output lacks %q:\n%s", want, out)
		}
	}
	rows := intent.RecentLearning(intent.ReadLedger(), f.root, f.now.Add(-time.Hour))
	if len(rows) != 2 {
		t.Fatalf("ledger learning rows = %+v, want one per decision (2)", rows)
	}
	for _, r := range rows {
		if r.Evidence == "" || (r.ID != "cl-cand" && r.ID != "cl-refuted") {
			t.Errorf("ledger row %+v carries no signals or names another claim", r)
		}
	}
	// PE-25: the fragment reads the intent ledger under the registered
	// roots; the fixture's root is the registered one (it sits in the test's
	// temp dir, so no scratch dir is named).
	if got := learningAttentionFragment(f.root, f.now.Add(time.Minute), ledgerroots.RegisteredRootsWith([]string{f.root}, nil)); got != "learning 2" {
		t.Errorf("attention fragment = %q, want \"learning 2\"", got)
	}
}
