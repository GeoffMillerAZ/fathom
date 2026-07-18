// Package planningrefinement provides an iterative plan-refinement loop with
// convergence detection and escalation after N non-converging rounds.
//
// Usage:
//
//	r := NewRefiner(Policy{MaxRounds: 3})
//	plan := Plan{ID: "p1", Content: "initial plan", Round: 0}
//	plan, converged := r.Refine(plan, SignalImprove)
//	// ... repeat until converged == true
//
// Convergence rules:
//   - SignalAccept  → converged immediately, Escalated=false.
//   - SignalReject  → converged immediately, Escalated=false (plan discarded).
//   - SignalImprove → round counter incremented; once Round reaches MaxRounds
//     the loop is marked Escalated=true and converged=true.
//
// Refine is deterministic: given the same plan and signal sequence it always
// produces the same output.
//
// A Refiner is immutable after construction and safe for concurrent use;
// Refine never mutates receiver state.
package planningrefinement

import "errors"

// Signal is a feedback signal that drives a refinement round.
type Signal string

const (
	// SignalImprove indicates the plan needs further work.
	SignalImprove Signal = "improve"

	// SignalAccept indicates the plan is satisfactory — convergence declared.
	SignalAccept Signal = "accept"

	// SignalReject indicates the plan is rejected — refinement stops.
	SignalReject Signal = "reject"
)

// ErrInvalidSignal is returned when an unrecognized signal is supplied.
var ErrInvalidSignal = errors.New("planningrefinement: invalid signal")

// Plan is a versioned plan that is iteratively refined.
type Plan struct {
	// ID is a stable, unique identifier.
	ID string

	// Content is the current plan text.
	Content string

	// Round is the zero-based refinement round counter.
	Round int
}

// Policy configures the refinement loop.
type Policy struct {
	// MaxRounds is the maximum number of non-converging rounds before escalation.
	MaxRounds int
}

// RefinementResult is the outcome of a single Refine() call.
type RefinementResult struct {
	// Converged is true when Refine declares the loop finished.
	Converged bool

	// Escalated is true when convergence was forced by hitting MaxRounds.
	Escalated bool
}

// Refiner drives the iterative refinement loop for a single plan.
// It is not safe for concurrent use from multiple goroutines.
type Refiner struct {
	policy Policy
}

// NewRefiner returns a Refiner configured with the given Policy.
// If policy.MaxRounds < 1 it is clamped to 1.
func NewRefiner(policy Policy) *Refiner {
	if policy.MaxRounds < 1 {
		policy.MaxRounds = 1
	}
	return &Refiner{policy: policy}
}

// Refine applies a single refinement round to plan given signal.
// Returns the updated plan and a RefinementResult.
//
// Signal semantics:
//   - SignalAccept  → plan unchanged, Converged=true, Escalated=false.
//   - SignalReject  → plan unchanged, Converged=true, Escalated=false.
//   - SignalImprove → plan.Round incremented; Converged and Escalated are
//     set to true only when plan.Round >= policy.MaxRounds.
//
// Returns ErrInvalidSignal if signal is not one of the recognized values.
func (r *Refiner) Refine(plan Plan, signal Signal) (Plan, RefinementResult, error) {
	switch signal {
	case SignalAccept, SignalReject:
		return plan, RefinementResult{Converged: true, Escalated: false}, nil
	case SignalImprove:
		plan.Round++
		if plan.Round >= r.policy.MaxRounds {
			return plan, RefinementResult{Converged: true, Escalated: true}, nil
		}
		return plan, RefinementResult{Converged: false, Escalated: false}, nil
	default:
		return plan, RefinementResult{}, ErrInvalidSignal
	}
}
