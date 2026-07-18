// Package circuitbreaker — valid test data that must unify with schema.cue.
package circuitbreaker

import "github.com/geoffmilleraz/agentic-kit/foundations/governance/circuit-breaker:circuitbreaker"

// validDefault is a typical production circuit breaker:
// opens after 5 failures, stays open 30s, needs 3 probe successes to close.
validDefault: circuitbreaker.#Config & {
	FailureThreshold: 5
	OpenCooldown:     30_000_000_000 // 30 seconds in nanoseconds
	HalfOpenProbes:   3
}

// validAggressive trips quickly and recovers quickly — useful for fast-fail scenarios.
validAggressive: circuitbreaker.#Config & {
	FailureThreshold: 1
	OpenCooldown:     1_000_000_000 // 1 second in nanoseconds
	HalfOpenProbes:   1
}

// validConservative requires many failures and a long cooldown before tripping.
validConservative: circuitbreaker.#Config & {
	FailureThreshold: 20
	OpenCooldown:     300_000_000_000 // 5 minutes in nanoseconds
	HalfOpenProbes:   10
}
