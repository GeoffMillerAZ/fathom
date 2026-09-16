package scenariobench

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// predictions_test.go is SB-26's anchor: TestEveryScenarioTaskHasPredictions
// gates every committed scenario corpus (testdata/scenariobench/*.json,
// never the raw fixture transcripts under streams/) on the pre-registration
// discipline vision 27 §2 requires: a criterion, and a prediction per arm
// per task — plus at least one registered no-effect case per corpus, so a
// run cannot silently visit only the rows where fathom is expected to win.

// scenarioCorpusFiles lists the committed corpora: every *.json directly
// under testdata/scenariobench (a Glob on a fixed pattern already excludes
// anything one level deeper, such as testdata/scenariobench/streams/*.json
// — the raw-transcript fixtures SB-15's FakeHarness replays, which are not
// corpora and carry no criterion of their own).
func scenarioCorpusFiles(t *testing.T) []string {
	t.Helper()
	matches, err := filepath.Glob("../../testdata/scenariobench/*.json")
	if err != nil {
		t.Fatalf("glob scenario corpora: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("no scenario corpora found under testdata/scenariobench")
	}
	return matches
}

func TestEveryScenarioTaskHasPredictions(t *testing.T) {
	for _, path := range scenarioCorpusFiles(t) {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			if strings.Contains(filepath.ToSlash(path), "/streams/") {
				t.Skip("raw transcript fixtures, not a corpus")
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			var c Corpus
			if err := json.Unmarshal(data, &c); err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			if c.Criterion.ID == "" {
				t.Fatalf("%s: no criterion", path)
			}
			if len(c.Arms) == 0 {
				t.Fatalf("%s: no arms", path)
			}
			if len(c.Tasks) == 0 {
				t.Fatalf("%s: no tasks", path)
			}

			sawNoEffectForTreatment := false
			for _, task := range c.Tasks {
				for arm := range c.Arms {
					pred, ok := task.Predictions[arm]
					if !ok || strings.TrimSpace(pred) == "" {
						t.Errorf("%s: task %s has no prediction for arm %q", path, task.ID, arm)
						continue
					}
					if arm == c.Criterion.Treatment && isNoEffectPrediction(pred) {
						sawNoEffectForTreatment = true
					}
				}
				if task.Derivation == "" {
					t.Errorf("%s: task %s has no derivation", path, task.ID)
				}
			}
			if !sawNoEffectForTreatment {
				t.Errorf("%s: no task predicts \"no effect\"/\"no-effect\" for treatment arm %q — a corpus that only registers wins is marketing, not a bench (vision 27 §1)", path, c.Criterion.Treatment)
			}
		})
	}
}

// isNoEffectPrediction reports whether pred registers a no-effect
// expectation, accepting "no effect" and "no-effect" (case-insensitive).
func isNoEffectPrediction(pred string) bool {
	lower := strings.ToLower(pred)
	return strings.Contains(lower, "no effect") || strings.Contains(lower, "no-effect")
}

// goldPackageDirs returns the distinct, sorted package directories
// (path.Dir — gold.Files are repo-relative, slash-separated, never
// OS-native) task.Gold.Files names.
func goldPackageDirs(files []string) []string {
	set := map[string]bool{}
	for _, f := range files {
		set[path.Dir(f)] = true
	}
	dirs := make([]string, 0, len(set))
	for d := range set {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	return dirs
}

// TestCrossPackageTaskHasEnoughWorkerBudget pins O55 item 6: a
// planner-workers task whose gold touches more than one Go package needs a
// worker budget that can actually reach every package's own row — either
// Sessions.Workers >= 2, or an EXPLICIT Sessions.MaxWorkersTotal at least
// as high as the package count, declared on the CORPUS ITSELF rather than
// left to RunSessions' own runtime default (Workers*3, O55 item 3) to
// happen to cover it.
//
// RE-SB-cli__cli-10043-K1 (testdata/scenariobench/reuse-go.json) is
// workers=1 for a two-package fix with no max_workers_total set — a null
// cell by construction before item 3's reassignment fix, and still an
// UNDER-DECLARED one after it (the runtime default of 3 happens to cover
// 2 packages here, but the corpus never says so, and would silently fall
// short for a wider fix). This test is meant to fail loudly for K1 until
// the corpus owner sets workers or max_workers_total explicitly on it —
// the corpus is deliberately left as-is here; do not silence this failure
// by editing the corpus in this wave.
func TestCrossPackageTaskHasEnoughWorkerBudget(t *testing.T) {
	for _, path := range scenarioCorpusFiles(t) {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			if strings.Contains(filepath.ToSlash(path), "/streams/") {
				t.Skip("raw transcript fixtures, not a corpus")
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			var c Corpus
			if err := json.Unmarshal(data, &c); err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			for _, task := range c.Tasks {
				if task.Sessions.Mode != ModePlannerWorkers {
					continue
				}
				dirs := goldPackageDirs(task.Gold.Files)
				packages := len(dirs)
				if packages <= 1 {
					continue
				}
				if task.Sessions.Workers >= 2 || task.Sessions.MaxWorkersTotal >= packages {
					continue
				}
				t.Errorf("%s: task %s touches %d packages %v but declares workers=%d, max_workers_total=%d — neither workers>=2 nor max_workers_total>=packages, so the reassignment loop (O55 item 3) is not guaranteed to reach every package's row",
					path, task.ID, packages, dirs, task.Sessions.Workers, task.Sessions.MaxWorkersTotal)
			}
		})
	}
}
