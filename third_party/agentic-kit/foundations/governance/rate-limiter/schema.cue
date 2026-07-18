// Package ratelimiter defines the schema for rate-limiter configuration.
// These constraints mirror the Go Config types and are evaluated with `cue vet ./...`.
package ratelimiter

// #Config holds the configuration for a token-bucket rate limiter.
#Config: {
	// RatePerSec is the sustained token refill rate in tokens per second.
	// Must be strictly positive.
	RatePerSec: >0 & number

	// Burst is the maximum number of tokens that can accumulate (bucket capacity).
	// Must be at least 1.
	Burst: >=1 & int
}
