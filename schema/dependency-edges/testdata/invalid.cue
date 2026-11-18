package dependencyedges

// These negative examples are compiled individually by TestDependencyEdgeSchema.
// case: {schema_version: 1, unexpected: true, files: []}
// case: {schema_version: 1, files: [{path: "/absolute.go", dialect: "go", measured: true, trusted: false, complete: true, edges: [], unknowns: []}]}
// case: {schema_version: 1, files: [{path: "sub/../x.go", dialect: "go", measured: true, trusted: false, complete: true, edges: [], unknowns: []}]}
// case: {schema_version: 1, files: [{path: "main.go", dialect: "go", measured: false, trusted: false, complete: true, edges: [], unknowns: []}]}
// case: {schema_version: 1, files: [{path: "main.go", dialect: "go", measured: true, trusted: false, complete: true, edges: null, unknowns: []}]}
// case: {schema_version: 1, files: [{path: "main.go", dialect: "go", measured: true, trusted: false, complete: true, edges: [], unknowns: [{from: "main.go", code: "parse-error", raw: "", line: 1, decidable_by: []}]}]}
// case: {schema_version: 1, files: [{path: "main.go", dialect: "go", measured: true, trusted: false, complete: false, edges: [], unknowns: [{from: "main.go", code: "invented", raw: "", line: 1, decidable_by: []}]}]}
// case: {schema_version: 1, files: [{path: "main.go", dialect: "go", measured: true, trusted: false, complete: false, edges: [], unknowns: [{from: "main.go", code: "parse-error", raw: "", line: 1, decidable_by: ["guesses"]}]}]}
// case: {schema_version: 1, files: [{path: "main.go", dialect: "go", measured: true, trusted: false, complete: false, edges: [{from: "main.go", kind: "import", raw: "", to: "go:https://example.org", resolution: "external", version: "", line: 1, provenance: "builtin"}], unknowns: []}]}
// case: {schema_version: 1, files: [{path: "main.go", dialect: "go", measured: false, trusted: true, complete: true, edges: [{from: "other.go", kind: "include", raw: "", to: "input.txt", resolution: "file", version: "", line: 1, provenance: "declared"}], unknowns: []}]}
// case: {schema_version: 1, files: [{path: "main.go", dialect: "go", measured: false, trusted: true, complete: true, edges: [{from: "main.go", kind: "import", raw: "fmt", to: "go:fmt", resolution: "external", version: "", line: 2, provenance: "builtin"}], unknowns: []}]}
// export: {declared_edges: []}
// export: [{from: {ecosystem: "repo", name: "main.go"}, to: {ecosystem: "go", name: "https://example.org"}}]
// export: [{from: {ecosystem: "repo", name: "../main.go"}, to: {ecosystem: "repo", name: "input.txt"}}]
// export: [{from: {ecosystem: "repo", name: "main.go"}, to: {ecosystem: "repo", name: "input.txt", version: "1"}}]
