package project

// One value from the files a repository commonly holds: a switchboard, a
// custodian contract, a binding, a guard and a layer rule, each under its
// own top-level key.
composed: #Project & {
	capabilities: guards: on: true
	budgets: list:         1500
	packs: ["codex", "taskfile"]
	concerns: router_thin: {
		want:  "the root AGENTS.md stays a router"
		check: {kind: "max_lines", file: "AGENTS.md", lines: 120}
	}
	bindings: [{module: "platform-standards", max_rules: 2}]
	guards: [{
		scopes: ["internal/store/**"]
		note: "Store schema is spec-governed."
	}]
}

// The project pack declares a generated output the same way a tech pack does.
generatedPair: #Project & {
	generated: [{output: "ui/app.css", source: ["ui/input.css"], regenerate: "task ui:css"}]
}

// The empty project is valid: every key is optional.
empty: #Project & {}
