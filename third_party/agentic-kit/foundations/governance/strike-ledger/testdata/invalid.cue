// Package strikeledger — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidConfig_ZeroThreshold:
//     Threshold: 0 violates >=1 — threshold must be at least 1
//
//   invalidConfig_EmptyBaseBlock:
//     BaseBlock: "" violates string & !="" — BaseBlock must not be empty
//
//   invalidStrike_EmptyID:
//     ID: "" violates string & !="" — ID must not be empty
//
//   invalidStrike_EmptyReason:
//     Reason: "" violates string & !="" — Reason must not be empty
//
//   invalidSnapshotEntry_NegativeBlockCount:
//     BlockCount: -1 violates >=0 — BlockCount must be non-negative
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package strikeledger

// // invalidConfig_ZeroThreshold — Threshold 0 violates >=1
// invalidConfig_ZeroThreshold: #Config & {
// 	Threshold: 0
// 	BaseBlock: "1m"
// 	MaxBlock:  "1h"
// }

// // invalidConfig_EmptyBaseBlock — BaseBlock must not be empty
// invalidConfig_EmptyBaseBlock: #Config & {
// 	Threshold: 3
// 	BaseBlock: ""
// 	MaxBlock:  "1h"
// }

// // invalidStrike_EmptyID — ID must not be empty
// invalidStrike_EmptyID: #Strike & {
// 	ID:         ""
// 	Reason:     "spam"
// 	OccurredAt: "2024-11-01T09:00:00Z"
// }

// // invalidStrike_EmptyReason — Reason must not be empty
// invalidStrike_EmptyReason: #Strike & {
// 	ID:         "a3f7b2c1d0e4f5a6"
// 	Reason:     ""
// 	OccurredAt: "2024-11-01T09:00:00Z"
// }

// // invalidSnapshotEntry_NegativeBlockCount — BlockCount must be >= 0
// invalidSnapshotEntry_NegativeBlockCount: #SnapshotEntry & {
// 	Strikes:    []
// 	BlockCount: -1
// }
