package viz

import (
	"strings"
	"testing"
)

func stepsFixture() []Step {
	return []Step{
		{Label: "filed", State: StepDone},
		{Label: "discovery", State: StepDone},
		{Label: "trial", State: StepCurrent},
		{Label: "judgment", State: StepUpcoming},
		{Label: "resolved", State: StepUpcoming},
	}
}

func TestSteps_Golden(t *testing.T) {
	got := Steps(stepsFixture(), StepsOptions{AriaLabel: "case status"})
	assertGolden(t, got, "steps_default")
}

func TestSteps_Empty(t *testing.T) {
	got := Steps(nil, StepsOptions{})
	if !strings.Contains(got, "<svg") {
		t.Errorf("empty steps missing svg")
	}
	if strings.Contains(got, "<circle") {
		t.Errorf("empty steps should render no nodes")
	}
}

func TestSteps_Single(t *testing.T) {
	got := Steps([]Step{{Label: "only", State: StepCurrent}}, StepsOptions{})
	if strings.Contains(got, "NaN") || strings.Contains(got, "Inf") {
		t.Errorf("single step has NaN/Inf: %s", got)
	}
	if !strings.Contains(got, "<circle") {
		t.Errorf("single step should render a node")
	}
}

func TestSteps_StatesRender(t *testing.T) {
	got := Steps(stepsFixture(), StepsOptions{})
	// Done steps draw a check path; current draws an inner dot. Expect both.
	if !strings.Contains(got, "<path") {
		t.Errorf("done steps should render check glyphs")
	}
	// Connectors: 4 between 5 steps.
	if c := strings.Count(got, "<line"); c != 4 {
		t.Errorf("expected 4 connectors, got %d", c)
	}
	if !strings.Contains(got, "ak-color-accent") {
		t.Errorf("done/current should use accent by default")
	}
}

func TestSteps_Escaping(t *testing.T) {
	got := Steps([]Step{{Label: `<script>`, State: StepDone}}, StepsOptions{})
	if strings.Contains(got, "<script>") {
		t.Errorf("steps did not escape label")
	}
}

func TestSteps_Determinism(t *testing.T) {
	a := Steps(stepsFixture(), StepsOptions{})
	b := Steps(stepsFixture(), StepsOptions{})
	if a != b {
		t.Errorf("steps non-deterministic")
	}
}

func TestSteps_Accessibility(t *testing.T) {
	got := Steps(stepsFixture(), StepsOptions{AriaLabel: "status"})
	if !strings.Contains(got, `role="img"`) || !strings.Contains(got, "<title>") {
		t.Errorf("steps missing accessibility")
	}
}
