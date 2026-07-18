// Package implementationqueue — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidItem_EmptyKey:
//     Key: "" violates string & !="" — Key must not be empty
//
//   invalidItem_EmptyPayload:
//     Payload: "" violates string & !="" — Payload must not be empty
//
//   invalidItem_BadStatus:
//     Status: "cancelled" violates #ItemStatus enum — must be one of pending/in_progress/done/failed
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package implementationqueue

// // invalidItem_EmptyKey — Key must not be empty
// invalidItem_EmptyKey: #Item & {
// 	Key:     ""
// 	Payload: "some work"
// 	Status:  "pending"
// }

// // invalidItem_EmptyPayload — Payload must not be empty
// invalidItem_EmptyPayload: #Item & {
// 	Key:     "impl-001"
// 	Payload: ""
// 	Status:  "pending"
// }

// // invalidItem_BadStatus — "cancelled" is not a valid #ItemStatus
// invalidItem_BadStatus: #Item & {
// 	Key:     "impl-002"
// 	Payload: "some work"
// 	Status:  "cancelled"
// }
