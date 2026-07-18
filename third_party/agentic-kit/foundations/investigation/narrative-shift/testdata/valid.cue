// valid.cue — concrete values that satisfy the #NarrativeShift and #ShiftKind
// constraints defined in schema.cue. Running `cue vet ./...` from the package
// root validates these against the schema.
package narrativeshift

// A content change: Alice said "home" in her deposition, then said "bar" in
// a later interview on the same proposition.
_shift1: #NarrativeShift & {
	Declarant:     "entity:alice"
	PropositionID: "prop:alice-location-9pm"
	FromID:        "assert:alice-depo-001"
	ToID:          "assert:alice-interview-002"
	Kind:          "content_change"
	GapDays:       30
}

// A recantation: Alice affirmed a proposition in her deposition and then
// explicitly withdrew it in a follow-up declaration.
_shift2: #NarrativeShift & {
	Declarant:     "entity:alice"
	PropositionID: "prop:contract-signed"
	FromID:        "assert:alice-depo-010"
	ToID:          "assert:alice-decl-011"
	Kind:          "recantation"
	GapDays:       7
}

// A stance change: Bob affirmed then denied the same proposition.
_shift3: #NarrativeShift & {
	Declarant:     "entity:bob"
	PropositionID: "prop:meeting-attended"
	FromID:        "assert:bob-email-001"
	ToID:          "assert:bob-depo-002"
	Kind:          "stance_change"
	GapDays:       0
}

// Same-day gap (GapDays == 0) is valid.
_shift4: #NarrativeShift & {
	Declarant:     "entity:carol"
	PropositionID: "prop:payment-received"
	FromID:        "assert:carol-am-001"
	ToID:          "assert:carol-pm-002"
	Kind:          "content_change"
	GapDays:       0
}
