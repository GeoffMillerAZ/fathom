// valid.cue — concrete values that satisfy the constraints defined in
// schema.cue. Running `cue vet ./...` from the package root validates these
// against the schema.
package anchor

// A pending anchor with a real-looking Merkle root and two leaves.
_anchorPending: #Anchor & {
	Root:      "e3b0c44298fc1c149afbf4c8996fb924a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4"
	LeafCount: 2
	CreatedAt: "2026-01-01T00:00:00Z"
	Authority: "rfc3161:freetsa.org"
	Status:    "pending"
}

// A confirmed anchor with an OpenTimestamps authority.
_anchorConfirmed: #Anchor & {
	Root:      "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	LeafCount: 5
	CreatedAt: "2026-03-15T12:30:00Z"
	Authority: "opentimestamps"
	Status:    "confirmed"
}

// A confirmed anchor with a CT log authority.
_anchorCTLog: #Anchor & {
	Root:      "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	LeafCount: 100
	CreatedAt: "2026-06-08T09:00:00Z"
	Authority: "ct-log:oak2024h2"
	Status:    "confirmed"
}

// Minimum viable anchor: single leaf, pending.
_anchorMinimal: #Anchor & {
	Root:      "x"
	LeafCount: 1
	CreatedAt: "2026-01-01T00:00:00Z"
	Authority: "opentimestamps"
	Status:    "pending"
}

// An inclusion proof with two siblings (3-level tree path).
_proofTwoSiblings: #InclusionProof & {
	LeafIndex:  3
	Siblings:   ["abc123def456abc123def456abc123def456abc123def456abc123def456abc1", "fedcba9876fedcba9876fedcba9876fedcba9876fedcba9876fedcba9876fedc"]
	Directions: [true, false]
}

// A single-leaf proof (no siblings required).
_proofNoSiblings: #InclusionProof & {
	LeafIndex:  0
	Siblings:   []
	Directions: []
}

// A proof with a promoted (nil) sibling entry at the first level.
_proofWithPromotedNode: #InclusionProof & {
	LeafIndex:  2
	Siblings:   [null, "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"]
	Directions: [false, false]
}
