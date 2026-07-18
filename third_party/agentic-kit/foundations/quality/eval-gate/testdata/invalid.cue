// Package evalgate — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidCheck_EmptyName:
//     Name: "" violates string & !="" — Name must not be empty
//
//   invalidCheck_ZeroWeight:
//     Weight: 0 violates number & >0 — Weight must be strictly positive
//
//   invalidCheck_NegativeWeight:
//     Weight: -1 violates number & >0 — Weight must be strictly positive
//
//   invalidResult_ScoreAboveOne:
//     Score: 1.5 violates number & >=0 & <=1 — Score must be in [0,1]
//
//   invalidResult_ScoreNegative:
//     Score: -0.1 violates number & >=0 & <=1 — Score must be in [0,1]
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package evalgate

// // invalidCheck_EmptyName — Name must not be empty
// invalidCheck_EmptyName: #Check & {
// 	Name:   ""
// 	Passed: true
// 	Weight: 1.0
// }

// // invalidCheck_ZeroWeight — Weight must be strictly positive
// invalidCheck_ZeroWeight: #Check & {
// 	Name:   "some-check"
// 	Passed: true
// 	Weight: 0
// }

// // invalidCheck_NegativeWeight — Weight must be strictly positive
// invalidCheck_NegativeWeight: #Check & {
// 	Name:   "some-check"
// 	Passed: false
// 	Weight: -1
// }

// // invalidResult_ScoreAboveOne — Score must be in [0,1]
// invalidResult_ScoreAboveOne: #Result & {
// 	Passed:   true
// 	Score:    1.5
// 	Failures: []
// }

// // invalidResult_ScoreNegative — Score must be in [0,1]
// invalidResult_ScoreNegative: #Result & {
// 	Passed:   false
// 	Score:    -0.1
// 	Failures: []
// }
