// Package resultaggregator — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidResult_EmptyKey:
//     Key: "" violates string & !="" — Key must not be empty
//
//   invalidCombined_NegativeCount:
//     Count: -1 violates >=0 — Count must be non-negative
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package resultaggregator

// // invalidResult_EmptyKey — Key must not be empty
// invalidResult_EmptyKey: #Result & {
// 	Key:   ""
// 	Value: "some value"
// }

// // invalidCombined_NegativeCount — Count must be >= 0
// invalidCombined_NegativeCount: #Combined & {
// 	Items: []
// 	Count: -1
// }
