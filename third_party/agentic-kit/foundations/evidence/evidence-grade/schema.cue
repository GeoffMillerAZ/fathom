// Package evidencegrade defines the CUE-first contract for evidence tier
// taxonomy and claim-grading rules. Go types in types.go and grade.go mirror
// these definitions; any drift is a bug.
//
// Invariant (from CLAUDE.md §4): a claim's grade tier ≤ the lowest (weakest)
// tier among its necessary spans. AI-inferred evidence alone can never lift a
// claim above "inferred".
package evidencegrade

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #Tier classifies an evidence span by its epistemic strength.
// Ordered strongest → weakest:
//
//	direct > documentary > circumstantial > testimonial > inferred
#Tier: "direct" | "documentary" | "circumstantial" | "testimonial" | "inferred"

// #Grade pairs a tier with a numeric confidence score.
//
// Backward-compat note: existing Confidence float64 consumers map to
// Grade{Tier: <chosen default>, Confidence: x}.
#Grade: {
	Tier:       #Tier
	Confidence: >=0 & <=1
}

// #GradedSpan annotates a reference to an evidence span with its grading
// attributes for the purpose of evaluating a specific claim.
//
//   - Necessary: the claim depends on this span (required support).
//   - Independent: corroborates the claim from a source that is epistemically
//     independent of all other necessary spans (no shared lineage).
//
// Disjointness invariant: a span is exactly one of {Necessary, Independent,
// neither}. It cannot be both (enforced by Go Validate() and the constraint below).
#GradedSpan: {
	SpanID:      _nonEmpty
	Tier:        #Tier
	Confidence:  >=0 & <=1
	Necessary:   bool
	Independent: bool

	// Necessary XOR Independent: both true simultaneously is invalid.
	// A necessary span sets the FloorTier; an independent span corroborates.
	// Encoding: !(Necessary && Independent) — CUE cannot express XOR directly,
	// so we encode the forbidden conjunction as a constraint.
	_necessaryXorIndependent: Necessary == false || Independent == false
}

// #PromotionRule controls whether GradeClaim may promote the result tier above
// the FloorTier when sufficient independent corroboration exists.
//
// MinCorroboratorConfidence defaults to 0, which is treated as "> 0 required" by
// the Go implementation — a zero-confidence corroborator never qualifies for
// tier promotion regardless of its nominal tier strength.
#PromotionRule: {
	AllowTierPromotion:            bool
	MinIndependentSourcesForPromotion: >=0
	MinCorroboratorConfidence:     >=0 & <=1
}

// #ClaimGrade is the result of evaluating a set of GradedSpans against a claim.
//
//   - Grade.Tier is the effective tier after any promotion.
//   - FloorTier is the weakest tier among necessary spans.
//   - Promoted flags whether tier was raised above FloorTier.
//   - DroppedSpanIDs lists spans that were neither Necessary nor Independent;
//     callers must surface these (invariant §7: no silent truncation).
//   - AIContributedConfidence records confidence added solely by TierInferred
//     corroborators (invariant §3: AI contributions must be labelled).
#ClaimGrade: {
	Grade:                   #Grade
	Promoted:                bool
	PromotionReason:         string
	FloorTier:               #Tier
	DroppedSpanIDs:          [...string]
	AIContributedConfidence: >=0 & <=1
}
