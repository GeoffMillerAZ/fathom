// Package confidencescoring — valid test data that must unify with schema.cue.
package confidencescoring

import "github.com/geoffmilleraz/agentic-kit/foundations/review/confidence-scoring:confidencescoring"

// validScores verifies the boundary values and midpoint of ConfidenceScore.
_score1: confidencescoring.#ConfidenceScore & 1
_score3: confidencescoring.#ConfidenceScore & 3
_score5: confidencescoring.#ConfidenceScore & 5

// validInputLow is an EvidenceInput with no evidence.
validInputLow: confidencescoring.#EvidenceInput & {
	Count:          0
	AgreementRatio: 0.0
}

// validInputHigh is an EvidenceInput with strong evidence.
validInputHigh: confidencescoring.#EvidenceInput & {
	Count:          10
	AgreementRatio: 0.95
}

// validInputMid is an EvidenceInput with moderate evidence.
validInputMid: confidencescoring.#EvidenceInput & {
	Count:          3
	AgreementRatio: 0.67
}
