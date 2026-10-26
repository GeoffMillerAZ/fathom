package hookeval

import "github.com/geoffmilleraz/agentic-kit/internal/hookeval:hookeval"

_digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

request: hookeval.#Request & {
	version:       "agent-hooks.spawn-request/1"
	requestId:     _digest
	contextSHA256: _digest
	policySHA256:  _digest
	policyPayload: "e30="
	subject: {
		host:        "codex"
		operation:   "spawn"
		model:       "m"
		effort:      "low"
		agentType:   "worker"
		fork:        false
		contextFork: false
		brief:       "work"
	}
	advisorySelected: false
}

result: hookeval.#Result & {
	version:           "agent-hooks.spawn-result/1"
	requestId:         _digest
	contextSHA256:     _digest
	policySHA256:      _digest
	noActionAuthority: true
	requiredDecision:  "pass"
	codes: ["allowed"]
	class:             "worker"
	classSource:       "default"
	advisoryEvaluated: false
	adviceMatches:     0
}
