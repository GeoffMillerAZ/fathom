// Package determinismgate — valid test fixtures that must unify with schema.cue.
// Run `cue vet ./...` from the package root or `cue vet ./testdata/` to
// validate these against the schema.
package determinismgate

import "github.com/geoffmilleraz/agentic-kit/foundations/quality/determinism-gate:determinismgate"

// validProbeResultStable is a well-formed stable probe result.
validProbeResultStable: determinismgate.#ProbeResult & {
	scenario_id:        "scenario-001"
	k:                  5
	agreement_rate:     1.0
	verdict_flip_count: 0
	per_field_entropy: {
		score:   0.0
		verdict: 0.0
	}
	stability:      "stable"
	stability_band: 0.9
}

// validProbeResultUnstable is a well-formed unstable probe result.
validProbeResultUnstable: determinismgate.#ProbeResult & {
	scenario_id:        "scenario-002"
	k:                  5
	agreement_rate:     0.6
	verdict_flip_count: 4
	per_field_entropy: {
		score:   0.97
		verdict: 1.0
	}
	stability:      "unstable"
	stability_band: 0.9
}

// validProbeResultK1 demonstrates a single-run probe (trivially stable).
validProbeResultK1: determinismgate.#ProbeResult & {
	scenario_id:        "scenario-003"
	k:                  1
	agreement_rate:     1.0
	verdict_flip_count: 0
	per_field_entropy: {
		score:   0.0
		verdict: 0.0
	}
	stability:      "stable"
	stability_band: 1.0
}
