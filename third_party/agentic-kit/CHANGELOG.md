# Changelog

Notable changes to agentic-kit. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/); versions are git tags.
Every release ships with the full gate green: `task validate` =
`fmt-check` + `vet` + `cue-vet` + `go test -race ./...`.

## [Unreleased]

Release impact: patch

## [0.8.1] — 2026-10-17

Release impact: patch

### Changed

- `intent-stale.sh` skips conditions inside superseded records and dates a condition by the newest commit touching any line of its paragraph, so a later disposition sentence resets its age.

## [0.8.0] — 2026-10-17

Release impact: major (pre-1.0: breaking bumps minor)

### Breaking

- Provider contracts add descriptive identity/capabilities and opt-in OpenAI Responses; anthropic typed effort/schema reject conflicting raw `output_config` overrides. Chat Completions defaults remain unchanged ([migration](docs/upgrades/governed-providers.md)).
- Token vaults reject forged identity and add versioned encrypted-record CUE, deterministic persistence and bounded memory storage without changing existing signatures ([migration](docs/upgrades/governed-token-records.md)).
- Decision port: the yes/no kind is `KindPredicate` and its canonical value is now `"predicate"`; the `KindNoul` constant is removed while `"noul"` still decodes to `KindPredicate`, and adapter wires are unchanged ([migration](docs/upgrades/decision-kind-predicate.md)).
- Decision port: the `LLM` and `TypeSafe` adapters move to `foundations/decision/llm` and `foundations/decision/typesafe` with their names unchanged (`llm.NewLLM`, `typesafe.NewTypeSafe`, `typesafe.TypeSafeError` and the rest); the core imports neither and adds `CanonicalKind` and `OutsideDeclaredLimits` for adapters in their own packages; wires, goldens, `Decider` and `Response` are unchanged. Every consumer that constructs an adapter changes its import ([migration](docs/upgrades/decision-adapter-subpackages.md)). Watch: fathom, migrated by its JF-24: the adapter constructors and names in `internal/decide` and `cmd/fathom` (`decideports.go`, `doctor_decide.go`), and the removed `KindNoul` in `internal/assess`, `internal/semantic` (`decisionrank.go`, `prefilter.go`), `cmd/fathom` (`certify_suggest.go`) and their tests.

### Added

- Instruction files: global instructions are one shared `~/.codex/AGENTS.md` that `~/.claude/CLAUDE.md` imports (`@~/.codex/AGENTS.md`); an adopter's `CLAUDE.md` content moves into `AGENTS.md` and the file is `@AGENTS.md` or deleted ([decision 0020](docs/intent/decisions/0020-agents-md-is-the-one-instruction-file-global-included.md), [adoption](docs/adopting-hosts.md)).
- `foundations/decision`: Deciders may declare capabilities (kinds, option and score-level limits, batching, and latency, cost and confidence-semantics classes) through the optional `CapabilityDeclarer` interface, read with `CapabilitiesOf`; `foundations/decision`: `NewReading` computes a site's calibration reading (n, accuracy by band with Wilson 95% bounds, Brier, AUROC, label provenance, recalibration due) and `CalibratedBand` keeps a confidence band only where that reading clears a stated bar (`HintBar`, `GateBar`), reading everything else as unsure; `foundations/decision`: four decorators that are Deciders themselves, built from plain options structs: `NewCascade` (cheap first, escalating per question on abstain or confidence below an uncalibrated default), `NewFallback` (the next Decider only on an error, never silent), `NewCached` (an injected in-process store keyed by question template and content hash, stamped with provider and model, hits kept apart from proof) and `NewRecorder`/`NewReplay` (a JSON transcript replayed without a provider call); each discloses per-answer provenance in a report and an `Observe` hook ([plan](docs/plans/decision-port.md#hexagonal-hardening), DP-17..DP-20); `foundations/decision`: `ComposeFromConfig` assembles those decorators from a JSON-decodable `Policy` tree (`#Policy` in its `schema.cue`) whose leaf providers, cache stores, transcripts and `Observe` hooks are named by key from a caller-supplied `Registry`, so a consumer reorders cascade stages or fallback Deciders, moves the escalation threshold or wraps a node in a cache or a recorder without a rebuild.
- `foundations/decision/openai`: an optional OpenAI Decisions API adapter (`openai.NewOpenAI`, `OpenAIConfig`, `OpenAIError`) beside Jev: one POST per Request, every answer type mapped (a refusal to `Response.Declined`), Confidence derived by the core's rule rather than the provider's own number, images as base64 data URLs, stdlib only; tested against `httptest`, no live call ([decision 0018](docs/intent/decisions/0018-one-decision-port-optional-provider-adapters.md), DP-25); `foundations/decision/typesafe`: Jev over OpenRouter's Decisions router (`TypeSafeConfig.Transport: TransportOpenRouter`, model `typesafe/jev-1.13`), and key sourcing shared by every remote adapter: `APIKeyFile` (read at first use, refused with `chmod 600 <path>` when others can read it, wins over a variable) or `APIKeyEnv`, with errors that name the source and never the key (DP-26, DP-27); `foundations/decision`: refusals are first-class (`Response.Declined`, `ErrDeclined`; `Fallback` treats a decline as a disclosed miss, `Cascade` escalates it, `Cached` never stores it), a Request may carry images (`Request.Images`, `Capabilities.Images`, `UnsupportedImages`; `llm` and `typesafe` refuse them), and `Capabilities.ModelPinning`; every new field is omitted from JSON when empty and a text-only `RequestKey` is unchanged (DP-28, DP-29).
- Experimental benchmark reuse plans bind retained trials and dependency evidence, permit historical regrading/repricing alongside required reruns or fresh live work, and never manufacture execution or independent samples; experimental assessment profiles bind domain criteria to versioned evidence, keep required failures/unknowns separate from scoped scores, and include a runnable benchmark/security reference.
- **Project Intent distribution and scoped ownership:** affected-path monorepo routing, inherited authority, evidence-aware doctor/audit and a reviewed, hash-pinned local skill snapshot for Claude Code and Codex ([decision 0007](docs/intent/decisions/0007-scoped-intent-and-pinned-distribution.md)).

- Versioned public-synthetic lesson bundles bind provenance and independent receiver reproduction, applicability and revocation; `kit-workflow lesson` validates local bytes without granting authority or claiming adoption ([contract](docs/designs/lesson-transfer.md)); experimental eval-runner benchmark receipts bind component identities, preserve failed tasks and unknown costs, and report reproducible paired acceptance comparisons.
- `kit-new host` plans, installs, probes and reverses project-local Claude/Codex hooks with ownership/drift checks; versioned receipts leave unobserved or unsupported controls non-passing ([adoption and limitations](docs/adopting-hosts.md)).
- The strict engineering profile requires scoped tools, guarded effects, bounded single-attempt accounting and independent completion evidence; receipts distinguish executed controls and unknown spend ([contract](kits/agent-harness/doc.go)).
- Offline sourced-prose/approval scenarios and `kit-workflow prose check|export` provide a bounded JSON front door with actual wire-contract tests and input-free help at every command level; source assertions remain unauthenticated and exports do not publish ([guide](docs/adopting-workflows.md)); durable HITL binds approval to authenticated actors, exact operations and current versions; atomic local commits consume decisions, while bounded remote attempts require reconciliation after ambiguous outcomes; strict sourced prose resolves versioned evidence, checks claims/arithmetic, rechecks freshness, invalidates corrections and exports escaped Markdown with a verifiable redaction-safe source map; actual JSON is checked against CUE. Legacy behavior remains unchanged, and semantic truth/external publication remain consumer-owned.
- MCP clients and registry bridges can opt into pre-call metadata verification that refuses missing, ambiguous or drifted tools; legacy calls are unchanged and server behavior after verification remains outside the guarantee.
- `read-manifest` records read/absent/unreadable/skipped paths, first-outcome-wins, and renders incomplete counts as lower bounds; constitution-lint's `read-coverage` now fails unreadable paths ([field absorption](docs/plans/2026-09-field-absorption.md)); `reliability-stats` adds `PassAtK`, `PassPowK`, seeded task-clustered percentile `PairedBootstrapCI`, exact/tie-corrected `Wilcoxon` and `A12`, with statistical limitations documented.
- Tool effects use MCP annotations, exposed by `EffectOf` and tools/list and pinned in description hashes; incoming annotations are untrusted unless `TrustedServer` is configured. Strict effect policy routes undeclared/destructive dispatch through HITL approval, refusing missing/denied/erroring approval with `is_error` and `gate_denied` ([field absorption](docs/plans/2026-09-field-absorption.md)).
- `WithEvidenceCompactor` installs the evidence compactor and requires `ContextBudget`; agent-cli exposes `-context-budget`/`-compaction-journal` ([field absorption](docs/plans/2026-09-field-absorption.md)); budget/max-token/max-turn stops carry `RunResult.Handoff` with cause, turns, tool history, final-state reference and summary, emitted once to sink/stream and printed by agent-cli; the kit stores no checkpoint ([field absorption](docs/plans/2026-09-field-absorption.md)).
- MCP exports `ProtocolRevision` (2025-11-25), `Deprecations` and `DeprecationsDue`, with a wall-clock test enforcing the 2027-07-28 retirement floor. Task-tree adds CUE task-state classes and `CanonicalState` across coordination enums, without an executor ([field absorption](docs/plans/2026-09-field-absorption.md)).
- `kit-new -package` creates tested package scaffolds with CUE fixtures, documentation, clock seam and optional mock interface, refusing overwrites; it prints or idempotently appends inventory/practice/changelog entries and updates the README count ([field absorption](docs/plans/2026-09-field-absorption.md)).
- Project Intent adds routed living documents, decision/revisit records, change-record rules and templates, adopted by the kit itself ([practice](docs/project-intent/README.md)). Monorepo guidance adds repository charters, component intent/routing and lifecycle/dependency rules plus doctor checks ([guide](docs/project-intent/monorepo.md), [kit charter](docs/intent/repo.md)).
- Heartbeat adds `CheckExistence`/`ExistenceExpectation`/`ExistenceAlarm` for channels with zero beats since a recorded start; malformed expectations are skipped, and existing liveness APIs are unchanged ([heartbeat](foundations/telemetry/heartbeat)).
- `claudecli` adds bounded subscription-backed completions, schema/effort mapping, explicit unsupported-field errors and local zero-token `Ping`; maxTokens remains unenforced; `codexcli` adds bounded subscription-backed completions with system/schema/effort mapping and a local `Ping` tripwire for its undocumented instruction-config dependency; openAI adds Azure deployment routing for complete/stream/probe, configurable pinned API version and API-key or caller-supplied Entra bearer authentication, with typed invalid-config errors and credential redaction; vanilla routing remains unchanged and token minting/refresh stays caller-owned ([Azure plan](docs/plans/azure-openai.md)); bedrock adds a stdlib credential chain—explicit, environment, shared files/process, SSO, IMDSv2—with independent region resolution, typed actionable errors and expiry-aware refresh; complete explicit credentials never fall through, and partial pairs fail.
- `tokenization/vaulttest` exports `RunShredConformance`/`VaultFactory` for downstream shreddable-vault adapters, preserving internal callers through delegation.
- Constitution-lint adds compatibility coverage, exported documentation, organization/model-literal hygiene, secret scanning and README package-count checks, with explicit waivers and mutation-tested canaries ([public-readiness plan](docs/plans/public-readiness.md)); the compatibility audit and v1/nested-module policy are documented alongside the dependency threat model, including caller-owned sandbox/deletion gaps and optional detector wiring; public classification/sign-off remains separate ([compatibility](docs/compat.md), [threat model](docs/threat-model.md)); `release-notes`/`task release-notes -- vX.Y.Z` prints one release block verbatim; agent-cli's walkthrough golden test checks the documented mock output.
- **`foundations/decision` (2026-09-22, docs/plans/decision-port.md):** a typed fast-decision port — callers ask typed `Question`s (`KindChoice`/`KindScore`/`KindNoul`, deliberately mirroring TypeSafe AI's System One primitives so a future `typesafe` adapter maps 1:1) about a `Request.State` and get probabilities back, never text. `Validate`, `Confidence` (the vendor-documented k/max formula, k=3 exactly TypeSafe's `(3*max-1)/2`), `Band`/`BandOf` (unsure/confirm/proceed at 0.5/0.9). Ships two adapters: `None` (zero-config `ErrNoProvider` stub) and `LLM` (one `llmclient` completion per `Decide` call, System contract + deterministic JSON prompt + a per-kind `OutputSchema`; a bad answer lands in `Response.Invalid` by question ID without failing the rest of the batch; probabilities filled-missing-as-zero and renormalised; scores/nouls clamped; confidence always computed by this package, never taken from the model). `Response.Records(site)` flattens to a sorted log-friendly shape; `MockDecider` test double. A `typesafe` adapter is deliberately parked, not built (Promote when: an API key exists and a consumer's A/B shows a gain) — schema.cue · golden-pinned prompt/schema · 34 tests. The exported API is a contract external consumers are coding against concurrently.
- **Push gates (`fathom-pack/scripts/push-check.sh`, `push-http.sh`; `docs/project-intent/operating.md` §5):** a push refuses without the lefthook pre-push hook, the gate verifies the tree it pushes, and the pack goes over smart HTTP derived from the push remote so no transport session is held open across a long gate — three rules each paid for by a consuming repo's incident (fathom INC-20/21/22). Each script ships with a `_test.sh` over temp repositories, run by the kit's `task scripts-check`.
- **`foundations/quality/check-catalog`** (experimental, 2026-09-09, fourth-consumer uplift RC-1): a gate's checks become catalog rows — `Severity` with blocking decided once, `Means` (deterministic | model), and `Reconcile` returning catalog↔implementation drift in both directions, so a consumer's catalog test is one call; `Run` orders by id and a check that cannot run says so (`Skipped`) instead of looking passed; **`foundations/review/capped-confidence`** (experimental, 2026-09-09, fourth-consumer uplift RC-2): a score bounded by the ceiling its evidence class permits, from a table loaded out of CUE (`#Table`, one `#Ceiling` per class with a mandatory `why`; classes are data, never Go constants), keeping the raw and rendering a capped score as a band ("≤50 — delimited-line evidence") rather than a point value; **`foundations/quality/structural-refusal`** (experimental, 2026-09-09, fourth-consumer uplift RC-3): a "meets" verdict with no citable evidence becomes "unknown" in the value itself (`Refuse`, `Normalize`, `New`), uncitable references are discarded before they are counted and the reason is recorded on the judgment, so the refusal survives every renderer and store downstream.
- `evidence-producer` catalogs findings/claims producers by kind, origin and tier cap so served tier never rises on import (`min(claimed, cap)`), corroboration counts only independent producer IDs, and a producer whose origin is the consuming system itself is refused (`Admit`); reuses the existing engineering evidence-tier ladder rather than redefining it.
- `intent-audit`'s `scripts/intent-stale.sh` (it ships inside the skill, so the team snapshot and a personal install carry it) lists the ten condition lines (`Revisit when`, `Promote when`, `Drop if`, `Wins if`) under `docs/intent/` and the decision records whose last change is oldest, by `git blame` author time, with age, `path:line` and text; git only, no fathom, deterministic, exit 0. It feeds the bounded review ([decision 0021](docs/intent/decisions/0021-intent-practice-v2-enforces-its-homes-adds-no-record-kind.md)).

### Changed

- `decisiontest.RunConformance` also checks one outcome per question, refusals (`Harness.Refusals`) and images: a Decider written outside the kit that answers a Request carrying `Images` without declaring `Capabilities.Images` now fails `ImagesOnlyWhenDeclared`; guardrail chains preserve structured injection matches and add `InjectDetectAction: "annotate"`, with blocking still default; invalid/repeated stages, invalid thresholds/actions and canceled runs fail closed ([workflow implementation](docs/plans/2026-09-governed-workflows.md)).
- Project intent admits bounded incubation and defines workflow assurance, precise prose traceability and reviewed lesson transfer ([direction](docs/designs/governed-workflows.md), decisions [0004](docs/intent/decisions/0004-bounded-incubation.md)–[0006](docs/intent/decisions/0006-evidence-and-lessons.md)).
- Fleet-facing specs now reflect shipped obligations/index freshness, defer unneeded ladder/batch/journal work behind evidence-based triggers, and retire obsolete proof-gate bypass instructions. The non-vacuous pre-push proof gate is exercised end to end ([H0](docs/plans/handoff-h0-index.md), [H4](docs/plans/handoff-h4-sextant.md), [H5](docs/plans/handoff-h5-eval-runner.md), [H7](docs/plans/handoff-h7-patterns-bench.md), [H8](docs/plans/handoff-h8-public-candidacy.md), [fathom asks](docs/plans/2026-07-fathom-asks.md)).
- **`AGENTS.md` is the one instruction file**: the contributor guide and stanzas live in `AGENTS.md` and this repo's `CLAUDE.md` is gone; `kit-new host install -host claude` now writes its managed stanza to `AGENTS.md`, as the Codex host does, and never creates a `CLAUDE.md`, which hid an adopter's `AGENTS.md` from Claude Code.
- `foundations/decision`: `Validate` no longer enforces Score's 2..10 levels, which is TypeSafe's vendor limit rather than a rule of the contract (it still requires at least 2 `Levels`, with Min/Max unset or matching them); `TypeSafe` now declares the limit in its `Capabilities` and refuses an out-of-limit question, explicit `Levels` or an integral range, into `Response.Invalid` by ID with a reason naming the limit, without sending it; the pre-push `fathom-verify` hook resolves `fathom` on PATH first (the deployed binary), then `FATHOM_BIN`, then the sibling `../fathom/bin/fathom` and `../sextant/bin/fathom` builds, and prints which one ran; a sibling build is only as current as the last compile in that checkout and had been checking plans with a September build; `constitution-lint`: the `org-agnostic` and `secret-scan` sweeps exempt only `.claude/worktrees/` (stale agent worktree copies), not all of `.claude/`, so the skills the kit ships from `.claude/skills` are scanned like any other file; a symlink to a directory is recorded as a skip rather than failing `read-coverage` as an unreadable file (the tree it names is swept at its real path).
- The `intent-*` skills live in `.claude/skills`, the one real copy (Claude Code discovers skills only there); `.agents/skills` is a relative link to it for Codex. The personal installer moves to `scripts/install-intent.sh`, and a link it installed earlier still resolves through the kit link. The snapshot updater runs from `.claude/skills/intent-init/scripts/vendor_intent.go`, still accepts a kit checkout from before the move, keeps the snapshot at `.agentic-kit/upstream/.agents/skills/` so existing manifests stay valid, and writes only the `.claude` links when an adopter's `.agents/skills` links to `.claude/skills` ([integration](docs/project-intent/integration.md#4-distributing-to-a-team-preferred-or-a-personal-machine)).
- Project Intent doctrine v2 ([decision 0021](docs/intent/decisions/0021-intent-practice-v2-enforces-its-homes-adds-no-record-kind.md)): a committed view needs a named reader and a `--check`; plans and records quote the request (`[org]` redacted) and no repository intent file is named `INTENT.md`; every rejected alternative carries `Wins if:` or `never`, and a declined owner request becomes a rejected record; futures may carry `Value` and `Basis` (README §13 amended for those two only); briefs name the governing record ids; monorepos cite `<scope> <id>` and gain an extraction checklist, a root routing table naming every component and a 32 KiB `AGENTS.md` chain budget.
- Project Intent templates: the decision template gains an optional `Request:` line (the deciding words verbatim and dated, `[org]` redacted) and requires `Wins if:` or `never` on every rejected alternative; the futures template gains optional `Value` and `Basis` lines, a measured Basis citing a path, instrument or commit ([decision 0021](docs/intent/decisions/0021-intent-practice-v2-enforces-its-homes-adds-no-record-kind.md)).
- `intent-*` skills: `intent-audit` gains the weekly bounded review (judge the ten oldest condition lines from its `scripts/intent-stale.sh`, record a fired one, commit each judged line); `intent-design` searches docs first (fathom docs search when present, else grep, about 1.5 KB); `intent-doctor` flags a bare id that crosses scope and an `AGENTS.md` chain over 32 KiB; `intent-decide` requires `Wins if:` or `never` and quotes the request; `intent-prune` moves a dropped future's reason into the record that dropped it ([decision 0021](docs/intent/decisions/0021-intent-practice-v2-enforces-its-homes-adds-no-record-kind.md)).

### Fixed

- Fitness registries snapshot nested checks on registration/read, reject NaN thresholds and add an injectable timing seam without changing scoring; eval-runner rejects missing, duplicate or foreign scenario outputs and mismatched producer model/prompt/parameter identities before probes, model calls or journal writes; strict prose and lesson decoders reject invalid UTF-8 and unpaired Unicode escapes before decoding can alter identities; sourced-prose Go copies/hashes also reject malformed identity/metadata strings; `NewPipelineWithClock` injects prose-regeneration/correction timestamps; shared telemetry keys move to `telemetry/event` while existing harness constants remain ([workflow implementation](docs/plans/2026-09-governed-workflows.md)); CLI adapters run the core conformance suite against fake shims with unsupported capabilities named; noop now records its actual partial conformance rather than claiming a full pass ([field absorption](docs/plans/2026-09-field-absorption.md)).
- Fathom hooks/downstream defaults support both `../fathom` and `../sextant`. `.fathomignore` replaces `.sextantignore` without changing exclusions, retaining worktree-pollution protection.

## [0.7.0] — 2026-07-16

### Added

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
