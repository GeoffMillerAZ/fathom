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

// validStrictEffects opts into the strict effect gate: an undeclared or
// destructive tool call needs a human approval before it dispatches.
validStrictEffects: agentharness.#Config & {
	MaxTurns:     6
	EffectPolicy: "strict"
}

// validHandoff is the record a run emits when it stops on max_turns after
// two tool calls.
validHandoff: agentharness.#Handoff & {
	stop_cause:     "max_turns"
	turns:          2
	last_tool:      "word_count"
	checkpoint_ref: "checkpoint:v1:turn2:0123456789abcdef"
	tried: ["read_file", "word_count"]
	summary: "run stopped on max_turns after 2 turn(s); last tool word_count; tried [read_file, word_count]; resume from checkpoint:v1:turn2:0123456789abcdef"
}

// validHandoffNoTools is the record of a run that stopped on budget before
// any tool was called.
validHandoffNoTools: agentharness.#Handoff & {
	stop_cause: "budget"
	turns:      0
	summary:    "run stopped on budget after 0 turn(s); no tool was called"
}
