package dadinvestigator_test

import (
	"strings"
	"testing"

	dad "github.com/geoffmilleraz/agentic-kit/foundations/investigation/dad-investigator"
)

// heavyEvidence generates evidence items whose combined weight will push the
// investigation to ConfidenceHigh after a single DAD loop.
func heavyEvidence() []dad.EvidenceItem {
	return []dad.EvidenceItem{
		{ID: "ev-001", Content: "Critical signal A.", Weight: 10},
		{ID: "ev-002", Content: "Critical signal B.", Weight: 10},
		{ID: "ev-003", Content: "Critical signal C.", Weight: 10},
	}
}

// lightEvidence generates evidence items with a low total weight.
func lightEvidence() []dad.EvidenceItem {
	return []dad.EvidenceItem{
		{ID: "ev-light-001", Content: "Weak signal.", Weight: 1},
	}
}

// runDADLoop drives the state machine to termination starting from an empty
// State. It feeds the provided evidence in the first Decompose step, then
// passes empty evidence for subsequent steps. Returns the terminal Decision.
func runDADLoop(t *testing.T, evidence []dad.EvidenceItem) (dad.State, dad.Decision) {
	t.Helper()
	inv := dad.Investigator{}
	state := dad.State{}
	var decision dad.Decision

	for step := 0; step < 50; step++ {
		ev := evidence
		if step > 0 {
			ev = nil
		}
		state, decision = inv.Step(state, ev)
		if decision.Terminal {
			return state, decision
		}
	}
	t.Fatal("DAD loop did not terminate within 50 steps")
	return state, decision
}

func TestDADConvergesToFoundWithHeavyEvidence(t *testing.T) {
	t.Parallel()

	state, decision := runDADLoop(t, heavyEvidence())
	if state.Phase != dad.PhaseFound {
		t.Errorf("expected Phase=found, got %q", state.Phase)
	}
	if decision.Confidence != dad.ConfidenceHigh {
		t.Errorf("expected Confidence=high, got %q", decision.Confidence)
	}
	if !decision.Terminal {
		t.Error("expected Terminal=true")
	}
}

func TestDADExhaustedWithLightEvidence(t *testing.T) {
	t.Parallel()

	state, decision := runDADLoop(t, lightEvidence())
	if state.Phase != dad.PhaseExhausted {
		t.Errorf("expected Phase=exhausted, got %q", state.Phase)
	}
	if decision.Terminal != true {
		t.Error("expected Terminal=true")
	}
}

func TestDADIsDeterministic(t *testing.T) {
	t.Parallel()

	evidence := heavyEvidence()
	_, d1 := runDADLoop(t, evidence)
	_, d2 := runDADLoop(t, evidence)

	if d1.Finding != d2.Finding {
		t.Errorf("Finding is non-deterministic:\n  run1: %q\n  run2: %q", d1.Finding, d2.Finding)
	}
	if d1.Confidence != d2.Confidence {
		t.Errorf("Confidence is non-deterministic: %q vs %q", d1.Confidence, d2.Confidence)
	}
	if d1.Terminal != d2.Terminal {
		t.Errorf("Terminal is non-deterministic: %v vs %v", d1.Terminal, d2.Terminal)
	}
}

func TestDADPhaseTransitions(t *testing.T) {
	t.Parallel()

	inv := dad.Investigator{}
	state := dad.State{}

	// Step 1: Decompose — should transition to Analyse.
	state, _ = inv.Step(state, heavyEvidence())
	if state.Phase != dad.PhaseAnalyse {
		t.Fatalf("after Decompose expected Phase=analyse, got %q", state.Phase)
	}

	// Step 2: Analyse — should transition to Decide.
	state, _ = inv.Step(state, nil)
	if state.Phase != dad.PhaseDecide {
		t.Fatalf("after Analyse expected Phase=decide, got %q", state.Phase)
	}

	// Step 3: Decide — with heavy evidence, should terminate in Found.
	state, dec := inv.Step(state, nil)
	if !dec.Terminal {
		t.Fatalf("after Decide with heavy evidence expected Terminal=true")
	}
	if state.Phase != dad.PhaseFound {
		t.Errorf("expected Phase=found after decide with high confidence, got %q", state.Phase)
	}
}

func TestDADTerminalStateIsIdempotent(t *testing.T) {
	t.Parallel()

	state, d1 := runDADLoop(t, heavyEvidence())
	// Calling Step again on a terminal state should return the same state.
	inv := dad.Investigator{}
	state2, d2 := inv.Step(state, nil)

	if state2.Phase != state.Phase {
		t.Errorf("Phase changed after terminal step: %q → %q", state.Phase, state2.Phase)
	}
	if d2.Terminal != d1.Terminal {
		t.Errorf("Terminal changed after terminal step: %v → %v", d1.Terminal, d2.Terminal)
	}
}

func TestDADFindingContainsEvidenceIDs(t *testing.T) {
	t.Parallel()

	_, decision := runDADLoop(t, heavyEvidence())
	// The terminal finding should mention at least one evidence ID.
	if !strings.Contains(decision.Finding, "ev-") {
		t.Errorf("expected terminal Finding to reference evidence IDs, got %q", decision.Finding)
	}
}

func TestDADConfidenceGTE(t *testing.T) {
	t.Parallel()

	ordered := []dad.Confidence{
		dad.ConfidenceNone,
		dad.ConfidenceLow,
		dad.ConfidenceMedium,
		dad.ConfidenceHigh,
	}
	for i := 0; i < len(ordered); i++ {
		if !dad.ConfidenceGTE(ordered[i], ordered[i]) {
			t.Errorf("ConfidenceGTE(%q, %q) should be true (reflexive)", ordered[i], ordered[i])
		}
		for j := i + 1; j < len(ordered); j++ {
			if dad.ConfidenceGTE(ordered[i], ordered[j]) {
				t.Errorf("ConfidenceGTE(%q, %q) should be false", ordered[i], ordered[j])
			}
			if !dad.ConfidenceGTE(ordered[j], ordered[i]) {
				t.Errorf("ConfidenceGTE(%q, %q) should be true", ordered[j], ordered[i])
			}
		}
	}
}

func TestDADMultipleRoundsOfEvidence(t *testing.T) {
	t.Parallel()

	// Feed evidence in three batches across multiple decompose rounds.
	// Each batch has weight 1 (total 3 < 5 for low) so we need many rounds.
	batch1 := []dad.EvidenceItem{{ID: "b1-e1", Content: "batch 1", Weight: 2}}
	batch2 := []dad.EvidenceItem{{ID: "b2-e1", Content: "batch 2", Weight: 8}}
	batch3 := []dad.EvidenceItem{{ID: "b3-e1", Content: "batch 3", Weight: 20}}

	inv := dad.Investigator{}
	state := dad.State{}
	var decision dad.Decision

	batches := [][]dad.EvidenceItem{batch1, batch2, batch3}
	batchIdx := 0

	for step := 0; step < 50; step++ {
		var ev []dad.EvidenceItem
		if batchIdx < len(batches) && state.Phase == dad.PhaseDecompose || state.Phase == "" {
			if batchIdx < len(batches) {
				ev = batches[batchIdx]
				batchIdx++
			}
		}
		state, decision = inv.Step(state, ev)
		if decision.Terminal {
			break
		}
	}

	if !decision.Terminal {
		t.Fatal("expected investigation to terminate")
	}
}

func TestDADZeroWeightTreatedAsOne(t *testing.T) {
	t.Parallel()

	// A single zero-weight item should be treated as weight=1.
	evidence := []dad.EvidenceItem{
		{ID: "zero-w", Content: "zero weight item", Weight: 0},
	}
	_, decision := runDADLoop(t, evidence)
	if !decision.Terminal {
		t.Error("expected terminal decision for zero-weight evidence (treated as 1)")
	}
}
