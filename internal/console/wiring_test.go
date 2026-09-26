package console

import (
	"strings"
	"testing"
)

// TestWiringView_DoctorDriftReceipts pins OC-40: doctor's repository-side
// rows grouped (wiring first) and counted; drift is every thing on disk that
// differs, each counted ONCE — the preview plan's apply and BLOCKED items,
// the owned MCP entries and skills changed since fathom recorded them, and
// doctor's failing or warning wiring rows the plan does not itemize. A
// doctor row that summarizes plan items (hooks_installed: the hooks the plan
// would add; stanza_present: its regions; wiring_drift: its BLOCKED drift
// and the MCP/skill drift beyond the plan) is named on those items and
// never counted again on top of them. Receipts are listed with their
// total. A root that is not on this machine makes every figure absent with
// the reason — never "0 failing" — and a plan that could not be computed
// makes drift a named lower bound from doctor's rows alone, never a clean
// zero.
func TestWiringView_DoctorDriftReceipts(t *testing.T) {
	f := WiringFacts{
		Service: "fathom", Root: "/r/sextant", Source: "fathom doctor --repo-only", Harness: "claude",
		Checks: []WiringCheck{
			{Name: "config_valid", Status: "pass", Reason: "config parses"},
			{Name: "hooks_installed", Status: "fail", Reason: "no PostToolUse hook", Remedy: "fathom init"},
			{Name: "stanza_present", Status: "pass", Reason: "stanza v3"},
			{Name: "skills_installed", Status: "fail", Reason: "fathom-custodian SKILL.md is stale", Remedy: "fathom init"},
			{Name: "guard_file", Status: "skipped", Reason: "no guard.cue"},
			{Name: "wiring_drift", Status: "warn", Reason: "2 drift(s): CLAUDE.md region hand-edited; mcp ~/.claude.json fathom", Remedy: "fathom fleet reconcile"},
		},
		Plan: []WiringPlanItem{
			{File: "CLAUDE.md", Target: "fathom stanza", Kind: "region", Verdict: "blocked", Cause: "drift", Reason: "region edited by hand"},
			{File: ".claude/settings.json", Target: "PostToolUse", Kind: "key", Verdict: "apply", Reason: "hook missing"},
			{File: "AGENTS.md", Target: "fathom stanza", Kind: "region", Verdict: "skip", Reason: "current"},
		},
		BeyondPlan:    []WiringDrift{{What: "mcp ~/.claude.json mcpServers.fathom", Detail: "the entry changed since fathom recorded it"}},
		PlanDigest:    "sha256:abc",
		Receipts:      []WiringReceipt{{ID: "r2", Time: "2026-09-25T10:00:00Z", Files: 2}, {ID: "r1", Time: "2026-09-20T10:00:00Z", Files: 1}},
		ReceiptsTotal: 5,
	}
	m := WiringView(f)
	if len(m.Checks) != 6 || m.Checks[0].Group != "wiring" || m.Checks[0].Name != "hooks_installed" || m.Checks[5].Group != "configuration" {
		t.Errorf("checks grouped wiring first in doctor's order: %+v", m.Checks)
	}
	if m.Pass.V != 2 || m.Fail.V != 2 || m.Warn.V != 1 || m.Skip.V != 1 || !strings.Contains(m.Fail.Denominator, "of 6 repository-side doctor checks") {
		t.Errorf("counts = pass %+v warn %+v fail %+v skipped %+v", m.Pass, m.Warn, m.Fail, m.Skip)
	}
	driftOf := func(m WiringViewModel) string {
		var drift []string
		for _, d := range m.Drift {
			entry := d.Source + ":" + d.What
			if d.Doctor != "" {
				entry += " (" + d.Doctor + ")"
			}
			drift = append(drift, entry)
			if d.Remedy == "" {
				t.Errorf("drift %q carries no remedy", d.What)
			}
		}
		return strings.Join(drift, " | ")
	}
	// Four things differ: the stale skill (no plan item names it), the MCP
	// entry and the hand-edited region (wiring_drift's two), the missing
	// hook (hooks_installed's one) — never six with the two summaries added.
	if got := driftOf(m); got != "doctor:skills_installed | doctor:mcp ~/.claude.json mcpServers.fathom (wiring_drift) | plan:CLAUDE.md · fathom stanza (wiring_drift) | plan:.claude/settings.json · PostToolUse (hooks_installed)" {
		t.Errorf("drift = %s", got)
	}
	if m.DriftCount.V != 4 || m.PlanApply.V != 1 || m.PlanBlocked.V != 1 || m.PlanCurrent.V != 1 || m.PlanDigest != "sha256:abc" {
		t.Errorf("drift %+v, plan apply %+v blocked %+v current %+v", m.DriftCount, m.PlanApply, m.PlanBlocked, m.PlanCurrent)
	}
	if !strings.Contains(m.DriftCount.Denominator, "counted once") {
		t.Errorf("the denominator says how drift is counted: %q", m.DriftCount.Denominator)
	}
	// A failing row the plan does not itemize stands on its own: under
	// another harness hooks_installed reads that harness's files, which the
	// Claude-shaped plan does not name.
	codex := f
	codex.Harness = "codex"
	if got := driftOf(WiringView(codex)); !strings.HasPrefix(got, "doctor:hooks_installed | doctor:skills_installed |") || WiringView(codex).DriftCount.V != 5 {
		t.Errorf("under codex hooks_installed is its own drift: %s", got)
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
	// A plan that failed: drift is a lower bound from doctor's rows alone —
	// each failing row once, its own summary standing for what it names.
	f.PlanErr = "CLAUDE.md: permission denied"
	partial := WiringView(f)
	if partial.DriftCount.State != StatePartial || partial.DriftCount.V != 3 || partial.PlanApply.State != StateError {
		t.Errorf("failed plan = drift %+v apply %+v", partial.DriftCount, partial.PlanApply)
	}
	if got := driftOf(partial); got != "doctor:hooks_installed | doctor:skills_installed | doctor:wiring_drift" {
		t.Errorf("failed plan drift = %s", got)
	}
	// Receipts that cannot be read are an error, not "none".
	f.ReceiptsErr = "receipts.jsonl: EPERM"
	if bad := WiringView(f); bad.ReceiptsTotal.State != StateError {
		t.Errorf("unreadable receipts = %+v", bad.ReceiptsTotal)
	}
}
