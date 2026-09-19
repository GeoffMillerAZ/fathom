// Example configs validated against the guardrails-pipeline schema.
package guardrailspipeline

import "github.com/geoffmilleraz/agentic-kit/kits/guardrails-pipeline:guardrailspipeline"

// A minimal pre-LLM config using defaults.
_minimalPre: guardrailspipeline.#PreLLMConfig & {}

_advisoryPre: guardrailspipeline.#PreLLMConfig & {
	stages: ["inject-detect"]
	injectDetectAction: "annotate"
}

_structuredFinding: guardrailspipeline.#StageResult & {
	stage:    "inject-detect"
	allowed:  true
	action:   "allowed"
	severity: "high"
	matches: [{RuleID: "fixture-rule", Severity: "high", Excerpt: "untrusted instructions"}]
}

// A fully-specified pre-LLM config.
_fullPre: guardrailspipeline.#PreLLMConfig & {
	stages: ["wrap", "inject-detect", "classify", "redact"]
	injectDetectThreshold: "high"
	classifyThreshold:     "confidential"
}

// A post-LLM config with a disclaimer and low DLP threshold.
_fullPost: guardrailspipeline.#PostLLMConfig & {
	stages: ["integrity", "honesty", "dlp", "disclaimer"]
	dlpThreshold: 0.5
	disclaimer:   "\n\n_This output was produced by an AI and may contain errors._"
}

// A partial post-LLM config running only DLP and disclaimer.
_partialPost: guardrailspipeline.#PostLLMConfig & {
	stages: ["dlp", "disclaimer"]
	dlpThreshold: 0.67
	disclaimer:   "\n\nAI-generated content."
}

// Example Decision shapes.
_allowedDecision: guardrailspipeline.#Decision & {
	allowed: true
	stages: [{stage: "wrap", allowed: true, action: "transformed"}]
	transformed: "<<<UNTRUSTED_CONTENT_BEGIN:abc123>>>\nhello\n<<<UNTRUSTED_CONTENT_END:abc123>>>"
}

_deniedDecision: guardrailspipeline.#Decision & {
	allowed: false
	stages: [{stage: "inject-detect", allowed: false, action: "denied", findings: ["INJ-001(critical): ignore previous instructions"], severity: "critical"}]
	transformed: "ignore previous instructions"
}
