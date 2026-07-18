// Positive CUE fixture for truth-bridge schema conformance.
// cue vet schema.cue testdata/valid.cue must pass.
package truthbridge

// Example ChunkLink binding a chunk to a documentary-grade span.
_link: #ChunkLink & {
	chunk_id:           "chunk-abc-001"
	span_locator_kind:  "line"
	span_locator_value: "42"
	grade_tier:         "documentary"
	source_id:          "src-registry-001"
	// 64 lowercase hex chars — valid content hash
	content_hash:      "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	anchor_proof_ref:  ""
}

// Example ChunkLink for an anchored direct-grade chunk.
_anchoredLink: #ChunkLink & {
	chunk_id:           "chunk-abc-002"
	span_locator_kind:  "char-range"
	span_locator_value: "100-200"
	grade_tier:         "direct"
	source_id:          "src-registry-002"
	content_hash:       "0000000000000000000000000000000000000000000000000000000000000001"
	anchor_proof_ref:   "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
}

// Example GradedResult for a graded chunk.
_graded: #GradedResult & {
	chunk_id:         "chunk-abc-001"
	score:            0.87
	grade:            "documentary"
	graded:           true
	source_id:        "src-registry-001"
	anchor_proof_ref: ""
}

// Example GradedResult for an unknown-grade chunk (the honest default).
_unknown: #GradedResult & {
	chunk_id:         "chunk-xyz-999"
	score:            0.43
	grade:            "unknown"
	graded:           false
	source_id:        ""
	anchor_proof_ref: ""
}
