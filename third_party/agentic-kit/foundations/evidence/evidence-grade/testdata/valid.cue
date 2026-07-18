// valid.cue — concrete values that satisfy the constraints defined in
// schema.cue. Running `cue vet ./...` from the package root validates these
// against the schema.
package evidencegrade

// A direct-tier grade at full confidence.
_grade1: #Grade & {
	Tier:       "direct"
	Confidence: 1.0
}

// A documentary-tier grade at mid confidence.
_grade2: #Grade & {
	Tier:       "documentary"
	Confidence: 0.75
}

// An inferred-tier grade at low confidence.
_grade3: #Grade & {
	Tier:       "inferred"
	Confidence: 0.3
}

// Confidence at the minimum boundary.
_grade4: #Grade & {
	Tier:       "circumstantial"
	Confidence: 0.0
}

// Confidence at the maximum boundary.
_grade5: #Grade & {
	Tier:       "testimonial"
	Confidence: 1.0
}

// A necessary span that is not independently corroborating.
_span1: #GradedSpan & {
	SpanID:      "span-001"
	Tier:        "direct"
	Confidence:  0.95
	Necessary:   true
	Independent: false
}

// An independent corroborating span (not necessary).
_span2: #GradedSpan & {
	SpanID:      "span-002"
	Tier:        "documentary"
	Confidence:  0.80
	Necessary:   false
	Independent: true
}

// A span that is neither necessary nor independent — it will appear in
// DroppedSpanIDs on the resulting ClaimGrade (invariant §7: no silent
// truncation). Callers must surface DroppedSpanIDs to end-users.
_span3: #GradedSpan & {
	SpanID:      "span-003"
	Tier:        "inferred"
	Confidence:  0.4
	Necessary:   false
	Independent: false
}

// A promotion rule with the default zero-value MinCorroboratorConfidence
// (treated as "> 0 required" by GradeClaim — see PromotionRule godoc).
_rule1: #PromotionRule & {
	AllowTierPromotion:                true
	MinIndependentSourcesForPromotion: 2
	MinCorroboratorConfidence:         0.0
}

// A promotion rule requiring at least 50% confidence in corroborators.
_rule2: #PromotionRule & {
	AllowTierPromotion:                true
	MinIndependentSourcesForPromotion: 1
	MinCorroboratorConfidence:         0.5
}

// A representative ClaimGrade — documentary tier, 72% aggregate confidence,
// no promotion, no dropped spans, no AI contribution.
_claimGrade1: #ClaimGrade & {
	Grade: {
		Tier:       "documentary"
		Confidence: 0.72
	}
	Promoted:                false
	PromotionReason:         ""
	FloorTier:               "documentary"
	DroppedSpanIDs:          []
	AIContributedConfidence: 0.0
}

// A promoted ClaimGrade — necessary floor is testimonial, promoted to
// circumstantial by two independent corroborators; 15% of confidence came
// from TierInferred corroborators (labelled for §3 compliance).
_claimGrade2: #ClaimGrade & {
	Grade: {
		Tier:       "circumstantial"
		Confidence: 0.91
	}
	Promoted:                true
	PromotionReason:         "promoted from testimonial to circumstantial: 2 independent corroborating span(s) at stronger tier"
	FloorTier:               "testimonial"
	DroppedSpanIDs:          ["span-ignored"]
	AIContributedConfidence: 0.15
}
