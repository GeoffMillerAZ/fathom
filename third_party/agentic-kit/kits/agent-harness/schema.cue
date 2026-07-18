// Package agentharness defines the schema for agent-harness configuration.
// These constraints mirror the data-shaped Go Config type (types.go) and are
// evaluated with `cue vet ./...`. Behavioral dependencies (the LLM client,
// tool registry, sinks, journal, clock) live in the Go Deps struct and have
// no CUE shape — CUE owns data, Go owns behavior.
package agentharness

// #InjectionAction mirrors the Go InjectionAction enum: what happens when
// the injection detector flags a tool result at or above
// InjectionMinSeverity.
#InjectionAction: "off" | "annotate" | "block"

// #Severity mirrors foundations/security/injection-detector severities,
// ordered low < medium < high < critical.
#Severity: "low" | "medium" | "high" | "critical"

// #Config holds the data-shaped settings of a Harness.
#Config: {
	// MaxTurns caps the number of model calls in one Run. Required.
	MaxTurns: int & >=1

	// System is the optional system prompt for every model call.
	System?: string

	// Model names the model; absent means the client's configured default.
	Model?: string

	// MaxTokensPerCall caps each response's length; absent/0 means the
	// model default.
	MaxTokensPerCall?: int & >=0

	// TreatMaxTokensAsError: absent or true surfaces a max_tokens stop
	// distinctly (stop cause "max_tokens"); false accepts the truncated
	// response as a normal completion.
	TreatMaxTokensAsError?: bool

	// InjectionAction: absent defaults to "block" in Go.
	InjectionAction?: #InjectionAction

	// InjectionMinSeverity: absent defaults to "high" in Go.
	InjectionMinSeverity?: #Severity

	// EstimatedUSDPerCall is the estimate fed to the budget guard before
	// every model call. Non-negative; absent/0 means only the guard's
	// request-count dimensions govern.
	EstimatedUSDPerCall?: number & >=0

	// ContextBudget, when > 0, is the token budget for context windowing before
	// each model call.
	ContextBudget?: int & >=0

	// ParallelTools: 0 or 1 = sequential dispatch (default). N > 1 = dispatch
	// up to N tool calls concurrently.
	ParallelTools?: int & >=0
}
