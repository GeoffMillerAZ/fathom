package evalgate_test

import (
	"testing"

	eg "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/eval-gate"
)

func TestGate_AllPass(t *testing.T) {
	g := eg.Gate{Threshold: 1.0}
	checks := []eg.Check{
		{Name: "a", Passed: true, Weight: 1},
		{Name: "b", Passed: true, Weight: 1},
	}
	r := g.Evaluate(checks)
	if !r.Passed {
		t.Errorf("expected passed=true, got false")
	}
	if r.Score != 1.0 {
		t.Errorf("expected score=1.0, got %v", r.Score)
	}
	if len(r.Failures) != 0 {
		t.Errorf("expected no failures, got %v", r.Failures)
	}
}

func TestGate_AllFail(t *testing.T) {
	g := eg.Gate{Threshold: 0.5}
	checks := []eg.Check{
		{Name: "a", Passed: false, Weight: 1},
		{Name: "b", Passed: false, Weight: 1},
	}
	r := g.Evaluate(checks)
	if r.Passed {
		t.Errorf("expected passed=false, got true")
	}
	if r.Score != 0.0 {
		t.Errorf("expected score=0.0, got %v", r.Score)
	}
	if len(r.Failures) != 2 {
		t.Errorf("expected 2 failures, got %v", r.Failures)
	}
}

func TestGate_WeightedPartial(t *testing.T) {
	// heavy check passes, light check fails → score = 2/3 ≈ 0.666
	g := eg.Gate{Threshold: 0.6}
	checks := []eg.Check{
		{Name: "heavy", Passed: true, Weight: 2},
		{Name: "light", Passed: false, Weight: 1},
	}
	r := g.Evaluate(checks)
	if !r.Passed {
		t.Errorf("expected passed=true (score %.4f >= threshold 0.6)", r.Score)
	}
	want := 2.0 / 3.0
	if r.Score < want-1e-9 || r.Score > want+1e-9 {
		t.Errorf("expected score ≈ %.4f, got %.4f", want, r.Score)
	}
	if len(r.Failures) != 1 || r.Failures[0] != "light" {
		t.Errorf("expected failures=[light], got %v", r.Failures)
	}
}

func TestGate_ZeroThresholdTreatedAsOne(t *testing.T) {
	// zero Threshold is treated as 1.0 — all checks must pass
	g := eg.Gate{Threshold: 0}
	checks := []eg.Check{
		{Name: "a", Passed: true, Weight: 1},
		{Name: "b", Passed: false, Weight: 1},
	}
	r := g.Evaluate(checks)
	if r.Passed {
		t.Errorf("expected passed=false because threshold=1.0 (zero default)")
	}
}

func TestGate_NonPositiveWeightSkipped(t *testing.T) {
	g := eg.Gate{Threshold: 1.0}
	checks := []eg.Check{
		{Name: "valid", Passed: true, Weight: 1},
		{Name: "zero-weight-fail", Passed: false, Weight: 0},
	}
	r := g.Evaluate(checks)
	// zero-weight check does not affect score
	if r.Score != 1.0 {
		t.Errorf("expected score=1.0 (zero-weight check excluded), got %v", r.Score)
	}
	// but it still appears in failures
	if len(r.Failures) != 1 || r.Failures[0] != "zero-weight-fail" {
		t.Errorf("expected failures=[zero-weight-fail], got %v", r.Failures)
	}
}

func TestGate_EmptyChecks(t *testing.T) {
	g := eg.Gate{Threshold: 0.5}
	r := g.Evaluate(nil)
	if r.Score != 0.0 {
		t.Errorf("expected score=0.0 for empty checks, got %v", r.Score)
	}
	// score(0) < threshold(0.5) → not passed
	if r.Passed {
		t.Errorf("expected passed=false for empty checks")
	}
}

func TestGate_Deterministic(t *testing.T) {
	g := eg.Gate{Threshold: 0.75}
	checks := []eg.Check{
		{Name: "c1", Passed: true, Weight: 3},
		{Name: "c2", Passed: false, Weight: 1},
	}
	r1 := g.Evaluate(checks)
	r2 := g.Evaluate(checks)
	if r1.Passed != r2.Passed || r1.Score != r2.Score {
		t.Errorf("evaluation is not deterministic: %+v vs %+v", r1, r2)
	}
}
