package agenticpracticesevaluator_test

import (
	"testing"

	ape "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/agentic-practices-evaluator"
)

// hasCitations is a reusable Practice check: the run must have at least one
// citation.
var hasCitations = ape.Practice{
	Name:        "uses-evidence-citations",
	Description: "The run must produce at least one evidence citation.",
	Check: func(run ape.AgentRun) bool {
		return len(run.Citations) > 0
	},
}

// hasSteps is a reusable Practice check: the run must record at least one step.
var hasSteps = ape.Practice{
	Name:        "records-steps",
	Description: "The run must record at least one named step.",
	Check: func(run ape.AgentRun) bool {
		return len(run.Steps) > 0
	},
}

// hasConfidence is a reusable Practice check: the run metadata must include a
// "confidence" key.
var hasConfidence = ape.Practice{
	Name:        "declares-confidence",
	Description: "The run metadata must include a confidence key.",
	Check: func(run ape.AgentRun) bool {
		_, ok := run.Metadata["confidence"]
		return ok
	},
}

func TestEvaluateAllPracticesPassed(t *testing.T) {
	t.Parallel()

	run := ape.AgentRun{
		Steps:     []string{"decompose", "analyse", "decide"},
		Citations: []string{"evidence:span-1"},
		Metadata:  map[string]string{"confidence": "high"},
	}
	sc := ape.Evaluate(run, []ape.Practice{hasCitations, hasSteps, hasConfidence})
	if sc.Score != 1.0 {
		t.Errorf("expected Score=1.0, got %f", sc.Score)
	}
	if len(sc.Misses) != 0 {
		t.Errorf("expected no misses, got %v", sc.Misses)
	}
}

func TestEvaluateNoPracticesPerfectScore(t *testing.T) {
	t.Parallel()

	run := ape.AgentRun{}
	sc := ape.Evaluate(run, nil)
	if sc.Score != 1.0 {
		t.Errorf("expected Score=1.0 for empty practices, got %f", sc.Score)
	}
	if len(sc.Misses) != 0 {
		t.Errorf("expected no misses for empty practices, got %v", sc.Misses)
	}
}

func TestEvaluateOneMissReducesScore(t *testing.T) {
	t.Parallel()

	// Run has steps and confidence but no citations.
	run := ape.AgentRun{
		Steps:    []string{"decompose"},
		Metadata: map[string]string{"confidence": "medium"},
	}
	sc := ape.Evaluate(run, []ape.Practice{hasCitations, hasSteps, hasConfidence})
	// 2 of 3 passed → 0.666...
	expected := 2.0 / 3.0
	if sc.Score != expected {
		t.Errorf("expected Score=%f, got %f", expected, sc.Score)
	}
	if len(sc.Misses) != 1 {
		t.Errorf("expected 1 miss, got %d", len(sc.Misses))
	}
	if sc.Misses[0].PracticeName != "uses-evidence-citations" {
		t.Errorf("expected miss for uses-evidence-citations, got %q", sc.Misses[0].PracticeName)
	}
}

func TestEvaluateAllMissesScoreZero(t *testing.T) {
	t.Parallel()

	run := ape.AgentRun{} // no steps, no citations, no metadata
	sc := ape.Evaluate(run, []ape.Practice{hasCitations, hasSteps, hasConfidence})
	if sc.Score != 0.0 {
		t.Errorf("expected Score=0.0, got %f", sc.Score)
	}
	if len(sc.Misses) != 3 {
		t.Errorf("expected 3 misses, got %d", len(sc.Misses))
	}
}

func TestEvaluateIsDeterministic(t *testing.T) {
	t.Parallel()

	run := ape.AgentRun{
		Steps:     []string{"step-a"},
		Citations: []string{"cite-1", "cite-2"},
		Metadata:  map[string]string{"confidence": "low"},
	}
	practices := []ape.Practice{hasCitations, hasSteps, hasConfidence}

	sc1 := ape.Evaluate(run, practices)
	sc2 := ape.Evaluate(run, practices)
	if sc1.Score != sc2.Score {
		t.Errorf("Score is non-deterministic: %f vs %f", sc1.Score, sc2.Score)
	}
	if len(sc1.Misses) != len(sc2.Misses) {
		t.Errorf("Misses count is non-deterministic: %d vs %d", len(sc1.Misses), len(sc2.Misses))
	}
}

func TestEvaluateMissOrderMatchesPracticeOrder(t *testing.T) {
	t.Parallel()

	// Run fails all three practices; misses should be in declaration order.
	run := ape.AgentRun{}
	practices := []ape.Practice{hasConfidence, hasCitations, hasSteps}
	sc := ape.Evaluate(run, practices)
	if len(sc.Misses) != 3 {
		t.Fatalf("expected 3 misses, got %d", len(sc.Misses))
	}
	wantOrder := []string{"declares-confidence", "uses-evidence-citations", "records-steps"}
	for i, want := range wantOrder {
		if sc.Misses[i].PracticeName != want {
			t.Errorf("Misses[%d]: expected %q, got %q", i, want, sc.Misses[i].PracticeName)
		}
	}
}

func TestEvaluateNilCheckCountsAsMiss(t *testing.T) {
	t.Parallel()

	nilCheckPractice := ape.Practice{
		Name:        "nil-check",
		Description: "Practice with nil check always fails.",
		Check:       nil,
	}
	sc := ape.Evaluate(ape.AgentRun{}, []ape.Practice{nilCheckPractice})
	if len(sc.Misses) != 1 {
		t.Errorf("expected 1 miss for nil Check, got %d", len(sc.Misses))
	}
}
