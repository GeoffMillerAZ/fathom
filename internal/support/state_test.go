package support

import "testing"

// TestComputeStateNeverAboveReading proves SM-6: ComputeState derives the
// closed vocabulary from registry/target/reading/plan-verdict facts only,
// and never reports a cell whose level is above what the reading actually
// proved.
func TestComputeStateNeverAboveReading(t *testing.T) {
	t.Run("capability not applicable to kind is n/a, whatever else is set", func(t *testing.T) {
		cell := ComputeState(CellInput{
			Capability:       "L4",
			ApplicableToKind: false,
			Registered:       true,
			LevelMet:         L4,
			Grade:            GradeExcellent,
		})
		if cell.State != StateNA {
			t.Fatalf("state = %q, want %q", cell.State, StateNA)
		}
		if cell.LevelMet != "" {
			t.Errorf("n/a cell claims level_met=%q", cell.LevelMet)
		}
	})

	t.Run("target declared, no registry entry yet: declared, no level", func(t *testing.T) {
		cell := ComputeState(CellInput{
			Capability:       "mcp-tools",
			ApplicableToKind: true,
			TargetDeclared:   true,
			Target:           CapabilityTarget{ProvenOnly: true},
			PlanRow:          "SM-13",
		})
		if cell.State != StateDeclared {
			t.Fatalf("state = %q, want %q", cell.State, StateDeclared)
		}
		if cell.LevelMet != "" {
			t.Errorf("declared cell claims level_met=%q", cell.LevelMet)
		}
		if cell.Target == nil || *cell.Target != "true" {
			t.Errorf("declared cell target = %v, want true", cell.Target)
		}
	})

	t.Run("registered, no reading: tolerated, no level (I54)", func(t *testing.T) {
		cell := ComputeState(CellInput{
			Capability:       "L1",
			ApplicableToKind: true,
			Registered:       true,
			TargetDeclared:   true,
			Target:           CapabilityTarget{Level: L4},
		})
		if cell.State != StateTolerated {
			t.Fatalf("state = %q, want %q", cell.State, StateTolerated)
		}
		if cell.LevelMet != "" {
			t.Errorf("tolerated cell claims level_met=%q", cell.LevelMet)
		}
	})

	t.Run("reading below the declared target: below-target, at the READING's own level", func(t *testing.T) {
		cell := ComputeState(CellInput{
			Capability:       "L1",
			ApplicableToKind: true,
			Registered:       true,
			TargetDeclared:   true,
			Target:           CapabilityTarget{Level: L4},
			LevelMet:         L1,
			Grade:            GradeGood,
			ReadingPath:      "docs/bench/readings/lang/typescript.json",
		})
		if cell.State != StateBelowTarget {
			t.Fatalf("state = %q, want %q", cell.State, StateBelowTarget)
		}
		if cell.LevelMet != L1 {
			t.Fatalf("below-target cell reports level_met=%q, want the reading's own L1 (never the L4 target)", cell.LevelMet)
		}
		if cell.Reading == "" {
			t.Error("below-target cell carries no reading path")
		}
	})

	t.Run("reading meets target at good grade with an open owning row: proven-good, not excellent", func(t *testing.T) {
		cell := ComputeState(CellInput{
			Capability:       "L4",
			ApplicableToKind: true,
			Registered:       true,
			TargetDeclared:   true,
			Target:           CapabilityTarget{Level: L4},
			LevelMet:         L4,
			Grade:            GradeExcellent,
			PlanRow:          "LB-Q-typescript",
			PlanVerdict:      "claimed-unproven",
			ReadingPath:      "docs/bench/readings/lang/typescript.json",
		})
		if cell.State != StateProvenGood {
			t.Fatalf("state = %q, want %q (an unproven owning row caps excellence)", cell.State, StateProvenGood)
		}
		if cell.LevelMet != L4 {
			t.Errorf("level_met = %q, want L4 (the reading's own level)", cell.LevelMet)
		}
	})

	t.Run("reading meets target at excellent grade, row proven: proven-excellent", func(t *testing.T) {
		cell := ComputeState(CellInput{
			Capability:       "L4",
			ApplicableToKind: true,
			Registered:       true,
			TargetDeclared:   true,
			Target:           CapabilityTarget{Level: L4},
			LevelMet:         L4,
			Grade:            GradeExcellent,
			PlanRow:          "LB-Q-typescript",
			PlanVerdict:      "proven",
			ReadingPath:      "docs/bench/readings/lang/typescript.json",
		})
		if cell.State != StateProvenExcellent {
			t.Fatalf("state = %q, want %q", cell.State, StateProvenExcellent)
		}
	})

	t.Run("reading proves a boolean (ProvenOnly) capability: proven-good/excellent, never below-target", func(t *testing.T) {
		cell := ComputeState(CellInput{
			Capability:       "mcp-tools",
			ApplicableToKind: true,
			Registered:       true,
			TargetDeclared:   true,
			Target:           CapabilityTarget{ProvenOnly: true},
			Grade:            GradeExcellent,
			ReadingPath:      "docs/bench/readings/support/client/claude-code.json",
		})
		if cell.State != StateProvenExcellent {
			t.Fatalf("state = %q, want %q (no level to fall below on a boolean target)", cell.State, StateProvenExcellent)
		}
	})

	t.Run("disposition refused/deferred wins over any reading", func(t *testing.T) {
		refused := ComputeState(CellInput{
			Capability:       "cli",
			ApplicableToKind: true,
			Registered:       true,
			Disposition:      "refused",
			PlanRow:          "SM-99",
		})
		if refused.State != StateRefused {
			t.Fatalf("state = %q, want %q", refused.State, StateRefused)
		}

		deferred := ComputeState(CellInput{
			Capability:       "cli",
			ApplicableToKind: true,
			Registered:       true,
			Disposition:      "deferred",
			PlanRow:          "SM-99",
			Trigger:          "when the ecosystem plan lands",
		})
		if deferred.State != StateDeferred {
			t.Fatalf("state = %q, want %q", deferred.State, StateDeferred)
		}
	})

	t.Run("invariant: whenever a level is reported, it is exactly the reading's own level", func(t *testing.T) {
		levels := []Level{L1, L2, L3, L4}
		for _, met := range levels {
			for _, target := range levels {
				for _, grade := range []Grade{GradeGood, GradeExcellent} {
					cell := ComputeState(CellInput{
						Capability:       "Lx",
						ApplicableToKind: true,
						Registered:       true,
						TargetDeclared:   true,
						Target:           CapabilityTarget{Level: target},
						LevelMet:         met,
						Grade:            grade,
						ReadingPath:      "reading.json",
					})
					if cell.LevelMet != met {
						t.Fatalf("met=%s target=%s grade=%s: cell.LevelMet=%s, want %s (never above/below the reading itself)", met, target, grade, cell.LevelMet, met)
					}
					if levelRank(met) < levelRank(target) && cell.State != StateBelowTarget {
						t.Fatalf("met=%s target=%s: state=%s, want below-target", met, target, cell.State)
					}
					if levelRank(met) >= levelRank(target) && cell.State == StateBelowTarget {
						t.Fatalf("met=%s target=%s: state=below-target but the reading meets the target", met, target)
					}
				}
			}
		}
	})
}
