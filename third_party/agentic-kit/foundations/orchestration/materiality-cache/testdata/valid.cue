// Package materialitycache — valid test data that must unify with schema.cue.
package materialitycache

import "github.com/geoffmilleraz/agentic-kit/foundations/orchestration/materiality-cache:materialitycache"

// validEntry — a well-formed cache entry.
validEntry: materialitycache.#Entry & {
	k:           "sha256:abc123def456"
	value:       'cached payload bytes'
	stored_at:   "2024-01-01T00:00:00Z"
	fingerprint: "sha256:abc123def456"
}

// validEntry_NoFingerprint — an entry without a fingerprint (empty is allowed).
validEntry_NoFingerprint: materialitycache.#Entry & {
	k:           "ns/v1/component-a"
	value:       'raw bytes'
	stored_at:   "2024-06-01T12:00:00Z"
	fingerprint: ""
}

// validGetResult_Hit — a hit result with an entry.
validGetResult_Hit: materialitycache.#GetResult & {
	hit: true
	entry: {
		k:           "sha256:abc123def456"
		value:       'cached payload bytes'
		stored_at:   "2024-01-01T00:00:00Z"
		fingerprint: "sha256:abc123def456"
	}
}

// validGetResult_Miss — a miss result with no entry.
validGetResult_Miss: materialitycache.#GetResult & {
	hit: false
}

// validPutRequest — a well-formed put request.
validPutRequest: materialitycache.#PutRequest & {
	key:         "ns/v2/component-b"
	value:       'some bytes'
	fingerprint: "sha256:deadbeef"
}
