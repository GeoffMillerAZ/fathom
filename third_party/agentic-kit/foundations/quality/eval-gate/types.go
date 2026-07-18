// Package evalgate provides a deterministic pass/fail gate over a set of
// weighted checks. No LLM is involved; every decision is fully reproducible
// given the same inputs and threshold.
//
// Concurrency: values are immutable and safe for concurrent use.
package evalgate

// Check is a single named evaluation criterion with a pass/fail result and a
// relative weight. Weight must be > 0; negative or zero weights are ignored
// during evaluation.
type Check struct {
	// Name is a human-readable identifier for this criterion.
	Name string

	// Passed is true when this criterion is satisfied.
	Passed bool

	// Weight is the relative importance of this check in the composite score.
	// All weights are summed and each check contributes Weight/TotalWeight to
	// the score. Must be > 0.
	Weight float64
}

// Result is the outcome of Gate.Evaluate.
type Result struct {
	// Passed is true when Score >= Gate.Threshold.
	Passed bool

	// Score is the weighted fraction of passing checks in [0, 1].
	// Score = sum(weight of passing checks) / sum(all weights).
	// Zero when there are no checks or all weights are non-positive.
	Score float64

	// Failures lists the names of checks that did not pass.
	Failures []string
}

// Gate holds the configuration for a single evaluation gate.
type Gate struct {
	// Threshold is the minimum Score required to pass. Must be in [0, 1].
	// Defaults to 1.0 (all checks must pass) when zero-valued.
	Threshold float64
}
