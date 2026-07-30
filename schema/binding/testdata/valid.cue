package binding

// A well-formed binding file (docs/design/platform-modules.md §3.1's worked
// example): one binding restricting scope, one binding leaning on schema
// defaults (max_rules: 2, no scope restriction, no tier floor).
withScopeAndTier: #Bindings & {
	bindings: [{
		module:    "platform-standards"
		scopes:    ["infra/**", "**/*.go"]
		max_rules: 3
		min_tier:  "doc-claimed"
	}]
}

minimalDefaults: #Bindings & {
	bindings: [{
		module: "platform-standards"
	}]
}

multipleBindings: #Bindings & {
	bindings: [
		{module: "platform-standards"},
		{module: "security-baseline", scopes: ["**/*.tf"], max_rules: 1},
	]
}
