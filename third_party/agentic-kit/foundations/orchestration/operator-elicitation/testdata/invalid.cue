// Package operatorelicitation — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidRequest_EmptyID:
//     ID: "" violates string & !="" — ID must not be empty
//
//   invalidRequest_EmptyPrompt:
//     Prompt: "" violates string & !="" — Prompt must not be empty
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package operatorelicitation

// // invalidRequest_EmptyID — ID must not be empty
// invalidRequest_EmptyID: #Request & {
// 	ID:       ""
// 	Prompt:   "some question"
// 	Answer:   ""
// 	Resolved: false
// }

// // invalidRequest_EmptyPrompt — Prompt must not be empty
// invalidRequest_EmptyPrompt: #Request & {
// 	ID:       "req-001"
// 	Prompt:   ""
// 	Answer:   ""
// 	Resolved: false
// }
