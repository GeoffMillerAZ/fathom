// Package fitnessfunctionregistry provides named fitness checks and versioned,
// evidence-linked assessment profiles, separating required outcomes from optional
// weighted scoring delegated to eval-gate.
//
// A fitness function pairs a human-readable Invariant with caller-supplied Checks,
// maturity metadata and a schedule hint. Execute scores those booleans; it does not
// run tests or establish that the invariant was measured. Assessment profiles add
// criterion/subject identities, evidence methods, freshness and explicit unknown/N/A
// outcomes. Evidence remains caller-declared; a hash does not authenticate it.
//
// This package is the PRIMITIVE. It deliberately does NOT implement the
// tier-gated promotion policy (e.g. "an artifact may only be promoted from
// prototype to apex once every apex-tier function passes"). That policy is a
// caller concern: a downstream scorecard or evidence consumer applies its own
// gating on top of the ExecutionResults this package produces.
//
// Scoring is delegated entirely to quality/eval-gate — this package never
// reinvents weighted-check scoring. Execution is fully deterministic: the
// caller injects both the RunID and the timestamp, and all iteration over the
// registry is sorted by name, so there is no map-ordering in any output.
//
// CUE contracts live in schema.cue and profile.cue; Go counterparts must
// stay in sync with those schemas. No LLM is involved; every projection is fully
// reproducible given the same inputs.
//
// Concurrency: Execute and ExecuteTimed are pure functions safe for concurrent
// use. Registry is NOT safe for concurrent mutation — guard Register calls with
// your own lock when registering from multiple goroutines. Concurrent reads
// (Get, List, Len, ExecuteAll) are safe once construction is complete.
package fitnessfunctionregistry

import (
	"time"

	evalgate "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/eval-gate"
)

// Tier names the maturity band at which a fitness function's invariant is
// binding. Tiers are ordered from weakest to strongest expectation:
// Prototype < Integration < Apex. The ordering is meaningful to callers
// implementing tier-gated promotion, but this package treats Tier as opaque
// metadata and never gates on it.
type Tier string

const (
	// TierPrototype is the weakest band: the invariant is exercised but not yet
	// load-bearing for promotion. Failures are informational.
	TierPrototype Tier = "prototype"

	// TierIntegration is the middle band: the invariant must hold for an
	// artifact to be integrated with its peers.
	TierIntegration Tier = "integration"

	// TierApex is the strongest band: the invariant is a hard precondition for
	// the highest level of trust (e.g. apex promotion / release).
	TierApex Tier = "apex"
)

// tierRank maps each recognised Tier to its ordinal strength. Used only by
// Tier.Valid; callers wanting to compare strength should use Rank.
var tierRank = map[Tier]int{
	TierPrototype:   0,
	TierIntegration: 1,
	TierApex:        2,
}

// Valid reports whether t is one of the recognised tiers.
func (t Tier) Valid() bool {
	_, ok := tierRank[t]
	return ok
}

// Rank returns the ordinal strength of the tier (Prototype=0, Integration=1,
// Apex=2) and ok=false for an unrecognised tier. It lets callers compare tier
// strength without hardcoding the ordering.
func (t Tier) Rank() (rank int, ok bool) {
	r, ok := tierRank[t]
	return r, ok
}

// FitnessFunction is a named invariant declaration with caller-supplied checks.
// It describes what to score, not how to collect or validate the evidence.
//
// CUE mirror: #FitnessFunction in schema.cue.
type FitnessFunction struct {
	// Name uniquely identifies the function within a Registry. Must be
	// non-empty. Registry iteration is sorted by this field.
	Name string

	// Description is a human-readable summary of what the function measures.
	Description string

	// Invariant states, in human-readable form, the property that must remain
	// true (e.g. "every persisted span carries provenance"). The Checks are the
	// caller-declared outcomes, not independent evidence that the invariant holds.
	Invariant string

	// Checks are the weighted criteria evaluated to decide whether the
	// invariant holds. They are scored by quality/eval-gate; this package does
	// not interpret them beyond passing them through. May be empty (an empty
	// function scores 0 — see Execute).
	Checks []evalgate.Check

	// Tier is the maturity band at which this invariant is binding. Opaque to
	// this package; consumed by callers implementing promotion policy.
	Tier Tier

	// RunFreq is how often this function should be executed (e.g. "per-commit",
	// "nightly", "pre-release"). It is advisory metadata for schedulers; this
	// package never acts on it.
	RunFreq string

	// Threshold is the minimum eval-gate Score required for the function to be
	// considered Passed, in [0, 1]. A zero value is treated as 1.0 (all checks
	// must pass) — this mirrors evalgate.Gate's zero-Threshold semantics.
	Threshold float64
}

// ExecutionResult is the deterministic outcome of executing a FitnessFunction.
// It is the record downstream scorecards and evidence consumers receive; every
// field is reproducible from (FitnessFunction, RunID, At).
//
// CUE mirror: #ExecutionResult in schema.cue.
type ExecutionResult struct {
	// FunctionName is the Name of the executed FitnessFunction.
	FunctionName string

	// Score is the weighted fraction of passing checks in [0, 1], as computed
	// by quality/eval-gate.
	Score float64

	// Passed is true when Score >= the function's effective threshold.
	Passed bool

	// Failures lists the names of checks that did not pass, in the order
	// eval-gate returns them (input order of the Checks slice). Empty when every
	// check passed.
	Failures []string

	// RunID is the caller-injected identifier tying this result to a run. It is
	// passed through unchanged for traceability and never generated internally.
	RunID string

	// At is the caller-injected timestamp marking when the run occurred. It is
	// passed through unchanged; the package never reads the wall clock.
	At time.Time

	// Duration is the optional measured wall-clock cost of producing the result.
	// Execute leaves it zero (it is a pure function and does no timing); use
	// ExecuteTimedWithClock, or set it from a caller-supplied measurement, when a duration
	// is wanted. A zero Duration means "not measured".
	Duration time.Duration
}
