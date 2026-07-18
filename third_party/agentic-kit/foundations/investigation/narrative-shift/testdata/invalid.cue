// invalid.cue — demonstrates #NarrativeShift values that VIOLATE the schema.
// This file is intentionally NOT included in the `cue vet` path (the valid.cue
// sibling is the one vetted in CI). It is kept here as a documentation
// artifact so reviewers understand what the schema rejects.
package narrativeshift

// _badEmptyDeclarant: empty Declarant violates the _nonEmpty constraint.
// _badEmptyDeclarant: #NarrativeShift & {
// 	Declarant:     ""
// 	PropositionID: "prop:foo"
// 	FromID:        "assert:001"
// 	ToID:          "assert:002"
// 	Kind:          "content_change"
// 	GapDays:       1
// }

// _badEmptyPropositionID: empty PropositionID violates the _nonEmpty constraint.
// _badEmptyPropositionID: #NarrativeShift & {
// 	Declarant:     "entity:alice"
// 	PropositionID: ""
// 	FromID:        "assert:001"
// 	ToID:          "assert:002"
// 	Kind:          "stance_change"
// 	GapDays:       0
// }

// _badUnknownKind: "fabricated" is not a member of #ShiftKind.
// _badUnknownKind: #NarrativeShift & {
// 	Declarant:     "entity:alice"
// 	PropositionID: "prop:foo"
// 	FromID:        "assert:001"
// 	ToID:          "assert:002"
// 	Kind:          "fabricated"
// 	GapDays:       5
// }

// _badNegativeGap: GapDays < 0 violates the >=0 constraint.
// _badNegativeGap: #NarrativeShift & {
// 	Declarant:     "entity:alice"
// 	PropositionID: "prop:foo"
// 	FromID:        "assert:001"
// 	ToID:          "assert:002"
// 	Kind:          "recantation"
// 	GapDays:       -1
// }

// _badEmptyFromID: empty FromID violates the _nonEmpty constraint.
// _badEmptyFromID: #NarrativeShift & {
// 	Declarant:     "entity:alice"
// 	PropositionID: "prop:foo"
// 	FromID:        ""
// 	ToID:          "assert:002"
// 	Kind:          "content_change"
// 	GapDays:       0
// }
