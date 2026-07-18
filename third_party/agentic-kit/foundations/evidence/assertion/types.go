// Package assertion defines the speech-act primitive for the evidence
// foundation. See stance.go for the Stance type.
//
// CUE contracts live in schema.cue; this file is the Go counterpart and must
// stay in sync with that schema.
//
// Concurrency contract: Assertion and Proposition are plain value types; values
// are safe to read concurrently. No mutable shared state exists in this package.
package assertion

import (
	"fmt"
	"time"

	evidencegrade "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-grade"
	evidencespan "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-span"
	aiprovenance "github.com/geoffmilleraz/agentic-kit/foundations/provenance/ai-provenance"
)

// Proposition is the normalized, subject-anchored statement that many
// declarants can each assert with different stances. It separates the
// semantic content of a claim from its speech-act wrapper.
//
// Example: "Alice was at the bar at 9 pm" is a single Proposition; Alice's
// deposition affirming it and Bob's interview denying it are two Assertions
// both referencing the same PropositionID.
//
// CUE mirror: #Proposition in schema.cue.
type Proposition struct {
	// ID uniquely identifies this proposition. Must be non-empty.
	ID string

	// Canonical is the normalized statement text (e.g. "Alice was at
	// The Tap Room on 2024-01-15 at 21:00 local time"). Must be non-empty.
	Canonical string

	// Subject is the EntityID or EventID this proposition is about.
	// Must be non-empty.
	Subject string
}

// Validate returns an error if any required field is missing.
func (p Proposition) Validate() error {
	if p.ID == "" {
		return fmt.Errorf("assertion: Proposition.ID must not be empty")
	}
	if p.Canonical == "" {
		return fmt.Errorf("assertion: Proposition.Canonical must not be empty")
	}
	if p.Subject == "" {
		return fmt.Errorf("assertion: Proposition.Subject must not be empty")
	}
	return nil
}

// Assertion is the speech-act record: a specific declarant taking a stance on
// a proposition at a particular time, captured from a particular source record.
//
// Design invariant: Declarant is the REAL-WORLD person or party who made the
// statement (e.g. witness "Alice"). Author is WHO OR WHAT logged this
// assertion into the system (e.g. "human:geoff" who typed up a deposition
// summary, or an AI extractor). These are explicitly distinct: Alice's
// deposition transcribed by a paralegal has Declarant="entity:alice" and
// Author=NewHuman("paralegal@firm.com"). This distinction is critical for
// provenance and impeachment reasoning.
//
// CUE mirror: #Assertion in schema.cue.
type Assertion struct {
	// ID uniquely identifies this assertion record. Must be non-empty.
	ID string

	// PropositionID references the Proposition being addressed.
	// Must be non-empty.
	PropositionID string

	// Declarant is the EntityID of the real-world person or party who made
	// the statement (e.g. a witness, party, or expert). Must be non-empty.
	// Distinct from Author — see package-level design note.
	Declarant string

	// Stance is the declarant's relationship to the proposition.
	Stance Stance

	// Content is the actual asserted value or verbatim text (e.g.
	// "I was at the bar at 9pm"). Must be non-empty.
	Content string

	// AsOf is the world-time: the moment in time that the declarant claims
	// the asserted fact is or was true. Required by ValidateAsEvidence.
	AsOf time.Time

	// StatedAt is the record-time of the statement: when the declarant
	// actually said it (e.g. the date of deposition or interview).
	// Required by ValidateAsEvidence.
	StatedAt time.Time

	// StatedIn is the RecordID of the source record containing this
	// statement (e.g. a deposition transcript record ID).
	// Required by ValidateAsEvidence.
	StatedIn string

	// Locator is an exact pointer into the StatedIn record for
	// click-traceability. Required by ValidateAsEvidence and must not be
	// Kind=="none".
	Locator evidencespan.Locator

	// Via is the evidence channel tier for this assertion. For witness
	// statements this is typically TierTestimonial.
	Via evidencegrade.Tier

	// Author records who or what recorded this assertion into the system.
	// This is NOT the declarant — see package-level design note.
	// Required by ValidateAsEvidence with non-empty Kind.
	Author aiprovenance.Author

	// Grade is the explicit epistemic grade of this assertion. Optional in
	// Validate; required by ValidateAsEvidence.
	Grade *evidencegrade.Grade

	// Knowledge classifies how the declarant came to know the asserted fact.
	// Optional (zero value "" means unset). When non-empty, must be a valid
	// Knowledge value. Use IsHearsay() to test for KnowledgeReported.
	Knowledge Knowledge

	// Role is a free-text procedural posture populated by the application layer
	// (e.g. the legal pack). The kit does not enumerate legal roles; any string
	// is accepted. Optional (zero value "" means unset).
	Role string
}

// Validate returns an error if any field fails the lenient rules.
// Rules:
//   - ID, PropositionID, Declarant, Content must be non-empty.
//   - Stance must be a recognised value.
//   - If Via is non-empty it must be a valid Tier.
//   - If Locator is non-zero it must pass Locator.Validate().
//   - If Author.Kind is non-empty it must pass Author.Validate().
//   - If Grade is non-nil it must pass Grade.Validate().
//   - If Knowledge is non-empty it must be a valid Knowledge value.
//   - Role is unconstrained.
//
// Provenance fields (AsOf, StatedAt, StatedIn) are NOT required here — use
// ValidateAsEvidence for full provenance enforcement.
func (a Assertion) Validate() error {
	if a.ID == "" {
		return fmt.Errorf("assertion: ID must not be empty")
	}
	if a.PropositionID == "" {
		return fmt.Errorf("assertion: PropositionID must not be empty")
	}
	if a.Declarant == "" {
		return fmt.Errorf("assertion: Declarant must not be empty")
	}
	if a.Content == "" {
		return fmt.Errorf("assertion: Content must not be empty")
	}
	if err := a.Stance.Validate(); err != nil {
		return err
	}
	if a.Via != "" {
		if err := a.Via.Validate(); err != nil {
			return fmt.Errorf("assertion: Via invalid: %w", err)
		}
	}
	if !a.Locator.IsZero() {
		if err := a.Locator.Validate(); err != nil {
			return err
		}
	}
	if a.Author.Kind != "" {
		if err := a.Author.Validate(); err != nil {
			return fmt.Errorf("assertion: Author invalid: %w", err)
		}
	}
	if a.Grade != nil {
		if err := a.Grade.Validate(); err != nil {
			return fmt.Errorf("assertion: Grade invalid: %w", err)
		}
	}
	if a.Knowledge != "" {
		if err := a.Knowledge.Validate(); err != nil {
			return fmt.Errorf("assertion: Knowledge invalid: %w", err)
		}
	}
	return nil
}

// IsHearsay reports whether this assertion was made from reported (hearsay)
// knowledge. Returns true only when Knowledge == KnowledgeReported.
func (a Assertion) IsHearsay() bool {
	return a.Knowledge == KnowledgeReported
}

// ValidateAsEvidence enforces the full Throughline provenance invariant (#1):
// "No provenance → not storable as evidence."
//
// It calls Validate() first (all lenient rules apply), then additionally
// requires:
//   - StatedIn non-empty (provenance invariant: source record required).
//   - Author valid with non-empty Kind.
//   - AsOf non-zero (world-time required).
//   - StatedAt non-zero (record-time required).
//   - Locator non-zero, valid, and Kind != "none" (must be a real pointer).
//   - Via must be a valid tier (non-empty).
//   - Grade non-nil and valid (explicit epistemic tier required).
//   - Knowledge, if non-empty, must be a valid Knowledge value.
//
// Call this before persisting an assertion to the append-only ledger.
func (a Assertion) ValidateAsEvidence() error {
	if err := a.Validate(); err != nil {
		return err
	}
	if a.StatedIn == "" {
		return fmt.Errorf("assertion: StatedIn required for evidence (provenance invariant)")
	}
	if a.Author.Kind == "" {
		return fmt.Errorf("assertion: Author.Kind must not be empty (provenance required)")
	}
	if err := a.Author.Validate(); err != nil {
		return fmt.Errorf("assertion: Author invalid: %w", err)
	}
	if a.AsOf.IsZero() {
		return fmt.Errorf("assertion: AsOf must not be zero (world-time required)")
	}
	if a.StatedAt.IsZero() {
		return fmt.Errorf("assertion: StatedAt must not be zero (record-time required)")
	}
	if a.Locator.IsZero() {
		return fmt.Errorf("assertion: Locator must not be zero (source pointer required)")
	}
	if err := a.Locator.Validate(); err != nil {
		return err
	}
	if a.Locator.Kind == "none" {
		return fmt.Errorf("assertion: Locator.Kind 'none' is not a real source pointer (evidence requires a locatable pointer)")
	}
	if a.Via == "" {
		return fmt.Errorf("assertion: Via must not be empty (evidence channel tier required)")
	}
	if err := a.Via.Validate(); err != nil {
		return fmt.Errorf("assertion: Via invalid: %w", err)
	}
	if a.Grade == nil {
		return fmt.Errorf("assertion: Grade must not be nil (explicit epistemic tier required)")
	}
	if err := a.Grade.Validate(); err != nil {
		return fmt.Errorf("assertion: Grade invalid: %w", err)
	}
	return nil
}
