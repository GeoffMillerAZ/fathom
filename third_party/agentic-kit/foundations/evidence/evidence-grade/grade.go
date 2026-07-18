package evidencegrade

import (
	"fmt"
	"math"
)

// PromotionRule controls whether GradeClaim is allowed to promote the result
// tier above the FloorTier when sufficient independent corroboration exists.
type PromotionRule struct {
	// AllowTierPromotion enables tier promotion at all.
	AllowTierPromotion bool

	// MinIndependentSourcesForPromotion is the minimum number of independent
	// corroborating spans (Necessary=false, Independent=true) whose tier is
	// strictly stronger than the FloorTier before promotion is attempted.
	// Values ≤ 0 are treated as 1.
	MinIndependentSourcesForPromotion int

	// MinCorroboratorConfidence is the minimum Confidence a corroborating span
	// must have to qualify for tier promotion. Spans below this threshold count
	// neither toward MinIndependentSourcesForPromotion nor toward the promoted
	// tier calculation. A value of 0 (the zero-value default) is treated as a
	// strict "> 0" gate — a zero-confidence corroborator contributes nothing
	// epistemic and must never grant a tier promotion regardless of its nominal
	// tier strength. To require at least 50% confidence, set this to 0.5.
	MinCorroboratorConfidence float64
}

// ClaimGrade is the result of evaluating a set of GradedSpans against a claim.
//
// Design:
//   - Grade.Tier is the effective tier after applying any promotion.
//   - FloorTier is the weakest tier among the necessary spans; it is the
//     ceiling when no promotion occurs and the hard floor for all-inferred claims.
//   - Promoted is true when promotion actually changed the tier.
//   - PromotionReason is a human-readable explanation (empty when Promoted=false).
//   - DroppedSpanIDs lists any spans that were neither Necessary nor Independent
//     (invariant §7: no silent truncation). Callers should surface these to
//     end-users — the analyst believes they submitted evidence that was evaluated,
//     but it was not. An empty slice means all spans participated.
//   - AIContributedConfidence is the incremental confidence added solely by
//     TierInferred corroborating spans (CLAUDE.md invariant §3: AI contributions
//     must be labelled). It equals (aggregate confidence with inferred corroborators)
//     minus (aggregate confidence without them), rounded to 10 decimal places.
//     Zero means no TierInferred corroborators were present or they added nothing.
//     Callers displaying a high Confidence value should warn when this field is
//     non-trivial (e.g. > 0.05) so reviewers know AI inference drove the score.
type ClaimGrade struct {
	Grade
	// Promoted is true when the effective tier was raised above FloorTier.
	Promoted bool
	// PromotionReason describes why promotion occurred (empty if not promoted).
	PromotionReason string
	// FloorTier is the weakest tier among the necessary spans.
	FloorTier Tier
	// DroppedSpanIDs lists SpanIDs that were neither Necessary nor Independent
	// and therefore did not participate in grading. Never nil when non-empty;
	// callers must surface these to comply with invariant §7 (no silent truncation).
	DroppedSpanIDs []string
	// AIContributedConfidence is the incremental confidence attributable solely
	// to TierInferred corroborating spans (see ClaimGrade doc for full semantics).
	// Zero when no TierInferred corroborators were present.
	AIContributedConfidence float64
}

// GradeClaim derives a ClaimGrade from a set of GradedSpans.
//
// # Confidence aggregation
//
// Confidence is computed in two stages:
//
//  1. Necessary spans: the base confidence is the product of individual
//     confidences (each span independently confirms a required link in the
//     chain; if any link is weak the whole chain is weak). This is the
//     "weakest-link product" model.
//  2. Independent corroborating spans (Necessary=false, Independent=true)
//     raise the aggregate confidence toward 1.0 using the complementary
//     product rule:
//     combined = 1 − ∏(1 − cᵢ) applied to the full set
//     {base} ∪ {corroborating confidences}, bounded to [0, 1].
//
// This means corroboration always helps but has diminishing returns, and a
// very weak necessary span (Confidence≈0) correctly drags the base low.
//
// # Tier determination
//
// FloorTier = weakest tier among necessary spans.
// Effective tier = FloorTier unless ALL of the following hold:
//   - rule.AllowTierPromotion is true.
//   - There are ≥ max(1, rule.MinIndependentSourcesForPromotion) independent
//     corroborating spans whose tier is strictly stronger than FloorTier.
//   - FloorTier is NOT TierInferred. If FloorTier is TierInferred (any necessary
//     span is inferred), promotion is blocked — AI-inferred evidence can never
//     lift a claim above inferred (hard invariant from CLAUDE.md §4).
//
// When promotion applies, the effective tier is the weakest tier among the
// qualifying corroborating spans (conservative: we only promote as far as
// the corroboration warrants).
//
// # Errors
//
// Returns an error if spans is empty or any span fails Validate().
func GradeClaim(spans []GradedSpan, rule PromotionRule) (ClaimGrade, error) {
	if len(spans) == 0 {
		return ClaimGrade{}, fmt.Errorf("evidencegrade: GradeClaim requires at least one span")
	}

	// Validate all spans upfront.
	for i, s := range spans {
		if err := s.Validate(); err != nil {
			return ClaimGrade{}, fmt.Errorf("evidencegrade: span[%d]: %w", i, err)
		}
	}

	// Partition into necessary, independent-corroborating, and dropped.
	// Dropped spans (Necessary=false && Independent=false) are surfaced in
	// DroppedSpanIDs to comply with invariant §7 (no silent truncation).
	var necessary []GradedSpan
	var corroborating []GradedSpan
	var droppedIDs []string
	for _, s := range spans {
		if s.Necessary {
			necessary = append(necessary, s)
		} else if s.Independent {
			corroborating = append(corroborating, s)
		} else {
			droppedIDs = append(droppedIDs, s.SpanID)
		}
	}

	if len(necessary) == 0 {
		return ClaimGrade{}, fmt.Errorf("evidencegrade: GradeClaim requires at least one necessary span")
	}

	// --- Determine FloorTier: weakest tier among necessary spans. ---
	floorTier := necessary[0].Tier
	for _, s := range necessary[1:] {
		if s.Tier.Rank() < floorTier.Rank() {
			floorTier = s.Tier
		}
	}

	// --- Confidence: weakest-link product over necessary spans. ---
	baseConf := 1.0
	for _, s := range necessary {
		baseConf *= s.Confidence
	}
	baseConf = clamp(baseConf, 0, 1)

	// Corroboration: complementary-product rule over {base} ∪ {corr confidences}.
	// combined = 1 − ∏(1 − cᵢ)
	//
	// We compute two aggregates to track AI contribution (invariant §3):
	//   confWithoutInferred: base + non-inferred corroborators only
	//   aggConf: base + all corroborators (including TierInferred)
	// AIContributedConfidence = aggConf - confWithoutInferred.
	aggConf := baseConf
	confWithoutInferred := baseConf
	if len(corroborating) > 0 {
		complement := 1.0 - baseConf
		complementWithoutInferred := 1.0 - baseConf
		for _, s := range corroborating {
			complement *= (1.0 - s.Confidence)
			if s.Tier != TierInferred {
				complementWithoutInferred *= (1.0 - s.Confidence)
			}
		}
		// Clamp to handle floating-point drift.
		aggConf = clamp(1.0-complement, 0, 1)
		confWithoutInferred = clamp(1.0-complementWithoutInferred, 0, 1)
	}
	// Round to 10 decimal places to eliminate floating-point noise.
	aggConf = math.Round(aggConf*1e10) / 1e10
	confWithoutInferred = math.Round(confWithoutInferred*1e10) / 1e10
	aiContrib := math.Round((aggConf-confWithoutInferred)*1e10) / 1e10
	if aiContrib < 0 {
		aiContrib = 0
	}

	// --- Tier promotion ---
	effectiveTier := floorTier
	promoted := false
	promotionReason := ""

	// Hard invariant: if FloorTier is TierInferred (any necessary span is inferred),
	// promotion is blocked — AI-inferred evidence can never lift a claim above inferred.
	floorIsInferred := floorTier == TierInferred

	if rule.AllowTierPromotion && !floorIsInferred && len(corroborating) > 0 {
		minSources := rule.MinIndependentSourcesForPromotion
		if minSources <= 0 {
			minSources = 1
		}

		// Collect corroborating spans strictly stronger than floorTier,
		// deduplicated by SpanID (keep first occurrence) so the same SpanID
		// submitted twice does not count as two independent sources.
		//
		// Confidence gate: a span must have Confidence > minCorrConf to qualify.
		// The zero-value default (0) enforces "> 0" — a zero-confidence span
		// contributes no epistemic weight and must never grant a tier promotion.
		minCorrConf := rule.MinCorroboratorConfidence
		seen := make(map[string]struct{})
		var qualifying []GradedSpan
		for _, s := range corroborating {
			if s.Tier.StrongerThan(floorTier) && s.Confidence > minCorrConf {
				if _, dup := seen[s.SpanID]; !dup {
					seen[s.SpanID] = struct{}{}
					qualifying = append(qualifying, s)
				}
			}
		}

		if len(qualifying) >= minSources {
			// Promote to the weakest tier among qualifying spans (conservative).
			promotedTier := qualifying[0].Tier
			for _, s := range qualifying[1:] {
				if s.Tier.Rank() < promotedTier.Rank() {
					promotedTier = s.Tier
				}
			}
			if promotedTier.StrongerThan(effectiveTier) {
				promoted = true
				promotionReason = fmt.Sprintf(
					"promoted from %s to %s: %d independent corroborating span(s) at stronger tier",
					floorTier, promotedTier, len(qualifying),
				)
				effectiveTier = promotedTier
			}
		}
	}

	return ClaimGrade{
		Grade: Grade{
			Tier:       effectiveTier,
			Confidence: aggConf,
		},
		Promoted:                promoted,
		PromotionReason:         promotionReason,
		FloorTier:               floorTier,
		DroppedSpanIDs:          droppedIDs,
		AIContributedConfidence: aiContrib,
	}, nil
}

// clamp restricts v to [lo, hi].
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
