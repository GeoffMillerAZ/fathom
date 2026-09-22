// Package decision — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid CUE,
// entirely commented out). It documents the constraints enforced by
// schema.cue. Kind-conditional and cross-field enforcement (2..255 unique
// choice options, score Max>Min, unique question IDs within a Request) is
// Go-level, in Validate — see validate.go and validate_test.go.
//
// To verify the schema rejects a binding, temporarily uncomment it and run:
//
//	cue vet ./testdata/   (expect errors for that binding)
//
// The following errors would be produced by `cue vet ./testdata/`:
//
//	_badKind: "maybe" is not a valid #Kind.
//	_badQuestionEmptyID: "" violates string & !="" (ID must be non-empty).
//	_badOptionEmptyName: "" violates string & !="" (Name must be non-empty).
//	_badRequestEmptyState: "" violates string & !="" (State must be non-empty).
package decision

// // _badKind: "maybe" is not one of the recognised #Kind values.
// _badKind: #Kind & "maybe"

// // _badQuestionEmptyID: ID must not be empty.
// _badQuestionEmptyID: #Question & {
// 	ID:           ""
// 	Kind:         "noul"
// 	Instructions: "some statement"
// 	Options: []
// 	Min:            0
// 	Max:            0
// 	Rubric:         ""
// 	TrueCriteria:   ""
// 	FalseCriteria:  ""
// }

// // _badOptionEmptyName: Option.Name must not be empty.
// _badOptionEmptyName: #Option & {
// 	Name:     ""
// 	Criteria: "some criteria"
// }

// // _badRequestEmptyState: Request.State must not be empty.
// _badRequestEmptyState: #Request & {
// 	State:     ""
// 	Questions: []
// }
