// Package convergencedetector — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidPolicy_ZeroWindow:
//     Window: 0 violates >=1 — Window must be at least 1
//
//   invalidPolicy_ZeroThreshold:
//     Threshold: 0 violates >0 — Threshold must be greater than 0
//
//   invalidPolicy_ThresholdTooHigh:
//     Threshold: 1.5 violates <=1 — Threshold must be at most 1.0
//
//   invalidPolicy_ZeroAbandonAfter:
//     AbandonAfter: 0 violates >=1 — AbandonAfter must be at least 1
//
//   invalidSignal_EmptyValue:
//     Value: "" violates string & !="" — Value must not be empty
//
//   invalidVerdict_Unknown:
//     "unknown" violates #Verdict enum — must be converged/exploring/abandoned
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package convergencedetector

// // invalidPolicy_ZeroWindow — Window must be >= 1
// invalidPolicy_ZeroWindow: #Policy & {
// 	Window:       0
// 	Threshold:    0.8
// 	AbandonAfter: 20
// }

// // invalidPolicy_ZeroThreshold — Threshold must be > 0
// invalidPolicy_ZeroThreshold: #Policy & {
// 	Window:       5
// 	Threshold:    0
// 	AbandonAfter: 20
// }

// // invalidPolicy_ThresholdTooHigh — Threshold must be <= 1
// invalidPolicy_ThresholdTooHigh: #Policy & {
// 	Window:       5
// 	Threshold:    1.5
// 	AbandonAfter: 20
// }

// // invalidPolicy_ZeroAbandonAfter — AbandonAfter must be >= 1
// invalidPolicy_ZeroAbandonAfter: #Policy & {
// 	Window:       5
// 	Threshold:    0.8
// 	AbandonAfter: 0
// }

// // invalidSignal_EmptyValue — Value must not be empty
// invalidSignal_EmptyValue: #Signal & {
// 	Value: ""
// }

// // invalidVerdict_Unknown — "unknown" is not a valid #Verdict
// invalidVerdict_Unknown: #Verdict & "unknown"
