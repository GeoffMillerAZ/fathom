// Package proberuntime — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidResult_EmptyName:
//     Name: "" violates string & !="" — Name must not be empty
//
//   invalidResult_BadStatus:
//     Status: "skipped" violates #ProbeStatus enum — must be pass/fail/error
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package proberuntime

// // invalidResult_EmptyName — Name must not be empty
// invalidResult_EmptyName: #ProbeResult & {
// 	Name:   ""
// 	Status: "pass"
// 	Detail: ""
// }

// // invalidResult_BadStatus — "skipped" is not a valid #ProbeStatus
// invalidResult_BadStatus: #ProbeResult & {
// 	Name:   "probe-x"
// 	Status: "skipped"
// 	Detail: ""
// }
