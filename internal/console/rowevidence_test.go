package console

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestRowEvidence_ReachPathAndOracleMatch pins OC-23: a row's evidence
// names its contract field by field, the gate's verdict with every
// criterion, the call path the reach claim stands on (the named test to
// the symbol, hop by hop, with every reaching test listed and bounded), or
// for a lexical row the line the pattern matched with its neighbors — and
// its history from the plan's commits, where per-row refutations are
// absent (the verify ledger keeps per-plan counts), never zero.
func TestRowEvidence_ReachPathAndOracleMatch(t *testing.T) {
	plan := PlanFacts{Service: "proof", Path: "/r/docs/plans/settlement.md", Rel: "docs/plans/settlement.md", Title: "Settlement", Rows: fixtureRows(), Verified: true}
	plan.Rows[1].Criteria = []CriterionFact{{Name: "symbol_present", Status: "pass", Detail: "settle.go:24"}, {Name: "test_reaches", Status: "pass", Detail: "TestReconcile_Matches"}}
	reach := &ReachFacts{
		Symbol: "Reconcile",
		Tests:  []ReachTest{{Test: "proof.TestReconcile_Matches"}, {Test: "proof.TestAll"}, {Test: "proof.TestMore", ViaDispatch: true, Implementers: 2}},
		From:   "proof.TestReconcile_Matches",
		Path:   []string{"proof.TestReconcile_Matches", "proof.reconcileBoth", "proof.Reconcile"},
	}
	history := []RowRevision{
		{Commit: "aaaaaaaa11", At: time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC), Present: true},
		{Commit: "bbbbbbbb22", At: time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC), Present: true, Claimed: true},
		{Commit: "cccccccc33", At: time.Date(2026, 9, 16, 9, 0, 0, 0, time.UTC), Present: true},
		{Commit: "dddddddd44", At: time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC), Present: true, Claimed: true},
	}
	v := RowEvidence(RowEvidenceInput{Service: "proof", Plan: plan, RowID: "S-2", Reach: reach, History: history, MaxTests: 2})
	if !v.Found || v.State != RowProven || v.Contract.Kind != ContractReach {
		t.Fatalf("row = %+v", v)
	}
	if v.Reach.State != StatePresent || v.Reach.Hops != 2 || strings.Join(v.Reach.Path, " → ") != "proof.TestReconcile_Matches → proof.reconcileBoth → proof.Reconcile" {
		t.Errorf("reach = %+v", v.Reach)
	}
	if v.Reach.Total != 3 || len(v.Reach.Tests) != 2 || v.Reach.Omitted != 1 {
		t.Errorf("reaching tests must be bounded and counted: %+v", v.Reach)
	}
	if len(v.Verdict.Criteria) != 2 || v.Verdict.Criteria[1].Detail != "TestReconcile_Matches" {
		t.Errorf("verdict = %+v", v.Verdict)
	}
	if v.Oracle.State != StateAbsent || !strings.Contains(v.Oracle.Reason, "does not apply") {
		t.Errorf("a reach row has no oracle: %+v", v.Oracle)
	}
	h := v.History
	if h.Added != "aaaaaaaa (2026-09-10 09:00)" || h.FirstClaimed != "bbbbbbbb (2026-09-14 09:00)" || h.Reopened != 1 || len(h.Changes) != 4 {
		t.Errorf("history = %+v", h)
	}
	if h.Refutations.State != StateAbsent || !strings.Contains(h.Refutations.Reason, "per-plan counts") {
		t.Errorf("per-row refutations are not measured: %+v", h.Refutations)
	}
	raw, _ := json.Marshal(h.Refutations)
	if strings.Contains(string(raw), `"v"`) {
		t.Errorf("absent refutations must carry no number: %s", raw)
	}

	// A lexical row: the matched line with its neighbors.
	text := "# proof\n\nintro\n\n## Window\n\nruns nightly\n"
	m := MatchOracle("README.md", "^## Window$", text, 1)
	if m.Count != 1 || m.Line != 5 || m.FirstLine != 4 || strings.Join(m.Lines, "|") != "|## Window|" {
		t.Fatalf("match = %+v", m)
	}
	v = RowEvidence(RowEvidenceInput{Service: "proof", Plan: plan, RowID: "S-3", Oracle: &m})
	if v.Contract.Kind != ContractLexical || v.Oracle.State != StatePresent || v.Oracle.Line != 5 {
		t.Errorf("oracle = %+v", v.Oracle)
	}
	if v.Reach.State != StateAbsent || !strings.Contains(v.Reach.Reason, "no reach claim") {
		t.Errorf("a lexical row makes no reach claim: %+v", v.Reach)
	}
	if v.State != RowClaimedFailing {
		t.Errorf("S-3 state = %q", v.State)
	}
	if m := MatchOracle("README.md", "^## Nope$", text, 1); m.Count != 0 || m.Line != 0 {
		t.Errorf("no match = %+v", m)
	}
	if m := MatchOracle("README.md", "(", text, 1); m.Err == "" {
		t.Error("a pattern that does not compile must say so")
	}
	// A test that reaches nothing is a measured zero, not a failure to read.
	v = RowEvidence(RowEvidenceInput{Service: "proof", Plan: plan, RowID: "S-1", Reach: &ReachFacts{Symbol: "Settle"}})
	if v.Reach.State != StateZero {
		t.Errorf("no reaching test = %+v", v.Reach)
	}
	// A graph that could not be read is an error.
	v = RowEvidence(RowEvidenceInput{Service: "proof", Plan: plan, RowID: "S-1", Reach: &ReachFacts{Err: "store closed"}})
	if v.Reach.State != StateError {
		t.Errorf("graph error = %+v", v.Reach)
	}
	// An unknown row is named.
	v = RowEvidence(RowEvidenceInput{Service: "proof", Plan: plan, RowID: "NOPE"})
	if v.Found || v.Verdict.State != "missing" {
		t.Errorf("missing row = %+v", v)
	}
}
