package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/contextbench"
)

// TestCmdLedger_TargetedRunNeverTouchesTheCommittedLedger (BP-33): a
// `--scope targeted` / `--only` run writes its row under the artifacts tree.
// The committed ledger is history a reader compares row to row; a
// diff-selected subset carries no verdict and must not grow it.
func TestCmdLedger_TargetedRunNeverTouchesTheCommittedLedger(t *testing.T) {
	dir := t.TempDir()
	tasks := writeGatedCorpus(t, dir)
	results := filepath.Join(dir, "results")
	writeCells(t, results, concat(
		threeTrials("T1", "fathom", true, 100), threeTrials("T1", "grep", false, 200),
	))
	committed := filepath.Join(dir, "docs", "bench", "ledger.jsonl")
	if err := os.MkdirAll(filepath.Dir(committed), 0o755); err != nil {
		t.Fatal(err)
	}
	artifacts := filepath.Join(dir, "workspace", "artifacts", "context-bench")

	run([]string{"ledger", "-tasks", tasks, "-results", results, "-ledger", committed,
		"-run-id", "r1", "-commit", "abc", "-arms", "fathom grep", "-repeat", "3",
		"-env", "compiled-layer", "-scope", contextbench.ScopeTargeted, "-only", "T1",
		"-artifacts", artifacts})

	if _, err := os.Stat(committed); !os.IsNotExist(err) {
		rows, _ := contextbench.LoadLedger(committed)
		t.Fatalf("the committed ledger was written by a targeted run (%d rows) — it must stay untouched", len(rows))
	}
	rows, err := contextbench.LoadLedger(filepath.Join(artifacts, "ledger-targeted.jsonl"))
	if err != nil || len(rows) != 1 {
		t.Fatalf("targeted ledger: %v, %d rows, want 1", err, len(rows))
	}
	// BP-33's second half: cells_lost counts only the cells the run
	// intended. T1 was the only selected task and every one of its cells
	// was scored — T2/T3 were never meant to be bought.
	if rows[0].CellsLost != 0 {
		t.Errorf("cells_lost = %d, want 0 (only T1 was selected and all its cells scored)", rows[0].CellsLost)
	}
	if rows[0].Scope != contextbench.ScopeTargeted {
		t.Errorf("scope = %q, want targeted", rows[0].Scope)
	}
}

// TestCmdLedger_ExplicitLedgerIsObeyed (BP-33): naming -ledger is how an
// operator says "this targeted run counts" — the redirect must never
// override an explicit choice.
func TestCmdLedger_ExplicitLedgerIsObeyed(t *testing.T) {
	dir := t.TempDir()
	tasks := writeGatedCorpus(t, dir)
	results := filepath.Join(dir, "results")
	writeCells(t, results, concat(
		threeTrials("T1", "fathom", true, 100), threeTrials("T1", "grep", false, 200),
	))
	ledger := filepath.Join(dir, "ledger.jsonl")

	run([]string{"ledger", "-tasks", tasks, "-results", results, "-ledger", ledger,
		"-run-id", "r1", "-commit", "abc", "-arms", "fathom grep", "-repeat", "3",
		"-env", "compiled-layer", "-scope", contextbench.ScopeTargeted, "-only", "T1",
		"-ledger-explicit", "-artifacts", filepath.Join(dir, "artifacts")})

	rows, err := contextbench.LoadLedger(ledger)
	if err != nil || len(rows) != 1 {
		t.Fatalf("explicit ledger: %v, %d rows, want 1", err, len(rows))
	}
}

// TestCmdLedger_RowCarriesPrimaryModelAndAux (BP-34): the row's model_id is
// the model that answered; the harness's small model rides along in
// aux_models instead of being comma-joined into the identity.
func TestCmdLedger_RowCarriesPrimaryModelAndAux(t *testing.T) {
	dir := t.TempDir()
	tasks := writeGatedCorpus(t, dir)
	results := filepath.Join(dir, "results")
	cells := concat(
		threeTrials("T1", "fathom", true, 100), threeTrials("T1", "grep", false, 200),
	)
	for i := range cells {
		cells[i].ModelID = "claude-sonnet-4-6"
		cells[i].OutputTokens = 900
		cells[i].AuxModels = []string{"claude-haiku-4-5"}
	}
	writeCells(t, results, cells)
	ledger := filepath.Join(dir, "ledger.jsonl")

	run([]string{"ledger", "-tasks", tasks, "-results", results, "-ledger", ledger,
		"-run-id", "r1", "-commit", "abc", "-arms", "fathom grep", "-repeat", "3",
		"-env", "compiled-layer"})

	rows, err := contextbench.LoadLedger(ledger)
	if err != nil || len(rows) != 1 {
		t.Fatalf("ledger: %v, %d rows", err, len(rows))
	}
	if rows[0].ModelID != "claude-sonnet-4-6" {
		t.Errorf("model_id = %q, want the primary claude-sonnet-4-6 alone", rows[0].ModelID)
	}
	if len(rows[0].AuxModels) != 1 || rows[0].AuxModels[0] != "claude-haiku-4-5" {
		t.Errorf("aux_models = %v, want [claude-haiku-4-5]", rows[0].AuxModels)
	}
}
