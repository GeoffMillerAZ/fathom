// Package fitnessfunctionregistry — valid test data that must unify with schema.cue.
// Running `cue vet ./...` from the package root and `cue vet ./testdata/` both
// validate these against the schema.
package fitnessfunctionregistry

import "github.com/geoffmilleraz/agentic-kit/foundations/quality/fitness-function-registry:fitnessfunctionregistry"

// validTierPrototype demonstrates the prototype maturity band.
validTierPrototype: fitnessfunctionregistry.#Tier & "prototype"

// validTierIntegration demonstrates the integration maturity band.
validTierIntegration: fitnessfunctionregistry.#Tier & "integration"

// validTierApex demonstrates the apex (strongest) maturity band.
validTierApex: fitnessfunctionregistry.#Tier & "apex"

// validCheckPassing is a well-formed, passing weighted criterion.
validCheckPassing: fitnessfunctionregistry.#Check & {
	Name:   "every-span-has-provenance"
	Passed: true
	Weight: 2.0
}

// validCheckFailing is a well-formed, failing criterion with a lower weight.
validCheckFailing: fitnessfunctionregistry.#Check & {
	Name:   "no-orphaned-records"
	Passed: false
	Weight: 1.0
}

// validCheckMinimalWeight exercises the Weight lower bound (>0, not >=0).
validCheckMinimalWeight: fitnessfunctionregistry.#Check & {
	Name:   "minimal-weight"
	Passed: true
	Weight: 0.001
}

// validFunctionMinimal is the smallest legal FitnessFunction: required fields
// only, no Tier (optional), empty Checks, zero Threshold (treated as 1.0 by Go).
validFunctionMinimal: fitnessfunctionregistry.#FitnessFunction & {
	Name:        "span-provenance-gate"
	Description: "Every persisted span must carry provenance."
	Invariant:   "no span is stored without a non-nil Author"
	Checks:      []
	RunFreq:     "per-commit"
	Threshold:   0.0
}

// validFunctionWithChecksAndTier exercises all fields including the optional Tier.
validFunctionWithChecksAndTier: fitnessfunctionregistry.#FitnessFunction & {
	Name:        "evidence-completeness"
	Description: "Evidence spans are complete and anchored."
	Invariant:   "every evidence span carries a RecordHash and a Grade"
	Checks: [
		{Name: "record-hash-present", Passed: true, Weight: 1.5},
		{Name: "grade-present", Passed: true, Weight: 1.5},
	]
	Tier:      "apex"
	RunFreq:   "nightly"
	Threshold: 0.95
}

// validFunctionThresholdBoundaryLow exercises Threshold at the minimum boundary.
validFunctionThresholdBoundaryLow: fitnessfunctionregistry.#FitnessFunction & {
	Name:        "boundary-low"
	Description: ""
	Invariant:   "always measured"
	Checks:      []
	RunFreq:     "nightly"
	Threshold:   0.0
}

// validFunctionThresholdBoundaryHigh exercises Threshold at the maximum boundary.
validFunctionThresholdBoundaryHigh: fitnessfunctionregistry.#FitnessFunction & {
	Name:        "boundary-high"
	Description: ""
	Invariant:   "all checks must pass"
	Checks:      [{Name: "check-a", Passed: true, Weight: 1}]
	RunFreq:     "per-commit"
	Threshold:   1.0
}

// validResultPassed is a well-formed passing ExecutionResult.
validResultPassed: fitnessfunctionregistry.#ExecutionResult & {
	FunctionName: "evidence-completeness"
	Score:        1.0
	Passed:       true
	Failures:     []
	RunID:        "run-2026-01-01-abc123"
	At:           "2026-01-01T00:00:00Z"
	Duration:     0
}

// validResultFailed is a well-formed failing ExecutionResult with failures listed.
validResultFailed: fitnessfunctionregistry.#ExecutionResult & {
	FunctionName: "span-provenance-gate"
	Score:        0.333333
	Passed:       false
	Failures:     ["grade-present", "record-hash-present"]
	RunID:        "run-2026-06-18-xyz789"
	At:           "2026-06-18T12:00:00.000000001Z"
	Duration:     42000
}

// validResultDurationed exercises a non-zero Duration (nanoseconds from ExecuteTimed).
validResultDurationed: fitnessfunctionregistry.#ExecutionResult & {
	FunctionName: "evidence-completeness"
	Score:        0.75
	Passed:       false
	Failures:     ["no-orphaned-records"]
	RunID:        "run-timed-001"
	At:           "2026-06-18T09:30:00+00:00"
	Duration:     1250000
}
