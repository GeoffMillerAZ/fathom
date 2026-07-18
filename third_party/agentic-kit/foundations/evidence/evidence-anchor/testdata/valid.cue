// Package evidenceanchor — valid test data that must unify with schema.cue.
// Running `cue vet ./...` from the package root and `cue vet ./testdata/` both
// validate these against the schema.
package evidenceanchor

import "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-anchor:evidenceanchor"

// validMinimalProof is the smallest legal SpanProof: every required field at
// its simplest legal value. SpanID may be any non-empty string; SpanDigest
// and Root are valid SHA-256 lowercase hex strings (64 chars).
validMinimalProof: evidenceanchor.#SpanProof & {
	SpanID:     "span-001"
	SpanDigest: "a3f1b2c4d5e6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"
	InclusionProof: {
		LeafIndex:  0
		Siblings:   []
		Directions: []
	}
	Root:       "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	AnchoredAt: "2026-01-01T00:00:00Z"
}

// validProofWithSiblings exercises a multi-leaf tree: the InclusionProof
// carries sibling hashes and directions as produced by AnchorSpans on a batch
// of two or more spans. Directions[0]=false means the current node is the
// left child at level 0.
validProofWithSiblings: evidenceanchor.#SpanProof & {
	SpanID:     "span-002"
	SpanDigest: "b3f1b2c4d5e6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"
	InclusionProof: {
		LeafIndex:  1
		Siblings:   ["c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5"]
		Directions: [true]
	}
	Root:       "e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6"
	AnchoredAt: "2026-06-15T09:30:00.000000001Z"
}

// validProofNullSibling shows that a promoted (unpaired) node at a tree level
// is represented as null in the Siblings slice, not omitted or as an empty
// string. This mirrors AnchorSpans behaviour for odd-count batches.
validProofNullSibling: evidenceanchor.#SpanProof & {
	SpanID:     "span-003"
	SpanDigest: "f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2"
	InclusionProof: {
		LeafIndex:  2
		Siblings:   [null, "a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3"]
		Directions: [false, false]
	}
	Root:       "b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4"
	AnchoredAt: "2025-12-31T23:59:59Z"
}

// validProofWithOffset demonstrates a non-zero RFC3339 fractional-second
// timestamp with a timezone offset, as produced by time.Time.Format(time.RFC3339Nano)
// in Go when the anchor's wall-clock carries sub-second precision.
validProofWithOffset: evidenceanchor.#SpanProof & {
	SpanID:     "span-004"
	SpanDigest: "c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5"
	InclusionProof: {
		LeafIndex:  0
		Siblings:   []
		Directions: []
	}
	Root:       "d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6"
	AnchoredAt: "2024-06-01T14:05:01.123456789+00:00"
}
