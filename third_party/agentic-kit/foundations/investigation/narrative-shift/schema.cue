// Package narrativeshift defines the CUE-first contracts for the narrative
// shift detector. Go types in types.go mirror these definitions; any drift is
// a bug caught by `cue vet`.
package narrativeshift

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #ShiftKind classifies the nature of a declarant's story change between two
// consecutive assertions on the same proposition.
//
// Priority rule (applied in order):
//   1. recantation   — the later assertion's Stance is a retraction.
//   2. stance_change — the later assertion carries a different Stance.
//   3. content_change — Stance is the same but Content differs.
//   (escalation is reserved for future use and currently unemitted.)
//
// Go mirror: ShiftKind in types.go.
#ShiftKind: "recantation" | "stance_change" | "content_change" | "escalation"

// #NarrativeShift records a single detected story change for one declarant on
// one proposition. From precedes To chronologically (From.StatedAt <=
// To.StatedAt).
//
// Go mirror: NarrativeShift in types.go.
#NarrativeShift: {
	// Declarant is the EntityID of the real-world person whose story changed.
	Declarant: _nonEmpty

	// PropositionID is the proposition both assertions address.
	PropositionID: _nonEmpty

	// From is the earlier assertion's identifier (source record reference).
	FromID: _nonEmpty

	// ToID is the later assertion's identifier.
	ToID: _nonEmpty

	// Kind classifies what type of story change occurred.
	Kind: #ShiftKind

	// GapDays is the number of whole calendar days between From.StatedAt and
	// To.StatedAt. Must be >= 0.
	GapDays: >=0
}
