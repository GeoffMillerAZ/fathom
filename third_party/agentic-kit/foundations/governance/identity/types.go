// Package identity provides privacy-preserving identity extraction and hashing.
// Raw PII is never stored or returned; all identifiers are salted SHA-256 hex
// digests. The package is safe for use in WASM environments (no file I/O,
// no time.Now() calls).
//
// Concurrency contract: Config is a plain value type; Extract is a pure
// function with no shared mutable state and is safe for concurrent use from
// multiple goroutines.
package identity

// Config controls how Extract selects and hashes the request identity.
type Config struct {
	// Salt is mixed into every hash to prevent cross-deployment correlation.
	// Must be kept secret. Rotate to invalidate all existing hashed identities.
	Salt string

	// TrustedProxyHops is the number of rightmost X-Forwarded-For entries
	// added by infrastructure you control (e.g. load-balancers, CDN edge).
	// Set to 0 to ignore XFF entirely and use remoteAddr. Set to N to skip
	// the N rightmost XFF entries and take the first untrusted hop.
	TrustedProxyHops int
}
