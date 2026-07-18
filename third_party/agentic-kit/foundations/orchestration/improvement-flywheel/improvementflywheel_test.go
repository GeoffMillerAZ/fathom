package improvementflywheel_test

import (
	"testing"

	fw "github.com/geoffmilleraz/agentic-kit/foundations/orchestration/improvement-flywheel"
)

// ---- zero-value Flywheel returns nil proposals ----------------------------

func TestFlywheel_Empty_NoProposals(t *testing.T) {
	var f fw.Flywheel
	if p := f.Proposals(); p != nil {
		t.Errorf("expected nil from empty flywheel, got %v", p)
	}
}

// ---- single signal generates one proposal --------------------------------

func TestFlywheel_SingleSignal_OneProposal(t *testing.T) {
	var f fw.Flywheel
	f.Observe(fw.Signal{
		Type:       fw.PatternFalsePositive,
		Key:        "revenue/q4-concern",
		WorkflowID: "wf-001",
		Detail:     "dismissed: within variance",
	})

	props := f.Proposals()
	if len(props) != 1 {
		t.Fatalf("expected 1 proposal, got %d", len(props))
	}
	p := props[0]
	if p.Frequency != 1 {
		t.Errorf("expected Frequency 1, got %d", p.Frequency)
	}
	if p.PatternType != fw.PatternFalsePositive {
		t.Errorf("expected PatternFalsePositive, got %q", p.PatternType)
	}
	if p.ChangeTarget != fw.TargetCatalogEntry {
		t.Errorf("expected TargetCatalogEntry for false_positive, got %q", p.ChangeTarget)
	}
}

// ---- multiple signals for same key accumulate frequency ------------------

func TestFlywheel_RepeatedSignals_IncreasesFrequency(t *testing.T) {
	var f fw.Flywheel
	for i := 0; i < 5; i++ {
		f.Observe(fw.Signal{
			Type:       fw.PatternFalsePositive,
			Key:        "concern/type-a",
			WorkflowID: "wf-001",
		})
	}
	props := f.Proposals()
	if len(props) != 1 {
		t.Fatalf("expected 1 proposal, got %d", len(props))
	}
	if props[0].Frequency != 5 {
		t.Errorf("expected Frequency 5, got %d", props[0].Frequency)
	}
}

// ---- different keys produce separate proposals ---------------------------

func TestFlywheel_DifferentKeys_SeparateProposals(t *testing.T) {
	var f fw.Flywheel
	f.Observe(fw.Signal{Type: fw.PatternFalsePositive, Key: "key-a", WorkflowID: "wf-1"})
	f.Observe(fw.Signal{Type: fw.PatternFalsePositive, Key: "key-b", WorkflowID: "wf-2"})

	props := f.Proposals()
	if len(props) != 2 {
		t.Fatalf("expected 2 proposals, got %d", len(props))
	}
}

// ---- different types for same key produce separate proposals -------------

func TestFlywheel_SameKeyDifferentType_SeparateProposals(t *testing.T) {
	var f fw.Flywheel
	f.Observe(fw.Signal{Type: fw.PatternFalsePositive, Key: "shared-key"})
	f.Observe(fw.Signal{Type: fw.PatternMissedClaim, Key: "shared-key"})

	props := f.Proposals()
	if len(props) != 2 {
		t.Fatalf("expected 2 proposals, got %d", len(props))
	}
}

// ---- proposals are sorted by frequency (descending) ----------------------

func TestFlywheel_Proposals_SortedByFrequency(t *testing.T) {
	var f fw.Flywheel

	// key-low seen once, key-high seen three times.
	f.Observe(fw.Signal{Type: fw.PatternMissedClaim, Key: "key-low"})
	for i := 0; i < 3; i++ {
		f.Observe(fw.Signal{Type: fw.PatternMissedClaim, Key: "key-high"})
	}
	f.Observe(fw.Signal{Type: fw.PatternMissedClaim, Key: "key-mid"})
	f.Observe(fw.Signal{Type: fw.PatternMissedClaim, Key: "key-mid"})

	props := f.Proposals()
	if len(props) != 3 {
		t.Fatalf("expected 3 proposals, got %d", len(props))
	}
	if props[0].Frequency < props[1].Frequency || props[1].Frequency < props[2].Frequency {
		t.Errorf("proposals not sorted descending by frequency: %v %v %v",
			props[0].Frequency, props[1].Frequency, props[2].Frequency)
	}
	if props[0].Frequency != 3 {
		t.Errorf("expected top proposal Frequency=3, got %d", props[0].Frequency)
	}
}

// ---- Proposals() is deterministic ----------------------------------------

func TestFlywheel_Proposals_Deterministic(t *testing.T) {
	var f fw.Flywheel
	for _, k := range []string{"z", "a", "m", "b"} {
		f.Observe(fw.Signal{Type: fw.PatternFalsePositive, Key: k})
	}
	p1 := f.Proposals()
	p2 := f.Proposals()

	if len(p1) != len(p2) {
		t.Fatalf("length mismatch: %d vs %d", len(p1), len(p2))
	}
	for i := range p1 {
		if p1[i].ID != p2[i].ID || p1[i].Frequency != p2[i].Frequency {
			t.Errorf("non-deterministic at index %d: %+v vs %+v", i, p1[i], p2[i])
		}
	}
}

// ---- WorkflowIDs are deduplicated ----------------------------------------

func TestFlywheel_WorkflowIDsDeduped(t *testing.T) {
	var f fw.Flywheel
	for i := 0; i < 4; i++ {
		f.Observe(fw.Signal{
			Type:       fw.PatternFalsePositive,
			Key:        "k",
			WorkflowID: "same-wf",
		})
	}
	obs := f.Observations()
	if len(obs) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(obs))
	}
	if len(obs[0].WorkflowIDs) != 1 {
		t.Errorf("expected 1 deduplicated workflow ID, got %d", len(obs[0].WorkflowIDs))
	}
}

// ---- Examples capped at maxExamples (5) ----------------------------------

func TestFlywheel_ExamplesCapped(t *testing.T) {
	var f fw.Flywheel
	for i := 0; i < 10; i++ {
		f.Observe(fw.Signal{
			Type:   fw.PatternOperatorFriction,
			Key:    "friction-key",
			Detail: "example detail",
		})
	}
	obs := f.Observations()
	if len(obs[0].Examples) > 5 {
		t.Errorf("expected at most 5 examples, got %d", len(obs[0].Examples))
	}
}

// ---- Change target mapping -----------------------------------------------

func TestFlywheel_ChangeTargetMapping(t *testing.T) {
	cases := []struct {
		pt     fw.PatternType
		target fw.ChangeTarget
	}{
		{fw.PatternFalsePositive, fw.TargetCatalogEntry},
		{fw.PatternMissedClaim, fw.TargetPromptSection},
		{fw.PatternRepeatedCorrection, fw.TargetProsePolicy},
		{fw.PatternOperatorFriction, fw.TargetFoundationBehavior},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.pt), func(t *testing.T) {
			var f fw.Flywheel
			f.Observe(fw.Signal{Type: tc.pt, Key: "k"})
			props := f.Proposals()
			if props[0].ChangeTarget != tc.target {
				t.Errorf("expected %q, got %q", tc.target, props[0].ChangeTarget)
			}
		})
	}
}

// ---- Observations returns a copy (mutation safety) -----------------------

func TestFlywheel_ObservationsReturnsCopy(t *testing.T) {
	var f fw.Flywheel
	f.Observe(fw.Signal{Type: fw.PatternFalsePositive, Key: "k", WorkflowID: "wf-1"})

	obs := f.Observations()
	if len(obs) == 0 {
		t.Fatal("expected at least one observation")
	}
	// Mutate the returned slice and verify the flywheel's internal state is intact.
	obs[0].Frequency = 999

	obs2 := f.Observations()
	if obs2[0].Frequency == 999 {
		t.Error("Observations returned a live reference; mutating it changed internal state")
	}
}
