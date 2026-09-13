package scenariobench

import (
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
