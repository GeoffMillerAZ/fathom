package binding

// INVALID examples — kept commented so `cue vet ./...` stays green; the Go
// test (internal/bindspec/schema_test.go) asserts each fails when
// uncommented into a unification (repo convention, CLAUDE.md "New CUE
// schemas").
//
// 1. Empty bindings list — an empty file has nothing to scaffold as valid;
//    `fathom init`'s commented template is the honest way to ship "no
//    bindings configured yet":
// bad1: #Bindings & {
// 	bindings: []
// }
//
// 2. Bad module name shape:
// bad2: #Bindings & {
// 	bindings: [{module: "Not_Valid"}]
// }
//
// 3. max_rules must be positive:
// bad3: #Bindings & {
// 	bindings: [{module: "platform-standards", max_rules: 0}]
// }
//
// 4. min_tier must be one of the recognised evidence tiers:
// bad4: #Bindings & {
// 	bindings: [{module: "platform-standards", min_tier: "gold"}]
// }
//
// 5. Empty scopes list — an author who writes `scopes: []` almost
//    certainly meant "omit scopes" (module's own scopes apply), not "match
//    nothing"; the non-empty-list constraint makes that typo loud instead
//    of silently deadening the binding:
// bad5: #Bindings & {
// 	bindings: [{module: "platform-standards", scopes: []}]
// }
