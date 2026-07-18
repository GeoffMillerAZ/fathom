// Package qaharness — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidItem_EmptyID:
//     ID: "" violates string & !="" — ID must not be empty
//
//   invalidScoreRecord_ScoreAboveOne:
//     Score: 1.1 violates number & >=0 & <=1 — Score must be in [0,1]
//
//   invalidScoreRecord_NegativeScore:
//     Score: -0.1 violates number & >=0 & <=1 — Score must be in [0,1]
//
//   invalidScoreRecord_EmptyItemID:
//     ItemID: "" violates string & !="" — ItemID must not be empty
//
//   invalidCriterion_ZeroWeight:
//     Weight: 0 violates number & >0 — Weight must be strictly positive
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package qaharness

// // invalidItem_EmptyID — ID must not be empty
// invalidItem_EmptyID: #Item & {
// 	ID:       ""
// 	Output:   "some output"
// 	Metadata: {}
// }

// // invalidScoreRecord_ScoreAboveOne — Score must be in [0,1]
// invalidScoreRecord_ScoreAboveOne: #ScoreRecord & {
// 	ItemID: "item-001"
// 	Score:  1.1
// }

// // invalidScoreRecord_NegativeScore — Score must be in [0,1]
// invalidScoreRecord_NegativeScore: #ScoreRecord & {
// 	ItemID: "item-001"
// 	Score:  -0.1
// }

// // invalidScoreRecord_EmptyItemID — ItemID must not be empty
// invalidScoreRecord_EmptyItemID: #ScoreRecord & {
// 	ItemID: ""
// 	Score:  0.5
// }

// // invalidCriterion_ZeroWeight — Weight must be strictly positive
// invalidCriterion_ZeroWeight: #RubricCriterion & {
// 	Name:   "some-criterion"
// 	Weight: 0
// }
