# Configuration experience — profiles, ~/.fathom/config.toml, health, smoke

Owner order 2026-07-17 (ambition 11): exact-model pinning in settings,
AWS GovCloud + Azure Government compatibility, a `~/.fathom/`-style config
home, and first-class visibility into both auth health and whether models
actually respond. Grounding (wf_22fe6355): fathom has ~14 env vars and ~15
overlapping flags, no config file, no config reader precedent — and the
always-on daemon's `compile_layer` is SILENTLY DISABLED today because the
launchd plist carries no env at all. The config file is the fix for a live
gap, not a convenience.

Design (normative):
- **File format TOML; validation CUE.** `schema/config/schema.cue`
  (#Config/#Profile) + testdata/valid.cue + commented invalid.cue per repo
  convention; byte-copy embedded into a new `internal/config` package with
  a drift test (the manifest.go precedent). The TOML is decoded, converted,
  and unified against the embedded schema at load — errors name file, key,
  and expected shape. Humans author TOML, never CUE (ADR-018 Dagger lesson).
- **Precedence: flags > env > project `.fathom/config.toml` > user
  `~/.fathom/config.toml` > built-in defaults.** Env vars keep exactly
  their current meaning (nothing shipped this week breaks); the file layers
  under them. Wiring pattern: a `Load()` result feeds flag DEFAULTS before
  fs.Parse (the defaultFathomURL precedent generalized), so per-subcommand
  diffs stay small.
- **Profiles are the unit of provider config.** `default_profile = "name"`
  plus `[profiles.<name>]` with: `provider` (the six lanes), `model` (exact
  pin — always wins over adapter defaults), `endpoint`, `region`,
  `deployment`, `api_version`, `bin`, `api_key_env`, `api_key_cmd`.
  **Raw secrets are unrepresentable**: the schema has no api_key field;
  `api_key_env` names an env var, `api_key_cmd` an argv-exec'd command
  (`op read …`, `security find-generic-password …`) whose stdout is the
  key — never logged, never echoed in errors. `--profile` flag on compile
  (and anything constructing an LLM); env FATHOM_LLM_PROVIDER et al. act as
  an implicit overlay profile at the env precedence tier.
- **GovCloud is a profile, not a mode.** Bedrock GovCloud = `region =
  "us-gov-west-1"` (standard host derives correctly) or explicit `endpoint`
  for FIPS (`bedrock-runtime-fips.us-gov-west-1.amazonaws.com`); Azure
  Government = `endpoint = "https://<resource>.openai.azure.us"`. Kit
  adapters already accept both (verified 2026-07-17); fathom's job is
  carrying them from profile to adapter config plus hermetic tests proving
  the construction paths and a documented example of each.
- **Health vs smoke are different verbs.** `fathom providers` = auth
  health: build `map[string]llmclient.Pinger` from configured profiles,
  one `llmclient.ProbeAll` call, zero/near-zero tokens; profiles whose
  adapter lacks Ping are reported "not probeable", never silently omitted.
  `fathom smoke [--profile X|--all]` = one tiny real completion per
  selected profile, reporting model, latency, tokens, and cost — it spends,
  and says so. Both: human table + `--json`. Both exit non-zero when any
  probed lane fails.
- **`fathom config` verb**: prints effective config; `--explain` annotates
  every key with provenance (flag/env/project/user/default) and the file
  path that supplied it. This is the flagship UX feature.
- **Daemon**: serve-http loads the same config at startup ([daemon] table:
  store, addr, call_log, auto_refresh; compile profile = default_profile) —
  restart-to-apply, stated in the file's own scaffold comment. This removes
  the need to ever edit plist env for provider selection.
- **`fathom init`** gains a write-if-absent step scaffolding
  `~/.fathom/config.toml` with commented examples (subscription, API-key,
  GovCloud AWS, Azure Government) and the secrets doctrine stated inline.

## Tasks

- [x] internal/config: TOML load + CUE-validated merge across user/project layers, typed errors naming file+key; embedded schema byte-copy with drift test {id: CF-1, symbol: Load, package: internal/config}
- [x] Profile resolution: named profiles, default_profile, env-overlay profile, api_key_env/api_key_cmd indirection (cmd exec'd argv-style, output never logged) {id: CF-2, symbol: ResolveProfile, package: internal/config}
- [x] buildLLMClientForProfile consumes a resolved Profile (env lane preserved byte-for-byte — the original env-only buildLLMClient and its six build*Client functions are untouched, so all 15 pre-existing compile_test.go tests pass unmodified; the profile-aware path is additive in cmd/fathom/llmprofile.go); --profile flag on compile {id: CF-3, symbol: buildLLMClientForProfile, package: cmd/fathom}
- [x] fathom config verb with --explain provenance per key {id: CF-4, symbol: cmdConfig, package: cmd/fathom}
- [x] fathom providers: ProbeAll-based auth health across configured profiles; not-probeable reported honestly; table + --json; non-zero exit on failure {id: CF-5, symbol: cmdProviders, package: cmd/fathom}
- [x] fathom smoke: opt-in real completion per profile with latency/tokens/cost; table + --json {id: CF-6, symbol: cmdSmoke, package: cmd/fathom}
- [x] serve-http loads [daemon] config + default_profile at startup — compile_layer works on the daemon with zero plist env {id: CF-7, symbol: runServeHTTP, package: cmd/fathom, skip_test: true, skip: "startup wiring exercised via the config-load unit tests + existing serve tests; runServeHTTP's own reach is the documented residue class for long-lived server mains"}
- [x] init scaffolds ~/.fathom/config.toml write-if-absent with commented profile examples incl. both GovClouds {id: CF-8, symbol: writeStarterConfig, package: cmd/fathom}
- [x] GovCloud construction proofs: bedrock gov region + FIPS endpoint profile → adapter config; azure .us endpoint profile → adapter config; hermetic tests, no network {id: CF-9, symbol: buildBedrockClientProfile, package: cmd/fathom}
- [x] docs/configuration.md: canonical guide (precedence, profiles, GovCloud recipes, providers/smoke, secrets doctrine, daemon restart-to-apply); README + coding-agents doc pointers {id: CF-10, symbol: cmdConfig, package: cmd/fathom, skip_test: true, skip: "docs deliverable anchored to the verb it documents; prose is not index-provable"}

## Deferred

- [-] Hot-reload / SIGHUP config re-read {id: CF-11, disposition: deferred, reason: "read-once matches every existing daemon knob; restart ritual is documented", trigger: "operator pain from restart-to-apply recorded in dogfood observations"}
- [-] Native keychain integrations {id: CF-12, disposition: deferred, reason: "api_key_cmd covers any vault/keychain via its own CLI", trigger: "a customer environment where command indirection is unavailable"}
- [-] Per-purpose profile binding (compile vs enrich vs eval) {id: CF-13, disposition: deferred, reason: "enrichment has zero production callers; eval lanes are kit-side", trigger: "enrichment revival or fathom-side eval workloads"}

## SEC-2 — repository config chooses behaviour only (2026-10-09)

Owner rule (SEC2 RULE, 2026-10-09; ADR-042 amendment SEC-2): a repository's
`.fathom/config.toml` may choose behaviour (provider, model, thresholds,
include lists, repo-relative paths); it may never name an executable fathom
runs, name a credential, name a path outside the repository, or switch on
automatic execution. Builds on SEC-1 (DP-40, `dropRepoKeySources`).

- [ ] Every field the loader accepts has a scope class (behaviour, repo-path or user-only) in one registry; a test walks the embedded `#Config` schema against it and fails on an unclassified field and on a classified field the schema dropped {id: CF-14, symbol: ScopeOf, package: internal/config, reachable_from: TestEveryConfigFieldHasAScopeClass}
- [ ] `config.Load` drops every user-only field from a repository's layer (`profiles.*.bin`, `profiles.*.endpoint`, the daemon's store, addr, call_log, auto_refresh and memory_limit, `[tenancy]`, SEC-1's key sources) and names file, field and reason, never the value, in `fathom doctor`'s `key_source_scope` row, `fathom config` (text and `--json`) and the daemon's startup log {id: CF-15, symbol: checkKeyScope, package: cmd/fathom, reachable_from: TestDoctorKeyScope_UserOnlyFieldsAndEscapingPathsNamed}
- [ ] No automatic path runs an executable a repository names or writes a path it names outside itself: a fixture `./evil.sh` never runs through serve-http's and serve-mcp's profile lanes or doctor (`TestSEC2_RepoBinNeverRunsOnAnyProfilePath`), the session hook's outbox sync never imports into a repository-chosen store (`TestSEC2_FindingsOutboxNeverWritesOutsideTheRepo`), and the test-run hook runs nothing it matches (`TestSEC2_TestRunHookRunsNothingTheRepositoryNames`) {id: CF-16, symbol: syncFindingsOutbox, package: cmd/fathom, reachable_from: TestSEC2_FindingsOutboxNeverWritesOutsideTheRepo}
- [ ] Repo-path fields stay inside the repository: one helper, `config.RepoPath`, refuses an absolute value, a `..` escape and a symlink escape by name, and every use site calls it (`tests.results_dir` at ingest and conserve, `findings.outbox`, `connector.include`, `work.ci_summaries[].path`) {id: CF-17, symbol: RepoPath, package: internal/config, reachable_from: TestRepoPath_RefusesEscapesByName}
- [ ] The records: ADR-042's SEC-2 amendment (the rule, the three classes, which verbs may run a repository's commands), CHANGELOG Breaking lines, the Scope table in `docs/configuration.md` pinned to the registry (`TestConfigurationDocScopeColumn`), O652..O654 {id: CF-18, oracle: lexical, path: ADR.md, pattern: "^\*\*Amended 2026-10-09 \(SEC-2, the second security amendment after SEC-1\): a repository's config chooses$"}
