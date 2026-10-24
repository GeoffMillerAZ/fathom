// A concerns file that exercises what this repository's own does not: a
// reference from a written concern to a hidden table, and two comprehensions
// that generate concerns from hidden tables.
packs: ["claude-code"]

_limits: {
	router: 120
	guide:  40
}

_guides: {
	docs_guide_thin:     "docs/AGENTS.md"
	internal_guide_thin: "internal/AGENTS.md"
}

concerns: {
	router_thin: {
		want:  "the root AGENTS.md stays a router"
		check: {kind: "max_lines", file: "AGENTS.md", lines: _limits.router}
	}
	gate_documented: {
		want:  "the gate is named in agent config and its target exists"
		check: {kind: "command_documented_and_exists", command: "task validate"}
	}
	stanza_check_off: {off: true}
	for id, doc in _guides {
		(id): {
			want:  "\(doc) stays within its line budget"
			check: {kind: "max_lines", file: doc, lines: _limits.guide}
		}
	}
}
