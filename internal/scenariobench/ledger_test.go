package scenariobench

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendAndLoadScenarioLedger(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "ledger.jsonl")

	row1 := ScenarioLedgerRow{RunID: "run-1", TaskID: "T1", Arm: "full", Trials: 2, Successes: 2}
	row2 := ScenarioLedgerRow{RunID: "run-1", TaskID: "T1", Arm: "control", Trials: 2, Successes: 1}

	if err := AppendScenarioLedger(path, row1); err != nil {
		t.Fatalf("AppendScenarioLedger row1: %v", err)
	}
	if err := AppendScenarioLedger(path, row2); err != nil {
		t.Fatalf("AppendScenarioLedger row2: %v", err)
	}

	rows, err := LoadScenarioLedger(path)
	if err != nil {
		t.Fatalf("LoadScenarioLedger: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Arm != "full" || rows[1].Arm != "control" {
		t.Errorf("expected file order preserved, got %+v", rows)
	}
}

func TestLoadScenarioLedger_MissingFileIsEmptyNotError(t *testing.T) {
	rows, err := LoadScenarioLedger(filepath.Join(t.TempDir(), "does-not-exist.jsonl"))
	if err != nil {
		t.Fatalf("expected no error for a missing ledger, got %v", err)
	}
	if rows != nil {
		t.Errorf("expected nil rows, got %v", rows)
	}
}

func TestLoadScenarioLedger_UnparseableLineIsAnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.jsonl")
	writeTestFile(t, path, "{\"run_id\":\"run-1\"}\nnot json\n")
	if _, err := LoadScenarioLedger(path); err == nil {
		t.Fatal("expected an error naming the bad line, not a silently shorter history")
	}
}

func TestRenderScenarioLedger(t *testing.T) {
	rows := []ScenarioLedgerRow{
		{RunID: "run-1", Commit: "abc123", TaskID: "T1", Category: CategoryBugFix, Arm: "full", ModelID: "sonnet", Trials: 2, Successes: 2, Prediction: "wins", CostUSD: 1.5},
	}
	out := RenderScenarioLedger(rows)
	for _, want := range []string{"run-1", "abc123", "T1", string(CategoryBugFix), "full", "sonnet", "wins"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered table missing %q:\n%s", want, out)
		}
	}
}

// TestRenderScenarioLedger_SurfaceCallsColumn covers item 6's markdown
// render: a row's SurfaceCalls prints, and SurfaceUnused annotates it.
func TestRenderScenarioLedger_SurfaceCallsColumn(t *testing.T) {
	rows := []ScenarioLedgerRow{
		{
			RunID: "run-1", TaskID: "T1", Arm: "comp", Trials: 1, Successes: 1,
			Readings: Readings{SurfaceCalls: 3},
		},
		{
			RunID: "run-1", TaskID: "T2", Arm: "comp", Trials: 1, Successes: 1,
			Readings: Readings{SurfaceCalls: 0}, SurfaceUnused: true,
		},
	}
	out := RenderScenarioLedger(rows)
	if !strings.Contains(out, "surface calls") {
		t.Errorf("rendered table missing the surface calls column header:\n%s", out)
	}
	if !strings.Contains(out, "| 3 |") {
		t.Errorf("rendered table missing SurfaceCalls=3 for T1:\n%s", out)
	}
	if !strings.Contains(out, "0 (unused)") {
		t.Errorf("rendered table missing the unused annotation for T2:\n%s", out)
	}
}

func TestRewriteScenarioLedgerSection(t *testing.T) {
	dir := t.TempDir()
	readme := filepath.Join(dir, "README.md")
	original := "# Bench\n\nSome prose.\n\n## Scenario runs\n\n" +
		ScenarioLedgerBeginMarker + "\n\n(nothing yet)\n\n" + ScenarioLedgerEndMarker +
		"\n\nMore prose after.\n"
	writeTestFile(t, readme, original)

	if err := RewriteScenarioLedgerSection(readme, "| a | b |\n|---|---|\n| 1 | 2 |\n"); err != nil {
		t.Fatalf("RewriteScenarioLedgerSection: %v", err)
	}

	raw, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "| a | b |") {
		t.Errorf("rewritten table missing from README:\n%s", text)
	}
	if !strings.Contains(text, "Some prose.") || !strings.Contains(text, "More prose after.") {
		t.Errorf("surrounding prose was disturbed:\n%s", text)
	}
	if strings.Contains(text, "(nothing yet)") {
		t.Errorf("old table content was not replaced:\n%s", text)
	}
}

func TestRewriteScenarioLedgerSection_MissingMarkerIsError(t *testing.T) {
	dir := t.TempDir()
	readme := filepath.Join(dir, "README.md")
	writeTestFile(t, readme, "# Bench\n\nno markers here\n")
	if err := RewriteScenarioLedgerSection(readme, "body"); err == nil {
		t.Fatal("expected an error when the markers are absent")
	}
}

func TestCriterionEvaluate(t *testing.T) {
	c := Criterion{ID: "C1", Treatment: "full", Control: "control", MinTrials: 2, RequireSuccessAtLeastEqual: true, MaxTokensPerSuccessRatio: 1.5}

	t.Run("indeterminate below min trials", func(t *testing.T) {
		rows := []ScenarioLedgerRow{{Arm: "full", Trials: 1, Successes: 1}}
		ev := c.Evaluate(rows)
		if ev.Verdict != ScenarioVerdictIndeterminate {
			t.Errorf("Verdict = %s, want indeterminate", ev.Verdict)
		}
	})

	t.Run("fail on lower success rate", func(t *testing.T) {
		rows := []ScenarioLedgerRow{
			{Arm: "full", Trials: 2, Successes: 1},
			{Arm: "control", Trials: 2, Successes: 2},
		}
		ev := c.Evaluate(rows)
		if ev.Verdict != ScenarioVerdictFail {
			t.Errorf("Verdict = %s, want fail", ev.Verdict)
		}
	})

	t.Run("fail on token ratio", func(t *testing.T) {
		rows := []ScenarioLedgerRow{
			{Arm: "full", Trials: 2, Successes: 2, Readings: Readings{TotalTokens: 1000}},
			{Arm: "control", Trials: 2, Successes: 2, Readings: Readings{TotalTokens: 100}},
		}
		ev := c.Evaluate(rows)
		if ev.Verdict != ScenarioVerdictFail {
			t.Errorf("Verdict = %s, want fail (10x token ratio)", ev.Verdict)
		}
	})

	t.Run("pass", func(t *testing.T) {
		rows := []ScenarioLedgerRow{
			{Arm: "full", Trials: 2, Successes: 2, Readings: Readings{TotalTokens: 100}},
			{Arm: "control", Trials: 2, Successes: 2, Readings: Readings{TotalTokens: 100}},
		}
		ev := c.Evaluate(rows)
		if ev.Verdict != ScenarioVerdictPass {
			t.Errorf("Verdict = %s, want pass: %s", ev.Verdict, ev.Reason)
		}
	})
}

func TestEscalationPlan(t *testing.T) {
	c := Criterion{ID: "C1", Treatment: "full", Control: "control", RequireSuccessAtLeastEqual: true, MaxTokensPerSuccessRatio: 1.5}

	t.Run("success delta within margin is flagged", func(t *testing.T) {
		rows := []ScenarioLedgerRow{
			// treatment 1/2 = 0.5, control 1/2 = 0.5 -> delta 0.0, within 0.15
			{TaskID: "T1", Arm: "full", Trials: 2, Successes: 1, Readings: Readings{TotalTokens: 100}},
			{TaskID: "T1", Arm: "control", Trials: 2, Successes: 1, Readings: Readings{TotalTokens: 100}},
		}
		got, _ := EscalationPlan(rows, c, 0.15)
		if len(got) != 1 || got[0].TaskID != "T1" {
			t.Fatalf("got %+v, want one cell for T1", got)
		}
		if got[0].Trials != 2 {
			t.Errorf("Trials = %d, want 2", got[0].Trials)
		}
		if got[0].SuccessDelta != 0 {
			t.Errorf("SuccessDelta = %v, want 0", got[0].SuccessDelta)
		}
	})

	t.Run("success delta outside margin is not flagged", func(t *testing.T) {
		rows := []ScenarioLedgerRow{
			// treatment 2/2 = 1.0, control 0/2 = 0.0 -> delta 1.0, far outside 0.15
			{TaskID: "T2", Arm: "full", Trials: 2, Successes: 2, Readings: Readings{TotalTokens: 100}},
			{TaskID: "T2", Arm: "control", Trials: 2, Successes: 0, Readings: Readings{TotalTokens: 100}},
		}
		got, _ := EscalationPlan(rows, c, 0.15)
		if len(got) != 0 {
			t.Fatalf("got %+v, want no cells (delta is decisive, not borderline)", got)
		}
	})

	t.Run("tokens ratio within margin is flagged", func(t *testing.T) {
		rows := []ScenarioLedgerRow{
			// success rates tie (no success-delta flag); tokens ratio =
			// (300/2)/(100/1) = 1.5, exactly the criterion's bar.
			{TaskID: "T3", Arm: "full", Trials: 2, Successes: 2, Readings: Readings{TotalTokens: 300}},
			{TaskID: "T3", Arm: "control", Trials: 2, Successes: 2, Readings: Readings{TotalTokens: 200}},
		}
		got, _ := EscalationPlan(rows, c, 0.15)
		if len(got) != 1 || got[0].TaskID != "T3" {
			t.Fatalf("got %+v, want one cell for T3", got)
		}
		if math.Abs(got[0].TokensRatio-1.5) > 1e-9 {
			t.Errorf("TokensRatio = %v, want 1.5", got[0].TokensRatio)
		}
	})

	t.Run("n=3 already escalated is not a candidate", func(t *testing.T) {
		rows := []ScenarioLedgerRow{
			{TaskID: "T4", Arm: "full", Trials: 3, Successes: 1, Readings: Readings{TotalTokens: 100}},
			{TaskID: "T4", Arm: "control", Trials: 3, Successes: 1, Readings: Readings{TotalTokens: 100}},
		}
		got, _ := EscalationPlan(rows, c, 0.15)
		if len(got) != 0 {
			t.Fatalf("got %+v, want no cells (n=3 already ran)", got)
		}
	})

	t.Run("rows from unrelated arms are ignored", func(t *testing.T) {
		rows := []ScenarioLedgerRow{
			{TaskID: "T5", Arm: "competitor-x", Trials: 2, Successes: 1},
		}
		got, _ := EscalationPlan(rows, c, 0.15)
		if len(got) != 0 {
			t.Fatalf("got %+v, want no cells (no treatment/control rows)", got)
		}
	})

	// TestEscalationPlan/rows_carrying_instrument_defect_are_skipped_and_counted
	// is finding 6's own proof: EscalationPlan must not build a
	// recommendation from a row ADR-034/RC-36 already voided (the same
	// exclusion Evaluate already applies), and must report how many rows
	// it skipped rather than silently dropping them.
	t.Run("rows carrying instrument_defect are skipped and counted", func(t *testing.T) {
		rows := []ScenarioLedgerRow{
			// Would otherwise flag T6 (delta 0, within margin) — voided by
			// InstrumentDefect on BOTH rows, so no evidence at all remains.
			{TaskID: "T6", Arm: "full", Trials: 2, Successes: 1, Readings: Readings{TotalTokens: 100}, InstrumentDefect: "no mcp tool calls"},
			{TaskID: "T6", Arm: "control", Trials: 2, Successes: 1, Readings: Readings{TotalTokens: 100}, InstrumentDefect: "no mcp tool calls"},
			// T7 is sound and should still be flagged.
			{TaskID: "T7", Arm: "full", Trials: 2, Successes: 1, Readings: Readings{TotalTokens: 100}},
			{TaskID: "T7", Arm: "control", Trials: 2, Successes: 1, Readings: Readings{TotalTokens: 100}},
		}
		got, skipped := EscalationPlan(rows, c, 0.15)
		if skipped != 2 {
			t.Errorf("skipped = %d, want 2 (T6's two instrument_defect rows)", skipped)
		}
		if len(got) != 1 || got[0].TaskID != "T7" {
			t.Fatalf("got %+v, want exactly one cell for T7 (T6 voided out of evidence entirely)", got)
		}
	})
}

// TestGoldSourceRateByCategory_WeightsByTrialsAndFillsZero is VB-4: the
// per-question-shape breakdown must reconstruct exact counts across rows
// of different sizes, and the render must list every tool seen in ANY
// category — including at zero for a category that never won with it.
func TestGoldSourceRateByCategory_WeightsByTrialsAndFillsZero(t *testing.T) {
	rows := []ScenarioLedgerRow{
		{
			TaskID: "g1", Category: "guards", Trials: 4,
			GoldSourceRate: map[string]float64{"mcp__fathom__obligations": 0.75, GoldSourceNone: 0.25},
		},
		{
			TaskID: "l1", Category: "lookup", Trials: 2,
			GoldSourceRate: map[string]float64{"Grep": 1.0},
		},
		// A row scored before VB-4 (no GoldSourceRate at all) must not
		// distort either category's total.
		{TaskID: "l2", Category: "lookup", Trials: 5},
	}
	byCat := GoldSourceRateByCategory(rows)
	guards := byCat["guards"]
	if got := guards["mcp__fathom__obligations"]; got != 0.75 {
		t.Errorf("guards obligations rate = %v, want 0.75", got)
	}
	if got := guards[GoldSourceNone]; got != 0.25 {
		t.Errorf("guards none rate = %v, want 0.25", got)
	}
	lookup := byCat["lookup"]
	if got := lookup["Grep"]; got != 1.0 {
		t.Errorf("lookup Grep rate = %v, want 1.0 (the unmeasured row must not water it down)", got)
	}

	table := RenderGoldSourceByCategory(byCat)
	if !strings.Contains(table, "### Gold source by question shape") {
		t.Errorf("must carry the section heading:\n%s", table)
	}
	if !strings.Contains(table, "| category | Grep | mcp__fathom__obligations | none |") {
		t.Errorf("header must list every tool seen in ANY category, sorted:\n%s", table)
	}
	// "Grep" never won in "guards" — the row must still show it at 0.00,
	// never a silent absence.
	if !strings.Contains(table, "| guards | 0.00 | 0.75 | 0.25 |") {
		t.Errorf("guards row must show Grep at 0.00:\n%s", table)
	}
	if !strings.Contains(table, "| lookup | 1.00 | 0.00 | 0.00 |") {
		t.Errorf("lookup row must show the other tools at 0.00:\n%s", table)
	}
}

func TestGoldSourceRateByCategory_Empty(t *testing.T) {
	if GoldSourceRateByCategory(nil) != nil {
		t.Error("no rows, no breakdown")
	}
	if RenderGoldSourceByCategory(nil) != "" {
		t.Error("no breakdown, no rendered table")
	}
}

// TestWriteScenarioLedger_RoundTrips proves the overwrite path: every row
// survives a write/reload cycle, in the given order.
func TestWriteScenarioLedger_RoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	rows := []ScenarioLedgerRow{
		{RunID: "run-1", TaskID: "T1", Arm: "full", Trials: 2, Successes: 2},
		{RunID: "run-1", TaskID: "T1", Arm: "control", Trials: 2, Successes: 1},
	}
	if err := WriteScenarioLedger(path, rows); err != nil {
		t.Fatalf("WriteScenarioLedger: %v", err)
	}
	got, err := LoadScenarioLedger(path)
	if err != nil {
		t.Fatalf("LoadScenarioLedger: %v", err)
	}
	if len(got) != 2 || got[0].Arm != "full" || got[1].Arm != "control" {
		t.Fatalf("unexpected round-trip: %+v", got)
	}
}

// TestWriteScenarioLedger_OverwritesExistingContent proves this is a
// REPLACE, not an append: a second write with fewer rows leaves exactly
// that many, never the union of both writes.
func TestWriteScenarioLedger_OverwritesExistingContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	if err := WriteScenarioLedger(path, []ScenarioLedgerRow{
		{RunID: "run-1", TaskID: "T1", Arm: "full"},
		{RunID: "run-1", TaskID: "T1", Arm: "control"},
	}); err != nil {
		t.Fatalf("first WriteScenarioLedger: %v", err)
	}
	if err := WriteScenarioLedger(path, []ScenarioLedgerRow{
		{RunID: "run-2", TaskID: "T2", Arm: "full"},
	}); err != nil {
		t.Fatalf("second WriteScenarioLedger: %v", err)
	}
	got, err := LoadScenarioLedger(path)
	if err != nil {
		t.Fatalf("LoadScenarioLedger: %v", err)
	}
	if len(got) != 1 || got[0].RunID != "run-2" {
		t.Fatalf("expected the second write to REPLACE the first, got %+v", got)
	}
}

// TestVoidScenarioLedgerRow_MarksMatchingRowOnly is O53 defect 7's own
// proof on a temp ledger: voiding one (run_id, task_id, arm) marks
// exactly that row's InstrumentDefect and leaves every other row —
// including a same-task different-arm row and a same-arm different-task
// row — untouched.
func TestVoidScenarioLedgerRow_MarksMatchingRowOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	for _, r := range []ScenarioLedgerRow{
		{RunID: "run-1", TaskID: "T1", Arm: "full", Trials: 3, Successes: 3},
		{RunID: "run-1", TaskID: "T1", Arm: "control", Trials: 3, Successes: 1},
		{RunID: "run-1", TaskID: "T2", Arm: "full", Trials: 3, Successes: 2},
		{RunID: "run-2", TaskID: "T1", Arm: "full", Trials: 3, Successes: 3},
	} {
		if err := AppendScenarioLedger(path, r); err != nil {
			t.Fatalf("AppendScenarioLedger: %v", err)
		}
	}

	if err := VoidScenarioLedgerRow(path, "run-1", "T1", "full", "the operator daemon was reachable"); err != nil {
		t.Fatalf("VoidScenarioLedgerRow: %v", err)
	}

	rows, err := LoadScenarioLedger(path)
	if err != nil {
		t.Fatalf("LoadScenarioLedger: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("expected voiding to change row COUNT not at all, got %d rows", len(rows))
	}
	for _, r := range rows {
		wantVoided := r.RunID == "run-1" && r.TaskID == "T1" && r.Arm == "full"
		if wantVoided && r.InstrumentDefect != "the operator daemon was reachable" {
			t.Errorf("row %s/%s/%s: InstrumentDefect = %q, want it voided", r.RunID, r.TaskID, r.Arm, r.InstrumentDefect)
		}
		if !wantVoided && r.InstrumentDefect != "" {
			t.Errorf("row %s/%s/%s: InstrumentDefect = %q, want untouched", r.RunID, r.TaskID, r.Arm, r.InstrumentDefect)
		}
	}
}

// TestVoidScenarioLedgerRow_RefusesWhenRowAbsent: voiding a row that was
// never filed is refused, and the ledger is left byte-for-byte unchanged.
func TestVoidScenarioLedgerRow_RefusesWhenRowAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	if err := AppendScenarioLedger(path, ScenarioLedgerRow{RunID: "run-1", TaskID: "T1", Arm: "full"}); err != nil {
		t.Fatalf("AppendScenarioLedger: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	err = VoidScenarioLedgerRow(path, "run-1", "T1", "control", "no such row")
	if err == nil {
		t.Fatal("expected an error voiding a row that was never filed")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("expected the ledger to be left untouched on refusal")
	}
}

// TestVoidScenarioLedgerRow_RefusesOnMissingLedgerFile: no ledger file at
// all (nothing has ever been filed) is also refused, not silently
// treated as an empty, successfully-voided ledger.
func TestVoidScenarioLedgerRow_RefusesOnMissingLedgerFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.jsonl")
	if err := VoidScenarioLedgerRow(path, "run-1", "T1", "full", "reason"); err == nil {
		t.Fatal("expected an error voiding against a ledger that does not exist")
	}
}

// TestVoidScenarioLedgerRow_MarksEveryTrialOfTheRow: a row voided twice
// under different reasons keeps the LATEST reason (a re-void corrects the
// annotation rather than appending to it).
func TestVoidScenarioLedgerRow_SecondVoidReplacesReason(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	if err := AppendScenarioLedger(path, ScenarioLedgerRow{RunID: "run-1", TaskID: "T1", Arm: "full"}); err != nil {
		t.Fatalf("AppendScenarioLedger: %v", err)
	}
	if err := VoidScenarioLedgerRow(path, "run-1", "T1", "full", "first reason"); err != nil {
		t.Fatalf("first VoidScenarioLedgerRow: %v", err)
	}
	if err := VoidScenarioLedgerRow(path, "run-1", "T1", "full", "corrected reason"); err != nil {
		t.Fatalf("second VoidScenarioLedgerRow: %v", err)
	}
	rows, err := LoadScenarioLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].InstrumentDefect != "corrected reason" {
		t.Fatalf("expected exactly 1 row with the corrected reason, got %+v", rows)
	}
}
