package determinismgate

import (
	"context"
	"fmt"
	"math"

	qaharness "github.com/geoffmilleraz/agentic-kit/foundations/quality/qa-harness"
	scenariobank "github.com/geoffmilleraz/agentic-kit/foundations/quality/scenario-bank"
)

// Probe runs judge K times against scenario and measures output variance.
// It returns a ProbeResult with the measured agreement rate, per-field entropy,
// verdict-flip count, and a Stable/Unstable verdict against opts.StabilityBand.
//
// Algorithm:
//  1. Build a qaharness.Item from scenario.Input (JSON-encoded as Item.Output).
//  2. Run judge.Score K times, collecting (verdict-inferred-from-score, score) pairs.
//  3. Compute pairwise agreement rate over all K*(K-1)/2 pairs.
//  4. Compute per-field Shannon entropy for "score" and "verdict" fields.
//  5. Classify as Stable(band) or Unstable(band) based on agreement rate vs
//     opts.StabilityBand.
//
// Score-to-verdict inversion: since qaharness.Judge returns a float64 score
// (not a Verdict), Probe treats the score as a continuous field. For the
// "verdict" entropy it buckets scores: >=0.5 → "pass", <0.5 → "fail".
// This is a conservative approximation; callers that need verdict-level
// stability should wrap their judge to return explicit verdicts and set
// opts.SchemaValidator.
//
// When opts.SchemaValidator is set, each output is validated before scoring.
// Structurally invalid outputs are counted as schema errors and excluded from
// variance metrics; their count is not currently exposed (a future addition).
//
// K must be >= 1. A K of 1 always yields AgreementRate=1.0 (trivially stable).
// Use K >= 5 for meaningful variance estimates.
//
// Non-guarantee: Probe cannot make a hosted model deterministic. High
// AgreementRate across K calls is evidence of consistency, not proof of
// determinism. See the package doc for the full non-guarantee statement.
func Probe(ctx context.Context, judge qaharness.Judge, scenario scenariobank.Scenario, K int, opts ProbeOptions) (ProbeResult, error) {
	if K < 1 {
		return ProbeResult{}, fmt.Errorf("determinismgate: K must be >= 1 (got %d)", K)
	}

	band := opts.StabilityBand
	if band <= 0 {
		band = 1.0
	}

	item := qaharness.Item{
		ID:     scenario.ID,
		Output: string(scenario.Input),
	}

	scores := make([]float64, 0, K)
	for i := 0; i < K; i++ {
		if ctx.Err() != nil {
			return ProbeResult{}, ctx.Err()
		}
		if opts.SchemaValidator != nil {
			if err := opts.SchemaValidator(item.Output); err != nil {
				// Schema error: skip this run from variance metrics.
				// We still return a result with what we have so far.
				continue
			}
		}
		score, err := judge.Score(ctx, item)
		if err != nil {
			// Judge error: skip from variance metrics.
			continue
		}
		scores = append(scores, score)
	}

	// If no valid runs, return a degenerate result.
	if len(scores) == 0 {
		return ProbeResult{
			ScenarioID:       scenario.ID,
			K:                K,
			AgreementRate:    0.0,
			VerdictFlipCount: 0,
			PerFieldEntropy:  map[string]float64{"score": 0, "verdict": 0},
			Stability:        StabilityUnstable,
			StabilityBand:    band,
		}, nil
	}

	// Compute pairwise agreement rate over verdict buckets.
	verdicts := make([]string, len(scores))
	for i, s := range scores {
		verdicts[i] = bucketVerdict(s)
	}
	totalPairs, agreePairs, flipCount := pairwiseAgreement(verdicts)

	var agreementRate float64
	if totalPairs == 0 {
		agreementRate = 1.0 // single run → trivially stable
	} else {
		agreementRate = float64(agreePairs) / float64(totalPairs)
	}

	// Per-field entropy.
	perFieldEntropy := map[string]float64{
		"score":   shannonEntropy(discretize(scores, 10)),
		"verdict": shannonEntropy(verdicts),
	}

	stability := StabilityUnstable
	if agreementRate >= band {
		stability = StabilityStable
	}

	return ProbeResult{
		ScenarioID:       scenario.ID,
		K:                K,
		AgreementRate:    agreementRate,
		VerdictFlipCount: flipCount,
		PerFieldEntropy:  perFieldEntropy,
		Stability:        stability,
		StabilityBand:    band,
	}, nil
}

// bucketVerdict maps a score in [0,1] to a coarse verdict for pairwise
// agreement computation.
func bucketVerdict(score float64) string {
	if score >= 0.5 {
		return "pass"
	}
	return "fail"
}

// pairwiseAgreement counts total pairs, agreeing pairs, and flipping pairs
// over the string slice.
func pairwiseAgreement(vals []string) (total, agree, flips int) {
	n := len(vals)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			total++
			if vals[i] == vals[j] {
				agree++
			} else {
				flips++
			}
		}
	}
	return total, agree, flips
}

// discretize maps float64 values into string bucket labels for entropy
// computation. buckets is the number of equal-width bins in [0,1].
func discretize(vals []float64, buckets int) []string {
	out := make([]string, len(vals))
	for i, v := range vals {
		b := int(v * float64(buckets))
		if b >= buckets {
			b = buckets - 1
		}
		out[i] = fmt.Sprintf("b%d", b)
	}
	return out
}

// shannonEntropy computes the Shannon entropy (in bits) of the label
// distribution in vals.
func shannonEntropy(vals []string) float64 {
	if len(vals) == 0 {
		return 0
	}
	counts := make(map[string]int)
	for _, v := range vals {
		counts[v]++
	}
	n := float64(len(vals))
	var h float64
	for _, c := range counts {
		p := float64(c) / n
		if p > 0 {
			h -= p * math.Log2(p)
		}
	}
	return h
}
