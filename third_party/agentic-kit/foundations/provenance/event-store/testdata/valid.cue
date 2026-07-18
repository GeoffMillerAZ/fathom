// valid.cue — concrete values that satisfy the #Entry constraint defined in
// schema.cue. Running `cue vet ./...` from the package root validates these
// against the schema.
package eventstore

// A well-formed genesis entry (Seq 0, PrevHash is 64 zeros).
_genesis: #Entry & {
	Seq:         0
	Kind:        "document.ingested"
	PayloadHash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	PrevHash:    "0000000000000000000000000000000000000000000000000000000000000000"
	Hash:        "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	RecordedAt:  "2025-01-15T10:00:00Z"
}

// A second entry whose PrevHash links to the genesis entry's Hash.
_second: #Entry & {
	Seq:         1
	Kind:        "entity.extracted"
	PayloadHash: "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	PrevHash:    "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	Hash:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	RecordedAt:  "2025-01-15T10:00:01Z"
}

// Minimum viable entry: Seq 0 with all required fields present.
_minimal: #Entry & {
	Seq:         0
	Kind:        "k"
	PayloadHash: "x"
	PrevHash:    ""
	Hash:        "h"
	RecordedAt:  "t"
}
