// Package materialitycache defines the CUE schema for materiality-cache types.
// These constraints are evaluated with `cue vet ./...`.
package materialitycache

// #Entry is a single cached value with metadata.
#Entry: {
	// k is the materiality key (non-empty content-addressed fingerprint).
	k:           string & !=""
	// value is the cached payload (bytes).
	value:       bytes
	// stored_at is an RFC 3339 timestamp recording when this entry was stored.
	stored_at:   string
	// fingerprint is an optional content hash of the value for drift detection.
	fingerprint: string
}

// #GetResult is the result of a cache lookup.
#GetResult: {
	hit: bool
	// entry is present only when hit is true.
	entry?: #Entry
}

// #PutRequest describes a cache Put operation.
#PutRequest: {
	key:         string & !=""
	value:       bytes
	fingerprint: string
}
