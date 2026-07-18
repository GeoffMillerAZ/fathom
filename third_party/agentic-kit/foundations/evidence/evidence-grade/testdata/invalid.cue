// Package evidencegrade — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid CUE).
// It documents the constraints enforced by schema.cue. Corresponding Go-level
// enforcement is in GradedSpan.Validate(), Grade.Validate(), and GradeClaim().
//
// To verify the schema rejects a binding, temporarily uncomment it and run:
//
//	cue vet ./testdata/   (expect errors for that binding)
//
// or:
//
//	cue eval -e '<name>' ./foundations/evidence/evidence-grade/testdata/
//
// The following errors would be produced by `cue vet ./testdata/`:
//
//	_badTier: "expert" is not one of the five recognised #Tier values.
//	_badGradeConfHigh: Confidence 1.1 violates <=1.
//	_badGradeConfLow: Confidence -0.01 violates >=0.
//	_badGradeUnknownTier: "hearsay" is not a valid #Tier.
//	_badSpanEmptyID: "" violates string & !="" (SpanID must be non-empty).
//	_badSpanConfHigh: Confidence 1.5 violates <=1.
//	_badSpanNecessaryAndIndependent: _necessaryXorIndependent evaluates to false.
//	_badPromotionRuleConfNegative: MinCorroboratorConfidence -0.1 violates >=0.
//	_badClaimGradeAIConfNegative: AIContributedConfidence -0.1 violates >=0.
package evidencegrade

// // _badTier: "expert" is not one of the five recognised tiers.
// _badTier: #Tier & "expert"

// // _badGradeConfHigh: Confidence > 1 violates the >=0 & <=1 constraint.
// _badGradeConfHigh: #Grade & {
// 	Tier:       "direct"
// 	Confidence: 1.1
// }

// // _badGradeConfLow: Confidence < 0 violates the >=0 & <=1 constraint.
// _badGradeConfLow: #Grade & {
// 	Tier:       "inferred"
// 	Confidence: -0.01
// }

// // _badGradeUnknownTier: unknown tier value.
// _badGradeUnknownTier: #Grade & {
// 	Tier:       "hearsay"
// 	Confidence: 0.5
// }

// // _badSpanEmptyID: empty SpanID violates the non-empty string constraint.
// _badSpanEmptyID: #GradedSpan & {
// 	SpanID:      ""
// 	Tier:        "direct"
// 	Confidence:  0.9
// 	Necessary:   true
// 	Independent: false
// }

// // _badSpanConfHigh: Confidence > 1 on a GradedSpan.
// _badSpanConfHigh: #GradedSpan & {
// 	SpanID:      "span-bad"
// 	Tier:        "documentary"
// 	Confidence:  1.5
// 	Necessary:   true
// 	Independent: false
// }

// // _badSpanNecessaryAndIndependent: a span cannot be both Necessary and
// // Independent. Necessary spans set the FloorTier; Independent spans
// // corroborate. The _necessaryXorIndependent field in #GradedSpan encodes
// // !(Necessary && Independent); both true makes it false, which CUE rejects.
// // Go also enforces this in GradedSpan.Validate().
// _badSpanNecessaryAndIndependent: #GradedSpan & {
// 	SpanID:      "span-dual-role"
// 	Tier:        "direct"
// 	Confidence:  0.9
// 	Necessary:   true
// 	Independent: true
// }

// // _badPromotionRuleConfNegative: MinCorroboratorConfidence must be >= 0.
// _badPromotionRuleConfNegative: #PromotionRule & {
// 	AllowTierPromotion:                true
// 	MinIndependentSourcesForPromotion: 1
// 	MinCorroboratorConfidence:         -0.1
// }

// // _badClaimGradeAIConfNegative: AIContributedConfidence must be >= 0.
// _badClaimGradeAIConfNegative: #ClaimGrade & {
// 	Grade: {
// 		Tier:       "documentary"
// 		Confidence: 0.7
// 	}
// 	Promoted:                false
// 	PromotionReason:         ""
// 	FloorTier:               "documentary"
// 	DroppedSpanIDs:          []
// 	AIContributedConfidence: -0.1
// }
