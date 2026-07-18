// Package operatorelicitation — valid test data that must unify with schema.cue.
package operatorelicitation

import "github.com/geoffmilleraz/agentic-kit/foundations/orchestration/operator-elicitation:operatorelicitation"

// validPendingRequest is a well-formed unresolved request.
validPendingRequest: operatorelicitation.#Request & {
	ID:       "req-1"
	Prompt:   "What is the preferred output format?"
	Answer:   ""
	Resolved: false
}

// validResolvedRequest is a request that has been answered.
validResolvedRequest: operatorelicitation.#Request & {
	ID:       "req-2"
	Prompt:   "Should we include raw evidence citations?"
	Answer:   "Yes, include all citations"
	Resolved: true
}

// validRequestEmptyAnswer verifies that Answer may be empty string.
validRequestEmptyAnswer: operatorelicitation.#Request & {
	ID:       "req-3"
	Prompt:   "Any additional constraints?"
	Answer:   ""
	Resolved: false
}
