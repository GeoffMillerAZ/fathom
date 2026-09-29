# agentic-kit — Implementation Tracking

Single source of truth for **what is built, what is tested, and what still
remains**. Every feature, every validation layer, every test gets a checkbox.

Legend: `[ ]` not started · `[~]` in progress · `[x]` done & tested · `[-]` deferred (out of current scope)

Each foundation sub-package, when implemented, must satisfy the **package shape**
(from `foundations-overview.md`):

```
schema.cue    — CUE contract (types, constraints, valid value space)   [skip for behavioral-only pkgs, e.g. llmclient]
types.go      — Go structs/interfaces mirroring the CUE contract
<impl>.go     — implementation (when logic lives in-foundation)
mock.go       — test double (where an interface exists)
*_test.go     — contract + behavior tests; testdata/*.cue for CUE examples
```

> **Generalization rule:** no org-specific names or hard-coded catalogs.
> Anything org-specific is configurable via CUE/config.

---

## Reference jump-off files (most complex components)

The 7 reconstructed Go files from the original scan are the richest design source
for the hardest components. Each is parked in a `_reference/` dir **next to the
package it informs** (Go ignores `_`-prefixed dirs, so they never affect the
build) with a provenance README. Port structure/logic from them and fill
`// [scan gap]` ranges from `tmp/design_inspiration/files/`. **Do not import `_reference/`.**

| Reference file | Informs | Status |
|----------------|---------|--------|
| `cueform-compose.go`, `cueform-response.go` | `surfaces/forms/cueform` — `surfaces/forms/_reference/` | ✅ ported (40 tests) |
| `runtime-bridges.go` | `runtimes/bridges/` — `runtimes/_reference/` | ✅ ported (40 tests) — package retired Wave 4 |
| `runtime-security.go` | `runtimes/security/` — `runtimes/security/_reference/` | ✅ ported (34 tests) — package retired Wave 4 |
| `workspace-query-builder.go` | `toolchains/materialization/workspace-query-builder/` — `…/_reference/` | ✅ ported (18 tests) |
| `workflow-engine.go` | `foundations/orchestration/workflow-engine/` — `…/_reference/` | ✅ ported (engine implemented from it) |
| `scoring.go` | `foundations/review/scoring/` — `…/_reference/` | ✅ ported (helpers implemented from it) |

(`go.mod` in the scan dir is just a throwaway module marker that keeps the parent
build from compiling the partial reference files — not a component.)

---

## 0. Repo scaffold & validation layers

- [~] **Operational evidence and selection (0013)** — existing eval-runner gains bounded
  reuse/change proposals with complete dependency bindings and no execution/sample
  inflation; prompt screening adds opt-in, fingerprinted provider structured output
  while retaining strict local checks. Historical derivatives remain available alongside
  required rerun/fresh-live proposals. Normal consumer collection, policy reconstruction
  and actual offline derivatives are delivered with clean-checkout proof in the
  [completion ledger](docs/plans/2026-09-operational-evidence.md). The operational
  implementation is complete; this row stays partial because fleet-wide/main gates
  retain unrelated non-passes (I4/I21). Further paid study is an explicit owner-directed
  TODO under 0014, not a passing empirical check or stability promotion.

- [x] **Engineering assessments and prompt screening (0012)** — extend the existing fitness registry and guardrails kit with evidence-linked required/scored profiles, explicit unknown/N/A, configurable refusal standards and a gated classifier continuation; [reference](examples/engineering-assessments/README.md) tests real counter dispatch and a disabled-control negative. Full `task validate` and focused race/CUE tests pass; [verification](docs/evidence/engineering-assessments/2026-09-16/README.md) records passing consumers and known downstream baseline failures. Same bounded incubation as 0011, not live model-quality evidence.

Governed workflow implementation: [GW-01–14](docs/plans/2026-09-governed-workflows.md)
records static wiring separately from executed gates. GW-08 adds provider identity,
Responses terminal-authoritative tools and first-class Anthropic effort; conformance
is synthetic. [Provider migration](docs/upgrades/governed-providers.md).

- [~] **Intent scope and distribution (0007)** — platform-engineering adopter;
  affected-path routing, inherited authority and offline pinned practice/skill updater.
  Explicit script suite: `go test -race ./.agents/skills/intent-init/scripts`;
  root `./...` does not discover hidden skill directories. Script tests/vet and adopter
  byte/link checks pass; full gate hit concurrent writes in `.claude/worktrees/gw-mcp`
  during its format sweep (2026-09-13); no full-gate completion claim.
- [x] `git` initialized
- [x] `go.mod` (`github.com/geoffmilleraz/agentic-kit`, go 1.26)
- [x] `go.work` workspace
- [x] `.gitignore`
- [x] `Taskfile.yml` — `fmt`, `fmt-check`, `vet`, `cue-vet`, `test`, `validate`, `doctor`
- [x] `lefthook.yml` — pre-commit (go-fmt-check, cue-vet) + pre-push (validate)
- [x] `lefthook install` wired to git hooks (`.git/hooks/pre-commit`, `pre-push`)
- [x] CUE module root (`cue.mod/module.cue`, `github.com/geoffmilleraz/agentic-kit`)
- [x] `toolchains/schemas/` canonical CUE schema collection (`#Archetype`/`#NodeKind`)
- [x] CI — `.github/workflows/ci.yml` (full `task validate` with cue installed + wasip1/js wasm builds)
- [x] `CLAUDE.md` — contributor/agent guide (gate, hard rules, package shape, layering)
- [x] Taskfile growth — `wasm-build` · `smoke-live` (skips without `ANTHROPIC_API_KEY`) · `sextant-check` (first downstream consumer) · `fuzz` (seed corpora of every fuzz target) / `fuzz-long` (coverage-guided, one target) · `docs-index` (regenerates `docs/catalog.md` via `cmd/kit-catalog`)
- [x] `cue-vet` widened — vets **all** `schema.cue` dirs (kits were silently unvetted) **and** `testdata/` fixtures (previously skipped by `cue ./...`); widening exposed + fixed 64 latent fixture import qualifiers and 4 real fixture bugs
- [x] **H8 Stage A (2026-07-16)** — `cmd/constitution-lint` 6 → 11 checks: `compat-coverage` (docs/compat.md classifies all 169 library packages), `doc-coverage` (2221 exported identifiers name-first documented; 94 first-run gaps fixed same change-set; `doc-waivers.txt` shipped empty), `org-agnostic` (`org-denylist.txt` + model-ID scan w/ provider-adapter carve-out), `secret-scan` (`secret-patterns.txt`, planted-positive canary `TestSecretScanCanary`, full-history sweep clean), `readme-count` (README count == source count, gate-enforced) · `cmd/release-notes` (`ExtractSection`, `task release-notes -- vX.Y.Z`) · `TestFirstAgentWalkthrough` golden-pins docs/first-agent.md's mock-mode output (fixed real `turns=2`→`turns=3` drift) · `docs/compat.md` (124 stable / 21 experimental / 24 internal-ish after re-audit against throughline HEAD; guardrails-pipeline experimental-with-live-consumer flagged — classification awaits owner sign-off) · `docs/threat-model.md` (VISION belief-6 ceiling, verbatim) · `docs/plans/public-readiness.md` (gated Stage-A claims + 22 bounded-everything re-verifications, 1 honest gap). Stage B (CONTRIBUTING/SECURITY, release.yml, public-files lint, pre-flip audit) BLOCKED on D4 ratification — untouched.
- [x] `README.md` (platform overview)
- [x] `VISION.md` — owner's vision statement: beliefs, what the kit refuses to be, the junk gate, practice-watch; the arbiter for scope decisions
- [x] `docs/practices.md` — agentic anti-pattern → kit-package mapping, honest about shipped vs partial vs roadmap

### Validation gate (must stay green)

- [x] **Benchmark evidence (0011), bounded first slice** — manifests, terminal trial receipts,
  failure-inclusive summaries and task-clustered paired comparisons compose existing
  eval-runner/fingerprint/statistics primitives. CUE projection and negative tests;
  synthetic real-consumer overlay, not production adoption or quality evidence.
  `task validate`, `task eval`, race/CUE projections and receipt reconstruction pass;
  [verification](docs/evidence/benchmarks/2026-09-16/README.md) records downstream limits.

- [x] `task fmt-check` passes
- [x] `task vet` passes
- [x] `task cue-vet` passes (all schema packages validate)
- [x] `task test` passes (`go test -race ./...`; last full verified run: uplift wave 3, `ad9518c`)
- [x] `task validate` (full gate) passes — exit 0
- [x] pre-commit hook runs the fast checks (verified: go-fmt-check ✔, cue-vet ✔)
- [x] pre-push hook runs `task validate` (wired; runs once repo has ≥1 commit)

---
- [x] **field-absorption FB-9 (2026-09-02)** — `cmd/constitution-lint` 11 → 12 checks: `read-coverage`. Every continue-on-error walk site (`walkWithManifest` behind `walkSweepFiles`, `libraryPkgDirs`, `hasGoFiles`, `packageDocComment`, `checkSchemaPresence`) records into a `foundations/telemetry/read-manifest` `Set` without changing what it skips; an unreadable file or directory is one FAIL finding each, and the summary line renders the finding count as a lower bound ("findings checked: ≥ N (k unreadable: …)") whenever the manifest is incomplete — an unreadable tree can no longer look healthy. `TestUnreadableTreeNeverPassesConstitution` (chmod 000 a file and a directory; skipped as root) · `TestWalkWithManifestRecordsOutcomes`

## 1. Foundations (28 categories)

> **Phase-1 vertical slice** (deep, end-to-end, tested) = `contracts`,
> `llmclient`, `review/correction-ledger`, `security/injection-detector`,
> `evidence/evidence-graph`. Remaining categories are scaffolded breadth-first
> afterward.

### 1.1 contracts  — canonical CUE-first types  ⭐slice ✅
- [x] `event-shapes` (`#EventShape`) — schema.cue · types.go · Validate · tests
- [x] `node-types` (`#NodeType` + enum) — schema.cue · types.go · Validate · tests
- [x] `answer-archetypes` (`#AnswerArchetype`) — schema.cue · types.go · Validate · tests
- [x] `knowledge-card-archetypes` (`#KnowledgeCardArchetype`, `#FieldSpec`) — schema.cue · types.go · Validate · tests
- [x] JSON round-trip + CUE positive fixture + negative cases (Go-asserted)

### 1.2 llmclient — single LLM interface + ladder  ⭐slice ✅ (behavioral, no schema.cue)
- [x] `LLMClient` interface (`Complete`, `Stream`) — types.go
- [x] `Ladder` / `Rung` / `Signal` (ordered tiers, `Climb` escalates only on signal, stops at top) — ladder.go
- [x] `RetryConfig` + `WithRetry` (exp backoff + injectable/seedable jitter, retryable vs non-retryable classification, `Attempts` surfaced) — retry.go
- [x] typed errors (`RateLimitError`/`ServerError` retryable; `InvalidRequestError`/`AuthError` fail-fast) — errors.go
- [x] deterministic `MockClient` — mock.go + 19 tests
- [x] **v2 agentic contract** — multi-turn `Message`/`ContentBlock` (tool_use / tool_result / unknown passthrough), `ToolDef` + `ToolChoice`, open `StopReason` constants, block-level prompt-cache hints, cache-token `Usage`; `Stream` redesigned to chunk channel with terminal StopReason/Usage/Err (**BREAKING**, see CHANGELOG); retry honors Retry-After, never retries after tool_use; `MockClient` tool_use scripting (source-compatible) — tests
- [x] **image content blocks** — `BlockImage` type constant, `ContentBlock.MediaType`/`Data` fields, `ImageBlock(mediaType, data)` constructor; `MockClient` passes image blocks verbatim; conformance suite updated — 35 tests (llmclient_test.go; `19a63b4`)
- [x] `anthropic` — first REAL provider: stdlib-only Messages API client (Complete + SSE streaming, tool use, prompt-cache `cache_control` mapping, unknown-block verbatim round-trip, typed errors incl. 429 Retry-After + 529, key-never-in-errors test) — schema.cue · 24 tests
- [x] `replay` — record/replay cassettes (JSONL via jsonl-log; DLP scrubbing ON by default; ordinal matching default, canonical-JSON hash opt-in; stream chunk segmentation preserved) — 15 tests
- [x] `replay` **canonical request-hash v2** — new entries record `hash_version:2`; v2 drops zero-value fields (empty strings, zero ints, nil/empty slices) before hashing so additive contract growth never invalidates cassettes; old cassettes remain readable (v2 hashing of decoded v1 entries); `task smoke-live` re-recorded; `HashVersion2` constant; `RequestHash()` helper (**BREAKING** for cassette format — see CHANGELOG)
- [x] `replay` **audit re-verified for `foundations/llmclient/structured`** (2026-07-29, prompted by fathom's `docs/plans/gke-expertchat.md` GKE-EC-5 citing this package by name): confirmed a first-class record/replay lane already existed (not a hand-rolled mock convention) — `docs/compat.md` already classifies it `stable`, `TestReplayClientConformance` already proves record→replay fidelity end to end. The one gap: nothing exercised the `structured.Complete` seam specifically, which the fathom plan names. Closed with `TestStructuredCompleteRoundTripsThroughFileCassette` (`structured_roundtrip_test.go`): records a `structured.Complete` call to a real on-disk JSONL cassette via `RecordingClient`, reloads it with `LoadFile`, and proves the replayed `structured.Complete` call is byte-identical (`Response` and the strictly-decoded target struct) with no network on either side, plus the expected loud error when replaying past the cassette's end.
- [x] `instrument` — `WithTelemetry`/`WithCost` middleware: one `llm_call` event per logical call, trace correlation, cache-aware cost, attempts surfaced — 18 tests; **Wave 2**: `WithBudget` middleware (pre-flight estimate → `ChargeTracked` → `Settle` to actual; per-logical-call granularity outside `WithRetry`; stream settles at terminal; typed `BudgetExceededError` non-retryable) + `ErrBudgetExceeded` sentinel (`errors.Is` compat via Unwrap) — 27 tests total; throughline's hand-rolled `budgetClient` (~62 lines) deleted, F3+F4 closed this wave
- [x] `llm-test` — `RunConformance` suite every client must pass (anthropic + replay pass it; replay fed from a MockClient-recorded cassette) — conformance.go
- [x] anthropic **bearer/OAuth auth** (`Config.AuthScheme`, `BetaHeaders` e.g. `oauth-2025-04-20`; no-leak tests cover both schemes) + zero-token `Ping` credential probe (GET /v1/models, typed error mapping) — proven by throughline's claudeauth pattern — tests
- [x] `Swappable` late-bound client (atomic hot-swap, `Available()`, typed non-retryable `UnavailableError`; passes conformance wrapping MockClient) — swap.go · tests
- [x] `egress` — gate-before-egress middleware (`WithEgress`: fail-closed `Transformer` over every outbound text field, deep-copy, once-per-logical-call under retry; `GateFromVault` tokenization + `GateFromDLP` adapters) — generalized from throughline's GatedClient — 14 tests
- [x] `egress` **inbound detokenization** (`WithInboundDetokenize`, `DetokenizeText`) — restores vaulted tokens in Complete responses and streamed chunks; pairs with `WithEgress` to form a full tokenize-on-egress / restore-on-ingress round-trip; best-effort (unresolvable tokens left in place); 11 tests (`78fb735`)
- [x] `failover` — ordered-lane failover wrapper (`WithFailover`, `FailoverDecision` accessor); eligible errors: `AuthError`, `RateLimitError`, `UnavailableError`; sticky cooldown (configurable per lane); zero-delta stream failover; passes conformance wrapping two MockClients — 15 tests (`c1ced16`)
- [x] `bedrock` — AWS Bedrock provider: stdlib-only SigV4 request signing (`sigv4.go`), eventstream framing (`eventstream.go`), streaming + complete (`stream.go`), typed error mapping, config with region + credential injection; passes `RunConformance` (conformance_test.go); design re-derived from reconstructed Update-2 scan — schema.cue · 28 tests
  - [x] `awscred` (`foundations/llmclient/bedrock/awscred`, 2026-07-16, docs/plans/aws-credchain.md) — the AWS default credential-resolution chain the adapter was missing: explicit Config → AWS_* env vars → shared config/credentials files (static keys, `credential_process` via stdlib argv-tokenized `exec.CommandContext`, no shell) → SSO (`aws sso login`'s modern `sso_session` form and the legacy inline form; token-cache lookup tries the session-name SHA-1 first, falling back to the start-URL SHA-1; an absent/expired token or a portal 401/403 returns a typed `*ExpiredSSOTokenError` naming `aws sso login --profile <p>`) → IMDSv2 last (token PUT + role-creds GET, ~1s timeout, honors `AWS_EC2_METADATA_DISABLED`). Region resolves independently (`AWS_REGION`→`AWS_DEFAULT_REGION`→profile `region` key). No aws-sdk-go dependency — every leg hand-rolled on stdlib, hermetically tested (temp HOME, fixture ini files, a fake `credential_process` script, `httptest` servers standing in for the SSO portal and IMDS; zero live AWS calls). `bedrock.New`/`Client` now consume the resolver via a mutex-guarded, injected-clock `credentialCache`: temporary credentials (SSO/process/IMDS) re-resolve within a 5-minute expiry margin instead of being frozen at construction, surfacing a refresh failure as a typed non-retryable `*llmclient.AuthError`; existing explicit-Config behavior is unchanged (byte-identical values; a `TestMain` hermetic-HOME baseline was added to `client_test.go` so the fuller chain never leaks a test host's real `~/.aws`). One named consumer (`bedrock`) by design (D8) — 38 tests (awscred) + 8 new bedrock-side tests
  - [x] Region/BaseURL mismatch guard (KP-2, docs/plans/claudecli-ping.md, 2026-07-17) — `New` now rejects an explicitly-set `Config.BaseURL` whose host does not contain an explicitly-set `Config.Region` (catches a GovCloud region paired with a commercial BaseURL, or the reverse) with a typed `*RegionBaseURLMismatchError` naming both offending values, before any credential resolution or HTTP call. Region-only and BaseURL-only configs are untouched — the guard keys off the raw `Config` fields, not the resolved region, so a `Config.Region`-empty caller relying on `AWS_REGION` is never checked. No partition table: a plain substring test, with a single generic exemption (`isLoopbackHost`, `net.ParseIP`-based) for loopback hosts so hermetic `httptest`-backed tests (this package's own `newTestClient`, used by 47 existing tests) are unaffected — 6 new subtests (`TestNew_RegionBaseURLMismatch`)
- [x] `noop` — `llmnoop.Client` always returns the canned refusal `Message` with `StopReason: StopRefusal` (the `"end_turn"` this row used to state was wrong) and zero `Usage`; the air-gap fallback when no real client is reachable. Conformance (FB-3, 2026-09-02): `TestNoopConformance` runs the seven core checks of `llmtest.RunConformance` in a re-executed test binary and pins the verdicts — 3 pass (AsMessageRoundTrip, StopReasonPopulated, StreamContextCancellation), 4 fail by design (CompleteTextMatchesBlocks and StreamLifecycle: the text is always `Message`, never the requested text; UsageOnComplete and UsageOnStream: zero tokens are consumed). It does NOT pass the suite; the "passes conformance" this row used to claim was an overclaim — 12 tests
- [x] `openai` — (D9, roadmap #10) cross-family judge lane: full chat-completions adapter (tools, streaming with per-index `tool_call` accumulation, images as data URIs, `OutputSchema`→`response_format json_schema`, `Effort`→`reasoning_effort`, `Thinking` typed-rejected, `ProviderParams` merge, typed errors + `Retry-After` both forms, `Ping`/`Pinger`; key-never-in-errors); through fattened `llm-test` conformance (18 subtests) + 14 wire tests. Roadmap #10 graduated.
- [x] `openai` Azure OpenAI support (docs/plans/azure-openai.md AZ-1..5, 2026-07-16) — extends the existing chat-completions adapter in place rather than forking it (wire mapping, streaming, and typed errors are 100% reused): `Config.AzureDeployment` switches endpoint construction (`buildEndpoint`) to `{BaseURL}/openai/deployments/{deployment}/chat/completions` with a mandatory `api-version` query parameter on every request — chat completions, streaming, and the Models `Ping` (which becomes `{BaseURL}/openai/models`) alike; `Config.AzureAPIVersion` overrides the pinned GA default (`DefaultAzureAPIVersion`, doc-commented pinning doctrine — expected to lag Azure's own latest by design, not neglect). Auth (`setAuthHeaders`): `api-key` header by default, or `Authorization: Bearer` when `Config.AzureAuthScheme` is `AzureAuthSchemeBearer`, for Entra-ID token passthrough — the kit mints/refreshes nothing (token acquisition deferred, AZ-6). Azure's OpenAI-shaped error envelope verified fixture-by-fixture (400/401/429/500/503) through the Azure path rather than assumed identical; key redaction (`redact`) proven to cover both the api-key and bearer header schemes. New Config fields default to zero-impact on vanilla construction — every pre-existing test in the package passes unmodified, plus a real-SSE streaming test through the Azure path — `azure_test.go`.
- [x] `structured` (`foundations/llmclient/structured`) — **fathom-asks FA-2/FA-3 (2026-07-16)**: typed strict-JSON completion over the `OutputSchema` seam — `ErrTruncated` (StopMaxTokens = budget error, not parse error) vs `ErrMalformed` (strict decode, unknown fields = schema drift); Response returned in every case for cost accounting; callers own skip/abort policy. First subscriber: fathom obligations compile (its errBadResponse hand-classification retires). Behavioral-only (no schema.cue per package shape) — 4 test funcs
- [x] `claudecli` (`foundations/llmclient/claudecli`, 2026-07-16) — Claude Code CLI adapter: shells out to `claude -p --output-format json --safe-mode --tools ""` so completions ride a Claude Pro/Max subscription's OAuth login instead of a metered `ANTHROPIC_API_KEY`; built on `foundations/agent/bounded-exec` for subprocess timeout/output bounding and process-tree cleanup rather than a second hand-rolled `exec.CommandContext`. `--safe-mode` (not `--bare`, which would force API-key-only auth) verified to fully suppress CLAUDE.md/MCP ambient-context leakage; `--json-schema` verified to genuinely enforce `Request.OutputSchema` (with its internal structured-output tool call's `stop_reason:"tool_use"` remapped to `StopEndTurn`); `--effort` honors `Request.Effort` (unlike anthropic/bedrock, which reject it). Unsupported fields (`Tools`, `Thinking`, `Temperature`, `StopSequences`, `ProviderParams`, non-text `Messages` blocks) fail loud with a typed `InvalidRequestError` rather than being silently dropped; `MaxTokens` is silently ignored (no CLI flag exists). Hermetic fake-shim tests only (no network, no real credentials touched). First subscriber: fathom's semantic-layer compile provider seam. Behavioral-only (no schema.cue per package shape) — 21 test funcs
  - [x] `Ping`/`llmclient.Pinger` (KP-1, docs/plans/claudecli-ping.md, 2026-07-17, `foundations/llmclient/claudecli/testdata/PROBES.md`) — a LOCAL, pre-API, zero-token liveness probe: runs the exact pinned production argv for a zero-valued `Request` with EMPTY stdin and no positional prompt argument, which the installed CLI (2.1.212) rejects locally with a stable stderr signature (`"Input must be provided either through stdin or as a prompt argument..."`, exit 1, empty stdout) before resolving a model or touching the network — live-verified across six probes (including a bogus-`--model` probe proving the input-presence check runs BEFORE model validation, and one real "reply pong" completion for cost/timing contrast: 5.6s wall + a real JSON envelope + `total_cost_usd`, versus Ping's 1-3s local rejection with empty stdout). Any other outcome — the CLI proceeding toward a call, a different local failure, or hitting Ping's own 15s bound — returns a typed `InvalidRequestError`. No undocumented-config-key dependence to tripwire (unlike codexcli's CX-10): this Ping proves the pinned argv's local input-gate is intact, not auth/model/quota. Hermetic shim fixtures for healthy/degraded/timeout/cancel shapes — 4 new test funcs
  - [x] Conformance (FB-1, 2026-09-02) — `TestClaudeCLIConformance` runs the seven core checks of `llmtest.RunConformance` against the hermetic fake shim with every optional `Capabilities` field false and named in a `t.Log` line alongside the adapter fact that keeps it off (no caller tools, no mid-stream failure mode, no provider-native blocks, no Retry-After source, no wire body, no cache-hint enforcement) — 22 test funcs now
- [x] `codexcli` (`foundations/llmclient/codexcli`, 2026-07-16, docs/plans/codexcli-adapter.md CX-1..CX-7) — OpenAI Codex CLI adapter, modeled on claudecli: shells out to `codex exec --json -C <scratch-dir> --skip-git-repo-check --ignore-user-config --ephemeral -s read-only -c model_instructions_file="<file>" -c include_environment_context=false -c include_apps_instructions=false -c include_collaboration_mode_instructions=false -c include_permissions_instructions=false [-m <model>] [-c model_reasoning_effort="<effort>"] [--output-schema <file>]` so completions ride a ChatGPT Plus/Pro subscription's OAuth login instead of a metered `OPENAI_API_KEY`; built on `foundations/agent/bounded-exec`. Live-probed (not assumed) ambient-context floor: 9,765 input tokens, a 27.4% cut from the 13,443-token unoptimized baseline (testdata/PROBES.md, 12 live probes) — the dominant lever is `-c model_instructions_file`, an undocumented (binary-strings-discovered) config key that both maps `Request.System` (REPLACES, not appends, the CLI's built-in persona — verified live via a "reply BANANA" override) and, incidentally, is the single biggest ambient-context-suppression lever found, bigger than every other flag combined. A broad `--disable <feature>` combo measured a lower floor (11,456) but was deliberately excluded from production: verified live that an unrecognised `--disable` value hard-errors (`Error: Unknown feature flag: ...`) while an unrecognised `-c key=value` is silently tolerated — a dozen hardcoded feature names would risk breaking every future call on the next codex-cli upgrade. `model_reasoning_effort` honors `Request.Effort` (real, verified live, unlike anthropic/bedrock). `--output-schema` genuinely enforces `Request.OutputSchema` (verified live). Real degradation vs claudecli: NO `stop_reason` field exists anywhere in codex's JSONL event schema, so Response.StopReason is always `StopEndTurn` and truncation is undetectable through this adapter. Because the undocumented-key dependence would degrade SILENTLY on a CLI upgrade (unrecognised `-c` keys exit 0 — verified; System silently lost, ~13k-token built-in persona silently returns), `Config` carries a version-pinning WARNING and the package implements `llmclient.Pinger` as a tripwire (CX-10): `Ping` runs the pinned argv with a deliberately EMPTY `model_instructions_file`; the CLI's hard LOCAL rejection of it (sub-second, pre-stdin, zero tokens — live-verified, probe 13) is the healthy signal, anything else is a typed `InvalidRequestError` naming the degradation. Unsupported fields (`Tools`, `Thinking`, `Temperature`, `StopSequences`, `ProviderParams`, non-text `Messages` blocks) fail loud with a typed `InvalidRequestError`; `MaxTokens` is silently ignored (no CLI flag exists). `turn.failed`/error mapping: the one live-triggered failure (bogus `--model`) is pinned by status code (400 -> `InvalidRequestError`, mirroring claudecli's `api_error_status`); auth-expiry/usage-limit are deliberately unprobed live and mapped speculatively by message-keyword content (full-word auth tokens, never the bare "auth" substring), documented as such. Hermetic fake-shim tests only (no network, no real credentials touched) — 34 test funcs incl. concurrent-Complete under `-race`, a >64KiB event-line pin, failure-wins partial-text discard, and healthy/degraded/timeout Ping shapes. CX-8 (incremental JSONL streaming) and CX-9 (session resume) deferred per the plan. First subscriber: fathom's semantic-layer compile provider seam. Behavioral-only (no schema.cue per package shape)
  - [x] Conformance (FB-2, 2026-09-02) — `TestCodexCLIConformance` runs the seven core checks of `llmtest.RunConformance` against the hermetic fake shim with every optional `Capabilities` field false and named in a `t.Log` line alongside the adapter fact that keeps it off (no caller tools, no mid-stream failure mode, no provider-native blocks, no Retry-After source, no wire body, no cache-hint enforcement) — 35 test funcs now
- [x] **Wave 1 — Provider & protocol currency** (v0.3.0):
  - [x] `Request.ProviderParams json.RawMessage` extension seam — verbatim top-level wire merge AFTER adapter mapping (override precedence documented); both adapters + mock validate object-ness; no typed field can be silently shadowed
  - [x] `Request.Thinking *ThinkingConfig` + `Request.Effort` — first-class thinking/effort controls; anthropic + bedrock map `thinking` wire object; `Effort` typed-rejected by both adapters (no stable API knob; ProviderParams is the escape hatch); conformance asserts mapped-or-rejected-never-dropped; Temperature-on-frontier hazard documented in doc.go
  - [x] `Config.CapabilityCheck func(Request) error` seam on both adapters — model-catalog integration point for Wave-3 judge independence; nil = permissive
  - [x] `Pinger` interface + `ProbeAll` — concurrent liveness probes across all lanes; typed error per lane; absent-means-healthy contract; bedrock gains `Ping` (signed GET /foundation-models on control-plane host; proves credentials+IAM list permission+reachability, NOT invoke entitlement — documented) — probe.go · 10 tests (`probe_test.go`)
  - [x] `Request.OutputSchema` → structured outputs end-to-end — maps to `output_config.format {type:"json_schema", schema}` on both adapters; `ToolDef.Strict`; closes roadmap #16 via `toolchains/jsonschema` (see §5)
  - [x] Raw preserved on ALL inbound content blocks (not just unknown) — round-trip fidelity for citations/server-tool payloads; precedence rule: non-empty Raw wins outbound; callers who edit received blocks must clear Raw; `ToolDef.Raw` verbatim tool passthrough (server tools reachable)
  - [x] Streamed unknown-block delta accumulation (roadmap #12 retired) — stream-assembled blocks equal Complete-assembled, key order preserved; bedrock eventstream parity
  - [x] `llm-test` conformance fattened (~24 new cases) — unknown-block round-trip both paths + stream/complete equivalence, typed-error taxonomy incl. Retry-After seconds+HTTP-date, usage-on-complete/stream, cache-hint limits behind PreflightValidation capability, stream tool_use terminal, all Wave-1 surfaces; suite caught + fixed: MockClient zero-usage responses now default to 1 token each (behavior note) — 39 tests total (`llmclient_test.go`)
  - [x] `model-catalog` (`foundations/llmclient/model-catalog`, package `modelcatalog`) — CUE `#ModelCatalog`/`#ModelEntry` (family/lineage/caps/cost_tier/deprecated), `Load`/`Entry`/`Validate` + `ValidateIndependence` (judge/producer family+lineage separation; unknown-model = violation); example-only testdata (no authoritative rows shipped — cost-tracker PriceTable precedent); design re-derived from reconstructed Update-2 scan — schema.cue · types.go · 20 tests (`catalog_test.go`)
  - [x] `kits/mcp-server` negotiates protocol version 2025-11-25 (latest); design note `docs/designs/mcp-streamable-http.md` (Streamable HTTP + W3C trace context, deferred with triggers)
  - [x] `foundations/agent/mcp-client` (package `mcpclient`) — MCP consuming side: `Transport` seam + `IOTransport`, initialize/tools-list/tools-call typed; tool-description SHA-256 hash-pinning with `ToolChangedError` + explicit `Repin` (tool-poisoning defense); `RegisterAll` bridge into tool-registry (all governance applies to foreign tools); subprocess spawning is caller's job via `bounded-exec` (documented) — 19 tests (`mcpclient_test.go`); **FB-17 (2026-09)**: `effectFromAnnotations` maps tools/list annotations onto `toolregistry.Effect` with the MCP defaults (absent `destructiveHint` = true; malformed = undeclared, never permissive), `ToolInfo.{Annotations, Effect, EffectTrusted}`, `Options.TrustedServer` (annotations are UNTRUSTED unless set — `RegisterAll` attaches the server's effect only then; `BridgeOptions.EffectMap` is the operator's override), annotations join the pinned hash (a flipped hint trips `ToolChangedError`; tools without annotations hash as before) — 6 new tests
    - **H6 (2026-07-15)**: HTTP transport (`NewHTTPTransport` + `Doer` seam, typed `HTTPStatusError`/`ErrResponseTooLarge`, SSE fail-closed, oversize-bounded) + client-side traceparent injection; existing `Client` unchanged over HTTP; e2e canary `TestStreamableHTTPTraceRoundTrip` (kits/mcp-server) proves client→server trace propagation with fresh child span
    - **FB-18 (2026-09-02, decision 0003)**: `ProtocolRevision` ("2025-11-25") + `Deprecations()`/`DeprecationsDue(now)` — the 2026-07-28 retirement clock (initialize handshake still implemented; sessions/ping/roots/sampling/logging/HTTP+SSE never built); `TestDeprecationFloorNotReached` reads the wall clock ON PURPOSE and goes red on 2027-07-28 — 6 tests (`revision_test.go`)

### 1.3 review — finding lifecycle & correction  ⭐slice (correction-ledger ✅)
- [x] `correction-ledger` — Finding FSM (open→reviewing→{corrected,dismissed,deferred}, reopen; forward-terminal), distinct `Correction`/`Dismissal`, append-only `Ledger` + deterministic `Replay` — schema.cue · types.go · finding_fsm.go · ledger.go · 11 tests
- [x] `correction-ledger-analytical` — versioned AnalyticalEvent projection — schema.cue · 12 tests
- [x] `concern-flags` (severity enum, Raise/ByTarget) — schema.cue · tests
- [x] `claim-lifecycle` (proposed→supported/refuted/uncertain, guarded) — schema.cue · tests
- [x] `review-queue` (priority+insertion-order, deterministic) — tests
- [x] `confidence-scoring` (1–5 derivation from evidence) — schema.cue · tests
- [x] `run-audit-record` (append-only audit) — tests
- [x] `scoring` (Normalize/DeriveDisposition/coverage, ported from reference) — tests
- [x] scoring `Tier` constants — exported `Tier` alias + `Tier*` constants (consumer-driven); rank-vs-ceiling asymmetry documented as intentional + pinned by test — 19 tests
- [x] `run-report` — (roadmap #20) `Render(Input)`→deterministic markdown (summary/activity/timeline/flags/cost); "not captured" degradation; golden + harness-integration tested — types.go · tests
- [x] `intake-triage` — (D35-1, kit-side done) human-only FSM: search→proposal→(imported|excluded(reason)|deferred); reversible exclusion; AI-suggest-only; throughline swap completed H2 Wave B (2026-07-15) — donor deleted, uuid/ledger stay consumer-side, kit core proven-shape — schema.cue · types.go · tests; **H2 Wave A (2026-07-15)**: `ResumeFSM` hydration at any persisted state incl. terminals (history-since-resume semantics documented) — +5 tests, unblocks the Wave-B donor swap

- [x] `capped-confidence` (`foundations/review/capped-confidence`, package `cappedconfidence`) — **fourth-consumer uplift RC-2 (2026-09-09, racli ADR-021 §5b)**: a score bounded by the ceiling its evidence class permits, from a table the caller loads out of CUE (`#Table`: `max` + one `#Ceiling` per class with a mandatory `why`; classes are data, never Go constants; `Config.Validate` re-checks every constraint, `NewTable` refuses an invalid one). `Table.Cap` clamps DOWN only and keeps `Raw` verbatim; `Score.State` is supported | capped | unscored and `Render` prints a supported score as is, a capped one as a BAND ("≤50 — delimited-line evidence", never the point value) and an unscored one as "unscored — <reason>" (nil table, undeclared class, NaN raw, the zero value — a check that cannot run never looks clean). Lands beside `review/scoring.ApplyCeiling` (fixed five-tier ints in Go, silent pass-through), not in place of it; fathom's `internal/memory/tier.go` works around exactly that gap by hand and is the named migration candidate. Generalizes racli's `cmd/metrics/sources.go` (`Capped`) + `resume/schema/sources.cue` (`#SourceClass`, `sourceCeilings`); one consumer, so `experimental` (D13) — schema.cue · testdata · 9 test funcs

### 1.4 security — defense-in-depth  ⭐slice (injection-detector ✅)
- [x] `injection-detector` — data-driven rules, `Scan`, severity ranking — schema.cue · 8 tests
- [x] `pre-llm-dlp-gate` — configurable scan+redact before LLM — schema.cue · tests
- [x] `sensitivity-classifier` — org-configurable CUE catalog, highest-match — schema.cue · tests
- [x] `data-sensitivity-types` — level enum + classification — schema.cue · tests
- [x] `instruction-hierarchy` — system>developer>user>untrusted precedence — schema.cue · tests
- [x] `output-integrity-checker` — required disclaimers / banned tokens — schema.cue · tests
- [x] `redaction-executor` / `redaction-policy` (data-driven rules, idempotent) — schema.cue · tests
- [x] `untrusted-content-wrapper` — delimit untrusted content — schema.cue · tests
- [x] `tokenization` — pure-logic sensitive-data tokenization + vault (RBAC + audit; vault port) — schema.cue · 21 tests; **Wave 3**: `KeyStore`/`MemoryKeyStore` (AES-256-GCM envelope keys), `EncryptedVault` + `ShreddableVault` extension interface (existing `Vault` consumers unchanged), `Shred(namespace)` = crypto-erasure with typed `ErrShredded`, `RunShredConformance` exported suite (6 cases). Roadmap #18 graduated. **H3 Stage 2 (2026-07-16)**: `RunShredConformance`/`VaultFactory` extracted to importable `foundations/security/tokenization/vaulttest` (non-test package, behavioral-only, no `schema.cue`) so downstream adapters (throughline's SQLite-backed vault) can import the suite; `conformance_test.go` is now a thin alias/delegation.
- [x] `token-lifecycle` — pure OAuth credential lifecycle (H2/D35-3, 2026-07-15): `Classify` (valid|refreshable|reauth_needed|not_found), `NextAction` (use|refresh|reauth, 60s default skew), `MergeRefreshed` (preserve-refresh-token rule), `RevocationTarget` (refresh-preferred); donor semantics pinned incl. skew boundary ±1ns; HTTP/PKCE/storage stay consumer-side — schema.cue · testdata · 8 tests
- [x] injection-detector recall uplift — seeded-corpus recall 12/22 → 22/22 (INJ-001/003 strengthened; INJ-011..017 added: tool abuse, exfil, base64 smuggling, jailbreak), Go/CUE rule mirrors in lockstep, 12 FP-resistance cases — recall_test.go · FuzzScan
- [x] redaction-executor fixpoint — redacts to a FIXPOINT (FuzzRedact found a single-pass PII leak via boundary unmasking; true idempotency restored, MaxPasses cap with `Converged` surfaced) — fixpoint_test.go · fuzz_test.go
- [x] pre-llm-dlp-gate hardening — `MustDefaultGate`; `ExtendedPolicy` (high-precision secret shapes only: PEM/AKIA/GitHub/Slack/JWT/AIza); honest best-effort docs (shape redaction, not DLP truth) — FuzzScan · 12 tests
- [x] untrusted-content-wrapper nonce redesign — per-wrap unguessable nonce fences (embedded delimiters can't forge a fence), spoofable `IsWrapped` removed, docs scoped to defense-in-depth (**BREAKING**) — 16 tests
- [x] `rule-core` — the shared compile/`ScanFirst`/`ReplaceFixpoint`/`Ordinal` core under the four regex engines (injection-detector, dlp-evaluator, pre-llm-dlp-gate, redaction-executor); all four engines refactored to delegate; exported APIs unchanged; their tests pass unmodified. Roadmap #19 prerequisite.
- [x] `httpauth` (`foundations/security/httpauth`, 2026-07-29) — minimal bearer-token HTTP middleware: `Bearer(tokenEnvName)` reads the token from a NAMED env var at construction (empty/unset = constructor error, never a silently-open gate), constant-time compare (`crypto/subtle`), 401 + `WWW-Authenticate` on missing/wrong, token never logged/echoed (grep-proven on recorded output). Clean re-implementation of the pattern proven in fathom's module host (`cmd/fathom/servehttp.go` `modulesAuthMiddleware`, GKE-MH-7) — no fathom import; path-scoping stays the caller's job. Behavioral-only (waived in `cmd/constitution-lint/waivers.txt`) — 11 tests incl. a concurrent-request race check. Intended first in-kit consumer: the hosted expertchat service, once it grows a serving mode (`examples/expertchat-service` is currently CLI-only, not wired).

### 1.5 evidence — core data-access layer  ⭐slice (evidence-graph ✅)
- [x] `evidence-graph` — in-memory graph, TTL `Sweep(now)`, `Contradictions(now)` — schema.cue · 11 tests
- [x] `evidence-span` — CANONICAL bitemporal span with full provenance (`Grade`, `Author`, `Locator`, record hashes, `OccurredAt`/`ObservedAt`); `ValidateAsEvidence` enforces "no provenance → not storable as evidence" — schema.cue · tests
- [x] `evidence-resolver` — ref→span resolution, order-preserving — tests
- [x] `source-registry` — sources (markdown/readme/doc/other) — schema.cue · tests
- [x] `source-linker` — span↔source link sets, sorted — tests
- [x] `entity-crosswalk` — alias↔canonical identity map — tests
- [x] `doc-provider` — `DocProvider` interface + Markdown/Readme (in-memory) — tests
- [x] `evidence-grade` — tier taxonomy + claim grading (the #1 truth-seeking primitive) — schema.cue · 28 tests
- [x] `assertion` — speech-act primitive (`Stance` + `Assertion`) — schema.cue · 24 tests
- [x] `conflict` — conflict/contradiction taxonomy over graded assertions + spans (join key: `Assertion.PropositionID == Span.EntityID`) — schema.cue · 10 tests; **Wave 3**: `DetectWithPropositions` three-way join (Assertion.PropositionID==Proposition.ID, Proposition.Subject==Span.EntityID), Field-aware conflict detection, Coverage diagnostics with `Degenerate()` (silent-zero-join now impossible), regression fixture mirroring throughline's disjoint ID scheme. Fixes `[evidence-spine/join]`.
- [x] `authenticity` — typed, provenanced authenticity/integrity assessment — schema.cue · 25 tests; **H2 Wave A (2026-07-15)**: `ScoreCeiling`/`VerdictCeiling`/`ClampToCeiling` per-method guidance (DKIM 0.95 vs Authentication-Results 0.55, asserted-vs-verified rationale in doc) — +5 tests
- [x] `authenticity` **`MethodDKIM` + `MethodAuthResultsHeader`** — two new `Method` constants added (throughline doc-35 same-day extraction, `cf40791`): `MethodDKIM` for independently-verified header signatures; `MethodAuthResultsHeader` for receiver-asserted verdicts (graded lower per adoption guidance)
- [x] `entity-resolution` — entity-match candidates; never auto-merges (human review required) — schema.cue · 26 tests
- [x] `record-fingerprint` — content-hash record dedup (new / unchanged / changed) — schema.cue · 11 tests
- [x] `evidence-anchor` — Merkle proof-of-existence for `evidence-span.Span` values: `CanonicalDigest` (schema-level hash), `AnchorSpans` (batched Merkle anchor via `provenance/anchor`), `Verify` (standalone proof check); injected clock seam; pure functions, safe for concurrent use; design re-derived from reconstructed Update-2 scan — schema.cue · 16 tests
- [x] category READMEs — evidence + provenance + reasoning pipeline docs incl. the two-Span-types warning (`evidence-graph.Span` is the legacy shape; no new pipelines on it) — README.md ×3
- [x] `evidence-signature` — (D8, Update-2) `#ConcernSignature`: per-concern evidence contract; Scope as path globs XOR registered source IDs; canary MANDATORY for absence archetypes; `PreconditionSatisfied`; `LivenessCheck` CI helper; `CoverageManifest` with D10 skip taxonomy + `Complete()` — schema.cue · types.go · tests
- [x] `evidence-ladder` — (D8, D10, Update-2) Tier1/2/3 interfaces; deterministic tier progression; `ABSENT_PROVEN` scoped-at-snapshot with coverage manifest + mandatory canary; fail-closed to `ABSENT_UNVERIFIED` on unexplained skips; Tier-3 timeout→`UNRESOLVED` never absent; `HumanVerdict.Feedback` mandatory; hitl composition documented, not imported — schema.cue · types.go · tests
- [x] `direct-investigator` — (D8, D10, Update-2) pure-Go `WalkDir`+regexp Tier-2 contract; skip accounting IS the guarantee; optional `rg` Engine via bounded-exec explicit-path-only, equivalence-tested; refs as `file://…#Lx-Ly` via `evidencespan.Locator` — tests
- [x] `extraction-gap-log` — (D8, Update-2) `GapRecord`s via `jsonl-log`; `HitRate`/`WarmList`/`Signals` feeding improvement-flywheel — tests
- [x] `case-file` — (roadmap #25) pure deterministic projection over assertions by type (stance buckets), tier-rank weighted support/against scores with documented math, net classification bands; byte-identical golden-tested; throughline UI is the named subscriber — tests
- [x] `evidence-producer` (2026-09-18) — producer catalog for findings/claims crossing into a consuming store: per-producer kind/origin/tier_cap so served tier = min(claimed, cap) (tier never rises on import), `Independent` scopes corroboration to distinct producer IDs, `Admit` refuses `origin: self` (circular re-import). Reuses the existing engineering ladder (`foundations/review/scoring.Tier*`) via `DefaultKindCaps`, does not redefine it. First consumer: fathom's `internal/semantic/producers.go` (ADR-040) — schema.cue · types.go · tests

### 1.6 communications ✅
- [x] `message-capture` (pluggable `Channel`, dedup/order) — schema.cue · tests
- [-] `participation-timeline` — per-author first/last/count stats; derivable in a few lines from captured messages; zero consumers verified repo-wide + sextant + throughline + growing-gardens-bi. Retired Wave 4, v0.6.0.
- [x] `clarification-loop` (open/answer questions) — tests
- [-] `source-sync-checkpoint` — **retired (H2/D35-5, 2026-07-15)**: zero consumers ever; the one real workload (throughline) proved checkpoint intent belongs on `coverage.SourceCoverage.Cursor`/`LastSync` (its doc-29 deviation); canonical mechanism now documented in `foundations/investigation/coverage`'s package doc

### 1.7 coordination ✅(core) — plan-only; executor is D7's kits/worker-pool (unbuilt)
- [x] `task-tree` (cycle-rejecting tree, status FSM) — schema.cue · tests; **FB-19 (2026-09-02)**: CUE `#TaskState` (pending/running/input-required/completed/failed/cancelled, `#TerminalState`/`#InterruptedState` classes) + `CanonicalState` mapping the three divergent `coordination/*` enums (task-tree/swarm-orchestrator/implementation-queue) onto it; plan-only, I20 — table over every enum value + schema.cue-read coverage test (`state_test.go`)
- [x] `swarm-orchestrator` (plans wave batches; does NOT spawn or execute workers; "spawns workers" language removed Wave 4) — schema.cue · tests
- [x] `worker-heartbeat` (liveness `Stale(now,ttl)`, injected clock) — schema.cue · tests
- [x] `convergence-detector` (sliding window, converged/exploring/abandoned) — schema.cue · tests
- [x] `result-aggregator` (deterministic merge/dedup) — schema.cue · tests
- [x] `implementation-queue` (FIFO + dedup + status) — schema.cue · tests

### 1.8 data ✅
- [x] `data-contract` (CUE validation + `Validate(record)`) — schema.cue · tests
- [x] `embedding-provenance` (tracker + Validate) — schema.cue · tests
- [x] `schema-migration-ledger` (append-only, replay) — schema.cue · tests

### 1.9 distribution
- [-] `artifact-builder` — sha256 content-addressed memoized copy of its input bytes; "builder" overstates (no transform step); its only importer is sibling `artifact-store`, itself zero-consumer. Retired Wave 4, v0.6.0.
- [-] `artifact-store` — versioned `Store` + `MemoryStore`; zero consumers beyond sibling `artifact-builder`. Retired Wave 4, v0.6.0.

### 1.10 embedding ✅
- [x] `embedding-client` — `Embedder` iface, `CachingEmbedder` (hit/miss), deterministic `MockEmbedder` — tests
- [x] `CachingEmbedder` bounded — LRU cap (default 4096), latent `EmbedBatch` dedup bug fixed, `DimensionedEmbedder` capability — 24 tests
- [x] `http-embedder` — OpenAI-compatible `/embeddings` client (hermetic httptest, key-never-in-errors test) — schema.cue · 16 tests
- [x] `vector-store` — `VectorStore` iface, `MemoryVectorStore`, `Cosine`, `Indexer` (lifted from sextant, generalized) — 16 tests

### 1.11 environment ✅
- [x] `access-broker` (generic credential negotiation, policy-as-data) — schema.cue · tests
- [x] `environment-registry` (dev/staging/prod) — schema.cue · tests
- [x] `platform-config` loader (JSON + Validate) — schema.cue · tests
- [x] `tool-execution-policy` (deny-by-default `Decide`) — schema.cue · tests

### 1.12 investigation ✅(core)
- [x] `investigation-journal` (append-only) — schema.cue · tests
- [x] `investigation-memo` (`Store` iface + in-memory; SQLite = future backend) — tests
- [x] `hypothesis-ranker` (support-sorted, deterministic) — schema.cue · tests
- [x] `convergence-oracle` (converged/exploring/needs_sources) — schema.cue · tests
- [x] `investigation-template` (named step sequences) — schema.cue · tests
- [x] `dad-investigator` (Decompose-Analyze-Decide FSM) — schema.cue · tests
- [x] investigation-journal persistence — telemetry sink hook, JSONL round-trip via `persistence/jsonl-log`, `NewFromEntries` — 37 tests
- [x] `coverage` — per-source completeness ledger (known / scanned / extracted) — schema.cue · 21 tests
- [x] `element-coverage` — claim/theory → named-elements coverage board — schema.cue · 12 tests
- [x] `narrative-shift` — declarant story-change detection over time (prior-inconsistent-statement gold) — schema.cue · 5 tests
- [x] `lead` — open investigative leads with provenance-bearing transition history — schema.cue · 24 tests
- [x] `query-planner` — (D35-2, kit-side done) deterministic expansion + `Refiner` seam + `ScreenFunc`; throughline swap completed H2 Wave B (2026-07-15): `gatedRefiner` adapter landed consumer-side, donor deleted, kit core proven-shape — schema.cue · types.go · tests

### 1.13 knowledge ✅
- [x] `document-parsing` (heading-segment parser) — schema.cue · tests
- [x] `knowledge-card-generation` (archetype template fill, imports contracts) — schema.cue · tests
- [x] `memory-manifest` (append-only manifest) — schema.cue · tests
- [x] `authority` — legal authority as a first-class node distinct from factual evidence: citation/treatment graph + good-law status — schema.cue · 41 tests
- [x] `catalog` (docs/designs/catalog-kit.md, D13: 3 in-fleet consumers named) — arbitrary-depth `Node`/`Catalog` (flat node-set, ID = slash-joined ancestor slugs, no pointer tree so depth is free), facets-not-levels (`Variants` Cartesian-expands an item's facet map, never a tree level), `Verify`/`Drift` drift gate (missing/extra/changed/moved, moved proven NOT to double-report as missing+extra), `Build`/`Layer` union-loader overlay model (ADD/EXTEND/OVERRIDE, `Provenance.ContributingLayers`/`Overridden`, no delete — only `Hidden`, `Drift.ForLayer` for per-source attribution), BM25 `Search` + injected `Ranker` seam + explicit no-match `Outcome`, deterministic markdown/JSON/tree renderers (byte-stable across independently constructed node sets), compute (`Rollup`/`DepthHistogram`/`CoverageByFacet`/`Orphans`/`DuplicateSlugs`), CUE `#Node` schema for authored catalogs + `LoadAuthoredJSON`/`Flatten`, MCP tool shapes as handler functions (no server) — schema.cue · package-doc-as-README · 90+ tests. Consumer #1 proven: `cmd/kit-catalog`'s own live source scan `Verify`s clean against the committed `docs/catalog.md` (`cmd/kit-catalog/consumer_gate_test.go`) — a real gate, not a demo. Worked example: `examples/catalog-demo` (design deliverable #8) scans a fixture tree and `Verify`s it against a deliberately stale `testdata/published.json`, exercising all four drift kinds including a rename-as-Moved. Deferred: wiring the licensed-component-corpus and MCP-tool-catalog consumers named in the design's §10 — no trigger yet beyond "a session picks up that slice."

### 1.14 orchestration ✅(core)
- [x] `workflow-engine` (FSM/DAG, guards+actions, replay, sentinel errors) — schema.cue · 16 tests
- [x] workflow-engine dead-policy removal — inert `RetryPolicy`/`TimeoutPolicy`/`BackoffStrategy` removed from types.go/schema.cue/fixtures (declared but never executed by the engine; zero consumers repo-wide + sextant); package doc now states retry/timeout are caller-owned (**BREAKING**, see CHANGELOG)
- [x] `express-lane` (size-check + spec-challenge fast path) — schema.cue · tests
- [x] `planning-refinement` (refine loop, escalate after N rounds) — schema.cue · tests
- [x] `operator-elicitation` (ask/answer/pending requests) — schema.cue · tests
- [x] `improvement-flywheel` (signal→prioritized proposals) — schema.cue · tests
- [x] `materiality-cache` (freshness-keyed cache + invalidation) — schema.cue · tests
- [x] `probe-runtime` (`Probe` iface, ordered RunAll, panic recovery) — schema.cue · tests

### 1.15 prose ✅(core)
- [x] `prose-as-code` (ProseSegment/Block + Validate) — schema.cue · tests
- [x] `prose-plan` (SectionPlan + Validate) — schema.cue · tests
- [x] `prose-policy` (configurable DLP posture, Evaluate) — schema.cue · tests
- [x] `traceable-prose` (annotations, redaction provenance, regen marking) — schema.cue · tests
- [x] `context-renderer` (budget-bounded segment render) — schema.cue · tests
- [x] `evidence-citation` (deterministic citation render) — tests
- [x] `evidence-citation` **parse direction** — `NewParser` (caller-supplied tag vocabulary), `Parse` with byte offsets, `Validate` anti-fabrication seam, offset-safe `Strip`/`Replace`, `Cite`→`Parse` lossless round-trip (fuzzed) — second-consumer-driven (throughline parseCitations) — 11 tests

### 1.16 quality ✅(core)
- [x] `qa-harness` (`Judge` iface, `RubricJudge`, `MockJudge`) — tests
- [x] `eval-gate` (weighted checks, threshold) — schema.cue · tests
- [x] `correction-estimator` (effort score+band) — schema.cue · tests
- [x] `dlp-evaluator` (configurable DLP rules, risk band) — schema.cue · tests
- [x] `integrity-evaluator` (fabricated/unresolvable refs) — schema.cue · tests
- [x] `honesty-gate` (allowed/hedge/block) — tests
- [x] `agentic-practices-evaluator` (checklist scorecard) — schema.cue · tests
- [x] `fitness-function-registry` — typed registry of named `FitnessFunction` values (`prototype` / `integration` / `apex` tiers, threshold 0–1, injected runner); `Register`, `Get`, `List` (deterministic sort), `ExecuteAll`; validates on registration (unrecognised tier, out-of-range threshold, duplicate name); design re-derived from reconstructed Update-2 scan — schema.cue · types.go · 17 tests
- [x] `eval-journal` — (D8, D9, Update-2) `#EvalRecord`; byte-stable JSONL via `jsonl-log`; empty-rationale⇒non-decisive enforced — schema.cue · types.go · tests
- [x] `scenario-bank` — (D8, D9, Update-2) provenance discipline (production-captured rejected in committed data); `RubricJudgeFrom` compiling into existing `qa-harness` `RubricJudge`; `HistoryFor` projection — schema.cue · types.go · tests
- [x] `diff-reporter` — (D8, Update-2) `Diff` + `TrajectoryDiff` (one primitive, two roles); caller-config thresholds with units — tests
- [x] `determinism-gate` — (D8, Update-2) structural primary: K-run variance probe, agreement/entropy/flips; sampling opportunistic per model-catalog capability caps; `Stable`/`Unstable` measured bands; `FitnessFunction` loop-honesty checks in `determinism-gate/fitness.go` — schema.cue · types.go · tests
- [x] `reliability-stats` — (FB-11/12, 2026-09-02) `PassAtK` (Chen et al. 2021 unbiased estimator, product form), `PassPowK` (τ-bench p^k; `PassPowK(0.9, 8) = 0.4305` pinned), `PairedBootstrapCI` (seeded, task-clustered, percentile, byte-stable golden), `Wilcoxon` (exact ≤ 25 by subset-sum enumeration vs a brute-force oracle, tie-corrected normal approximation above, zeros dropped and reported), `A12` (Vargha–Delaney, ties half, pinned to the rank-sum form); non-guarantees stated (no multiple-comparison correction — the caller pre-registers; percentile not BCa; NaN for undefined inputs); consumers: eval-runner `Regression` pass^3 (FB-13) and fathom's bench — schema.cue · types.go · 20 tests
- [x] `check-catalog` (`foundations/quality/check-catalog`, package `checkcatalog`) — **fourth-consumer uplift RC-1 (2026-09-09, racli ADR-021 §5a)**: a gate's checks as rows — `Severity` (fatal | major | data | improvement; `Blocking` decided once, on the severity, never per check), `Means` (deterministic | model), `Row`/`Check[In]` generic over the input, `Reconcile(catalog, checks)` returning the drift in BOTH directions (`Unimplemented`: a row with no check; `Undeclared`: a check with no row; plus field mismatches, duplicate ids, nil `Fn`, invalid rows) so the consumer's bidirectional test is one call, `Run` in id order with `Skipped(reason)` so a check that cannot run never looks passed (a nil `Fn` is skipped, not silent), `DeterministicShare`; fatal ⇒ deterministic in schema.cue and `Row.Validate`. Generalizes racli's audit catalog (`cmd/audit/audit.go`, `cmd/audit/catalog_test.go`, `resume/schema/audits.cue`); one consumer, so `experimental` (D13) — schema.cue · testdata · 14 test funcs
- [x] `structural-refusal` (`foundations/quality/structural-refusal`, package `structuralrefusal`) — **fourth-consumer uplift RC-3 (2026-09-09, racli ADR-021 §5c)**: an affirmative verdict with no citable evidence is refused IN THE VALUE — `Refuse(v, evidence)` rewrites `meets` to `unknown` (a refusal that lives in a renderer is a refusal one renderer can forget), `Normalize` discards uncitable references BEFORE counting them and records why in `Refused` (idempotent; a standing verdict carries no note), `New` is the only Go constructor, `UnknownRate` normalizes on READ so a judgment that reached a store through a loader is counted by the same rule, `Rate.Render(floor)` says "not enough judgments yet" below the floor. `does-not-meet` needs no evidence by design (only the affirmative claim carries a burden of proof); schema.cue states the same rule (`meets` ⇒ non-empty evidence; a standing verdict cannot carry `refused`). Generalizes racli's `cmd/tracker/match.go` (`RequirementMatch`, `NormalizeMatch`, `UnknownRate`); one consumer, so `experimental` (D13) — schema.cue · testdata · 12 test funcs

### 1.17 retrieval ✅
- [-] `chat-retriever` — index-backed `Retriever` + embedding reranker stub; superseded by `evidence-ranker` (LLM-free deterministic feature ranker, properly named and scoped). Zero consumers. Retired Wave 4, v0.6.0.
- [x] `intent-classifier` (configurable keyword rules) — schema.cue · tests
- [-] `lookup-index` — inverted index, overlap ranking; self-described as a "stub"; superseded by `evidence-ranker` + `vector-store`. Zero consumers. Retired Wave 4, v0.6.0.
- [x] `thread-resolver` (group messages into threads) — tests
- [x] `fuzzy` — pure fuzzy match/rank foundation: `Match` (greedy subsequence, word-boundary + contiguous-run score bonuses, Unicode), `MatchTerms` (AND over space-split terms), `Rank` (ranked slice with source Index preserved; deterministic tie-break by insertion order); no dependencies on stdlib beyond `sort`/`unicode` — 25 tests (`264f754`)
- [x] `truth-bridge` — (roadmap #24) `#ChunkLink` span↔chunk link; `Graded()` with loud `GradeUnknown` never silent — schema.cue · types.go · tests
- [x] `retrieval-metrics` — (roadmap #24) Precision/Recall/MRR + `FalseAbsentRate` from canary records + `CoverageFraction`; per-archetype applicability matrix; typed not-applicable never NaN — schema.cue · types.go · tests
- [x] `evidence-ranker` — (roadmap #24) LLM-free deterministic feature ranker: pattern strength/path specificity/hit density/ref stability/grade tier; caller-config weights; lexicographic tie-break; similarity mode marked non-deterministic and REFUSED for absence archetypes with `ErrSimilarityRefusedForAbsence` — tests

### 1.18 schema ✅
- [x] `schema-evolver` (versioned migration graph, BFS path, compat check) — schema.cue · tests
- [x] `taxonomy-manager` (cycle-rejecting hierarchy, org-configurable) — schema.cue · tests

### 1.19 telemetry ✅
- [x] `event` — normalization + CUE-validated `#Event` shapes — schema.cue · tests
- [x] `sink` — `Sink` interface, `MultiSink`/`MemorySink`/`FilterSink` — tests
- [x] `event` trace correlation — `TraceContext`, context carry, reserved attrs — trace.go · 36 tests (event total)
- [x] `metrics` — Prometheus text-format registry — 14 tests
- [x] production sinks — JSONL sink + slog bridge — jsonl.go · slog.go · 29 tests (sink total)
- [x] `event` W3C trace carrier — **H6 (2026-07-15)**: `FormatTraceparent`/`ParseTraceparent` (version-00 strict, future-version tolerant, fail-closed on malformed/all-zero), `AppendTraceparent`/`TraceFromEnviron` env carrier for subprocess hops (roadmap #21 graduated; bounded-exec injection = pure composition, canary-tested) — 8 test funcs
- [x] `otel-genai` — semconv v1.27.0 `#AttrMapping` (CUE); OTLP/JSON traces sink; spans for `llm_call`, span events for the rest; drop-and-count failure policy; batch + `Flush`; no goroutines — schema.cue · tests
- [x] `heartbeat` (`foundations/telemetry/heartbeat`) — **fathom-asks FA-4/FA-5 (2026-07-16)**: byte-stable JSONL channel beats (injected clock, accept-partial Load) + pure `Rollup`/`CheckLiveness` projections (paired if/then expectations → advisory alarms; never a process exit). First subscriber: fathom Gap-1 (the four-dark-days injection channel). Subscribe-when second: this repo's own proof-gate liveness — schema.cue · 5 test funcs
- [x] `read-manifest` (`foundations/telemetry/read-manifest`, package `readmanifest`) — **field-absorption FB-8 (2026-09-02)**: a reader's blindness made visible — `Outcome` read | absent | unreadable(errno) | skipped(category), `Set.Record` first-outcome-wins (a retry cannot erase a recorded failure), `Manifest.Complete` false iff any path was unreadable, `LowerBound` renders "≥ N (k unreadable: …)" over an incomplete manifest and a plain "N" otherwise; sorted, byte-stable, `sync.Mutex`-safe, stdlib only. Generalizes fathom's `internal/readset` (its ADR-027: one unreadable file made an audit report 0 invariants for 37). Consumers: `cmd/constitution-lint` (`walkWithManifest`, `read-coverage`); fathom by pin bump — schema.cue · testdata · 8 test funcs

### 1.20 topology ✅(core)
- [x] `live-graph` (cycle/conflict detection, critical path, wave planning) — 24 tests. **Plan-only** — plans wave batches; does NOT spawn or execute workers; the executor is D7's `kits/worker-pool`, unbuilt. Subscribe-when: a real fan-out workload.
- [x] `workspace-materialization` (transitive dep set from topology) — tests
- [-] `topology-task-type` — three-value task-kind enum (rollout/validate/drain) + `Validate`; no engine in the kit executes these tasks; zero consumers verified repo-wide + throughline + sextant. Retired Wave 4, v0.6.0.

### 1.21 governance — abuse & cost control (WASM-safe: wasip1 + js/wasm) ✅
- [x] `rate-limiter` — token bucket with sliding-window semantics, single-key `Limiter` + `KeyedLimiter` — schema.cue · 11 tests
- [x] `circuit-breaker` — closed/open/half-open breaker for downstream protection — schema.cue · 11 tests
- [x] `budget-guard` — multi-dimension quota (req/min, req/day, USD/day): the bill cap for agent workflows — schema.cue · 11 tests; **Wave 2**: two-phase `ChargeTracked`/`Settle` (idempotent, WASM-safe preserved); closes friction F4
- [x] `strike-ledger` — append-only violation ledger, escalating-backoff blocklisting — schema.cue · 16 tests
- [x] `proof-of-work` — hashcash-style gate; **redesigned**: challenges HMAC-SHA256-signed by a server-side `Issuer` (the old client-trusted `Issue`/`Verify` was forgeable and is REMOVED — **BREAKING**); bounded `SeenSet` (TTL eviction + hard cap, fail-closed) — schema.cue · 28 tests
- [x] `identity` — privacy-preserving identity extraction (salted SHA-256 digests; raw PII never stored or returned) — 16 tests
- [x] `topic-gate` — cheapest-first in-scope decision before paying for a model call; `JudgeOnly` mode for no-embedding deployments — 16 tests
- [x] `answer-cache` — two-level match cache for near-static LLM answers (cost saver) — 12 tests
- [x] `retention-policy` — (roadmap #18, WASM-safe) pure `Due()` evaluation of caller-supplied retention rules; `shred|flag|report` actions; the foundation decides, callers execute — schema.cue · types.go · tests
- [x] `identity-quota` (`foundations/governance/identity-quota`) — per-identity rolling-day quota, the companion to `budget-guard`'s aggregate one: bounds what a SINGLE identity may consume per rolling day (req/day, USD/day), capacity-bounded (`MaxIdentities`) with TTL eviction so a rotating-identity attacker cannot grow the tracker without bound — schema.cue · tests

### 1.22 agent — tool & loop primitives ✅
- [x] `tool-registry` — define a tool once (`llmclient.ToolDef` + handler): sorted `Defs` for Request.Tools/MCP, `Dispatch` with policy hook (tool-execution-policy adapter), panic recovery, per-tool timeouts, `is_error` tool_results; flat `ObjectSchema` builder + RawMessage escape hatch — 37 tests; **Wave 2**: lethal-trifecta capability taxonomy (`CapabilitySet` nil-unknown vs declared-empty distinction; `TrifectaStrict` fail-closed registration blocking with provenance-bearing `TrifectaOverride{AuthorizedBy, Reason}`; unknown caps = worst-case under strict); `mcp-client` `RegisterAll` gains `BridgeOptions.CapabilityMap`; **2026-09 field absorption (FB-14, decision 0002)**: `Effect{ReadOnlyHint, DestructiveHint, IdempotentHint, OpenWorldHint}` — the MCP tool-annotation vocabulary verbatim — as an optional per-tool declaration (`Tool.Effect`, `EffectOf`; nil = undeclared = destructive under a strict harness, `&Effect{}` = declared additive), `Defs()` unchanged; `schema.cue` `#Effect` + fixtures — 4 new tests
- [x] `conversation` — multi-turn state that makes API invariants unrepresentable to violate: exact tool_use/tool_result pairing in ONE user message, role alternation, pair-safe budget windowing with `PreserveHead` + role-safe compaction hook, snapshot/restore — 21 tests; **Wave 2**: `Compactor` widened to `(ctx, dropped []Message) (Message, bool, error)`; `WindowCtx` primary; error semantics = no silent loss
- [x] `bounded-exec` — resource-BOUNDED subprocess runner (hard deadline, process-group kill on unix, output caps, explicit env allowlist); doc states **NOT A SANDBOX** + non-guarantees — 17 tests
- [x] `evidence-compactor` (NEW, closes roadmap #1, practices §1 closer) — journals dropped messages verbatim (jsonl-log, caller-supplied writer) BEFORE the LLM summary call; versioned prompt-template with `PromptHash`/`ParamsHash` provenance; structured `[COMPACTION: …]` marker + journal ref in the summary; LLM failure → never a lossy fallback; constitution-lint tracking-coverage requires this entry — schema.cue · types.go · 12 tests. **Wiring truth (FB-6, 2026-09)**: until `agentharness.WithEvidenceCompactor` landed, nothing in the kit installed this compactor — `compat.md` had claimed it "wired into agent-harness's `Config.ContextBudget` compaction hook" while the hook only ever dropped; now `WithEvidenceCompactor(cfg, ec)` installs it (refusing a budget-less config) and `examples/agent-cli` wires it behind `-context-budget`/`-compaction-journal`

### 1.23 cost · cache · persistence · prompt ✅
- [x] `cost-tracker` (`foundations/cost/cost-tracker`) — config-driven price table (no prices shipped), concurrency-safe tracker — schema.cue · 10 tests
- [x] `result-cache` (`foundations/cache/result-cache`) — generic TTL+LRU memoization, single-flight `GetOrCompute` — 15 tests
- [x] `jsonl-log` (`foundations/persistence/jsonl-log`) — append-log writer/reader, torn-tail recovery surfaced — 16 tests
- [x] `prompt-template` (`foundations/prompt/prompt-template`) — versioned template render producing `PromptHash`/`ParamsHash` for ai-provenance (closes the no-producer gap) — schema.cue · 9 tests. **fathom-asks FA-1 (2026-07-16)**: `Template.TextHash()` — content-addressed template identity for downstream digest gates (per-template, vs per-render `PromptHash`); H4's hand-rolled sha256-of-Text pin becomes this method

### 1.24 provenance + reasoning ✅
- [x] `ai-provenance` (`foundations/provenance/ai-provenance`) — authorship record (human or AI with full reproduction key: model + prompt + params + input digest) — schema.cue · 25 tests
- [x] `event-store` (`foundations/provenance/event-store`) — append-only hash-chained event store (chain-of-custody system-of-record) — schema.cue · 28 tests; **Wave 3**: JSONL persistence (`AttachSink`/`JSONLSink`/`LoadJSONL`/`NewStoreFromJSONL`); payload-hash-only persistence documented; sqlite/kafka adapters are consumer-side
- [x] `anchor` (`foundations/provenance/anchor`) — Merkle proof-of-existence over batches of content-hashes — schema.cue · 29 tests
- [x] `defeasible` (`foundations/reasoning/defeasible`) — effective-state engine: base provisions + layered typed modifiers, `Resolve` with conflict/supersession handling, full trace + confidence — schema.cue · 33 tests

### 1.25 decision — typed fast-decision port ✅ (2026-09-22)

- [x] `decision` (`foundations/decision`, docs/plans/decision-port.md) — typed fast-decision port: callers ask typed `Question`s (`KindChoice`/`KindScore`/`KindNoul`, mirroring TypeSafe AI's System One primitives on purpose) about a `Request.State` and get probabilities back, never text. `Validate` (IDs non-empty/unique, known kinds, choice 2..255 unique-named options, score Max>Min, non-empty State); `Confidence(probabilities)` implements the vendor-documented k/max formula (k<=1 -> 1; else (k*max-1)/(k-1) clamped to [0,1]; k=3 is exactly TypeSafe's (3*max-1)/2); `Band`/`BandOf` (unsure <0.5, confirm, proceed >=0.9). Two adapters ship: `None` (`ErrNoProvider`, the zero-config stub) and `LLM` (one `llmclient.Request` per `Decide` call — a fixed System contract, a deterministic JSON Prompt with Question order preserved, an OutputSchema built per-kind — parses the reply per question: an invalid choice/missing/malformed answer lands in `Response.Invalid` by question ID without failing the rest of the batch; probabilities are filled-missing-as-zero and renormalised; scores/nouls are clamped; Confidence is always computed by this package's formula, never taken from the model; a reply that is not JSON at all is the one hard error). `Response.Records(site)` flattens to a sorted, storage-friendly `[]Record`. `MockDecider` test double. schema.cue (`#Kind`/`#Option`/`#Question`/`#Request`) · testdata valid/invalid pairs · golden fixtures pinning the rendered LLM prompt and OutputSchema (testdata/prompt.golden, testdata/schema.golden) — 34 tests. The exported API is a contract external consumers are coding against concurrently (not a speculative package); zero in-kit consumers today. **DP-10/DP-11 (2026-09-22, docs/plans/decision-port.md):** the `typesafe` adapter, promoted from parked — D8's hold ("no named second provider until a workload pulls for it") is lifted: fathom, a real workload, pulled for it on owner direction. `TypeSafe`/`NewTypeSafe`/`TypeSafeConfig` call TypeSafe's System One API directly (`POST {BaseURL}/v1/systemone`, one request per `Decide` batch, questions keyed by ID on a Go map so the wire body is byte-deterministic); per-question classification into `Response.Answers`/`Response.Invalid` mirrors `LLM` exactly (missing id, type mismatch, choice outside Options, probabilities not within 0.01 of 1, noul/score out of range), and Confidence is always this package's own formula — TypeSafe's wire `confidence` field is read for nothing; the package's `Confidence` is proven to compute the identical statistic. `TypeSafe.Ping` satisfies `llmclient.Pinger`'s method set without importing `llmclient`. `TypeSafeError{Status, Message}` (`Message` the response body capped to 512 bytes, never the API key) with `Retryable()` true for 429/529. `Question.Levels` (DP-11, additive): an optional ordered `[]string` of 2..10 level descriptions for `KindScore`, enforced by `Validate` (Min/Max must then be unset or exactly `0`/`len(Levels)-1`); `LLM` renders it into the prompt/schema when set and is byte-identical to before Levels existed when empty (the pre-existing golden tests pass unchanged, proving it); `TypeSafe` sends Levels verbatim as `criteria`, or synthesizes an ordered level list from an integral 2..10-value `[Min, Max]` range when Levels is unset — a score question satisfying neither lands in `Response.Invalid` without ever reaching the wire. schema.cue gains `Levels`; testdata valid.cue gains a Levels-bearing fixture. `testdata/typesafe_request.golden` pins the exact wire body for a mixed batch (one choice, one Levels score, one noul-with-criteria). 35 top-level test funcs total in the package (was 34) plus table-driven subtests, `-race` clean, zero network (`httptest`-backed). Still zero in-kit consumers; fathom (sibling repo) is the first named consumer, wiring pending there.

---

## 2. Kits (compositions)
- [x] `lesson-transfer` (`kits/lesson-transfer`) — bounded public-synthetic bundles, trusted receiver checks, applicability and revocation; actual strict-harness unknown controls and seeded false-pass regression, owned callbacks and wire/CUE tests pass. Cross-repository adoption is separately recorded in GW-14; fixture hashes alone prove no adoption.
- [x] `investigation` (`kits/investigation`) — evidence→DAD-investigator→journal→convergence, raises contradiction concerns — 8 tests; **Wave 3**: migrated to canonical `evidencespan.Span`; findings carry `SupportingEvidenceIDs`; `EvidenceDegraded=true` auto-raises a high-severity "finding_without_evidence" concern flag (fail-closed-honest; zero external consumers verified). Consumer annotation: consumed by `scenarios/governed-investigation`.
- [x] `command-center` (`kits/command-center`) — workflow-engine + review-queue + telemetry command surface — 10 tests. **Composition unproven** — no in-fleet consumer yet; zero consumers verified repo-wide + throughline + sextant + growing-gardens-bi.
- [x] `governed-prose-kit` — legacy bibliography/policy chain plus strict versioned `SourcedPipeline` (GW-09/10): independent per-claim review, exact offsets/arithmetic, correction invalidation and redaction-safe source maps; UTF-8 guards and shared private `internal/jsonwire` prevent identity normalization before copying/hashing or strict prose/lesson decoding (INC-12/13). Focused race/full validation green; no live consumer adoption or source-truth guarantee. [Contract](docs/designs/sourced-prose.md).
- [x] `endpoint-guard` (`kits/endpoint-guard`) — governance primitives wired into one hardened HTTP endpoint guard (PoW secret config, fail-closed defaults) — schema.cue · 20 tests. Consumer: `cmd/endpoint-guard` production sidecar.
- [x] `mcp-server` — stdio MCP server over agent/tool-registry: newline-delimited JSON-RPC 2.0, initialize version negotiation, tools/list + tools/call, bounded message size with fail-closed resync, batch rejection, JSONL call log (payloads excluded by default); tools-only v1 — schema.cue · 15 tests; **FB-16 (2026-09)**: `annotationsFromEffect` emits MCP tool annotations (all four hints explicit) in tools/list for tools with a declared `Effect`, no key for undeclared tools; `schema.cue` `#ToolAnnotations` = `toolregistry.#Effect` — 2 new tests. **Composition unproven** — serving side has no in-fleet consumer yet; zero consumers verified repo-wide + throughline + sextant + growing-gardens-bi.
  - **H6 (2026-07-15)**: Streamable HTTP transport (`NewHTTPHandler`/`HTTPOptions` — handshake-free stateless POST, MCP-Protocol-Version negotiation w/ 2025-03-26 absent-header fallback per spec, Origin fail-closed allowlist, MaxBytesReader, byte-identical dispatch vs stdio pinned by 11-case conformance table) + server-side traceparent extraction → child span; roadmap #15 transport half graduated (prompts/resources still unfired); design note docs/designs/mcp-streamable-http.md flipped BUILT with as-built findings
  - **FB-18 (2026-09-02, decision 0003)**: `ProtocolRevision` ("2025-11-25") + `Deprecations()`/`DeprecationsDue(now)` — the 2026-07-28 retirement clock (initialize handshake and ping still implemented; sessions/roots/sampling/logging/HTTP+SSE never built); CUE `#ProtocolRevision`/`#Deprecation`; `TestDeprecationFloorNotReached` reads the wall clock ON PURPOSE and goes red on 2027-07-28; `TestDeprecationsMatchClient` pins both ledgers to one feature set — 7 tests (`revision_test.go`)
- [x] `agent-harness` — the governed agent loop rebuilt around ONE `runLoop` with a `loopSink` seam — schema.cue · 65 tests (was 16); **Wave 2** additions: `RunConversation` (continuation from prior messages), `RunFromCheckpoint`+`Checkpoint` bundle {version, conversation snapshot, optional budget snapshot, turn cursor} (closes roadmap #9), `RunStream`/`RunStreamFromCheckpoint` (identical governed loop streaming: same budget, same injection screen, same stop causes; `HarnessEvent` channel, no-leak tested), `RunBlocks`/`RunStreamBlocks` (multimodal user turns), `RunOnce`/`RunOnceStream` (governed single completion: budget + journal + telemetry + `PreModelCall` hook, no tool loop; nil Registry allowed for RunOnce-only harnesses — consumer-driven, from oracle friction log); `Config.ContextBudget` (WindowCtx before each model call; model sees window, conversation keeps ground truth; compaction journaled; compactor error = run stops, never silent loss); `Config.Conversation` options passthrough; `Config.Hooks{PreModelCall, PreDispatch, PostToolResult}` (synchronous, fail-closed; `StopHook` stop cause; PreDispatch deny → denial tool_result, loop continues; identical on stream path; kits/hitl integration point); `Config.ParallelTools` (opt-in bounded parallel tool dispatch, result order preserved, per-result screening + hooks, -race proven). **2026-09 field absorption**: `Config.EffectPolicy` (`permissive` default | `strict`) — under strict an undeclared or destructive dispatch goes through the PreDispatch seam to `RequireApproval` → `Deps.Approver` (composes `kits/hitl.Preview` as the approval card; `ApproverFunc` adapter); denied/absent/errored approval = `is_error` to the model + `gate_denied` event (+ journal, + `HarnessEventGateDenied`), approval = `gate_approved` with `approved_by`; per call, after the caller's hook, sequential even under `ParallelTools`, `Defs()` unchanged (FB-15, decision 0002) — 13 tests; `WithEvidenceCompactor(cfg, ec)` installs `evidence-compactor` as the conversation `Compactor` (FB-6) — 2 tests; `Handoff{StopCause, Turns, LastTool, CheckpointRef, Tried, Summary}` via `HandoffFor`/`CheckpointRef`, set on `RunResult.Handoff` and emitted once (sink `step=handoff`, stream `HarnessEventHandoff`, journal) on `StopBudget`/`StopMaxTokens`/`StopMaxTurns`, never stored (FB-20, F2) — 6 tests; `schema.cue` `#EffectPolicy`, `#Handoff`. Consumers: `examples/agent-cli`, `scenarios/governed-docextract-hitl`, `scenarios/governed-investigation` (strict gate), `cmd/kit-new` generated output.
- [x] `docextract` (`kits/docextract`) — normalizes attached documents to prompt-ready text via a pluggable `Handler` seam (core module: zero external deps); format handlers `pdf` and `xlsx` live in nested Go modules (`kits/docextract/pdf/go.mod`, `kits/docextract/xlsx/go.mod`) each with their own dependency tree, preserving the root module's zero-require constitution (D1) — 7 tests (`44cfc99`). Consumer: growing-gardens-bi (assessed; D1 adaptation pending consumer side).
- [x] `hitl` (`kits/hitl`) — generic propose→preview→approve→apply engine (`Engine[State, Op]`): dry-runs ops via injected `Applier`+`Cloner`, computes impact `Gauge` metrics before commitment, optional `WithGuard` policy hook (fail-closed); fully generic over state and op types — 9 tests (`5512d5f`). Consumer: `scenarios/governed-docextract-hitl`. growing-gardens-bi assessed: not-yet — shape mismatch (no proposal/preview/approve cycle exists in its current code); adopt-worthy in principle; subscribe-when: a concrete human-approval change request is identified.
  GW-11 adds authenticated, version-bound stored decisions, atomic local commit and bounded remote reconciliation; reconstruction, replay, corruption, ownership, cancellation and expiry tests pass. Memory reference is not durable storage; [decision 0008](docs/intent/decisions/0008-bound-approval-transactions.md).
- [x] `eval-runner` (`kits/eval-runner`) — **H5 Stage 1 (2026-07-15), roadmap #5 graduated**: the honest QA loop live — CUE `#EvalConfig`/`#JudgeLane` config; judge independence validated at CONSTRUCTION via `modelcatalog.ValidateIndependence` (decisive verdicts require a cross-family judge; same-family constructs only non-decisive); judge-stack `Probe` at startup (fail-fast `ErrJudgeStackUnavailable`); budget fail-closed pre-charge (`WithBudget`, budget exhaustion journals error records + typed stop); every judge call → `evaljournal` record; `Regression(baseline, current)` gate over `diffreporter`; `determinismgate` K-run variance probe; provenance-gated scenarios via `scenariobank` (production-captured refused in committed testdata); fitness canaries wired to `fitness-function-registry` — schema.cue · testdata · 31 unit + integration tests. First subscriber per D9: this repo (`task eval`, replay mode, no network). Corpus per D6: throughline chat-prompt scenarios (donor commit-pinned, production-derived-redacted) + agent-cli-derived; **corpus provenance ratification = owner hand-back**. Stages 2–3 (judge pool/panel/escalation, conviction statistics) remain parked with verbatim triggers in the H5 plan. **FB-13 (2026-09-02)**: `Regression` additionally reports per-scenario pass^3 (`RegressionResult.PassPowK` via `reliability-stats`; reported, never gated; NaN on zero judged trials, `Excluded` counted).
- [x] `guardrails-pipeline` (`kits/guardrails-pipeline`) — pre/post-LLM chains with structured findings and explicit annotate/block policy; throughline chat adoption is default-vendor tested (`a007075`, kit `188c53c`). Experimental upgrade risk accepted under intent decision 0010; no semantic security or source-truth guarantee.
- [x] `cmd/endpoint-guard` production mode — real providers (anthropic + http-embedder), judge-only topic-gate fallback, `http.Server` timeouts + `MaxBytesReader`, fail-closed startup (placeholder salt / missing key / inadequate PoW secret refuse to start), Prometheus outcome counters on `/metrics`, seconds-canonical config (legacy ns accepted) — schema.cue · 21 tests
- [x] `expertchat` (`kits/expertchat`) — governed answer loop for fathom "expert modules" (`docs/plans/gke-expertchat.md`, agentic-kit side of the governed-knowledge-engine wave): `Retriever` seam (`HTTPRetriever` speaks fathom's REAL HTTP MCP dispatch contract — a `tools/call` JSON-RPC envelope, content-unwrapped, per `sextant/internal/mcp/dispatch.go` — invoking the PINNED `module_query` v0 tool (reconciled 2026-07-28 against fathom's own implementation of the same contract: `{text, module?, max?}` args → `{results: [{module, service, watermark, claims: [...], nodes (ignored — symbols aren't claims), staleness (reserved), error (per-module degradation, skipped unless every module errors)}]}`, `wire.go`; `FakeRetriever` deterministic test double) → tier/staleness `Policy` filtering (reuses `foundations/review/scoring`'s tier vocabulary; an unrecognized `TierThreshold` is a loud validation error, not a silently-disabled floor) → identity hygiene (an evidence item with an empty or token-unsafe module/claim_id/tier, or a module-host duplicate-claim-token collision, is dropped/hard-errored before the model ever sees it; empty Anchor/ModuleVersion are legitimate and allowed) → `Run` composes via `foundations/llmclient` with evidence content quoted/escaped against prompt injection, validating the model cites only supplied evidence tokens (reject + retry-once, then refuse on fabrication; an HONEST "CITES: (none)" refuses immediately, never retried) → `CitedAnswer` (zero citations unconstructable — `NewCitedAnswer` rejects an empty set; sorted, deterministic `Render()`) or a typed `*Refusal` (`no-evidence` / `below-tier-threshold` / `stale-only-evidence` / `fabricated-citation` / `evidence-does-not-answer`); optional `foundations/telemetry/sink` wiring — 67 tests. Critic-reviewed (MERGE-WITH-FIXES; all findings applied) + wire-reconciled against fathom's landed `module_query` (both same pass). **Composition unproven** — no in-fleet consumer yet; `examples/expertchat-service` is a runnable composition example (built-in demo retriever + a real `-retriever http` mode), not a consumer. app-kit's `appkit/ui/chat` UI wiring point is documented in the package doc, not built here (app-kit's own track).

### Wave 4 additions (v0.6.0)
- [x] `cmd/kit-new` — governed-by-default scaffolder: budget cap + injection block mode + journal + telemetry/cost + retry wired in generated `main.go`; `-minimal` opt-out with loud comment; `go:embed` templates; golden + integration-build tests — 15 tests · **`-package <root>/<category>/<name>` (2026-09-02, FB-21)** — scaffolds the six-file kit package shape into this repo (schema.cue `#Config` stub + `testdata/{valid,invalid}.cue`, types.go with `Validate`, doc.go with concurrency contract / non-guarantees / Verified-against line, `<name>.go` with an injected `now func() time.Time` seam, table-driven `<name>_test.go`, mock.go with `-interface`) and prints — or with `-apply` appends, each once — the exact TRACKING.md row, `docs/compat.md` row (experimental), CHANGELOG.md line, `docs/practices.md` row (`-practice N`) and the source-counted README package total; refuses to overwrite an existing scaffold and, with `-apply`, refuses before writing anything when a ledger anchor is missing; golden-tested (files, report, applied ledgers, idempotent re-apply), the scaffold passes the per-package constitution checks (gofmt, doc-coverage, concurrency-contract, schema-presence, no bare `time.Now()`, `cue vet`) and its own `go vet` + `go test -race` in a fixture module — 21 test functions
- [x] `cmd/eval-runner` — composition root for `task eval`: replay client (MatchByRequestHash), guard from config budget, `LoadBank`, regression gate; offline, deterministic, exit-code contract — H5 (2026-07-15)
- [x] `scenarios/governed-docextract-hitl` — first in-repo kit-composition proof: docextract (CSV, dep-free) → agent-harness (MockClient, budget + journal on) → hitl propose→preview→approve→apply; run-report rendered; 10 deterministic tests; skeptical-newcomer README
- [x] `docs/first-agent.md` — timed newcomer walkthrough; code blocks hand-verified against the current API; README's first "Start here" link
- [x] `docs/plans/{llm-surface,agent-loop,evidence-spine}.md` — 12 verified claims with `{symbol:}` anchors, all grep-verified to exist and be test-reached; fathom verify pre-push gate checks these (**2026-07-16 revision, vision-fit audit:** the sibling fathom binary's `test_reaches` bug is FIXED as of fathom `b7f6476` — the gate is non-vacuous; this row's old "claims hand-verified until fixed" note is retired)
- [x] `specs/` real-or-retired: `contracts/llmclient/injection-detector/evidence-graph` = REAL (checkboxes filled, enforcing tests named); `review/correction-ledger` = PARKED (design evolved into claim-lifecycle's different FSM; subscribe-when noted)
- [x] Adoption round-trips recorded:
  - sextant→vector-store: "Deferred. Sextant's feature branch is mid-flight; kit side ready. Subscribe-when: the branch merges and the embedding pipeline is wire-tested." `[-]` deferred — kit side done.
  - growing-gardens-bi→kits/hitl: "Not-yet — shape mismatch: no proposal/preview/approve cycle exists in its current code; adopt-worthy in principle (the engine fits a 'propose tuition edit → human approves' flow); subscribe-when: a concrete human-approval change request is identified."
- [x] `operator-elicitation` demoted per D2: "composition unproven" doc language added (throughline's richer elicitation shape is domain-specific; absorption would need org types)

## 3. Runtimes (2 active; 6 retired Wave 4, v0.6.0)
- [-] `execution/` — governed Runner (policy→guard→DLP-redact→LLM); execution story consolidated into `kits/agent-harness`; zero consumers outside the runtime itself; reference-ported only. Retired Wave 4, v0.6.0.
- [-] `connectors/` — `Channel`-polling capture runtime + MockChannel; adapters for a pipeline that doesn't exist here; zero consumers. Retired Wave 4, v0.6.0.
- [x] `policy/` — `Enforcer` over tool-execution-policy, deny-by-default — 9 tests
- [-] `security/` — content inspection: payload redaction, untrusted-instruction/credential-scope/repair-broadening guards; kept only if `execution/` was rebuilt on the harness lineage — it wasn't. Zero consumers. Retired Wave 4, v0.6.0.
- [-] `bridges/` — foundation→runtime bridges: outcome→journal projection, escalation, concern construction; adapters for the retired execution pipeline. Retired Wave 4, v0.6.0.
- [-] `ingest/` — raw→telemetry.Event normalization pipeline; zero consumers outside the runtime itself. Retired Wave 4, v0.6.0.
- [-] `mocks/` — `StubRunner`/`StubGuard` shared doubles; depended on retired `execution/`; no remaining consumers. Retired Wave 4, v0.6.0.
- [x] `credresolve` (`runtimes/credresolve`) — credential-resolution runtime: explicit config > env var > Claude Code CLI config, with macOS Keychain support (darwin build tag); typed resolution chain, injectable for tests — 15 tests (`c440004`)

## 4. Surfaces (2 active; 1 retired Wave 4, v0.6.0)
- [x] `forms/cueform` — two-file form engine: `Compose` (resolve paths/validate defs) + `NewDraftResponse`/`Validate` (ported from ref) — schema.cue · 40 tests
- [x] `editor/` — ResolvedForm→editor FieldModel (depth-first, deterministic) — 10 tests
- [-] `demo/` — capability demo registry shell; zero consumers; superseded by `scenarios/` and `docs/first-agent.md` as the newcomer on-ramp. Retired Wave 4, v0.6.0.

## 5. Toolchains (4 active; 1 retired Wave 4, v0.6.0)
- [x] `schemas/` — canonical CUE root (`#Archetype`/`#NodeKind`) + Go registry — schema.cue · 7 tests
- [-] `generators/` — deterministic Go-const + JSON-Schema generators; intent fully absorbed by `toolchains/jsonschema` (roadmap #16, graduated Wave 1); zero consumers of the old generators after the absorption. Retired Wave 4, v0.6.0.
- [x] `materialization/workspace-query-builder` — fluent DSL → deterministic SQL, in-memory execute/join (ported from ref) — 18 tests
- [x] `materialization/workspace-materializer` — execute packs → content-addressed `MaterializationReceipt` — 5 tests
- [x] `jsonschema` (`toolchains/jsonschema`, package `jsonschemagen`) — CUE→JSON Schema draft 2020-12 via the `cue` binary (OpenAPI export + pure-Go transform); deterministic output; `CheckFresh`; honest unsupported-construct failures; end-to-end proof test (CUE → derived schema → wire body in `outputschema_e2e_test.go`); implements roadmap #16 (graduated) — 15 tests

## 6. Scenarios, examples & demos
- [x] `scenarios/governed-investigation` — seeded end-to-end evidence→finding blueprint, deterministic — 9 tests; **2026-09**: dispatches one destructive tool (`rollback_deployment`, `DestructiveHint`) through agent-harness `EffectStrict` with a seeded approver — `Report.Remediation` carries the gate outcome; the consumer decision 0002 named — 3 new tests
- [x] `scenarios/governed-docextract-hitl` — first in-repo kit-composition proof: docextract → agent-harness → hitl; run-report rendered; 10 deterministic tests; skeptical-newcomer README (Wave 4)
- [x] `examples/governance-wasm` — governance suite compiled to js/wasm with server-authoritative PoW verify (`task wasm-build`; js/wasm-tagged, so excluded from host `go list`)
- [x] `examples/agent-cli` — the hello-world agent: mock/replay/live modes, committed DLP-scrubbed cassette, path-confined file tools via `os.Root`, cost report, documented middleware composition; `task smoke-live` (skips without `ANTHROPIC_API_KEY`) — 10 tests; **2026-09**: tools declare read-only `Effect`s, `-context-budget`/`-compaction-journal` wire `WithEvidenceCompactor` (FB-6), a `max_turns`/`budget`/`max_tokens` stop prints the `Handoff` (FB-20) — 4 new tests
- [ ] more `scenarios/` blueprints (deployment-safety, governed-release)
- [-] `demos/` — retired Wave 4 (see Surfaces § above); replaced by `scenarios/` + `docs/first-agent.md`

---

## Progress summary

Counts as of Wave 4 (v0.6.0, 2026-07-10). Source-counted packages include build-tagged ones invisible to host `go list`. Wave 4 deleted 16 packages (kill-list sweep, D5/§6); catalog regenerated to 174 packages total.

| Area | Go packages (active) | Retired Wave 4 | Status |
|------|----------------------|----------------|--------|
| Foundations (28 categories, 1.1–1.24) | 149¹ | 4 (participation-timeline, artifact-builder, artifact-store, topology-task-type) + 2 retrieval (chat-retriever, lookup-index) = 6 | all active pkgs tested |
| Kits | 11 | 0 | all tested |
| Runtimes | 2 | 6 (execution, connectors, security, bridges, ingest, mocks) | active tested |
| Surfaces | 2 | 1 (demo) | active tested |
| Toolchains | 4 | 1 (generators) | active tested |
| cmd / examples / scenarios | 6 / 2¹ / 2 | 0 | tested |

¹ `examples/governance-wasm` is js/wasm-tagged so the host `go list` sees one fewer. Catalog total = 174 (source-counted, all areas including cmd/examples/scenarios sub-packages).

Wave 4 adds: `cmd/kit-new` (governed scaffolder, 15 tests), `scenarios/governed-docextract-hitl` (kit-composition proof, 10 tests), `docs/first-agent.md` (newcomer walkthrough), `docs/plans/{llm-surface,agent-loop,evidence-spine}.md` (12 verified claims with `{symbol:}` anchors). Removes 16 packages. Honesty passes: swarm-orchestrator doc ("spawns"→"plans"), workflow-engine doc ("DAG/FSM"→"FSM"), operator-elicitation ("composition unproven" language), coordination/* and topology/live-graph plan-only annotations. Kit consumer annotations added throughout §2.

**Gate status: GREEN** — `task validate` passes; `constitution-lint` all checks passed.

- [x] **Proof gate re-armed and PROVEN LIVE (handoff H1, closed 2026-07-16)** —
  H1 tail complete: daemon restarted on the fixed binary (TCC wedge
  root-caused; restart procedure now routine), HTTP-mode acceptance 15/15
  on the capability plans (after fathom's extractor-epoch fingerprint fix,
  O9, cleared stale pre-fix observations fleet-wide), and the pre-push hook
  ran NON-VACUOUSLY on the fathom-asks pushes — its first real sweep blocked
  a push by catching 13 malformed acceptance contracts across five handoff
  plans (repaired with cited skips, d728bc3): the gate's founding episode.
  Original re-arm record follows. — the fathom `verify`
  bug class that forced the v0.5.0/v0.6.0 `FATHOM_BIN` bypass is fixed at the
  root: 7 defects (callgraph dedup poisoning, method-edge resolution,
  `package:` grammar drop, `.claude/` index pollution + missing orphan purge,
  alias canonicalization, same-package bare-call binding, constructor-scoped
  receiver inference), each pinned red-first, on sextant branch
  `fix/verify-proof-gate` (gate green there; `fix/verify-test-reaches` =
  main-based reference). Store-mode proof: **all 15 anchored claims verify
  15/15**; fresh scratch ingest indexes zero `.claude/` paths. Open —
  human-gated: Geoff runs `scripts/reingest-fleet.sh` from a login terminal
  (launchctl wedges from sandboxed tools) + merges the branch; then
  HTTP-mode verify + one real pre-push flips this row `[x]`.

### Downstream consumers (the gate's oracles)

- [x] `task sextant-check` — ../sextant builds + tests against kit HEAD (go.work)
- [x] `task throughline-check` — ../throughline builds + tests against kit HEAD (`-mod=readonly` bypasses its vendor/ so the check sees HEAD, not the vendored snapshot)
- [x] `task downstream-check` — both consumers in one gate; `growing-gardens-bi` joins as third consumer (skip-if-absent, Wave 0 item)
- [~] growing-gardens-bi consumer-side D1 adaptation — its go.mod needs require/replace pairs for the two nested modules (`kits/docextract/pdf`, `kits/docextract/xlsx` → `../agentic-kit/kits/docextract/{pdf,xlsx}`) + `go mod tidy`; downstream-check red for that consumer until applied (sextant + throughline green at v0.2.1). Consumer-repo change — apply from a session in that repo.
- [x] throughline audit (2026-06-10): builds green at HEAD; consumes 15 evidence/provenance/security foundations (155 imports — the spine's deep validation); adoption brief at `docs/upgrades/throughline-v0.2-adoption.md`
- [x] throughline budget adoption (Wave 2): hand-rolled `budgetClient` (~62 lines) deleted; `llminstrument.WithBudget` wired in; throughline gate green, vendor re-synced
- [-] sextant budget-stack wiring (Wave 2 deferred): sextant's checkout is mid-flight on a feature branch with a dirty tree; wiring deferred to Wave 3 once its branch is merged

## Pull queue (consumer friction has priority; bounded incubation per decision 0004)

Workflow implementation status: [2026-09 governed workflows](docs/plans/2026-09-governed-workflows.md).
- [x] INC-8 clock/layer repair: `NewPipelineWithClock`, canonical telemetry attributes with compatible harness aliases, and repository import-direction regression; `task validate` and affected package race tests green.

Consumer-proven asks from `docs/upgrades/throughline-v0.2-adoption-friction.md` and the doc-35 acquisition addendum. Each is a hand-rolled pattern already proven in a real consumer; the kit should absorb it so the consumer can delete its copy.

| # | Ask | Source | Status |
|---|-----|--------|--------|
| F1 | `GateFromVault` deterministic-token option (`WithDeterministicTokens()`) — value-stable per-namespace HMAC minting so the same PII entity gets the same token across calls; needed for stable model references and reproducible `PromptHash` | friction §1 | `[x]` **closed Wave 3** — `GateFromVaultOpts`+`WithDeterministicTokens` landed; `MemoryVault.TokenizeDeterministic` added |
| F2 | `VaultRule.Action` block semantic — refuse egress entirely for privileged content classes rather than tokenize; fail-closed Transformer refusal already the contract | friction §2 | `[x]` **closed Wave 3** — `VaultRule.Action` block field + typed `BlockedError` landed |
| F3 | `llminstrument.WithBudget` middleware — `budget-guard` on the call path (pre-flight estimate → `Charge` → typed `BudgetExceededError`); every consumer hand-rolls ~40 lines today | friction §3 | `[x]` **closed Wave 2** — `WithBudget` + `ErrBudgetExceeded` landed; throughline `adapters/llmclient.budgetClient` (~62 lines) deleted; downstream-check green |
| F4 | Two-phase `Charge → Settle` on `budget-guard` — `Settle(actualUSD)` or `Adjust(delta)` so the USD dimension converges on provider ground truth; conservative estimates over-throttle today | friction §4 | `[x]` **closed Wave 2** — `ChargeTracked`/`Settle` landed on `budget-guard` (idempotent, WASM-safe); consumed by `WithBudget` |
| F5 | `Wrapper.WrapWithNonce(content, nonce)` — caller-supplied content-derived nonce for reproducible `PromptHash` while preserving injection resistance; or document per-wrap-Wrapper as intended | friction §5 | `[x]` **closed Wave 3** — `WrapWithNonce` landed; threat tradeoff documented in package doc |
| F6 | `prompttemplate.ParamsFromStruct(v any)` — struct→params-map via `json:` tags, sharing canonical-JSON hash path; eliminates silent key-drift risk | friction §6 | `[x]` **closed Wave 3** — `ParamsFromStruct` landed; json-tag canonical params; key-drift regression pinned by test |
| F7 | `RunOnce`/`RunOnceStream` — governed single completion (budget + journal + telemetry + `PreModelCall` hook, no tool loop); nil Registry allowed for RunOnce-only harnesses | oracle friction log (Wave 2) | `[x]` **closed same wave** — built as direct output of oracle session; pull-ordered doctrine working as designed (friction found → kit item built in same wave) |
| F8 | Structured injection findings and explicit advisory/blocking policy in `PreLLMChain`. | throughline chat | `[x]` Kit regression and actual Throughline `a007075` default-vendor full gate pass at pinned kit `188c53c`; chat uses `Matches` rather than a parallel detector workaround. See consumer `docs/upgrades/governed-kit-delivery.md`. |
| F9 | Restart-stable authenticated encrypted token records, deterministic bindings and shred tombstones. | throughline SQLite vault | `[x]` Throughline `a007075` replaces runtime crypto with the kit-backed record adapter; immediate migration transaction, rollback/corruption/concurrency regressions and default-vendor full gate pass. [Contract](docs/designs/token-records.md), [migration](docs/upgrades/governed-token-records.md). |
| D35-1 | `intake-triage` foundation (`foundations/review/`) — generic human-only FSM: search→proposal→(imported\|excluded(reason)\|deferred), reversible exclusion, AI may only suggest | doc-35 §1 | `[x]` **closed H2 Wave B (2026-07-15)** — donor deleted: throughline `app/acquire` aliases `intaketriage.ProposalState`, `Service.Decide` hydrates via `ResumeFSM` and routes through `FSM.Decide`/`Restore` (excluded→imported internally Restore-then-Decide), `ai:*`→`ErrAIActorForbidden`; hand-rolled guard + local state constants deleted (grep-proven); uuid/ledger stay in Service (the detanglement); zero test-assertion changes — caller audit found no pinned forbidden transitions |
| D35-2 | `query-planner` investigation primitive — brief+entities+date-ranges → provider-agnostic query plans with per-query provenance (deterministic expansion + optional LLM refinement + DLP warnings) | doc-35 §2 | `[x]` **closed H2 Wave B (2026-07-15)** — donor deleted: throughline `app/acquire/refiner.go` (`gatedRefiner` = the `GatedClient`→`Refiner` adapter, provenance-capturing, mutex-guarded) + `planner.go` wraps `queryplanner.Planner`; `extractKeywords`/`stopwords`/`buildGmailQuery`/`extractEmails`/`llmRefineQueries` deleted (grep-proven); `ScoreRelevance` stays (different concern); `planner_test.go` parity canary: zero assertion changes |
| D35-3 | OAuth token-lifecycle core — expiry/refresh/revoke state machine (clock-injected, pure logic); HTTP exchanges are adapter work | doc-35 §3 | `[x]` **kit-side done H2 Wave A (2026-07-15)** — `foundations/security/token-lifecycle`: pure `Classify`/`NextAction`/`MergeRefreshed`/`RevocationTarget`, donor semantics pinned (60s skew boundary ±1ns, preserve-refresh-token rule, zero-expiry-never-valid) — schema.cue · testdata · 8 tests; throughline `adapters/oauth` rewire = Wave B → **`[x]` closed H2 Wave B (2026-07-15)**: `TokenStatus`→`Classify`, `Token`→`NextAction`, carry-forward→`MergeRefreshed`, revoke→`RevocationTarget`; four donor logic blocks deleted (grep-proven); pkce tests pass with zero changes; noted follow-up: empty-token-file `not_found` vs `reauth_needed` distinction stays adapter-side remap |
| D35-4 | Authenticity grading guidance — named constants or documented ceiling values for `MethodDKIM` (0.95) vs `MethodAuthResultsHeader` (0.55) so consumers don't re-derive the asserted-vs-verified distinction | doc-35 §4 | `[x]` **closed H2 Wave A (2026-07-15)** — `Method.ScoreCeiling` (0.95/0.55/1.0) + `Method.VerdictCeiling` (auth_results_header ⌈ likely_authentic) + opt-in `ClampToCeiling` (never lifts adverse verdicts, does not retro-fix stored assessments); `Assess` unchanged by design; asserted-vs-verified rationale in package doc |
| D35-5 | `source-sync-checkpoint` adoption question — either add multi-consumer checkpoint semantics or fold intent into `coverage`; throughline persists cursors via coverage projection (deviation in its doc 29) | doc-35 §5 | `[x]` **closed H2 Wave A (2026-07-15)** — decision: coverage wins (zero checkpoint consumers ever; throughline's doc-29 deviation is the proof); `source-sync-checkpoint` deleted; `coverage` package doc names `Cursor`/`LastSync` canonical |
| H8-1 | Opt-in `Client.CallToolVerified` and `BridgeOptions.VerifyPinBeforeCall` refuse missing, unpinned, ambiguous or drifted metadata before dispatch; legacy `CallTool` remains unchanged. | MCP bridge; workflow assurance 0005 | `[x]` GW-05: focused race tests, isolated full validation and Fathom static wiring pass. Remote list/dispatch cannot be atomic; local repinning is serialized through dispatch. |

_Doctrine: consumer defects and demonstrated friction retain priority. Decision [0004](docs/intent/decisions/0004-bounded-incubation.md) replaces the absolute zero-consumer prohibition with at most two bounded incubations. Scope G5–G8 and the [workflow design](docs/designs/governed-workflows.md) describe planned compositions; none is marked implemented by this assessment._

---

## Deferred (documented decisions)

From the uplift's adversarial design review — deliberate gaps, not oversights.
The full forward-looking treatment (25 pre-thought patterns, each with a
"subscribe when" trigger for new workloads) lives in **[docs/roadmap.md](docs/roadmap.md)**;
this list keeps only the v0.2.0-era decisions for the audit trail:

- [x] ~~`eval-runner` deferred~~ — landed H5 Stage 1; see §2's implementation and remaining evidence-integration limits.
- [x] ~~OpenAI-compatible chat client~~ — **landed Wave 3**: `foundations/llmclient/openai` (roadmap #10 graduated); the cross-family judge lane D9 requires.
- [-] cross-session memory — investigation-memo is in-memory by design; a SQLite-backed `Store` behind the same interface is the documented future backend.
- [-] HTTP MCP transport — the MCP spec moved to Streamable HTTP (old HTTP+SSE deprecated); v1 ships stdio-only, the streamable-HTTP listener is the documented roadmap in `kits/mcp-server` (design note: `docs/designs/mcp-streamable-http.md`).
- [x] ~~data-retention / crypto-shred for append-only stores~~ — **landed Wave 3**: `tokenization` gains `EncryptedVault`/`ShreddableVault`/`KeyStore`/`RunShredConformance`; `governance/retention-policy` evaluates Due() actions; roadmap #18 graduated.
- [~] evidence-graph Span migration — `evidence-graph.Span` is the legacy shape; migration to canonical `evidence-span.Span` assessed NOT CHEAP (TTL coupling, 250+ line ripple); `kits/investigation` migrated to canonical Span this wave; evidence-graph graph queries themselves deferred — honest assessment recorded; roadmap #23 annotated. See `foundations/evidence/README.md`.
- [x] ~~anthropic streaming: accumulate unknown-block (e.g. thinking) deltas~~ — **landed Wave 1**: streamed unknown-block delta accumulation implemented (roadmap #12 graduated); stream-assembled blocks now equal Complete-assembled.

### Deprecation watch (zero-consumer packages)

Wave 4 retired all previously-flagged packages. The three entries below have been promoted to `[-]` retired rows in their respective sections above (1.6, 1.9, 1.20). Retrieval stubs `chat-retriever` and `lookup-index` likewise retired (§1.17). Runtimes pipeline (bridges/connectors/ingest/mocks/execution/security) retired (§3). Surfaces demo/ retired (§4). Toolchains generators/ retired (§5).

No packages currently in the deprecation watch. The v0.7.0 trigger above **fired 2026-07-16** (v0.7.0 tagged) and was **re-evaluated 2026-09-02**: `coordination/*` and `topology/live-graph` still have zero external consumers, and they are KEPT — they are plan-only by I20 (D7's worker-pool trigger has not fired; `designs/eval-waves-d7.md` decided against firing it), so "zero executor consumers" is their designed state, not evidence of rot. One change came out of the re-evaluation instead of a retirement: `coordination/task-tree` now carries the single CUE `#TaskState` plus `CanonicalState`, mapping the three divergent state enums in the tree onto one vocabulary (FB-19) — the cheapest thing that makes the plan-only packages worth keeping. Next audit: if either still has zero external consumers by v0.9.0, re-evaluate against D7 again.

_Updated as work lands. The validation gate must be green before any item is marked `[x]`._
