package dependencyedges

sample: #Batch & {
	schema_version: 1
	files: [
		{
			path:     "main.go"
			dialect:  "go"
			measured: false
			trusted:  false
			complete: false
			edges: [
				{from: "main.go", kind: "config-ref", raw: "go.mod", to: "go.mod", resolution: "file", version: "", line: 0, provenance: "builtin"},
				{from: "main.go", kind: "import", raw: "fmt", to: "go:fmt", resolution: "external", version: "", line: 2, provenance: "builtin"},
			]
			unknowns: [
				{from: "main.go", code: "unmeasured-reader", raw: "", line: 0, decidable_by: []},
				{from: "main.go", code: "external-unpinned", raw: "fmt", line: 2, decidable_by: []},
			]
		},
		{
			path:     "solo.go"
			dialect:  "go"
			measured: false
			trusted:  true
			complete: true
			edges: []
			unknowns: []
		},
		{
			path:     "pinned.go"
			dialect:  "go"
			measured: false
			trusted:  true
			complete: true
			edges: [{from: "pinned.go", kind: "import", raw: "example.org/lib", to: "go:example.org/lib", resolution: "external", version: "v1.0.0", line: 2, provenance: "extension"}]
			unknowns: []
		},
	]
}

exchange: #DeclaredExport & [{
	from: {ecosystem: "repo", name: "main.go"}
	to: {ecosystem: "go", name: "fmt"}
}]
