// Package ratelimiter — valid test data that must unify with schema.cue.
package ratelimiter

import "github.com/geoffmilleraz/agentic-kit/foundations/governance/rate-limiter:ratelimiter"

// validStandard is a typical API rate limit: 10 requests/second with a burst of 50.
validStandard: ratelimiter.#Config & {
	RatePerSec: 10.0
	Burst:      50
}

// validConservative is a conservative rate limit for expensive LLM operations.
validConservative: ratelimiter.#Config & {
	RatePerSec: 0.5
	Burst:      2
}

// validHigh is a high-throughput configuration for internal event streams.
validHigh: ratelimiter.#Config & {
	RatePerSec: 1000.0
	Burst:      5000
}

// validMinimal is the smallest valid configuration.
validMinimal: ratelimiter.#Config & {
	RatePerSec: 0.001
	Burst:      1
}
