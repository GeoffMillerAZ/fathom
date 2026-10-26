package scenariobench

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunExternalCorpus_NullHarnessFailsHiddenTest(t *testing.T) {
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOSUMDB", "off")
	t.Setenv("FATHOM_DECIDE", "off")
	repo, commit := gitFixtureRepoWithFiles(t, map[string]string{
		"go.mod":        "module nullfixture\n\ngo 1.21\n",
		"thing.go":      "package thing\n\nfunc Add(a, b int) int { return a - b }\n",
		"thing_test.go": "package thing\n\nimport \"testing\"\n\nfunc TestStable(t *testing.T) { if Add(0, 0) != 0 { t.Fatal(\"zero sum changed\") } }\n",
	})
	corpus := Corpus{
		Arms: map[string]Arm{"control": {Instructions: "none"}},
		Tasks: []Task{{
			ID: "null-hidden-test", Category: CategoryBugFix,
			Prompt:   "Fix Add so it returns the sum.",
			Subject:  Subject{RepoURL: repo, BaseCommit: commit},
			Sessions: Sessions{Mode: ModeSingle, Count: 1},
			Oracle:   Oracle{FailToPass: []string{"TestFix"}, PassToPass: []string{"TestStable"}},
			Gold:     Gold{TestPatch: oracleCalibrationTestPatch},
		}},
	}
	sha, err := CanonicalCorpusSHA256(corpus)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	opts := RunExternalCorpusOptions{
		Root: root, Corpus: corpus, CorpusSHA256: sha, Harness: NewNullHarness(),
		RunID: "null-hidden-test", HarnessVersion: "null-1",
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
	if cell.Success || cell.Oracle.Passed || cell.Oracle.FailToPassOK || !cell.Oracle.PassToPassOK {
		t.Errorf("want hidden-test FAIL with stable tests passing, got %+v", cell.Oracle)
	}
	calib := cell.Oracle.Calibration
	if calib == nil || calib.NotMeasurable != "" || calib.FailToPassFailedAtBase != 1 || calib.PassToPassEffective != 1 {
		t.Fatalf("want a calibrated hidden test, got %+v", calib)
	}
	output, err := os.ReadFile(cell.Oracle.TestOutputRef)
	if err != nil || !strings.Contains(string(output), "--- FAIL: TestFix") {
		t.Errorf("want evidence that the hidden test ran and failed: %s, err=%v", output, err)
	}
	if len(cell.Sessions) != 1 || cell.Sessions[0].FinalAnswer != "" || len(cell.Sessions[0].FilesEdited) != 0 || cell.Readings.TotalCostUSD != 0 {
		t.Errorf("want no answer, edits or spend, got %+v", cell)
	}
	stored, ok, err := BorrowScenarioCell(opts.CellsDir, cell.CellKey, 1)
	if err != nil || !ok || stored.Success || stored.Oracle.Passed {
		t.Errorf("want stored FAIL, got %+v, borrowed=%v, err=%v", stored, ok, err)
	}
}
