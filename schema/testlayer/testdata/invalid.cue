package testlayer

// INVALID examples — kept commented so `cue vet ./...` stays green; the Go
// test (internal/testlayer/schema_test.go) asserts each fails when
// uncommented into a unification (repo convention, schema/CLAUDE.md).
//
// 1. Empty name:
// bad1: #Layers & {
// 	layers: [{name: ""}]
// }
//
// 2. Empty globs list — a name typo away from "match nothing" (same
//    reasoning as schema/guard's empty scopes/command_scopes case):
// bad2: #Layers & {
// 	layers: [{name: "x", globs: []}]
// }
//
// 3. Empty build_tags list:
// bad3: #Layers & {
// 	layers: [{name: "x", build_tags: []}]
// }
//
// 4. Empty dirs list:
// bad4: #Layers & {
// 	layers: [{name: "x", dirs: []}]
// }
