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
// 3. A tier the v1+M1 claim model does not admit ("verified" was added by
//    the M1 amendment for deterministic Model-empty compile paths — see
//    ADR-016's amendment note in ADR.md — but "ai-derived" still awaits its
//    own revival evidence per ADR-016 §4.2):
// bad3: #Layer & {
// 	name: "x", version: "v1", kind: "x"
// 	corpus: include: ["docs/**"]
// 	prompt_version: "x/v1"
// 	tier: "ai-derived"
// 	budgets: {max_tokens_per_run: 1, max_usd_per_run: 0}
// }
//
// 4. A stratum outside the two values a #Layer instance can carry (ADR-024,
//    docs/design/pack-taxonomy.md): "dialect" and "provider" are recorded
//    strata with no #Layer instance — dialects are code, provider packs are
//    parked (TX-3) — so only "concern"/"platform" are admitted here.
// bad4: #Layer & {
// 	name: "x", version: "v1", kind: "x"
// 	corpus: include: ["docs/**"]
// 	prompt_version: "x/v1"
// 	tier: "doc-claimed"
// 	stratum: "provider"
// 	budgets: {max_tokens_per_run: 1, max_usd_per_run: 0}
// }
