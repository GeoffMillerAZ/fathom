package scenariobench

import (
	"encoding/json"
	"os"
	"path/filepath"
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
