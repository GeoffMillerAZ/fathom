package semantic

// INVALID examples — kept commented so `cue vet ./...` stays green; the Go
// test (internal/semantic/schema_test.go) asserts each fails when uncommented
// into a unification.
//
// 1. Empty corpus include (a layer must select a corpus):
// bad1: #Layer & {
// 	name: "x", version: "v1", kind: "x"
// 	corpus: include: []
// 	prompt_version: "x/v1"
// 	tier: "doc-claimed"
// 	budgets: {max_tokens_per_run: 1, max_usd_per_run: 0}
// }
//
// 2. Serving cap above the ≤6-rule payload contract:
// bad2: #Layer & {
// 	name: "x", version: "v1", kind: "x"
// 	corpus: include: ["docs/**"]
// 	prompt_version: "x/v1"
// 	tier: "doc-claimed"
// 	budgets: {max_tokens_per_run: 1, max_usd_per_run: 0}
// 	serving: max_rules: 12
// }
//
// 3. A tier the v1 claim model does not admit:
// bad3: #Layer & {
// 	name: "x", version: "v1", kind: "x"
// 	corpus: include: ["docs/**"]
// 	prompt_version: "x/v1"
// 	tier: "verified"
// 	budgets: {max_tokens_per_run: 1, max_usd_per_run: 0}
// }
