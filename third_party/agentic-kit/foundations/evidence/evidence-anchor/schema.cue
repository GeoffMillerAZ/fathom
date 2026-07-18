// Package evidenceanchor defines the CUE-first contract for span-aware
// cryptographic anchoring. Go types in evidenceanchor.go mirror these
// definitions; any drift is a bug. These constraints are evaluated with
// `cue vet ./...`.
package evidenceanchor

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// _hexString constrains a field to a non-empty string. The Go implementation
// enforces that the value is lowercase hex; CUE cannot enforce the character
// set in a short constraint, so we constrain to non-empty.
_hexString: _nonEmpty

// _sha256Hex constrains a field to a 64-character hex string, the canonical
// form of a SHA-256 digest produced by hex.EncodeToString(sha256.Sum256(...)[:]).
_sha256Hex: string & =~"^[0-9a-f]{64}$"

// #InclusionProof proves that a single leaf was included in the Merkle tree
// that produced a known root. It mirrors provenance/anchor.InclusionProof
// inlined here for CUE schema self-containment.
//
// Siblings contains the sibling hash at each level from the leaf to the root
// (exclusive). Each element is a 32-byte SHA-256 hash or null for a promoted
// (unpaired) node. Directions[i] is true when the current node at level i is
// the right child.
#InclusionProof: {
	// LeafIndex is the 0-based position of the proven leaf.
	LeafIndex: int & >=0

	// Siblings are the sibling hashes along the path from leaf to root.
	Siblings: [...(string | null)]

	// Directions[i] is true when the current node at level i is the right child.
	Directions: [...bool]
}

// #SpanProof is the per-span witness returned by AnchorSpans. It binds one
// evidence span to the batch's Merkle root at AnchoredAt and carries everything
// needed to verify that binding offline.
//
// SpanID is a convenience correlator and is NOT part of the trust boundary;
// Verify recomputes the digest from the supplied span. SpanDigest is the
// CanonicalDigest of the span — a lowercase hex SHA-256. Root is the Merkle
// root of the whole batch.
#SpanProof: {
	// SpanID echoes the proven span's ID for correlation. Not part of the
	// trust boundary; Verify recomputes the digest from the span itself.
	SpanID: string

	// SpanDigest is the lowercase hex SHA-256 CanonicalDigest of the span.
	// Exactly 64 lowercase hex characters.
	SpanDigest: _sha256Hex

	// InclusionProof is the Merkle path from this span's leaf to Root.
	InclusionProof: #InclusionProof

	// Root is the lowercase hex SHA-256 Merkle root of the whole batch.
	Root: _sha256Hex

	// AnchoredAt is the RFC 3339 UTC wall-clock instant at which the batch was
	// anchored. In Go this is time.Time; in CUE we constrain to a non-empty
	// string (the serialised form). Trust in this instant comes from the
	// external authority receipt on the Anchor, not from this field alone.
	AnchoredAt: _nonEmpty
}
