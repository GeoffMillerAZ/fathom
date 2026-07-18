// Package schemaevolver defines CUE contracts for schema evolution records.
// These constraints mirror the Go types and are evaluated with `cue vet ./...`.
package schemaevolver

// #Version represents a two-part schema version with a major and minor component.
// Major increments signal backward-incompatible changes; minor increments are
// backward-compatible additions.
#Version: {
	// Major is the major version component; must be a non-negative integer.
	Major: >=0 & int
	// Minor is the minor version component; must be a non-negative integer.
	Minor: >=0 & int
}

// #CompatibilityMode describes whether a migration step preserves backward
// compatibility for existing consumers.
#CompatibilityMode: "backward-compatible" | "breaking"

// #Migration describes a single controlled migration step between two schema
// versions. Registering a migration makes its path discoverable by the Evolver.
#Migration: {
	// From is the source version for this migration step.
	From: #Version

	// To is the destination version for this migration step.
	To: #Version

	// Description is a human-readable explanation of what changed.
	Description: string & !=""

	// CompatibilityMode classifies whether this step breaks existing consumers.
	CompatibilityMode: #CompatibilityMode
}
