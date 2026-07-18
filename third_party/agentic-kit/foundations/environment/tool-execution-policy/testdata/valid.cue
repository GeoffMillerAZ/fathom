// valid.cue — positive fixtures for every type in schema.cue.
// Every value here must unify cleanly on `cue vet ./...`.
package toolexecutionpolicy

import "github.com/geoffmilleraz/agentic-kit/foundations/environment/tool-execution-policy:toolexecutionpolicy"

// allowRule demonstrates a rule that explicitly permits a tool.
allowRule: toolexecutionpolicy.#ToolPolicy & {
	Tool:         "web_search"
	Allow:        true
	Environments: ["dev", "staging"]
}

// denyRule demonstrates a rule that explicitly denies a tool in production.
denyRule: toolexecutionpolicy.#ToolPolicy & {
	Tool:         "file_delete"
	Allow:        false
	Environments: ["prod"]
}

// emptyEnvironments demonstrates that an empty Environments list is valid.
inertRule: toolexecutionpolicy.#ToolPolicy & {
	Tool:         "code_runner"
	Allow:        true
	Environments: []
}
