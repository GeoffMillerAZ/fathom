package console

import (
	"strings"
	"testing"
)

// TestWiringView_DoctorDriftReceipts pins OC-40: doctor's repository-side
// rows grouped (wiring first) and counted; drift is doctor's failing or
// warning wiring rows plus the preview plan's apply and BLOCKED items, each
// with its remedy; receipts listed with their total. A root that is not on
// this machine makes every figure absent with the reason — never
// "0 failing" — and a plan that could not be computed makes drift a named
// lower bound, never a clean zero.
func TestWiringView_DoctorDriftReceipts(t *testing.T) {
	f := WiringFacts{
		Service: "fathom", Root: "/r/sextant", Source: "fathom doctor --repo-only",
		Checks: []WiringCheck{
			{Name: "config_valid", Status: "pass", Reason: "config parses"},
			{Name: "hooks_installed", Status: "fail", Reason: "no PostToolUse hook", Remedy: "fathom init"},
			{Name: "stanza_present", Status: "pass", Reason: "stanza v3"},
			{Name: "guard_file", Status: "skipped", Reason: "no guard.cue"},
			{Name: "wiring_drift", Status: "warn", Reason: "CLAUDE.md region hand-edited", Remedy: "fathom fleet reconcile"},
		},
		Plan: []WiringPlanItem{
			{File: "CLAUDE.md", Target: "fathom stanza", Verdict: "blocked", Cause: "drift", Reason: "region edited by hand"},
			{File: ".claude/settings.json", Target: "PostToolUse", Verdict: "apply", Reason: "hook missing"},
			{File: "AGENTS.md", Target: "fathom stanza", Verdict: "skip", Reason: "current"},
		},
		PlanDigest:    "sha256:abc",
		Receipts:      []WiringReceipt{{ID: "r2", Time: "2026-09-25T10:00:00Z", Files: 2}, {ID: "r1", Time: "2026-09-20T10:00:00Z", Files: 1}},
		ReceiptsTotal: 5,
	}
	m := WiringView(f)
	if len(m.Checks) != 5 || m.Checks[0].Group != "wiring" || m.Checks[0].Name != "hooks_installed" || m.Checks[4].Group != "configuration" {
		t.Errorf("checks grouped wiring first in doctor's order: %+v", m.Checks)
	}
	if m.Pass.V != 2 || m.Fail.V != 1 || m.Warn.V != 1 || m.Skip.V != 1 || !strings.Contains(m.Fail.Denominator, "of 5 repository-side doctor checks") {
		t.Errorf("counts = pass %+v warn %+v fail %+v skipped %+v", m.Pass, m.Warn, m.Fail, m.Skip)
	}
	var drift []string
	for _, d := range m.Drift {
		drift = append(drift, d.Source+":"+d.What)
		if d.Remedy == "" {
			t.Errorf("drift %q carries no remedy", d.What)
		}
	}
	if got := strings.Join(drift, " | "); got != "doctor:hooks_installed | doctor:wiring_drift | plan:CLAUDE.md · fathom stanza | plan:.claude/settings.json · PostToolUse" {
		t.Errorf("drift = %s", got)
	}
	if m.DriftCount.V != 4 || m.PlanApply.V != 1 || m.PlanBlocked.V != 1 || m.PlanCurrent.V != 1 || m.PlanDigest != "sha256:abc" {
		t.Errorf("drift %+v, plan apply %+v blocked %+v current %+v", m.DriftCount, m.PlanApply, m.PlanBlocked, m.PlanCurrent)
	}
	if m.ReceiptsTotal.V != 5 || len(m.Receipts) != 2 || !strings.Contains(m.ReceiptsTotal.Denominator, "the newest 2 listed") {
		t.Errorf("receipts = %+v (%d listed)", m.ReceiptsTotal, len(m.Receipts))
	}

	// No local root: absent everywhere, with the reason.
	gone := WiringView(WiringFacts{Service: "remote", RootNote: "the registered root /x is not a directory on this machine"})
	for name, v := range map[string]Value[int]{"fail": gone.Fail, "drift": gone.DriftCount, "receipts": gone.ReceiptsTotal, "apply": gone.PlanApply} {
		if v.State != StateAbsent || !strings.Contains(v.Reason, "not a directory") {
			t.Errorf("%s with no root = %+v", name, v)
		}
	}
	// A plan that failed: drift is a lower bound from doctor alone.
	f.PlanErr = "CLAUDE.md: permission denied"
	partial := WiringView(f)
	if partial.DriftCount.State != StatePartial || partial.DriftCount.V != 2 || partial.PlanApply.State != StateError {
		t.Errorf("failed plan = drift %+v apply %+v", partial.DriftCount, partial.PlanApply)
	}
	// Receipts that cannot be read are an error, not "none".
	f.ReceiptsErr = "receipts.jsonl: EPERM"
	if bad := WiringView(f); bad.ReceiptsTotal.State != StateError {
		t.Errorf("unreadable receipts = %+v", bad.ReceiptsTotal)
	}
}
