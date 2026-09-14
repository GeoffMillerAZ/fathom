package verify_test

// selfskip_test.go — O56 defect A: pure fixture-plan coverage for
// AnnotateSelfSkip/OverlaySelfSkip and verifyTask's skip_test criterion
// (skip-without-reason, self-skipped, and a reasoned/predating skip that
// passes). The git-backed end-to-end path (a throwaway git repo driving
// `fathom verify`) lives in cmd/fathom/selfskip_test.go; this file is the
// no-I/O half — every scenario expressed directly as Plan fixtures.

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// TestVerifyPlan_SkipWithoutReason: a bare `skip_test: true` (no `skip:`
// reason) is a grammar error — refused as skip-without-reason, counted
// unproven — independent of AnnotateSelfSkip entirely.
func TestVerifyPlan_SkipWithoutReason(t *testing.T) {
	ev := fakeEvidence{
		symbols: map[string][]core.SymbolNode{"Foo": {node("Foo", "a.go", 1, core.TierVerified)}},
	}
	rep := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Claimed: true, Expect: verify.Expect{Symbol: "Foo", SkipTest: true}},
	}}, ev)
	v := rep.Tasks[0]
	if v.Status != verify.Fail || v.Label != "skip-without-reason" {
		t.Fatalf("got %s/%s, want fail/skip-without-reason", v.Status, v.Label)
	}
}

// TestVerifyPlan_SkipWithReason_NotSelfSkipped_Passes: a reasoned skip_test
// on a row AnnotateSelfSkip never touched (SelfSkipped stays false) passes.
func TestVerifyPlan_SkipWithReason_NotSelfSkipped_Passes(t *testing.T) {
	ev := fakeEvidence{
		symbols: map[string][]core.SymbolNode{"Foo": {node("Foo", "a.go", 1, core.TierVerified)}},
	}
	rep := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Claimed: true, Expect: verify.Expect{Symbol: "Foo", SkipTest: true, SkipReason: "cmd oracle pending"}},
	}}, ev)
	v := rep.Tasks[0]
	if v.Status != verify.Pass || v.Label != "verified" {
		t.Fatalf("got %s/%s, want pass/verified", v.Status, v.Label)
	}
}

// TestVerifyPlan_SelfSkipped_Refused: a row AnnotateSelfSkip flagged fails
// outright, whatever its skip: reason says.
func TestVerifyPlan_SelfSkipped_Refused(t *testing.T) {
	ev := fakeEvidence{
		symbols: map[string][]core.SymbolNode{"Foo": {node("Foo", "a.go", 1, core.TierVerified)}},
	}
	rep := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Claimed: true, SelfSkipped: true, Expect: verify.Expect{Symbol: "Foo", SkipTest: true, SkipReason: "a perfectly good reason"}},
	}}, ev)
	v := rep.Tasks[0]
	if v.Status != verify.Fail || v.Label != "self-skipped" {
		t.Fatalf("got %s/%s, want fail/self-skipped", v.Status, v.Label)
	}
}

// TestAnnotateSelfSkip_BareSkip_NotFlagged: AnnotateSelfSkip only ever
// flags a row that IS claimed AND DOES carry skip_test — a bare, unreasoned
// skip is a SEPARATE failure (skip-without-reason) that does not need a
// prior plan to detect at all.
func TestAnnotateSelfSkip_BareSkip_NotFlagged(t *testing.T) {
	current := verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Claimed: true, Expect: verify.Expect{Symbol: "Foo", SkipTest: true}},
	}}
	out := verify.AnnotateSelfSkip(current, nil)
	if out.Tasks[0].SelfSkipped {
		t.Fatalf("bare skip_test must not be flagged self-skipped by AnnotateSelfSkip: %+v", out.Tasks[0])
	}
}

// TestAnnotateSelfSkip_PredatesClaim_NotFlagged: the skip was already
// present in prior (whether or not that row was claimed yet) — stands.
func TestAnnotateSelfSkip_PredatesClaim_NotFlagged(t *testing.T) {
	prior := verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Claimed: false, Expect: verify.Expect{Symbol: "Foo", SkipTest: true, SkipReason: "predates the claim"}},
	}}
	current := verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Claimed: true, Expect: verify.Expect{Symbol: "Foo", SkipTest: true, SkipReason: "predates the claim"}},
	}}
	out := verify.AnnotateSelfSkip(current, &prior)
	if out.Tasks[0].SelfSkipped {
		t.Fatalf("a skip present in prior must stand, not self-skip: %+v", out.Tasks[0])
	}
}

// TestAnnotateSelfSkip_AddedWithClaim_Flagged: prior has the row WITHOUT
// skip_test; current claims it AND adds skip_test — flagged.
func TestAnnotateSelfSkip_AddedWithClaim_Flagged(t *testing.T) {
	prior := verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Claimed: false, Expect: verify.Expect{Symbol: "Foo"}},
	}}
	current := verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Claimed: true, Expect: verify.Expect{Symbol: "Foo", SkipTest: true, SkipReason: "added right now"}},
	}}
	out := verify.AnnotateSelfSkip(current, &prior)
	if !out.Tasks[0].SelfSkipped {
		t.Fatalf("a skip added alongside the claim must be flagged self-skipped: %+v", out.Tasks[0])
	}
}

// TestAnnotateSelfSkip_AddedWithClaim_RowAbsentFromPrior_Flagged: prior
// exists (a real comparison happened) but has NO row for this ID at all —
// same treatment as "did not have the skip."
func TestAnnotateSelfSkip_AddedWithClaim_RowAbsentFromPrior_Flagged(t *testing.T) {
	prior := verify.Plan{} // e.g. the plan file did not exist at that git ref
	current := verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Claimed: true, Expect: verify.Expect{Symbol: "Foo", SkipTest: true, SkipReason: "born claimed"}},
	}}
	out := verify.AnnotateSelfSkip(current, &prior)
	if !out.Tasks[0].SelfSkipped {
		t.Fatalf("a row absent from a real prior plan must be flagged self-skipped: %+v", out.Tasks[0])
	}
}

// TestAnnotateSelfSkip_NilPrior_Inapplicable_NeverFlagged: prior == nil
// (no git, or the caller determined the check does not apply) never flags
// anything — fail-open, the same discipline as an Unknown verdict.
func TestAnnotateSelfSkip_NilPrior_Inapplicable_NeverFlagged(t *testing.T) {
	current := verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Claimed: true, Expect: verify.Expect{Symbol: "Foo", SkipTest: true, SkipReason: "no git here"}},
	}}
	out := verify.AnnotateSelfSkip(current, nil)
	if out.Tasks[0].SelfSkipped {
		t.Fatalf("nil prior must never flag self-skipped: %+v", out.Tasks[0])
	}
}

// TestAnnotateSelfSkip_UnclaimedRow_NeverFlagged: AnnotateSelfSkip only
// evaluates CLAIMED rows — an open row gaining a skip is the edit hook's
// note (cmd/fathom/edithook_skipnote.go), never a gate failure.
func TestAnnotateSelfSkip_UnclaimedRow_NeverFlagged(t *testing.T) {
	prior := verify.Plan{Tasks: []verify.Task{{ID: "T-1", Expect: verify.Expect{Symbol: "Foo"}}}}
	current := verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Claimed: false, Expect: verify.Expect{Symbol: "Foo", SkipTest: true, SkipReason: "not claimed yet"}},
	}}
	out := verify.AnnotateSelfSkip(current, &prior)
	if out.Tasks[0].SelfSkipped {
		t.Fatalf("an unclaimed row must never be flagged self-skipped: %+v", out.Tasks[0])
	}
}

// TestOverlaySelfSkip_ForcesFailAndRecountsSummary: the HTTP-mode overlay
// (mirroring OverlayLocal for the lexical oracle) forces a flagged row's
// report entry to Fail/self-skipped even when the daemon-computed verdict
// (built with no git access) read Pass, and recomputes the summary.
func TestOverlaySelfSkip_ForcesFailAndRecountsSummary(t *testing.T) {
	rep := verify.Report{
		Tasks: []verify.TaskVerdict{
			{TaskID: "T-1", Status: verify.Pass, Label: "verified", Claimed: true},
			{TaskID: "T-2", Status: verify.Pass, Label: "verified", Claimed: true},
		},
		Summary: verify.Summary{Total: 2, Pass: 2},
	}
	plan := verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Claimed: true, SelfSkipped: true, Expect: verify.Expect{Symbol: "Foo", SkipTest: true, SkipReason: "self-graded"}},
		{ID: "T-2", Claimed: true, Expect: verify.Expect{Symbol: "Bar"}},
	}}
	verify.OverlaySelfSkip(&rep, plan)

	if rep.Tasks[0].Status != verify.Fail || rep.Tasks[0].Label != "self-skipped" {
		t.Fatalf("T-1 = %+v, want fail/self-skipped", rep.Tasks[0])
	}
	if rep.Tasks[1].Status != verify.Pass {
		t.Fatalf("T-2 must be untouched: %+v", rep.Tasks[1])
	}
	if rep.Summary != (verify.Summary{Total: 2, Pass: 1, Fail: 1}) {
		t.Fatalf("summary = %+v, want recomputed 1 pass / 1 fail", rep.Summary)
	}
}

// TestOverlaySelfSkip_NoFlags_LeavesReportByteForByte: a plan with no
// SelfSkipped rows must not even touch the summary (OverlaySelfSkip's own
// "untouched" short-circuit).
func TestOverlaySelfSkip_NoFlags_LeavesReportByteForByte(t *testing.T) {
	rep := verify.Report{
		Tasks:   []verify.TaskVerdict{{TaskID: "T-1", Status: verify.Pass, Label: "verified", Claimed: true}},
		Summary: verify.Summary{Total: 1, Pass: 1},
	}
	want := rep
	plan := verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Claimed: true, Expect: verify.Expect{Symbol: "Foo", SkipTest: true, SkipReason: "fine"}},
	}}
	verify.OverlaySelfSkip(&rep, plan)
	if rep.Summary != want.Summary ||
		rep.Tasks[0].TaskID != want.Tasks[0].TaskID ||
		rep.Tasks[0].Status != want.Tasks[0].Status ||
		rep.Tasks[0].Label != want.Tasks[0].Label ||
		len(rep.Tasks[0].Criteria) != len(want.Tasks[0].Criteria) {
		t.Fatalf("report changed with nothing flagged: got %+v, want %+v", rep, want)
	}
}
