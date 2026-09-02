package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// TestMeasuredStaleReading pins the one definition every reader shares
// (PC-5): unknown daemon → not stale; daemon ≠ the verifying binary → stale;
// daemon ≠ the tree commit → stale ONLY inside the fathom source tree (O5).
func TestMeasuredStaleReading(t *testing.T) {
	const a, b = "6c6f4377b53aad47cf6ab86224721df8592cadf0", "3f0c3b1deadbeef0000000000000000000000000"
	cases := []struct {
		name       string
		row        verifyLedgerRow
		fathomTree bool
		want       bool
	}{
		{"unknown daemon is not stale", verifyLedgerRow{BinaryCommit: a, Commit: b}, true, false},
		{"daemon matches binary and tree", verifyLedgerRow{DaemonCommit: a, BinaryCommit: a, Commit: a}, true, false},
		{"daemon differs from the verifying binary", verifyLedgerRow{DaemonCommit: a, BinaryCommit: b}, false, true},
		{"O20: daemon and CLI lag HEAD together, in the fathom tree", verifyLedgerRow{DaemonCommit: a, BinaryCommit: a, Commit: b}, true, true},
		{"adopter repo: tree commit is never compared", verifyLedgerRow{DaemonCommit: a, BinaryCommit: a, Commit: b}, false, false},
		{"dirty stamp still names the commit", verifyLedgerRow{DaemonCommit: a + "-dirty", BinaryCommit: a[:12]}, false, false},
	}
	for _, c := range cases {
		if got := measuredStaleReading(c.row, c.fathomTree); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// TestMarkMeasuredStale: rows of a stale reading are flagged and counted;
// rows of a current reading are untouched; plans come back sorted.
func TestMarkMeasuredStale(t *testing.T) {
	const a, b = "6c6f4377b53a", "3f0c3b1deadb"
	rows := []intent.PlanRowState{
		{Plan: "docs/plans/z.md", ID: "row 1", Claimed: true, Failing: true},
		{Plan: "docs/plans/a.md", ID: "row 1", Claimed: true, Failing: true},
		{Plan: "docs/plans/a.md", ID: "2 claimed row(s) with no contract", Claimed: true, Unknown: true},
		{Plan: "docs/plans/ok.md", ID: "row 1", Claimed: true, Failing: true},
	}
	newest := map[string]verifyLedgerRow{
		"docs/plans/z.md":  {DaemonCommit: a, BinaryCommit: b},
		"docs/plans/a.md":  {DaemonCommit: a, BinaryCommit: b},
		"docs/plans/ok.md": {DaemonCommit: a, BinaryCommit: a},
	}
	n, plans := markMeasuredStale(rows, newest, false)
	if n != 3 {
		t.Errorf("flagged = %d, want 3", n)
	}
	if strings.Join(plans, ",") != "docs/plans/a.md,docs/plans/z.md" {
		t.Errorf("plans = %v, want sorted a,z", plans)
	}
	if rows[3].MeasuredStale {
		t.Error("a current reading's row must not be flagged")
	}
	// The audit turns flagged rows into one NOT COMPUTED line, never U1/U2.
	a2 := intent.RunAudit(intent.AuditInput{PlanRows: rows})
	for _, c := range a2.Unproven {
		if c.Kind == "U1" || c.Kind == "U2" {
			if c.Ref != "docs/plans/ok.md row 1" {
				t.Errorf("stale row surfaced as %s %s — must be a coverage gap", c.Kind, c.Ref)
			}
		}
	}
	found := false
	for _, u := range a2.Uncomputable {
		if strings.Contains(u, "3 claimed row(s) in 2 plan(s) measured-stale") {
			found = true
		}
	}
	if !found {
		t.Errorf("audit must name the measured-stale count: %v", a2.Uncomputable)
	}
	if countMeasuredStale(rows) != 3 {
		t.Errorf("countMeasuredStale = %d, want 3", countMeasuredStale(rows))
	}
}

// TestRollupPlanStale: under a stale daemon, a claimed row it did not
// prove is measured-stale, never claimed-unproven; proven stays proven.
func TestRollupPlanStale(t *testing.T) {
	rows := []planRow{
		{Task: verify.Task{ID: "A", Claimed: true}, Verdict: verify.TaskVerdict{Status: verify.Pass}},
		{Task: verify.Task{ID: "B", Claimed: true}, Verdict: verify.TaskVerdict{Status: verify.Fail}},
		{Task: verify.Task{ID: "C"}},
	}
	fresh := rollupPlanStale("p.md", rows, false)
	if fresh.ClaimedUnproven != 1 || fresh.MeasuredStale != 0 || fresh.Proven != 1 || fresh.Open != 1 {
		t.Errorf("fresh rollup = %+v", fresh)
	}
	stale := rollupPlanStale("p.md", rows, true)
	if stale.ClaimedUnproven != 0 || stale.MeasuredStale != 1 || stale.Proven != 1 || stale.Open != 1 {
		t.Errorf("stale rollup = %+v", stale)
	}
	if plain := rollupPlan("p.md", rows); plain != fresh {
		t.Errorf("rollupPlan must equal the non-stale rollup: %+v vs %+v", plain, fresh)
	}
}

// TestLiveDaemonStale: a v1 daemon is unknown (not stale); a daemon whose
// stamp differs from this binary is stale with a reason naming both.
func TestLiveDaemonStale(t *testing.T) {
	hc := &http.Client{}
	v1 := statuszServer(t, "", false, "w")
	defer v1.Close()
	if stale, why := liveDaemonStale(hc, v1.URL, t.TempDir()); stale || why != "" {
		t.Errorf("v1 daemon must be unknown, got stale=%v why=%q", stale, why)
	}
	other := statuszServer(t, "0000000000000000000000000000000000000000", false, "w")
	defer other.Close()
	stale, why := liveDaemonStale(hc, other.URL, t.TempDir())
	if !stale || !strings.Contains(why, "000000000000") {
		t.Errorf("foreign daemon must be stale with its stamp named: stale=%v why=%q", stale, why)
	}
}
