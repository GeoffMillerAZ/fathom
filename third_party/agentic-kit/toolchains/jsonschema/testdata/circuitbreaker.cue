// testdata/circuitbreaker.cue — fixture for jsonschemagen golden tests.
//
// Copied from foundations/governance/circuit-breaker/schema.cue to keep the
// test self-contained.  Changes to the source schema should be reflected here.
//
// This definition exercises: named definition, integer fields with >=N and >0
// bounds (the >0 form exercises exclusiveMinimum normalisation).
package circuitbreaker

// #Config holds the tunable parameters for a circuit breaker.
#Config: {
	// FailureThreshold is the number of consecutive failures required to open
	// the circuit. Must be at least 1.
	FailureThreshold: >=1 & int

	// OpenCooldown is the duration (in nanoseconds) the breaker stays open
	// before transitioning to half-open. Represented as a positive integer
	// (Go time.Duration is int64 nanoseconds).
	OpenCooldown: >0 & int

	// HalfOpenProbes is the number of consecutive successes required in
	// half-open state before the breaker closes. Must be at least 1.
	HalfOpenProbes: >=1 & int
}
