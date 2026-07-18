// Package confidencescoring — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidScore_Zero:
//     0 violates >=1 & <=5 — ConfidenceScore must be 1–5
//
//   invalidScore_Six:
//     6 violates >=1 & <=5 — ConfidenceScore must be 1–5
//
//   invalidInput_NegativeCount:
//     Count: -1 violates >=0 — Count must be non-negative
//
//   invalidInput_BadRatio:
//     AgreementRatio: 1.5 violates <=1.0 — AgreementRatio must be 0..1
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package confidencescoring

// // invalidScore_Zero — 0 is below the minimum 1
// invalidScore_Zero: #ConfidenceScore & 0

// // invalidScore_Six — 6 exceeds the maximum 5
// invalidScore_Six: #ConfidenceScore & 6

// // invalidInput_NegativeCount — Count must be >= 0
// invalidInput_NegativeCount: #EvidenceInput & {
// 	Count:          -1
// 	AgreementRatio: 0.5
// }

// // invalidInput_BadRatio — AgreementRatio must be <= 1.0
// invalidInput_BadRatio: #EvidenceInput & {
// 	Count:          3
// 	AgreementRatio: 1.5
// }
