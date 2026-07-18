// Package evidencegrade defines the tier taxonomy and claim-grading primitives
// for the evidence foundation. It is the #1 truth-seeking primitive in the kit.
//
// CUE contracts live in schema.cue; this file and grade.go are the Go
// counterparts and must stay in sync with that schema. CUE is authoritative
// for shape; Go is authoritative for behaviour.
//
// Backward-compat note: existing Confidence float64 consumers map to
// Grade{Tier: <chosen default>, Confidence: x}.
//
// Concurrency contract: Grade and GradedSpan are plain value types with no
// mutable state; values are safe to read concurrently. GradeClaim is a pure
// function and may be called from multiple goroutines simultaneously.
package evidencegrade

import "fmt"

// Tier classifies an evidence span by its epistemic strength.
//
// CUE mirror: #Tier in schema.cue.
// Order (strongest → weakest): Direct > Documentary > Circumstantial >
// Testimonial > Inferred.
type Tier string

const (
	// TierDirect is first-hand, unmediated observation (e.g. sensor reading,
	// cryptographic proof, witnessed event). Strongest tier.
	TierDirect Tier = "direct"

	// TierDocumentary is a record created at or near the time of the event
	// (e.g. contemporaneous email, log entry, signed contract).
	TierDocumentary Tier = "documentary"

	// TierCircumstantial is evidence that requires an inference step to connect
	// to the claim (e.g. pattern of behaviour, corroborating metadata).
	TierCircumstantial Tier = "circumstantial"

	// TierTestimonial is a statement by a human witness (deposition, interview,
	// affidavit). Subject to memory and bias.
	TierTestimonial Tier = "testimonial"

	// TierInferred is evidence produced by AI reasoning or probabilistic
	// inference with no direct observational anchor. Weakest tier.
	// AI-inferred evidence can never lift a claim above this tier alone.
	TierInferred Tier = "inferred"
)

// validTiers is the canonical ordered set for validation.
var validTiers = map[Tier]int{
	TierDirect:         5,
	TierDocumentary:    4,
	TierCircumstantial: 3,
	TierTestimonial:    2,
	TierInferred:       1,
}

// Rank returns the numeric strength of the tier (5=strongest, 1=weakest, 0=unknown).
// This is the canonical ordering used by GradeClaim.
func (t Tier) Rank() int {
	if r, ok := validTiers[t]; ok {
		return r
	}
	return 0
}

// Validate returns an error if t is not one of the five recognised tiers.
func (t Tier) Validate() error {
	if _, ok := validTiers[t]; !ok {
		return fmt.Errorf("evidencegrade: unknown tier %q; must be one of direct|documentary|circumstantial|testimonial|inferred", string(t))
	}
	return nil
}

// StrongerThan reports whether t is epistemically stronger than o.
// An unknown tier is never stronger than anything.
func (t Tier) StrongerThan(o Tier) bool {
	return t.Rank() > o.Rank()
}

// Grade pairs a tier with a numeric confidence score.
//
// CUE mirror: #Grade in schema.cue.
type Grade struct {
	Tier       Tier
	Confidence float64
}

// Validate returns an error if any field is invalid.
// Rules:
//   - Tier must be one of the five recognised values.
//   - Confidence must be in [0, 1].
func (g Grade) Validate() error {
	if err := g.Tier.Validate(); err != nil {
		return err
	}
	if g.Confidence < 0 || g.Confidence > 1 {
		return fmt.Errorf("evidencegrade: Confidence %g is outside [0, 1]", g.Confidence)
	}
	return nil
}

// GradedSpan annotates a reference to an evidence span with its grading
// attributes for the purpose of evaluating a specific claim.
//
// CUE mirror: #GradedSpan in schema.cue.
type GradedSpan struct {
	// SpanID references the span in an evidence-span registry. Must be non-empty.
	SpanID string

	// Tier is the epistemic strength of this span.
	Tier Tier

	// Confidence is the certainty of the span's claim in [0, 1].
	Confidence float64

	// Necessary is true when the claim logically depends on this span.
	// Necessary spans determine the FloorTier of the resulting ClaimGrade.
	Necessary bool

	// Independent is true when this span corroborates the claim from a source
	// that is epistemically independent of all other spans (no shared lineage).
	// Independent corroborating spans can raise confidence and, under
	// PromotionRule, raise the tier.
	Independent bool
}

// Validate returns an error if any field is invalid.
func (gs GradedSpan) Validate() error {
	if gs.SpanID == "" {
		return fmt.Errorf("evidencegrade: GradedSpan.SpanID must not be empty")
	}
	if err := gs.Tier.Validate(); err != nil {
		return err
	}
	if gs.Confidence < 0 || gs.Confidence > 1 {
		return fmt.Errorf("evidencegrade: GradedSpan %q Confidence %g is outside [0, 1]", gs.SpanID, gs.Confidence)
	}
	if gs.Necessary && gs.Independent {
		return fmt.Errorf("evidencegrade: GradedSpan %q cannot be both Necessary and Independent: necessary spans set the floor; independent spans corroborate — a span is one role or the other", gs.SpanID)
	}
	return nil
}
