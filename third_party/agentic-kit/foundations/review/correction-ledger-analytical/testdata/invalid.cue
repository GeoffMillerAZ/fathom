// Package correctionledgeranalytical — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidEvent_BadKind:
//     Kind: "audit" violates #AnalyticalEventKind enum — must be "correction" or "dismissal"
//
//   invalidEvent_EmptyID:
//     ID: "" violates string & !="" — ID must not be empty
//
//   invalidEvent_EmptyTarget:
//     Target: "" violates string & !="" — target must not be empty
//
//   invalidEvent_EmptyAt:
//     At: "" violates string & !="" — At must not be empty
//
//   invalidEvent_ZeroVersion:
//     Version: 0 violates >=1 & int — version must be at least 1
//
//   invalidEvent_NegativeVersion:
//     Version: -1 violates >=1 & int — version must be at least 1
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package correctionledgeranalytical

// // invalidEvent_BadKind — "audit" is not a valid #AnalyticalEventKind
// invalidEvent_BadKind: #AnalyticalEvent & {
// 	ID:      "audit:finding-999:1"
// 	Kind:    "audit"
// 	Target:  "report/section/costs.q1.v1"
// 	At:      "2024-11-01T10:00:00Z"
// 	Version: 1
// }

// // invalidEvent_EmptyID — ID must not be empty
// invalidEvent_EmptyID: #AnalyticalEvent & {
// 	ID:      ""
// 	Kind:    "correction"
// 	Target:  "report/section/costs.q1.v1"
// 	At:      "2024-11-01T10:00:00Z"
// 	Version: 1
// }

// // invalidEvent_EmptyTarget — Target must not be empty
// invalidEvent_EmptyTarget: #AnalyticalEvent & {
// 	ID:      "correction:finding-001:1"
// 	Kind:    "correction"
// 	Target:  ""
// 	At:      "2024-11-01T10:00:00Z"
// 	Version: 1
// }

// // invalidEvent_EmptyAt — At must not be empty
// invalidEvent_EmptyAt: #AnalyticalEvent & {
// 	ID:      "correction:finding-001:1"
// 	Kind:    "correction"
// 	Target:  "report/section/costs.q1.v1"
// 	At:      ""
// 	Version: 1
// }

// // invalidEvent_ZeroVersion — Version 0 is out of range [1, ∞)
// invalidEvent_ZeroVersion: #AnalyticalEvent & {
// 	ID:      "correction:finding-001:0"
// 	Kind:    "correction"
// 	Target:  "report/section/costs.q1.v1"
// 	At:      "2024-11-01T10:00:00Z"
// 	Version: 0
// }

// // invalidEvent_NegativeVersion — Version -1 is out of range [1, ∞)
// invalidEvent_NegativeVersion: #AnalyticalEvent & {
// 	ID:      "correction:finding-001:-1"
// 	Kind:    "correction"
// 	Target:  "report/section/costs.q1.v1"
// 	At:      "2024-11-01T10:00:00Z"
// 	Version: -1
// }
