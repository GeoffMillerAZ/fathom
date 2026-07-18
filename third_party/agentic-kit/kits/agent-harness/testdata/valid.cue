// Package agentharness — valid test data that must unify with schema.cue.
//
// EXAMPLE VALUES ONLY: model names and dollar figures are illustrative
// placeholders chosen to exercise the schema, not real provider identifiers
// or prices.
package agentharness

import "github.com/geoffmilleraz/agentic-kit/kits/agent-harness:agentharness"

// validMinimal is the smallest possible valid configuration: every optional
// field absent, so the Go defaults apply (model default tokens, max_tokens
// surfaced distinctly, injection action "block" at severity "high").
validMinimal: agentharness.#Config & {
	MaxTurns: 1
}

// validGoverned is a fully specified governed-agent configuration.
validGoverned: agentharness.#Config & {
	MaxTurns:              8
	System:                "You are a careful research assistant. Use tools when they help."
	Model:                 "example-model-large"
	MaxTokensPerCall:      2048
	TreatMaxTokensAsError: true
	InjectionAction:       "block"
	InjectionMinSeverity:  "high"
	EstimatedUSDPerCall:   0.05
}

// validLenient accepts truncated responses and only annotates critical
// injection detections instead of blocking.
validLenient: agentharness.#Config & {
	MaxTurns:              3
	Model:                 "example-model-small"
	TreatMaxTokensAsError: false
	InjectionAction:       "annotate"
	InjectionMinSeverity:  "critical"
	EstimatedUSDPerCall:   0.001
}

// validScanOff disables injection scanning explicitly (equivalent to
// injecting no detector).
validScanOff: agentharness.#Config & {
	MaxTurns:        20
	InjectionAction: "off"
}
