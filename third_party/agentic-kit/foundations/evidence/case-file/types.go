// Package casefile provides a deterministic case-file aggregation primitive
// for the evidence foundation.
//
// A CaseFile is a pure projection of everything believed about a single
// proposition, assembled from caller-supplied inputs (assertions, conflicts,
// authenticity assessments, narrative shifts). The same inputs always produce
// byte-identical output: no side-effects, no I/O, no clocks.
//
// # Aggregation Math
//
// Stance partitioning: each Assertion is placed in exactly one bucket based on
// its Stance:
//   - affirms  → Supporting
//   - denies   → Contradicting
//   - withdraws | recants → Retracting
//   - qualifies → Qualifying
//   - does_not_recall | declines → not counted in support weights
//
// TierRank is the numeric tier rank from evidencegrade.Tier.Rank():
//
//	direct=5, documentary=4, circumstantial=3, testimonial=2, inferred=1, ungraded=0.
//
// # SupportSummary computation
//
//	supportingWeight   = sum of TierRank for all affirming assertions
//	contradictingWeight = sum of TierRank for all denying assertions
//	maxPossible = (AffirmingCount + DenyingCount) * 5
//	SupportScore = supportingWeight / maxPossible  (0 when maxPossible == 0)
//	AgainstScore = contradictingWeight / maxPossible  (0 when maxPossible == 0)
//	net = SupportScore - AgainstScore
//	  net >  0.60 → "strong_support"
//	  net >  0.25 → "moderate_support"
//	  net > -0.25 → "contested"
//	  net > -0.60 → "moderate_against"
//	  else        → "strong_against"
//	  maxPossible == 0 → "unresolved"
//
// Retractions and qualifications are visible in their counts but do not move
// the SupportScore/AgainstScore — they hedge the evidence without adding
// directional weight.
//
// # Concurrency contract
//
// Build is a pure function with no shared mutable state; it is safe to call
// from multiple goroutines concurrently. CaseFile, GradedAssertion, ShiftDigest,
// AuthenticityDigest, and SupportSummary are plain value types; they may be
// read concurrently without additional synchronization.
package casefile

import (
	assertion "github.com/geoffmilleraz/agentic-kit/foundations/evidence/assertion"
	authenticity "github.com/geoffmilleraz/agentic-kit/foundations/evidence/authenticity"
	conflict "github.com/geoffmilleraz/agentic-kit/foundations/evidence/conflict"
	evidencegrade "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-grade"
	narrativeshift "github.com/geoffmilleraz/agentic-kit/foundations/investigation/narrative-shift"
)

// SupportTier classifies the overall support stance for a proposition computed
// from the graded, stance-partitioned assertion set.
//
// CUE mirror: #SupportTier in schema.cue.
type SupportTier string

const (
	// SupportTierStrongSupport means affirming weight dominates and the average
	// tier is high (net score > 0.60).
	SupportTierStrongSupport SupportTier = "strong_support"

	// SupportTierModerateSupport means affirming weight dominates but grade tier
	// is lower or mixed (net score 0.25..0.60].
	SupportTierModerateSupport SupportTier = "moderate_support"

	// SupportTierContested means affirming and contradicting weights are within a
	// small band, or are equal (net score -0.25..0.25].
	SupportTierContested SupportTier = "contested"

	// SupportTierModerateAgainst means contradicting weight dominates but grade
	// tier is lower or mixed (net score -0.60..-0.25].
	SupportTierModerateAgainst SupportTier = "moderate_against"

	// SupportTierStrongAgainst means contradicting weight dominates and the
	// average tier is high (net score <= -0.60).
	SupportTierStrongAgainst SupportTier = "strong_against"

	// SupportTierUnresolved means there is insufficient graded evidence
	// (no affirming or denying assertions) to determine a position.
	SupportTierUnresolved SupportTier = "unresolved"
)

// GradedAssertion pairs an assertion reference with its resolved epistemic
// grade. It is a compact projection of assertion.Assertion; the caller
// supplies the full Assertion values and Build extracts the fields needed for
// aggregation and display.
//
// CUE mirror: #GradedAssertion in schema.cue.
type GradedAssertion struct {
	// AssertionID is the stable identifier of the underlying Assertion.
	AssertionID string

	// Declarant is the EntityID of the real-world person who made the assertion.
	Declarant string

	// Stance is the declarant's relationship to the proposition.
	Stance assertion.Stance

	// Content is the actual asserted value or verbatim text.
	Content string

	// Tier is the resolved epistemic tier label for this assertion.
	Tier evidencegrade.Tier

	// TierRank is the numeric tier rank (1..5, 0 = ungraded) used for
	// aggregation. Matches evidencegrade.Tier.Rank().
	TierRank int
}

// AuthenticityDigest is a concise, read-only summary of an authenticity
// assessment for a source record cited by an assertion in this case file.
// It omits the full Signal slice for compactness.
//
// CUE mirror: #AuthenticityDigest in schema.cue.
type AuthenticityDigest struct {
	// RecordID identifies the source record that was assessed.
	RecordID string

	// Overall is the aggregate authenticity verdict.
	Overall authenticity.Verdict

	// Confidence is the aggregate confidence in Overall, in [0, 1].
	Confidence float64

	// Conservative is true when the overall verdict was driven by a credible
	// adverse signal (conservative aggregation rule from authenticity.Assess).
	Conservative bool
}

// ShiftDigest is a compact record of a single detected narrative shift,
// carrying assertion IDs and timing rather than the full Assertion structs.
// The full NarrativeShift is available from narrativeshift.Detect; this
// digest is the CUE-representable subset included in a CaseFile.
//
// CUE mirror: #ShiftDigest in schema.cue.
type ShiftDigest struct {
	// Declarant is the EntityID of the real-world person whose story changed.
	Declarant string

	// PropositionID is the proposition both assertions address.
	PropositionID string

	// FromID is the ID of the earlier (prior) assertion.
	FromID string

	// ToID is the ID of the later (changed) assertion.
	ToID string

	// Kind classifies the type of story change.
	Kind narrativeshift.ShiftKind

	// GapDays is the number of whole calendar days between the two assertions.
	// Always >= 0.
	GapDays int
}

// SupportSummary carries the aggregated evidence stance for a proposition.
// See the package-level doc for the exact aggregation math.
//
// CUE mirror: #SupportSummary in schema.cue.
type SupportSummary struct {
	// SupportScore is the fraction of maximum possible weight from affirming
	// assertions. Range [0, 1].
	SupportScore float64

	// AgainstScore is the fraction of maximum possible weight from denying
	// assertions. Range [0, 1].
	AgainstScore float64

	// Tier classifies the net stance based on SupportScore − AgainstScore.
	Tier SupportTier

	// AffirmingCount is the number of affirming assertions (Stance: affirms).
	AffirmingCount int

	// DenyingCount is the number of denying assertions (Stance: denies).
	DenyingCount int

	// RetractingCount is the number of retracting assertions
	// (Stance: withdraws | recants).
	RetractingCount int

	// QualifyingCount is the number of qualifying assertions
	// (Stance: qualifies).
	QualifyingCount int
}

// CaseFile is the deterministic, read-only projection of everything believed
// about a single proposition. Build constructs it from caller-supplied inputs;
// the same inputs always produce byte-identical output.
//
// CaseFile is a pure value type: it carries no unexported state and can be
// safely copied, marshalled, and read concurrently.
//
// CUE mirror: #CaseFile in schema.cue.
type CaseFile struct {
	// PropositionID is the stable identifier of the proposition this file
	// summarises. Non-empty.
	PropositionID string

	// Supporting is the slice of graded assertions whose Stance is "affirms".
	// Sorted by (TierRank desc, AssertionID asc). Nil when none.
	Supporting []GradedAssertion

	// Contradicting is the slice of graded assertions whose Stance is "denies".
	// Sorted by (TierRank desc, AssertionID asc). Nil when none.
	Contradicting []GradedAssertion

	// Retracting is the slice of graded assertions whose Stance is a retraction
	// (withdraws | recants). Sorted by (TierRank desc, AssertionID asc). Nil when none.
	Retracting []GradedAssertion

	// Qualifying is the slice of graded assertions whose Stance is "qualifies".
	// Sorted by (TierRank desc, AssertionID asc). Nil when none.
	Qualifying []GradedAssertion

	// Conflicts is the detected conflict list for this proposition, filtered to
	// those whose Subject matches PropositionID. Sorted by (Kind, A.ID, B.ID). Nil when none.
	Conflicts []conflict.Conflict

	// AuthenticityDigests is the condensed set of authenticity assessments for
	// evidence records cited by assertions in this case file.
	// Sorted by RecordID. Nil when none.
	AuthenticityDigests []AuthenticityDigest

	// ShiftHistory is the list of narrative shift digests for declarants who
	// made assertions on this proposition.
	// Sorted by (Declarant, FromID). Nil when none.
	ShiftHistory []ShiftDigest

	// SupportSummary is the computed aggregate stance. Always populated.
	SupportSummary SupportSummary
}

// Inputs carries the caller-supplied evidence collections from which Build
// assembles a CaseFile. All fields are optional (nil or empty is valid); Build
// ignores assertions whose PropositionID does not match the target proposition.
type Inputs struct {
	// Assertions is the full set of assertions across all propositions. Build
	// filters to those with PropositionID == target.
	Assertions []assertion.Assertion

	// Conflicts is the full set of detected conflicts. Build filters to those
	// whose Subject matches the target proposition ID.
	Conflicts []conflict.Conflict

	// Assessments is the set of authenticity assessments for source records
	// referenced by assertions. Build includes digests for records cited via
	// Assertion.StatedIn.
	Assessments []authenticity.Assessment

	// Shifts is the full narrative-shift output from narrativeshift.Detect
	// (or any compatible source). Build filters to those whose PropositionID
	// matches the target proposition.
	Shifts []narrativeshift.NarrativeShift
}
