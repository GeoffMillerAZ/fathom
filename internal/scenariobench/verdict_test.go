package scenariobench

import (
	"strings"
	"testing"
)

func TestRenderVerdict_BeatsMatchesAndVoids(t *testing.T) {
	criteria := map[string]Criterion{
		"sha-guards": {ID: "M1-1", Treatment: "full", Control: "control"},
	}
	rows := []ScenarioLedgerRow{
		{
			RunID: "guards-pilot-3", TaskID: "i1", Arm: "full", CorpusSHA256: "sha-guards", Trials: 2, Successes: 2, CostUSD: 0.9,
			Readings: Readings{TotalTokens: 1000, SurfaceCalls: 6},
		},
		{
			RunID: "guards-pilot-3", TaskID: "i1", Arm: "control", CorpusSHA256: "sha-guards", Trials: 2, Successes: 1, CostUSD: 1.0,
			Readings: Readings{TotalTokens: 2000, SurfaceCalls: 0},
		},
		{
			RunID: "guards-pilot-3", TaskID: "i11", Arm: "full", CorpusSHA256: "sha-guards", Trials: 1, Successes: 0,
			InstrumentDefect: "O41: no MCP tools",
		},
	}

	out := renderVerdict(rows, criteria)

	if !strings.HasPrefix(out, VerdictHeading) {
		t.Fatalf("must open with the heading:\n%s", out)
	}
	if !strings.Contains(out, "guards-pilot-3") {
		t.Errorf("must name the run:\n%s", out)
	}
	if !strings.Contains(out, "success 2/2 vs 1/2") {
		t.Errorf("must report treatment vs control successes:\n%s", out)
	}
	if !strings.Contains(out, "tokens 0.500×") {
		t.Errorf("must report the token ratio:\n%s", out)
	}
	if !strings.Contains(out, "Surface calls: control=0, full=6") {
		t.Errorf("must report surface call totals per arm, sorted:\n%s", out)
	}
	if !strings.Contains(out, "On guards-pilot-3, fathom's arm beat control on success (2 v 1) at 0.500× the tokens; the tools were called 6 time(s).") {
		t.Errorf("must render the plain-language sentence exactly:\n%s", out)
	}
	if !strings.Contains(out, "Voided: 1 row(s) carry `instrument_defect` and are excluded from the numbers above — i11/full.") {
		t.Errorf("must name the voided row and exclude it from the numbers:\n%s", out)
	}
	// The voided row's tokens/successes must not have leaked into the sound
	// arm's totals (full stays 2/2, not 2/3; tokens stay 1000, not more).
	if strings.Contains(out, "3/2") {
		t.Errorf("a voided row must not be counted into any arm's totals:\n%s", out)
	}
}

func TestRenderVerdict_MatchedAndLostToWording(t *testing.T) {
	criteria := map[string]Criterion{"sha": {Treatment: "full", Control: "grep"}}

	matched := renderVerdict([]ScenarioLedgerRow{
		{RunID: "r1", Arm: "full", CorpusSHA256: "sha", Trials: 2, Successes: 2, Readings: Readings{TotalTokens: 100}},
		{RunID: "r1", Arm: "grep", CorpusSHA256: "sha", Trials: 2, Successes: 2, Readings: Readings{TotalTokens: 100}},
	}, criteria)
	if !strings.Contains(matched, "fathom's arm matched grep") {
		t.Errorf("equal success rates must render as matched:\n%s", matched)
	}

	lost := renderVerdict([]ScenarioLedgerRow{
		{RunID: "r2", Arm: "full", CorpusSHA256: "sha", Trials: 2, Successes: 0, Readings: Readings{TotalTokens: 100}},
		{RunID: "r2", Arm: "grep", CorpusSHA256: "sha", Trials: 2, Successes: 2, Readings: Readings{TotalTokens: 100}},
	}, criteria)
	if !strings.Contains(lost, "fathom's arm lost to grep") {
		t.Errorf("a lower success rate must render as lost to:\n%s", lost)
	}
}

// TestRenderVerdict_IndeterminateDoesNotClaimBeat (review finding 4): a
// run below the criterion's registered replication floor must not claim
// beat/lost-to from the raw success rates, and must still render the
// criterion's own indeterminate verdict.
func TestRenderVerdict_IndeterminateDoesNotClaimBeat(t *testing.T) {
	criteria := map[string]Criterion{"sha": {ID: "M1-1", Treatment: "full", Control: "control", MinTrials: 3}}
	rows := []ScenarioLedgerRow{
		{RunID: "r1", Arm: "full", CorpusSHA256: "sha", Trials: 1, Successes: 1, Readings: Readings{TotalTokens: 100}},
		{RunID: "r1", Arm: "control", CorpusSHA256: "sha", Trials: 1, Successes: 0, Readings: Readings{TotalTokens: 100}},
	}
	out := renderVerdict(rows, criteria)
	if strings.Contains(out, "fathom's arm beat") || strings.Contains(out, "fathom's arm lost to") {
		t.Errorf("a run below the replication floor must not claim beat/lost to:\n%s", out)
	}
	if !strings.Contains(out, "did not run enough trials to decide against control") {
		t.Errorf("must say the run did not measure enough:\n%s", out)
	}
	if !strings.Contains(out, "Registered criterion M1-1: indeterminate") {
		t.Errorf("must still render the criterion's own verdict:\n%s", out)
	}
}

func TestRenderVerdict_UnknownCriterionFallsBackToRawTotals(t *testing.T) {
	rows := []ScenarioLedgerRow{
		{RunID: "vs-pilot-1", TaskID: "t1", Arm: "full", CorpusSHA256: "sha-unknown", Trials: 2, Successes: 2},
		{RunID: "vs-pilot-1", TaskID: "t1", Arm: "gopls", CorpusSHA256: "sha-unknown", Trials: 2, Successes: 1},
	}
	out := renderVerdict(rows, nil)
	if !strings.Contains(out, "no registered criterion for this corpus") {
		t.Errorf("an unmapped corpus must say so, not guess a comparison:\n%s", out)
	}
	if !strings.Contains(out, "full 2/2") || !strings.Contains(out, "gopls 1/2") {
		t.Errorf("must still list every arm's raw totals:\n%s", out)
	}
}

func TestRenderVerdict_EveryRowVoided(t *testing.T) {
	out := renderVerdict([]ScenarioLedgerRow{
		{RunID: "r3", TaskID: "t1", Arm: "full", InstrumentDefect: "O41"},
	}, nil)
	if !strings.Contains(out, "every row is voided") {
		t.Errorf("a run with no sound row must say so plainly:\n%s", out)
	}
	if !strings.Contains(out, "Voided: 1 row(s)") {
		t.Errorf("must still name the voided row:\n%s", out)
	}
}

func TestRenderVerdict_DeterministicRunOrder(t *testing.T) {
	rows := []ScenarioLedgerRow{
		{RunID: "zzz-run", Arm: "full", InstrumentDefect: "x"},
		{RunID: "aaa-run", Arm: "full", InstrumentDefect: "x"},
	}
	out := renderVerdict(rows, nil)
	if strings.Index(out, "aaa-run") > strings.Index(out, "zzz-run") {
		t.Errorf("runs must render in sorted order:\n%s", out)
	}
}

func TestRenderVerdict_NoRows(t *testing.T) {
	out := renderVerdict(nil, nil)
	if !strings.Contains(out, VerdictHeading) || !strings.Contains(out, "No runs recorded") {
		t.Errorf("an empty ledger must render a sentence, not a blank block:\n%q", out)
	}
}

func TestRenderVerdict_ExportedWrapperMatches(t *testing.T) {
	rows := []ScenarioLedgerRow{{RunID: "r1", Arm: "full", InstrumentDefect: "x"}}
	if RenderVerdict(rows, nil) != renderVerdict(rows, nil) {
		t.Error("RenderVerdict must delegate to renderVerdict verbatim")
	}
}
