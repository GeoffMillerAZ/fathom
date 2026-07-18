// Package convergencedetector — valid test data that must unify with schema.cue.
package convergencedetector

import "github.com/geoffmilleraz/agentic-kit/foundations/coordination/convergence-detector:convergencedetector"

// validPolicy is a well-formed convergence policy.
validPolicy: convergencedetector.#Policy & {
	Window:       5
	Threshold:    0.8
	AbandonAfter: 20
}

// validPolicy_MinimalWindow is a policy with the minimum window size.
validPolicy_MinimalWindow: convergencedetector.#Policy & {
	Window:       1
	Threshold:    1.0
	AbandonAfter: 10
}

// validSignal is a well-formed result signal.
validSignal: convergencedetector.#Signal & {
	Value: "answer-42"
}

// validState_Exploring is a detector in the exploring state.
validState_Exploring: convergencedetector.#DetectorState & {
	SignalCount: 3
	Verdict:     "exploring"
}

// validState_Converged is a detector that has reached convergence.
validState_Converged: convergencedetector.#DetectorState & {
	SignalCount: 10
	Verdict:     "converged"
}

// validState_Abandoned is a detector that gave up.
validState_Abandoned: convergencedetector.#DetectorState & {
	SignalCount: 25
	Verdict:     "abandoned"
}

// validVerdicts verifies all verdict enum values are accepted.
_v1: convergencedetector.#Verdict & "converged"
_v2: convergencedetector.#Verdict & "exploring"
_v3: convergencedetector.#Verdict & "abandoned"
