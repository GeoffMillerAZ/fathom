// Package workerheartbeat — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidHeartbeat_EmptyWorkerID:
//     WorkerID: "" violates string & !="" — WorkerID must not be empty
//
//   invalidHeartbeat_EmptyAt:
//     At: "" violates string & !="" — At must not be empty
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package workerheartbeat

// // invalidHeartbeat_EmptyWorkerID — WorkerID must not be empty
// invalidHeartbeat_EmptyWorkerID: #Heartbeat & {
// 	WorkerID: ""
// 	At:       "2024-11-01T09:00:00Z"
// }

// // invalidHeartbeat_EmptyAt — At must not be empty
// invalidHeartbeat_EmptyAt: #Heartbeat & {
// 	WorkerID: "worker-1"
// 	At:       ""
// }
