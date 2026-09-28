package trace

// INVALID examples — kept commented so `cue vet ./...` stays green; the Go
// test (internal/review/tracemapping_schema_test.go) asserts each fails when
// uncommented into a unification (repo convention, schema/CLAUDE.md).
//
// 1. No clauses (at least one is required):
// bad1: #StandardMapping & {
// 	standard: "empty"
// 	clauses: []
// }
//
// 2. A standard id that is not a lower-case slug:
// bad2: #StandardMapping & {
// 	standard: "Has Spaces"
// 	clauses: [{id: "C-1"}]
// }
//
// 3. A clause with an empty id:
// bad3: #StandardMapping & {
// 	standard: "x"
// 	clauses: [{id: ""}]
// }
