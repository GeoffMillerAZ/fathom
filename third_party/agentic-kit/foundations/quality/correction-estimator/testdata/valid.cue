// Package correctionestimator — valid test data that must unify with schema.cue.
package correctionestimator

import "github.com/geoffmilleraz/agentic-kit/foundations/quality/correction-estimator:correctionestimator"

// validStatsClean demonstrates a finding set with no issues.
validStatsClean: correctionestimator.#FindingStats & {
	Critical:      0
	Major:         0
	Minor:         0
	Informational: 0
}

// validStatsHeavy demonstrates a finding set with multiple severities.
validStatsHeavy: correctionestimator.#FindingStats & {
	Critical:      2
	Major:         5
	Minor:         10
	Informational: 3
}

// validEstimateNegligible demonstrates a negligible burden estimate.
validEstimateNegligible: correctionestimator.#Estimate & {
	Score: 0.0
	Band:  "negligible"
}

// validEstimateLow demonstrates a low burden estimate.
validEstimateLow: correctionestimator.#Estimate & {
	Score: 0.08
	Band:  "low"
}

// validEstimateMedium demonstrates a medium burden estimate.
validEstimateMedium: correctionestimator.#Estimate & {
	Score: 0.4
	Band:  "medium"
}

// validEstimateHigh demonstrates a high burden estimate.
validEstimateHigh: correctionestimator.#Estimate & {
	Score: 0.7
	Band:  "high"
}

// validEstimateCritical demonstrates a critical burden estimate.
validEstimateCritical: correctionestimator.#Estimate & {
	Score: 1.0
	Band:  "critical"
}

// validBands verifies all band enum values are accepted.
_b1: correctionestimator.#Band & "negligible"
_b2: correctionestimator.#Band & "low"
_b3: correctionestimator.#Band & "medium"
_b4: correctionestimator.#Band & "high"
_b5: correctionestimator.#Band & "critical"
