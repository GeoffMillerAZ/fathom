// A concerns file in the shape fathom config audit reads (schema/project #Concern).
packs: ["claude-code", "taskfile"]

// Hidden: a documents table the comprehension below reads.
_docs: {
	intro_thin:  "docs/intro.md"
	guide_thin:  "docs/guide.md"
	notes_thin:  "docs/notes.md"
}

concerns: {
	deploy_safety: {
		want:  "no raw-cp deploy instruction anywhere in agent config"
		why:   "a copied binary was never signed"
		check: {kind: "forbidden_pattern", pattern: "cp bin/tool", where: "agent-config"}
	}
	router_thin: {
		want:  "the root AGENTS.md stays a router"
		check: {kind: "max_lines", file: "AGENTS.md", lines: 120}
	}
	pointers_resolve_off: {off: true}
	for id, doc in _docs {
		(id): {
			want:  "\(doc) stays within its line budget"
			check: {kind: "max_lines", file: doc, lines: 150}
		}
	}
}
