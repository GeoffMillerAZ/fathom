// Package correctionledgeranalytical — valid test data that must unify with schema.cue.
package correctionledgeranalytical

import "github.com/geoffmilleraz/agentic-kit/foundations/review/correction-ledger-analytical:correctionledgeranalytical"

// validCorrectionEvent demonstrates a well-formed analytical event from a correction.
validCorrectionEvent: correctionledgeranalytical.#AnalyticalEvent & {
	ID:      "correction:finding-001:1"
	Kind:    "correction"
	Target:  "report/section/revenue.q4.v3"
	At:      "2024-11-01T10:30:00Z"
	Version: 1
}

// validDismissalEvent demonstrates a well-formed analytical event from a dismissal.
validDismissalEvent: correctionledgeranalytical.#AnalyticalEvent & {
	ID:      "dismissal:finding-002:1"
	Kind:    "dismissal"
	Target:  "finding-002"
	At:      "2024-11-02T14:15:00Z"
	Version: 1
}

// validHighVersionEvent demonstrates a version > 1 (second event on the same target).
validHighVersionEvent: correctionledgeranalytical.#AnalyticalEvent & {
	ID:      "correction:finding-003:2"
	Kind:    "correction"
	Target:  "report/section/costs.q1.v2"
	At:      "2024-11-03T08:00:00Z"
	Version: 2
}

// validKinds verifies all kind enum values are accepted.
_k1: correctionledgeranalytical.#AnalyticalEventKind & "correction"
_k2: correctionledgeranalytical.#AnalyticalEventKind & "dismissal"
