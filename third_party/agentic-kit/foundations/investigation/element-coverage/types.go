// Package elementcoverage implements the element coverage board for the
// investigation engine. It decomposes a claim or legal theory into named
// elements and reports, per element, whether evidence exists and at what
// epistemic strength — distinct from source-coverage (which tracks ingestion
// completeness) and convergence (which tracks finding agreement).
//
// CUE contracts live in schema.cue; this file is the Go counterpart and must
// stay in sync with that schema. CUE is authoritative for shape; Go is
// authoritative for behaviour.
//
// Relationship to sibling foundations:
//   - evidence-grade: GradeClaim drives the per-element assessment.
//   - investigation/coverage: coverage tracks "have we looked everywhere?";
//     element-coverage tracks "does what we found support every required element?"
//
// Concurrency contract: all exported types are plain value types with no
// mutable state; Assess is a pure function safe to call concurrently from
// multiple goroutines.
package elementcoverage

import (
	"fmt"

	evidencegrade "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-grade"
)

// Status is the coverage outcome for a single element. It is a validated
// string enum; only the three named constants are valid.
//
// CUE mirror: #Status in schema.cue.
type Status string

const (
	// StatusMet means the element is supported by evidence at or above the
	// required tier with confidence at or above the minimum threshold.
	StatusMet Status = "met"

	// StatusUnderEvidenced means some evidence exists for the element but it
	// falls below the required tier or minimum confidence threshold.
	StatusUnderEvidenced Status = "under_evidenced"

	// StatusUnmet means no evidence was submitted for the element at all.
	StatusUnmet Status = "unmet"
)

var validStatuses = map[Status]struct{}{
	StatusMet:            {},
	StatusUnderEvidenced: {},
	StatusUnmet:          {},
}

// Validate returns an error if s is not one of the three recognised statuses.
func (s Status) Validate() error {
	if _, ok := validStatuses[s]; !ok {
		return fmt.Errorf("elementcoverage: unknown status %q; must be one of met|under_evidenced|unmet", string(s))
	}
	return nil
}

// Element is a named component of a theory or claim that must be supported by
// evidence. Examples in legal theory: "duty", "breach", "causation", "damages".
//
// CUE mirror: #Element in schema.cue.
type Element struct {
	// ID is the unique identifier for this element within the theory. Must be
	// non-empty.
	ID string

	// Name is the human-readable label for the element (e.g. "Breach of duty").
	// Must be non-empty.
	Name string

	// Required flags whether the element is a necessary component of the theory.
	// Optional elements (Required=false) are still assessed and reported; they
	// do not change the coverage Status semantics.
	Required bool
}

// Validate returns an error if the Element is malformed.
func (e Element) Validate() error {
	if e.ID == "" {
		return fmt.Errorf("elementcoverage: Element.ID must not be empty")
	}
	if e.Name == "" {
		return fmt.Errorf("elementcoverage: Element.Name must not be empty (ID=%q)", e.ID)
	}
	return nil
}

// ElementCoverage is the assessment result for a single element.
//
// CUE mirror: #ElementCoverage in schema.cue.
type ElementCoverage struct {
	// ElementID matches Element.ID.
	ElementID string

	// Status is the coverage outcome: met | under_evidenced | unmet.
	Status Status

	// WeakestTier is the floor tier produced by GradeClaim for this element.
	// Empty string when no evidence was submitted (Status == StatusUnmet).
	WeakestTier evidencegrade.Tier

	// Confidence is the aggregate confidence score from GradeClaim in [0, 1].
	// Zero when no evidence was submitted.
	Confidence float64

	// EvidenceSpanIDs lists every SpanID in the evidence provided for this
	// element, in submission order. Never nil when evidence was provided.
	EvidenceSpanIDs []string

	// Gap is a human-readable description of the weakest link preventing a
	// "met" verdict. Empty when Status == StatusMet.
	Gap string
}
