// Package evalrunner — valid test data that must unify with schema.cue.
//
// EXAMPLE VALUES ONLY: model IDs, family tags, and dollar figures are
// illustrative placeholders chosen to exercise the schema, not real provider
// identifiers or prices. Real values are always caller data.
//
// Negative cases (documented here, proven by Go rejection tests):
//   - JudgeLanes: []                     → fails (at least one lane required)
//   - JudgeLanes: [{Name: "", ...}]      → fails (empty lane name)
//   - Producer: {Model: ""}              → fails (empty producer model)
//   - Mode: "record"                     → fails (not in #Mode)
//   - DeterminismK: -1                   → fails (>=0)
//   - StabilityBand: 1.5                 → fails (<=1)
//   - Budget: {MaxUSDPerDay: 0}          → fails (>0)
package evalrunner

import "github.com/geoffmilleraz/agentic-kit/kits/eval-runner:evalrunner"

// validMinimal is the smallest possible valid configuration: one judge lane,
// a producer, replay mode; every optional field absent, so the Go defaults
// apply (no determinism probe, only verdict flips count as regressions,
// gate threshold 1.0).
validMinimal: evalrunner.#EvalConfig & {
	RunID: "chat-prompts"
	JudgeLanes: [{
		Name:   "primary"
		Model:  "example-judge-large"
		Family: "example-family-a"
	}]
	Producer: {Model: "example-producer-large"}
	Mode: "replay"
}

// validFull is a fully specified replay-mode configuration as `task eval`
// would use it: two cross-family judge lanes, budget limits for the
// composition root's guard, a determinism probe of K=3, and regression-gate
// thresholds with explicit units (fractions of score in [0,1]).
validFull: evalrunner.#EvalConfig & {
	RunID: "chat-prompts"
	JudgeLanes: [{
		Name:   "primary"
		Model:  "example-judge-large"
		Family: "example-family-a"
	}, {
		Name:   "secondary"
		Model:  "example-judge-small"
		Family: "example-family-b"
	}]
	Producer: {
		Model:      "example-producer-large"
		PromptHash: "sha256:0000000000000000000000000000000000000000000000000000000000000000"
		ParamsHash: "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	}
	Mode:               "replay"
	Independence:       {SameSession: false, SharedHistory: false}
	ScenarioBankPath:   "testdata/scenarios.json"
	BaselinePath:       "testdata/baseline.jsonl"
	PriceTableSource:   "testdata/prices.json"
	Budget:             {MaxReqPerMin: 60, MaxReqPerDay: 500, MaxUSDPerDay: 5.0}
	DeterminismK:       3
	StabilityBand:      0.9
	ScoreDropThreshold: 0.05
	GateThreshold:      1.0
	JudgeMaxTokens:     1024
	JudgeSystem:        "You are an impartial evaluation judge."
}

// validNonDecisive documents the escape hatch: a config whose judge and
// producer share a family constructs ONLY with NonDecisive set, and no
// record from it is ever marked decisive.
validNonDecisive: evalrunner.#EvalConfig & {
	RunID: "same-family-smoke"
	JudgeLanes: [{
		Name:   "primary"
		Model:  "example-judge-large"
		Family: "example-family-a"
	}]
	Producer: {Model: "example-producer-sibling"}
	Mode:        "replay"
	NonDecisive: true
}
