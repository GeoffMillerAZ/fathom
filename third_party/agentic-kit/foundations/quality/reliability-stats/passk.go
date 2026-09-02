package reliabilitystats

import "math"

// PassAtK returns the unbiased pass@k estimate for n sampled attempts of
// which c passed: 1 − C(n−c, k)/C(n, k), the probability that at least
// one of k attempts drawn without replacement from the n passes (Chen et
// al. 2021, "Evaluating Large Language Models Trained on Code", the
// numerically stable product form 1 − ∏_{i=n−c+1}^{n} (1 − k/i)). When
// n−c < k the estimate is exactly 1: fewer than k failures cannot fill a
// k-sample.
//
// Undefined inputs — n < 1, k < 1, k > n, c < 0, c > n — return NaN.
func PassAtK(n, c, k int) float64 {
	if n < 1 || k < 1 || k > n || c < 0 || c > n {
		return math.NaN()
	}
	if n-c < k {
		return 1
	}
	prod := 1.0
	for i := n - c + 1; i <= n; i++ {
		prod *= 1 - float64(k)/float64(i)
	}
	return 1 - prod
}

// PassPowK returns pass^k = p^k: the probability that all k independent
// attempts pass when each passes with probability p (τ-bench, Yao et al.
// 2024, "τ-bench: A Benchmark for Tool-Agent-User Interaction in
// Real-World Domains" — the consistency metric that separates an agent
// which passes once from one that passes every time). PassPowK(0.9, 8)
// = 0.4305 to four places: a 90% agent is a coin flip over eight tries.
//
// p outside [0, 1] or k < 0 returns NaN; k = 0 returns 1.
func PassPowK(p float64, k int) float64 {
	if k < 0 || math.IsNaN(p) || p < 0 || p > 1 {
		return math.NaN()
	}
	return math.Pow(p, float64(k))
}
