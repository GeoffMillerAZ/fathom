// Package assertion defines the speech-act primitive for the evidence
// foundation. See stance.go for the Stance type and types.go for Assertion.
//
// CUE contracts live in schema.cue; this file is the Go counterpart for the
// Knowledge type and must stay in sync with that schema.
//
// Concurrency contract: all exported types are plain value types with no
// mutable state; values are safe to read concurrently without additional
// synchronization.
package assertion

import "fmt"

// Knowledge classifies how the declarant came to know the asserted fact.
// It captures epistemic provenance — first-hand observation, reported
// (hearsay), or held belief — which is critical for legal admissibility
// analysis and evidence weighting.
//
// CUE mirror: #Knowledge in schema.cue.
type Knowledge string

const (
	// KnowledgeFirstHand means the declarant directly observed or experienced
	// the asserted fact themselves.
	KnowledgeFirstHand Knowledge = "first_hand"

	// KnowledgeReported means the declarant learned of the fact through another
	// person's statement. This is the hearsay marker.
	KnowledgeReported Knowledge = "reported"

	// KnowledgeBelief means the declarant holds the assertion as a belief or
	// inference rather than direct observation or reported statement.
	KnowledgeBelief Knowledge = "belief"
)

// validKnowledge is the canonical set for validation.
var validKnowledge = map[Knowledge]struct{}{
	KnowledgeFirstHand: {},
	KnowledgeReported:  {},
	KnowledgeBelief:    {},
}

// Validate returns an error if k is not one of the recognised Knowledge values.
func (k Knowledge) Validate() error {
	if _, ok := validKnowledge[k]; !ok {
		return fmt.Errorf("assertion: unknown knowledge %q; must be one of first_hand|reported|belief", string(k))
	}
	return nil
}
