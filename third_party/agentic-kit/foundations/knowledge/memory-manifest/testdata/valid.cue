// valid.cue — concrete values that satisfy the #ManifestEntry constraint
// defined in schema.cue. Running `cue vet ./...` from the package root or
// the repo root validates these against the schema.
package memorymanifest

// A well-formed manifest entry with a SHA-256 style hash.
_entry1: #ManifestEntry & {
	CardID:    "card-abc-001"
	Hash:      "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	CreatedAt: "2024-11-01T09:00:00Z"
}

// A second entry with a different card and hash.
_entry2: #ManifestEntry & {
	CardID:    "card-xyz-002"
	Hash:      "a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3"
	CreatedAt: "2024-11-02T14:30:00Z"
}

// An entry representing a freshly generated card.
_entry3: #ManifestEntry & {
	CardID:    "card-svc-payments"
	Hash:      "d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2"
	CreatedAt: "2025-01-15T08:00:00Z"
}
