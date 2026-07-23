package archconformance

// A well-formed rule pack — the shape a project's own
// .fathom/rules/archconformance.cue converts to (minus the package clause
// and "#RuleSet &" wrapper: a real rule pack is plain data, see schema.cue's
// doc comment).
example: #RuleSet & {
	rules: [
		{from_pkg: "connector", to_pkg: "query", allow: false},
		{from_pkg: "extractor", to_pkg: "mcp", allow: false},
		{from_pkg: "*", to_pkg: "store", allow: true},
	]
	deny_by_default: false
}
