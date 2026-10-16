// Package conflict provides a legal-grade conflict and contradiction taxonomy
// for graded, provenanced, declarant-bearing assertions and evidence spans.
//
// It is intentionally separate from evidence-graph, which operates on an
// impoverished span type and performs no tier- or declarant-aware reasoning.
// This package operates on [assertion.Assertion] and [evidencespan.Span] and
// produces a typed taxonomy with deterministic severity.
//
// # claim_vs_evidence Matching Convention (Detect)
//
// An Assertion references a proposition; a Span references an (EntityID, Field)
// pair. There is no Field on an Assertion. The matching key chosen here is:
//
//	Assertion.PropositionID == Span.EntityID
//
// When that key matches AND Assertion.Content != Span.Value AND the assertion
// is not a retraction, a [KindClaimVsEvidence] conflict is emitted.
// Rationale: the PropositionID is the subject anchor (see assertion.Proposition.Subject);
// treating it as equivalent to Span.EntityID is the simplest, fully
// deterministic convention that produces meaningful matches without requiring
// a semantic field-level mapping that does not exist in the schema.
// The Subject field on Proposition is not available on Assertion directly, so
// PropositionID is used as the stable subject identifier.
//
// # claim_vs_evidence Matching Convention (DetectWithPropositions)
//
// When the caller has the full [assertion.Proposition] slice available,
// [DetectWithPropositions] performs the structurally correct join:
//
//	Proposition.Subject == Span.EntityID
//	Assertion.PropositionID == Proposition.ID
//
// This handles the common case where PropositionID and Span.EntityID belong to
// different namespaces (e.g. "prop-{hex}" vs record IDs). Without the
// Proposition bridge the join always produces zero matches — a silent false
// negative. The result carries a [Coverage] value that surfaces join statistics
// so callers can detect a degenerate (zero-join despite non-empty inputs) state
// without relying on an empty conflict list alone.
//
// # Severity Formula
//
// Severity is a float64 in [0, 1] computed from the tier ranks of the two
// sides using the formula:
//
//	maxRank  = max(rankA, rankB)    // 1..5  (direct=5, inferred=1)
//	minRank  = min(rankA, rankB)
//	severity = (maxRank/5.0) * 0.7 + (minRank/5.0) * 0.3
//
// Interpretation:
//   - The stronger side drives 70% of the score — a direct/documentary source
//     contradicting anything scores high regardless of the other side's tier.
//   - The weaker side contributes 30% — two strong sides score higher than
//     strong-vs-weak.
//   - A DIRECT (rank 5) vs TESTIMONIAL (rank 2) pair: (5/5)*0.7+(2/5)*0.3 = 0.82.
//   - Two TESTIMONIAL (rank 2) assertions: (2/5)*0.7+(2/5)*0.3 = 0.40.
//   - Ungraded sides use rank 0; their severity contribution is 0.
//
// For qualifications (KindQualification) and recantations (KindRecantation)
// the same formula is applied to Grade/Via-tier ranks where available. When
// neither Grade nor Via is set, the rank is treated as 0.
//
// Concurrency contract: all exported types are plain value types with no
// mutable state; Detect and DetectWithPropositions are pure functions safe to
// call from multiple goroutines concurrently.
package conflict

import (
	assertion "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/assertion"
	evidencegrade "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-grade"
)

// ConflictKind classifies the type of conflict detected between two evidence
// items. All values are lowercase strings for readability in JSON and logs.
//
// CUE mirror: #ConflictKind in schema.cue.
type ConflictKind string

const (
	// KindClaimVsClaim is a contradiction between two different declarants
	// making incompatible assertions about the same proposition.
	KindClaimVsClaim ConflictKind = "claim_vs_claim"

	// KindClaimVsEvidence is a contradiction between an assertion and a
	// documentary/physical evidence span on the same subject.
	// See package-level doc for the matching convention.
	KindClaimVsEvidence ConflictKind = "claim_vs_evidence"

	// KindSelfInconsistency is a contradiction where the SAME declarant makes
	// incompatible assertions about the same proposition, neither of which is
	// a retraction.
	KindSelfInconsistency ConflictKind = "self_inconsistency"

	// KindRecantation is detected when a declarant issues a retraction
	// (StanceWithdraws or StanceRecants) on a proposition they previously
	// addressed.
	KindRecantation ConflictKind = "recantation"

	// KindQualification is a low-severity flag: a declarant qualifies
	// (StanceQualifies) a proposition that also has an affirming or denying
	// assertion from any declarant.
	KindQualification ConflictKind = "qualification"

	// KindPartial is reserved for partial overlaps not covered by the above
	// kinds (e.g. two spans that partially conflict on a range). Not emitted
	// by Detect in v1; available for external use.
	KindPartial ConflictKind = "partial"
)

// validKinds is the canonical set for Validate.
var validKinds = map[ConflictKind]struct{}{
	KindClaimVsClaim:      {},
	KindClaimVsEvidence:   {},
	KindSelfInconsistency: {},
	KindRecantation:       {},
	KindQualification:     {},
	KindPartial:           {},
}

// Validate returns an error if k is not one of the recognised ConflictKind values.
func (k ConflictKind) Validate() error {
	if _, ok := validKinds[k]; !ok {
		return &ValidationError{Field: "ConflictKind", Value: string(k)}
	}
	return nil
}

// RefKind discriminates which type of evidence item a Ref points at.
type RefKind string

const (
	// RefKindAssertion indicates the Ref points at an assertion.Assertion by ID.
	RefKindAssertion RefKind = "assertion"

	// RefKindSpan indicates the Ref points at an evidencespan.Span by ID.
	RefKindSpan RefKind = "span"
)

// Ref is a discriminated pointer at one side of a conflict.
//
// CUE mirror: #Ref in schema.cue.
type Ref struct {
	// Kind discriminates whether this ref points at an assertion or a span.
	// Must be "assertion" or "span".
	Kind RefKind

	// ID is the ID of the referenced item.
	ID string
}

// Conflict is a typed, graded conflict between two evidence items.
//
// A and B are ordered so that A.ID <= B.ID within each (Kind, Subject, Field)
// group, ensuring each unordered pair is reported exactly once.
//
// CUE mirror: #Conflict in schema.cue.
type Conflict struct {
	// Kind classifies the nature of the conflict.
	Kind ConflictKind

	// Subject is the shared anchor: the PropositionID (for assertion-vs-assertion
	// conflicts) or the Span.EntityID (for claim-vs-evidence conflicts).
	Subject string

	// Field is the Span.Field when one side is a span, otherwise empty.
	Field string

	// A is the first (lexicographically smaller or equal ID) side of the conflict.
	A Ref

	// B is the second side of the conflict.
	B Ref

	// Severity is a deterministic score in [0, 1] indicating the epistemic
	// weight of this conflict. See the package-level doc for the formula.
	Severity float64

	// Reason is a human-readable explanation of why this pair was flagged.
	Reason string
}

// Coverage reports the join statistics from a [DetectWithPropositions] call.
// All counts are non-negative. A Coverage value where Degenerate() returns true
// indicates that the inputs were non-empty but no proposition-to-span join could
// be formed — a strong signal that the ID schemes on each side are disjoint.
// The caller SHOULD inspect this value and surface an alarm or metric when
// Degenerate() is true; an empty Conflicts list paired with degenerate coverage
// means "the detector saw nothing" NOT "there are no conflicts".
//
// CUE mirror: #Coverage in schema.cue.
type Coverage struct {
	// AssertionCount is the number of assertions passed in.
	AssertionCount int

	// PropositionCount is the number of propositions passed in.
	PropositionCount int

	// SpanCount is the number of spans passed in.
	SpanCount int

	// PropositionSubjectsMatched is the number of distinct Proposition.Subject
	// values that matched at least one Span.EntityID.
	PropositionSubjectsMatched int

	// PropositionSubjectsUnmatched is the number of distinct Proposition.Subject
	// values that matched NO Span.EntityID.
	PropositionSubjectsUnmatched int

	// SpanEntityIDsMatched is the number of distinct Span.EntityID values that
	// matched at least one Proposition.Subject.
	SpanEntityIDsMatched int

	// SpanEntityIDsUnmatched is the number of distinct Span.EntityID values that
	// matched NO Proposition.Subject.
	SpanEntityIDsUnmatched int
}

// Degenerate returns true when inputs on both sides were non-empty yet NO join
// could be formed between Proposition.Subject values and Span.EntityID values.
// This is the canonical "silent zero-join" signal:
//
//	Coverage.Degenerate() == true
//	⟹ the caller's proposition ID scheme and span entity ID scheme are disjoint.
//
// Callers MUST treat a degenerate result as an operational alarm, not as
// "no conflicts found". An empty Conflicts list accompanied by a degenerate
// Coverage means the detector was blind, not that the evidence was clean.
func (c Coverage) Degenerate() bool {
	return c.PropositionCount > 0 &&
		c.SpanCount > 0 &&
		c.PropositionSubjectsMatched == 0 &&
		c.SpanEntityIDsMatched == 0
}

// DetectResult is the return value of [DetectWithPropositions]. It pairs the
// detected conflicts with the join-coverage diagnostics.
//
// CUE mirror: #DetectResult in schema.cue.
type DetectResult struct {
	// Conflicts is the detected conflict list, sorted deterministically
	// by (Kind, Subject, Field, A.ID, B.ID). Nil when no conflicts are found.
	Conflicts []Conflict

	// Coverage reports join statistics for the claim-vs-evidence pass.
	// Always populated (even when Conflicts is nil).
	Coverage Coverage
}

// ValidationError is returned by Validate when a field contains an unrecognised value.
type ValidationError struct {
	Field string
	Value string
}

// Error implements the error interface, naming the offending field and the
// unrecognised value it held.
func (e *ValidationError) Error() string {
	return "conflict: invalid " + e.Field + " value: " + e.Value
}

// pairKey is used to deduplicate unordered pairs within the same kind and subject.
type pairKey struct {
	kind    ConflictKind
	subject string
	field   string
	aID     string
	bID     string
}

// severity computes the deterministic severity score from two tier ranks.
// See the package-level formula documentation.
func severity(rankA, rankB int) float64 {
	maxR := rankA
	minR := rankB
	if rankB > rankA {
		maxR = rankB
		minR = rankA
	}
	return (float64(maxR)/5.0)*0.7 + (float64(minR)/5.0)*0.3
}

// viaRank returns the Rank() of a Via tier, or 0 if the tier is empty/invalid.
func viaRank(via evidencegrade.Tier) int {
	if via == "" {
		return 0
	}
	return via.Rank()
}

// effectiveTierRank returns the tier rank for an assertion, preferring the
// explicit Grade if present, falling back to Via.
func effectiveTierRank(a assertion.Assertion) int {
	if a.Grade != nil {
		return a.Grade.Tier.Rank()
	}
	return viaRank(a.Via)
}

// orderRefs returns (a, b) such that a.ID <= b.ID for canonical pair ordering.
func orderRefs(x, y Ref) (Ref, Ref) {
	if x.ID <= y.ID {
		return x, y
	}
	return y, x
}
