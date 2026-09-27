package config

// INVALID examples — kept commented so `cue vet ./...` stays green; the Go
// test (internal/config/schema_test.go) asserts each fails when uncommented
// into a unification.
//
// 1. A literal secret: #Profile is a closed struct with no api_key field,
//    so pasting a raw key into the file is a hard validation error, not a
//    silently-ignored key:
// bad1: #Config & {
// 	profiles: leaked: {
// 		provider: "anthropic"
// 		api_key:  "sk-ant-not-representable"
// 	}
// }
//
// 2. An unknown provider value (not one of the six lanes):
// bad2: #Config & {
// 	profiles: typo: {
// 		provider: "anthropic-cloud"
// 	}
// }
//
// 3. An empty api_key_cmd — must name at least the executable:
// bad3: #Config & {
// 	profiles: broken: {
// 		provider:     "openai"
// 		api_key_cmd: []
// 	}
// }
//
// 4. A profile missing its required provider field:
// bad4: #Config & {
// 	profiles: noprovider: {
// 		model: "gpt-4o"
// 	}
// }
//
// 5. An unrecognised daemon.state_dir value (O63 item 5) — the only
//    recognised value is "workspace" (the in-tree opt-out); anything else,
//    including an absolute path, is rejected rather than silently
//    misinterpreted:
// bad5: #Config & {
// 	daemon: state_dir: "/abs/path"
// }
//
// 6. An unknown [decide] provider value (ADR-042: "none", "llm", or
//    "typesafe" only):
// bad6: #Config & {
// 	decide: provider: "jev"
// }
//
// 7. A [decide] threshold outside [0,1]:
// bad7: #Config & {
// 	decide: compile_threshold: 1.5
// }
//
// 8. A literal secret in [decide] (ADR-042 amendment, 2026-09-22): #Decide
//    is a closed struct with no api_key field, same discipline as #Profile
//    — the key must come from api_key_env or api_key_cmd, never a literal:
// bad8: #Config & {
// 	decide: {
// 		provider: "typesafe"
// 		api_key:  "sk-or-not-representable"
// 	}
// }
//
// 9. A daemon.memory_limit that is neither Go size syntax nor "off" (W-N,
//    docs/plans/daemon-footprint.md) — this pattern only bounds the SHAPE;
//    internal/config.ParseMemoryLimit still rejects a shape-valid but
//    nonsensical value (e.g. a unit-less number) at load time:
// bad9: #Config & {
// 	daemon: memory_limit: "3 gigs"
// }
//
// 10. An unrecognised [decide] sites entry (ADR-044 decision 8): closed to
//     the five sites that exist today ("route", "compile_prefilter",
//     "obligations_rerank", "certify_suggest", "assess") — an entry outside that set
//     is refused by name, the same discipline as provider's own enum:
// bad10: #Config & {
// 	decide: sites: ["route", "made_up_site"]
// }
//
// 12. A [tenancy] budget that is neither "auto" nor an IEC size (ADR-047,
//     design §1):
// bad12: #Config & {
// 	tenancy: budget: "4 gigs"
// }
//
// 13. A [tenancy] shed mark above the budget (shed_at is a fraction ≤ 1):
// bad13: #Config & {
// 	tenancy: shed_at: 1.5
// }
//
// 14. A swap-used FRACTION trigger: there is none (design §2.1, RT-154 —
//     macOS grows swap on demand, so used/total is routinely 80% at normal
//     pressure); the closed [tenancy.host] table refuses it:
// bad14: #Config & {
// 	tenancy: host: swap_used_fraction: 0.8
// }
//
// 15. An unknown [tenancy] admission mode:
// bad15: #Config & {
// 	tenancy: admission: "strict"
// }
