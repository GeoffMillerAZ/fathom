package guard

// INVALID examples — kept commented so `cue vet ./...` stays green; the Go
// test (internal/guardspec/schema_test.go) asserts each fails when
// uncommented into a unification (repo convention, CLAUDE.md "New CUE
// schemas").
//
// 1. Empty guards list:
// bad1: #Guards & {
// 	guards: []
// }
//
// 2. max_rules must be positive:
// bad2: #Guards & {
// 	guards: [{scopes: ["**/*.go"], max_rules: 0}]
// }
//
// 3. Empty scopes list — an author who writes `scopes: []` almost
//    certainly meant "omit scopes", not "match nothing":
// bad3: #Guards & {
// 	guards: [{scopes: []}]
// }
//
// 4. Empty command_scopes list, same reasoning as case 3:
// bad4: #Guards & {
// 	guards: [{command_scopes: []}]
// }
