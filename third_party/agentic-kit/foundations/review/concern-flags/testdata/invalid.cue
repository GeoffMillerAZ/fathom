// Package concernflags — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidFlag_BadSeverity:
//     Severity: "extreme" violates #Severity enum — must be low|medium|high|critical
//
//   invalidFlag_EmptyID:
//     ID: "" violates string & !="" — ID must not be empty
//
//   invalidFlag_EmptyTarget:
//     Target: "" violates string & !="" — Target must not be empty
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package concernflags

// // invalidFlag_BadSeverity — "extreme" is not a valid #Severity
// invalidFlag_BadSeverity: #ConcernFlag & {
// 	ID:        "flag-bad-sev"
// 	Type:      "accuracy"
// 	Severity:  "extreme"
// 	Target:    "doc/section/1"
// 	Rationale: "test"
// }

// // invalidFlag_EmptyID — ID must not be empty
// invalidFlag_EmptyID: #ConcernFlag & {
// 	ID:        ""
// 	Type:      "accuracy"
// 	Severity:  "low"
// 	Target:    "doc/section/1"
// 	Rationale: "test"
// }

// // invalidFlag_EmptyTarget — Target must not be empty
// invalidFlag_EmptyTarget: #ConcernFlag & {
// 	ID:        "flag-no-target"
// 	Type:      "accuracy"
// 	Severity:  "low"
// 	Target:    ""
// 	Rationale: "test"
// }
