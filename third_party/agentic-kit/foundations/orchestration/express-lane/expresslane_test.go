package expresslane_test

import (
	"bytes"
	"testing"

	el "github.com/geoffmilleraz/agentic-kit/foundations/orchestration/express-lane"
)

// ---- helpers ---------------------------------------------------------------

func validTask(id string) el.Task {
	return el.Task{
		ID:      id,
		Payload: []byte("small payload"),
		Spec: el.TaskSpec{
			Description:          "Summarise the Q4 revenue report",
			EstimatedComplexity:  3,
			RequiredCapabilities: []string{"summarisation"},
		},
	}
}

// ---- fast lane: eligible task ---------------------------------------------

func TestAssess_FastLane_Eligible(t *testing.T) {
	task := validTask("t-001")
	d := el.Assess(task, el.DefaultConfig())
	if d.Lane != el.LaneFast {
		t.Errorf("expected LaneFast, got %q", d.Lane)
	}
	if d.Reason != el.ReasonEligible {
		t.Errorf("expected ReasonEligible, got %q", d.Reason)
	}
	if d.TaskID != task.ID {
		t.Errorf("expected TaskID %q, got %q", task.ID, d.TaskID)
	}
}

// ---- full lane: size exceeded ---------------------------------------------

func TestAssess_FullLane_SizeExceeded(t *testing.T) {
	cfg := el.Config{MaxPayloadBytes: 10}
	task := el.Task{
		ID:      "t-002",
		Payload: bytes.Repeat([]byte("x"), 11),
		Spec: el.TaskSpec{
			Description:         "some task",
			EstimatedComplexity: 2,
		},
	}
	d := el.Assess(task, cfg)
	if d.Lane != el.LaneFull {
		t.Errorf("expected LaneFull, got %q", d.Lane)
	}
	if d.Reason != el.ReasonSizeExceeded {
		t.Errorf("expected ReasonSizeExceeded, got %q", d.Reason)
	}
}

// ---- full lane: size exactly at threshold is OK ---------------------------

func TestAssess_AtThreshold_IsFast(t *testing.T) {
	cfg := el.Config{MaxPayloadBytes: 10}
	task := el.Task{
		ID:      "t-003",
		Payload: bytes.Repeat([]byte("x"), 10),
		Spec: el.TaskSpec{
			Description:         "some task",
			EstimatedComplexity: 1,
		},
	}
	d := el.Assess(task, cfg)
	if d.Lane != el.LaneFast {
		t.Errorf("payload == threshold should be fast, got %q", d.Lane)
	}
}

// ---- full lane: spec incomplete — missing description ---------------------

func TestAssess_FullLane_SpecIncomplete_NoDescription(t *testing.T) {
	task := el.Task{
		ID:      "t-004",
		Payload: []byte("small"),
		Spec: el.TaskSpec{
			Description:         "",
			EstimatedComplexity: 3,
		},
	}
	d := el.Assess(task, el.DefaultConfig())
	if d.Lane != el.LaneFull {
		t.Errorf("expected LaneFull for missing description, got %q", d.Lane)
	}
	if d.Reason != el.ReasonSpecIncomplete {
		t.Errorf("expected ReasonSpecIncomplete, got %q", d.Reason)
	}
}

// ---- full lane: spec incomplete — zero complexity ------------------------

func TestAssess_FullLane_SpecIncomplete_ZeroComplexity(t *testing.T) {
	task := el.Task{
		ID:      "t-005",
		Payload: []byte("small"),
		Spec: el.TaskSpec{
			Description:         "valid description",
			EstimatedComplexity: 0,
		},
	}
	d := el.Assess(task, el.DefaultConfig())
	if d.Lane != el.LaneFull {
		t.Errorf("expected LaneFull for zero complexity, got %q", d.Lane)
	}
	if d.Reason != el.ReasonSpecIncomplete {
		t.Errorf("expected ReasonSpecIncomplete, got %q", d.Reason)
	}
}

// ---- full lane: spec incomplete — negative complexity -------------------

func TestAssess_FullLane_SpecIncomplete_NegativeComplexity(t *testing.T) {
	task := el.Task{
		ID:      "t-006",
		Payload: []byte("small"),
		Spec: el.TaskSpec{
			Description:         "valid description",
			EstimatedComplexity: -1,
		},
	}
	d := el.Assess(task, el.DefaultConfig())
	if d.Lane != el.LaneFull {
		t.Errorf("expected LaneFull for negative complexity, got %q", d.Lane)
	}
}

// ---- size check precedes spec check ---------------------------------------

func TestAssess_SizeCheckBeforeSpec(t *testing.T) {
	// Oversized payload AND incomplete spec — size must win.
	cfg := el.Config{MaxPayloadBytes: 5}
	task := el.Task{
		ID:      "t-007",
		Payload: bytes.Repeat([]byte("x"), 100),
		Spec:    el.TaskSpec{}, // also incomplete
	}
	d := el.Assess(task, cfg)
	if d.Reason != el.ReasonSizeExceeded {
		t.Errorf("expected size check to fire first, got %q", d.Reason)
	}
}

// ---- determinism ----------------------------------------------------------

func TestAssess_Deterministic(t *testing.T) {
	task := validTask("determ-1")
	cfg := el.DefaultConfig()

	d1 := el.Assess(task, cfg)
	d2 := el.Assess(task, cfg)

	if d1 != d2 {
		t.Errorf("Assess is not deterministic: %+v vs %+v", d1, d2)
	}
}

// ---- zero-value config falls back to defaults ----------------------------

func TestAssess_ZeroConfig_FallsBack(t *testing.T) {
	task := validTask("zero-cfg-1")
	d := el.Assess(task, el.Config{})
	if d.Lane != el.LaneFast {
		t.Errorf("zero config should use defaults and accept small task; got %q", d.Lane)
	}
}
