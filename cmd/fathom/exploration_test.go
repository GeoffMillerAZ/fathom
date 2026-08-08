package main

import (
	"strings"
	"testing"
	"time"
)

// TestExplorationLedger pins EX-1 (docs/plans/exploration-ledger.md): the
// transcript parser measures BOTH discovery modes in one session — raw
// exploration (Grep/Glob/Read) and indexed discovery (fathom MCP calls) —
// with result-size token estimates from the same estimator, plus repeated
// reads (the subagent-multiplier tell).
func TestExplorationLedger(t *testing.T) {
	lines := []string{
		// main chain: a fathom query answers small
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"f1","name":"mcp__fathom__find_symbol"}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"f1","content":[{"type":"text","text":"` + strings.Repeat("a", 400) + `"}]}]}}`,
		// main chain: raw exploration answers big
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"r1","name":"Read","input":{"file_path":"/repo/big.go"}}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"r1","content":[{"type":"text","text":"` + strings.Repeat("b", 4000) + `"}]}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"r2","name":"Grep"}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"r2","content":[{"type":"text","text":"` + strings.Repeat("c", 2000) + `"}]}]}}`,
		// a sidechain re-reads the SAME file: duplicated exploration
		`{"type":"assistant","isSidechain":true,"message":{"content":[{"type":"tool_use","id":"r3","name":"Read","input":{"file_path":"/repo/big.go"}}]}}`,
		`{"type":"user","isSidechain":true,"message":{"content":[{"type":"tool_result","tool_use_id":"r3","content":[{"type":"text","text":"` + strings.Repeat("d", 4000) + `"}]}]}}`,
	}
	s := summarizeTranscriptBytes([]byte(strings.Join(lines, "\n")))

	if s.RawDiscoveryCalls != 3 {
		t.Errorf("RawDiscoveryCalls = %d, want 3 (2 Reads + 1 Grep)", s.RawDiscoveryCalls)
	}
	if s.FathomCalls != 1 {
		t.Errorf("FathomCalls = %d, want 1", s.FathomCalls)
	}
	if s.RawDiscoveryTokensEst == nil || *s.RawDiscoveryTokensEst < 2000 {
		t.Errorf("RawDiscoveryTokensEst = %v, want ≈2500 (10000 chars / 4)", s.RawDiscoveryTokensEst)
	}
	if s.FathomTokensEst == nil || *s.FathomTokensEst > 200 {
		t.Errorf("FathomTokensEst = %v, want ≈100 (400 chars / 4)", s.FathomTokensEst)
	}
	if s.RepeatedReadFiles != 1 {
		t.Errorf("RepeatedReadFiles = %d, want 1 (/repo/big.go read twice)", s.RepeatedReadFiles)
	}
	if s.RepeatedReadTokensEst == nil || *s.RepeatedReadTokensEst < 500 {
		t.Errorf("RepeatedReadTokensEst = %v, want ≈1000 (the second read's 4000 chars / 4)", s.RepeatedReadTokensEst)
	}
	if s.SidechainToolResults != 1 {
		t.Errorf("SidechainToolResults = %d, want 1", s.SidechainToolResults)
	}
	if !strings.Contains(s.ExplorationNote, "estimate") {
		t.Errorf("ExplorationNote must label the estimator, got %q", s.ExplorationNote)
	}
}

// TestExplorationLedger_EmptyTranscript: absence carries a note, never zeros
// dressed as measurements.
func TestExplorationLedger_EmptyTranscript(t *testing.T) {
	s := summarizeTranscriptBytes(nil)
	if s.RawDiscoveryTokensEst != nil || s.FathomTokensEst != nil {
		t.Error("empty transcript must not fabricate token estimates")
	}
}

// TestLedgerExplorationFields pins EX-2: the builder maps the summary's
// exploration ledger onto the session row.
func TestLedgerExplorationFields(t *testing.T) {
	raw, idx, rep := 9000, 100, 4000
	ts := transcriptSummary{
		RawDiscoveryCalls: 3, RawDiscoveryTokensEst: &raw,
		FathomCalls: 5, FathomTokensEst: &idx,
		RepeatedReadFiles: 1, RepeatedReadTokensEst: &rep,
		SidechainToolResults: 2, ExplorationNote: "token figures are an estimate",
		TokensNote: "n/a",
	}
	e := buildSessionLedgerEntry("s", "repo", ts, nil, time.Now())
	if e.RawDiscoveryCalls != 3 || e.FathomCalls != 5 || e.RawDiscoveryTokensEst == nil || *e.RawDiscoveryTokensEst != 9000 {
		t.Errorf("entry = %+v", e)
	}
	if e.RepeatedReadFiles != 1 || e.SidechainToolResults != 2 || !strings.Contains(e.ExplorationNote, "estimate") {
		t.Errorf("entry exploration fields = %+v", e)
	}
}
