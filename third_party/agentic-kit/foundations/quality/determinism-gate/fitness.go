package determinismgate

// fitness.go registers loop-honesty FitnessFunctions for the eval loop
// (spec §4.5). No new package is created; these functions live here because
// determinism-gate is the final layer that certifies the eval loop's
// configuration is honest.
//
// The registered functions certify:
//
//  1. eval.independence-validated — ValidateIndependence was called and passed
//     before any verdict was marked decisive (checked structurally at
//     registration time by the caller).
//  2. eval.provenance-gates-on — no production-captured scenario appears in
//     the committed bank (checked by scenariobank.Validate).
//  3. eval.journal-attached — an eval-journal writer is wired to the loop
//     (structural check by the caller's config).
//  4. eval.determinism-probed — Probe was called with K >= 3 on the judge
//     before any run is marked stable (structural check by the caller).
//
// IMPORTANT: FitnessFunctions returns a slice of FitnessFunction values whose
// Checks are PRE-POPULATED by the caller via EvalLoopConfig. This function
// does NOT make LLM calls, does NOT run Probe, and does NOT validate the
// catalog — it compiles the caller's assertions into the standard
// fitnessfunctionregistry shape. The caller is responsible for providing
// honest Checks.
//
// Typical usage:
//
//	cfg := determinismgate.EvalLoopConfig{
//	    IndependenceValidated: true,
//	    ProvenanceGatesOn:     true,
//	    JournalAttached:       journalWriter != nil,
//	    DeterminismProbed:     probeResult.K >= 3,
//	}
//	fns := determinismgate.FitnessFunctions(cfg)
//	for _, fn := range fns {
//	    registry.Register(fn)
//	}
//
// task repo-fitness (coordinator wires this): run `go test ./foundations/quality/...`
// then execute each registered fitness function against EvalLoopConfig derived
// from the actual eval loop config. The coordinator adds a Taskfile target:
//
//	repo-fitness:
//	  go test -run TestFitnessRegistration ./foundations/quality/determinism-gate/...

import (
	evalgate "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/eval-gate"
	fitnessfunctionregistry "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/fitness-function-registry"
)

// EvalLoopConfig is the caller's honest assertion about the eval loop's
// configuration. Each field is a boolean claim; FitnessFunctions compiles
// these into evalgate.Check values.
//
// All fields default to false (the fail-closed default). Callers must
// explicitly set each field to true only after verifying the corresponding
// property holds.
type EvalLoopConfig struct {
	// IndependenceValidated is true when modelcatalog.ValidateIndependence was
	// called and returned zero violations before any verdict was marked decisive.
	IndependenceValidated bool

	// ProvenanceGatesOn is true when the scenario bank has been validated and
	// no production-captured scenarios are present in committed testdata.
	ProvenanceGatesOn bool

	// JournalAttached is true when an eval-journal writer is wired to the
	// eval loop (every verdict is persisted).
	JournalAttached bool

	// DeterminismProbed is true when Probe was called with K >= 3 on the
	// judge before any run was marked Stable.
	DeterminismProbed bool
}

// FitnessFunctions returns the four loop-honesty fitness functions for the
// eval loop, pre-populated with Checks from cfg. Register them in a
// fitnessfunctionregistry.Registry and run ExecuteAll to certify the loop.
//
// Each function uses TierIntegration maturity and RunFreq "per-commit".
func FitnessFunctions(cfg EvalLoopConfig) []fitnessfunctionregistry.FitnessFunction {
	return []fitnessfunctionregistry.FitnessFunction{
		{
			Name:        "eval.independence-validated",
			Description: "Judge and producer independence was validated before any decisive verdict.",
			Invariant:   "modelcatalog.ValidateIndependence returned zero violations before any verdict was marked decisive",
			Tier:        fitnessfunctionregistry.TierIntegration,
			RunFreq:     "per-commit",
			Threshold:   1.0,
			Checks: []evalgate.Check{
				{
					Name:   "independence-validated",
					Passed: cfg.IndependenceValidated,
					Weight: 1.0,
				},
			},
		},
		{
			Name:        "eval.provenance-gates-on",
			Description: "Scenario bank contains no production-captured scenarios in committed testdata.",
			Invariant:   "every committed scenario has provenance production-derived-redacted or synthetic-adversarial",
			Tier:        fitnessfunctionregistry.TierIntegration,
			RunFreq:     "per-commit",
			Threshold:   1.0,
			Checks: []evalgate.Check{
				{
					Name:   "no-production-captured-in-testdata",
					Passed: cfg.ProvenanceGatesOn,
					Weight: 1.0,
				},
			},
		},
		{
			Name:        "eval.journal-attached",
			Description: "Every verdict is persisted to an eval-journal writer.",
			Invariant:   "an eval-journal writer is wired to the eval loop so no verdict is lost",
			Tier:        fitnessfunctionregistry.TierIntegration,
			RunFreq:     "per-commit",
			Threshold:   1.0,
			Checks: []evalgate.Check{
				{
					Name:   "journal-writer-wired",
					Passed: cfg.JournalAttached,
					Weight: 1.0,
				},
			},
		},
		{
			Name:        "eval.determinism-probed",
			Description: "Probe was called with K >= 3 on the judge before marking any run Stable.",
			Invariant:   "determinismgate.Probe with K>=3 was called before any stable classification",
			Tier:        fitnessfunctionregistry.TierIntegration,
			RunFreq:     "per-commit",
			Threshold:   1.0,
			Checks: []evalgate.Check{
				{
					Name:   "probe-called-k-gte-3",
					Passed: cfg.DeterminismProbed,
					Weight: 1.0,
				},
			},
		},
	}
}
