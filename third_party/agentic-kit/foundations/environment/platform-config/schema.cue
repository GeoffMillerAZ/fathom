// Package platformconfig defines the CUE schema for platform configuration.
// These constraints are the authoritative shape specification; the Go types
// in types.go mirror them. Evaluated with `cue vet ./...`.
package platformconfig

// #Config holds the platform-level configuration for a single environment context.
#Config: {
	// Environment is the name of the target environment. Must be non-empty.
	Environment!: string & !=""

	// Settings is an arbitrary string-to-string map of platform settings.
	// The map itself must be present (non-null); values may be empty strings.
	Settings!: {[string]: string}
}
