package qaharness_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	qh "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/qa-harness"
)

func ctx() context.Context { return context.Background() }

// ---- MockJudge ----

func TestMockJudge_ReturnsFixedScore(t *testing.T) {
	j := qh.MockJudge{FixedScore: 0.75}
	score, err := j.Score(ctx(), qh.Item{ID: "x", Output: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 0.75 {
		t.Errorf("expected 0.75, got %v", score)
	}
}

func TestMockJudge_PropagatesError(t *testing.T) {
	sentinel := errors.New("judge unavailable")
	j := qh.MockJudge{FixedScore: 0, Err: sentinel}
	_, err := j.Score(ctx(), qh.Item{ID: "x", Output: "hello"})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}
}

// ---- RubricJudge ----

func TestRubricJudge_AllCriteriaPass(t *testing.T) {
	j := qh.RubricJudge{
		Criteria: []qh.RubricCriterion{
			{Name: "not-empty", Weight: 1, Check: func(i qh.Item) bool { return i.Output != "" }},
			{Name: "has-period", Weight: 1, Check: func(i qh.Item) bool { return strings.HasSuffix(i.Output, ".") }},
		},
	}
	score, err := j.Score(ctx(), qh.Item{ID: "a", Output: "Hello."})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 1.0 {
		t.Errorf("expected 1.0, got %v", score)
	}
}

func TestRubricJudge_AllCriteriaFail(t *testing.T) {
	j := qh.RubricJudge{
		Criteria: []qh.RubricCriterion{
			{Name: "has-question-mark", Weight: 2, Check: func(i qh.Item) bool { return strings.Contains(i.Output, "?") }},
		},
	}
	score, err := j.Score(ctx(), qh.Item{ID: "b", Output: "Hello."})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 0.0 {
		t.Errorf("expected 0.0, got %v", score)
	}
}

func TestRubricJudge_WeightedPartial(t *testing.T) {
	// heavy criterion passes, light criterion fails → score = 2/(2+1) ≈ 0.667
	j := qh.RubricJudge{
		Criteria: []qh.RubricCriterion{
			{Name: "heavy", Weight: 2, Check: func(i qh.Item) bool { return i.Output != "" }},
			{Name: "light", Weight: 1, Check: func(i qh.Item) bool { return false }},
		},
	}
	score, err := j.Score(ctx(), qh.Item{ID: "c", Output: "text"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := 2.0 / 3.0
	if score < want-1e-9 || score > want+1e-9 {
		t.Errorf("expected score≈%.4f, got %.4f", want, score)
	}
}

func TestRubricJudge_NoCriteria(t *testing.T) {
	j := qh.RubricJudge{}
	score, err := j.Score(ctx(), qh.Item{ID: "d", Output: "text"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 0.0 {
		t.Errorf("expected 0.0 for empty criteria, got %v", score)
	}
}

func TestRubricJudge_NonPositiveWeightSkipped(t *testing.T) {
	j := qh.RubricJudge{
		Criteria: []qh.RubricCriterion{
			{Name: "real", Weight: 1, Check: func(i qh.Item) bool { return true }},
			{Name: "zero-weight", Weight: 0, Check: func(i qh.Item) bool { return false }},
		},
	}
	score, err := j.Score(ctx(), qh.Item{ID: "e", Output: "text"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 1.0 {
		t.Errorf("expected 1.0 (zero-weight criterion excluded), got %v", score)
	}
}

func TestRubricJudge_Deterministic(t *testing.T) {
	j := qh.RubricJudge{
		Criteria: []qh.RubricCriterion{
			{Name: "c1", Weight: 3, Check: func(i qh.Item) bool { return strings.Contains(i.Output, "foo") }},
			{Name: "c2", Weight: 1, Check: func(i qh.Item) bool { return strings.Contains(i.Output, "bar") }},
		},
	}
	item := qh.Item{ID: "f", Output: "foo baz"}
	s1, _ := j.Score(ctx(), item)
	s2, _ := j.Score(ctx(), item)
	if s1 != s2 {
		t.Errorf("non-deterministic: %v vs %v", s1, s2)
	}
}

func TestRubricJudge_ScoreInRange(t *testing.T) {
	j := qh.RubricJudge{
		Criteria: []qh.RubricCriterion{
			{Name: "a", Weight: 1, Check: func(i qh.Item) bool { return true }},
		},
	}
	score, _ := j.Score(ctx(), qh.Item{ID: "g", Output: "x"})
	if score < 0 || score > 1 {
		t.Errorf("score %v out of [0,1]", score)
	}
}

// ---- Judge interface compliance ----

func TestJudgeInterface_MockJudge(t *testing.T) {
	var _ qh.Judge = qh.MockJudge{}
}

func TestJudgeInterface_RubricJudge(t *testing.T) {
	var _ qh.Judge = qh.RubricJudge{}
}
