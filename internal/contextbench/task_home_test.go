package contextbench

import (
	"encoding/json"
	"os"
	"testing"
)

// TestLoadTasks_HomeCorpus mirrors TestLoadTasks_RealCorpus (the B1
// grep-native corpus's test) for the home-turf, index-native corpus
// (docs/plans/home-bench.md H1-1): same shape checks (LoadTasks succeeds,
// 12-15 tasks, no duplicate IDs, every task validates) plus the one thing
// that's new here — every task must carry a recognized, non-empty Class,
// and all seven classes named by the plan's design section must appear at
// least once across the corpus.
func TestLoadTasks_HomeCorpus(t *testing.T) {
	tasks, err := LoadTasks("../../testdata/contextbench/tasks-home.json")
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if n := len(tasks); n < 12 || n > 15 {
		t.Fatalf("expected 12-15 tasks per docs/plans/home-bench.md H1-1, got %d", n)
	}

	wantClasses := map[TaskClass]bool{
		ClassObligations:     false,
		ClassVerify:          false,
		ClassBlastRadius:     false,
		ClassClaimProvenance: false,
		ClassConcernSearch:   false,
		ClassContradiction:   false,
		ClassStaleness:       false,
	}

	seen := map[string]bool{}
	for _, task := range tasks {
		if seen[task.ID] {
			t.Errorf("duplicate task id %q", task.ID)
		}
		seen[task.ID] = true
		if err := task.Validate(); err != nil {
			t.Errorf("task %s: %v", task.ID, err)
		}
		if task.Class == "" {
			t.Errorf("task %s: home-turf corpus tasks must carry a class (unlike the grep-native B1 corpus)", task.ID)
			continue
		}
		if _, known := wantClasses[task.Class]; !known {
			t.Errorf("task %s: class %q is not one of the seven docs/plans/home-bench.md H1-1 classes", task.ID, task.Class)
			continue
		}
		wantClasses[task.Class] = true
	}

	for class, present := range wantClasses {
		if !present {
			t.Errorf("class %q from docs/plans/home-bench.md H1-1 has zero tasks in the corpus", class)
		}
	}
}

// TestLoadTasks_HomeCorpus_PreRegistration proves the H2-1 pre-registration
// block travels with the corpus: metric "success", strict-better success,
// and a token ratio <= 1x are all present as committed data, not just prose
// in docs/plans/home-bench.md. LoadTasks ignores unknown top-level JSON
// keys (it only decodes "tasks"), so this test parses the raw file
// independently to assert the registration block's presence and shape.
func TestLoadTasks_HomeCorpus_PreRegistration(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/contextbench/tasks-home.json")
	if err != nil {
		t.Fatalf("reading corpus file: %v", err)
	}

	var doc struct {
		PreRegisteredCriterion struct {
			ID                     string `json:"id"`
			RegisteredBeforeAnyRun bool   `json:"registered_before_any_run"`
			RegisteredAtCommit     string `json:"registered_at_commit"`
			PrimaryMetric          string `json:"primary_metric"`
			Criterion              string `json:"criterion"`
		} `json:"pre_registered_criterion"`
		UnmeasurableByComparison []struct {
			Capability       string `json:"capability"`
			WhyGrepHasNoPath string `json:"why_grep_has_no_path"`
		} `json:"unmeasurable_by_comparison"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parsing corpus file: %v", err)
	}

	crit := doc.PreRegisteredCriterion
	if crit.ID != "H2-1" {
		t.Errorf("pre_registered_criterion.id = %q, want H2-1", crit.ID)
	}
	if !crit.RegisteredBeforeAnyRun {
		t.Error("pre_registered_criterion.registered_before_any_run must be true — H2-1 requires registration before any results run")
	}
	if crit.RegisteredAtCommit == "" {
		t.Error("pre_registered_criterion.registered_at_commit must be set — the commit the criterion was pinned at")
	}
	if crit.PrimaryMetric != "task success" {
		t.Errorf("pre_registered_criterion.primary_metric = %q, want %q per the plan's design section", crit.PrimaryMetric, "task success")
	}
	if crit.Criterion == "" {
		t.Error("pre_registered_criterion.criterion must state the win condition")
	}

	if len(doc.UnmeasurableByComparison) == 0 {
		t.Error("unmeasurable_by_comparison must be non-empty per the plan's fairness rule 3 (mark store-only capabilities instead of padding the scored corpus)")
	}
	for i, u := range doc.UnmeasurableByComparison {
		if u.Capability == "" || u.WhyGrepHasNoPath == "" {
			t.Errorf("unmeasurable_by_comparison[%d]: both capability and why_grep_has_no_path must be non-empty", i)
		}
	}
}
