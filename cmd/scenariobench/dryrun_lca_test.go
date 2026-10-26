package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/scenariobench"
)

func TestCmdDryRunLCA(t *testing.T) {
	dataset := repoRelative(t, "internal", "scenariobench", "testdata", "lca", "fixture.jsonl")
	corpus, sha, err := loadCorpusOrLCA("lca", dataset)
	if err != nil {
		t.Fatal(err)
	}
	cells := filepath.Join(t.TempDir(), "cells")
	for _, task := range corpus.Tasks {
		for arm := range corpus.Arms {
			for _, name := range []string{"null", "oracle", "wrong-patch"} {
				for trial := 1; trial <= 3; trial++ {
					cell := scenariobench.CellResult{TaskID: task.ID, Arm: arm, Trial: trial, HarnessVersion: name + "-1", CorpusSHA256: sha, RunID: "lca-fixture", Success: name == "oracle", Sessions: []scenariobench.SessionResult{{}}, CellKey: task.ID + "-" + arm + "-" + name}
					if err := scenariobench.StoreScenarioCell(cells, cell); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
	}
	out := captureStdout(t, func() {
		if rc := run([]string{"dryrun", "-corpus", "lca", "-dataset", dataset, "-cells", cells, "-run-id", "lca-fixture"}); rc != 0 {
			t.Errorf("exit=%d", rc)
		}
	})
	for _, task := range corpus.Tasks {
		if !strings.Contains(out, task.ID+": keep\n") {
			t.Errorf("missing verdict for %s: %q", task.ID, out)
		}
	}
	help := captureStderr(t, func() { run([]string{"dryrun", "-help"}) })
	if !strings.Contains(help, datasetFlagUsage) {
		t.Errorf("missing dataset usage: %s", help)
	}
}
