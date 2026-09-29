package trace

// A mapping with exact ids and a pattern.
full: #StandardMapping & {
	standard: "example-std"
	title:    "Example standard (repository-authored)"
	clauses: [
		{id: "C-1", title: "Changes are traceable", requirements: ["CX-14", "CX-16/accept"]},
		{id: "C-2", patterns: ["^CX-1[0-9]$"]},
	]
}

// A clause mapped to nothing is legal (reported as a gap in the pack).
gap: #StandardMapping & {
	standard: "gaps-only"
	clauses: [{id: "C-9"}]
}
