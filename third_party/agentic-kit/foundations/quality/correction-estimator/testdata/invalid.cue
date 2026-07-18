// Package correctionestimator — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidStats_NegativeCritical:
//     Critical: -1 violates int & >=0 — counts must be non-negative
//
//   invalidEstimate_ScoreAboveOne:
//     Score: 1.5 violates number & >=0 & <=1 — Score must be in [0,1]
//
//   invalidEstimate_BadBand:
//     Band: "extreme" violates #Band enum
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package correctionestimator

// // invalidStats_NegativeCritical — counts must be non-negative
// invalidStats_NegativeCritical: #FindingStats & {
// 	Critical:      -1
// 	Major:         0
// 	Minor:         0
// 	Informational: 0
// }

// // invalidEstimate_ScoreAboveOne — Score must be in [0,1]
// invalidEstimate_ScoreAboveOne: #Estimate & {
// 	Score: 1.5
// 	Band:  "high"
// }

// // invalidEstimate_BadBand — "extreme" is not a valid #Band
// invalidEstimate_BadBand: #Estimate & {
// 	Score: 0.9
// 	Band:  "extreme"
// }
