package authenticity

import (
	"fmt"
	"math"
	"sort"
)

// Assess aggregates a slice of Signals for a single record into an Assessment.
//
// # Pre-conditions
//
//   - signals must be non-empty.
//   - All signals must pass Signal.Validate().
//   - All signals must reference the same RecordID.
//
// # Conservative aggregation rule
//
// The rule is designed for legal safety — when in doubt, report the worst
// credible verdict rather than the best average:
//
//  1. Find the worst (most adversarial) verdict among all signals, weighted by
//     each signal's Score. A signal is "credible" when Score > 0.
//  2. If any credible adverse verdict (likely_synthetic, synthetic, or
//     tampered) exists, it dominates. The Overall verdict is the worst verdict
//     whose contributing signals have a combined weighted score > 0. This holds
//     even if many authentic signals are present. A zero-Score adverse signal is
//     not credible and does not trigger the conservative rule — but it is still
//     preserved in Assessment.Signals so it is never silently discarded
//     (no-silent-truncation, CLAUDE.md §7).
//  3. Multiple independent authentic signals raise Confidence via the
//     complementary-product rule: combined = 1 − ∏(1 − Scoreᵢ), applied
//     across authentic/likely_authentic signals. This models the probability
//     that at least one of the authentic detectors correctly confirms authenticity.
//     A signal with Score == 0 contributes zero additional confidence. If all
//     authentic signals have Score == 0, the authentic branch does not fire —
//     the result is inconclusive (see rule 4).
//  4. Inconclusive-only inputs, or inputs where every signal has Score == 0,
//     yield an inconclusive Overall. A zero-confidence authentic verdict is
//     indistinguishable from no information at all.
//
// # Exact determination of Overall
//
// Let adverseSignals = signals with Verdict ∈ {likely_synthetic, synthetic, tampered} AND Score > 0
// Let authentics = signals with Verdict ∈ {authentic, likely_authentic} AND Score > 0
// Let inconclusives = remaining signals with Verdict == inconclusive (any Score)
// (Zero-Score adverse and authentic signals are preserved in Assessment.Signals but excluded from
// all verdict/confidence calculations.)
//
// If adverseSignals is non-empty:
//   - Overall = the worst Verdict among adverse signals (by adversity rank).
//   - Conservative = true.
//
// Else if authentics is non-empty:
//   - If combined Score (complementary-product) ≥ 0.5, Overall = authentic.
//   - Otherwise Overall = likely_authentic.
//   - Conservative = false.
//
// Else (inconclusive-only, all signals have Score == 0, or only zero-Score authentics):
//   - Overall = inconclusive.
//   - Conservative = false.
//
// Confidence for the adverse case is the max Score among the dominant adverse signals.
// Confidence for the authentic case is the complementary-product of authentic Scores.
// Confidence for inconclusive is the mean Score of inconclusive signals.
func Assess(signals []Signal) (Assessment, error) {
	if len(signals) == 0 {
		return Assessment{}, fmt.Errorf("authenticity: Assess requires at least one signal")
	}

	// Validate all signals and check uniform RecordID.
	recordID := signals[0].RecordID
	for i, s := range signals {
		if err := s.Validate(); err != nil {
			return Assessment{}, fmt.Errorf("authenticity: signal[%d]: %w", i, err)
		}
		if s.RecordID != recordID {
			return Assessment{}, fmt.Errorf("authenticity: all signals must reference the same RecordID; signal[0] has %q, signal[%d] has %q", recordID, i, s.RecordID)
		}
	}

	// Partition signals by adversity category.
	// adverseSignals: adverse verdict (likely_synthetic, synthetic, tampered) with Score > 0.
	// Zero-Score adverse signals are not credible but still appear in Assessment.Signals.
	var adverseSignals []Signal  // Score > 0 adverse — these can dominate
	var credAuthSignals []Signal // authentic/likely_authentic with Score > 0
	var inconclusiveSignals []Signal

	for _, s := range signals {
		rank := s.Verdict.AdversityRank()
		switch {
		case rank >= 4: // likely_synthetic, synthetic, tampered
			if s.Score > 0 {
				adverseSignals = append(adverseSignals, s)
			}
			// Zero-Score adverse signals are preserved in Assessment.Signals via the
			// defensive copy below but do not contribute to the verdict.
		case rank >= 1 && rank <= 2: // authentic, likely_authentic
			if s.Score > 0 {
				credAuthSignals = append(credAuthSignals, s)
			}
			// Zero-Score authentic signals contribute nothing to confidence; they fall
			// through to the inconclusive branch if no credible authentic signals exist.
		default: // inconclusive
			inconclusiveSignals = append(inconclusiveSignals, s)
		}
	}

	var overall Verdict
	var confidence float64
	conservative := false

	switch {
	case len(adverseSignals) > 0:
		// Conservative rule: the worst credible adverse verdict dominates.
		conservative = true
		overall = worstVerdict(adverseSignals)
		// Confidence = max Score among signals that share the dominant verdict.
		for _, s := range adverseSignals {
			if s.Verdict == overall && s.Score > confidence {
				confidence = s.Score
			}
		}

	case len(credAuthSignals) > 0:
		// At least one authentic/likely_authentic signal with Score > 0 is present.
		// Multiple independent signals raise confidence via the complementary-product
		// rule: P(at least one is correct) = 1 − ∏(1 − Scoreᵢ).
		combined := complementaryProduct(credAuthSignals)
		if combined >= 0.5 {
			overall = VerdictAuthentic
		} else {
			overall = VerdictLikelyAuthentic
		}
		confidence = combined

	default:
		// All signals are inconclusive, all have Score == 0, or only zero-Score
		// authentic signals are present. A zero-confidence authentic verdict carries
		// no more information than inconclusive.
		overall = VerdictInconclusive
		confidence = meanScore(inconclusiveSignals)
	}

	// Round to 10 decimal places to eliminate floating-point noise.
	confidence = math.Round(confidence*1e10) / 1e10
	confidence = clamp(confidence, 0, 1)

	// Build a defensive copy of the input signals, sorted by ID for
	// determinism — the caller must not be able to observe internal ordering.
	sigsCopy := make([]Signal, len(signals))
	copy(sigsCopy, signals)
	sort.Slice(sigsCopy, func(i, j int) bool {
		return sigsCopy[i].ID < sigsCopy[j].ID
	})

	return Assessment{
		RecordID:     recordID,
		Overall:      overall,
		Confidence:   confidence,
		Signals:      sigsCopy,
		Conservative: conservative,
	}, nil
}

// worstVerdict returns the most adversarial Verdict among signals.
// Signals must be non-empty.
func worstVerdict(signals []Signal) Verdict {
	worst := signals[0].Verdict
	for _, s := range signals[1:] {
		if s.Verdict.AdversityRank() > worst.AdversityRank() {
			worst = s.Verdict
		}
	}
	return worst
}

// complementaryProduct computes the complementary-product of the signal Scores:
//
//	combined = 1 − ∏(1 − Scoreᵢ)
//
// This models the probability that at least one of the signals is correct.
// Each additional confirming signal reduces the probability that ALL of them
// are wrong, yielding diminishing returns toward 1.0.
func complementaryProduct(signals []Signal) float64 {
	if len(signals) == 0 {
		return 0
	}
	complement := 1.0
	for _, s := range signals {
		complement *= (1.0 - s.Score)
	}
	return 1.0 - complement
}

// meanScore returns the mean Score of signals, or 0 if empty.
func meanScore(signals []Signal) float64 {
	if len(signals) == 0 {
		return 0
	}
	sum := 0.0
	for _, s := range signals {
		sum += s.Score
	}
	return sum / float64(len(signals))
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
