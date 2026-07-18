// Package memorymanifest defines the CUE-first contract for memory-manifest
// entries. The Go struct ManifestEntry in types.go mirrors this definition.
// Any drift between the two is a bug.
package memorymanifest

// #ManifestEntry records a single knowledge card stored in the manifest.
#ManifestEntry: {
	// CardID is the unique, stable identifier of the knowledge card.
	// Must be non-empty.
	CardID: string & !=""

	// Hash is the deterministic content fingerprint of the card.
	// Must be non-empty.
	Hash: string & !=""

	// CreatedAt is an RFC 3339 timestamp recording when the entry was appended.
	// The string must be non-empty.
	CreatedAt: string & !=""
}
