package hookeval

import (
	"list"
)

#Digest: =~"^[0-9a-f]{64}$"
#Subject: {
	host!:        "claude" | "codex"
	operation!:   "spawn" | "resume"
	model!:       string
	effort!:      string
	agentType!:   string
	fork!:        bool
	contextFork!: bool
	brief!:       string
	if len(model) > 256 {model: _|_}
	if len(effort) > 64 {effort: _|_}
	if len(agentType) > 128 {agentType: _|_}
	if len(brief) > 65536 {brief: _|_}
	if host == "claude" && contextFork {contextFork: _|_}
	if host == "codex" && fork {fork: _|_}
	if operation == "resume" && (host != "claude" || fork || contextFork) {operation: _|_}
}

#Request: {
	version!:       "agent-hooks.spawn-request/1"
	requestId!:     #Digest
	contextSHA256!: #Digest
	policySHA256!:  #Digest
	policyPayload!: string & =~"^[A-Za-z0-9+/]+={0,2}$"
	if len(policyPayload) > 349528 {policyPayload: _|_}
	subject!:          #Subject
	advisorySelected!: bool
}

#Denial:         "no_model" | "unknown_model" | "tier_ceiling" | "no_effort" | "fork_unmarked" | "fork_denied"
#ResolvedSource: "worker_marker" | "critic_marker" | "critic_pattern" | "default"
#Result: {
	version!:           "agent-hooks.spawn-result/1"
	requestId!:         #Digest
	contextSHA256!:     #Digest
	policySHA256!:      #Digest
	noActionAuthority!: true
	requiredDecision!:  "pass" | "deny" | "unknown" | "not-applicable"
	codes!: [...("allowed" | #Denial | "policy_invalid" | "profile_invalid" | "resume_not_spawn")] & list.MaxItems(10) & list.MinItems(1)
	if !list.UniqueItems(codes) {codes: _|_}
	class!:             "worker" | "critic" | "unknown" | "not-applicable"
	classSource!:       #ResolvedSource | "unknown" | "not-applicable"
	advisoryEvaluated!: bool
	adviceMatches!:     int & >=0 & <=32
	if requiredDecision == "pass" {
		if len(codes) != 1 || codes[0] != "allowed" {codes: _|_}
		class:       "worker" | "critic"
		classSource: #ResolvedSource
	}
	if requiredDecision == "deny" {
		codes: [#Denial, ...#Denial]
		class:       "worker" | "critic"
		classSource: #ResolvedSource
	}
	if requiredDecision == "unknown" {
		codes: ["policy_invalid" | "profile_invalid", ...("policy_invalid" | "profile_invalid")]
		if class != "unknown" {class: _|_}
		if classSource != "unknown" {classSource: _|_}
	}
	if requiredDecision == "not-applicable" {
		if len(codes) != 1 || codes[0] != "resume_not_spawn" {codes: _|_}
		if class != "not-applicable" {class: _|_}
		if classSource != "not-applicable" {classSource: _|_}
	}
	if requiredDecision != "pass" && advisoryEvaluated {advisoryEvaluated: _|_}
	if requiredDecision != "pass" && adviceMatches != 0 {adviceMatches: _|_}
	if !advisoryEvaluated && adviceMatches != 0 {adviceMatches: _|_}
	if class == "worker" {classSource: "worker_marker" | "default"}
	if class == "critic" {classSource: "critic_marker" | "critic_pattern"}
}
