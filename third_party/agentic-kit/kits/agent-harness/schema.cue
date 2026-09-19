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

// #EffectPolicy mirrors the Go EffectPolicy enum: how the harness treats a
// tool's declared effect at dispatch. "permissive" dispatches whatever the
// registry and hooks allow; "strict" routes undeclared and destructive
// dispatches through a human approval first.
#EffectPolicy: "permissive" | "strict"

// #ControlStatus mirrors the receipt status vocabulary.
#ControlStatus: "pass" | "fail" | "unknown"

// #AttemptUsage mirrors one attempt's observable cost. An omitted ActualUSD
// means unknown; invalid runtime values fail closed in Go.
#AttemptUsage: {ActualUSD?: number & >=0}

// #CompletionEvidence mirrors independently supplied completion evidence.
#CompletionEvidence: {ID: string, Independent: bool}

// #EngineeringProfileConfig mirrors EngineeringProfileConfig's data fields.
// Behavioral controls are required by NewEngineeringProfile, including typed
// non-nil interfaces that CUE cannot represent.
#EngineeringProfileConfig: {
	Harness: #Config & {
		MaxTokensPerCall: int & >0
		EstimatedUSDPerCall: 0
		EffectPolicy: "strict"
		InjectionAction: "block"
		// Absent uses the Harness's true default; false would turn truncated
		// output into a passing completion and is refused by the profile.
		TreatMaxTokensAsError?: true
	}
	MaxAttempts: int & >=1
	ReservedUSDPerAttempt: number & >0
	AllowedTools: [string & !="", ...string & !=""]
}

// #AttemptAccounting mirrors attempt accounting in CoverageReceipt.
#AttemptAccounting: {
	Attempts: int & >=0
	ReservedUSD: number & >=0
	SpentUSD: number & >=0
	UnknownActuals: int & >=0
	InvalidActuals: int & >=0
}

// #CoverageReceipt mirrors the runtime receipt. Control keys are runtime
// observations; an absent/unrun control is represented by "unknown".
#CoverageReceipt: {
	Version: string & !=""
	Controls: [string]: #ControlStatus
	AllowedTools: [...string & !=""]
	Accounting: #AttemptAccounting
	NonGuarantees: [...string & !=""]
}

// #EngineeringResult mirrors EngineeringResult's JSON envelope. RunResult is
// the existing harness runtime record; its detailed shape remains owned by
// the harness API rather than duplicated by this profile schema.
#EngineeringResult: {
	Run: _
	Evidence: #CompletionEvidence
	Coverage: #CoverageReceipt
}

// #Handoff mirrors the Go Handoff record: the receipt of one run that
// stopped on budget, max_tokens or max_turns. Derived from the RunResult,
// never stored by the kit.
#Handoff: {
	// stop_cause is why the run stopped.
	stop_cause: "budget" | "max_tokens" | "max_turns"

	// turns is the number of completed model calls.
	turns: int & >=0

	// last_tool is the last tool the model called; absent when none was.
	last_tool?: string & !=""

	// checkpoint_ref is CheckpointRef(RunResult.Final).
	checkpoint_ref?: =~"^checkpoint:v[0-9]+:turn[0-9]+:[0-9a-f]{16}$"

	// tried lists the distinct tool names called, in first-call order.
	tried?: [...string & !=""]

	// summary is the one-line human rendering of the record.
	summary: string & !=""
}

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

	// EffectPolicy: absent defaults to "permissive" in Go.
	EffectPolicy?: #EffectPolicy
}
