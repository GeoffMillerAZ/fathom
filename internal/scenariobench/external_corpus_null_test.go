package scenariobench

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRunExternalCorpus_NullHarnessFailsLocalization(t *testing.T) {
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOSUMDB", "off")
	t.Setenv("FATHOM_DECIDE", "off")
	repo, commit := gitFixtureRepo(t)
	corpus := Corpus{
		Arms: map[string]Arm{"control": {Instructions: "none"}},
		Tasks: []Task{{
			ID: "null-localization", Category: CategoryLocalization,
			Prompt:   "Name the file to change.",
			Subject:  Subject{RepoURL: repo, BaseCommit: commit},
			Sessions: Sessions{Mode: ModeSingle, Count: 1},
			Oracle:   Oracle{Localization: true}, Gold: Gold{Files: []string{"marker.txt"}},
		}},
	}
	sha, err := CanonicalCorpusSHA256(corpus)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	opts := RunExternalCorpusOptions{
		Root: root, Corpus: corpus, CorpusSHA256: sha, Harness: NewNullHarness(),
		RunID: "null-localization", HarnessVersion: "null-1",
		RawDir: filepath.Join(root, "raw"), CellsDir: filepath.Join(root, "cells"),
	}
	result, err := RunExternalCorpus(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Lost) != 0 || len(result.Cells) != 1 || len(result.Rows) != 1 {
		t.Fatalf("want one measured cell and row, got %+v", result)
	}
	cell := result.Cells[0]
	if cell.Success || cell.Oracle.Passed || cell.Oracle.Localization == nil || cell.Oracle.Localization.HitAt1 {
		t.Fatalf("want localization FAIL, got %+v", cell)
	}
	if result.Rows[0].Trials != 1 || result.Rows[0].Successes != 0 {
		t.Errorf("want a failed trial in the ledger row, got %+v", result.Rows[0])
	}
	stored, ok, err := BorrowScenarioCell(opts.CellsDir, cell.CellKey, 1)
	if err != nil || !ok || stored.Success || stored.Oracle.Passed {
		t.Errorf("want stored FAIL, got %+v, borrowed=%v, err=%v", stored, ok, err)
	}
	if len(cell.Sessions) != 1 || cell.Sessions[0].Turns != 0 || cell.Sessions[0].FinalAnswer != "" || len(cell.Sessions[0].FilesEdited) != 0 || cell.Readings.TotalCostUSD != 0 {
		t.Errorf("want a zero-turn, empty, spend-free session, got %+v", cell)
	}
}
