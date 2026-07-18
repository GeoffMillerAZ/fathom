// Package anchor defines the CUE-first contract for cryptographic
// proof-of-existence anchors. Go types in types.go, builder.go, and
// validate.go mirror these definitions; any drift is a bug.
//
// Design intent: each #Anchor binds a Merkle root (computed from a batch of
// evidence content-hashes) to an external timestamp authority's receipt. Each
// #InclusionProof allows a single evidence item to be proven included in the
// batch without revealing the other items.
package anchor

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// _hexString is a constraint for non-empty lowercase-hex strings.
// CUE cannot enforce the character set directly in a short expression, so we
// constrain to non-empty string. The Go implementation enforces hex encoding.
_hexString: _nonEmpty

// #AnchorStatus enumerates the lifecycle states of an Anchor record.
#AnchorStatus: "pending" | "confirmed"

// #Anchor is a durable proof-of-existence record binding a Merkle root to
// an external timestamp authority's receipt at a specific wall-clock instant.
//
// Invariants (enforced by Go Validate()):
//   - Root must be non-empty (64 lowercase hex chars for SHA-256).
//   - LeafCount must be >= 1.
//   - CreatedAt must be a non-empty RFC 3339 string (non-zero in Go).
//   - Authority must be non-empty.
//   - Status must be a valid #AnchorStatus.
#Anchor: {
	// Root is the lowercase hex-encoded SHA-256 Merkle root of the batch.
	Root: _hexString

	// LeafCount is the number of evidence items in the batch.
	LeafCount: >=1

	// CreatedAt is the RFC 3339 UTC timestamp at which this Anchor was created.
	// In Go this field is time.Time; in CUE we constrain it as a non-empty string
	// (the serialised form). The Go clock is injected via AnchorBuilder, never
	// time.Now() inline.
	CreatedAt: _nonEmpty

	// Authority identifies the external timestamp authority.
	// Examples: "rfc3161:freetsa.org", "opentimestamps", "ct-log:oak2024h2".
	Authority: _nonEmpty

	// Status is the lifecycle state of this Anchor record.
	Status: #AnchorStatus
}

// #InclusionProof proves that a single leaf was included in the Merkle tree
// that produced a known root. Presenting this proof reveals only the sibling
// hashes along the path — not the content or hashes of the other leaves.
//
// Invariants (enforced by Go Prove()/Verify()):
//   - LeafIndex must be >= 0.
//   - len(Siblings) == len(Directions).
//   - Each element of Siblings is a 32-byte (64 hex char) SHA-256 hash or nil
//     for a promoted (unpaired) node at that level.
#InclusionProof: {
	// LeafIndex is the 0-based position of the proven leaf.
	LeafIndex: >=0

	// Siblings contains the sibling hash at each Merkle level from leaf to root
	// (exclusive). A null entry indicates a promoted (unpaired) node.
	Siblings: [...(string | null)]

	// Directions[i] is true when the current node at level i is the right child.
	Directions: [...bool]
}
