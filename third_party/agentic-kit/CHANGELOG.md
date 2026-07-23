# Changelog

Notable changes to agentic-kit. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/); versions are git tags.
Every release ships with the full gate green: `task validate` =
`fmt-check` + `vet` + `cue-vet` + `go test -race ./...`.

## [Unreleased]

### Added

- **`foundations/telemetry/heartbeat` gains `CheckExistence`/`ExistenceExpectation`/`ExistenceAlarm` (2026-07-22, Phase K K3-1, promoted from fathom doctrine law 7 — `docs/vision/11-dial-to-eleven.md` in the sibling repo, "Instruments get existence checks. An adoption metric whose traffic source can silently vanish (or never start) is theater."):** `CheckLiveness` already answers "did effect channel B fire alongside activity channel A"; it cannot answer "did channel X ever fire at all since a recorded start time" — a channel that never starts and a channel that stops are indistinguishable to it, and fathom's own B2 zero-call investigation needed exactly that distinction (recorded t0 + connected-client existence check). `CheckExistence(beats, []ExistenceExpectation{{Channel, Since}}, ref)` alarms once per expectation with zero beats for `Channel` in `[Since, ref]`; malformed expectations (empty channel, zero `Since`, `Since` after `ref`) are skipped rather than alarmed, mirroring `CheckLiveness`'s posture. Purely additive — `Expectation`/`Alarm`/`CheckLiveness` untouched; `schema.cue` gains `#ExistenceExpectation` with a positive fixture and two documented-negative fixtures (empty channel, missing `since`); package doc extended to state which of the two checks a caller wants. `go test -race`/`cue vet` green. Named consumer: fathom's `scripts/check-surface-traffic.sh` (companion change in the sibling repo) against the MCP `server_stats` tool's `surface_traffic` block.

- **`foundations/llmclient/claudecli` gains `Ping`/`llmclient.Pinger`, and `foundations/llmclient/bedrock` gains a Region/BaseURL mismatch guard (2026-07-17, docs/plans/claudecli-ping.md KP-1/KP-2):** a fathom `providers`-style health surface built on `llmclient.ProbeAll` audited every adapter and found claudecli was the one implementation NOT satisfying `Pinger` — a silent coverage gap for the most-used subscription lane. `Ping` follows the `codexcli` precedent exactly: a LOCAL, pre-API, zero-token probe. It runs the exact pinned production argv `Complete` would build for a zero-valued `Request` (no `--model`/`--system-prompt`/`--effort`/`--json-schema`), but with EMPTY stdin and — per this adapter's invocation shape — never a positional prompt argument either; the installed CLI (2.1.212) rejects that combination locally, before resolving a model or touching the network, with a stable stderr signature (`"Input must be provided either through stdin or as a prompt argument..."`, exit 1, empty stdout). Verified live across six probes recorded in the new `foundations/llmclient/claudecli/testdata/PROBES.md`: the signature is stable across every flag combination tried, survives a deliberately bogus `--model` value (proving the input-presence check runs BEFORE model resolution), and is contrasted against one real "reply pong" completion (5.6s wall time, a full JSON result envelope, real `total_cost_usd`) to establish what the healthy zero-token path must never produce. A merely-blank (whitespace-only) stdin does NOT trigger the same rejection (probe C) — the CLI treats it as a provided-but-empty prompt and proceeds differently — which is why `Ping`'s stdin is left `nil` rather than blank. Unlike `codexcli`'s `Ping` (a tripwire for an undocumented, silently-droppable config-key dependence), claudecli's `Ping` has no analogous dependence to guard — it proves the pinned argv's local input-gate is intact, not auth/model/quota. Any outcome other than the exact healthy signature — including `Ping`'s own 15s bound — returns a typed `*llmclient.InvalidRequestError`; hermetic shim fixtures cover healthy/degraded/timeout/caller-cancel shapes (4 new test funcs, `-race` clean). Second, smaller gap from the GovCloud pass: `bedrock.New` previously accepted an explicitly-set `Config.Region` and `Config.BaseURL` naming two different AWS endpoints (e.g. a GovCloud region paired with a commercial `BaseURL`, or the reverse) and would only fail later as a confusing 403 or DNS error with nothing pointing back at the `Config` that caused it. `New` now returns a typed `*bedrock.RegionBaseURLMismatchError` naming both offending values whenever BOTH fields are explicitly set and the `BaseURL`'s host does not contain the `Region` string — Region-only and BaseURL-only configs are untouched, and the check keys off the raw `Config` fields (not the resolved region), so a `Config.Region`-empty caller relying on `AWS_REGION` is never checked. No partition table or region allowlist: a plain substring test, with one generic, `net.ParseIP`-based exemption for loopback hosts so hermetic `httptest`-backed tests (including this package's own `newTestClient` helper, used by 47 existing tests) are unaffected — 6 new subtests (`TestNew_RegionBaseURLMismatch`), `-race` clean.

- **`foundations/llmclient/openai` Azure OpenAI support (2026-07-16, docs/plans/azure-openai.md AZ-1..5):** extends the existing chat-completions adapter in place rather than forking it — the wire request/response mapping, SSE streaming, and typed error mapping are 100% reused; only endpoint and auth-header construction vary. `Config.AzureDeployment` switches the client into Azure mode: chat completions (both `Complete` and `Stream`, which share `do`) route to `{BaseURL}/openai/deployments/{deployment}/chat/completions`, and `Ping` routes to `{BaseURL}/openai/models`, with a mandatory `api-version` query parameter on every one of those requests (`buildEndpoint`). `Config.AzureAPIVersion` overrides the pinned GA default `DefaultAzureAPIVersion`, whose doc comment states the pinning doctrine: the constant is expected to lag Azure's own latest api-version release by design (never a preview version), not by neglect. Auth (`setAuthHeaders`): the credential travels in the `api-key` header by default, or `Authorization: Bearer` when `Config.AzureAuthScheme` is `AzureAuthSchemeBearer` — Entra-ID token passthrough, with `Config.APIKey` then holding a caller-minted managed-identity or client-credential access token; this package mints or refreshes no tokens itself (deferred as AZ-6, a named non-goal in doc.go). `New` rejects Azure mode with an empty `BaseURL` (would otherwise silently default to the OpenAI public API and misroute) and rejects `AzureAPIVersion` set without `AzureDeployment`, both as typed `InvalidRequestError`. Azure's OpenAI-shaped error envelope verified fixture-by-fixture (400/401/429/500/503) through the Azure path rather than assumed identical to vanilla; key redaction proven to cover both the api-key and bearer header schemes (the resolved credential is scrubbed from error text regardless of which header carried it). Every pre-existing test in the package passes unmodified — the vanilla path is byte-preserved — plus a real-SSE streaming test and 20+ new Azure-specific test funcs/subtests (`azure_test.go`); `schema.cue`/`testdata/valid.cue` extended with the new optional Config fields and two Azure fixtures. Spec deviation: the plan's AZ-4 anchor names a symbol `mapAPIError` that does not exist anywhere in the kit (every adapter — anthropic, bedrock, openai — uses `mapHTTPError`, confirmed by cross-package grep); AZ-4 tests target the real, already-shared `mapHTTPError` instead of renaming a stable, multi-package-consistent name to chase a plan typo. Named consumer: fathom's compile path (`azure-openai` branch of `FATHOM_LLM_PROVIDER`).

- **`foundations/llmclient/bedrock/awscred` (2026-07-16, docs/plans/aws-credchain.md AC-1..6):** closes the SSO gap in the `bedrock` adapter, which previously resolved credentials from explicit `Config` fields or four env vars only — no SSO, no shared config/credentials files, no `credential_process`, no IMDS, no expiry-aware refresh. The new package hand-rolls the full AWS default credential chain on the standard library (no aws-sdk-go dependency, matching the parent package's SigV4/event-stream precedent): explicit Config → `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY`/`AWS_SESSION_TOKEN` → the shared config/credentials files (`AWS_PROFILE`, `AWS_CONFIG_FILE`, `AWS_SHARED_CREDENTIALS_FILE`; static keys, then `credential_process` executed via a stdlib argv-tokenized `exec.CommandContext` — no shell is invoked) → SSO (both the modern `sso_session` form and the legacy inline `sso_start_url` form; the on-disk token cache is looked up by the SHA-1 of the session name first, falling back to the SHA-1 of the start URL; an absent/expired cached token or a portal 401/403 returns a typed `*ExpiredSSOTokenError` whose message names the exact fix, `aws sso login --profile <name>`) → IMDSv2 last (token PUT + role-credentials GET, ~1s timeout, honors `AWS_EC2_METADATA_DISABLED`). Region resolves independently: `AWS_REGION` → `AWS_DEFAULT_REGION` → the shared profile's `region` key, with `Config.Region` always winning. The `bedrock` adapter now consumes this resolver end to end: `New` builds an `awscred.Config` and, for a credential that carries an `Expiry` (SSO/`credential_process`/IMDS — explicit/env/shared-static credentials remain static), `Complete`/`Stream`/`Ping` re-resolve through a new mutex-guarded `credentialCache` whenever the cached value is within a 5-minute expiry margin, so a long-lived `Client` backed by `aws sso login` or an instance role keeps working across a token refresh without reconstruction; a refresh failure surfaces as a typed, non-retryable `*llmclient.AuthError` carrying the underlying `awscred` error's actionable text. Existing explicit-Config behavior is unchanged byte-for-byte (same resolved values, same defaults, same error substrings); a new `TestMain` in `client_test.go` pins a hermetic `HOME`/AWS-env baseline for the whole package so the fuller chain can never leak a test host's real `~/.aws` into test outcomes. Hermetic tests only — temp `HOME`, fixture ini files, a fake `credential_process` script, `httptest` servers standing in for the SSO portal and IMDS, zero live AWS calls — 38 test funcs in `awscred` plus 8 new wiring/cache tests in `bedrock`, all `-race` clean. Nested under `foundations/llmclient/bedrock` rather than a shared `foundations/awscred` by design (D8): it has exactly one named consumer (`bedrock`) and graduates only when a second consumer names the need. AssumeRole/role-chaining (`role_arn` profiles) is explicitly deferred — no named consumer routes through it yet. **Hardened same-day after critique** (second commit): **explicit credentials never fall through to ambient state — a complete explicit pair short-circuits before any file/SSO/IMDS access, and a partial explicit (or env) pair hard-errors with a typed `*PartialCredentialsError`** instead of silently signing as the `~/.aws` default profile's identity (this retires the first commit's cfg-field+env-var per-field mixing); an unreadable — not merely absent — `~/.aws` therefore cannot break explicit/env-credentialed callers, while chain users who genuinely reach the file legs still hard-fail on it. `credential_process` gained a 1-minute default timeout (aws-sdk-go-v2 parity) + `cmd.WaitDelay`, a 4 KiB capture-time stderr cap (a `set -x` tool echoing its credentials can't leak them into error messages), and a typed error for an unparseable `Expiration` (RFC3339 accepted; silently-static would cache forever — the exact expiry failure mode this package exists to kill). SSO: 200-with-empty-`roleCredentials` and non-401/403 portal failures are a typed `*SSOPortalError`; the token-cache lookup doc now states first-match-wins (an expired session-name-hash hit does not fall through to the start-URL hash).

- **`foundations/llmclient/claudecli` (2026-07-16, docs/plans/claudecli-adapter.md CA-1..10):** a new `llmclient.LLMClient` implementation that shells out to the `claude` CLI (`claude -p --output-format json --safe-mode --tools ""`) instead of calling an HTTP API directly, so completions ride a Claude Pro/Max subscription's OAuth login instead of a metered `ANTHROPIC_API_KEY`. Built on `foundations/agent/bounded-exec` for subprocess timeout/output bounding and process-tree cleanup rather than a second hand-rolled `exec.CommandContext`. Verified against the installed CLI (2.1.211) rather than assumed: `--safe-mode` (not `--bare`, which would force API-key-only auth) fully suppresses CLAUDE.md/MCP ambient-context leakage; `--json-schema` genuinely enforces `Request.OutputSchema` (its internal structured-output tool call's `stop_reason:"tool_use"` is remapped to `StopEndTurn`); `--effort` honors `Request.Effort` (unlike the anthropic/bedrock adapters, which reject it — no stable API knob there); provider-side errors surface on stdout with an empty stderr, not the stderr-carried shape the original design brief assumed; the installed CLI has no `--max-turns` flag, so `--tools ""` is the mechanism guaranteeing no tool round-trip instead. Unsupported `Request` fields (`Tools`, `Thinking`, `Temperature`, `StopSequences`, `ProviderParams`, non-text `Messages` blocks) fail loud with a typed `InvalidRequestError` rather than being silently dropped; `MaxTokens` is silently ignored (no CLI flag exists). Hermetic fake-shim tests only — 21 test funcs, `-race` clean, zero network, zero real credentials touched. First subscriber: fathom's semantic-layer compile provider seam (a companion change in the sibling repo).

- **`foundations/llmclient/codexcli` (2026-07-16, docs/plans/codexcli-adapter.md CX-1..7, CX-8/CX-9 deferred):** a new `llmclient.LLMClient` implementation modeled directly on claudecli that shells out to the OpenAI `codex` CLI (`codex exec --json -C <scratch-dir> --skip-git-repo-check --ignore-user-config --ephemeral -s read-only -c model_instructions_file="<file>" -c include_environment_context=false -c include_apps_instructions=false -c include_collaboration_mode_instructions=false -c include_permissions_instructions=false`) instead of calling an HTTP API directly, so completions ride a ChatGPT Plus/Pro subscription's OAuth login instead of a metered `OPENAI_API_KEY`. Built on `foundations/agent/bounded-exec`, same composition choice as claudecli. Verified against the installed CLI (codex-cli 0.144.5) with roughly a dozen live probes (testdata/PROBES.md) rather than assumed: a bare `codex exec` call from a repo cwd burns 13,443 input tokens; this adapter's pinned combination reaches a measured floor of 9,765 input tokens (27.4% lower) via documented CLI flags plus five UNDOCUMENTED config keys (`model_instructions_file` and the four `include_*` keys — they exist only as field names inside the 0.144.5 binary, located via strings inspection; `--help` mentions none of them) that the `-c` mechanism tolerates silently when unrecognised. The dominant lever, discovered mid-probe, is `-c model_instructions_file`, which turned out to both fully REPLACE (verified via a "reply BANANA" override) the CLI's large built-in instructions_template AND be the real `Request.System` mapping, unifying what the design brief treated as two separate concerns (CX-3/CX-4 ambient-context suppression and CX-6 System mapping). Because that dependence would degrade SILENTLY on a future CLI upgrade (an unrecognised `-c` key exits 0 with no warning — verified; System would be silently lost and the ~13k-token built-in persona would silently return), the package carries a version-pinning WARNING on `Config` and implements `llmclient.Pinger` as a purpose-built tripwire (CX-10): `Ping` runs the pinned argv with a deliberately EMPTY `model_instructions_file` — a CLI that still honors the key rejects it with a hard LOCAL error (sub-second, zero JSONL events, before stdin is read, zero tokens; live-verified, probe 13) which Ping treats as healthy, while any other outcome returns a typed `InvalidRequestError` naming the degradation, so a deploy-time `ProbeAll` fails loud before the first compile call quietly quadruples its spend. A broader `--disable <feature>` combination measured a lower raw floor (11,456) but was deliberately excluded from the shipped argv: verified live that an unrecognised `--disable` value is a hard local error while an unrecognised `-c key=value` is silently tolerated, and `codex features list` already shows several feature names tagged "removed" in the currently installed version — proof this CLI actively retires flags across releases, making a hardcoded `--disable` list a forward-compatibility risk this adapter's compile-path reliability cannot absorb. `model_reasoning_effort` honors `Request.Effort` (real, verified live, unlike anthropic/bedrock). `--output-schema` genuinely enforces `Request.OutputSchema` (verified live, delivered through the same agent_message channel as any other completion). A real degradation relative to claudecli: codex's JSONL event schema carries NO `stop_reason` field at all, so `Response.StopReason` is always `StopEndTurn` and truncation can never be detected through this adapter. `turn.failed`/error mapping is pinned by status code for the one live-triggered failure shape (a bogus `--model` value, mapped 400 -> `InvalidRequestError`); auth-expiry (401/403) and usage-limit (429) are deliberately unprobed live per the design brief and mapped speculatively by message-keyword content, documented as such rather than presented as a pinned contract. Unsupported `Request` fields (`Tools`, `Thinking`, `Temperature`, `StopSequences`, `ProviderParams`, non-text `Messages` blocks) fail loud with a typed `InvalidRequestError`; `MaxTokens` is silently ignored (no CLI flag exists). Hermetic fake-shim tests only — 34 test funcs (including a two-in-flight concurrent-Complete race exercise, a >64KiB event-line scanner-buffer pin, a failure-wins partial-text-discard pin, and healthy/degraded/timeout Ping shapes), `-race` clean, zero network, zero real credentials touched. First subscriber: fathom's semantic-layer compile provider seam (a companion change in the sibling repo, the `codex-cli` branch of `FATHOM_LLM_PROVIDER`).

- **Spec-corpus revisions from the vision-fit audit (2026-07-16, four critic
  agents checked every in-flight/parked spec against the current fleet
  state):** `docs/plans/handoff-h4-sextant.md` — Stage 2's obligations-prompt
  half retired post-M1 (TextHash-derived `obligationsPromptVersion` already
  shipped; the enrich template rewritten to the same doctrine), Stage 5
  (Tier-1 ladder adapter + gap warming) parked `[-]` with a subscribe-when
  (zero Ladder consumers exist; Stage 5c daemon-warming wiring deleted
  outright), Stage 6 re-premised from "the index is stale" to "prove the
  index is fresh under the epoch salt" with the package-count proof made
  run-time-derived. `docs/plans/2026-07-fathom-asks.md` — FA-7
  (verdict-journal) trigger rewritten to an evidence-based condition (fathom
  already carries its own `core.ClaimEvent` store-native event format, so the
  old trigger's second clause was never satisfiable as written).
  `docs/plans/handoff-h7-patterns-bench.md` — WO-4 (batch API, roadmap #11)
  adjudicated NOT fired despite its verbatim trigger textually firing
  (eval-runner runs replay-mode only, zero live bulk calls yet); grounding
  refreshed (bedrock/noop/evidence-anchor/fitness-function-registry
  committed; roadmap #5/#15/#21 graduation noted, with roadmap.md's own
  annotation drift called out); self-referencing filename typo fixed
  (`h7-patterns-bench.md` → `handoff-h7-patterns-bench.md`).
  `docs/plans/handoff-h7-patterns-bench.md`, `handoff-h8-public-candidacy.md`,
  `handoff-h5-eval-runner.md` — the `fathom verify` `test_reaches`
  bug-and-bypass instructions retired (H1 landed 2026-07-08, fathom `b7f6476`;
  gate is non-vacuous); anchors verify through the live gate by default now,
  with only the O9 residue (aliased test calls, re-ambiguated methods) as a
  named fallback case. `docs/plans/handoff-h0-index.md` — roster table
  updated: H1/H2/H6 SATISFIED/EXECUTED, H5 Stage 1 done with stages 2–3
  parked pending owner ratification; live dependency graph restated.
  `TRACKING.md` — the stale "sibling fathom binary has a known broken
  `test_reaches`" mirror note corrected to reflect the fix.

- **`foundations/security/tokenization/vaulttest` (H3 Stage 2, throughline
  adoption): the shred-conformance suite is now importable.**
  `RunShredConformance(t, factory)` + `VaultFactory` move out of
  `tokenization`'s internal `package tokenization_test` into this new,
  non-test package so downstream adapters (e.g. a durable SQLite- or
  KMS-backed `ShreddableVault`) can run the canonical shred cases against
  their own implementation from their own repo. `tokenization`'s
  `conformance_test.go` is now a thin type-alias + delegation so existing
  internal callers (`encrypted_vault_test.go`) are unchanged. Deliberately
  behavioral-only (no `schema.cue`, per the documented exemption) and
  deliberately imports `testing` in a non-`_test.go` file (the whole point of
  an importable suite) — both noted in the package doc.

- **H8 Stage A — public-candidacy hygiene (2026-07-16; Stage B remains
  BLOCKED on D4, still undecided — see the [0.6.0] D4 section below):**
  - `cmd/constitution-lint` grows five checks (6 → 11): `compat-coverage`
    (every foundations/kits/runtimes/surfaces/toolchains package classified
    in docs/compat.md), `doc-coverage` (every exported package-level
    identifier documented, name-first; nested-module dirs skipped with a
    stated rationale; `doc-waivers.txt` escape hatch — shipped EMPTY: all 94
    first-run gaps across 23 packages were fixed in this same change-set),
    `org-agnostic` (token denylist in `org-denylist.txt` + model-ID-literal
    scan with a provider-adapter carve-out; waivers in
    `org-agnostic-waivers.txt`), `secret-scan` (well-known secret shapes
    from `secret-patterns.txt`, always-on, no sibling-binary dependency;
    planted-positive canary `TestSecretScanCanary` proves the committed
    patterns catch a fake credential — mutation-verified in review), and
    `readme-count` (README's package-count sentence must equal the
    source-counted catalog total — the "148" drift class is now
    gate-enforced). One-time `git log -p` full-history secret sweep run
    alongside: no real leaks (18 hits, all documented-safe test
    placeholders such as AWS's own published example access-key ID —
    not quoted here verbatim precisely because the secret-scan check
    now flags anything shaped like it, including in this file).
  - `docs/compat.md` — API stability audit (169 packages classified: 124
    stable / 21 experimental / 24 internal-ish, methodology stated for
    challenge; counts reflect a re-audit against throughline HEAD that
    falsified three "no consumer" rationales, and the
    experimental-with-live-consumer status of guardrails-pipeline is
    flagged for the owner rather than silently resolved), the compatibility
    promise (pre-1.0 and at-1.0, typed errors and CUE schemas included),
    and the v1 decision record (no `/v1` path suffix; nested modules tagged
    independently; tracked go.work rationale). Coverage enforced by
    `compat-coverage`; the classification itself awaits Geoff's sign-off
    before any public tag.
  - `docs/threat-model.md` — kit-as-dependency threat model; VISION belief
    6 restated verbatim as the ceiling; no new guarantees invented; "add it
    yourself" gaps (real sandboxing, deletion outside the vault) named
    plainly. Presence lint (`public-files`) is Stage B, so this entry is
    the doc's record of existence per the plans-README convention.
    Review hardening: the self-review's adversarial pass found a
    pre-existing overclaim — docs (practices.md §14, copied into the first
    threat-model draft, and `CallTool`'s own doc comment) claimed the
    mcp-client SHA-256 description pin is re-verified "before every
    CallTool"; the code has never done a pre-call pin check (verification
    happens at ListTools only). All three docs corrected to match the code,
    and the pre-call check is proposed as pull-queue row H8-1 (named
    consumer: the Bridge path; decision Geoff's) instead of being papered
    over or silently implemented. Also qualified: tool-result screening
    runs only when `Deps.Detector` is wired (nil = no screening).
  - `cmd/release-notes` + `task release-notes -- vX.Y.Z` — prints one
    version's CHANGELOG section verbatim (`ExtractSection`, tested against
    fixtures and the real CHANGELOG; single-shot CLI, not safe for
    concurrent use).
  - `examples/agent-cli/walkthrough_test.go` — `TestFirstAgentWalkthrough`
    golden-checks docs/first-agent.md's fenced mock-mode output against the
    live CLI in-process; caught and fixed a real drift (`turns=2` → the
    actual `turns=3`); mutation-verified (doc edited → test fails red).
  - `docs/plans/public-readiness.md` — the Stage-A checklist as verified
    claims in the gated grammar (22 bounded-everything claims re-verified
    against real pinning tests; 1 honest gap recorded: cmd/endpoint-guard's
    GC-goroutine wiring has no dedicated test); D4 and H1 status quoted in
    the header.
  - README refreshed: generated package count (lint-enforced), stale
    `helpers/` and retired-package references removed, `downstream-check`
    promoted, compat/threat-model links added.

- **Proof gate closed out (H1 tail, 2026-07-16):** daemon restarted on the
  fixed fathom binary; HTTP-mode acceptance 15/15 (initial 13/15 root-caused
  to stale-extractor observations, fixed by fathom's extractor-epoch
  fingerprint salt — O9); first NON-VACUOUS pre-push sweep blocked a real
  push by catching 13 malformed acceptance contracts across five handoff
  plans, repaired with cited skips. Done-means-proven is live end to end.

- **Fathom asks W1 (docs/plans/2026-07-fathom-asks.md — three seams the
  fleet's instrument needs, each behind an observed failure):**
  `prompttemplate.Template.TextHash()` — content-addressed template identity
  for downstream digest gates (per-template, distinct from per-render
  `PromptHash`; motivated by a prompt bump silently keeping 27/32 stale
  sections). `foundations/llmclient/structured` — typed strict-JSON
  completion over the `OutputSchema` seam: `ErrTruncated` classifies
  StopMaxTokens as the budget error it is (a fleet compile once aborted on
  "invalid character 'd'" that was really truncation), `ErrMalformed`
  catches schema drift via unknown-field-strict decode; callers own policy.
  `foundations/telemetry/heartbeat` — byte-stable JSONL channel beats +
  pure `Rollup`/`CheckLiveness` projections (paired if/then expectations →
  advisory alarms), so forced channels get a denominator and a silent
  channel is detectable (motivated by fathom's injection channel being dark
  four days unnoticed). Parked with subscribe-when triggers per D8:
  verdict-journal, tier1 conformance kit, eval verdict reader, shared tier
  vocabulary.

- **`kits/eval-runner` + `cmd/eval-runner` (H5 Stage 1 — roadmap #5
  graduated): the honest QA loop is live.** Judge–producer independence is a
  construction-time fatal error (cross-family judge required for decisive
  verdicts, families/lineage from model-catalog CUE data); judge stack probed
  before any run; every judge call budget-charged fail-closed and journaled;
  regression gate over eval-journal baselines; determinism variance probe;
  provenance-gated scenario bank (committed corpus: throughline chat-prompt
  scenarios, donor commit-pinned and redacted, + agent-cli-derived; cassettes
  byte-stable, recorded offline). `task eval` runs the whole loop in replay
  mode — deterministic, free, no network. Judge pool/panel/escalation and
  conviction statistics remain parked behind their subscribe-when triggers.
- **Protocol surface completion (H6).** `kits/mcp-server` gains the
  Streamable HTTP transport (stateless POST, protocol-version negotiation,
  fail-closed Origin allowlist, byte-identical dispatch vs stdio pinned by a
  shared conformance table); `foundations/agent/mcp-client` gains the HTTP
  transport (typed status errors, bounded bodies, SSE fail-closed) — kit
  agents can now consume remote MCP servers; `foundations/telemetry/event`
  gains the W3C traceparent codec + env carrier (roadmap #21 graduated):
  cross-process trace propagation over MCP HTTP hops and bounded-exec
  subprocesses, proven end-to-end by `TestStreamableHTTPTraceRoundTrip`.
- `foundations/security/token-lifecycle` (H2/D35-3, kit side) — pure OAuth
  credential lifecycle extracted from throughline's proven `adapters/oauth`:
  `Classify`, `NextAction` (60s default refresh skew, boundary pinned ±1ns),
  `MergeRefreshed` (preserve-refresh-token rule), `RevocationTarget`.
  Storage, PKCE, and HTTP exchanges stay consumer-side by design.
- `foundations/evidence/authenticity`: per-method grading guidance
  (H2/D35-4) — `ScoreCeiling` (DKIM 0.95, Authentication-Results 0.55),
  `VerdictCeiling`, opt-in `ClampToCeiling`. `Assess` unchanged; clamping
  never lifts adverse verdicts and does not retro-fix stored assessments.
- `foundations/review/intake-triage`: `ResumeFSM` — hydrate the proposal FSM
  at any persisted state including terminals (history-since-resume semantics
  documented); the seam persistence-backed consumers needed for adoption.

### Removed

- `foundations/communications/source-sync-checkpoint` (H2/D35-5 decision:
  coverage wins) — zero consumers ever; throughline's doc-29 deviation proved
  checkpoint intent belongs on `coverage.SourceCoverage.Cursor`/`LastSync`,
  now documented there as the canonical mechanism. No migration needed.

### Changed

- **Pull queue closed (H2 Wave B): extraction-done = donor-deleted, proven.**
  Throughline (changes left uncommitted for owner review): `app/acquire`
  adopts `intaketriage` (ProposalState aliased; `Service.Decide` hydrates via
  `ResumeFSM`, routes through `FSM.Decide`/`Restore`; hand-rolled `ai:` guard
  and local state constants deleted) and `queryplanner` (new `gatedRefiner`
  adapter; `extractKeywords`/`stopwords`/`buildGmailQuery`/`extractEmails`/
  `llmRefineQueries` deleted); `adapters/oauth` rewires onto `tokenlifecycle`
  (four donor logic blocks deleted). Zero test-assertion changes across all
  three parity canaries; throughline gate green with an identical pass set to
  the pre-change baseline; `task downstream-check` green.

### Fixed

- **The done-means-proven gate is re-armed (handoff H1).** `fathom verify`'s
  `test_reaches` failed every anchored claim while its own index disagreed —
  releases v0.5.0/v0.6.0 were pushed with the gate bypassed and claims
  hand-verified. Seven distinct defects fixed in the fathom repo (sextant
  branch `fix/verify-proof-gate`, feature-base; `fix/verify-test-reaches` is
  the main-based reference): callgraph ambiguity poisoning from duplicate
  observations; unresolvable receiver-variable method edges; the `package:`
  plan annotation parsed-but-dropped; `.claude/` agent worktrees entering
  the index (skipDir + orphan purge on full ingest); import-alias callees
  recorded under file-local aliases; bare in-package cross-file calls going
  globally ambiguous (Go semantics: they bind to the caller's package); and
  constructor-scoped receiver method calls (`l := pkg.New(...); l.Run(...)`)
  never resolving. Every fix is pinned by a red-first regression test
  (artifacts in the fathom repo, `docs/dogfood/h1-red-first/`). All 15
  anchored `[x]` claims across the four capability plans verify 15/15
  against a fresh store-mode index. Pending (human-gated): daemon restart
  via `scripts/reingest-fleet.sh` + branch merge, then HTTP-mode acceptance
  re-arms the pre-push hook end to end.
- `.sextantignore` gains `.claude/` — agent-worktree copies are tool state
  and must never enter the fleet index (belt-and-braces with the fathom-side
  skipDir fix).
- **Downstream-consumer path defaults survive the fathom repo's pending
  directory rename (owner order 2026-07-17).** The fathom repo dir will
  eventually rename `sextant` → `fathom`; `lefthook.yml`'s `fathom-verify`
  (pre-push) and `fathom-refresh` (post-commit) hooks, and `Taskfile.yml`'s
  `sextant-check` (rolled into `downstream-check`), hardcoded
  `../sextant/bin/fathom` / `-d ../sextant` as their only default. `FATHOM_BIN`
  still wins outright when set; the no-override default now resolves the
  first-existing of `../fathom/bin/fathom` then `../sextant/bin/fathom`
  (`fathom-refresh` additionally falls back to the older `../sextant/bin/sextant`
  binary name, unchanged from before). `fathom-refresh`'s ingest `--store` path
  is derived from wherever `$FATHOM` actually resolved (`${FATHOM%/bin/*}`)
  instead of a second independent `../sextant` hardcode, so it can never
  disagree with which checkout supplied the binary. `README.md`/`CLAUDE.md`
  prose annotated to name both directory names without a full rewrite.
  **Same-cause fix, this repo's own file:** removing fathom's `.sextantignore`
  fallback (the companion fathom-side change) meant agentic-kit's own
  `.sextantignore` — the only ignore file this repo had — would stop being
  honored on the next fleet re-ingest, silently reopening the `.claude/`
  worktree-pollution hole H1 closed above. Renamed `.sextantignore` →
  `.fathomignore` (content unchanged) in the same change so nothing regresses.

## [0.6.0] — 2026-07-10

Wave 4 ("Prune & prove") of `docs/plans/2026-07-uplift.md` — the final wave.
Kill-list sweep (D5/§6): 16 packages deleted, zero consumers verified across
the repo + throughline + sextant + growing-gardens-bi. New on-ramp tooling:
`cmd/kit-new`, `scenarios/governed-docextract-hitl`, `docs/first-agent.md`.
Verified-claims plan files with `{symbol:}` anchors. Kit consumer annotations
and honesty passes throughout. Gate green; catalog regenerated to 174 packages;
`constitution-lint` all checks passed.

### Removed

**Runtime pipeline (6 packages) — D5/§6 kill list, zero consumers verified**

- **`runtimes/bridges/`** — foundation→runtime bridges (outcome→journal
  projection, escalation, concern construction); adapters for a pipeline that
  doesn't exist; the pattern lives in `kits/agent-harness` hooks today.
- **`runtimes/connectors/`** — `Channel`-polling capture runtime + MockChannel;
  zero consumers; the harness's tool-dispatch is the in-kit equivalent.
- **`runtimes/execution/`** — governed Runner (policy→guard→DLP-redact→LLM);
  execution story consolidated into `kits/agent-harness`; keeping a second
  governed-execution story was the confusion, not the feature.
- **`runtimes/security/`** — content inspection guards (untrusted-instruction /
  credential-scope / repair-broadening); kept only if `execution/` was rebuilt
  on the harness lineage — it wasn't; `kits/guardrails-pipeline` is the
  replacement path.
- **`runtimes/ingest/`** — raw→telemetry.Event normalization pipeline; zero
  consumers; callers normalize events directly via `foundations/telemetry/event`.
- **`runtimes/mocks/`** — `StubRunner`/`StubGuard` shared doubles; depended on
  the retired `execution/` package; no remaining consumers.

**Foundations (6 packages) — deprecation-watch graduates to retired**

- **`foundations/communications/participation-timeline`** — per-author
  first/last/count stats; derivable in a few lines from captured messages; zero
  consumers verified.
- **`foundations/distribution/artifact-builder`** — sha256 content-addressed
  memoized copy of its input bytes; "builder" overstates (no transform step);
  its only importer was sibling `artifact-store`.
- **`foundations/distribution/artifact-store`** — versioned `Store` +
  `MemoryStore`; zero consumers beyond sibling `artifact-builder`.
- **`foundations/retrieval/chat-retriever`** — index-backed `Retriever` +
  embedding reranker stub; superseded by `evidence-ranker` (LLM-free
  deterministic feature ranker, properly named and scoped).
- **`foundations/retrieval/lookup-index`** — inverted index, overlap ranking;
  self-described as a stub; superseded by `evidence-ranker` + `vector-store`.
- **`foundations/topology/topology-task-type`** — three-value task-kind enum
  (rollout/validate/drain) + `Validate`; no engine in the kit executes these
  tasks; zero consumers.

**Surfaces (1 package)**

- **`surfaces/demo/`** — capability demo registry shell; superseded by
  `scenarios/` + `docs/first-agent.md` as the newcomer on-ramp.

**Toolchains (1 package)**

- **`toolchains/generators/`** — deterministic Go-const + JSON-Schema
  generators; intent fully absorbed by `toolchains/jsonschema` (roadmap #16,
  graduated Wave 1).

### Added

- **`cmd/kit-new`** — governed-by-default scaffolder: generates a new consumer
  module from the `agent-cli` blueprint with budget cap, injection screen in
  block mode, journal to `workspace/`, telemetry/cost, and retry wired into
  the generated `main.go`. `-minimal` flag opts out with a loud comment.
  `go:embed` templates; golden + integration-build tests — 15 tests.
- **`scenarios/governed-docextract-hitl`** — the first in-repo proof that "you
  compose kits" is true: docextract (CSV, dep-free) → agent-harness (MockClient,
  budget + journal on) → hitl propose→preview→approve→apply; run-report
  rendered; skeptical-newcomer README — 10 deterministic tests.
- **`docs/first-agent.md`** — timed newcomer walkthrough (zero to governed in
  10 minutes); code blocks hand-verified against the current API; linked from
  README as the first "Start here" entry.
- **`docs/plans/{llm-surface,agent-loop,evidence-spine}.md`** — 12 verified
  claims with `{symbol:}` anchors, all grep-verified to exist and be
  test-reached; the fathom verify pre-push gate enforces these files. Note:
  the sibling fathom binary has a known broken `test_reaches` (contradicts its
  own index); claims are hand-verified until that is fixed.
- **`specs/`** real-or-retired pass: `contracts`, `llmclient`,
  `injection-detector`, `evidence-graph` = REAL (checkboxes filled, enforcing
  tests named); `review/correction-ledger` = PARKED (design evolved into
  `claim-lifecycle`'s different FSM; subscribe-when noted in spec).

### Changed

**Honesty passes (no API changes)**

- **`foundations/coordination/swarm-orchestrator`** — package doc: "spawns
  workers" → "plans wave batches; does NOT spawn or execute; executor is D7's
  kits/worker-pool, unbuilt." All six coordination/* packages annotated
  plan-only.
- **`foundations/orchestration/workflow-engine`** — package doc: "DAG/FSM" →
  "FSM" (no DAG topology exists in the engine; the reference file had DAG
  language that was never implemented).
- **`foundations/topology/live-graph`** — package doc: plan-only annotation
  added; does NOT spawn or execute; subscribe-when: real fan-out workload.
- **`foundations/orchestration/operator-elicitation`** — package doc demoted
  per D2: "composition unproven" language added (throughline's richer
  elicitation shape is domain-specific; absorption would need org types).
- **`kits/mcp-server`** — TRACKING annotation: "composition unproven —
  serving side has no in-fleet consumer yet."
- **`kits/command-center`**, **`kits/governed-prose-kit`** — TRACKING
  annotations: "composition unproven — no in-fleet consumer yet."
- **`kits/guardrails-pipeline`** — TRACKING annotation: "subscribers named,
  wiring pending."
- **Catalog regenerated** — `docs/catalog.md` rebuilt to 174 packages (down
  from prior count; 16 packages deleted).

### Adoption round-trips recorded (Wave 4)

- **sextant→vector-store**: Deferred. Sextant's feature branch is mid-flight;
  kit side ready. Subscribe-when: the branch merges and the embedding pipeline
  is wire-tested.
- **growing-gardens-bi→kits/hitl**: Not-yet — shape mismatch (no
  proposal/preview/approve cycle exists in its current code); adopt-worthy in
  principle; subscribe-when: a concrete human-approval change request is
  identified.

### D4 (public/1.0)

Presented to owner with the v0.6.0 ledger. Decision is Geoff's.

## [0.5.0] — 2026-07-10

Wave 3 of `docs/plans/2026-07-uplift.md` — Evidence moat + pull burn-down.
Every item in the Wave 3 checklist is done or honestly annotated; gate green;
TRACKING.md updated; roadmap entries #10, #18, #19, #20, #24, #25 graduated.

### Added

**Evidence moat (conflict keystone fix + acquisition ladder)**

- **`foundations/evidence/conflict` — `DetectWithPropositions`** — three-way
  join (`Assertion.PropositionID == Proposition.ID`, `Proposition.Subject ==
  Span.EntityID`); field-aware conflict detection; `Coverage` diagnostics with
  `Degenerate()` method (silent-zero-join is now a loud error, not a silent
  empty result); regression fixture mirrors the flagship consumer's disjoint ID
  scheme. Fixes `[evidence-spine/join]`.
- **`foundations/evidence/evidence-signature`** (D8, Update-2) — `#ConcernSignature`:
  per-concern evidence contract; Scope as path globs XOR registered source IDs;
  canary MANDATORY for absence archetypes; `PreconditionSatisfied`;
  `LivenessCheck` CI helper; `CoverageManifest` with D10 skip taxonomy +
  `Complete()`. schema.cue · types.go · tests.
- **`foundations/evidence/evidence-ladder`** (D8, D10, Update-2) — Tier1/2/3
  interfaces; deterministic tier progression; `ABSENT_PROVEN` scoped-at-snapshot
  with coverage manifest + mandatory canary; fail-closed to `ABSENT_UNVERIFIED`
  on unexplained skips; Tier-3 timeout → `UNRESOLVED`, never absent;
  `HumanVerdict.Feedback` mandatory; hitl composition documented, not imported.
  schema.cue · types.go · tests.
- **`foundations/evidence/direct-investigator`** (D8, D10, Update-2) — pure-Go
  `WalkDir`+regexp Tier-2 contract; skip accounting IS the guarantee; optional
  `rg` Engine via `bounded-exec` with explicit-path-only equivalence-tested;
  refs as `file://…#Lx-Ly` via `evidencespan.Locator`. tests.
- **`foundations/evidence/extraction-gap-log`** (D8, Update-2) — `GapRecord`s
  via `jsonl-log`; `HitRate`/`WarmList`/`Signals` feeding
  `improvement-flywheel`. tests.
- **`foundations/evidence/case-file`** (roadmap #25) — pure deterministic
  projection: stance buckets by assertion type, tier-rank weighted
  support/against scores with documented math, net classification bands;
  byte-identical golden-tested; throughline UI is the named subscriber. tests.

**Eval slice (D8, D9, Update-2)**

- **`foundations/quality/eval-journal`** — `#EvalRecord`; byte-stable JSONL
  via `jsonl-log`; empty-rationale ⇒ non-decisive enforced. schema.cue ·
  types.go · tests.
- **`foundations/quality/scenario-bank`** — provenance discipline
  (production-captured rejected in committed data); `RubricJudgeFrom` compiling
  into existing `qa-harness` `RubricJudge`; `HistoryFor` projection.
  schema.cue · types.go · tests.
- **`foundations/quality/diff-reporter`** — `Diff` + `TrajectoryDiff` (one
  primitive, two roles: trajectory golden-diff + per-concern QA-run delta);
  caller-config thresholds with units. tests.
- **`foundations/quality/determinism-gate`** — structural primary: K-run
  variance probe, agreement/entropy/flips; sampling opportunistic per
  model-catalog capability caps; `Stable`/`Unstable` measured bands;
  `FitnessFunction` loop-honesty checks in `fitness.go`. schema.cue · types.go
  · tests.

**Cross-family judge lane (D9)**

- **`foundations/llmclient/openai`** (roadmap #10, promoted per D9) — full
  chat-completions adapter: tools, streaming with per-index `tool_call`
  accumulation, images as data URIs, `OutputSchema`→`response_format
  json_schema`, `Effort`→`reasoning_effort`, `Thinking` typed-rejected,
  `ProviderParams` merge, typed errors + `Retry-After` both forms (seconds +
  HTTP-date), `Ping`/`Pinger`; key-never-in-errors; through fattened `llm-test`
  conformance (18 subtests) + 14 wire tests. Roadmap #10 graduated.

**RAG truth layer (roadmap #24)**

- **`foundations/retrieval/truth-bridge`** — `#ChunkLink` span↔chunk link;
  `Graded()` with loud `GradeUnknown`, never silent. schema.cue · types.go ·
  tests.
- **`foundations/retrieval/retrieval-metrics`** — Precision/Recall/MRR +
  `FalseAbsentRate` from canary records + `CoverageFraction`; per-archetype
  applicability matrix; typed not-applicable, never NaN. schema.cue · types.go
  · tests.
- **`foundations/retrieval/evidence-ranker`** — LLM-free deterministic feature
  ranker: pattern strength / path specificity / hit density / ref stability /
  grade tier; caller-config weights; lexicographic tie-break; similarity mode
  marked non-deterministic and REFUSED for absence archetypes with
  `ErrSimilarityRefusedForAbsence`. The notes' garbled modes re-derived from
  first principles, not ported. tests.

**Security — shared rule core + guardrails pipeline (roadmap #19)**

- **`foundations/security/rule-core`** — the ONE compile/`ScanFirst`/
  `ReplaceFixpoint`/`Ordinal` core under the four regex engines; all four
  engines refactored to delegate; exported APIs unchanged; their tests pass
  unmodified.
- **`kits/guardrails-pipeline`** (roadmap #19) — pre-LLM chain
  (wrap→inject-detect→classify→redact) + post-LLM chain
  (integrity→honesty→DLP→disclaimer); ONE `Decision` envelope
  `{Allowed, Stages[], Transformed}`; fail-closed on stage error; CUE chain
  config; sink seam; `llmegress` documented as the transport backstop.
  Subscribers: throughline chat + governed-prose-kit (adoption sketches in
  package doc). Roadmap #19 graduated.

**Retention / crypto-shred (roadmap #18)**

- **`foundations/security/tokenization`** — `KeyStore`/`MemoryKeyStore`
  (AES-256-GCM envelope keys); `EncryptedVault` + `ShreddableVault` extension
  interface (existing `Vault` consumers unchanged); `Shred(namespace)` =
  crypto-erasure with typed `ErrShredded`; `RunShredConformance` exported
  conformance suite (6 cases). Roadmap #18 graduated.
- **`foundations/governance/retention-policy`** (WASM-safe) — pure `Due()`
  evaluation of caller-supplied retention rules; `shred|flag|report` actions;
  the foundation decides, callers execute. schema.cue · types.go · tests.

**Observability**

- **`foundations/telemetry/otel-genai`** — semconv v1.27.0 `#AttrMapping`
  (CUE); OTLP/JSON traces sink; spans for `llm_call`, span events for the
  rest; drop-and-count failure policy; batch + `Flush`; no goroutines.
  schema.cue · tests.
- **`foundations/review/run-report`** (roadmap #20) — `Render(Input)`→
  deterministic markdown: summary / activity / timeline / flags / cost;
  "not captured" degradation; golden + harness-integration tested. Roadmap #20
  graduated.

**Pull queue burn-down**

- **`foundations/security/tokenization`** — `GateFromVaultOpts` +
  `WithDeterministicTokens` (F1 closed); `VaultRule.Action` block field +
  typed `BlockedError` (F2 closed); `MemoryVault.TokenizeDeterministic`.
- **`foundations/security/untrusted-content-wrapper`** — `WrapWithNonce`
  (F5 closed; threat tradeoff documented).
- **`foundations/prompt/prompt-template`** — `ParamsFromStruct` (F6 closed;
  json-tag canonical params; key-drift regression pinned by test).
- **`foundations/review/intake-triage`** (D35-1, kit-side done) — human-only
  FSM: search→proposal→(imported|excluded(reason)|deferred); reversible
  exclusion; AI-suggest-only. schema.cue · types.go · tests.
- **`foundations/investigation/query-planner`** (D35-2, kit-side done) —
  deterministic expansion + `Refiner` seam + `ScreenFunc`. schema.cue ·
  types.go · tests.

**Persistence**

- **`foundations/provenance/event-store`** — JSONL persistence:
  `AttachSink`/`JSONLSink`/`LoadJSONL`/`NewStoreFromJSONL`; payload-hash-only
  persistence (payloads are hashed, not stored inline); sqlite/kafka adapters
  are consumer-side.

### Changed

- **`foundations/security/rule-core` refactor** — the four regex engines
  (`injection-detector`, `dlp-evaluator`, `pre-llm-dlp-gate`,
  `redaction-executor`) now delegate to a shared `rule-core` under the hood.
  All exported APIs are **unchanged**; callers see no difference. Their test
  suites pass unmodified.
- **`kits/investigation`** — migrated to canonical `evidencespan.Span`;
  findings carry `SupportingEvidenceIDs`; `EvidenceDegraded=true` auto-raises
  a high-severity "finding_without_evidence" concern flag (fail-closed-honest).
  Zero external consumers verified before migration.

### Deferred (honest annotation)

- **`evidence-graph` Span migration (roadmap #23)** — assessed NOT CHEAP:
  TTL coupling + 250+ line ripple across the graph package; `kits/investigation`
  migrated this wave, but `evidence-graph`'s own graph queries still use the
  legacy internal Span. Honest assessment recorded in `foundations/evidence/README.md`
  and roadmap #23. Marked `[~]` in TRACKING.md.
- **D35-1 `intake-triage` donor deletion** — kit core landed; throughline donor
  (`app/acquire`) NOT deleted due to uuid/ledger entanglement. Honest note in
  package doc. Full extraction closes when throughline wires the kit type.
- **D35-2 `query-planner` donor deletion** — kit core landed; throughline donor
  (`app/acquire/planner.go`) NOT deleted; needs a `GatedClient`→`Refiner`
  adapter on the throughline side. Honest note in package doc.

## [0.4.0] — 2026-07-10

Wave 2 of `docs/plans/2026-07-uplift.md` — The governed loop becomes the real
on-ramp. Every item in the Wave 2 checklist is done or honestly annotated;
gate green; TRACKING.md updated; roadmap entries #1 and #9 graduated.

### BREAKING

- **`conversation.Compactor` signature widened** to
  `func(ctx context.Context, dropped []Message) (Message, bool, error)`.
  The prior signature `func([]Message) Message` had zero consumers (the
  seam existed but no caller supplied a non-nil Compactor); the widening
  adds context propagation and error return so compaction failures can stop
  a run cleanly rather than losing messages silently. Update any code that
  constructs a `Compactor` value.
- **`kits/agent-harness` API additions** (not removals, but callers that
  embed `Config` by value in tests may need recompilation):
  `Config.ContextBudget`, `Config.Conversation`, `Config.Hooks`, and
  `Config.ParallelTools` are new fields; zero values preserve prior
  behaviour. `New` no longer rejects a nil Registry — callers using
  `RunOnce`/`RunOnceStream` (no tool loop) may now omit the registry.

### Added

- **`kits/agent-harness` rebuilt around a single `runLoop`/`loopSink`
  seam** — 65 tests (was 16). New entry points:
  - `RunConversation` — continuation run from caller-supplied prior messages.
  - `RunFromCheckpoint` / `Checkpoint` — snapshot bundle {version,
    conversation snapshot, optional budget snapshot, turn cursor}; restore
    and resume a run without replay. Closes roadmap #9.
  - `RunStream` / `RunStreamFromCheckpoint` — identical governed loop,
    streaming; same budget charge, same injection screen, same stop causes;
    `HarnessEvent` channel; no goroutine leaks under `-race`.
  - `RunBlocks` / `RunStreamBlocks` — multimodal user turns: caller supplies
    `[]llmclient.ContentBlock` instead of a string.
  - `RunOnce` / `RunOnceStream` — governed single completion: budget + journal
    + telemetry + `PreModelCall` hook, no tool loop; nil Registry is allowed
    for RunOnce-only harnesses. Built as a direct result of the oracle
    friction log (pull-ordered doctrine working as designed).
- **`Config.ContextBudget`** — wires `conversation.WindowCtx` before every
  model call; model sees the windowed view, conversation keeps ground truth;
  compaction is journaled; a compactor error stops the run cleanly, never
  silent loss.
- **`Config.Conversation`** — passthrough of `conversation.Options` for
  callers that need to tune windowing or compaction parameters.
- **`Config.Hooks{PreModelCall, PreDispatch, PostToolResult}`** —
  synchronous, fail-closed hook seam; any hook returning an error stops the
  run with `StopHook`. `PreDispatch` deny returns a `denial` tool_result
  and continues the loop (the model sees the denial, not a crash).
  Identical behaviour on the stream path. This is `kits/hitl`'s integration
  point and the landing zone for external guardrails.
- **`Config.ParallelTools`** — opt-in bounded-concurrency parallel tool
  dispatch; result order is preserved; each result goes through the injection
  screen and hooks; proven `-race`-clean.
- **`foundations/agent/evidence-compactor`** (NEW — closes roadmap #1,
  practices §1 closer) — journals dropped messages verbatim to a
  caller-supplied `io.Writer` (jsonl-log format) BEFORE the LLM summary
  call, so the ground truth is durable even if the model call fails;
  versioned prompt-template with `PromptHash`/`ParamsHash` provenance;
  summary carries a structured `[COMPACTION: …]` marker with a journal ref;
  LLM failure → never a lossy fallback (error returned, no silent
  substitution). schema.cue · types.go · 12 tests.
- **`foundations/llmclient/instrument` — `WithBudget` middleware** —
  pre-flight token estimate → `ChargeTracked` → model call → `Settle` to
  actual token count; per-logical-call granularity, sits outside `WithRetry`
  so retries don't double-charge; streaming settles at the terminal chunk;
  typed `BudgetExceededError` is non-retryable. `ErrBudgetExceeded` sentinel
  exported for `errors.Is` compat via `Unwrap`. Closes friction F3.
  27 tests total in the `instrument` package.
- **`foundations/governance/budget-guard` — two-phase `ChargeTracked`/`Settle`**
  — `ChargeTracked` reserves an estimate and returns a settle func;
  `Settle(actual)` adjusts to ground truth; idempotent (double-settle
  is a no-op); WASM-safe preserved. Closes friction F4.
- **`foundations/agent/tool-registry` — lethal-trifecta capability taxonomy**
  — `CapabilitySet` with nil-unknown vs declared-empty distinction;
  `TrifectaStrict` fail-closed registration mode blocks tools that complete
  the reads-untrusted / reads-private / writes-external trifecta without a
  `TrifectaOverride{AuthorizedBy, Reason}`; unknown capabilities treated as
  worst-case under strict mode; `mcp-client` `RegisterAll` gains
  `BridgeOptions.CapabilityMap` to annotate foreign tools on bridge.
  `docs/practices.md` §13 (trifecta), §14 (tool poisoning), §15 (memory
  poisoning, partial — honest). 37 tests total.
- **`conversation.WindowCtx`** — primary windowing API; `Compactor` error
  semantics enforce no-silent-loss.

### Changed

- **throughline budget adoption** — throughline's hand-rolled `budgetClient`
  (~62 lines in `adapters/llmclient/`) deleted; `llminstrument.WithBudget`
  wired in its place; throughline gate green, vendor re-synced. Extraction
  complete per doctrine amendment §3.
- **`kits/agent-harness`** `New` no longer returns an error for a nil
  `Registry` (previously rejected); callers using RunOnce-only harnesses
  that have no tools may now pass nil.

### Deferred (honest annotation)

- **Oracle chat-loop port**: the Wave 2 plan called for porting throughline's
  chat loop onto the harness. The oracle session found that throughline's
  chat is a single-turn grounded completion, not an agentic loop; forcing
  the harness onto it would be fake complexity. The friction produced two
  real kit items instead (`RunOnce`/`RunOnceStream` + `ErrBudgetExceeded`),
  both built this wave. Full harness-loop port awaits throughline's first
  agentic workload (M18.1 per its roadmap).
- **Sextant budget-stack wiring**: sextant's checkout was mid-flight on a
  feature branch with a dirty tree at Wave 2 close; wiring deferred to
  Wave 3 once its branch is merged.

## [0.3.0] — 2026-07-09

Wave 1 of `docs/plans/2026-07-uplift.md` — Provider & protocol currency. Every
item in the Wave 1 checklist is done; gate green; TRACKING.md updated; roadmap
entries #12 and #16 graduated.

### Added

- **`Request.ProviderParams json.RawMessage` extension seam** — verbatim
  top-level wire merge AFTER adapter field mapping, so any undocumented
  provider capability is reachable today without a kit release. Both adapters
  (anthropic, bedrock) + MockClient validate that the value is a JSON object
  (not an array or scalar). Override precedence is documented: ProviderParams
  wins over any typed field it shadows.
- **`Request.Thinking *ThinkingConfig` + `Request.Effort`** — first-class
  thinking/effort controls: anthropic and bedrock map a `thinking` wire object;
  `Effort` is typed-rejected by both adapters with a clear error (no stable API
  knob yet; ProviderParams is the escape hatch). Conformance asserts
  mapped-or-rejected-never-dropped for all new fields. Temperature-on-frontier
  hazard (400s on extended-thinking models) documented in `doc.go`.
- **`Config.CapabilityCheck func(Request) error` seam** on anthropic and bedrock
  adapters — the model-catalog integration point. Nil = permissive; Wave-3
  judge independence wires this to ValidateIndependence.
- **`Pinger` interface + `ProbeAll`** — concurrent liveness probes over every
  lane; typed error per lane; absent-means-healthy contract. Bedrock gains
  `Ping` (signed GET /foundation-models on the control-plane host; proves
  credentials + IAM list permission + network reachability, NOT invoke
  entitlement — scope documented). Anthropic `Ping` already existed and
  satisfies the interface. (`probe.go`, 10 tests)
- **Structured outputs end-to-end** — `Request.OutputSchema json.RawMessage`
  maps to `output_config.format {type:"json_schema", schema}` on both adapters;
  `ToolDef.Strict bool`. Closes roadmap #16 via the new `toolchains/jsonschema`
  package (see below).
- **Raw preserved on ALL inbound content blocks** (not just unknown types) —
  round-trip fidelity for citations and server-tool payloads. Precedence rule:
  non-empty `Raw` wins on the outbound path; callers who edit received blocks
  must clear `Raw`. `ToolDef.Raw json.RawMessage` verbatim passthrough makes
  server tools (web search, code execution, memory tool) reachable without a
  kit release.
- **Streamed unknown-block delta accumulation** (roadmap #12 graduated) —
  stream-assembled blocks are now equal to Complete-assembled blocks, key order
  preserved; bedrock eventstream parity. The documented limitation
  ("start-JSON only") is retired.
- **`llm-test` conformance fattened** (~24 new cases) — unknown-block
  round-trip on both adapters + stream/complete equivalence; typed-error
  taxonomy including Retry-After seconds + HTTP-date forms; usage-on-complete
  and usage-on-stream; cache-hint limits gated behind `PreflightValidation`
  capability; stream `tool_use` terminal block; all Wave-1 surfaces. 39 tests
  total in `llmclient_test.go`.
- **`foundations/llmclient/model-catalog`** (package `modelcatalog`) — CUE
  `#ModelCatalog` / `#ModelEntry` schema: family, lineage, capability flags
  (thinking / effort / temperature-accepted / structured-outputs / seed), cost
  tier, deprecation. Go: `Load`, `Entry`, `Validate`, `ValidateIndependence`
  (judge/producer family+lineage separation; unknown model = violation). Testdata
  is example-only — no authoritative rows shipped (cost-tracker `PriceTable`
  precedent). Design re-derived from reconstructed Update-2 scan (D8).
  `schema.cue` · `types.go` · 20 tests.
- **`kits/mcp-server`** now negotiates protocol version `2025-11-25` (latest
  stable); design note `docs/designs/mcp-streamable-http.md` documents the
  Streamable HTTP + W3C trace context transport (deferred, triggers documented).
- **`foundations/agent/mcp-client`** (package `mcpclient`) — MCP consuming
  side: `Transport` seam + `IOTransport` (newline-delimited JSON-RPC over any
  `io.ReadWriter`); `Initialize`, typed `ListTools` / `CallTool`; tool-description
  SHA-256 hash-pinning with `ToolChangedError` + explicit `Repin` (tool-poisoning
  defense); `RegisterAll` bridge surfaces MCP tools into `tool-registry` so all
  governance (timeout, panic recovery, injection screen, policy hook) applies to
  foreign tools. Subprocess spawning is caller's job via `bounded-exec`
  (documented). 19 tests.
- **`toolchains/jsonschema`** (package `jsonschemagen`) — CUE → JSON Schema
  draft 2020-12 via the `cue` binary (OpenAPI export + pure-Go transform);
  deterministic output; `CheckFresh` for CI; honest errors on unsupported CUE
  constructs. End-to-end proof test (`outputschema_e2e_test.go`): CUE definition
  → derived schema → wire body round-trip. Implements roadmap #16 (graduated to
  TRACKING). 15 tests.

### Changed

- **MockClient unscripted Usage now defaults to 1 token each** — previously a
  MockClient response with no explicit Usage produced `{0,0,0}`, which made
  `WithCost` emit zero-cost events silently. The conformance suite caught this;
  MockClient now synthesizes `{InputTokens:1, OutputTokens:1}` when the whole
  Usage is unscripted. An explicitly scripted partial Usage (e.g. only
  InputTokens) is honored verbatim — the test author's intent wins.
- **Cassette byte-stability note** — Wave 1 adds `ProviderParams`, `Thinking`,
  `Effort`, `OutputSchema` fields to `Request`. Recorded cassettes from ≤0.2.1
  remain readable via v2 hashing (zero-value fields are dropped before hashing,
  so the hash is stable). Cassettes recorded pre-Wave-1 that byte-compare
  request bodies will need re-recording to include the new zero-value fields.

## [0.2.1] — 2026-07-09

Twelve commits landed since v0.2.0 while the ledger was frozen. This entry
restores ledger discipline (Wave 0.6 of `docs/plans/2026-07-uplift.md`).

### BREAKING

- **Cassette format** (`foundations/llmclient/replay`): new entries record
  `hash_version:2`. The v2 canonical form drops zero-value fields (empty
  strings, zero ints, nil/empty slices) before hashing, so additive contract
  growth no longer invalidates recordings. Old cassettes remain readable —
  the replayer re-hashes decoded v1 entries using v2 canonicalization. **To
  upgrade an existing cassette, delete and re-record** (`task smoke-live` or
  the `Recorder` API); the on-disk format is otherwise identical. The
  `HashVersion2` constant and `RequestHash()` helper are exported for callers
  using `MatchByRequestHash`.

### Added

- `foundations/llmclient` **image content blocks** — `BlockImage` type
  constant, `ContentBlock.MediaType` + `Data` fields (base64, no prefix),
  `ImageBlock(mediaType, data)` constructor; `MockClient` passes image blocks
  verbatim through both `Complete` and `Stream`; conformance suite covers the
  new block type (`19a63b4`).
- `foundations/llmclient/egress` **inbound detokenization** —
  `WithInboundDetokenize` wraps an inner client and applies `DetokenizeText`
  to every `Complete` response and streamed text chunk, restoring vaulted
  tokens after the model sees substituted placeholders. Best-effort: any token
  that cannot be resolved is left as-is. Pairs with `WithEgress`+`GateFromVault`
  to form a full tokenize-on-egress / restore-on-ingress round-trip (`78fb735`).
- `foundations/llmclient` **failover wrapper** — `WithFailover` wraps an
  ordered slice of `LLMClient` lanes with transparent failover on eligible
  errors (`AuthError`, `RateLimitError`, `UnavailableError`); per-lane sticky
  cooldown (configurable); zero-delta stream failover; `FailoverDecision`
  accessor for observability; passes `RunConformance` wrapping two
  `MockClient`s (`c1ced16`).
- `foundations/llmclient/bedrock` — AWS Bedrock provider: stdlib-only SigV4
  request signing, vnd.amazon.eventstream framing, streaming + complete paths,
  typed error mapping (throttle→`RateLimitError`, validation→`InvalidRequestError`,
  access→`AuthError`), config with region + injected credential resolver;
  passes `RunConformance`; design re-derived from reconstructed Update-2 scan.
  schema.cue · 28 tests.
- `foundations/llmclient/noop` — `llmnoop.Client` always returns a canned
  `StopReason:"end_turn"` refusal with zero cost and no network calls; passes
  `RunConformance`; intended as a safe default when no real client is wired.
  10 tests.
- `foundations/retrieval/fuzzy` — pure fuzzy match/rank foundation: `Match`
  (greedy subsequence, word-boundary + contiguous-run score bonuses, Unicode),
  `MatchTerms` (AND semantics over space-split terms), `Rank` (ranked slice
  with source index preserved; deterministic tie-break by insertion order).
  No dependencies beyond stdlib `sort`/`unicode`. 25 tests (`264f754`).
- `foundations/evidence/evidence-anchor` — Merkle proof-of-existence for
  `evidence-span.Span` values: `CanonicalDigest` (schema-level hash of all
  provenance fields), `AnchorSpans` (batches spans into `provenance/anchor`
  Merkle tree), `Verify` (standalone proof check); injected clock seam; all
  exported functions are pure and safe for concurrent use; design re-derived
  from reconstructed Update-2 scan. schema.cue · 16 tests.
- `foundations/evidence/authenticity` — `MethodDKIM` and
  `MethodAuthResultsHeader` constants: two new `Method` values extracted
  same-day from throughline's DKIM detector (doc-35 track, `cf40791`).
- `foundations/quality/fitness-function-registry` — typed registry of named
  `FitnessFunction` values (`prototype`/`integration`/`apex` tiers, threshold
  0–1, injected runner `func() (float64, error)`); `Register` validates on
  insertion (unrecognised tier, out-of-range threshold, duplicate name); `List`
  is deterministically sorted; design re-derived from reconstructed Update-2
  scan. schema.cue · types.go · 17 tests.
- `kits/docextract` — normalizes attached documents to prompt-ready text via
  a pluggable `Handler` seam (core root module: zero external deps, compiles
  with the root go.mod); format handlers `pdf` and `xlsx` live in nested Go
  modules (`kits/docextract/pdf/go.mod`, `kits/docextract/xlsx/go.mod`) each
  with their own external dependencies, preserving the root module's zero-require
  constitution (doctrine D1, `44cfc99`). 7 tests.
- `kits/hitl` — generic propose→preview→approve→apply engine
  (`Engine[State, Op]`): dry-runs each `Op` via injected `Applier`+`Cloner`,
  computes `Gauge` impact metrics before any live state change, optional
  `WithGuard` policy hook (fail-closed), `PreviewAll` for batch dry-runs. Fully
  generic over state and op types; safe for concurrent `Preview` calls (`5512d5f`).
  9 tests.
- `runtimes/credresolve` — credential-resolution runtime: explicit config >
  environment variable > Claude Code CLI config file, with macOS Keychain
  support (darwin build tag); typed resolution chain; injectable for tests
  (`c440004`). 15 tests.
- `docs/plans/2026-07-uplift.md` — 12-area adversarial audit findings + adopted
  uplift plan (Wave 0–4 roadmap, doctrine amendments, owner decisions D1–D10,
  Update-2 merge-map).
- `docs/upgrades/throughline-v0.2-adoption-friction.md` — 6 consumer-proven
  kit improvement asks from the throughline v0.2 adoption, plus doc-35
  acquisition addendum (5 extraction candidates). Drives TRACKING.md Pull queue.
- `VISION.md` — the owner's vision statement: what the kit believes, what it refuses to be, the junk gate, and the practice-watch process. The arbiter for scope decisions.
- `docs/practices.md` — agentic anti-pattern → kit-package mapping (which packages enforce which practice; honest about shipped vs partial vs roadmap).
- `docs/roadmap.md` — 25 pre-thought patterns awaiting a subscriber, each with a "subscribe when" trigger; referenced from TRACKING.md's Deferred section.
- anthropic client: bearer/OAuth auth (`Config.AuthScheme` + `BetaHeaders`, e.g. Claude Code OAuth credentials via `oauth-2025-04-20`) and a zero-token `Ping(ctx)` credential probe (GET /v1/models, typed error mapping). Key-never-in-errors tests cover both schemes.
- `llmclient.Swappable` — late-bound/hot-swappable client for runtime-acquired credentials (`NewSwappable`, atomic `Swap`, `Available()`); new typed non-retryable `UnavailableError`.
- `foundations/llmclient/egress` (package `llmegress`) — gate-before-egress middleware: `WithEgress` applies a fail-closed `Transformer` to every outbound text field (deep-copied, once per logical call under retry); `GateFromVault` (tokenization) and `GateFromDLP` adapters. Generalized from throughline's proven GatedClient.
- `prose/evidence-citation` parse direction: `NewParser(tags...)`, `Parse` (byte offsets), `Validate` (anti-fabrication seam), offset-safe `Strip`/`Replace`; `Cite`→`Parse` lossless round-trip, fuzzed.
- Taskfile: `throughline-check` (second downstream consumer, `-mod=readonly` to bypass its vendor dir) and `downstream-check` (both consumers).
- `docs/adopting.md`: third consumption pattern documented (replace + vendor, no go.work — throughline's hermetic pattern); surfaces/ tier importability blessed (cueform doc + adopting note).
- `docs/upgrades/throughline-v0.2-adoption.md` — paste-able adoption brief for the throughline project.

### Removed

- **BREAKING** `orchestration/workflow-engine`: the inert `RetryPolicy`, `TimeoutPolicy`, and `BackoffStrategy` types and the `Retry`/`Timeouts` fields of `WorkflowDefinition` are removed from Go, the CUE schema, and fixtures. They were declared but never executed by the engine — configuring them did nothing — and had zero known consumers (repo-wide and sextant). The package doc now states retry/timeout are intentionally caller-owned (e.g. `llmclient.WithRetry` for model calls).

## [0.2.0] — 2026-06-10

The four-commit uplift (`9b46c1a..ad9518c`): wave 0 security hardening + CI,
wave 1 llmclient v2 + ops/persistence/embedding spine, wave 2 first real
provider + agent primitives + cost/observability, wave 3 kits + production
endpoint-guard + the hello-world agent.

### BREAKING

- `llmclient`: `Stream` returns `(<-chan StreamChunk, error)`, was `(<-chan string, error)` — range over chunks; the terminal chunk carries `StopReason`/`Usage`/`Err`.
- `governance/proof-of-work`: package-level `Issue`/`Verify` removed (client-trusted challenges were forgeable) — construct a server-side `NewIssuer(secret, cfg, now)` and call `Issuer.Issue`/`Issuer.Verify`.
- `security/untrusted-content-wrapper`: `Wrap` returns `(Wrapped, error)` with a per-wrap nonce fence; spoofable `IsWrapped` removed — keep the returned `Wrapped` (its nonce) to unwrap.
- `llmclient`: `RateLimitError.RetryAfter` is now `time.Duration`, was `int` seconds — multiply old values by `time.Second`.

### Added

- `foundations/llmclient` v2 agentic contract: multi-turn `Message`/`ContentBlock` (tool_use / tool_result / unknown passthrough), `ToolDef` + `ToolChoice`, open `StopReason` constants, block-level prompt-cache hints, cache-token `Usage`.
- `foundations/llmclient/anthropic` — the kit's first real provider: stdlib-only Messages API client (Complete + SSE streaming, tool use, prompt-cache `cache_control` mapping, unknown-block verbatim round-trip, typed errors incl. 429 Retry-After + 529; key-never-in-errors pinned by test).
- `foundations/llmclient/replay` — record/replay cassettes (JSONL; DLP scrubbing ON by default; ordinal matching default, canonical-JSON hash opt-in; stream chunk segmentation preserved).
- `foundations/llmclient/llm-test` — `RunConformance` suite all clients must pass (anthropic and replay both pass it).
- `foundations/llmclient/instrument` + `foundations/cost/cost-tracker` — `WithTelemetry`/`WithCost` middleware emitting one `llm_call` event per logical call with trace correlation and cache-aware cost; config-driven price table (no prices shipped).
- `foundations/agent` — `tool-registry` (define a tool once: sorted defs, `Dispatch` with policy hook, panic recovery, per-tool timeouts), `conversation` (makes API invariants unrepresentable to violate: tool_use/tool_result pairing, role alternation, pair-safe windowing, snapshot/restore), `bounded-exec` (resource-bounded subprocess runner — explicitly NOT a sandbox, non-guarantees documented).
- `foundations/embedding` — `http-embedder` (OpenAI-compatible `/embeddings`, hermetic tests) and `vector-store` (`VectorStore`, `MemoryVectorStore`, `Cosine`, `Indexer`).
- `foundations/telemetry` — `metrics` (Prometheus text-format registry), trace correlation (`TraceContext`, context carry, reserved attrs), production sinks (JSONL, slog bridge).
- `foundations/cache/result-cache` — generic TTL+LRU memoization with single-flight `GetOrCompute`.
- `foundations/persistence/jsonl-log` — append-log writer/reader with torn-tail recovery surfaced.
- `foundations/prompt/prompt-template` — versioned render producing `PromptHash`/`ParamsHash` for ai-provenance.
- `kits/mcp-server` — stdio MCP server over agent/tool-registry: newline-delimited JSON-RPC 2.0, initialize version negotiation, tools/list + tools/call, bounded message size with fail-closed resync, batch rejection, JSONL call log (payloads excluded by default). Tools-only v1.
- `kits/agent-harness` — the governed agent loop: conversation + tool-registry + injected LLMClient, optional budget-guard charged fail-closed before every call, injection scanning of tool results (blocked payloads provably never re-enter the transcript), per-step telemetry, optional journal; stop causes are values, not errors.
- `cmd/endpoint-guard` production mode — real providers wired (anthropic + http-embedder), judge-only topic-gate fallback (`topicgate.JudgeOnly`), `http.Server` timeouts + `MaxBytesReader`, fail-closed startup (placeholder salt / missing key / inadequate PoW secret refuse to start), Prometheus outcome counters on `/metrics`, seconds-canonical config.
- `examples/agent-cli` — hello-world agent (mock/replay/live modes, committed scrubbed cassette, path-confined file tools via `os.Root`, cost report); `task smoke-live` (skips without `ANTHROPIC_API_KEY`).
- CI: `.github/workflows/ci.yml` (full gate, cue installed, wasm builds); Taskfile `sextant-check`, `fuzz`/`fuzz-long`, `wasm-build`, `smoke-live`; `CLAUDE.md` contributor/agent guide.
- Category READMEs for `foundations/evidence`, `foundations/provenance`, `foundations/reasoning` (incl. the two-Span-types warning).

### Changed

- llmclient retry honors `Retry-After` and never retries after tool_use; `Ladder` gains rung-model injection; `MockClient` extended with tool_use scripting (source-compatible).
- embedding `CachingEmbedder` is now bounded (LRU, default 4096); `DimensionedEmbedder` capability added.
- `review/scoring`: exported `Tier` alias + constants; rank-vs-ceiling asymmetry documented as intentional and pinned by test.
- `security/pre-llm-dlp-gate`: `MustDefaultGate`; `ExtendedPolicy` (high-precision secret shapes only: PEM/AKIA/GitHub/Slack/JWT/AIza); docs honestly scoped to best-effort shape redaction.
- `investigation-journal`: telemetry sink hook, JSONL round-trip persistence, `NewFromEntries`.
- `task cue-vet` now vets all `schema.cue` dirs (kits were silently unvetted) and `testdata/` fixtures (previously skipped by `cue ./...`); the widening exposed and fixed 64 latent fixture import qualifiers + 4 real fixture bugs.

### Fixed

- `security/redaction-executor` redacts to a fixpoint: FuzzRedact found a single-pass PII leak via boundary unmasking; true idempotency restored (MaxPasses cap, `Converged` surfaced).
- embedding `EmbedBatch` latent dedup bug.
- `runtimes/security`: zero-value `SecurityConfig` no longer silently disables the repair-capability-broadening guard (dead-branch bug); `execution.Runner` honors a custom `PayloadRedactor`.

### Security

- proof-of-work challenges are HMAC-SHA256-signed by a server-side `Issuer`; forged/tampered/zero-bit challenges rejected, the bypassable client-trusted API removed, `SeenSet` bounded (TTL eviction + hard cap, fail-closed). Consumers rewired (kits/endpoint-guard, cmd/endpoint-guard, examples/governance-wasm).
- untrusted-content-wrapper uses per-wrap unguessable nonce fences (embedded delimiters can no longer forge a fence); docs honestly scoped to defense-in-depth, not a security boundary.
- injection-detector recall 12/22 → 22/22 on the seeded corpus (INJ-001/003 strengthened; INJ-011..017 added: tool abuse, exfiltration, base64 smuggling, jailbreak), Go/CUE mirrors in lockstep, 12 FP-resistance cases, fuzz target.
- replay cassettes scrub via DLP by default; anthropic + http-embedder never put API keys in error strings (pinned by tests).

### Post-review hardening

Follow-ups from the wave-3 adversarial review, landed as one hardening pass:

- `kits/endpoint-guard`: judge calls now charge the budget guard; proof-of-work is verified before the topic gate (no free judge spend for unproven callers); `HTTPHandler` caps request-body size.
- `foundations/llmclient/anthropic`: refuses cross-origin redirects, bounds all response-body reads, and surfaces transport failures as typed errors.
- `foundations/llmclient/replay`: synthesizes the terminal stream chunk (StopReason/Usage) when a cassette lacks one.
- `foundations/llmclient` MockClient: enforces its configured request limit.
- Doc truth fixes: package counts reconciled (source-counted 148 vs host `go list` 147 — the js/wasm-only `examples/governance-wasm`), stale CLAUDE.md layout lines, clock-injection doctrine scoped honestly (foundations require injected clocks; kits may default nil Now to time.Now, documented), boundary notes between the two JSONL appenders (`telemetry/sink` emission vs `persistence/jsonl-log` durable append-log).

Investigation-grade foundation library. Pure stdlib, CUE-first, `-race` clean.

- The original 20 foundation categories (contracts, llmclient v1, review,
  security, evidence, …, topology) plus kits (investigation, command-center,
  governed-prose-kit, endpoint-guard), runtimes, surfaces, toolchains,
  scenarios — all 7 reconstructed reference files ported.
- Evidence spine: evidence-grade, evidence-span (bitemporal + provenance),
  record-fingerprint, entity-resolution, entity-crosswalk (rebuilt), conflict,
  assertion, authenticity.
- Provenance: ai-provenance, event-store (hash-chained system-of-record),
  anchor (Merkle proof-of-existence).
- Investigation: coverage, element-coverage, narrative-shift, lead; journal /
  hypothesis / convergence fixes.
- Review: claim-lifecycle rebuilt event-sourced; correction-ledger clock +
  race + provenance fixes. Knowledge: authority (citation/treatment graph +
  good-law). Reasoning: defeasible effective-state engine.
- Security: tokenization (vault port + RBAC + audit).
- Governance (WASM-safe): rate-limiter, circuit-breaker, budget-guard,
  strike-ledger, proof-of-work, identity, topic-gate, answer-cache.
