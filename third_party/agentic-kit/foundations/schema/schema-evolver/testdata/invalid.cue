// Package schemaevolver — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidVersion_NegativeMajor:
//     Major: -1 violates >=0 — major version must be non-negative
//
//   invalidMigration_EmptyDescription:
//     Description: "" violates string & !="" — description must not be empty
//
//   invalidMigration_BadCompatibilityMode:
//     CompatibilityMode: "unknown" violates #CompatibilityMode enum
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package schemaevolver

// // invalidVersion_NegativeMajor — Major -1 is out of range [0, ∞)
// invalidVersion_NegativeMajor: #Version & {
// 	Major: -1
// 	Minor: 0
// }

// // invalidMigration_EmptyDescription — Description must not be empty
// invalidMigration_EmptyDescription: #Migration & {
// 	From:              {Major: 1, Minor: 0}
// 	To:                {Major: 1, Minor: 1}
// 	Description:       ""
// 	CompatibilityMode: "backward-compatible"
// }

// // invalidMigration_BadCompatibilityMode — "unknown" is not a valid #CompatibilityMode
// invalidMigration_BadCompatibilityMode: #Migration & {
// 	From:              {Major: 1, Minor: 0}
// 	To:                {Major: 2, Minor: 0}
// 	Description:       "some change"
// 	CompatibilityMode: "unknown"
// }
