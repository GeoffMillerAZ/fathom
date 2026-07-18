# Changelog

All notable changes to sextant. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/); dates are ISO-8601.

## [Unreleased]

### Removed — sextant back-compat surface (owner decision 2026-07-17: no back-compat wanted)
- **Pre-rename back-compat is gone, not deprecated.** The rename shipped
  2026-07-07 (ADR-015) with several shims so a mid-migration fleet wouldn't
  break; the owner overrode that "kept by design" decision and asked for all
  of it out. Removed:
  - `cmd/fathom/serviceident.go`'s `withNameAliases` — the `sextant`↔`fathom`
    service-name aliasing that `session-hook`, `edit-hook`, and `stop-hook`
    used to resolve either brand era against the daemon's index. Each hook
    now looks up only the candidate's own (lower-cased) name; the double
    loop in all three hooks collapses to a single lookup.
  - `internal/connector.LegacyIgnoreFileName` (`.sextantignore`) and its
    fallback read in `loadIgnoreFile` — only `.fathomignore` is honoured
    now. `TestLocalConnector_Discover_LegacyIgnoreFallback` flips to
    `TestLocalConnector_Discover_NoLegacyIgnoreFallback`, asserting a
    `.sextantignore`-only repo now gets zero excludes from it (the file is
    just an ordinary discovered file to `Discover`).
  - Migration prose pointing users from `sextant session-hook` to `fathom
    session-hook` (`docs/use-cases/getting-started.md`).
  - `docs/rename-migration.md` is marked historical/superseded at the top
    (its back-compat bullet list is no longer accurate) rather than
    rewritten — it's a record of what shipped 2026-07-07, not living
    guidance.
  **Behavioral consequence:** stores ingested under the service name
  `"sextant"` and repos relying on `.sextantignore` are no longer
  recognized — re-ingest under the real service name, or rename the ignore
  file to `.fathomignore`. The fleet's own store already uses `"fathom"`
  post `reingest-fleet.sh`, so the dogfood machine is unaffected.
  `scripts/fathom-adoption.sh`'s `mcp__sextant__`-prefix counting is
  intentionally untouched — it is retrospective measurement over pre-rename
  session transcripts that still exist on disk under the old directory
  name, not a runtime back-compat code path.

### Added — layer catalog: prompt-as-data, pack shape, adr-rules + spec-bindings, `fathom layers` (docs/plans/layer-catalog.md)
- **Prompts are data, not Go (LC-1).** Every `#Layer` instance now carries
  `prompt_file`; `promptForLayer` is generic over `LayerDef` instead of a
  Go switch statement per layer name — the silent obligations fallback
  for an unwired-but-manifest-valid layer name (a correctness bug) is
  gone. Zero identity rotation: `obligations` stays `obligations/c02b1aa1`,
  pinned by test.
- **`serving.drift_tracked` replaces the `kind=="finding"` hardcode
  (LC-2)** at all three remediation-drift sites (compile-side scope-fp
  writing, and both serve-side `code_drifted` sites) — a schema field, not
  a Go type check, so a future layer opts into drift tracking by setting
  one CUE field. `findings` sets it `true`; `obligations` stays at the
  schema default (`false`).
- **Built-ins migrate to the vision's pack shape (LC-4):**
  `schema/semantic/layers/<name>/{layer.cue,prompt.md,eval/,KILL.md}`,
  mirrored (`layer.cue`+`prompt.md` only) into `internal/semantic/layers/`
  and `go:embed`'d — `schema.cue` now holds only the `#Layer` definition.
- **The pack loader unions embedded packs with a project's own
  `.fathom/layers/` (LC-3)** — presence = enabled (ADR-018 reconciler
  doctrine, no separate enabled/disabled state anywhere), a project pack
  overriding an embedded one of the same name. `pkg/client.CompileLayer`
  resolves through this union rooted at the service's own ingested path,
  so a project-local pack compiles and serves exactly like a built-in one
  (`TestClient_CompileLayer_ProjectLocalPack`).
- **`fathom layers list|show|install`** (LC-7): store-direct like `fathom
  claim`'s v0 convention. `install` validates a candidate pack against
  `#Layer` (missing `prompt.md`/empty `eval/` warn, don't fail) and copies
  it into `.fathom/layers/<name>/` — users never hand-author CUE.
- **Two new embedded packs, both cleared the precision/recall shipping
  gate LIVE against `FATHOM_LLM_PROVIDER=claude-cli`** (docs/design/
  semantic-layers.md §4.6; no API key needed): `adr-rules` (the layer-2
  proof — architectural constraints from `ADR.md`/`docs/design/*.md`,
  precision 1.00 / recall 1.00) and `spec-bindings` (the SDD pack —
  gated-plan task bindings from `docs/plans/**` excluding `README.md`,
  scoped to each task's `package:` annotation, precision 1.00 / recall
  1.00 on the first attempt). Results in each pack's `eval/RESULTS.md`.
- **Third-layer end-to-end proof (LC-8):** a fixture pack nobody wrote any
  Go for compiles and serves — including remediation-drift tracking for a
  PROJECT-local pack, which surfaced and fixed a real gap: drift-tracking
  resolution now consults the same embedded ∪ project union a compile saw
  (via `ServiceMeta.Path`), not an embedded-only lookup.
- **Fixed:** `#Layer.corpus.exclude` was decoded from the manifest but
  never wired into any `SelectCorpus` call — `spec-bindings`' corpus is
  the first to need it. `SelectCorpus` now takes an independent exclude
  matcher.
- Docs: `docs/layers.md` is the new catalog reference (per-pack when-to-use,
  corpus, cost, gate metrics); `docs/mcp-tools.md`'s `compile_layer` row and
  `docs/use-cases/coding-agents.md`/`change-review.md` updated for the new
  verb and packs.
- **Pre-merge hardening (critic review of the slice):**
  - A broken project pack no longer takes the whole union down: a
    half-authored `.fathom/layers/<name>/` is skipped (embedded packs —
    including one it would have overridden — keep serving), warned once on
    stderr, and surfaced by `fathom layers list` via `LayerInfo.LoadError`;
    only requesting it by name errors. Drift tracking for embedded
    `findings` provably survives an invalid sibling pack
    (`TestDriftedFindings_SurvivesBrokenProjectPack`).
  - Honest uninstall semantics: pack presence governs COMPILATION; serving
    derives from the STORE. `layers list --store --service` gains an
    orphaned-layers section (claims served with no defining pack); a real
    `layers uninstall` that retires claims is deferred as LC-13.
  - `fathom compile`'s flag-parse-time layer check is advisory-only now
    (warn + pass through): a hard CWD-rooted rejection false-negatived
    cross-repo project layers (`--service B --layer <B's pack>` from repo
    A) — the backend's service-rooted resolution is the only rejection
    (`TestCompile_AcceptsCrossRepoProjectLayer`).
  - Install hygiene: `layers install` stages into a hidden temp sibling and
    atomically renames (a mid-copy failure leaves no half-written pack);
    reinstalling refuses without `--force`; `--force` replaces the whole
    directory, never merges.
  - Eval evidence hardening: fixture sets are explicit
    (`labels.json`.`docs`, never a glob — `RESULTS.md` can't leak into the
    corpus); both new packs' fixtures expanded to the obligations
    precedent (adr-rules 7 rules + 2 traps, spec-bindings 6 rules + 2
    traps) and RE-GATED at 1.00/1.00 twice each under the PINNED production
    compile model (`claude-haiku-4-5-20251001`, claude-cli lane), recorded
    with model+lane+date+command in each `eval/RESULTS.md`. The obligations
    layer re-run under that same pinned model recorded precision 1.00 but
    recall straddling its bar (0.71/0.86) — a pre-OP-6 signal, recorded in
    `schema/semantic/layers/obligations/eval/RESULTS.md` and escalated.
  - `layerIsDriftTracked` memoized per `ObligationsFor` call (one union
    load per distinct layer, not per served rule, on the edit-hook path).

### Added — configuration experience: profiles, ~/.fathom/config.toml, health, smoke (docs/plans/config-experience.md)
- **`internal/config`**: TOML-authored, CUE-validated (`schema/config/schema.cue`,
  embedded byte-identical with a drift test — the `internal/semantic/manifest.go`
  precedent) runtime configuration. `Load` merges a user layer
  (`~/.fathom/config.toml`) and a project layer (`.fathom/config.toml`)
  field-by-field — a partial project-layer override never blanks out fields
  it didn't mention — and tracks per-field `Provenance` (which layer/env-var
  supplied every value). `ResolveProfile` selects a named profile
  (`--profile`, else `default_profile`) and layers the **env-overlay**
  (`FATHOM_LLM_PROVIDER`, `FATHOM_COMPILE_MODEL`, …) on top, so fathom's
  pre-profile env-only behavior is reproduced byte-for-byte when no config
  file exists. **Raw secrets are unrepresentable**: `#Profile` is a closed
  CUE struct with no `api_key` field — `api_key_env` (read at LLM-client
  construction time, never at load) or `api_key_cmd` (argv-exec, no shell,
  10s bound, trimmed stdout, output never logged or embedded in errors) are
  the only credential paths.
- **`compile --profile`**: a new `cmd/fathom/llmprofile.go` construction
  path (`buildLLMClientForProfile` + six `build*ClientProfile` functions)
  sits ALONGSIDE `compile.go`'s original env-only `buildLLMClient` and its
  six `build*Client` functions, which are byte-for-byte untouched — all 15
  pre-existing `compile_test.go` provider tests pass unmodified, source and
  expectations both. `compileModelForProfile` adds the profile as a
  precedence tier between `FATHOM_COMPILE_MODEL` (still wins) and the
  cheap-tier default; `compileModel()` itself is unchanged.
- **`fathom config [--explain] [--json] [--profile X]`**: prints the
  effective configuration. `--explain` is the flagship feature — every key
  annotated with its source (`flag`/`env:VAR`/`project:path`/`user:path`/
  `default`).
- **`fathom providers [--profile X] [--json]`**: auth-health probe —
  `map[string]llmclient.Pinger` built from every configured profile (or just
  one), one `llmclient.ProbeAll` call (zero/near-zero tokens). A profile
  whose adapter doesn't implement `Pinger` (a type-assertion, not a
  provider-name allowlist) is reported `not probeable`, never silently
  dropped. Non-zero exit on any probed or construction failure.
- **`fathom smoke --profile X|--all [--json]`**: one tiny real completion
  per selected profile — model, wall-clock latency, tokens, and an estimated
  cost (the same flat per-KTok rates every `compile` run's `cost_usd`
  already uses). Spends real tokens, so `--profile`/`--all` is required
  explicitly; no implicit "smoke everything" default. No live calls in
  tests — fake shims / `httptest`, mirroring `compile_test.go`'s patterns.
- **`serve-http`** loads the same config at startup: `[daemon]`
  (`store`/`addr`/`call_log`/`auto_refresh`) feeds flag *defaults* (a flag
  on the invocation still wins), and `[daemon].default_profile` (falling
  back to the top-level `default_profile`) selects `compile_layer`'s
  provider — closing the gap where the launchd plist carries no env at all
  and `compile_layer` was silently disabled on the always-on daemon.
  Read-once: a config change reaches the daemon only after a restart, like
  every other daemon knob.
- **`fathom init`** gains a write-if-absent step scaffolding
  `~/.fathom/config.toml` with every profile shape (subscription, API-key,
  AWS GovCloud, Azure Government) commented out and the secrets doctrine
  stated inline; never overwrites an existing file.
- **AWS GovCloud / Azure Government** are profiles, not modes: `region =
  "us-gov-west-1"` or an explicit FIPS `endpoint` for Bedrock; `endpoint =
  "https://<resource>.openai.azure.us"` for Azure OpenAI. Hermetic
  construct-level proofs (no network) that a profile's region/endpoint
  actually reaches the kit adapter's `Config`, not just that fathom's own
  pre-checks pass.
- Docs: `docs/configuration.md` is the new canonical guide (precedence,
  profiles, GovCloud/Government recipes, `config`/`providers`/`smoke`,
  secrets doctrine, daemon restart-to-apply); pointers from `README.md` and
  `docs/use-cases/coding-agents.md` §5 (the provider-matrix table stays the
  single source, linked rather than duplicated).
- Repurposed the pre-existing (unused, zero-importer) `internal/config`
  package rather than colliding names elsewhere — its old JSON
  query-profile API (`coding`/`investigation`/`ci`/`platform`) predated this
  feature and had no callers anywhere in the tree.
- Review hardening (coordinator critique of the first landing): the two
  pre-existing serve-http tests that reach the new config load now isolate
  HOME/cwd (they would have started binding real `[daemon].addr` ports the
  day an operator adopts `[daemon]`); a modelless bedrock **profile**
  fails fast naming the requirement instead of silently constructing
  around the Anthropic-API default model id (which passes `providers`'
  Ping but breaks the first compile opaquely) — plus a table-driven
  equivalence test (`llmprofile_equivalence_test.go`) that drives all six
  lanes through BOTH the frozen env-only builders and the profile builders
  from the same environment and fails on any construct-vs-error or routing
  divergence; `compile --profile` without `--store` is now an error naming
  the constraint (HTTP mode compiles with the daemon's own configured
  profile) instead of silently ignoring the flag; TOML parse errors carry
  line/column; CUE validation errors print each offending key once
  (grouped per path); the daemon's fail-to-start on a malformed config is
  now tested and documented (pre-flight with `fathom config`); the
  `providers` "not probeable" branch is tested end-to-end through a
  builder seam (`providerClientBuilder`).

### Added — provider matrix: openai, azure-openai, bedrock, codex-cli (docs/plans/provider-matrix.md)
- **Four more `FATHOM_LLM_PROVIDER` lanes**, rounding `buildLLMClient`
  (`cmd/fathom/compile.go`) out to six: `openai` (`buildOpenAIClient` —
  vanilla OpenAI Chat Completions via `agentic-kit/foundations/llmclient/openai`,
  `OPENAI_API_KEY` optional for local/compatible hosts, `FATHOM_OPENAI_BASE_URL`
  override for Ollama/vLLM), `azure-openai` (`buildAzureOpenAIClient` — the
  same adapter's Azure mode; `AZURE_OPENAI_ENDPOINT` and
  `AZURE_OPENAI_DEPLOYMENT` are checked by fathom itself, before calling the
  adapter, so a missing one names the env var an operator actually sets
  rather than the adapter's Go `Config` field), `bedrock`
  (`buildBedrockClient` — AWS Bedrock via `agentic-kit/foundations/llmclient/bedrock`'s
  full credential-chain resolver: explicit → env → shared files including
  `aws sso login` sessions and `credential_process` → IMDSv2, expiry-aware
  refresh, no static keys required from fathom; `FATHOM_COMPILE_MODEL` is
  fathom's own requirement — Bedrock has no meaningful default model, so a
  missing value fails construction naming the variable), and `codex-cli`
  (`buildCodexCLIClient` — mirrors `claude-cli` via
  `agentic-kit/foundations/llmclient/codexcli`, `FATHOM_CODEX_BIN` override;
  **also Pings the constructed client** before returning it, since
  codexcli's `Request.System` mapping and ambient-context suppression ride
  undocumented `codex-cli` config keys a future CLI upgrade could silently
  drop — a failed Ping is a construction error naming the silent-degradation
  risk, so fathom never hands back a client that could silently lose its
  system prompt and revert to the CLI's ~13k-token built-in persona).
  The unknown-provider error now lists all six lanes.
  Follow-up in the same landing: CLI `--store` mode no longer hardcodes the
  Anthropic model string into `WithLLM` — both construction sites share
  `compileModel()` (`FATHOM_COMPILE_MODEL`, cheap-tier default), because
  `Request.Model` wins over any adapter's `DefaultModel` when non-empty and
  the hardcoded string would have silently overridden every non-Anthropic
  lane in store-direct compiles.
  Docs: `docs/use-cases/coding-agents.md` §5 gets a provider matrix table
  (lane, auth mode, env vars, marginal-cost story) including the honest
  `codex-cli` cost line — roughly 10k input tokens of ambient overhead per
  call (~300k for a 31-section compile) against ChatGPT plan limits, ~60%
  cache-discounted steady-state, versus `claude-cli`'s near-zero ambient
  overhead under `--safe-mode`.
  go.mod's `agentic-kit` require bumped to the pseudo-version for the
  commit landing all three kit slices (aws-credchain, azure-openai,
  codexcli-adapter). Tests: `cmd/fathom/compile_test.go` — construct-level
  coverage for `openai`/`azure-openai`/`bedrock` (env fixtures via
  `t.Setenv`, including a base-URL/Azure-deployment routing round trip
  against a local `httptest` server), and hermetic fake-shim coverage for
  `codex-cli` covering both the healthy and degraded Ping outcomes. No real
  network/API/CLI-login calls in any of it.

### Added — subscription-backed compile provider (claudecli)
- **`FATHOM_LLM_PROVIDER=claude-cli`.** `cmd/fathom/compile.go`'s single
  construction site (`buildAnthropicClient`, called by both `compile
  --dry-run=false` and `serve-http`'s `compile_layer`) is now
  `buildLLMClient`, which switches on `FATHOM_LLM_PROVIDER`: unset or
  `"anthropic"` keeps today's behavior (needs `ANTHROPIC_API_KEY`);
  `"claude-cli"` shells out to a locally logged-in Claude Code CLI instead
  (via `github.com/geoffmilleraz/agentic-kit/foundations/llmclient/claudecli`,
  new in agentic-kit), so obligations/findings compiles ride a Claude Pro/Max
  subscription's OAuth login instead of a metered key.
  `FATHOM_CLAUDE_BIN` optionally overrides the resolved `claude` binary path
  (e.g. for the `serve-http` launchd daemon, whose PATH may not include it).
  The no-provider-available error now names both options. Reported
  `cost_usd` is the same API-equivalent accounting under either provider;
  actual marginal spend under `claude-cli` is $0 (subscription plan limits,
  not a metered key) — documented in `docs/use-cases/coding-agents.md` §5.
  go.mod's `agentic-kit` require bumped to the pseudo-version pinning the
  commit that added `claudecli` (kit `go.work`+`replace` still couples the
  local sibling checkout for day-to-day dev; the pin matters for a cold
  clone). Tests: `cmd/fathom/compile_test.go` covers `buildLLMClient`'s
  switch (default/explicit anthropic without a key, claude-cli construct +
  one real `Complete` through a hermetic fake-shim, unresolvable
  `FATHOM_CLAUDE_BIN`, an unknown provider value) — the anthropic path's
  (nonexistent) tests are untouched.

### Added — findings-as-claims tracer bullet (docs/plans/findings-tracer.md D3, FT-1..FT-6)
- **Kind:"finding" unlocked end-to-end.** A `findings` `#Layer` instance
  (`schema/semantic/schema.cue`: kind `finding`, corpus `findings/**`, tier
  `sme-stated`) joins `obligations` in the manifest. `internal/semantic.Compile`
  is now layer-aware — it resolves name/kind/tier/tier_profile/prompt from
  `Compiler.Layer` (falling back to the pre-M1 obligations identity when no
  layer is loaded, so every existing direct-Compiler test and the
  expert-module non-git-corpus test are unaffected) — instead of hardcoding
  the obligations identity everywhere. A new findings system prompt
  (`internal/semantic/prompts.go`) shares the obligations claim envelope and
  JSON schema; only the compile instructions differ (FT-1).
- **`pkg/client.CompileLayer(ctx, service, layer, dryRun)`** generalizes
  `CompileObligations` (now a thin wrapper). Fixes finding F7: `cmd/fathom
  compile --layer` and the `compile_layer` MCP tool validated the requested
  layer name via `LoadLayer` and then silently discarded it, always compiling
  "obligations" regardless of what was asked for — both now route the
  validated name all the way through.
- **Kind-aware serving.** `semantic.ObligationsFor` now reads claims across
  every compiled layer (not just `obligations`) and the served `Obligation`
  gains a `kind` field (`json:"kind,omitempty"`, omitted for the obligations
  layer — byte-compatible with every existing consumer) so a finding claim
  scoped to a file is labeled, not silently indistinguishable from an
  obligation (FT-2).
- **`fathom claim refute|corroborate --store DB --service S --claim ID --note
  "..."`** wires the previously-dead `claim_events` machinery
  (`AppendClaimEvent` had zero non-test callers before this): `refute` appends
  a `refuted` event and flips claim status to `ClaimRefuted`; `corroborate`
  appends a `corroborated` event and never changes status or tier —
  corroboration is provenance, not evidence-laundering (FT-3).
- **Refutation durability.** `PutClaims` (both `DuckStore` and `MemoryStore`)
  now retains `ClaimRefuted` status across an upsert of the same `claim_id` —
  a recompile that re-emits an unchanged rule can no longer silently
  resurrect a suppressed finding to the compiler's default fresh status
  (FT-4).
- **`fathom report --store DB --service S --analyzer security_rollup --out
  findings/`** is the deterministic bridge: writes ONE markdown finding
  document (per-category sections — hardcoded secrets, unchecked shell-outs,
  unsafe HTTP client usage — each with `path:line` lists), mirroring
  `internal/mcp/extra_tools.go`'s `security_rollup` shape, so deterministic
  analyzer output flows through the normal ingest→compile→claims pipeline
  instead of a parallel persistence path. Output is a pure function of the
  corpus working tree at run time (`Sources()` re-reads disk, not the index;
  no clock/random ordering in the body), verified by a double-run
  byte-compare — including with `--out` INSIDE the corpus, the intended
  deployment: source files under the resolved `--out` directory are excluded
  from the scan, closing the self-scan feedback loop where run N+1 would
  find shell-out/http snippets embedded in run N's own doc and mutate it
  every run. Secret-category findings are rendered redacted (`path:line` +
  rule only — the scanner messages embed the matched source line, and a
  committable document must never exfiltrate the credential it reports)
  (FT-5).
- **Remediation drift (FT-6).** The compiler now persists, for finding-kind
  claims only, a `ClaimLink{Rel:"scope-fp"}` per scope-matched file with its
  compile-time store fingerprint. `semantic.DriftedFindings` is a new, pure
  read that re-compares those fingerprints against the CURRENT store state;
  `ObligationsFor` calls it and sets `code_drifted:true` on the served
  payload when the scoped code changed since compile. Chosen design: a live
  re-read at serve time rather than a persisted flag — keeps `Sweep` and
  `ObligationsFor` both pure functions of current store state, with no third
  claim-status axis, and matches the vision's explicit rule that code drift
  is a serving annotation, never a status flip (a finding whose code changed
  might be FIXED, not wrong). Anchor persistence is loud, never best-effort:
  a fingerprint read failure (or a fingerprint-less indexed file) at compile
  time aborts the run (`errDriftAnchor`) instead of silently omitting the
  link — the digest gate would never recompile the unchanged section, so a
  missing anchor would permanently blind drift for that file. This upholds
  the invariant DriftedFindings relies on (every compiled finding claim
  carries ≥1 scope-fp link), and serving is bounded: a per-call fingerprint
  cache queries each distinct anchored file at most once per
  `ObligationsFor`, regardless of how many claims share it (the edit-hook
  hot path). Post-merge follow-up closed FT-2's last hop: the edit-hook's
  rendered bullet now labels non-obligation kinds after the tier
  (`[…, sme-stated, finding]`) and appends `code drifted since compile`
  when `code_drifted` is set — obligation bullets are byte-identical to
  before.
- **Findings prompt: no bare `**` scopes.** The findings compile prompt
  instructs the narrowest affected-path globs the document justifies; a
  genuinely repo-wide finding scopes to its affected file class (e.g.
  `**/*.go`), never a bare `**` — aligned with the O8 precision work's
  compile guard. The prompt identity is content-addressed, so this rotates
  only `findings/<hash8>`; no compiled priors existed.

### Added — O8 precision: serve-time trust (docs/plans/o8-precision-and-speclines.md OP-1..OP-5)
- **Prompt v3** (`internal/semantic/prompts.go`): the obligations compile system
  prompt now instructs file-class scoping — a rule binds only the file class its
  governing document addresses (code rules → source globs like `**/*.go`, docs
  rules → markdown globs like `**/*.md`) and a bare `"**"`/repo-wide catch-all
  scope is never allowed. All v2 instructions (binding-rules-only, narrowest
  scope, 28-word rule cap, the `{"claims":[...]}` envelope) are kept. The
  content-addressed `PromptVersion` (TextHash-derived) rotated to
  `obligations/c02b1aa1`, forcing every section to recompile under the new
  prompt rather than silently keeping v2 claims (the v1→v2 hand-bumped-const
  bug class this mechanism exists to prevent).
- **Over-broad-scope guard** (`internal/semantic/compile.go` `verifyDraft`): a
  claim whose scope glob matches more than half of `IndexedFiles` is rejected
  and counted in `ClaimsRejected`, exactly like a failed citation — the
  compile-time backstop for prompt v3's "never `**`" instruction. This makes
  `"**"` always over-broad in a non-empty corpus, so every prior test scope of
  `["**"]` was rewritten to a real narrow glob (`internal/mcp/**` against the
  shared 3-file test fixture). **Exemption (review fix):** single-file-class
  extension globs — a glob whose final segment is `*.<ext>` with one literal
  extension (`**/*.go`, `docs/**/*.md`, `*.pb.go`) — are exempt from the
  breadth check (never from zero-match rejection): they are class-scoped by
  construction, exactly the shape prompt v3 mandates for legitimately
  fleet-wide rules, and in a go-majority corpus `**/*.go` necessarily exceeds
  50% — without the exemption the guard would reject every prompt-compliant
  repo-wide code rule at the OP-6 recompile. Bare `**`, directory globs
  (`internal/**`), and multi-extension patterns matching >50% stay rejected.
- **Edit-hook root refusal** (`cmd/fathom/edithook.go` `cmdEditHook`): a file
  that does not relativize under ANY resolved service-candidate root now skips
  that candidate outright instead of falling back to querying the daemon with
  the raw out-of-repo path — the O8 addendum (obligations fired for a file
  outside every indexed repo) goes silent. **Review fixes:** the check is not
  purely lexical — `relativeToRoot` retries with both sides
  `filepath.EvalSymlinks`-resolved (falling back to the lexical path on error),
  so symlinked layouts (`~/code` → `/Volumes/…/code`, macOS `/var` →
  `/private/var`) where `gitToplevel` returns the physical root while the
  session addresses files through the symlink don't false-refuse every in-repo
  file; the `..`-escape test is `rel == ".." || strings.HasPrefix(rel, "../")`
  (a root-level `..foo` entry is not an escape); and a relative `file_path` is
  absolutized before the candidate loop so it cannot bypass the refusal.
- **Spec-line injection** (`cmd/fathom/edithook.go`): when the edited file cites
  bracketed spec IDs (`internal/sdd.ExtractRefsFromText`, the same extraction
  `spec_refs` uses), the edit-hook looks up each ID (capped at 3) via the
  `spec_refs` MCP tool and appends `"▸ implements [ID] — defined at
  <file:line>"` for the location OUTSIDE the edited file — literally tying the
  cited spec to where it's defined, in the channel that already works
  (docs/vision/04 §4 item 3). **Review fix:** spec lines fire independently of
  obligations — a file citing a spec ID with zero matching rules (the common
  case post-recompile, with prompt v3's narrower scopes) gets a
  spec-lines-only payload under its own `⚖️ fathom spec refs` header;
  zero-output-on-clean is preserved (no rules AND no spec IDs → silent).
  Shares the obligations payload's ≤150-token budget; spec lines are dropped
  first (never truncating obligations), and when obligations alone already
  spend the budget every spec_refs RPC is skipped up front. Silent on any
  error (unreadable file, daemon down, etc.) per hook doctrine.
- **`plan` project skill** (`.claude/skills/plan/SKILL.md`): scaffolds a
  gate-ready grammar-v2 plan (stable `id`s, `{symbol, package}` contracts
  grounded via `pack_context`/`test_impact`, typed `disposition`s for skips,
  `skip_test` always paired with `skip:`, spec-ID brackets, the `fathom verify`
  command to run before checking any box) so new plans are born gate-ready
  instead of needing an honesty rewrite after the fact.

### Added — coding-agents canonical doc + staleness sweep (IA-4/IA-5)
- docs/use-cases/coding-agents.md is THE adoption guide (the loop, hooks,
  gates, CLAUDE.md/AGENTS.md doctrine, agent-surface registration, the honest
  cost story); usage.md set_profile guidance retired; the June
  claude-code-integration.md banner-superseded (assessment content kept);
  getting-started/coding-context scoped and pointed at the canonical doc.

### Added — fathom init: one-command coding-agent wiring (docs/plans/init-and-adoption.md IA-1/IA-2/IA-3)
- `fathom init` wires a repo end-to-end, idempotently and additive-only: creates
  `.fathomignore` if absent, calls the daemon's `ingest` MCP tool and confirms
  the service is indexed (fail-soft exit 2 with startup instructions when the
  daemon is unreachable or the service still has no symbols), JSON-merges
  project-level `.claude/settings.json` hooks (SessionStart `session-hook`,
  PostToolUse `edit-hook`, Stop `stop-hook`), writes the canonical fathom
  operating-contract stanza into both CLAUDE.md and AGENTS.md between
  `<!-- fathom:begin/end -->` markers, and prints the manual steps it cannot
  safely automate (MCP registration one-liner, lefthook pre-push verify
  snippet). Every mutation is create-if-absent or marker-replace, so a second
  run changes zero bytes and reports "already present" throughout.
- `mergeHookSettings` preserves every unrelated top-level key, hook event, and
  sibling hook entry in an existing `settings.json`; detection is by command
  substring (`"fathom session-hook"` etc.), so re-running never duplicates an
  entry regardless of the exact binary path baked in.

### Changed — MCP surfaces week-1: execute-and-flag, agent surface, loopback default (docs/plans/surfaces-week1.md, amends ADR-014)
- Gate semantics change from hard-gate to **execute-and-flag**: an off-surface
  `tools/call` or `batch` sub-call now executes instead of erroring, logged
  with `off_surface:true` in the JSONL call log — an error taught abandonment,
  which looked identical to no-demand in 8 days of dogfood evidence. The
  pre-amendment hard gate stays available behind `Server.strictSurfaces` for
  any future multi-tenant shape.
- The `companion` allowlist becomes the 15-tool **agent** surface (minus
  `batch`/`brief`, plus `verify_plan`/`capabilities` — the top non-hook tool
  and the discovery escape hatch), served at the new primary path
  `POST /rpc/agent` with `POST /rpc/companion` kept as a compatible alias.
- `set_profile` is removed entirely (amends ADR-010/014's stdio clause): no
  production caller reaches it any more, so stdio now always serves the full
  registry.
- Four zero-call alias tool registrations deleted: `platform_callers`,
  `test_debt`, `api_routes`, `deps` — use the canonical `cross_repo_callers`,
  `markers`, `routes`, `dependencies`.
- `serve-http`'s default listen address changes from `:7575` (all interfaces)
  to `127.0.0.1:7575` (loopback) — surfaces were never a security boundary,
  so the default must not listen on all interfaces regardless.

### Added — Gap-1 channel instrumentation: denominator, liveness, origin (docs/plans/gap1-instrumentation.md)
- Hooks beat a central log (FATHOM_HOOKS_LOG, default ~/.fathom/hooks.jsonl)
  via agentic-kit v0.7.0 telemetry/heartbeat: edit-hook seen/answered/error,
  session-hook fired — injection coverage finally has a denominator, and beat
  failures are swallowed (hooks never break their silent contract).
- `fathom instrument`: coverage (answered/seen vs the ≥90%% gate), per-day
  rollup, and liveness alarms from the standing expectations (sessions
  without edits in 7d; edits unanswered in 1d — the four-dark-days failure
  made visible). Advisory: exit 0 always.
- X-Fathom-Origin: every hook/CLI HTTP call declares its channel; the daemon
  call log records `origin` so demand numbers and kill clocks separate
  plumbing from agent choice.
- Grammar hardening (GI-7): the plan-annotation parser takes the TRAILING
  brace block — prose braces and code literals in titles no longer hijack
  contracts (caught live by the first non-vacuous gate run over the fleet).

### Added — M1: the manifest goes live (semantic layers, ADR-016 amendment)
- **`schema/semantic/schema.cue` is now a runtime artifact, not documentation.**
  `internal/semantic.LoadLayer` embeds a package-local, byte-identity-tested
  copy of the schema (`internal/semantic/manifest_schema.cue`,
  `TestManifestSchemaMatchesSource`) and decodes `#Layer` instances (corpus
  globs, tier, budgets, serving caps) via `cuelang.org/go`. The obligations
  layer's corpus include list and per-claim rule-length budget now come from
  the manifest instead of the hand-synced `ObligationsInclude` Go var and a
  bare `maxRuleWords` const — both retired (grep-clean under
  `internal/semantic`). `cmd/fathom compile` and the `compile_layer` MCP tool
  accept any manifest-listed layer (today still exactly `obligations`) and
  reject unknown names with the manifest's live layer list instead of a
  hardcoded string check.
- **Content-addressed prompt identity** (agentic-kit v0.7.0
  `prompttemplate`). The obligations system prompt moved into a
  `prompttemplate.Template` (`internal/semantic/prompts.go`); `PromptVersion`
  is no longer a hand-bumped const — `obligationsPromptVersion()` derives it
  as `obligations/<hash8>` from `Template.TextHash()`. The digest gate
  (`allDigestsMatch`) keys on this derived identity, so any edit to the
  prompt text is now automatically a new compile identity — the exact bug
  class that silently left 27/32 sections on stale v1 claims after the
  v1→v2 bump cannot recur.
  **This is a flag-day recompile.** The prompt text did not change in this
  slice, but its identity's FORM did (hash-derived vs. the retired
  `obligations/v2` const), so the very next real compile will see every
  existing claim's `PromptVersion` mismatch and recompile the whole corpus
  once. No compile was run as part of this change (no API keys, no LLM
  calls — mocks only); sequence the actual fleet recompile deliberately —
  ideally folded with the O8 serve-time precision prompt revision so the
  fleet pays one recompile, not two — and never mid-measurement-window.
- **Typed compile failures** (agentic-kit `llmclient/structured`). Section
  completion now goes through `structured.Complete`, which sets
  `Request.OutputSchema` to the claim-draft JSON Schema and classifies
  truncation/malformed model output as `structured.ErrTruncated` /
  `structured.ErrMalformed` — the hand-rolled `errBadResponse` classifier and
  its fenced-code-block-stripping retire. Skip-section policy
  (`sections_failed++`) is unchanged; transport errors still abort the run.
- **Run-level budget enforcement** (`enforceBudgets`, ADR-016 §4.1). A real
  compile now refuses to start when its pre-flight, digest-gate-aware cost
  estimate exceeds the layer's `max_usd_per_run` (typed refusal, still
  ledgered), and stops cleanly mid-corpus when the running token total
  crosses `max_tokens_per_run` — sections not yet reached are left untouched
  and resume via the ordinary digest gate on the next invocation. Both cases
  are recorded on the existing `compile_runs` ledger, additively
  (`core.CompileRun.Refused` / `RefusalReason` / `BudgetStopped`; new
  columns on the DuckDB backend, migrated idempotently).
- **`#Layer.tier` widens to admit `verified`** for deterministic,
  Model-empty compile paths (ADR-018 oracle classes: `index`/`cmd`/`lexical`)
  — amendment recorded under ADR-016 in `ADR.md`. No layer emits it yet;
  `ai-derived` stays unadmitted, still awaiting its own revival evidence.
- Spec: `docs/plans/m1-manifest.md` (M1-1..M1-8, all landed).
- **Adversarial-review fixes (same slice, pre-recompile):**
  (F1) the structured-outputs schema is now OBJECT-rooted
  (`{"claims":[...]}`) with `additionalProperties:false` on every object —
  Anthropic structured outputs reject array roots / open objects with a 400,
  which is transport-class and would have aborted every real compile at
  section 1; the system prompt and decode target moved to the envelope shape
  (`TestClaimDraftSchema_AnthropicStructuredOutputCompatible` pins both wire
  constraints). This prompt/schema change MUST land before the flag-day
  recompile — the TextHash identity covers the prompt text, not the schema,
  so sequencing is the protection.
  (F2) superseded/refuted claims no longer feed the digest gate: one
  superseded row (old digest or old PromptVersion) previously forced its
  subject to re-LLM on every future run, forever
  (`TestCompile_SupersededClaimsDoNotDefeatDigestGate`).
  (F5) a budget-stopped run is no longer indistinguishable from success at
  the operator surfaces: `compile_layer` returns
  `budget_stopped`/`refusal_reason` (and `refused` symmetrically) and the
  CLI printers render them.

### Fixed — extractor-epoch fingerprint salt: extractor upgrades now reach unchanged files (O9)
- The pass-3 hash-gate keyed fingerprints on file content only, so a binary
  with improved extraction (e.g. H1's aliased-selector canonicalization and
  method-edge forms) silently kept every unchanged file's OLD observations —
  the fleet store served pre-H1 `Calls` forms and `test_reaches` failed on
  genuinely-tested symbols (dogfood O9, caught by the first HTTP-mode run of
  the re-armed agentic-kit gate). Fingerprints are now salted with
  `fusion.ExtractorEpoch` (bump on any extraction-semantics change → full
  re-extract on next ingest); `ObligationsIndexStale` adopts the shared
  `fusion.FingerprintHash` so the formulas cannot diverge. Regression:
  `TestIngest_ExtractorEpochInvalidatesOldFingerprints`.

### Added — SDD reconciler v0: plan grammar v2, wave gate, CUE shadow, Stop hook (ADR-018)
- **Plan grammar v2** (`internal/verify`, additive — v1 plans parse unchanged):
  `[~]` in-progress checkbox; author-owned stable `id:` keys (positional
  `md-NNN` renumbering was fatal for cross-edit diffs); typed dispositions on
  `[-]` (`disposition: refused|deferred` + `reason:` + recorded-only
  `trigger:`); `skip:` justification for `skip_test`; double-quoted annotation
  values may contain commas. `verify.LintPlan` reports convention violations
  as warnings (never exit-code changes in v0).
- **Wave gate**: `fathom verify --require-all` (`verify.GateRequireAll`) —
  fails on open/in-progress tasks so a wave advances only when every task is
  claimed-and-verified or explicitly skipped. The recorded exit-0 run is the
  transition record; there is no transition engine (ADR-018: state is
  derived, never driven).
- **CUE shadow**: `fathom gen-plan-cue` emits a generated-only `#Plan`
  document (schema/plan gains state/disposition/reason/trigger +
  expect.package/skip_reason) and proves **lossless round-trip** against the
  Markdown parse. CUE is output, never an authoring surface (the Dagger
  lesson); hand-authored plan CUE is deprecated.
- **Stop hook**: `fathom stop-hook` — done-means-proven at session
  granularity: tasks newly claimed `[x]` this session (working tree vs HEAD,
  matched by stable ID) that FAIL verification surface at session end.
  Advisory by default; `--block` refuses session end with the failing
  citation. Silent exit 0 on every operational failure (edit-hook doctrine);
  Unknown never fires (fail-open honesty).
- Specs: `docs/design/sdd-reconciler.md` (normative), ADR-018, gated plan
  `docs/plans/sdd-reconciler-v0.md`; grammar reference updated in
  `docs/plans/README.md`. Vision record: `docs/vision/09-deterministic-sdd.md`.

### Changed — obligations v2: first-fleet-compile hardening
- **Prompt v1 → v2 + corpus narrowed to binding docs** (`CLAUDE.md`, `ADR.md`,
  `docs/plans/README.md`). The first fleet compile turned `docs/design/**` essays into
  noisy `**`-scoped aspirational rules ("achieve precision ≥0.90") injected on every
  edit — the trust-fatal class ADR-016 §4.6 warns about. v2 compiles binding rules only
  and prefers the narrowest scope the source supports.
- **Digest gate now includes `PromptVersion`.** A prompt bump previously did not
  recompile unchanged sections — the v1→v2 recompile silently kept v1 claims on 27/32
  sections. Covered by `TestCompile_PromptBumpDefeatsDigestGate`.
- **Per-section failure resilience.** A malformed or truncated model response
  (strict-JSON parse failure, `StopMaxTokens`) now fails only its own section —
  counted in the new `compile_runs.sections_failed` — instead of aborting an
  88-section fleet corpus; transport/auth errors still abort. `MaxTokens` 1200→2400
  (rule-dense sections were truncating mid-JSON).
- **Corpus-departure retirement.** Claims whose subject left the corpus selector are
  superseded on the next compile, not served forever.
- First full fleet compile under v2: 31/32 sections, 72 claims (9 rejected by
  compile-time verification), $0.047.

### Added — semantic layers v1: the obligations tracer bullet (ADR-016)
- **Governed-claims substrate.** Four additive tables beside observations —
  `semantic_claims` (content-addressed `claim_id`; tier + `tier_profile`; status
  fresh/stale/contradicted/superseded/refuted; flattened provenance: model, prompt
  version, repro key, input digest, compile watermark), `claim_links` (`cites` with
  line-range hash, `scopes` globs), `compile_runs` (real token/USD ledger), and
  append-only `claim_events` (the refute channel). Superseded claims are status-flipped,
  never deleted. Both store implementations + contract tests.
- **Obligations compiler** (`internal/semantic`): governing docs → markdown sections
  (fence-aware, stable subjects) → digest gate (unchanged sections cost $0) → DLP
  redaction → injected LLM (strict JSON; mock/replay in tests) → **compile-time anchor
  and scope verification** (a claim whose cited lines don't resolve or whose scope globs
  match no indexed file is rejected, counted per run) → transactional store + supersede.
  Rules are word-budgeted at compile (≤28 words) so a 6-rule payload stays ≤150 tokens.
  Dry-run estimation is the default spending gate. Layer definition schema:
  `schema/semantic/schema.cue` (+ Go-asserted negatives).
- **Citation sweep** (post-ingest, token-free): re-hashes every served claim's cited
  lines; drift flips status to `stale`, unresolvable citations to `contradicted`,
  restored docs heal back to `fresh`. Wired into `IngestService` so the daemon's
  auto-refresh keeps claims honest.
- **Decision-moment serving.** `obligations` MCP tool (on the companion surface, now 15
  tools): rules governing a file, ordered by scope specificity, capped at 6, each with
  `src` citation + tier, drifted rules served *flagged*; **zero-output-on-clean**.
  `compile_layer` tool (full surface only; `dry_run` defaults true) + `fathom compile`
  CLI. `staleness_report` gains a per-layer claims summary.
- **PostToolUse injection** (`fathom edit-hook`): after every Edit/Write, Claude Code
  gets the edited file's obligations injected (≤150 tokens) plus an index-behind-your-
  working-tree warning; silent on clean, silent on any error (hooks never block —
  the coding-agent-excellence §5.3 failure policy). This is the configuration-forced
  delivery channel the 0–7% pulled-tool share demanded.
- **Precision eval harness** (`internal/semantic/eval_test.go` + labeled fixture corpus
  with deliberate scope traps): gates the layer at precision ≥0.90 / recall ≥0.75
  against a real model; opt-in (`FATHOM_EVAL_LLM=1`), deterministic suite stays free.

### Added — fleet onboarding ergonomics
- **`ingest` tool onboards new services**: optional `path` (absolute corpus root)
  ingests a repo the index has never seen — through the live daemon, no
  single-writer lock dance. The session-hook's "add this repo" guidance now names
  the exact call. Capability forwarded through the auto-refresh wrapper (the O7
  lesson) with a through-the-wrapper test.
- **Subdirectory-rooted projects resolve their service**: both hooks now try the
  project dir's basename, then the enclosing git repo's basename (a Claude
  project rooted at `<repo>/deploy` no longer resolves to service "deploy");
  the edit-hook resolves file paths against whichever root matched.
  `reingest-fleet.sh` gains nested-path support + three fleet repos
  (growing-gardens-bi, car-assistant, crossplane-platform); the adoption
  script's default names follow.

### Added — done-means-proven gate (H1.2)
- **Checkbox semantics**: `[x]` in a plan is a *claim*; `verify.Gate` fails any claimed
  task whose contract fails (`--strict` also gates Unknown). `[ ]` open and `[-]`
  skipped never gate.
- **`fathom verify` CLI**: HTTP-first against the daemon, `--store` for CI; the gate is
  always computed locally so a stale daemon can't launder a claim. Exit 0/1/2 with
  fail-open pre-push (exit 2 warns and allows) and fail-closed strict CI. Wired into
  lefthook pre-push + `scripts/ci.sh` here and in agentic-kit (`docs/plans/` in both).
  First gated run caught 3 over-claims by the plan's own author (demo:
  `docs/plans/README.md`; recall gap filed as dogfood O6).
- **Gate honesty: an empty index is "cannot verify", never "all claims false".**
  Caught live against the pre-migration daemon: verifying a service with zero
  indexed symbols made every claim read `symbol not found` — evidence-shaped
  output for an operational problem, which would have blocked every push (the
  §5.3 trust-fatal mode). `fathom verify` now probes the index first (HTTP:
  `list_services`; store: symbol count) and exits 2 (fail-open) with a
  fix-it message when the service isn't indexed.

### Added — born-portable stores (ADR-017)
- **Repo-relative anchors** from every connector (LocalConnector included — absolute
  refs from legacy stores still read); **`fathom remap`** repoints a service root in one
  metadata update; **Watermark SPI** (git HEAD or content-addressed `manifest:` hash)
  makes freshness domain-neutral — non-git corpora (docs folders, findings exports)
  auto-refresh and sweep honestly; **ingest self-heal** purges pre-ADR-017
  absolute-anchored content wholesale so no symbol duplicates across anchor styles.
  Contract: `TestStorePortability_RoundTrip` (move corpus + remap → citations, queries,
  freshness, and hash-gating all survive).

### Fixed
- `fathom-adoption.sh` measured nothing for this repo after the rename (transcript dirs
  are path-keyed): name-era aliasing, per-ISO-week rollup, and a daemon call-log summary
  section. Measured baseline: **0% fathom share (0 vs 193 grep calls)** — the number the
  forced channels exist to move.


### Renamed
- **sextant → fathom** (ADR-015). Module path `github.com/geoffmilleraz/fathom`,
  binary `cmd/fathom` → `bin/fathom`, MCP serverInfo/resource URIs/metrics prefixes,
  docs and scripts. Back-compat kept where the old name is data in the field:
  `.sextantignore` still honoured (legacy fallback behind `.fathomignore`),
  adoption metrics count both `mcp__sextant__`/`mcp__fathom__` prefixes,
  `session-hook` treats the two service/dir names as one identity, and
  `reingest-fleet.sh` finds the repo under either directory name. Historical
  records (this changelog below, USAGE_JOURNAL.md, dogfood-observations.md)
  intentionally keep "sextant". Operator steps: `docs/rename-migration.md`.

Uplifts from mining local Claude Code usage across the fleet. The finding: sextant works
when called, but is dramatically under-used — agents reach for shell grep 10×+ more than
sextant even where the index exists, because "index first" lived only in passive docs.

### Added
- **Path-scoped tool surfaces on the shared HTTP daemon.** `POST /rpc/{surface}` serves an
  immutable per-request view of the registry: `companion` is a curated 14-tool coding-agent
  hot loop (`find_symbol`, `query_symbols`, `references`, `outline`, `repo_map`,
  `find_callers`, `symbol_context`, `pack_context`, `brief`, `entrypoints`, `batch`,
  `ingest`, `staleness_report`, `list_services`); `coding`/`investigation`/`shared`/
  `platform` mirror the group profiles. Bare `/rpc` remains the full surface (the right
  registration for schema-deferring clients like Claude Code); scoped endpoints cut the
  tools/list schema payload ~80% for clients that load every schema up front. One daemon,
  one `mcp.Server`, one store — a surface is context carried per request
  (`mcp.WithRequestScope`), not a second server. Motivated by the tool-surface strategy
  review (86 tools ≈ 8.6k schema tokens for non-deferring clients).
- **Surfaces hard-gate `tools/call`, closing the profile-bypass hole.** Previously a tool
  hidden from `tools/list` by the active profile was still callable (`findTool` scanned the
  full registry) — and `batch` dispatched sub-call handlers directly, bypassing any filter.
  On a named HTTP surface, off-surface calls (including `batch` sub-calls) now return an
  error naming the surface and pointing at `POST /rpc`. Canned prompts are scoped the same
  way: a surface hides prompts whose workflow references tools it gates (resources stay
  available everywhere — data, not capability). The stdio profile stays advisory (a single
  client owns that process), pinned by regression tests on the non-gating side.
- **`set_profile` is rejected over HTTP.** The profile is process-global on the shared
  daemon: any client flipping it changed `tools/list` for *every* session, and
  `listChanged:false` meant nobody re-fetched. The error points at path-scoped surfaces;
  stdio keeps `set_profile` and gains `companion` as a valid value. Since every HTTP call
  to it is dead, `set_profile` is also dropped from every HTTP `tools/list` view — scoped
  endpoints advertise only what they serve. `capabilities` and the manifest resource now
  report `active_surface` alongside `active_profile`.
- **Tool-call telemetry wired into the daemon.** `serve-http --call-log` (default
  `workspace/logs/mcp-calls.jsonl`, empty disables) appends the existing JSONL CallLogger
  records — now with the request `surface`, with `batch` sub-calls logged individually
  (`via:"batch"`) so bundled hot-loop lookups are counted per tool, and with each record
  written as a single atomic `O_APPEND` write so a daemon kill can't merge lines — making
  which-tools-does-the-hot-loop-actually-use measurable instead of argued (feeds the
  sextant-adoption metric and any future surface curation). Decision record: ADR-014.
- **`session-hook` — a Claude Code SessionStart adoption hook.** `sextant session-hook`
  pings the shared serve-http daemon, maps the project dir to a service, and injects an
  active reminder ("this repo is service X (N symbols); resolve symbols/callers/structure
  via sextant BEFORE Grep/Read") — or an honest signal when the daemon is down or the repo
  isn't indexed. Short HTTP timeout, always exits 0, hides malformed services from the list.

### Changed
- **Query-side staleness is constant-size now** (USAGE_JOURNAL.md ask #4: an 8-node
  `query_symbols` answer arrived buried under ~117 lines of per-source staleness
  metadata). The `staleness` block on query responses now carries at most 3 example rows
  (the oldest stale sources) plus `stale_sources`, `total_sources`, `fresh_sources`, and
  a new `oldest_ingested_at` low-water mark; `reingest_recommended` unchanged. The full
  source list and commit drift stay available via the `staleness_report` tool.
  Regression: `TestQuery_StalenessReport_CompactWhenManyStale`.
- **`find_symbol` no longer punishes not knowing the exact identifier**
  (USAGE_JOURNAL.md ask #3: `find_symbol("serveHTTP")` → `[]` because that wasn't the
  exact spelling). Matching now degrades deterministically — exact/key-suffix →
  case-insensitive (same equality rules, case-folded) → substring (capped at 20,
  tightest names first, explicit `truncated`) — and a full miss returns up to 5
  `did_you_mean` suggestions when any exist: edit-distance typo candidates from the
  symbol list first ("authentiction" → `Authentication`), else concern-ranked. The
  response reports `matched_via` (`exact`/`case_insensitive`/`substring`/`none`);
  results are existence proof only for the first two tiers. Fallback tiers run only on
  an exact miss, in the MCP layer, so no backend interface changed. Regressions:
  `internal/mcp/findsymbol_test.go`.
- **`verify_plan`'s tool schema slimmed ~27% (~150 tokens; description nearly halved)**
  (it was the heaviest schema on the surface at ~554 tokens, ~6% of the whole registry
  for schema-loading clients). `min_tier`'s valid values moved from prose into a proper
  JSON-schema `enum`. Behavior is unchanged; the full verdict reference lives in
  `docs/mcp-tools.md`.

### Fixed
- **Ingest no longer aborts on a symlink-to-directory (Nix/devbox trees).** `WalkDir`
  does not follow symlinks, so a link like `.devbox/nix/profile/default-9-link`
  surfaced as a non-dir entry; `Fetch`'s `os.ReadFile` then failed with "is a
  directory", and because the fusion fetch pass treats the first fetch error as
  fatal, **one stray link aborted the entire ingest**. `LocalConnector.Discover` now
  emits only regular files (symlinks/devices/sockets/pipes skipped). Found
  dogfooding `resume_as_code`, which went from a hard abort to 1,160 clean symbols
  (the Nix store behind the link is correctly excluded, not indexed as noise).
  Regression: `TestLocalConnector_Discover_SkipsSymlinkToDir`.
- **`IngestService` re-ingests from the recorded path, not the service ID.** It used
  `ServiceRef{ID: service, Path: service}` — re-walking the bare ID as a directory, which
  ingested nothing and **clobbered `ServiceMeta.path` to empty**. Since auto-refresh,
  `staleness_report`, and the `ingest` MCP tool all key off that path, every refresh
  silently froze the shared daemon's index. Now it uses the stored `meta.Path` (mirroring
  `Sources`), so refreshes preserve the path and actually pick up new commits. Regression:
  `TestIngestService_UsesAndPreservesStoredPath`. Also serves `serve-http --auto-refresh`
  (added so the always-on daemon self-refreshes) and `scripts/reingest-fleet.sh` (rebuild
  the fleet index cleanly with path metadata + no malformed/bloated services).
- **Reject path-like service IDs (fleet hygiene).** `ingest --service ../app-kit` had
  polluted the fleet with duplicate, confusingly-named services. `core.ValidServiceID`
  rejects path separators / `..` / whitespace at the `client.Ingest` chokepoint (CLI + the
  ingest MCP tool), suggesting a clean name. Prevents recurrence.
- **Bound list-tool output to a char budget.** A real `query_symbols` result hit ~57KB and
  blew the caller's token budget. `query_symbols` and `references` now keep the longest
  prefix of records that fits ~16KB and flag `truncated`, so one result can't blow context.

## [0.2.0] — 2026-06-16

Enhancements driven by mining throughline's real Claude Code sessions (841 sextant calls,
1666 shell greps, a heavy manual freshness loop).

### Added
- **`references` MCP tool** — evidence-tiered, fleet-wide occurrence search (substring /
  whole-word / regex). Returns text matches at the `pre-scan` tier plus symbol definitions
  cross-referenced from the index at their real tier. Turns the ~658 repo-by-repo greps a
  session does into one structured call.
- **`verify_plan` accepts a Markdown checklist** (`plan_markdown`), and `gen-status-cue`
  accepts a `.md` plan file: each `- [ ]/[x] title [SPEC-ID] {symbol: Foo}` line becomes a
  verifiable task. A `/verify` slash command runs the loop — surfacing `[x]`-but-not-verified
  items before "done" is trusted. (throughline's sessions were hand-rolling this.)

### Changed
- **`serve-mcp --auto-refresh` (default on)** — a query for a service whose git HEAD has
  drifted from the indexed commit transparently re-ingests (hash-gated, debounced per
  commit) before answering. Removes the manual staleness→ingest loop (49 ingests / 19
  staleness checks per 3 sessions). A second `serve-mcp` on the same store now refuses with
  actionable guidance instead of DuckDB's raw single-writer lock error.

## [0.1.0] — 2026-06-15

First tagged milestone: a multi-language code-intelligence + evidence engine with an MCP
server, a language-agnostic cross-repo graph, and an evidence-backed verification layer.

### Added
- **Multi-language extraction (~25 languages/tools).** Tree-sitter via CGO (ADR-013) for
  general languages — Python, JS/TS, Java, Ruby, Rust, C#, C, C++, Scala, PHP, Lua — and
  pure-Go parsers for config/IaC/tooling: Terraform/HCL, Kubernetes, Helm, Dockerfile,
  docker-compose, GitHub Actions, Protocol Buffers, GraphQL, CUE, Markdown, SQL, TOML,
  JSON. 93 `SourceKind`s. Interim intra-file call edges for the tree-sitter languages.
  Calibration corpus + harness.
- **Language-agnostic cross-repo blast radius.** `internal/platformlink` builds the
  cross-repo graph from stored multi-language observations instead of a Go-only filesystem
  walk; `pkg/platform.BuildFromStore`; the `platform-callers` CLI defaults to the store
  path (`--repos` is a labelled legacy Go-only mode). Cross-repo edges now resolve for
  every language whose extractor populates call edges.
- **Evidence-backed definition-of-done.** `internal/verify` engine + the `verify_plan` MCP
  tool: a per-task verdict computed from code evidence (the claimed symbol exists at a
  sufficient tier ∧ a test statically reaches it ∧ a required call-edge is wired ∧ no
  contradiction ∧ declared specs are covered), each with cited evidence. The
  `gen-status-cue` CLI renders a deterministic, one-directional `STATUS.cue` from a
  human-owned plan file (`schema/plan` CUE schema). **Static by construction**:
  `runtime_pass` is always `unknown` — a static check never claims a test passes.
- **Use-case adoption docs** under `docs/use-cases/` — a getting-started guide plus six
  how-tos (coding-context, cross-repo blast radius, spec verification, incident
  investigation, change review, fleet health) for wiring sextant into Claude Code, with a
  one-doc-per-use-case maintenance convention.
- **Build-identity / staleness signal.** `sextant version` reports the embedded VCS
  revision, commit time, and dirty flag (Go `-buildvcs`, no ldflags); `serve-mcp` logs it
  at startup, so a long-running server that has outlived commits is self-evident — compare
  against `git rev-parse HEAD`. (O3.)
- Earlier foundation in this release: typed edge-graph `traverse` + commit `history`
  ingestion; `spec_refs` and `brief` tools; git-HEAD auto-staleness; line numbers carried
  through to MCP results; `.sextantignore` repo-specific excludes; the Claude Code
  integration playbook and assessment.

### Fixed
- **`SymbolNode.Package` persists through the store.** It was silently dropped in the
  `SymbolNode → Observation` mapping, degrading `SymbolKey` reconstruction to `"svc::.Name"`
  and breaking precise cross-repo / call-graph matching off stored evidence. Added a
  `package` column with an idempotent migration.
- **The call graph resolves cross-package selector calls.** A callee recorded as
  `"pkg.Name"` (e.g. an external `*_test` package calling the symbol under test) never
  matched the node key `"svc::pkg.Name"`, so `test_impact` / `find_callers` /
  `rename_impact` under-counted in the common Go external-test-package pattern. Fixed with
  a precise `package.Name` index (a receiver-method call is never mistaken for a package
  selector).
- **`outline` honors its repo-relative contract.** `symbolnav.Outline` matched the stored
  absolute `FilePath` against the `file` arg by exact string, so a repo-relative path —
  the documented contract — returned `{"symbols":[]}`. Now resolves a relative query by
  path-segment suffix (absolute still exact). Found via fleet dogfooding (throughline).
  (O1.)

### Changed
- **The `post-commit` hook rebuilds `bin/sextant` on Go changes** before re-indexing, so
  the on-disk binary tracks HEAD. A running `serve-mcp` still needs a restart to load a new
  binary. (O3.)
