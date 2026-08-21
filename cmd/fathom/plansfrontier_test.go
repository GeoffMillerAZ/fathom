package main

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// TestFrontierEligibilityDerivesFromNeeds pins WF-4: frontier rows carry
// derived eligible/blocked_on — eligible iff every needs: target is
// claimed-and-verified or skipped-with-disposition; a row with no needs:
// is eligible by definition. Pure function of (plan, evidence), computed
// at query time — nothing stored.
func TestFrontierEligibilityDerivesFromNeeds(t *testing.T) {
	rows := []planRow{
		{
			PlanFile: "p.md", Task: verify.Task{ID: "A-1", Title: "Base, claimed and proven", Claimed: true},
			Verdict: verify.TaskVerdict{Status: verify.Pass},
		},
		{PlanFile: "p.md", Task: verify.Task{ID: "A-2", Title: "Needs A-1 (met) and A-3 (missing)", Needs: []string{"A-1", "A-3"}}},
		{PlanFile: "p.md", Task: verify.Task{ID: "A-4", Title: "Needs nothing"}},
		{PlanFile: "p.md", Task: verify.Task{ID: "A-5", Title: "Needs a refused row", Needs: []string{"A-6"}}},
		{PlanFile: "p.md", Task: verify.Task{ID: "A-6", Title: "Refused", Skipped: true, Disposition: "refused", Reason: "out of scope"}},
		{PlanFile: "p.md", Task: verify.Task{ID: "A-7", Title: "Needs A-8 (open, unmet)", Needs: []string{"A-8"}}},
		{PlanFile: "p.md", Task: verify.Task{ID: "A-8", Title: "Open dependency"}},
	}

	out := frontierForPlan(rows, nil, time.Now())
	byID := map[string]frontierRow{}
	for _, r := range out {
		byID[r.ID] = r
	}

	// A-1 is claimed and proven: excluded from the frontier entirely
	// (pre-existing E3-2 behaviour) — never appears in out.
	if _, ok := byID["A-1"]; ok {
		t.Error("A-1 (claimed and proven) should not appear in the frontier at all")
	}
	// A-6 is skipped: also excluded from the frontier.
	if _, ok := byID["A-6"]; ok {
		t.Error("A-6 (skipped) should not appear in the frontier at all")
	}

	a2, ok := byID["A-2"]
	if !ok {
		t.Fatal("A-2 missing from frontier")
	}
	if a2.Eligible {
		t.Error("A-2 should be blocked: needs A-3, which does not exist")
	}
	if len(a2.BlockedOn) != 1 || a2.BlockedOn[0].ID != "A-3" || a2.BlockedOn[0].State != "missing" {
		t.Errorf("A-2 BlockedOn = %+v, want [{A-3 missing}]", a2.BlockedOn)
	}

	a4, ok := byID["A-4"]
	if !ok {
		t.Fatal("A-4 missing from frontier")
	}
	if !a4.Eligible || len(a4.BlockedOn) != 0 {
		t.Errorf("A-4 (no needs:) should be eligible with no blockers, got %+v", a4)
	}

	a5, ok := byID["A-5"]
	if !ok {
		t.Fatal("A-5 missing from frontier")
	}
	if !a5.Eligible {
		t.Errorf("A-5 should be eligible: its only needs-target is skipped-with-disposition, blocked_on=%+v", a5.BlockedOn)
	}

	a7, ok := byID["A-7"]
	if !ok {
		t.Fatal("A-7 missing from frontier")
	}
	if a7.Eligible {
		t.Error("A-7 should be blocked: needs A-8, which is open (unmet)")
	}
	if len(a7.BlockedOn) != 1 || a7.BlockedOn[0].ID != "A-8" || a7.BlockedOn[0].State != "open" {
		t.Errorf("A-7 BlockedOn = %+v, want [{A-8 open}]", a7.BlockedOn)
	}
}

// TestFrontierDecoratesAndFreesLeases pins WF-6: `plans next` decorates a
// frontier row holding a fresh lease; an expired lease is invisible — the
// row simply reappears undecorated.
func TestFrontierDecoratesAndFreesLeases(t *testing.T) {
	rows := []planRow{
		{PlanFile: "docs/plans/p.md", Task: verify.Task{ID: "A-1", Title: "Open row"}},
		{PlanFile: "docs/plans/p.md", Task: verify.Task{ID: "A-2", Title: "Also open"}},
	}
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	leases := []leaseRow{
		{
			Plan: "docs/plans/p.md", RowID: "A-1", Holder: "sess-1", Harness: "claude",
			Filed: now.Add(-10 * time.Minute), Expires: now.Add(50 * time.Minute),
		}, // fresh
		{
			Plan: "docs/plans/p.md", RowID: "A-2", Holder: "sess-2", Harness: "claude",
			Filed: now.Add(-3 * time.Hour), Expires: now.Add(-1 * time.Hour),
		}, // expired
	}

	out := frontierForPlan(rows, leases, now)
	byID := map[string]frontierRow{}
	for _, r := range out {
		byID[r.ID] = r
	}

	a1 := byID["A-1"]
	if !a1.Leased || a1.LeaseHolder != "sess-1" {
		t.Errorf("A-1 should show a fresh lease from sess-1, got %+v", a1)
	}
	a2 := byID["A-2"]
	if a2.Leased {
		t.Errorf("A-2's lease is expired — must be invisible, got %+v", a2)
	}

	// --free semantics: filtering leased rows out leaves only A-2.
	var free []frontierRow
	for _, r := range out {
		if !r.Leased {
			free = append(free, r)
		}
	}
	if len(free) != 1 || free[0].ID != "A-2" {
		t.Errorf("free rows = %+v, want only A-2", free)
	}
}

// TestCmdPlansNext_EligibleOnlyAndFree is the CLI-level smoke test: both
// flags actually filter `plans next`'s output end-to-end, through
// cmdPlansNext → frontierForPlan (WF-4/WF-6's reachable_from anchor).
func TestCmdPlansNext_EligibleOnlyAndFree(t *testing.T) {
	storePath := seedPlansFixtureStore(t)
	root := t.TempDir()
	pf := writeTempPlan(t, "# EF\n\n"+
		"- [ ] Free and eligible {id: E-1}\n"+
		`- [ ] Blocked on a missing dep {id: E-2, needs: E-9}`+"\n")

	// Lease E-1 so it should disappear under --free.
	var leaseOut, leaseErr bytes.Buffer
	if code := run([]string{"plans", "lease", "E-1", "--plan", pf, "--holder", "sess-1", "--ttl", "2h", "--root", root}, &leaseOut, &leaseErr); code != 0 {
		t.Fatalf("plans lease exit = %d, stderr: %s", code, leaseErr.String())
	}

	var out, errBuf bytes.Buffer
	code := run([]string{
		"plans", "next", "--service", "payment-service", "--store", storePath,
		"--plan", pf, "--root", root, "--json",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("plans next exit = %d, stderr: %s", code, errBuf.String())
	}
	var rows []frontierRow
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("plans next --json not valid JSON: %v\n%s", err, out.String())
	}
	byID := map[string]frontierRow{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	if !byID["E-1"].Leased {
		t.Errorf("E-1 should be decorated with the fresh lease, got %+v", byID["E-1"])
	}
	if byID["E-2"].Eligible {
		t.Errorf("E-2 should be blocked (needs E-9, which does not exist), got %+v", byID["E-2"])
	}

	var eligOut, eligErr bytes.Buffer
	if code := run([]string{
		"plans", "next", "--service", "payment-service", "--store", storePath,
		"--plan", pf, "--root", root, "--eligible-only", "--json",
	}, &eligOut, &eligErr); code != 0 {
		t.Fatalf("plans next --eligible-only exit = %d, stderr: %s", code, eligErr.String())
	}
	var eligRows []frontierRow
	if err := json.Unmarshal(eligOut.Bytes(), &eligRows); err != nil {
		t.Fatalf("--eligible-only --json not valid JSON: %v\n%s", err, eligOut.String())
	}
	for _, r := range eligRows {
		if r.ID == "E-2" {
			t.Error("--eligible-only must filter out E-2 (blocked)")
		}
	}

	var freeOut, freeErr bytes.Buffer
	if code := run([]string{
		"plans", "next", "--service", "payment-service", "--store", storePath,
		"--plan", pf, "--root", root, "--free", "--json",
	}, &freeOut, &freeErr); code != 0 {
		t.Fatalf("plans next --free exit = %d, stderr: %s", code, freeErr.String())
	}
	var freeRows []frontierRow
	if err := json.Unmarshal(freeOut.Bytes(), &freeRows); err != nil {
		t.Fatalf("--free --json not valid JSON: %v\n%s", err, freeOut.String())
	}
	for _, r := range freeRows {
		if r.ID == "E-1" {
			t.Error("--free must filter out E-1 (leased)")
		}
	}

	// Human output names the block and the lease.
	var humanOut, humanErr bytes.Buffer
	if code := run([]string{
		"plans", "next", "--service", "payment-service", "--store", storePath,
		"--plan", pf, "--root", root,
	}, &humanOut, &humanErr); code != 0 {
		t.Fatalf("plans next (human) exit = %d, stderr: %s", code, humanErr.String())
	}
	if !bytes.Contains(humanOut.Bytes(), []byte("blocked on E-9")) {
		t.Errorf("human output missing the blocked-on suffix:\n%s", humanOut.String())
	}
	if !bytes.Contains(humanOut.Bytes(), []byte("leased (sess-1,")) {
		t.Errorf("human output missing the leased suffix:\n%s", humanOut.String())
	}
}
