// Package concernflags — valid test data that must unify with schema.cue.
package concernflags

import "github.com/geoffmilleraz/agentic-kit/foundations/review/concern-flags:concernflags"

// validFlagLow demonstrates a low-severity accuracy concern.
validFlagLow: concernflags.#ConcernFlag & {
	ID:        "flag-low-001"
	Type:      "accuracy"
	Severity:  "low"
	Target:    "doc/section/revenue"
	Rationale: "Minor rounding discrepancy in revenue table."
}

// validFlagCritical demonstrates a critical-severity DLP concern.
validFlagCritical: concernflags.#ConcernFlag & {
	ID:        "flag-critical-001"
	Type:      "dlp"
	Severity:  "critical"
	Target:    "doc/section/pii"
	Rationale: "PII detected in output; must be redacted before publication."
}

// validSeverities verifies all enum values are accepted.
_s1: concernflags.#Severity & "low"
_s2: concernflags.#Severity & "medium"
_s3: concernflags.#Severity & "high"
_s4: concernflags.#Severity & "critical"
