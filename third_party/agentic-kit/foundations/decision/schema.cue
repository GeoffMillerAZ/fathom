// Package decision defines the CUE-first contracts for the fast-decision
// port: the shape of a Question a caller may ask and the Request that
// bundles State with the Questions to ask about it. Go types in types.go
// mirror these definitions; any drift is a bug.
//
// Kind-conditional requirements CUE cannot express cleanly across one open
// struct (Options required and bounded to 2..255 only for "choice", Max>Min
// only for "score", ID/Kind uniqueness across a Request) are enforced in
// Go's Validate — see that function's doc comment for the full contract.
package decision

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #Kind is the closed set of fast-decision question shapes, mirroring
// TypeSafe AI's System One primitives (Choice/Score/Noul) — see doc.go.
#Kind: "choice" | "score" | "noul"

// #Option is one named alternative for a #Kind "choice" Question.
// Criteria is optional free text describing when this option applies; it
// may be empty.
#Option: {
	Name:     _nonEmpty
	Criteria: string
}

// #Question is one typed fast-decision question. Kind selects which of the
// remaining fields are meaningful:
//   - "choice": Options (2..255 entries, enforced in Go — see Validate)
//   - "score":  Min, Max (Max > Min, enforced in Go), Rubric optional
//   - "noul":   TrueCriteria/FalseCriteria optional
//
// Fields not meaningful for a given Kind are simply left at their zero
// value; CUE keeps them open here rather than modeling three disjoint
// struct shapes, matching Go's single-struct Question type field for field.
#Question: {
	ID:            _nonEmpty
	Kind:          #Kind
	Instructions:  string
	Options: [...#Option]
	Min:           number
	Max:           number
	Rubric:        string
	TrueCriteria:  string
	FalseCriteria: string
}

// #Request bundles a caller's State (the blob of state being asked about,
// rendered as text) with the Questions to ask about it.
#Request: {
	State: _nonEmpty
	Questions: [...#Question]
}
