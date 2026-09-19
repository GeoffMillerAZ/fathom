// SU-24 (docs/plans/substrate.md section F, ADR-041 decision 9): every
// committed corpus must plan cleanly — one cell per (task, arm, trial), a
// non-empty key, no error, no two cells colliding on the same key — before
// it is ever run. TestAllCorporaPlan is this row's own symbol (skip_test:
// true — the row's symbol IS the test): it walks testdata/scenariobench/*.json
// (the same set predictions_test.go's TestEveryScenarioTaskHasPredictions
// covers) plus testdata/scenariobench/lca/*.json (the external LCA corpus's
// own raw record file, loaded through LoadLCARecords/BuildLCACorpus exactly
// as `scenariobench plan -corpus lca -dataset <path>` does — never through
// LoadScenarioCorpus, which is not this file's shape) and runs each through
// PlanCells (plan.go), the pure enumeration `scenariobench plan` itself
// bottoms out on. No network, no subject checkout, no daemon.
package scenariobench

import (
	"path/filepath"
	"testing"
)

// allCorporaPlanOptions is a fixed, synthetic PlanOptions every corpus in
// this test is planned under — the exact values are arbitrary (this test
// never compares them against anything committed), they only need to be
// non-empty so CellInvariants.Validate() passes for every wired arm.
var allCorporaPlanOptions = PlanOptions{
	FathomCommit:   "test-fathom-commit",
	ModelID:        "test-model",
	HarnessVersion: "test-harness-version",
	Scoring:        "test-scoring-v1",
}

// nonDroppedTaskCount is len(c.Tasks) minus any task RC-15 marked dropped
// (planCells/PlanCells skip those entirely — see public-go.json's own
// SB-cli__cli-10009).
func nonDroppedTaskCount(c Corpus) int {
	n := 0
	for _, task := range c.Tasks {
		if task.Disposition != "dropped" {
			n++
		}
	}
	return n
}

// checkPlan asserts PlanCells(c, ...) produced exactly one cell per
// (non-dropped task, arm, trial), every cell carrying a non-empty key, and
// no two cells sharing a key.
func checkPlan(t *testing.T, label string, c Corpus, corpusSHA string) {
	t.Helper()
	trials := c.Criterion.MinTrials
	if trials <= 0 {
		trials = 1
	}
	opts := allCorporaPlanOptions
	opts.Trials = trials

	cells := PlanCells(c, opts, corpusSHA)

	want := nonDroppedTaskCount(c) * len(c.Arms) * trials
	if len(cells) != want {
		t.Errorf("%s: got %d cells, want %d (%d tasks x %d arms x %d trials)",
			label, len(cells), want, nonDroppedTaskCount(c), len(c.Arms), trials)
	}

	// A cell's Key (ScenarioCellKey over CellInvariants) deliberately carries
	// no trial component — cellTrialFile ("t%d.json") is what distinguishes
	// trials of the same (task, arm) ON DISK, inside the one directory the
	// shared key names (cell.go). So the collision floor this test enforces
	// is over (key, trial) pairs, never the bare key alone — two DIFFERENT
	// (task, arm) pairs colliding on the same key would be the real defect
	// (an invariant silently missing from the hash), which is exactly what
	// this still catches.
	type keyTrial struct {
		key   string
		trial int
	}
	seen := make(map[keyTrial]CellPlan, len(cells))
	for _, cp := range cells {
		if cp.Key == "" {
			t.Errorf("%s: task %s arm %s trial %d: empty cell key", label, cp.Task.ID, cp.Arm, cp.Trial)
			continue
		}
		if err := cp.Inv.Validate(); err != nil {
			t.Errorf("%s: task %s arm %s trial %d: invariants: %v", label, cp.Task.ID, cp.Arm, cp.Trial, err)
		}
		kt := keyTrial{cp.Key, cp.Trial}
		if prior, ok := seen[kt]; ok {
			t.Errorf("%s: task %s arm %s trial %d shares a (key, trial) pair with task %s arm %s trial %d",
				label, cp.Task.ID, cp.Arm, cp.Trial, prior.Task.ID, prior.Arm, prior.Trial)
			continue
		}
		seen[kt] = cp
	}
}

func TestAllCorporaPlan(t *testing.T) {
	t.Run("testdata/scenariobench/*.json", func(t *testing.T) {
		for _, path := range scenarioCorpusFiles(t) {
			path := path
			t.Run(filepath.Base(path), func(t *testing.T) {
				c, sha, err := loadCorpusForPlan(path)
				if err != nil {
					t.Fatalf("load %s: %v", path, err)
				}
				checkPlan(t, path, c, sha)
			})
		}
	})

	t.Run("testdata/scenariobench/lca/*.json", func(t *testing.T) {
		matches, err := filepath.Glob("../../testdata/scenariobench/lca/*.json")
		if err != nil {
			t.Fatalf("glob lca corpora: %v", err)
		}
		if len(matches) == 0 {
			t.Fatal("no lca corpora found under testdata/scenariobench/lca")
		}
		for _, path := range matches {
			path := path
			t.Run(filepath.Base(path), func(t *testing.T) {
				records, err := LoadLCARecords(path)
				if err != nil {
					t.Fatalf("LoadLCARecords(%s): %v", path, err)
				}
				c, err := BuildLCACorpus(records, path)
				if err != nil {
					t.Fatalf("BuildLCACorpus(%s): %v", path, err)
				}
				sha, err := CanonicalCorpusSHA256(c)
				if err != nil {
					t.Fatalf("CanonicalCorpusSHA256(%s): %v", path, err)
				}
				checkPlan(t, path, c, sha)
			})
		}
	})
}

// loadCorpusForPlan mirrors cmd/scenariobench's own loadCorpus (main.go):
// LoadScenarioCorpus plus the corpus file's own CorpusSHA256, side by side,
// exactly what planCells needs beyond the corpus itself. Re-implemented
// here (rather than imported — cmd/scenariobench is package main) using the
// same two exported functions cmd/scenariobench's own loadCorpus calls.
func loadCorpusForPlan(path string) (Corpus, string, error) {
	c, err := LoadScenarioCorpus(path)
	if err != nil {
		return Corpus{}, "", err
	}
	sha, err := CorpusSHA256(path)
	if err != nil {
		return Corpus{}, "", err
	}
	return c, sha, nil
}
