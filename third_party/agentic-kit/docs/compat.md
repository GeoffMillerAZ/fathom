# API stability audit + compatibility promise

This is the API-stability audit for agentic-kit's public-readiness work
(handoff-h8, Stage A). It enumerates every Go package under the five roots
that make up the kit's public surface — `foundations/`, `kits/`, `runtimes/`,
`surfaces/`, `toolchains/` (explicitly **not** `cmd/`, `examples/`,
`scenarios/`, or `specs/`, which are tooling/demos/acceptance-contracts, not
library surface) — classifies each as `stable` / `experimental` /
`internal-ish`, and states what each classification promises a consumer.

**Measurement method.** Enumerated live via `cmd/internal/catalog.Collect`
(the same walker `cmd/kit-catalog` and `cmd/constitution-lint`'s
`catalog-fresh` check already share), filtered to the five roots above. This
is more reliable than hand-listing with `ls` because `Collect` walks the
whole tree with `go/parser` and already flattens nested Go modules for free
— `kits/docextract`, `kits/docextract/pdf`, and `kits/docextract/xlsx` come
back as three separate entries even though `pdf`/`xlsx` are their own Go
modules.

**Result: 169 packages** across the five roots (foundations 148 · kits 13 ·
runtimes 2 · surfaces 2 · toolchains 4; the 169th —
`foundations/security/tokenization/vaulttest` — landed via the concurrent H3
wave during this audit's review and was absorbed on rebase, exactly the
drift `compat-coverage` exists to force). `docs/catalog.md`'s header is the all-areas count — it additionally includes
`cmd/` (8, including `cmd/release-notes`, which landed in the same Stage-A
change-set as this audit), `examples/` (2), and `scenarios/` (2)
sub-packages, which are correctly out of scope for this compatibility
promise. At audit time: 169 + 8 + 2 + 2 = 181; after the three same-day
post-audit adapter additions below (claudecli, awscred, codexcli):
172 + 8 + 2 + 2 = 184, confirmed against the regenerated catalog.

**Post-audit drift note (2026-07-16):** `foundations/llmclient/claudecli`
landed after this audit and is classified `experimental` below (zero
in-kit consumers yet; first subscriber is fathom, an external repo) —
the same kind of post-audit addition `vaulttest` was above, absorbed here
rather than re-running the whole audit. `foundations/llmclient/bedrock/awscred`
(2026-07-16, docs/plans/aws-credchain.md) landed the same day: the AWS
default credential-resolution chain (shared config/credentials files, SSO,
IMDSv2) the `bedrock` adapter now consumes for expiry-aware re-resolution —
classified `experimental` below (one named consumer, `bedrock`, per its own
package doc's D8 graduation trigger). The
classification-counts table below is left as the point-in-time sign-off
snapshot except for the two rows that changed.

**Second post-audit drift note (2026-07-16, same day):**
`foundations/llmclient/codexcli` landed immediately after claudecli,
modeled directly on it — same classification (`experimental`: zero
in-kit consumers, landed same-day, first subscriber is fathom, an
external repo) and the same absorb-rather-than-re-audit treatment. With
all three same-day additions (claudecli, awscred, codexcli) the live
library count is 172 (169 + 3); the classification-counts table is left
unchanged for the same reason stated above.

**Third post-audit drift note (2026-07-30):** `foundations/knowledge/catalog`
landed (docs/designs/catalog-kit.md, D13: 3 in-fleet consumers named:
this kit's own package catalog, a consumer's MCP tool catalog, and the
licensed component corpus). Classified `experimental` below: unlike the
other post-audit additions, one of its three named consumers IS proven
same-day (`cmd/kit-catalog`'s own catalog `Verify`s clean against
`docs/catalog.md` via `cmd/kit-catalog/consumer_gate_test.go`), but the
other two remain unwired, so it stays `experimental` rather than
`stable` until a second real consumer lands. The classification-counts
table is left unchanged for the same reason stated above.

## Classification counts (for sign-off)

| Classification | Count |
|---|---|
| `stable` | 124 |
| `experimental` | 30 |
| `internal-ish` | 24 |
| **Total** | **178** |

This is a real distribution, not a blanket stamp: 124 packages (70%) reflect
four waves of hardening with no churn and either a real consumer or a
foundational role nothing else has displaced; 30 (17%) are honestly flagged
as still-moving (plan-only designs, packages that landed in the last one to
two days, or kits whose composition TRACKING.md itself calls "unproven");
24 (13%) are real, tested, and useful but narrow/plumbing rather than a
primary extension point a new adopter would reach for first. (The counts are
recomputed from the rows below, not carried forward: the 2026-09-02
field-absorption merge found the summary reading 25 experimental against 28
actual rows, so it had drifted before this wave's two new rows landed.) (Three rows —
`runtimes/credresolve`, `foundations/evidence/record-fingerprint`,
`foundations/evidence/source-registry` — were re-audited against
throughline's HEAD after the first draft and promoted `internal-ish` →
`stable` when live imports falsified their "no consumer" rationale; two of
those imports landed the same day via the concurrent H3 adoption wave.)

**Flagged for sign-off — a live policy question, not silently resolved
here:** `kits/guardrails-pipeline` is classified `experimental` ("may change
API shape … at any time") yet as of 2026-07-16 it has a live, disclosed
downstream consumer: throughline wired `NewPreLLMChain` into
`app/chat/untrusted.go` (H3 Stage 1). An experimental classification with a
production consumer means that consumer has no compatibility promise. Geoff
should either promote the row to `stable` (accepting the promise) or
explicitly accept the consumer-on-experimental risk — the classification
table keeps `experimental` until that call is made.

**Methodology, stated plainly so any row can be challenged:**

1. **`experimental`** — the package carries a live, stated
   composition/API-stability risk: TRACKING.md or `docs/plans/2026-07-uplift.md`
   marks it "composition unproven" or "wiring pending"; it is `coordination/*`
   or `topology/live-graph` (explicitly plan-only per uplift-plan D7 — the
   executor, `kits/worker-pool`, does not exist yet); it is one of the four
   packages TRACKING.md's pull-queue section explicitly names as
   "grandfathered in-flight Wave 0" (`llmclient/bedrock`, `llmclient/noop`,
   `evidence/evidence-anchor`, `quality/fitness-function-registry`); or it
   landed within the last one to two days with zero consumers anywhere
   (in-kit or named external) and no roadmap-graduated marker.
2. **`internal-ish`** — real, tested, real judgment says its *purpose* is
   narrow/plumbing (a thin data holder, lookup catalog, or config loader)
   rather than a capability a consumer would compose directly, and no
   consumer (in-kit or named external) has reached for it yet.
3. **`stable`** (the default) — a genuine, complete, tested
   capability-bearing primitive (an engine, evaluator, protocol adapter,
   scorer, taxonomy, or interface meant for composition) that has existed at
   least one full wave with no churn, **regardless of whether a consumer has
   wired it yet** — this codebase's own stated philosophy is that
   foundations are "scaffolded breadth-first" ahead of the kits that will
   eventually compose them (TRACKING.md §1), so an unconsumed-but-substantial
   primitive is still `stable`; only a *thin* one drops to `internal-ish`.

## Classification table

| Package | Classification | Rationale |
|---|---|---|
| `foundations/agent/bounded-exec` | stable | Load-bearing resource-confinement seam since early waves; non-sandbox guarantees documented at length in VISION/practices, consumed by direct-investigator's optional `rg` engine. |
| `foundations/agent/conversation` | stable | Core multi-turn invariant-enforcement primitive underlying agent-harness; widened twice (Wave 2 `Compactor`) with no breaking churn to callers. |
| `foundations/agent/evidence-compactor` | stable | Roadmap #1 graduated. Corrected 2026-09 (FB-5/FB-6): the earlier rationale claimed it was "wired into agent-harness's `Config.ContextBudget` compaction hook", but nothing in the kit installed it — the hook only dropped messages. It is now installed by `agentharness.WithEvidenceCompactor` (which refuses a budget-less config) and consumed by `examples/agent-cli` behind `-context-budget`; the package's own contract (journal-before-summary, provenance marker) was always true. |
| `foundations/agent/mcp-client` | stable | Tool-poisoning defense (hash-pinning) is security-critical and heavily documented (practices §14); H6 added the HTTP transport without breaking the existing `Client`. |
| `foundations/agent/tool-registry` | stable | The single tool primitive; consumed by both agent-harness and mcp-server; lethal-trifecta taxonomy (practices §13) is load-bearing. 2026-09: additive `Tool.Effect`/`EffectOf` (the MCP annotation vocabulary; decision 0002) — no existing call site changes. |
| `foundations/cache/result-cache` | stable | Generic TTL+LRU+single-flight memoization; long-standing, no churn, the bounded-cache exemplar cited in VISION belief 4. |
| `foundations/communications/clarification-loop` | internal-ish | Thin open-question tracker; no consumer today; narrow by design, not a primary extension point. |
| `foundations/communications/message-capture` | stable | Real pluggable-`Channel` extension point with dedup/order guarantees (same pattern as `embedding-client`'s `Embedder`); survived the Wave 4 zero-consumer purge that retired its two siblings. |
| `foundations/contracts` | stable | Canonical CUE-first types shared platform-wide; Phase-1 vertical slice, zero churn since inception. |
| `foundations/coordination/convergence-detector` | experimental | `coordination/*` is explicitly plan-only per uplift-plan D7; executor (`kits/worker-pool`) does not exist. |
| `foundations/coordination/implementation-queue` | experimental | Same D7 plan-only status as the rest of `coordination/*`. |
| `foundations/coordination/result-aggregator` | experimental | Same D7 plan-only status as the rest of `coordination/*`. |
| `foundations/coordination/swarm-orchestrator` | experimental | Same D7 plan-only status; "spawns workers" language was removed Wave 4 precisely because it doesn't. |
| `foundations/coordination/task-tree` | experimental | Same D7 plan-only status as the rest of `coordination/*`. |
| `foundations/coordination/worker-heartbeat` | experimental | Same D7 plan-only status as the rest of `coordination/*`. |
| `foundations/cost/cost-tracker` | stable | Config-driven cost accounting; consumed by `llminstrument.WithCost` and eval-runner; a README headline capability. |
| `foundations/data/data-contract` | internal-ish | Thin CUE-validated data-layer wrapper; no consumer; narrow by design. |
| `foundations/data/embedding-provenance` | internal-ish | Thin provenance tracker for embeddings; no consumer today. |
| `foundations/data/schema-migration-ledger` | internal-ish | Thin append-only ledger; no consumer today. |
| `foundations/decision` | experimental | Landed 2026-09-22 (docs/plans/decision-port.md): typed fast-decision port (Choice/Score/Noul questions, mirroring TypeSafe AI's System One primitives) with `None`, `LLM`, and `TypeSafe` adapters — `TypeSafe` (DP-10/DP-11, same day) calls TypeSafe's System One API directly, promoted from parked once fathom (a real workload) pulled for it on owner direction; `Question.Levels` gives `KindScore` an explicit or synthesized ordered level list. Zero in-kit consumers; the exported API is a contract external consumers are coding against concurrently, so it is not a speculative package — see D13's own "second named consumer" trigger, tracked the same way as claudecli/codexcli's same-day landings above. |
| `foundations/embedding/embedding-client` | stable | The single embedding interface; `CachingEmbedder`'s bounded LRU is the VISION belief-4 exemplar; consumed by http-embedder, vector-store, and eval flows. |
| `foundations/embedding/http-embedder` | stable | Real OpenAI-compatible adapter, hermetic httptest-tested, key-never-in-errors proven. |
| `foundations/embedding/vector-store` | stable | Lifted-and-generalized from a real prior system (fathom); mature, tested; fathom's own adoption is deferred pending its branch merge, not the package's readiness. |
| `foundations/environment/access-broker` | internal-ish | Thin generic credential-negotiation shim; no consumer today. |
| `foundations/environment/environment-registry` | internal-ish | Thin named catalog of runtime environments; no consumer today. |
| `foundations/environment/platform-config` | internal-ish | Thin config loader+validator; no consumer today. |
| `foundations/environment/tool-execution-policy` | stable | Deny-by-default policy consulted before every tool dispatch via the registry's `Options.Policy` hook; security-load-bearing (practices §5/§9/§13). |
| `foundations/evidence/assertion` | stable | Speech-act primitive underpinning the evidence moat; real fan-in (conflict, case-file). |
| `foundations/evidence/authenticity` | stable | Real named consumer: throughline's auth-header grading; per-method score ceilings closed D35-4. |
| `foundations/evidence/case-file` | stable | Roadmap #25 graduated; throughline UI is the named subscriber. |
| `foundations/evidence/conflict` | stable | Evidence-moat keystone; Wave 3 fixed its join logic with a regression fixture mirroring a real consumer's disjoint ID scheme. |
| `foundations/evidence/direct-investigator` | stable | Part of the deliberate D8/D10 Tier-2 evidence-acquisition system (with evidence-ladder/evidence-signature/extraction-gap-log); rich, tested guarantees (skip accounting is the guarantee). |
| `foundations/evidence/doc-provider` | stable | Real interface (`DocProvider`) plus two implementations meant for composition, same pattern as `Embedder`/`Channel`. |
| `foundations/evidence/entity-crosswalk` | internal-ish | Thin alias-to-canonical identity map; no consumer today. |
| `foundations/evidence/entity-resolution` | stable | Entity-match candidates with a real safety property (never auto-merges; human review required); 26 tests. |
| `foundations/evidence/evidence-anchor` | experimental | One of the four packages TRACKING.md explicitly names as grandfathered in-flight Wave 0. |
| `foundations/evidence/evidence-grade` | stable | The "#1 truth-seeking primitive" per practices.md; Phase-1 vertical slice; heavy fan-in. |
| `foundations/evidence/evidence-graph` | stable | Frozen/legacy by design: real consumer (`kits/investigation`) but explicitly the legacy `Span` shape per `foundations/evidence/README.md`; migration to `evidence-span` assessed not-cheap and deferred — no new pipelines may build on it, which is itself a stability property (it won't move). |
| `foundations/evidence/evidence-ladder` | stable | The D8/D10 Tier1/2/3 evidence-acquisition ladder; deterministic tier progression with fail-closed `ABSENT_UNVERIFIED` semantics — a rich, deliberate moat component. |
| `foundations/evidence/evidence-producer` | experimental | 2026-09-18 producer catalog for findings/claims crossing into a consuming store (kind, origin, tier cap); first consumer is fathom's findings loop (ADR-040); shape may move as a second producer kind lands. |
| `foundations/evidence/evidence-resolver` | stable | Deterministic span-reference registry; real fan-in. |
| `foundations/evidence/evidence-signature` | stable | Per-concern evidence contract (D8); mandatory-canary + coverage-manifest guarantees are substantial, not thin. |
| `foundations/evidence/evidence-span` | stable | The canonical bitemporal evidence unit with full provenance; heaviest fan-in in the evidence category. |
| `foundations/evidence/extraction-gap-log` | stable | Flywheel store completing the D8/D10 Tier-2→Tier-3 escalation system; deliberate, not incidental. |
| `foundations/evidence/record-fingerprint` | stable | Content-hash dedup; consumed by throughline production code (`app/ingest/fingerprint.go`) as of 2026-07-16 — the concurrent H3 adoption wave landed this import the same day this audit was first drafted (stale-on-arrival, corrected on re-audit). |
| `foundations/evidence/source-linker` | internal-ish | Thin span↔source link-set utility; no consumer today. |
| `foundations/evidence/source-registry` | stable | Source catalog; consumed by throughline production code (`app/evidenceacquisition/sourceregistry.go`, `tier2.go`) as of 2026-07-16 — same concurrent-H3 stale-on-arrival correction as record-fingerprint. |
| `foundations/governance/answer-cache` | stable | Two-level match cache for LLM answers; real fan-in, a cost-saver used in practice. |
| `foundations/governance/budget-guard` | stable | The bill cap for agent workflows; heaviest fan-in in governance; VISION belief-4 exemplar. |
| `foundations/governance/circuit-breaker` | stable | Closed/open/half-open breaker; real fan-in, mature. |
| `foundations/governance/identity` | stable | Privacy-preserving identity extraction; real fan-in in the governance gauntlet. |
| `foundations/governance/identity-quota` | experimental | Landed alongside this audit's follow-on work (per-identity rolling-day quota, `budget-guard`'s companion); zero named in-kit consumers yet. |
| `foundations/governance/proof-of-work` | stable | Redesigned to be unforgeable (HMAC-signed challenges); bounded `SeenSet`; real fan-in. |
| `foundations/governance/rate-limiter` | stable | Token-bucket limiter; real fan-in, no churn since inception. |
| `foundations/governance/retention-policy` | stable | Roadmap #18 graduated; the deletion-story companion to `tokenization`'s crypto-shred; the foundation decides, callers execute, by design. |
| `foundations/governance/strike-ledger` | stable | Append-only violation ledger with escalating backoff; real fan-in. |
| `foundations/governance/topic-gate` | stable | Cheapest-first in-scope decision before paying for a model call; real fan-in. |
| `foundations/investigation/convergence-oracle` | stable | Real fan-in; core investigation-moat FSM. |
| `foundations/investigation/coverage` | stable | Per-source completeness ledger; the canonical mechanism now also absorbing `source-sync-checkpoint`'s retired intent — 21 tests. |
| `foundations/investigation/dad-investigator` | stable | Deterministic DAD FSM over evidence; real fan-in. |
| `foundations/investigation/element-coverage` | stable | Claim/theory coverage board; substantial, not thin — 12 tests. |
| `foundations/investigation/hypothesis-ranker` | stable | Deterministic support-sorted ranking; real fan-in. |
| `foundations/investigation/investigation-journal` | stable | Append-only step journal with sinks; heaviest fan-in in the investigation category. |
| `foundations/investigation/investigation-memo` | stable | Real `Store` interface awaiting a SQLite backend by documented design, not neglect. |
| `foundations/investigation/investigation-template` | internal-ish | Thin named-step-sequence holder; no consumer today. |
| `foundations/investigation/lead` | stable | Provenance-bearing transition-history FSM; 24 tests, substantial. |
| `foundations/investigation/narrative-shift` | stable | Real consumer (`case-file`); deposition-prep "prior inconsistent statement" gold. |
| `foundations/investigation/query-planner` | stable | Real named external consumer: throughline's `app/acquire` swap completed H2 Wave B, donor code deleted, kit core proven-shape. |
| `foundations/knowledge/authority` | stable | Legal authority as a first-class node; 41 tests, substantial. |
| `foundations/knowledge/catalog` | experimental | Landed 2026-07-30 (docs/designs/catalog-kit.md, D13: 3 named in-fleet consumers). One consumer proven for real same-day (`cmd/kit-catalog`'s own 190-package catalog, gated via `Verify` in `cmd/kit-catalog/consumer_gate_test.go`); the licensed-component-corpus and MCP-tool-catalog consumers named in the design are not yet wired. |
| `foundations/knowledge/document-parsing` | stable | Heading-segment parser; real fan-in. |
| `foundations/knowledge/knowledge-card-generation` | stable | Real generation engine (archetype template fill over contracts), not thin glue. |
| `foundations/knowledge/memory-manifest` | internal-ish | Thin append-only manifest; no consumer today. |
| `foundations/llmclient` | stable | Phase-1 vertical slice; the single LLM interface; heaviest fan-in in the repo. |
| `foundations/llmclient/anthropic` | stable | First real provider; README headline; long-standing, 24 tests, real fan-in. |
| `foundations/llmclient/bedrock` | experimental | One of the four packages TRACKING.md explicitly names as grandfathered in-flight Wave 0. |
| `foundations/llmclient/bedrock/awscred` | experimental | Landed 2026-07-16 (docs/plans/aws-credchain.md); AWS default credential-resolution chain (shared config/credentials files, SSO, IMDSv2) for the `bedrock` adapter. One named consumer (`bedrock`) by design — D8 forbids speculative generality; graduates to a shared foundations package only when a second consumer names the need (recorded in the package doc). |
| `foundations/llmclient/claudecli` | experimental | Landed 2026-07-16; shells out to the `claude` CLI so a Claude Pro/Max subscription substitutes for `ANTHROPIC_API_KEY`. Zero in-kit consumers; first subscriber is fathom's semantic-layer compile provider seam (external repo, not yet composed here). **Accepted 2026-09-02: consumer-on-experimental.** fathom's compile lanes wire this adapter (`cmd/fathom/llmprofile.go`, with `cmd/fathom/compile.go`), so a real downstream consumer runs without a compatibility promise — recorded, not silently resolved. Promotion to `stable` is the owner's call (`repo.md` §Promote); the trigger has fired. |
| `foundations/llmclient/codexcli` | experimental | Landed 2026-07-16, same day as claudecli and modeled directly on it; shells out to the `codex` CLI so a ChatGPT Plus/Pro subscription substitutes for `OPENAI_API_KEY`. Zero in-kit consumers; first subscriber is fathom's semantic-layer compile provider seam (external repo, not yet composed here). **Accepted 2026-09-02: consumer-on-experimental.** fathom's compile lanes wire this adapter (`cmd/fathom/llmprofile.go`, with `cmd/fathom/compile.go`), so a real downstream consumer runs without a compatibility promise — recorded, not silently resolved. Promotion to `stable` is the owner's call (`repo.md` §Promote); the trigger has fired. |
| `foundations/llmclient/egress` | stable | Gate-before-egress middleware generalized from a real prior system (throughline's `GatedClient`); mature design lineage. |
| `foundations/llmclient/instrument` | stable | `WithTelemetry`/`WithCost`/`WithBudget`; throughline deleted its hand-rolled ~62-line equivalent in favor of this. |
| `foundations/llmclient/llm-test` | stable | The conformance suite every `LLMClient` implementation must pass; anthropic/replay/bedrock/noop/openai all depend on it. |
| `foundations/llmclient/model-catalog` | stable | Structurally required by eval-runner's judge-independence gate (`ValidateIndependence`); the shipped catalog *data* is deliberately example-only, but that is a data-completeness note, not an API-stability concern. |
| `foundations/llmclient/noop` | experimental | One of the four packages TRACKING.md explicitly names as grandfathered in-flight Wave 0. |
| `foundations/llmclient/openai` | stable | Roadmap #10 graduated; full conformance suite passing; the cross-family judge lane D9 requires. Azure OpenAI covered (2026-07-16, docs/plans/azure-openai.md): `Config.AzureDeployment`/`AzureAPIVersion`/`AzureAuthScheme` extend the same package, vanilla path byte-preserved. |
| `foundations/llmclient/replay` | stable | Record/replay cassette infra used across nearly every test suite in the repo; canonical-hash v2 shipped with backward compatibility. |
| `foundations/llmclient/structured` | experimental | Landed 2026-07-16 (fathom-asks FA-2/FA-3); zero in-kit consumers yet; first subscriber is an external repo not yet composed here. |
| `foundations/orchestration/express-lane` | internal-ish | Thin size-check + spec-challenge fast-path gate; no consumer today. |
| `foundations/orchestration/improvement-flywheel` | stable | Real fan-in; deterministic signal-to-proposal accumulator. |
| `foundations/orchestration/materiality-cache` | internal-ish | Freshness-keyed cache utility; real but narrow, no consumer today. |
| `foundations/orchestration/operator-elicitation` | experimental | Explicitly demoted per D2: "composition unproven" language added Wave 4 (throughline's richer elicitation shape is domain-specific). |
| `foundations/orchestration/planning-refinement` | stable | Named SHIPPED in practices.md §3 (bounded refine rounds, `Escalated=true`); core escalation-structure primitive. |
| `foundations/orchestration/probe-runtime` | internal-ish | Thin ordered-probe runner with panic recovery; no consumer today. |
| `foundations/orchestration/workflow-engine` | stable | FSM/DAG engine; real fan-in; dead-policy fields honestly removed Wave 4 rather than left inert. |
| `foundations/persistence/jsonl-log` | stable | Append-log writer/reader with torn-tail recovery; heaviest fan-in outside llmclient/evidence. |
| `foundations/prompt/prompt-template` | stable | Versioned template rendering producing the reproduction hashes AI-provenance requires; gained `TextHash()` (fathom-asks FA-1) without breaking existing callers. |
| `foundations/prose/context-renderer` | stable | Budget-bounded segment assembly; core "1.15 prose (core)" primitive, substantial not thin. |
| `foundations/prose/evidence-citation` | stable | Deterministic citation render/parse round trip; second-consumer-driven (throughline `parseCitations`), real fan-in. |
| `foundations/prose/prose-as-code` | stable | Core structured-prose type primitive underlying the whole prose category. |
| `foundations/prose/prose-plan` | stable | Core structured section-plan type used before any prose generation begins. |
| `foundations/prose/prose-policy` | stable | Configurable DLP posture evaluation engine; real fan-in. |
| `foundations/prose/traceable-prose` | stable | Annotation/redaction-provenance/regen-marking primitive; real fan-in. |
| `foundations/provenance/ai-provenance` | stable | The authorship record attached to every AI/human artifact; among the heaviest fan-in in the repo. |
| `foundations/provenance/anchor` | stable | Merkle proof-of-existence over content-hash batches; real fan-in (evidence-anchor). |
| `foundations/provenance/event-store` | stable | Append-only hash-chained system of record; Wave 3 added JSONL persistence without breaking the in-memory contract. |
| `foundations/quality/agentic-practices-evaluator` | stable | Explicitly designated in `docs/practices.md`'s closing section as "the scorecard seam" for self-audit — a deliberate, documented extension point even without a named user yet. |
| `foundations/quality/correction-estimator` | internal-ish | Thin effort/likelihood scorer; no consumer today. |
| `foundations/quality/determinism-gate` | stable | Structural + variance-based determinism checking; real fan-in (eval-runner). |
| `foundations/quality/diff-reporter` | stable | One primitive, two roles (Diff/TrajectoryDiff); real fan-in (eval-runner regression gate). |
| `foundations/quality/dlp-evaluator` | stable | Configurable DLP risk-band engine; real fan-in. |
| `foundations/quality/eval-gate` | stable | Deterministic weighted-check gate; heavy fan-in (eval-runner, fitness-function-registry). |
| `foundations/quality/eval-journal` | stable | Append-only judge-call truth record; real fan-in (eval-runner). |
| `foundations/quality/fitness-function-registry` | experimental | One of the four packages TRACKING.md explicitly names as grandfathered in-flight Wave 0. |
| `foundations/quality/honesty-gate` | stable | Deterministic overclaiming detector; real fan-in (guardrails-pipeline). |
| `foundations/quality/integrity-evaluator` | stable | Fabricated/unresolvable-reference checker; real fan-in (guardrails-pipeline). |
| `foundations/quality/qa-harness` | stable | The `Judge` interface + `RubricJudge`/`MockJudge`; real fan-in, compiled into eval-runner's scenario bank. |
| `foundations/quality/reliability-stats` | experimental | Landed 2026-09-02 (field-absorption FB-11..13): pass@k, pass^k, task-clustered bootstrap CI, Wilcoxon, A12. One in-kit consumer (eval-runner `Regression` pass^3); fathom's bench is the named external one. |
| `foundations/quality/scenario-bank` | stable | Provenance-disciplined eval scenarios; real fan-in (eval-runner). |
| `foundations/reasoning/defeasible` | stable | Complete, substantial "effective state" engine (layered typed modifiers, full trace + confidence); 33 tests; not thin glue, awaiting a real workload the same way several evidence-moat pieces did before their consumers arrived. |
| `foundations/retrieval/evidence-ranker` | stable | Roadmap #24 graduated; LLM-free deterministic reranker for the RAG-that-cites layer. |
| `foundations/retrieval/fuzzy` | stable | Real, substantial matching/ranking algorithm (word-boundary + contiguous-run scoring, Unicode-aware); complete and tested, not thin. |
| `foundations/retrieval/intent-classifier` | internal-ish | Thin configurable keyword-rule classifier; no consumer today. |
| `foundations/retrieval/retrieval-metrics` | stable | Roadmap #24 graduated; Precision/Recall/MRR for the evidence-spine RAG layer. |
| `foundations/retrieval/thread-resolver` | internal-ish | Thin message-to-thread grouping utility; no consumer today. |
| `foundations/retrieval/truth-bridge` | stable | Roadmap #24 graduated; the span↔chunk link layer joining the evidence spine to vector retrieval. |
| `foundations/review/claim-lifecycle` | stable | Named SHIPPED in practices.md §2 (proposed→supported/refuted/uncertain); core evidence-linking primitive. |
| `foundations/review/concern-flags` | stable | Real fan-in; severity-ranked flag registry used across the review pipeline. |
| `foundations/review/confidence-scoring` | stable | Named SHIPPED in practices.md §2 (tier confidence ceilings); core anti-overclaiming mechanism. |
| `foundations/review/correction-ledger` | stable | Phase-1 vertical slice; the finding-lifecycle FSM; real fan-in. |
| `foundations/review/correction-ledger-analytical` | internal-ish | Narrow Parquet-export projection of correction-ledger; no consumer today. |
| `foundations/review/intake-triage` | stable | Real named external consumer: throughline's `app/acquire` swap completed H2 Wave B, donor deleted, kit core proven-shape. |
| `foundations/review/review-queue` | stable | Deterministic priority/insertion-order queue; real fan-in. |
| `foundations/review/run-audit-record` | internal-ish | Thin append-only per-run audit record; no consumer today. |
| `foundations/review/run-report` | stable | Roadmap #20 graduated; complete deterministic Markdown renderer over journal/telemetry/cost-tracker inputs, ready for consumer wiring. |
| `foundations/review/scoring` | stable | Named SHIPPED in practices.md §2 (`TierCeiling`/`ApplyCeiling`); rank-vs-ceiling asymmetry deliberately documented and pinned. |
| `foundations/schema/schema-evolver` | stable | Real algorithm (versioned migration graph, BFS compat-path check), not thin glue; complete and tested. |
| `foundations/schema/taxonomy-manager` | stable | Real cycle-rejecting hierarchy engine for org-configurable taxonomies; not thin glue. |
| `foundations/security/data-sensitivity-types` | stable | Core sensitivity-classification type primitive; real fan-in. |
| `foundations/security/httpauth` | experimental | Landed 2026-07-29 (D13, ahead of a second consumer): bearer-token HTTP middleware, a clean re-implementation of the pattern proven in fathom's module host (`cmd/fathom/servehttp.go`, GKE-MH-7) — not imported. Zero in-kit consumers yet; the hosted expertchat service is the intended first one once it grows a serving mode. Same same-day-addition treatment as claudecli/codexcli/expertchat above. |
| `foundations/security/injection-detector` | stable | Phase-1 vertical slice; recall uplifted to 22/22 on the seeded corpus; heaviest security fan-in. |
| `foundations/security/instruction-hierarchy` | stable | System>developer>user>untrusted precedence; small, stable, extensively discussed in VISION/practices as a real shipped guarantee. |
| `foundations/security/output-integrity-checker` | internal-ish | Thin required-disclaimer/banned-token checker; no consumer today. |
| `foundations/security/pre-llm-dlp-gate` | stable | Hardened (fuzz-tested, `ExtendedPolicy` high-precision shapes); real fan-in. |
| `foundations/security/redaction-executor` | stable | Fuzz-hardened to a true fixpoint after a real bug was found (single-pass PII leak); mature. |
| `foundations/security/rule-core` | stable | The shared engine under all four security/quality regex engines; real fan-in, all four callers' tests pass unmodified through the refactor. |
| `foundations/security/sensitivity-classifier` | stable | Org-configurable CUE catalog classifier; real fan-in. |
| `foundations/security/token-lifecycle` | stable | Real named external consumer: throughline's `adapters/oauth` rewire closed H2 Wave B, donor logic deleted. |
| `foundations/security/tokenization` | stable | Pure-logic sensitive-data tokenization + vault; Wave 3 added crypto-shred (`EncryptedVault`/`ShreddableVault`) without breaking existing `Vault` consumers. |
| `foundations/security/tokenization/vaulttest` | experimental | Importable shred-conformance suite (`RunShredConformance`/`VaultFactory`), extracted 2026-07-16 (H3 Stage 2) for downstream vault adapters; the named consumer (throughline's durable SQLite vault) has not landed yet — per methodology rule 1 (just landed), classified experimental until it does. |
| `foundations/security/untrusted-content-wrapper` | stable | Nonce-fence defense-in-depth; redesigned Wave-era to close a spoofing gap (BREAKING, documented); real fan-in. |
| `foundations/telemetry/event` | stable | Canonical event shape + W3C trace carrier; among the heaviest fan-in in the repo. |
| `foundations/telemetry/heartbeat` | experimental | Landed 2026-07-16 (fathom-asks FA-4/FA-5); zero in-kit consumers; first subscriber is an external repo, second subscriber (this repo's own proof-gate liveness) not yet wired. |
| `foundations/telemetry/read-manifest` | experimental | Landed 2026-09-02 (field-absorption FB-8): one in-kit consumer (`cmd/constitution-lint`'s `read-coverage` check, FB-9); the second, fathom's `internal/readset`, arrives by pin bump in the sibling repo — promote once that bump lands and the sibling deletes its copy. |
| `foundations/telemetry/metrics` | stable | Prometheus text-format registry; real fan-in. |
| `foundations/telemetry/otel-genai` | stable | Complete, standard-conformant OTel GenAI semconv adapter implementing the stable `sink.Sink` interface; no in-tree wiring yet is a consumer choice, not an incompleteness. |
| `foundations/telemetry/sink` | stable | The `Sink` interface + Multi/Memory/Filter implementations; heaviest fan-in in telemetry. |
| `foundations/topology/live-graph` | experimental | Explicitly plan-only per uplift-plan D7; plans wave batches but does not spawn or execute workers (language corrected Wave 4). |
| `foundations/topology/workspace-materialization` | experimental | Exists only to consume a `live-graph` topology, which is itself plan-only (D7); shares that status by direct dependency. |
| `kits/agent-harness` | stable | The flagship governed agent loop; 65 tests; named consumers `examples/agent-cli`, `scenarios/governed-docextract-hitl`, `cmd/kit-new` generated output. |
| `kits/command-center` | experimental | TRACKING.md explicitly: "Composition unproven — no in-fleet consumer yet." |
| `kits/docextract` | stable | Real named external consumer: growing-gardens-bi (assessed; consumer-side D1 adaptation pending, not a kit-side gap). |
| `kits/docextract/pdf` | stable | Nested-module format handler for the same real consumer story as `kits/docextract`; own release cadence (see v1 decision record). |
| `kits/docextract/xlsx` | stable | Nested-module format handler for the same real consumer story as `kits/docextract`; own release cadence (see v1 decision record). |
| `kits/endpoint-guard` | stable | Real consumer: the `cmd/endpoint-guard` production sidecar. |
| `kits/eval-runner` | experimental | Stage 1 landed 2026-07-15; single self-hosted subscriber so far (this repo's own `task eval`); Stages 2-3 (judge pool/panel/escalation) explicitly parked. |
| `kits/eval-runner/gen-cassettes` | internal-ish | An internal `package main` generator for regenerating eval-runner's own committed test fixtures; not an importable API surface at all. |
| `kits/expertchat` | experimental | New this wave (`docs/plans/gke-expertchat.md`, agentic-kit repo side of the governed-knowledge-engine wave), critic-reviewed (MERGE-WITH-FIXES, applied) and wire-reconciled 2026-07-28 against fathom's landed `module_query` tool (both same-wave passes): the citation-mandatory answer loop over a fathom module host. Zero in-fleet consumer yet — `examples/expertchat-service` is a composition example, not a consumer. `wire.go` speaks fathom's REAL `tools/call` JSON-RPC envelope (verified against `sextant/internal/mcp/dispatch.go`) invoking the PINNED `module_query` v0 contract (`{text, module?, max?}` → `{results: [...]}`, field-for-field reconciled with fathom's own round-trip test) — no remaining wire assumption. |
| `kits/governed-prose-kit` | experimental | TRACKING.md explicitly: "Composition unproven — no in-fleet consumer yet"; adoption sketch recorded but not wired. |
| `kits/guardrails-pipeline` | experimental | TRACKING.md said "wiring pending" when this audit was drafted; throughline wired it the same day (`app/chat/untrusted.go`, H3 Stage 1). Classification deliberately NOT silently upgraded — see the flagged policy question in the sign-off section above. |
| `kits/hitl` | stable | Real in-repo composition proof (`scenarios/governed-docextract-hitl`); growing-gardens-bi assessed adopt-worthy-in-principle with a stated subscribe-when trigger, not a red flag against the kit. |
| `kits/investigation` | stable | Real in-repo composition proof (`scenarios/governed-investigation`); migrated to canonical `evidencespan.Span` this wave, not flagged "composition unproven" unlike its sibling kits that are. |
| `kits/mcp-server` | experimental | TRACKING.md explicitly: "Composition unproven — serving side has no in-fleet consumer yet," even after H6's Streamable HTTP addition. |
| `runtimes/credresolve` | stable | Real, tested (15 tests) credential resolution; consumed by throughline (`adapters/claudeauth/resolver.go`) since 2026-06-11 — the initial "no consumer" draft of this row was falsified by re-audit against the consumer's HEAD. |
| `runtimes/policy` | internal-ish | Thin runtime `Enforcer` wrapper over the `tool-execution-policy` foundation; no consumer today. |
| `surfaces/editor` | internal-ish | Real, tested (10 tests) `ResolvedForm`→editor rendering layer; no consumer beyond its sibling `cueform`, narrow scope. |
| `surfaces/forms/cueform` | stable | Two-file form engine; 40 tests, mature; real fan-in (consumed by `surfaces/editor`). |
| `toolchains/jsonschema` | stable | Roadmap #16 graduated; closes `llmclient`'s `OutputSchema`→JSON-Schema mapping end to end; real fan-in. |
| `toolchains/materialization/workspace-materializer` | stable | Reference-ported, mature, completes the materialization toolchain pair with `workspace-query-builder`. |
| `toolchains/materialization/workspace-query-builder` | stable | Reference-ported fluent DSL; real fan-in (`workspace-materializer`). |
| `toolchains/schemas` | internal-ish | The canonical CUE schema root, real and tested, but genuinely unconsumed today — no other `schema.cue` or Go package imports it yet, Go or CUE side. |

## The compatibility promise

**Pre-1.0 (current):**

- `stable` packages may still change, but any breaking change ships with (1)
  a documented migration note in `CHANGELOG.md`'s Changed/Removed section
  explaining what changed and how to adapt, and (2) `TRACKING.md` updated in
  the same commit. **No silent breaking changes** — "silent" is the failure
  mode this promise rules out, not "never breaks."
- `experimental` packages may change API shape, be renamed, retired, or have
  their behavior redefined at any time without ceremony — though even
  experimental changes should still get a `CHANGELOG.md` line, because
  honesty about churn is cheap and searchability matters.
- `internal-ish` packages follow the `stable` no-silent-breakage promise (they
  are real and tested, not toys) but are not advertised as primary
  entry points; a package may be promoted to `stable` (noted in CHANGELOG)
  once a real consumer or roadmap purpose lands, or retired per the existing
  zero-consumer doctrine if one never does.

**At 1.0:** strict semver applies to every `stable` package. Any breaking
change to a `stable` package's exported Go API, typed-error taxonomy, or
`schema.cue` contract requires a major version bump. `experimental` and
`internal-ish` packages remain outside the semver promise until they are
promoted — promotion itself is a documented, one-way event recorded in
`CHANGELOG.md` and this file.

**Typed-error compatibility.** Sentinel values and typed error structs
exported by `stable` packages (e.g. `llmclient.RateLimitError`,
`llminstrument.ErrBudgetExceeded`, `toolregistry.ErrTrifectaBlocked`,
`tokenization.ErrShredded`) are part of the contract. `errors.Is`/`errors.As`
matching behavior will not silently change. Adding a **new** typed error for
a previously-unclassified failure mode is additive (non-breaking); changing
what an **existing** sentinel means, or removing one, is breaking and follows
the same-version-pre-1.0 / major-bump-at-1.0 rule above.

**CUE-schema compatibility.** For `stable` packages that ship a `schema.cue`,
changes to that schema (field removal, type narrowing, newly-required
fields) are breaking changes under this same promise — CUE owns the *shape*
(VISION belief 1), so a shape change is exactly the kind of API change this
promise governs, even when no Go signature moves. `task cue-vet` (inside
`task validate`) already vets every `schema.cue` and its `testdata/`
fixtures on every change, so a schema-breaking commit fails the gate before
it can land silently.

## The v1 decision record

- **No `/v1` module path suffix.** Go requires a module path suffix only at
  v2 and above; a `v1.0.0` tag on `github.com/geoffmilleraz/agentic-kit` is
  a pure tag decision that changes nothing about the import path. Do not
  "fix" this later by adding `/v1` to the module path — doing so would
  itself be the breaking change this promise exists to prevent.
- **Nested modules get their own tags.** `kits/docextract/pdf` and
  `kits/docextract/xlsx` each carry their own `go.mod` (D1 — moved there
  specifically so their third-party dependency trees never touch the root
  module's zero-`require` constitution). Consequently each gets its own
  independent `kits/docextract/{pdf,xlsx}/vX.Y.Z` tag on its own release
  cadence, separate from the root module's tags. A root `v1.0.0` does not
  imply either nested module is also at `v1.0.0` — they are versioned,
  released, and promised independently.
- **The tracked `go.work` stays as-is.** It lists the root module plus the
  two nested modules only — no sibling-repo paths — and exists purely for
  local multi-module development ergonomics inside this one repo (so
  `go build ./...` and editor tooling see all three modules at once, and
  `task downstream-check` can build against kit HEAD). It is not how
  external consumers should resolve the nested modules: they use a plain
  `require` (once real tags exist for the nested modules) or an explicit
  `replace` directive, never a workspace file of their own pointed at this
  repo.
