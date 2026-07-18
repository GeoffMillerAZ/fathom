// valid.cue — example EvidenceItem and Decision values that satisfy their
// respective CUE definitions. Used by `cue vet ./...`.
package dadinvestigator

import "github.com/geoffmilleraz/agentic-kit/foundations/investigation/dad-investigator:dadinvestigator"

// HighWeightEvidence is a valid, high-weight evidence item.
HighWeightEvidence: dadinvestigator.#EvidenceItem & {
	ID:      "ev-001"
	Content: "Service X returned 503 errors for 12% of requests between 14:00 and 14:30."
	Weight:  10
}

// LowWeightEvidence is a valid, low-weight supplementary evidence item.
LowWeightEvidence: dadinvestigator.#EvidenceItem & {
	ID:      "ev-002"
	Content: "No configuration changes were deployed in the 24 hours prior to the incident."
	Weight:  2
}

// HighConfidenceDecision is a valid terminal Decision.
HighConfidenceDecision: dadinvestigator.#Decision & {
	Finding:    "Root cause identified: database connection pool exhausted under load."
	Confidence: "high"
	Terminal:   true
}

// LowConfidenceDecision is a valid non-terminal Decision.
LowConfidenceDecision: dadinvestigator.#Decision & {
	Finding:    "Initial signal detected; further evidence required."
	Confidence: "low"
	Terminal:   false
}
