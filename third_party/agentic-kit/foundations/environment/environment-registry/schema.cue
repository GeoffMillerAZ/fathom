// Package environmentregistry defines the CUE schema for environment records.
// These constraints are the authoritative shape specification; the Go types
// in types.go mirror them. Evaluated with `cue vet ./...`.
package environmentregistry

// #Kind enumerates the valid environment tier values.
#Kind: "dev" | "staging" | "prod"

// #Environment describes a named runtime context with a tier classification.
#Environment: {
	// Name is the unique, human-readable identifier. Must be non-empty.
	Name!: string & !=""

	// Kind classifies the environment tier.
	Kind!: #Kind
}
