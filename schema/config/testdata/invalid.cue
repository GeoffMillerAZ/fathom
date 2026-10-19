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
// 6. An unknown [decide] provider value (ADR-042: "none", "llm",
//    "typesafe" or "openai" only):
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
//    refuses api_key with a message naming api_key_file — the key must
//    come from api_key_file, api_key_env or api_key_cmd, never a literal:
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
//     the six sites that exist today ("route", "compile_prefilter",
//     "obligations_rerank", "certify_suggest", "assess", "preflight") — an entry outside that set
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
//
// 16. A [tenancy] refuse mark below the shed mark (the ladder's rungs are
//     ordered: shed ≤ refuse):
// bad16: #Config & {
// 	tenancy: {
// 		shed_at:   1.0
// 		refuse_at: 0.5
// 	}
// }
//
// 16b. A [tenancy.host] allowance outside 0..1 (it is × budget; 0 is the
//      rule before 2026-10-19, unset the 0.6 × shed_at default):
// bad42: #Config & {
// 	tenancy: host: allowance: 1.5
// }
//
// 17. A [console] conserve_max_files below 1:
// bad17: #Config & {
// 	console: conserve_max_files: 0
// }

// [hooks] test_run is a bool (TR-9); a string is refused.
// bad18: #Config & {
// 	hooks: test_run: "yes"
// }
//
// [hooks] claim_check and gate_preflight are bools (RO-38, RO-40):
// badClaimCheck: #Config & {
// 	hooks: claim_check: "on"
// }
// badGatePreflight: #Config & {
// 	hooks: gate_preflight: 1
// }
//
// [hooks] read_outline_hint is a bool (H8):
// badReadOutlineHint: #Config & {
// 	hooks: read_outline_hint: "yes"
// }

// [[work.ci_summaries]] needs a path, and a commit is a hex object name
// (WI-2w):
// bad19: #Config & {
// 	work: ci_summaries: [{commit: "0123abc"}]
// }
// bad20: #Config & {
// 	work: ci_summaries: [{path: "ci/junit.xml", commit: "main"}]
// }

// [hooks] pin_check is off, record or feedback (PR-6); any other value is
// refused:
// bad21: #Config & {
// 	hooks: pin_check: "warn"
// }

// A [decide] sites entry naming the judge_* family itself, or a judge site
// with no name: one "judge_<name>" per site, never a wildcard (JF-4):
// bad22: #Config & {
// 	decide: sites: ["judge_*"]
// }
// bad23: #Config & {
// 	decide: sites: ["judge_"]
// }

// A claude-cli profile timeout must be a Go duration ("20m"), never prose
// or a bare number (EW-34):
// bad24: #Config & {
// 	profiles: slow: {
// 		provider: "claude-cli"
// 		timeout:  "soon"
// 	}
// }

// A [decide] transport that does not fit the provider: openai speaks only
// to its own API, typesafe to its own API or OpenRouter's (2026-10-09):
// bad25: #Config & {
// 	decide: {
// 		provider:  "openai"
// 		transport: "openrouter"
// 	}
// }
// bad26: #Config & {
// 	decide: {
// 		provider:  "typesafe"
// 		transport: "openai"
// 	}
// }

// A [decide] api_key_file that is not an absolute or ~/ path (a key pasted
// into the wrong field is refused without repeating it):
// bad27: #Config & {
// 	decide: api_key_file: "secrets/token"
// }

// A [decide] api_key_env that is not a variable name:
// bad28: #Config & {
// 	decide: api_key_env: "not-a-variable-name"
// }

// A [receipts] repository_uri that is not an absolute URI (no scheme), or
// not a string:
// bad29: #Config & {
// 	receipts: repository_uri: "example.test/org/repo"
// }
// bad30: #Config & {
// 	receipts: repository_uri: 42
// }

// A [typed] skip_modules glob that is empty, or a list given as one string
// (EW-68):
// bad31: #Config & {
// 	typed: skip_modules: [""]
// }
// bad32: #Config & {
// 	typed: skip_modules: "calibration/**"
// }

// The [decide] consent and kill fields (KS-3): enabled and
// allow_repo_provider are bools, session_sites names only the session
// sites, and deny_sites only known sites:
// bad33: #Config & {
// 	decide: enabled: "no"
// }
// bad34: #Config & {
// 	decide: allow_repo_provider: 1
// }
// bad35: #Config & {
// 	decide: session_sites: ["route"]
// }
// bad36: #Config & {
// 	decide: deny_sites: ["made_up_site"]
// }

// The [readouts] keys (RO-15): the mode is one of off, observe, lean, rich;
// roots are non-empty strings; a rule's table is closed:
// bad37: #Config & {
// 	readouts: mode: "loud"
// }
// bad38: #Config & {
// 	readouts: roots: [""]
// }
// bad40: #Config & {
// 	readouts: rules: "ls-long": mode: "verbose"
// }
// bad41: #Config & {
// 	readouts: rules: "ls-long": trust: true
// }
