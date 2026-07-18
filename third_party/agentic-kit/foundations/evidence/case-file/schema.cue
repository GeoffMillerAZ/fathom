// Package casefile defines the CUE-first contract for the case-file aggregation
// primitive. Go types in types.go and casefile.go mirror these definitions; any
// drift is a build failure (cue vet runs in pre-commit).
//
// A CaseFile is a deterministic projection of everything believed about a
// single proposition, assembled from caller-supplied evidence inputs. Same
// inputs always produce the same byte-identical projection (pure function,
// stable sort).
//
// Primary consumer: Throughline UI (reads CaseFile to render proposition panels).
// The projection is a read-only snapshot; it does not mutate any input.
package casefile

import (
	conflict "github.com/geoffmilleraz/agentic-kit/foundations/evidence/conflict:conflict"
)

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #SupportTier classifies the overall support stance computed from graded evidence.
//
// Values in rough descending order of net support for the proposition:
//   strong_support   — the weighted majority is affirming with high average grade tier
//   moderate_support — affirming weight dominates but grade tier is lower or mixed
//   contested        — affirming and contradicting weight are within a small band
//   moderate_against — contradicting weight dominates but grade tier is lower or mixed
//   strong_against   — the weighted majority is contradicting with high average grade tier
//   unresolved       — insufficient graded evidence to determine a position
#SupportTier:
	"strong_support" |
	"moderate_support" |
	"contested" |
	"moderate_against" |
	"strong_against" |
	"unresolved"

// #GradedAssertion pairs an assertion reference with its resolved epistemic grade.
#GradedAssertion: {
	// AssertionID is the stable identifier of the underlying Assertion.
	AssertionID: _nonEmpty

	// Declarant is the EntityID of the real-world person who made the assertion.
	Declarant: _nonEmpty

	// Stance is the declarant's relationship to the proposition
	// ("affirms" | "denies" | "qualifies" | "withdraws" | "recants" | "does_not_recall" | "declines").
	Stance: _nonEmpty

	// Content is the actual asserted value or verbatim text.
	Content: _nonEmpty

	// Tier is the resolved epistemic tier label for this assertion.
	// One of: direct | documentary | circumstantial | testimonial | inferred | ungraded.
	Tier: _nonEmpty

	// TierRank is the numeric tier rank (1..5, 0 = ungraded) used for
	// aggregation. Matches evidencegrade.Tier.Rank().
	TierRank: int & >=0 & <=5
}

// #AuthenticityDigest is a concise, read-only summary of an authenticity
// assessment attached to the case file. It omits the full Signal slice (which
// can be large) and carries only the fields needed for display and downstream
// reasoning.
#AuthenticityDigest: {
	// RecordID identifies the source record that was assessed.
	RecordID: _nonEmpty

	// Overall is the aggregate authenticity verdict.
	// One of: authentic | likely_authentic | inconclusive | likely_synthetic | synthetic | tampered.
	Overall: _nonEmpty

	// Confidence is the aggregate confidence in Overall, in [0, 1].
	Confidence: >=0 & <=1

	// Conservative is true when the overall verdict was driven by a credible
	// adverse signal (conservative aggregation).
	Conservative: bool
}

// #ShiftDigest is a compact, CUE-representable record of a single detected
// narrative shift. It carries assertion IDs and timing rather than the full
// Assertion structs so that the CUE schema remains self-contained.
//
// Go mirror: ShiftDigest in types.go. The Go casefile package extracts these
// from the full narrativeshift.NarrativeShift values produced by
// narrativeshift.Detect.
#ShiftDigest: {
	// Declarant is the EntityID of the real-world person whose story changed.
	Declarant: _nonEmpty

	// PropositionID is the proposition both assertions address.
	PropositionID: _nonEmpty

	// FromID is the ID of the earlier (prior) assertion.
	FromID: _nonEmpty

	// ToID is the ID of the later (changed) assertion.
	ToID: _nonEmpty

	// Kind classifies the type of story change.
	// One of: recantation | stance_change | content_change | escalation.
	Kind: _nonEmpty

	// GapDays is the number of whole calendar days between the two assertions.
	// Always >= 0.
	GapDays: int & >=0
}

// #SupportSummary carries the aggregated evidence stance and the inputs that
// produced it. Callers can re-derive the summary from the inputs; the fields
// here are provided for rendering convenience.
//
// Aggregation math (documented for auditability — no magic scores):
//
//   For each GradedAssertion with Stance "affirms":
//     supportingWeight += TierRank      (each affirming assertion adds its tier rank)
//
//   For each GradedAssertion with Stance "denies":
//     contradictingWeight += TierRank   (each denying assertion adds its tier rank)
//
//   Retractions (Stance withdraws | recants) and qualifications (Stance qualifies)
//   are counted separately and DO NOT contribute to supportingWeight or
//   contradictingWeight — they would dilute the signal without adding directional
//   weight. Callers can inspect RetractingCount and QualifyingCount to understand
//   how many assertions took non-affirmative stances.
//
//   MaxPossibleWeight = (AffirmingCount + DenyingCount) * 5
//     (if every assertion were direct-tier, this is the ceiling)
//
//   SupportScore = supportingWeight / MaxPossibleWeight   when MaxPossibleWeight > 0, else 0.0
//   AgainstScore = contradictingWeight / MaxPossibleWeight  when MaxPossibleWeight > 0, else 0.0
//
//   Tier classification (applied to SupportScore and AgainstScore):
//     net = SupportScore - AgainstScore
//     net >  0.60 → "strong_support"
//     net >  0.25 → "moderate_support"
//     net > -0.25 → "contested"          (includes net == 0)
//     net > -0.60 → "moderate_against"
//     else        → "strong_against"
//     unresolved: when MaxPossibleWeight == 0 (no affirming or denying assertions)
#SupportSummary: {
	// SupportScore is the fraction of maximum possible weight from affirming assertions.
	// Range [0, 1]. 0 means no affirming assertions (or all ungraded).
	SupportScore: >=0 & <=1

	// AgainstScore is the fraction of maximum possible weight from denying assertions.
	// Range [0, 1]. 0 means no denying assertions (or all ungraded).
	AgainstScore: >=0 & <=1

	// Tier classifies the net stance based on SupportScore − AgainstScore.
	Tier: #SupportTier

	// AffirmingCount is the number of affirming assertions (Stance: affirms).
	AffirmingCount: int & >=0

	// DenyingCount is the number of denying assertions (Stance: denies).
	DenyingCount: int & >=0

	// RetractingCount is the number of retracting assertions (Stance: withdraws | recants).
	RetractingCount: int & >=0

	// QualifyingCount is the number of qualifying assertions (Stance: qualifies).
	QualifyingCount: int & >=0
}

// #CaseFile is the deterministic, read-only projection of everything believed
// about a single proposition at a point in time. It is assembled from
// caller-supplied slices (assertions, grades, conflicts, authenticity signals,
// narrative shifts) and is never stored — callers re-derive it on demand.
//
// Same inputs, byte-identical output (stable sort throughout). This is a
// projection function result, not a ledger entry.
//
// Primary consumer: Throughline UI proposition panels.
#CaseFile: {
	// PropositionID is the stable identifier of the proposition this file
	// summarises. Must be non-empty.
	PropositionID: _nonEmpty

	// Supporting is the slice of graded assertions whose Stance is "affirms".
	// Sorted by (TierRank desc, AssertionID asc) for stable rendering order.
	// Nil/empty when no affirming assertions were supplied.
	Supporting: [...#GradedAssertion]

	// Contradicting is the slice of graded assertions whose Stance is "denies".
	// Sorted by (TierRank desc, AssertionID asc).
	// Nil/empty when no denying assertions were supplied.
	Contradicting: [...#GradedAssertion]

	// Retracting is the slice of graded assertions whose Stance is a retraction
	// (withdraws | recants). Sorted by (TierRank desc, AssertionID asc).
	Retracting: [...#GradedAssertion]

	// Qualifying is the slice of graded assertions whose Stance is "qualifies".
	// Sorted by (TierRank desc, AssertionID asc).
	Qualifying: [...#GradedAssertion]

	// Conflicts is the detected conflict list for this proposition, filtered to
	// those whose Subject matches PropositionID. Sorted by (Kind, A.ID, B.ID).
	// Nil/empty when no conflicts were detected for this proposition.
	Conflicts: [...conflict.#Conflict]

	// AuthenticityDigests is the condensed set of authenticity assessments for
	// evidence records cited by assertions in this case file.
	// Sorted by RecordID. Nil/empty when no assessments were supplied.
	AuthenticityDigests: [...#AuthenticityDigest]

	// ShiftHistory is the list of narrative shift digests detected for declarants
	// who made assertions on this proposition.
	// Sorted by (Declarant, FromID).
	// Nil/empty when no shifts were detected.
	ShiftHistory: [...#ShiftDigest]

	// SupportSummary is the computed aggregate stance for this proposition.
	// Always present (populated even when all counts are zero).
	SupportSummary: #SupportSummary
}
