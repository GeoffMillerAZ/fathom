// Package swarmorchestrator — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidTask_EmptyID:
//     ID: "" violates string & !="" — ID must not be empty
//
//   invalidTask_EmptyLabel:
//     Label: "" violates string & !="" — Label must not be empty
//
//   invalidTask_BadStatus:
//     Status: "queued" violates #TaskStatus enum — must be pending/assigned/done/failed
//
//   invalidWave_NegativeIndex:
//     Index: -1 violates >=0 & int — Index must be non-negative
//
//   invalidPlan_ZeroConcurrency:
//     MaxConcurrency: 0 violates >=1 & int — must be at least 1
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package swarmorchestrator

// // invalidTask_EmptyID — ID must not be empty
// invalidTask_EmptyID: #Task & {
// 	ID:     ""
// 	Label:  "some task"
// 	Status: "pending"
// }

// // invalidTask_EmptyLabel — Label must not be empty
// invalidTask_EmptyLabel: #Task & {
// 	ID:     "task-001"
// 	Label:  ""
// 	Status: "pending"
// }

// // invalidTask_BadStatus — "queued" is not a valid #TaskStatus
// invalidTask_BadStatus: #Task & {
// 	ID:     "task-002"
// 	Label:  "some task"
// 	Status: "queued"
// }

// // invalidWave_NegativeIndex — Index must be >= 0
// invalidWave_NegativeIndex: #Wave & {
// 	Index:   -1
// 	TaskIDs: ["task-001"]
// }

// // invalidPlan_ZeroConcurrency — MaxConcurrency must be >= 1
// invalidPlan_ZeroConcurrency: #Plan & {
// 	MaxConcurrency: 0
// 	Waves:          []
// }
