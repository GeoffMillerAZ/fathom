// Package evalgate defines the CUE schema for the eval-gate foundation.
// These constraints mirror the Go types and are evaluated with `cue vet ./...`.
package evalgate

// #Check is a single named evaluation criterion.
#Check: {
	// Name is a non-empty human-readable identifier for this criterion.
	Name: string & !=""

	// Passed indicates whether this criterion is satisfied.
	Passed: bool

	// Weight is the relative importance; must be strictly positive.
	Weight: number & >0
}

// #Result is the outcome of Gate.Evaluate.
#Result: {
	// Passed is true when Score >= the configured threshold.
	Passed: bool

	// Score is the weighted fraction of passing checks in [0, 1].
	Score: number & >=0 & <=1

	// Failures lists names of checks that did not pass; may be empty.
	Failures: [...string]
}
