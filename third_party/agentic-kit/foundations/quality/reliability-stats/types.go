// Package reliabilitystats is the small statistics kit a bench needs to
// say "A beats B" or "this scenario is reliable" without lying about how
// much evidence it has: the unbiased pass@k estimator, pass^k, a
// task-clustered paired bootstrap confidence interval, the Wilcoxon
// signed-rank test, and the Vargha–Delaney A12 effect size. Every
// function is pure, seeded where it samples, and stdlib-only.
//
// # What each primitive answers
//
//   - PassAtK(n, c, k): of n sampled attempts c passed — the probability
//     that at least one of k attempts passes (Chen et al. 2021,
//     "Evaluating Large Language Models Trained on Code": the unbiased
//     estimator in its numerically stable product form).
//   - PassPowK(p, k): the probability that ALL k independent attempts
//     pass, p^k (τ-bench, Yao et al. 2024) — the reliability number an
//     operator cares about when an agent must succeed every time.
//   - PairedBootstrapCI: a percentile bootstrap interval for the mean
//     paired difference, resampling TASKS (clusters), not trials, so
//     repeated trials of one task cannot masquerade as independent
//     evidence. Deterministic under its seed.
//   - Wilcoxon: the signed-rank test on paired differences — exact by
//     enumeration for n <= 25 pairs, normal approximation with
//     continuity correction above; ties take average ranks; zero
//     differences are dropped and reported.
//   - A12: Vargha & Delaney's A statistic (as recommended for
//     randomized-algorithm comparison by Arcuri & Briand 2011) — the
//     probability a draw from a exceeds a draw from b, ties counted half;
//     0.5 is no effect.
//
// # Non-guarantees (stated as plainly as the guarantees)
//
//   - No multiple-comparison correction is applied anywhere. A caller
//     that runs many tests must pre-register its comparisons (fix the
//     hypotheses before looking at the data) and correct the family-wise
//     error rate itself; running Wilcoxon over every scenario and
//     reporting the smallest p is exactly the theater this package does
//     not prevent.
//   - PairedBootstrapCI is a PERCENTILE interval, not BCa or studentized:
//     it is biased for skewed statistics and needs enough tasks to mean
//     anything — with one task it collapses to the point estimate.
//   - PassAtK assumes the n attempts are exchangeable samples from one
//     distribution; PassPowK assumes k independent attempts with a
//     stationary pass probability. Neither models drift, caching, or
//     correlated failures, and a p estimated from fewer than k trials
//     is a point on no evidence — report the trial count next to it.
//   - Wilcoxon's exact p-value is exact under the null of differences
//     symmetric about zero; the achievable alpha is coarse at small n
//     (n = 5 cannot reach p < 0.0625 two-sided) and nothing here
//     smooths that.
//   - A12 is an effect size, not a test: it says nothing about
//     significance, and with tiny samples it is a coarse ratio.
//   - Undefined inputs (invalid n/c/k, an empty sample for A12) return
//     NaN rather than a silent 0 or 1: a NaN in a report is a visible
//     caller bug; a 0 would be a claim.
//
// # Concurrency contract
//
// Every function is pure and safe for concurrent use; results are
// immutable values. PairedBootstrapCI builds its own math/rand source
// from the seed per call and shares nothing between calls.
package reliabilitystats

import "errors"

// Typed refusals, matched with errors.Is.
var (
	// ErrEmptySample is returned when a sample has no observations.
	ErrEmptySample = errors.New("reliabilitystats: empty sample")

	// ErrLengthMismatch is returned when paired samples differ in length.
	ErrLengthMismatch = errors.New("reliabilitystats: paired samples differ in length")

	// ErrNoPairs is returned by Wilcoxon when every paired difference is
	// zero: the test is undefined, not "p = 1".
	ErrNoPairs = errors.New("reliabilitystats: no non-zero paired differences")

	// ErrInvalidSample is returned when a sample contains NaN or ±Inf.
	ErrInvalidSample = errors.New("reliabilitystats: sample contains NaN or Inf")

	// ErrInvalidResamples is returned when a bootstrap is asked for fewer
	// than one replicate.
	ErrInvalidResamples = errors.New("reliabilitystats: resamples must be >= 1")

	// ErrInvalidLevel is returned when a confidence level is outside the
	// open interval (0, 1).
	ErrInvalidLevel = errors.New("reliabilitystats: level must be in (0, 1)")
)

// Observation is one paired trial: Task identifies the cluster the trial
// belongs to (the scenario, problem, or prompt) and Diff is the
// treatment-minus-baseline score for that trial. All trials of one Task
// are resampled together.
//
// CUE mirror: #Observation in schema.cue.
type Observation struct {
	// Task is the cluster id; non-empty.
	Task string `json:"task"`

	// Diff is treatment minus baseline for this trial.
	Diff float64 `json:"diff"`
}

// Interval is a percentile bootstrap confidence interval around a point
// estimate of the mean paired difference.
//
// CUE mirror: #Interval in schema.cue.
type Interval struct {
	// Low is the lower percentile bound.
	Low float64 `json:"low"`

	// High is the upper percentile bound; never below Low.
	High float64 `json:"high"`

	// Point is the statistic on the observed sample (the mean over tasks
	// of each task's mean Diff).
	Point float64 `json:"point"`

	// Tasks is the number of distinct clusters the interval resampled —
	// the unit count a reader should weigh the interval by.
	Tasks int `json:"tasks"`
}

// WilcoxonResult is the outcome of a two-sided Wilcoxon signed-rank test.
//
// CUE mirror: #WilcoxonResult in schema.cue.
type WilcoxonResult struct {
	// W is min(WPlus, WMinus): the conventional T statistic.
	W float64 `json:"w"`

	// WPlus is the rank sum of the positive differences.
	WPlus float64 `json:"w_plus"`

	// WMinus is the rank sum of the negative differences.
	WMinus float64 `json:"w_minus"`

	// N is the number of pairs after zero differences are dropped.
	N int `json:"n"`

	// Zeros is the number of zero differences dropped.
	Zeros int `json:"zeros"`

	// P is the two-sided p-value in [0, 1].
	P float64 `json:"p"`

	// Exact is true when P came from the enumerated null distribution
	// (N <= 25) and false when from the normal approximation.
	Exact bool `json:"exact"`
}
