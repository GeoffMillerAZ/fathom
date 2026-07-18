package elementcoverage

import (
	"fmt"

	evidencegrade "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-grade"
)

// Assess evaluates a set of named elements against the supplied evidence map
// and returns one ElementCoverage per element, in the same order as elements.
//
// For each element Assess calls evidencegrade.GradeClaim on its spans; the
// resulting ClaimGrade determines Status:
//   - met:              graded tier >= requiredTier AND confidence >= minConfidence
//   - under_evidenced:  evidence exists but tier or confidence is below the bar
//   - unmet:            no evidence was submitted for this element
//
// The Gap field is populated for under_evidenced and unmet results, naming the
// weakest link (invariant §7: no silent truncation — every element always
// appears in the output).
//
// Determinism: output order matches the elements slice exactly.
//
// Nil elements returns (nil, nil). Any invalid Element.ID or Element.Name
// returns an error immediately.
func Assess(
	elements []Element,
	evidenceByElement map[string][]evidencegrade.GradedSpan,
	requiredTier evidencegrade.Tier,
	minConfidence float64,
	rule evidencegrade.PromotionRule,
) ([]ElementCoverage, error) {
	if len(elements) == 0 {
		return nil, nil
	}

	if err := requiredTier.Validate(); err != nil {
		return nil, fmt.Errorf("elementcoverage.Assess: invalid requiredTier: %w", err)
	}
	if minConfidence < 0 || minConfidence > 1 {
		return nil, fmt.Errorf("elementcoverage.Assess: minConfidence %g is outside [0, 1]", minConfidence)
	}

	out := make([]ElementCoverage, 0, len(elements))

	for _, elem := range elements {
		if err := elem.Validate(); err != nil {
			return nil, fmt.Errorf("elementcoverage.Assess: %w", err)
		}

		spans := evidenceByElement[elem.ID] // nil if not present

		if len(spans) == 0 {
			// No evidence at all — unmet.
			out = append(out, ElementCoverage{
				ElementID:       elem.ID,
				Status:          StatusUnmet,
				WeakestTier:     "",
				Confidence:      0,
				EvidenceSpanIDs: nil,
				Gap:             fmt.Sprintf("no evidence for %q", elem.Name),
			})
			continue
		}

		// Collect span IDs for provenance reporting (defensive copy, input order).
		spanIDs := make([]string, len(spans))
		for i, s := range spans {
			spanIDs[i] = s.SpanID
		}

		grade, err := evidencegrade.GradeClaim(spans, rule)
		if err != nil {
			return nil, fmt.Errorf("elementcoverage.Assess: element %q: GradeClaim: %w", elem.ID, err)
		}

		tierOK := grade.FloorTier.Rank() >= requiredTier.Rank()
		confOK := grade.Confidence >= minConfidence

		var status Status
		var gap string

		switch {
		case tierOK && confOK:
			status = StatusMet
		default:
			status = StatusUnderEvidenced
			gap = buildGap(elem.Name, grade.FloorTier, grade.Confidence, requiredTier, minConfidence, tierOK, confOK)
		}

		out = append(out, ElementCoverage{
			ElementID:       elem.ID,
			Status:          status,
			WeakestTier:     grade.FloorTier,
			Confidence:      grade.Confidence,
			EvidenceSpanIDs: spanIDs,
			Gap:             gap,
		})
	}

	return out, nil
}

// buildGap constructs a human-readable description of why an element is
// under_evidenced, naming the weakest link that prevents a "met" verdict.
func buildGap(
	name string,
	floorTier evidencegrade.Tier,
	confidence float64,
	requiredTier evidencegrade.Tier,
	minConfidence float64,
	tierOK, confOK bool,
) string {
	switch {
	case !tierOK && !confOK:
		return fmt.Sprintf(
			"weakest evidence for %q is %s (need %s) and confidence %.2f is below minimum %.2f",
			name, floorTier, requiredTier, confidence, minConfidence,
		)
	case !tierOK:
		return fmt.Sprintf(
			"no %s evidence for %q (weakest tier is %s)",
			requiredTier, name, floorTier,
		)
	default: // !confOK
		return fmt.Sprintf(
			"confidence %.2f for %q is below minimum %.2f (tier %s is sufficient)",
			confidence, name, minConfidence, floorTier,
		)
	}
}
