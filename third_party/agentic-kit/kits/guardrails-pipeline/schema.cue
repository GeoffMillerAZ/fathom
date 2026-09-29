// Package guardrails-pipeline CUE contract.
// #PreLLMConfig, #PostLLMConfig, #Decision, and #StageResult mirror the Go
// structs in types.go. CUE is the authority for shape; Go is the authority
// for behaviour.
package guardrailspipeline

// _preLLMStages enumerates the valid pre-LLM stage names.
_preLLMStages: "wrap" | "inject-detect" | "classify" | "redact"

// _postLLMStages enumerates the valid post-LLM stage names.
_postLLMStages: "integrity" | "honesty" | "dlp" | "disclaimer"

// #PreLLMConfig is the configuration for a pre-LLM gate chain.
#PreLLMConfig: {
	// stages lists the stage names in execution order.
	// An empty list runs all four in default order.
	stages?: [..._preLLMStages]

	// injectDetectThreshold is the minimum severity that triggers deny.
	// Valid values: "low" | "medium" | "high" | "critical". Default: "high".
	injectDetectThreshold?: "low" | "medium" | "high" | "critical"
	injectDetectAction?:    "block" | "annotate"

	// classifyThreshold is the minimum sensitivity level that triggers deny.
	// Valid values: "public" | "internal" | "confidential" | "restricted". Default: "confidential".
	classifyThreshold?: "public" | "internal" | "confidential" | "restricted"
}

// #PostLLMConfig is the configuration for a post-LLM gate chain.
#PostLLMConfig: {
	// stages lists the stage names in execution order.
	// An empty list runs all four in default order.
	stages?: [..._postLLMStages]

	// dlpThreshold is the minimum DLP score [0, 1] that triggers deny.
	// Default: 0.67.
	dlpThreshold?: number & >=0 & <=1

	// disclaimer is text appended to the transformed output. Empty = no-op.
	disclaimer?: string
}

// #StageResult is the outcome of one pipeline stage.
#StageResult: {
	stage:   string & !=""
	allowed: bool
	action:  "allowed" | "denied" | "skipped" | "transformed" | "error"
	findings?: [...string]
	matches?: [...{
		RuleID:   string & !=""
		Severity: "low" | "medium" | "high" | "critical"
		Excerpt:  string
	}]
	severity?: string
}

// #Decision is the outcome of a chain Run call.
#Decision: {
	allowed: bool
	stages: [...#StageResult]
	transformed: string
	// err is omitted from CUE (it is Go-only; excluded from JSON serialisation)
}
