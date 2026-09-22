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
// 6. An unknown [decide] provider value (ADR-042: only "none" or "llm"):
// bad6: #Config & {
// 	decide: provider: "jev"
// }
//
// 7. A [decide] threshold outside [0,1]:
// bad7: #Config & {
// 	decide: compile_threshold: 1.5
// }
