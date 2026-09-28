package support

// state.go — ComputeState (SM-6, design §3): the pure function that derives
// one cell's closed-vocabulary state from registry, target, reading and
// plan-verdict facts only. `regressed` is NOT computed here — it needs
// ledger history, which Project (SM-9) overlays afterward.

// CellInput is ComputeState's whole input for one (entry, capability): no
// hidden clock, no file read, no ledger — just the facts design §3's table
// switches on.
type CellInput struct {
	Capability string

	// ApplicableToKind is false for a capability the kind does not carry
	// (e.g. L4 for a tool format) — the cell is n/a regardless of anything
	// else.
	ApplicableToKind bool

	// TargetDeclared is true when a target/queue row names this
	// capability for (kind, id) at all.
	TargetDeclared bool
	// Target is the declared target when TargetDeclared is true: either a
	// Level, or ProvenOnly (the target value was literal `true`).
	Target CapabilityTarget

	// Registered is true when a registry entry exists for (kind, id).
	Registered bool

	// LevelMet/Grade are the reading's own computed result; both zero
	// ("") when no committed reading exists for this (kind, id,
	// capability). A boolean (ProvenOnly) capability that has been proven
	// carries a Grade with no LevelMet.
	LevelMet Level
	Grade    Grade
	// ReadingPath is the reading's own path (design §3.2's evidence link),
	// copied into Cell.Reading whenever a level or grade is claimed.
	ReadingPath string

	// PlanRow/PlanVerdict describe the owning plan row, when the
	// capability is tied to one; PlanVerdict is one of "proven",
	// "claimed-unproven", "open", or "" (no owning row — the capability
	// verifies on registration/reading alone, design §3's "or the
	// capability has no row").
	PlanRow     string
	PlanVerdict string

	// Disposition is "refused" or "deferred" when the owning plan row is
	// `[-]` with that disposition (design §3's refused/deferred row);
	// "" otherwise. Trigger is the deferred row's revisit condition, for
	// the label only.
	Disposition string
	Trigger     string
}

// ComputeState derives one Cell from in. It never reports a level above
// what in.LevelMet/in.Grade actually proved (SM-6's own anchor): a below-
// target reading always carries its OWN met level, never the target's; a
// tolerated or declared cell never carries a level at all.
func ComputeState(in CellInput) Cell {
	cell := Cell{Capability: in.Capability}

	if !in.ApplicableToKind {
		cell.State = StateNA
		return cell
	}

	switch in.Disposition {
	case "refused":
		cell.State = StateRefused
		cell.PlanRow = in.PlanRow
		setTarget(&cell, in.Target)
		return cell
	case "deferred":
		cell.State = StateDeferred
		cell.PlanRow = in.PlanRow
		setTarget(&cell, in.Target)
		return cell
	}

	hasReading := in.LevelMet != LevelNone || in.Grade != GradeNone

	if !in.Registered && !hasReading {
		if in.TargetDeclared {
			cell.State = StateDeclared
			cell.PlanRow = in.PlanRow
			setTarget(&cell, in.Target)
		}
		// Neither registered, targeted nor read: nothing to project — the
		// caller (Project) never builds a cell for this shape; ComputeState
		// still returns a zero-value declared/absent cell rather than
		// guessing a state.
		return cell
	}

	if in.Registered && !hasReading {
		cell.State = StateTolerated
		setTarget(&cell, in.Target)
		return cell
	}

	// hasReading: proven-excellent, proven-good or below-target, in that
	// priority — never a level above in.LevelMet/in.Grade.
	setTarget(&cell, in.Target)
	cell.LevelMet = in.LevelMet
	cell.Reading = in.ReadingPath
	cell.PlanRow = in.PlanRow
	cell.Verdict = planVerdict(in.PlanVerdict)

	switch {
	case in.Target.Level != "" && levelRank(in.LevelMet) < levelRank(in.Target.Level):
		cell.State = StateBelowTarget
	case in.Grade == GradeExcellent && rowVerifies(in.PlanVerdict):
		cell.State = StateProvenExcellent
	default:
		cell.State = StateProvenGood
	}
	return cell
}

// rowVerifies reports whether the owning plan row's verdict lets a reading
// count as fully proven: an explicit "proven" verdict, or no owning row at
// all (design §3: "or the capability has no row").
func rowVerifies(verdict string) bool {
	return verdict == "" || verdict == "proven"
}

// planVerdict maps ComputeState's plain verdict strings onto the schema's
// closed Verdict vocabulary.
func planVerdict(v string) Verdict {
	switch v {
	case "proven":
		return VerdictProven
	case "claimed-unproven":
		return VerdictClaimedUnproven
	case "open":
		return VerdictOpen
	default:
		return VerdictNone
	}
}

// setTarget renders a CapabilityTarget into Cell.Target's string form: a
// Level string, or the JSON boolean `true` for ProvenOnly (never the
// string "true" — #Cell.target is `#Level | true`, and CUE's `true` only
// unifies against a JSON boolean). Left unset when neither is declared.
func setTarget(cell *Cell, t CapabilityTarget) {
	switch {
	case t.Level != "":
		cell.Target = string(t.Level)
	case t.ProvenOnly:
		cell.Target = true
	}
}
