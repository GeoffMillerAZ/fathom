package main

import (
	"strings"
	"testing"
)

// TestVerifyFigure (PC-13): one figure, denominators and window in the
// sentence, refusals counted as events, zero rows named as uninstrumented.
func TestVerifyFigure(t *testing.T) {
	rows := []verifyLedgerRow{
		{At: "2026-08-20T10:00:00Z", RepoRoot: "/r/a", Plan: "docs/plans/x.md", Claimed: 5, Verified: 3, Failing: 2},
		{At: "2026-08-15T09:00:00Z", RepoRoot: "/r/a", Plan: "docs/plans/x.md", Claimed: 5, Verified: 5},
		{At: "2026-08-25T12:00:00Z", RepoRoot: "/r/b", Plan: "docs/plans/y.md", Claimed: 2, Verified: 1, Failing: 1},
		{At: "2026-08-26T12:00:00Z", RepoRoot: "/r/b", Plan: "docs/plans/y.md", Claimed: 2, Verified: 1, Failing: 1},
	}
	f := verifyFigure(rows)
	if f.Runs != 4 || f.Refusals != 3 || f.FailingRows != 4 || f.Plans != 2 || f.Repos != 2 {
		t.Fatalf("figure = %+v", f)
	}
	if f.First != "2026-08-15T09:00:00Z" || f.Last != "2026-08-26T12:00:00Z" {
		t.Errorf("window = %s..%s", f.First, f.Last)
	}
	// Order-independent (determinism is a contract).
	rev := []verifyLedgerRow{rows[3], rows[2], rows[1], rows[0]}
	if verifyFigure(rev) != f {
		t.Error("verifyFigure depends on row order")
	}
	s := f.Sentence()
	for _, want := range []string{"refused 3 of 4 recorded runs", "2 plan(s)", "2 repo(s)", "2026-08-15", "2026-08-26", "4 failing-row readings", "NOT the figure", "verify-ledger.jsonl"} {
		if !strings.Contains(s, want) {
			t.Errorf("sentence lacks %q: %s", want, s)
		}
	}
	if e := verifyFigure(nil); e.Runs != 0 || !strings.Contains(e.Sentence(), "not instrumented") {
		t.Errorf("empty ledger must read as uninstrumented: %+v / %s", e, e.Sentence())
	}

	// The adoption gauge reads the same computation (one figure, not three).
	n, runs, _ := gateCatchStatsFromLedger(rows)
	if n == nil || *n != f.Refusals || runs != f.Runs {
		t.Errorf("gateCatchStatsFromLedger diverged from verifyFigure: %v/%d vs %+v", n, runs, f)
	}
}
