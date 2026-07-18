// Package planningrefinement defines the CUE contract for planning refinement.
// These constraints mirror the Go types and are evaluated with `cue vet ./...`.
package planningrefinement

// #Signal enumerates the feedback signals that drive a refinement round.
#Signal: "improve" | "accept" | "reject"

// #Plan is a versioned plan that is iteratively refined.
#Plan: {
	// ID is a stable, non-empty identifier for the plan.
	ID: string & !=""

	// Content is the current plan text.
	Content: string & !=""

	// Round is the zero-based refinement round counter.
	Round: >=0 & int
}

// #Policy configures the refinement loop.
#Policy: {
	// MaxRounds is the maximum number of non-converging rounds before escalation.
	// Must be at least 1.
	MaxRounds: >=1 & int
}

// #RefinementResult is the outcome of a single Refine() call.
#RefinementResult: {
	// Converged is true when the plan was accepted or max rounds reached.
	Converged: bool

	// Escalated is true when MaxRounds was exhausted without convergence.
	Escalated: bool
}
