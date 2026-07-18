// Package planningrefinement — valid test data that must unify with schema.cue.
package planningrefinement

import "github.com/geoffmilleraz/agentic-kit/foundations/orchestration/planning-refinement:planningrefinement"

// validPlan is a well-formed plan at round zero.
validPlan: planningrefinement.#Plan & {
	ID:      "plan-001"
	Content: "Identify data sources and define scope"
	Round:   0
}

// validPlanRound2 is a plan that has undergone two refinement rounds.
validPlanRound2: planningrefinement.#Plan & {
	ID:      "plan-002"
	Content: "Revised scope with stakeholder input incorporated"
	Round:   2
}

// validPolicy is a well-formed refinement policy.
validPolicy: planningrefinement.#Policy & {
	MaxRounds: 5
}

// validResultConverged is a converged, non-escalated result.
validResultConverged: planningrefinement.#RefinementResult & {
	Converged: true
	Escalated: false
}

// validResultEscalated is a converged, escalated result.
validResultEscalated: planningrefinement.#RefinementResult & {
	Converged: true
	Escalated: true
}

// validResultOngoing is an ongoing (not yet converged) result.
validResultOngoing: planningrefinement.#RefinementResult & {
	Converged: false
	Escalated: false
}

// validSignals verifies all signal enum values are accepted.
_sig1: planningrefinement.#Signal & "improve"
_sig2: planningrefinement.#Signal & "accept"
_sig3: planningrefinement.#Signal & "reject"
