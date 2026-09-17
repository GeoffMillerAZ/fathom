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

	out := renderVerdict(rows, criteria, nil)

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
	}, criteria, nil)
	if !strings.Contains(matched, "fathom's arm matched grep") {
		t.Errorf("equal success rates must render as matched:\n%s", matched)
	}

	lost := renderVerdict([]ScenarioLedgerRow{
		{RunID: "r2", Arm: "full", CorpusSHA256: "sha", Trials: 2, Successes: 0, Readings: Readings{TotalTokens: 100}},
		{RunID: "r2", Arm: "grep", CorpusSHA256: "sha", Trials: 2, Successes: 2, Readings: Readings{TotalTokens: 100}},
	}, criteria, nil)
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
	out := renderVerdict(rows, criteria, nil)
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
	out := renderVerdict(rows, nil, nil)
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
	}, nil, nil)
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
	out := renderVerdict(rows, nil, nil)
	if strings.Index(out, "aaa-run") > strings.Index(out, "zzz-run") {
		t.Errorf("runs must render in sorted order:\n%s", out)
	}
}

func TestRenderVerdict_NoRows(t *testing.T) {
	out := renderVerdict(nil, nil, nil)
	if !strings.Contains(out, VerdictHeading) || !strings.Contains(out, "No runs recorded") {
		t.Errorf("an empty ledger must render a sentence, not a blank block:\n%q", out)
	}
}

func TestRenderVerdict_ExportedWrapperMatches(t *testing.T) {
	rows := []ScenarioLedgerRow{{RunID: "r1", Arm: "full", InstrumentDefect: "x"}}
	if RenderVerdict(rows, nil, nil) != renderVerdict(rows, nil, nil) {
		t.Error("RenderVerdict must delegate to renderVerdict verbatim")
	}
}

// TestRenderVerdict_GoldSourceLine is VB-4: the verdict block names which
// tool earned the treatment arm's passes.
func TestRenderVerdict_GoldSourceLine(t *testing.T) {
	criteria := map[string]Criterion{"sha": {ID: "M1", Treatment: "full", Control: "grep"}}
	rows := []ScenarioLedgerRow{
		{
			RunID: "r1", TaskID: "t1", Arm: "full", CorpusSHA256: "sha", Trials: 2, Successes: 2,
			GoldSourcePasses: map[string]int{"mcp__fathom__obligations": 2},
		},
		{
			RunID: "r1", TaskID: "t2", Arm: "full", CorpusSHA256: "sha", Trials: 1, Successes: 1,
			GoldSourcePasses: map[string]int{"Grep": 1},
		},
		{RunID: "r1", TaskID: "t1", Arm: "grep", CorpusSHA256: "sha", Trials: 2, Successes: 1},
	}
	out := renderVerdict(rows, criteria, nil)
	if !strings.Contains(out, "gold source: mcp__fathom__obligations earned 2 of 3 passes.") {
		t.Errorf("must name the top tool and the pass totals for the TREATMENT arm:\n%s", out)
	}
}

// TestRenderVerdict_RowCriterionSurvivesCorpusMove is the wave N2 defect
// (docs/bench/README.md's "vs-pilot-3 (no registered criterion for this
// corpus)"): the render looks a run's criterion up by the corpus
// SHA256 recorded on the row, and testdata/scenariobench/lookups-vs.json
// was rewritten after vs-pilot-3 filed, so the SAME criterion (M6-1) now
// lives under a DIFFERENT sha in the freshly-globbed criteria map. A row
// that also carries its own CriterionID (stamped at filing time) must
// still resolve its criterion by ID, even though its CorpusSHA256 no
// longer matches any key in criteria.
func TestRenderVerdict_RowCriterionSurvivesCorpusMove(t *testing.T) {
	criteria := map[string]Criterion{
		// The corpus moved: this is its CURRENT sha, not the sha recorded
		// on the rows below.
		"sha-after-move": {ID: "M6-1", Treatment: "full", Control: "grep"},
	}
	rows := []ScenarioLedgerRow{
		{RunID: "vs-pilot-3", TaskID: "t1", Arm: "full", CorpusSHA256: "sha-before-move", CriterionID: "M6-1", Trials: 2, Successes: 2, Readings: Readings{TotalTokens: 100}},
		{RunID: "vs-pilot-3", TaskID: "t1", Arm: "grep", CorpusSHA256: "sha-before-move", CriterionID: "M6-1", Trials: 2, Successes: 1, Readings: Readings{TotalTokens: 200}},
	}
	out := renderVerdict(rows, criteria, nil)
	if strings.Contains(out, "no registered criterion for this corpus") {
		t.Errorf("a row carrying its own criterion id must resolve it after the corpus moves:\n%s", out)
	}
	if !strings.Contains(out, "full vs grep: success 2/2 vs 1/2") {
		t.Errorf("must render the comparison using the row's own criterion:\n%s", out)
	}
}

func TestRenderVerdict_GoldSourceLineAbsentWhenUnrecorded(t *testing.T) {
	criteria := map[string]Criterion{"sha": {ID: "M1", Treatment: "full", Control: "grep"}}
	rows := []ScenarioLedgerRow{
		{RunID: "r1", TaskID: "t1", Arm: "full", CorpusSHA256: "sha", Trials: 2, Successes: 2},
		{RunID: "r1", TaskID: "t1", Arm: "grep", CorpusSHA256: "sha", Trials: 2, Successes: 1},
	}
	out := renderVerdict(rows, criteria, nil)
	if strings.Contains(out, "gold source:") {
		t.Errorf("a run with no GoldSourcePasses reading anywhere must not print the line:\n%s", out)
	}
}

// TestRenderVerdict_OldRowFallsBackToCorpusFamilyByTaskID is O65 (wave
// N12) item 2's own regression: guards-pilot-5's rows were filed before
// CriterionID existed (empty on every row, the real shape recorded in
// docs/bench/scenarios-ledger.jsonl) AND testdata/scenariobench/
// guards-fathom.json has been rewritten again since filing, so the row's
// own CorpusSHA256 no longer matches ANY key in a freshly-globbed
// criteria map either — both of N8's own lookups (sha, then ID) fail, and
// the pre-fix render regresses a run that used to compare cleanly to "(no
// registered criterion for this corpus)". byTaskID resolves it: every
// task id the row's own family (guards-fathom.json) declares still maps
// to the SAME criterion, sha rewrite notwithstanding. A second,
// DECOY criterion (M1-2, a different corpus registering the exact same
// "full"/"control" arm pair — this repo's own guards-adr.json really
// does) proves the match is genuinely by task id, not merely "the only
// criterion whose arms fit": arm names alone cannot disambiguate here.
//
// The expected text below is byte-for-byte the pre-N8 rendering vs-pilot-5
// evidence names for guards-pilot-5 (this test's own row totals ARE that
// run's real committed aggregate: full 7/8, control 6/10, tokens
// 31608749 vs 41835495, cost 11.5380296 vs 15.3707146, surface_calls 45,
// gold_source_passes {"none":7}, 7 voided rows).
func TestRenderVerdict_OldRowFallsBackToCorpusFamilyByTaskID(t *testing.T) {
	criteria := map[string]Criterion{
		// The corpus moved AGAIN since guards-pilot-5 filed: this is its
		// CURRENT sha, never the sha recorded on the rows below.
		"sha-guards-fathom-current": {ID: "M1-1", Treatment: "full", Control: "control", MinTrials: 2, RequireSuccessAtLeastEqual: true, MaxTokensPerSuccessRatio: 1.0},
		// A DIFFERENT corpus family registering the exact same arm pair —
		// disambiguation must not be possible from arm names alone.
		"sha-guards-adr-current": {ID: "M1-2", Treatment: "full", Control: "control", MinTrials: 2, RequireSuccessAtLeastEqual: true, MaxTokensPerSuccessRatio: 1.0},
	}
	byTaskID := map[string]Criterion{
		"guards-i1-packagedeps-sorted": criteria["sha-guards-fathom-current"],
		"guards-i11-mcp-tool-doc":      criteria["sha-guards-fathom-current"],
		"guards-adr-i1-decoy":          criteria["sha-guards-adr-current"],
	}
	const orphanedSHA = "sha-guards-pilot5-orphaned"
	rows := []ScenarioLedgerRow{
		{
			RunID: "guards-pilot-5", TaskID: "guards-i1-packagedeps-sorted", Arm: "full",
			CorpusSHA256: orphanedSHA, Trials: 8, Successes: 7,
			Readings:         Readings{TotalTokens: 31608749, SurfaceCalls: 45},
			CostUSD:          11.5380296,
			GoldSourcePasses: map[string]int{"none": 7},
		},
		{
			RunID: "guards-pilot-5", TaskID: "guards-i1-packagedeps-sorted", Arm: "control",
			CorpusSHA256: orphanedSHA, Trials: 10, Successes: 6,
			Readings: Readings{TotalTokens: 41835495},
			CostUSD:  15.3707146,
		},
	}
	for _, pair := range []struct{ task, arm string }{
		{"guards-i16-raw-cp-deploy", "control"},
		{"guards-i16-raw-cp-deploy", "full"},
		{"guards-i2-store-purego", "control"},
		{"guards-i2-store-purego", "full"},
		{"guards-i28-duckdb-path", "control"},
		{"guards-i28-duckdb-path", "full"},
		{"guards-i31-workspace-artifact", "full"},
	} {
		rows = append(rows, ScenarioLedgerRow{
			RunID: "guards-pilot-5", TaskID: pair.task, Arm: pair.arm,
			CorpusSHA256: orphanedSHA, InstrumentDefect: "O57: not adjudicable",
		})
	}

	out := renderVerdict(rows, criteria, byTaskID)

	const want = "**guards-pilot-5** — full vs control: success 7/8 vs 6/10, tokens 0.756×, cost 0.751×.\n" +
		"Surface calls: control=0, full=45.\n" +
		"On guards-pilot-5, fathom's arm beat control on success (7 v 6) at 0.756× the tokens; the tools were called 45 time(s). Registered criterion M1-1: pass (treatment met the registered bar).\n" +
		"gold source: none earned 7 of 7 passes.\n" +
		"Voided: 7 row(s) carry `instrument_defect` and are excluded from the numbers above — " +
		"guards-i16-raw-cp-deploy/control, guards-i16-raw-cp-deploy/full, guards-i2-store-purego/control, guards-i2-store-purego/full, guards-i28-duckdb-path/control, guards-i28-duckdb-path/full, guards-i31-workspace-artifact/full.\n"

	if !strings.Contains(out, want) {
		t.Errorf("guards-pilot-5 must render byte-for-byte as the pre-N8 text:\ngot:\n%s\nwant substring:\n%s", out, want)
	}
	if strings.Contains(out, "no registered criterion for this corpus") {
		t.Errorf("must not fall back to the unresolved-criterion text once byTaskID can resolve the family:\n%s", out)
	}
	if strings.Contains(out, "M1-2") {
		t.Errorf("must never resolve to the decoy corpus's own criterion:\n%s", out)
	}
}

// TestRenderVerdict_PairedStatsLine is wave N15 item 1: the verdict block
// renders the bench-confidence layer's own paired bootstrap statistics
// (ComputePairedRunStats/RenderPairedRunStats) for a run whose treatment
// and control both resolve — placed strictly AFTER the voided-row summary
// line, never between it and gold source (which would break
// TestRenderVerdict_OldRowFallsBackToCorpusFamilyByTaskID's own
// byte-for-byte pin above).
func TestRenderVerdict_PairedStatsLine(t *testing.T) {
	criteria := map[string]Criterion{"sha": {ID: "LCA-1", Treatment: "fathom", Control: "control"}}
	rows := []ScenarioLedgerRow{
		{RunID: "lca-run-1", TaskID: "t1", Arm: "fathom", CorpusSHA256: "sha", Trials: 3, Successes: 3, Readings: Readings{TotalTokens: 3000}},
		{RunID: "lca-run-1", TaskID: "t1", Arm: "control", CorpusSHA256: "sha", Trials: 3, Successes: 3, Readings: Readings{TotalTokens: 6000}},
		{RunID: "lca-run-1", TaskID: "t2", Arm: "fathom", CorpusSHA256: "sha", InstrumentDefect: "O41", Trials: 3, Successes: 3},
	}
	out := renderVerdict(rows, criteria, nil)
	if !strings.Contains(out, "Paired (bootstrap, n=1, seed 20260902, 20000 resamples): success-rate diff +0.000 [+0.000, +0.000] (95% CI); token ratio 0.50× [0.50×, 0.50×].\n") {
		t.Errorf("expected the paired-stats line for the one sound paired task, got:\n%s", out)
	}
	goldIdx := strings.Index(out, "Registered criterion")
	pairedIdx := strings.Index(out, "Paired (bootstrap")
	voidedIdx := strings.Index(out, "Voided:")
	if voidedIdx == -1 || pairedIdx == -1 || goldIdx == -1 {
		t.Fatalf("expected all three markers present:\n%s", out)
	}
	if !(goldIdx < voidedIdx && voidedIdx < pairedIdx) {
		t.Errorf("expected order: criterion sentence, then Voided, then Paired — got indices %d, %d, %d in:\n%s", goldIdx, voidedIdx, pairedIdx, out)
	}
}

// TestRenderVerdict_PairedStatsLineOmittedWhenNothingPairs proves the
// line is skipped, never a placeholder, when no task ran in both arms.
func TestRenderVerdict_PairedStatsLineOmittedWhenNothingPairs(t *testing.T) {
	criteria := map[string]Criterion{"sha": {ID: "LCA-1", Treatment: "fathom", Control: "control"}}
	rows := []ScenarioLedgerRow{
		{RunID: "r1", TaskID: "t1", Arm: "fathom", CorpusSHA256: "sha", Trials: 3, Successes: 3},
		{RunID: "r1", TaskID: "t2", Arm: "control", CorpusSHA256: "sha", Trials: 3, Successes: 3},
	}
	out := renderVerdict(rows, criteria, nil)
	if strings.Contains(out, "Paired (bootstrap") {
		t.Errorf("expected no paired-stats line when no task ran in both arms:\n%s", out)
	}
}

// TestRenderVerdict_PairedCriterionRendersNullSentence is wave N17's own
// end-to-end failing-first test for the defect docs/bench/README.md
// rendered for lca-run-1: "pass (treatment met the registered bar)"
// immediately above a paired 95% CI that includes zero — contradicting
// VB-27's registered rule (docs/vision/29-verbs-not-lookups.md §5). With
// Criterion.Paired set, the plain-language sentence's own "Registered
// criterion ...:" clause must carry the paired verdict word ("null") and
// cite the same interval the "Paired (bootstrap...)" line below it
// reports — every task's diff is exactly 0 here, so the CI is the
// deterministic [0, 0].
func TestRenderVerdict_PairedCriterionRendersNullSentence(t *testing.T) {
	criteria := map[string]Criterion{
		"sha": {ID: LCACriterionID, Treatment: "fathom", Control: "control", MinTrials: 2, Paired: true},
	}
	rows := []ScenarioLedgerRow{
		{RunID: "lca-run-x", TaskID: "t1", Arm: "fathom", CorpusSHA256: "sha", Trials: 3, Successes: 3},
		{RunID: "lca-run-x", TaskID: "t1", Arm: "control", CorpusSHA256: "sha", Trials: 3, Successes: 3},
		{RunID: "lca-run-x", TaskID: "t2", Arm: "fathom", CorpusSHA256: "sha", Trials: 3, Successes: 1},
		{RunID: "lca-run-x", TaskID: "t2", Arm: "control", CorpusSHA256: "sha", Trials: 3, Successes: 1},
		{RunID: "lca-run-x", TaskID: "t3", Arm: "fathom", CorpusSHA256: "sha", Trials: 3, Successes: 0},
		{RunID: "lca-run-x", TaskID: "t3", Arm: "control", CorpusSHA256: "sha", Trials: 3, Successes: 0},
	}
	out := renderVerdict(rows, criteria, nil)
	want := "Registered criterion " + LCACriterionID +
		": null (paired 95% CI of the success-rate difference includes zero: +0.000 [+0.000, +0.000]; the null is published).\n"
	if !strings.Contains(out, want) {
		t.Errorf("expected the null verdict sentence, got:\n%s", out)
	}
	if !strings.Contains(out, "Paired (bootstrap, n=3, seed 20260902, 20000 resamples): success-rate diff +0.000 [+0.000, +0.000] (95% CI)") {
		t.Errorf("expected the paired-stats line to cite the same interval, got:\n%s", out)
	}
}
