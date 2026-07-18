// Package circuitbreaker implements a closed/open/half-open circuit breaker
// for protecting downstream dependencies from cascading failures.
//
// State machine:
//
//	closed  --[failures ≥ threshold]--> open
//	open    --[cooldown elapsed]------> half-open
//	half-open --[probe success ≥ n]---> closed
//	half-open --[any failure]---------> open
//
// Design constraints (WASM-safe):
//   - stdlib only: no third-party dependencies.
//   - No time.Now() in logic: callers inject a now func() time.Time.
//   - No direct file I/O: all state is in-memory.
//   - Concurrency-safe via sync.Mutex.
//   - Deterministic and fail-closed: when open, Allow returns false.
package circuitbreaker

import (
	"sync"
	"time"
)

// State represents the circuit breaker state.
type state int

const (
	stateClosed   state = iota
	stateOpen     state = iota
	stateHalfOpen state = iota
)

// Config holds the tunable parameters for a Breaker.
type Config struct {
	// FailureThreshold is the number of consecutive failures required to open
	// the circuit. Must be ≥ 1.
	FailureThreshold int

	// OpenCooldown is how long the breaker stays open before transitioning to
	// half-open and allowing probe requests through.
	OpenCooldown time.Duration

	// HalfOpenProbes is the number of consecutive successes required while in
	// half-open state before the breaker closes.
	HalfOpenProbes int
}

// Breaker is a circuit breaker that tracks failure/success rates and opens the
// circuit to protect a downstream dependency.
//
// It is safe for concurrent use.
type Breaker struct {
	mu             sync.Mutex
	cfg            Config
	now            func() time.Time
	cur            state
	failures       int // consecutive failures in closed state
	probeSuccesses int // consecutive successes in half-open state
	openedAt       time.Time
}

// New constructs a new Breaker starting in the closed state.
func New(cfg Config, now func() time.Time) *Breaker {
	return &Breaker{
		cfg: cfg,
		now: now,
		cur: stateClosed,
	}
}

// State returns the current state as a human-readable string:
// "closed", "open", or "half-open".
func (b *Breaker) State() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.checkCooldown()
	return stateString(b.cur)
}

// Allow returns true if a request is permitted to proceed.
//
//   - closed: always true.
//   - open: false (fail-closed).
//   - half-open: true (probe request).
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.checkCooldown()
	return b.cur != stateOpen
}

// Record records the outcome of a request. success=true is a success,
// success=false is a failure.
//
// In closed state: consecutive failures are counted; reaching FailureThreshold
// opens the circuit. A success resets the consecutive failure counter.
//
// In half-open state: a failure immediately re-opens the circuit; consecutive
// successes accumulate and close the circuit once HalfOpenProbes is reached.
//
// In open state: recordings are ignored.
func (b *Breaker) Record(success bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.checkCooldown()

	switch b.cur {
	case stateClosed:
		if success {
			b.failures = 0
			return
		}
		b.failures++
		if b.failures >= b.cfg.FailureThreshold {
			b.openCircuit()
		}

	case stateHalfOpen:
		if !success {
			b.openCircuit()
			return
		}
		b.probeSuccesses++
		if b.probeSuccesses >= b.cfg.HalfOpenProbes {
			b.closeCircuit()
		}

	case stateOpen:
		// ignore
	}
}

// Trip manually opens the circuit breaker. This is a programmatic trigger
// equivalent to reaching the failure threshold.
func (b *Breaker) Trip() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.openCircuit()
}

// ForceOpen is a kill-switch that opens the circuit breaker unconditionally,
// even if it is currently healthy. The breaker will not auto-recover until
// Reset is called or it transitions through its normal cooldown cycle.
func (b *Breaker) ForceOpen() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.openCircuit()
}

// Reset manually closes the circuit breaker and clears all counters.
// This is a manual recovery mechanism for operators.
func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closeCircuit()
}

// checkCooldown transitions open → half-open if OpenCooldown has elapsed.
// Must be called with b.mu held.
func (b *Breaker) checkCooldown() {
	if b.cur != stateOpen {
		return
	}
	if b.now().Sub(b.openedAt) >= b.cfg.OpenCooldown {
		b.cur = stateHalfOpen
		b.probeSuccesses = 0
	}
}

// openCircuit transitions to the open state and records the open time.
// Must be called with b.mu held.
func (b *Breaker) openCircuit() {
	b.cur = stateOpen
	b.openedAt = b.now()
	b.failures = 0
	b.probeSuccesses = 0
}

// closeCircuit transitions to the closed state and resets all counters.
// Must be called with b.mu held.
func (b *Breaker) closeCircuit() {
	b.cur = stateClosed
	b.failures = 0
	b.probeSuccesses = 0
}

// stateString converts internal state to string. No lock required.
func stateString(s state) string {
	switch s {
	case stateClosed:
		return "closed"
	case stateOpen:
		return "open"
	case stateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}
