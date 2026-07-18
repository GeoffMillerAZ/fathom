// Package redactionexecutor — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidRule_EmptyID:
//     ID: "" violates string & !="" — rule ID must not be empty
//
//   invalidRule_EmptyPattern:
//     Pattern: "" violates string & !="" — pattern must not be empty
//
//   invalidPolicy_EmptyName:
//     Name: "" violates string & !="" — policy name must not be empty
//
//   invalidPolicy_EmptyVersion:
//     Version: "" violates string & !="" — policy version must not be empty
//
//   invalidPolicy_EmptyRules:
//     Rules: [] violates [#RedactionRule, ...#RedactionRule] — at least one rule required
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package redactionexecutor

// // invalidRule_EmptyID — ID must not be empty
// invalidRule_EmptyID: #RedactionRule & {
// 	ID:          ""
// 	Pattern:     ".*"
// 	Replacement: "[REDACTED]"
// }

// // invalidRule_EmptyPattern — Pattern must not be empty
// invalidRule_EmptyPattern: #RedactionRule & {
// 	ID:          "REDACT-EMPTY-001"
// 	Pattern:     ""
// 	Replacement: "[REDACTED]"
// }

// // invalidPolicy_EmptyName — policy Name must not be empty
// invalidPolicy_EmptyName: #RedactionPolicy & {
// 	Name:    ""
// 	Version: "1.0.0"
// 	Rules: [
// 		{ID: "R-001", Pattern: ".*", Replacement: "[REDACTED]"},
// 	]
// }

// // invalidPolicy_EmptyVersion — policy Version must not be empty
// invalidPolicy_EmptyVersion: #RedactionPolicy & {
// 	Name:    "pii-v1"
// 	Version: ""
// 	Rules: [
// 		{ID: "R-001", Pattern: ".*", Replacement: "[REDACTED]"},
// 	]
// }

// // invalidPolicy_EmptyRules — Rules must have at least one element
// invalidPolicy_EmptyRules: #RedactionPolicy & {
// 	Name:    "empty-v1"
// 	Version: "1.0.0"
// 	Rules:   []
// }
