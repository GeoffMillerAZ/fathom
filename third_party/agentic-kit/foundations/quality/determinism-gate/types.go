// Package determinismgate provides structural and variance-based determinism
// checking for LLM judge outputs in the eval loop.
//
// # Layer model (INVERTED from naive approach)
//
// PRIMARY — always-on structural layer:
//  1. Schema validation: judge outputs are validated against a CUE/JSON schema
//     embedded in ProbeOptions.OutputSchema. Structural invalidity is detected
//     on every run, not sampled.
//  2. K-run variance probe: Probe(ctx, judge, scenario, K, opts) runs the judge
//     K times and measures agreement rate, per-field Shannon entropy, and
//     verdict-flip count. Returns a ProbeResult with a measured band — callers
//     MUST consume the measured band rather than a hardcoded constant.
//
// OPPORTUNISTIC — sampling layer (temperature/seed pinning):
// Pin temperature or seed only when the model catalog confirms the target
// accepts those knobs (modelcatalog.Caps.Temperature / .Seed). Never construct
// a request with a parameter the catalog says is rejected. This is a
// BEST-EFFORT layer: hosted models cannot be made deterministic by knob pinning;
// hard determinism requires replay cassettes only.
//
// # Non-guarantees (documented, not wished away)
//
// This package CANNOT make hosted models deterministic. Variance is inherent in
// frontier model inference. What this package provides:
//   - Detection of measured agreement rate below a caller-specified threshold.
//   - Evidence that a judge's output structure is valid (schema layer).
//   - A measured stability band for downstream confidence intervals.
//
// Hard determinism (byte-identical outputs) is only achievable via replay
// cassettes (llmclient/replay). If a downstream consumer requires hard
// determinism, route it through a cassette-backed client.
//
// # Concurrency contract
//
// Probe is a pure function (modulo the judge's own concurrency guarantees).
// ProbeResult is an immutable value type; share across goroutines freely.
// ProbeOptions is read-only after construction; concurrent calls with the
// same ProbeOptions are safe.
package determinismgate

import (
	qaharness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/qa-harness"
	scenariobank "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/scenario-bank"
)

// Stability is the measured variance verdict.
type Stability string

const (
	// StabilityStable: measured agreement_rate >= opts.StabilityBand.
	StabilityStable Stability = "stable"
	// StabilityUnstable: measured agreement_rate < opts.StabilityBand.
	StabilityUnstable Stability = "unstable"
)

// ProbeResult is the output of Probe. It is an immutable value type.
//
// CUE mirror: #ProbeResult in schema.cue.
type ProbeResult struct {
	// ScenarioID identifies the probed scenario.
	ScenarioID string `json:"scenario_id"`

	// K is the number of runs performed.
	K int `json:"k"`

	// AgreementRate is the pairwise agreement fraction in [0,1]. For K=1 it is
	// always 1.0 (trivially stable — run more iterations for meaningful results).
	AgreementRate float64 `json:"agreement_rate"`

	// VerdictFlipCount is the number of (i, j) run pairs where the verdict
	// differed (i < j, K*(K-1)/2 total pairs examined).
	VerdictFlipCount int `json:"verdict_flip_count"`

	// PerFieldEntropy maps field name to Shannon entropy in bits, measured
	// across the K runs. Zero entropy = identical values every run. Higher
	// values indicate more variability in that field.
	//
	// Fields measured: "score", "verdict".
	PerFieldEntropy map[string]float64 `json:"per_field_entropy"`

	// Stability is the overall verdict: StabilityStable when AgreementRate >=
	// opts.StabilityBand, StabilityUnstable otherwise.
	Stability Stability `json:"stability"`

	// StabilityBand is the AgreementRate threshold used for this probe.
	// Callers MUST consume this measured band rather than a hardcoded constant.
	StabilityBand float64 `json:"stability_band"`
}

// ProbeOptions configures a Probe call.
type ProbeOptions struct {
	// StabilityBand is the minimum agreement_rate to qualify as Stable, in
	// [0,1]. Must be in [0,1]; defaults to 1.0 (all runs agree) when zero.
	// Callers should pick a band informed by their tolerance for variance.
	StabilityBand float64

	// SchemaValidator, when non-nil, is called on each judge output
	// (Item.Output) to validate structural correctness. A non-nil error means
	// the output is structurally invalid and is NOT counted in variance metrics
	// (it is counted as an error run). When nil, structural validation is skipped.
	//
	// Typical implementation: parse Item.Output as JSON and validate against
	// the CUE schema (toolchains/jsonschema + cue.Value.Validate).
	SchemaValidator func(output string) error
}

// compiletime checks — ensure we import the packages we depend on.
var (
	_                 = scenariobank.Scenario{}
	_ qaharness.Judge = qaharness.MockJudge{}
)
