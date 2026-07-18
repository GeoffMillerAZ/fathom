// Package tasktree — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidNode_EmptyID:
//     ID: "" violates string & !="" — ID must not be empty
//
//   invalidNode_EmptyTitle:
//     Title: "" violates string & !="" — title must not be empty
//
//   invalidNode_BadStatus:
//     Status: "cancelled" violates #Status enum — must be one of pending/running/done/failed
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package tasktree

// // invalidNode_EmptyID — ID must not be empty
// invalidNode_EmptyID: #TaskNode & {
// 	ID:       ""
// 	ParentID: ""
// 	Title:    "some task"
// 	Status:   "pending"
// }

// // invalidNode_EmptyTitle — Title must not be empty
// invalidNode_EmptyTitle: #TaskNode & {
// 	ID:       "task-001"
// 	ParentID: ""
// 	Title:    ""
// 	Status:   "pending"
// }

// // invalidNode_BadStatus — "cancelled" is not a valid #Status
// invalidNode_BadStatus: #TaskNode & {
// 	ID:       "task-002"
// 	ParentID: ""
// 	Title:    "some task"
// 	Status:   "cancelled"
// }
