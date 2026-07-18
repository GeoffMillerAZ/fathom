package confidencescoring

import "fmt"

// Derive returns a deterministic ConfidenceScore derived from EvidenceInput.
//
// Derivation rules (deterministic, no randomness):
//
//   - 0 evidence items → score 1
//   - Agreement ratio < 0.5 → score 1 (majority disagrees)
//   - 1 item, agreement ≥ 0.5 → score 2
//   - 2–3 items, agreement ≥ 0.5 → score 3
//   - 4–6 items, agreement ≥ 0.5 → score 4
//   - 7+ items, agreement ≥ 0.5 → score 5
//   - agreement ratio ≥ 0.9 adds +1 (capped at 5)
//
// Derive validates the input (AgreementRatio must be 0..1) and returns an
// error for invalid inputs.
func Derive(input EvidenceInput) (ConfidenceScore, error) {
	if input.AgreementRatio < 0.0 || input.AgreementRatio > 1.0 {
		return 0, fmt.Errorf(
			"confidencescoring: AgreementRatio must be 0..1, got %v",
			input.AgreementRatio,
		)
	}
	if input.Count < 0 {
		return 0, fmt.Errorf(
			"confidencescoring: Count must be >= 0, got %d",
			input.Count,
		)
	}

	if input.Count == 0 || input.AgreementRatio < 0.5 {
		return 1, nil
	}

	var base int
	switch {
	case input.Count == 1:
		base = 2
	case input.Count <= 3:
		base = 3
	case input.Count <= 6:
		base = 4
	default:
		base = 5
	}

	// High agreement bonus.
	if input.AgreementRatio >= 0.9 && base < 5 {
		base++
	}

	score := ConfidenceScore(base)
	return score, nil
}
