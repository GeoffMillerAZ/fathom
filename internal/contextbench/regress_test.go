package contextbench_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/contextbench"
)

func pt(id string, ts, tt int) contextbench.PairedTask {
	return contextbench.PairedTask{TaskID: id, TreatmentSuccesses: ts, TreatmentTrials: tt, ControlTrials: tt}
}

func ledgerRow(id, sha string, per ...contextbench.PairedTask) contextbench.LedgerRow {
	r := row(id, contextbench.VerdictPass)
	r.CorpusSHA256 = sha
	r.PerTask = per
	return r
}

const (
	shaA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	shaB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func TestRegress_FlagsAFallListsFlakyIgnoresAdded(t *testing.T) {
	rows := []contextbench.LedgerRow{
		ledgerRow("r1", shaA, pt("T1", 3, 3), pt("T2", 3, 3), pt("T3", 0, 3), pt("T4", 2, 3), pt("T9", 3, 3)),
		ledgerRow("r2", shaA, pt("T1", 3, 3), pt("T2", 0, 3), pt("T3", 3, 3), pt("T4", 3, 3), pt("T5", 0, 3)),
	}
	got := contextbench.Regress(rows, "")
	if got.Verdict != contextbench.VerdictFail {
		t.Fatalf("T2 went 3/3 -> 0/3: want fail, got %s (%s)", got.Verdict, got.Reason)
	}
	if len(got.Regressions) != 1 || got.Regressions[0].TaskID != "T2" || got.Regressions[0].BaselineRate != 1 || got.Regressions[0].CurrentRate != 0 {
		t.Errorf("regressions = %+v, want only T2", got.Regressions)
	}
	if !reflect.DeepEqual(got.Flaky, []string{"T4"}) {
		t.Errorf("T4 was flaky in the baseline: listed, not failed — got %v", got.Flaky)
	}
	if !reflect.DeepEqual(got.Added, []string{"T5"}) || !reflect.DeepEqual(got.Removed, []string{"T9"}) {
		t.Errorf("added/removed = %v/%v, want [T5]/[T9]", got.Added, got.Removed)
	}
	if got.BaselineRunID != "r1" || got.CurrentRunID != "r2" || got.Compared != 4 {
		t.Errorf("baseline/current/compared = %s/%s/%d", got.BaselineRunID, got.CurrentRunID, got.Compared)
	}
	// T3 improved: not a regression; T5 added at 0/3: not a regression.
	again := contextbench.Regress(rows, "")
	if !reflect.DeepEqual(got, again) {
		t.Error("Regress must be deterministic")
	}
}

func TestRegress_PassesWhenNothingFell(t *testing.T) {
	rows := []contextbench.LedgerRow{
		ledgerRow("r1", shaA, pt("T1", 3, 3), pt("T2", 0, 3)),
		ledgerRow("r2", shaA, pt("T1", 3, 3), pt("T2", 3, 3)),
	}
	if got := contextbench.Regress(rows, ""); got.Verdict != contextbench.VerdictPass {
		t.Fatalf("want pass, got %s (%s)", got.Verdict, got.Reason)
	}
}

func TestRegress_IndeterminateWithoutAComparableBaseline(t *testing.T) {
	cases := map[string][]contextbench.LedgerRow{
		"empty ledger":     nil,
		"one row":          {ledgerRow("r1", shaA, pt("T1", 3, 3))},
		"no corpus hash":   {ledgerRow("r1", "", pt("T1", 3, 3)), ledgerRow("r2", "", pt("T1", 3, 3))},
		"different corpus": {ledgerRow("r1", shaB, pt("T1", 3, 3)), ledgerRow("r2", shaA, pt("T1", 3, 3))},
		"no per_task":      {ledgerRow("r1", shaA), ledgerRow("r2", shaA)},
		"backfilled row before": {
			ledgerRow("backfill", "", nil...),
			ledgerRow("r2", shaA, pt("T1", 3, 3)),
		},
	}
	for name, rows := range cases {
		got := contextbench.Regress(rows, "")
		if got.Verdict != contextbench.VerdictIndeterminate {
			t.Errorf("%s: want indeterminate, got %s (%s)", name, got.Verdict, got.Reason)
		}
	}
}

func TestRegress_SkipsOverAnUnrelatedCorpusToFindTheBaseline(t *testing.T) {
	rows := []contextbench.LedgerRow{
		ledgerRow("r1", shaA, pt("T1", 3, 3)),
		ledgerRow("lookup", shaB, pt("B1", 3, 3)),
		ledgerRow("r3", shaA, pt("T1", 0, 3)),
	}
	got := contextbench.Regress(rows, "")
	if got.BaselineRunID != "r1" || got.Verdict != contextbench.VerdictFail {
		t.Fatalf("want baseline r1 and a fail, got %+v", got)
	}
}

func TestRegress_ExplicitBaselineMustShareTheCorpus(t *testing.T) {
	rows := []contextbench.LedgerRow{
		ledgerRow("r1", shaB, pt("T1", 3, 3)),
		ledgerRow("r2", shaA, pt("T1", 3, 3)),
		ledgerRow("r3", shaA, pt("T1", 0, 3)),
	}
	if got := contextbench.Regress(rows, "r1"); got.Verdict != contextbench.VerdictIndeterminate || !strings.Contains(got.Reason, "different questions") {
		t.Errorf("a named baseline on another corpus is indeterminate: %+v", got)
	}
	if got := contextbench.Regress(rows, "r2"); got.Verdict != contextbench.VerdictFail {
		t.Errorf("a named same-corpus baseline compares: %+v", got)
	}
	if got := contextbench.Regress(rows, "nope"); got.Verdict != contextbench.VerdictIndeterminate {
		t.Errorf("an unknown baseline id is indeterminate: %+v", got)
	}
}

func TestRegress_NoTasksInCommonIsNothingToRegress(t *testing.T) {
	// Mirrors the kit's Regression: an empty check set passes, stated
	// explicitly.
	rows := []contextbench.LedgerRow{
		ledgerRow("r1", shaA, pt("T1", 3, 3)),
		ledgerRow("r2", shaA, pt("T2", 0, 3)),
	}
	got := contextbench.Regress(rows, "")
	if got.Verdict != contextbench.VerdictPass || got.Compared != 0 || !strings.Contains(got.Reason, "nothing to regress") {
		t.Fatalf("got %+v", got)
	}
}

// ── PB-4: resume ─────────────────────────────────────────────────────

func TestMergeResumedResults_UnionFreshWinsSorted(t *testing.T) {
	prior := []contextbench.ArmResult{res("T2", "grep", 1, true, 1), res("T1", "fathom", 2, false, 1), res("T1", "fathom", 1, true, 1)}
	fresh := []contextbench.ArmResult{res("T1", "fathom", 2, true, 9), res("T1", "grep", 1, true, 1)}
	got := contextbench.MergeResumedResults(prior, fresh)
	want := []contextbench.ArmResult{
		res("T1", "fathom", 1, true, 1), res("T1", "fathom", 2, true, 9), res("T1", "grep", 1, true, 1), res("T2", "grep", 1, true, 1),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merge = %+v\nwant %+v", got, want)
	}
	swapped := contextbench.MergeResumedResults(fresh, prior)
	if swapped[1].TotalTokens != 1 {
		t.Error("the second argument wins a collision")
	}
}

func TestCellsLost_CountsIntendedCellsThatNeverScored(t *testing.T) {
	tasks := []contextbench.Task{{ID: "T1"}, {ID: "T2"}}
	arms := []string{"fathom", "grep"}
	results := []contextbench.ArmResult{
		res("T1", "fathom", 1, true, 1), res("T1", "fathom", 2, true, 1), res("T1", "fathom", 3, true, 1),
		res("T1", "grep", 1, true, 1), res("T1", "grep", 2, true, 1),
		res("T2", "grep", 0, true, 1), // single-trial artifact: counts as trial 1
		res("T7", "grep", 1, true, 1), // never intended: not counted either way
	}
	// Intended 2 tasks × 2 arms × 3 = 12; present: T1/fathom×3, T1/grep×2, T2/grep×1 = 6.
	if got := contextbench.CellsLost(tasks, arms, 3, results); got != 6 {
		t.Fatalf("cells lost = %d, want 6", got)
	}
	if got := contextbench.CellsLost(tasks, arms, 0, results); got != 1 {
		t.Fatalf("repeat 0 reads as 1: lost = %d, want 1 (T2/fathom; T2/grep's trial-0 artifact counts as trial 1)", got)
	}
}
