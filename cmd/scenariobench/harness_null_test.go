package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/scenariobench"
)

func TestCmdRun_NullHarnessFailsLocalization(t *testing.T) {
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOSUMDB", "off")
	t.Setenv("FATHOM_DECIDE", "off")
	dir := t.TempDir()
	repo, commit := gitFixtureSubject(t)
	corpusPath := writeCorpusFixture(t, dir, repo, commit)
	corpus, err := scenariobench.LoadScenarioCorpus(corpusPath)
	if err != nil {
		t.Fatal(err)
	}
	corpus.Tasks[0].Category = scenariobench.CategoryLocalization
	corpus.Tasks[0].Oracle = scenariobench.Oracle{Localization: true}
	corpus.Tasks[0].Gold = scenariobench.Gold{Files: []string{"thing.go"}}
	data, err := json.Marshal(corpus)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corpusPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	cellsDir := filepath.Join(dir, "cells")
	stdout := captureStdout(t, func() {
		rc := run([]string{
			"run", "-corpus", corpusPath, "-root", filepath.Join(dir, "root"),
			"-cells", cellsDir, "-raw-dir", filepath.Join(dir, "raw"),
			"-run-id", "null-fixture", "-harness-version", "null-1", "-harness", "null",
		})
		if rc != 1 {
			t.Errorf("run exit = %d, want 1 (graded failure)", rc)
		}
	})
	if !strings.Contains(stdout, "T1/control/t1: FAIL") {
		t.Fatalf("expected a graded FAIL, got %q", stdout)
	}
	cell := readStoredCell(t, findStoredCell(t, cellsDir))
	if cell.Success || cell.Oracle.Passed || cell.InstrumentDefect != "" {
		t.Errorf("expected a measured failure, got %+v", cell)
	}
	if cell.Oracle.Localization == nil || cell.Oracle.Localization.HitAt1 || cell.Oracle.Localization.HitAt5 {
		t.Errorf("expected empty answer to miss gold files, got %+v", cell.Oracle.Localization)
	}
	if len(cell.Sessions) != 1 || cell.Sessions[0].FinalAnswer != "" || len(cell.Sessions[0].FilesEdited) != 0 {
		t.Errorf("expected one session with no answer or edits, got %+v", cell.Sessions)
	}
}

func TestCmdRun_HelpListsNullHarness(t *testing.T) {
	help := captureStderr(t, func() { run([]string{"run", "-help"}) })
	if !strings.Contains(help, "null (no edits or answer, no spend)") {
		t.Fatalf("run help omits null harness: %s", help)
	}
}
