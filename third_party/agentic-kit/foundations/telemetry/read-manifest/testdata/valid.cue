// valid.cue — positive fixtures for the read-manifest schema.
package readmanifest

// complete: every considered path was read, genuinely absent, or
// deliberately skipped — a count over it renders as a plain number.
complete: #Manifest & {
	entries: [
		{path: "docs/intent/invariants.md", outcome: {kind: "read", bytes: 4096}},
		{path: "docs/intent/questions.md", outcome: {kind: "absent"}},
		{path: "foundations/x/y/y_test.go", outcome: {kind: "skipped", detail: "test-file"}},
		{path: "workspace", outcome: {kind: "skipped", detail: "sweep-skip-dir"}},
	]
}

// incomplete: one failed read — every count over it is a lower bound.
incomplete: #Manifest & {
	entries: [
		{path: "docs/intent/invariants.md", outcome: {kind: "unreadable", detail: "operation not permitted"}},
		{path: "docs/intent/scope.md", outcome: {kind: "read", bytes: 1200}},
	]
}

// empty: a manifest that considered nothing is complete by definition.
empty: #Manifest & {}
