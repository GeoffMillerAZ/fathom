// Package fitnessfunctionregistry defines the CUE-first contract for the
// "provable-excellence" (E2) backbone. Go types in types.go mirror these
// definitions; any drift is a bug. These constraints are evaluated with
// `cue vet ./...`.
package fitnessfunctionregistry

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// _rfc3339 constrains a string to the RFC3339 timestamp format produced by
// Go time.Time marshalling (mirrors ExecutionResult.At).
_rfc3339: string & =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})$"

// #Tier enumerates the maturity bands, weakest to strongest.
#Tier: "prototype" | "integration" | "apex"

// #Check mirrors evalgate.#Check: a single weighted, named criterion.
#Check: {
	// Name is a non-empty human-readable identifier for this criterion.
	Name: _nonEmpty

	// Passed indicates whether this criterion is satisfied.
	Passed: bool

	// Weight is the relative importance; must be strictly positive.
	Weight: number & >0
}

// #FitnessFunction is a single named, machine-checkable invariant.
#FitnessFunction: {
	// Name uniquely identifies the function within a registry.
	Name: _nonEmpty

	// Description is a human-readable summary of what the function measures.
	Description: string

	// Invariant states the property that must remain true.
	Invariant: string

	// Checks are the weighted criteria scored by eval-gate. May be empty.
	Checks: [...#Check]

	// Tier is the maturity band at which the invariant is binding. Optional;
	// when set it must be a recognised tier.
	Tier?: #Tier

	// RunFreq is advisory scheduling metadata (e.g. "per-commit", "nightly").
	RunFreq: string

	// Threshold is the minimum score required to pass, in [0, 1]. A zero value
	// is treated as 1.0 by the Go implementation.
	Threshold: number & >=0 & <=1
}

// #ExecutionResult is the deterministic outcome of executing a #FitnessFunction.
#ExecutionResult: {
	// FunctionName is the Name of the executed function.
	FunctionName: _nonEmpty

	// Score is the weighted fraction of passing checks in [0, 1].
	Score: number & >=0 & <=1

	// Passed is true when Score >= the function's effective threshold.
	Passed: bool

	// Failures lists names of checks that did not pass; may be empty.
	Failures: [...string]

	// RunID is the caller-injected run identifier.
	RunID: _nonEmpty

	// At is the caller-injected timestamp (RFC3339) of the run.
	At: _rfc3339

	// Duration is the optional measured cost in nanoseconds; 0 = not measured.
	Duration: int & >=0
}
