// Package integrityevaluator — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidViolation_EmptyRef:
//     Ref: "" violates string & !="" — Ref must not be empty
//
//   invalidViolation_EmptyDetail:
//     Detail: "" violates string & !="" — Detail must not be empty
//
//   invalidViolation_BadKind:
//     Kind: "wrong_kind" violates #ViolationKind enum
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package integrityevaluator

// // invalidViolation_EmptyRef — Ref must not be empty
// invalidViolation_EmptyRef: #Violation & {
// 	Kind:   "fabricated_ref"
// 	Ref:    ""
// 	Detail: "some detail"
// }

// // invalidViolation_EmptyDetail — Detail must not be empty
// invalidViolation_EmptyDetail: #Violation & {
// 	Kind:   "fabricated_ref"
// 	Ref:    "[1]"
// 	Detail: ""
// }

// // invalidViolation_BadKind — "wrong_kind" is not a valid #ViolationKind
// invalidViolation_BadKind: #Violation & {
// 	Kind:   "wrong_kind"
// 	Ref:    "[1]"
// 	Detail: "some detail"
// }
