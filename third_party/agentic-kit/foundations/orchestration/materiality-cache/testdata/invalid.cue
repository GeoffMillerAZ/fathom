// Package materialitycache — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidEntry_EmptyKey:
//     k: "" violates string & !="" — key must not be empty
//
//   invalidPutRequest_EmptyKey:
//     key: "" violates string & !="" — key must not be empty
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package materialitycache

// // invalidEntry_EmptyKey — key must not be empty
// invalidEntry_EmptyKey: #Entry & {
// 	k:           ""
// 	value:       'some bytes'
// 	stored_at:   "2024-01-01T00:00:00Z"
// 	fingerprint: ""
// }

// // invalidPutRequest_EmptyKey — put request key must not be empty
// invalidPutRequest_EmptyKey: #PutRequest & {
// 	key:         ""
// 	value:       'bytes'
// 	fingerprint: "fp"
// }
