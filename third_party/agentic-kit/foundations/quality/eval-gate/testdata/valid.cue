// Package evalgate — valid test data that must unify with schema.cue.
package evalgate

import "github.com/geoffmilleraz/agentic-kit/foundations/quality/eval-gate:evalgate"

// validCheck demonstrates a well-formed passing check.
validCheck: evalgate.#Check & {
	Name:   "response-not-empty"
	Passed: true
	Weight: 1.0
}

// validCheckFailed demonstrates a well-formed failing check.
validCheckFailed: evalgate.#Check & {
	Name:   "no-fabricated-citations"
	Passed: false
	Weight: 2.0
}

// validResult demonstrates a well-formed passing result.
validResult: evalgate.#Result & {
	Passed:   true
	Score:    0.8
	Failures: []
}

// validResultWithFailures demonstrates a well-formed failing result.
validResultWithFailures: evalgate.#Result & {
	Passed:   false
	Score:    0.5
	Failures: ["no-fabricated-citations", "sources-resolve"]
}

// validResultPerfect demonstrates a perfect score.
validResultPerfect: evalgate.#Result & {
	Passed:   true
	Score:    1.0
	Failures: []
}

// validResultZero demonstrates a zero score.
validResultZero: evalgate.#Result & {
	Passed:   false
	Score:    0.0
	Failures: ["all-checks-failed"]
}
