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
			edges: [{from: "main.go", kind: "import", raw: "fmt", to: "go:fmt", resolution: "external", version: "", line: 2, provenance: "builtin"}]
			unknowns: [{from: "main.go", code: "external-unpinned", raw: "fmt", line: 2, decidable_by: []}]
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
	]
}

exchange: #DeclaredExport & [{
	from: {ecosystem: "repo", name: "main.go"}
	to: {ecosystem: "go", name: "fmt"}
}]
