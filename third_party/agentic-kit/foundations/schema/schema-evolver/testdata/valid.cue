// Package schemaevolver — valid test data that must unify with schema.cue.
package schemaevolver

import "github.com/geoffmilleraz/agentic-kit/foundations/schema/schema-evolver:schemaevolver"

// validVersion demonstrates a well-formed schema version.
validVersion: schemaevolver.#Version & {
	Major: 2
	Minor: 1
}

// validMigration_BreakingMajorBump demonstrates a breaking migration that
// increments the major version.
validMigration_BreakingMajorBump: schemaevolver.#Migration & {
	From: {Major: 1, Minor: 0}
	To:   {Major: 2, Minor: 0}
	Description:       "Add required `owner` field; consumers must populate it."
	CompatibilityMode: "breaking"
}

// validMigration_BackwardCompatibleMinorBump demonstrates a safe minor-version
// migration that adds an optional field.
validMigration_BackwardCompatibleMinorBump: schemaevolver.#Migration & {
	From: {Major: 2, Minor: 0}
	To:   {Major: 2, Minor: 1}
	Description:       "Add optional `tags` field; existing consumers unaffected."
	CompatibilityMode: "backward-compatible"
}

// validCompatibilityModes verifies all enum values are accepted.
_c1: schemaevolver.#CompatibilityMode & "backward-compatible"
_c2: schemaevolver.#CompatibilityMode & "breaking"
