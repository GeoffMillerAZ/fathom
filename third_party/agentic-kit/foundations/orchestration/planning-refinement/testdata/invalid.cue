// Package planningrefinement — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidPlan_EmptyID:
//     ID: "" violates string & !="" — ID must not be empty
//
//   invalidPlan_EmptyContent:
//     Content: "" violates string & !="" — Content must not be empty
//
//   invalidPlan_NegativeRound:
//     Round: -1 violates >=0 & int — Round must be non-negative
//
//   invalidPolicy_ZeroMaxRounds:
//     MaxRounds: 0 violates >=1 & int — MaxRounds must be at least 1
//
//   invalidSignal_Unknown:
//     "unknown" violates #Signal enum — must be improve/accept/reject
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package planningrefinement

// // invalidPlan_EmptyID — ID must not be empty
// invalidPlan_EmptyID: #Plan & {
// 	ID:      ""
// 	Content: "some plan"
// 	Round:   0
// }

// // invalidPlan_EmptyContent — Content must not be empty
// invalidPlan_EmptyContent: #Plan & {
// 	ID:      "plan-001"
// 	Content: ""
// 	Round:   0
// }

// // invalidPlan_NegativeRound — Round must be >= 0
// invalidPlan_NegativeRound: #Plan & {
// 	ID:      "plan-001"
// 	Content: "some plan"
// 	Round:   -1
// }

// // invalidPolicy_ZeroMaxRounds — MaxRounds must be >= 1
// invalidPolicy_ZeroMaxRounds: #Policy & {
// 	MaxRounds: 0
// }

// // invalidSignal_Unknown — "unknown" is not a valid #Signal
// invalidSignal_Unknown: #Signal & "unknown"
