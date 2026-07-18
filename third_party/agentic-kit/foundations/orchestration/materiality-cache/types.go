// Package materialitycache provides a deterministic cache keyed by a
// materiality/freshness key with explicit invalidation.
//
// A materiality key is a content-addressed fingerprint (e.g. a SHA-256 hex
// string or a composite of namespace + version + hash). Cache entries are
// valid until explicitly invalidated — there is no background eviction. This
// makes the cache deterministic: given the same sequence of Put/Invalidate
// calls, Get always returns the same result.
//
// The zero-value Cache is not safe for use; create one with New.
//
// A Cache is not safe for concurrent use; guard it with your own
// synchronization when sharing it across goroutines.
package materialitycache

import "time"

// Key is the materiality/freshness key used to address a cache entry.
// It should be a stable, content-addressed fingerprint for the underlying data.
type Key = string

// Entry is a single cached value with metadata.
type Entry struct {
	// K is the key under which this entry was stored.
	K Key
	// Value is the cached payload.
	Value []byte
	// StoredAt records when this entry was stored.
	StoredAt time.Time
	// Fingerprint is an optional content hash of the Value, set by the caller.
	// Used by callers to detect whether a freshly produced value differs from
	// the cached one without reading the full payload.
	Fingerprint string
}

// GetResult is the result of a Cache.Get call.
type GetResult struct {
	// Hit is true when the key was found and has not been invalidated.
	Hit bool
	// Entry is populated when Hit is true.
	Entry Entry
}
