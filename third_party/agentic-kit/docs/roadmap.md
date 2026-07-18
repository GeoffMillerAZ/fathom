# Roadmap — known patterns awaiting a subscriber

This is the kit's **pre-thought future**: patterns we already know agentic
workloads will need, designed far enough to be claimable, deliberately NOT
built yet. Per [VISION.md](../VISION.md), capabilities land when a real
workload pulls for them — a half-baked primitive that demos well is the most
dangerous artifact in a foundation.

**How to use this doc when starting a new workload:** scan the entries. If
your project needs one, you are its **subscriber**: say so, and the pattern
gets built against your real requirements under the junk gate (contract,
deterministic `-race` tests, honest non-guarantees, gate green). If you need
something that is *not* here, that's a signal too — add the entry or build it.

Relationship to the other records: [TRACKING.md](../TRACKING.md) is the audit
log of what IS built and tested; this file is what we KNOW we'll likely need;
[VISION.md](../VISION.md) is the doctrine both serve. Entries graduate from
here into TRACKING.md when a subscriber lands them. Nothing here is a
commitment.

Each entry: **What** → why it serves the vision → **Exists today** (the pieces
already in the kit) → **Subscribe when**.

---

## Context engineering

### 1. Evidence-preserving compactor
**✅ Graduated — Wave 2 (v0.4.0).** Landed as `foundations/agent/evidence-compactor`
(package `evidencecompactor`): journals dropped messages verbatim (jsonl-log,
caller-supplied writer) BEFORE the LLM summary call; versioned prompt-template
with `PromptHash`/`ParamsHash` provenance; structured `[COMPACTION: …]` marker
+ journal ref in summary; LLM failure → never a lossy fallback. `Compactor`
signature on `conversation` widened to `(ctx, dropped []Message) (Message, bool, error)`
to carry the error channel. Closes practices.md §1. See TRACKING.md §1.22.

### 2. Generalized context assembler
**What:** budget-aware assembly of heterogeneous content (retrieved chunks,
journal entries, memos, tool docs) into a prompt under a token budget with
declared priorities and deterministic truncation order — `prose/context-renderer`
generalized beyond prose segments, with a pluggable token-estimator seam
(chars/4 today; a real tokenizer adapter when accuracy matters).
**Exists today:** context-renderer, cost-tracker estimator seam, conversation
windowing.
**Subscribe when:** the first RAG-grade workload where "what goes in the
prompt" is a ranked decision, not a concatenation.

## Memory & knowledge

### 3. Cross-session agent memory
**What:** an episodic/semantic memory composition: write policy (what's worth
remembering), retrieval-into-context, consolidation/summarization with
provenance, decay/expiry. Built from vector-store + jsonl-log + knowledge
cards — the missing composition, not new storage.
**Exists today:** all the ingredients (embedding, vector-store, jsonl-log,
knowledge-card-generation, memory-manifest as a receipt log).
**Subscribe when:** an assistant must remember across sessions ("what did we
learn about X last week").

### 4. Knowledge consolidation flywheel
**What:** a periodic pipeline distilling journals/transcripts/evidence into
validated knowledge cards (archetype-checked, provenance-stamped), replacing
re-derivation with recall.
**Exists today:** knowledge-card-generation, investigation-journal sinks,
ai-provenance, improvement-flywheel as the loop shape.
**Subscribe when:** a workload accumulates runs whose lessons are being
re-learned.

## Evaluation & regression

### 5. eval-runner (prompt/behavior regression harness)
**What:** CUE-defined eval cases + datasets, a runner over `llmclient`, an
LLM-as-judge implementing `qaharness.Judge`, JSONL baselines, regression
diffing, `eval-gate` thresholding — CI for prompts.
**Exists today:** qa-harness, eval-gate, replay cassettes (deterministic
inputs), prompt-template (versioned prompts to regress).
**Subscribe when:** the first project with prompts whose quality matters
across edits — likely throughline's chat/system prompts or the first
production agent.

### 6. Trajectory-level regression
**What:** golden-diff entire agent runs — the sequence of tool calls,
decisions, and stop causes — not just single responses; built on replay
cassettes + harness transcripts + journal entries.
**Exists today:** replay (chunk-exact), RunResult.Messages, journal JSONL.
**Subscribe when:** an agent's *behavior* (not just answers) must not regress.

### 7. Canned practices catalog
**What:** a default, opinionated `agentic-practices-evaluator` checklist
derived from [practices.md](practices.md) (citations present, budget wired,
injection screen on, provenance stamped…), so run self-audits work
out-of-the-box instead of caller-supplied-only.
**Exists today:** agentic-practices-evaluator (engine, no catalog).
**Subscribe when:** the second workload wants the same self-audit the first
one hand-wrote.

## Orchestration & scale

### 8. Worker-pool executor for swarm waves
**What:** the execution half of the coordination layer: actually RUN
swarm-orchestrator wave plans with bounded concurrency, worker-heartbeat
liveness, result-aggregator merging, and task-tree subtree delegation
(sub-agent spawn/collect).
**Exists today:** swarm-orchestrator/task-tree/worker-heartbeat/
result-aggregator (all plan-only, single-goroutine by design).
**Subscribe when:** the first multi-agent workload that fans out for real.

### 9. Agent run checkpoint/resume
**✅ Graduated — Wave 2 (v0.4.0).** Landed as `RunFromCheckpoint` +
`Checkpoint` bundle in `kits/agent-harness`: snapshot bundles {version,
conversation snapshot, optional budget snapshot, turn cursor}; `RunFromCheckpoint`
and `RunStreamFromCheckpoint` restore and resume the governed loop without
replay. See TRACKING.md §2 (agent-harness entry).

## Model & provider surface

### 10. OpenAI-compatible client
**✅ Graduated — Wave 3 (v0.5.0).** Landed as `foundations/llmclient/openai`
(package `openai`): full chat-completions adapter with tools, streaming,
images as data URIs, `OutputSchema`→`response_format json_schema`,
`Effort`→`reasoning_effort`, `Thinking` typed-rejected, `ProviderParams` merge,
typed errors + `Retry-After` both forms, `Ping`/`Pinger`; passes `RunConformance`
(18 subtests) + 14 wire tests. Promoted from deferred to required per D9
(cross-family judge independence). See TRACKING.md §1.2.

### 11. Batch API support
**What:** provider batch submission/polling in the LLM contract (~50% cost
on bulk work) — the second-biggest cost lever after prompt caching.
**Subscribe when:** eval-runner or any bulk enrichment workload lands.

### 12. Streamed unknown-block delta accumulation
**✅ Graduated — Wave 1 (v0.3.0).** Stream-assembled unknown blocks now equal
Complete-assembled blocks; key order preserved; bedrock eventstream parity.
The documented limitation is retired. See TRACKING.md §1.2 Wave 1.

### 13. Multi-provider routing/failover
**What:** Ladder generalized across providers (rung = client+model), with
health-probe-driven failover and per-rung cost weighting.
**Exists today:** Ladder, Swappable client, anthropic Ping.
**Subscribe when:** uptime across providers matters to a workload.

## Protocol surface

### 14. MCP client foundation
**What:** the consuming side of MCP — connect to external MCP servers, list
their tools, surface them through tool-registry so a kit agent can use any
MCP server's tools. (The kit serves MCP today; it cannot yet *consume* it.)
**Subscribe when:** an agent needs tools the owner didn't write.

### 15. MCP server: Streamable HTTP transport + prompts/resources
**What:** the current kit MCP server is stdio + tools-only (deliberate v1).
Remote serving needs the Streamable HTTP transport (NOT the deprecated
HTTP+SSE); prompts/resources capabilities when a server has content to serve.
**Subscribe when:** an MCP server must serve non-local clients.

### 16. CUE → tool-schema codegen
**✅ Graduated — Wave 1 (v0.3.0).** Landed as `toolchains/jsonschema`
(package `jsonschemagen`): CUE → JSON Schema draft 2020-12 via the `cue`
binary (OpenAPI export + pure-Go transform); deterministic output; `CheckFresh`
for CI; end-to-end proof test (CUE → derived schema → wire body via
`Request.OutputSchema`). See TRACKING.md §5.

## Governance & safety

### 17. Per-principal budgets & authenticated callers
**What:** keyed budget-guard (per-identity/tenant quotas) + an
authenticated-caller primitive (API-key/HMAC verification, scopes) so
endpoints can distinguish entitled callers from the anonymous public —
today's gauntlet treats everyone as anonymous IPs by design.
**Subscribe when:** the first endpoint with paying/known users.

### 18. Retention & crypto-shredding for append-only stores
**✅ Graduated — Wave 3 (v0.5.0).** Landed as two pieces:
`foundations/security/tokenization` gains `KeyStore`/`MemoryKeyStore`
(AES-256-GCM envelope keys), `EncryptedVault`, `ShreddableVault` extension
interface (existing `Vault` consumers unchanged), `Shred(namespace)` =
crypto-erasure with typed `ErrShredded`, and `RunShredConformance` (6-case
exported conformance suite). `foundations/governance/retention-policy` (WASM-safe)
provides pure `Due()` evaluation of caller-supplied retention rules with
`shred|flag|report` actions — the foundation decides, callers execute.
`practices.md` §12 status updated to shipped. See TRACKING.md §1.4 and §1.21.

### 19. Guardrails-pipeline kit + shared rule core
**✅ Graduated — Wave 3 (v0.5.0).** Landed as two pieces: `foundations/security/rule-core`
is the shared compile/`ScanFirst`/`ReplaceFixpoint`/`Ordinal` core under all
four regex engines (injection-detector, dlp-evaluator, pre-llm-dlp-gate,
redaction-executor); exported APIs of those four packages are unchanged.
`kits/guardrails-pipeline` provides pre-LLM (wrap→inject-detect→classify→redact)
+ post-LLM (integrity→honesty→DLP→disclaimer) chains with one `Decision`
envelope `{Allowed, Stages[], Transformed}`, fail-closed on stage error, CUE
chain config, and a sink seam; `llmegress` is documented as the transport
backstop. Subscribers: throughline chat + governed-prose-kit (adoption sketches
in package doc). See TRACKING.md §1.4 and §2.

## Observability

### 20. Run report renderer
**✅ Graduated — Wave 3 (v0.5.0).** Landed as `foundations/review/run-report`
(package `runreport`): `Render(Input)` produces deterministic markdown with
summary, activity log, timeline, concern flags, and cost section; "not
captured" degradation is explicit, never silent; golden-tested + harness
integration test. See TRACKING.md §1.3.

### 21. Cross-process trace propagation
**What:** carry RunID/TraceContext across process boundaries (MCP calls,
bounded-exec subprocesses, HTTP hops) via env/headers so multi-process runs
correlate into one trace.
**Subscribe when:** the first multi-process agent topology.

## Evidence & retrieval

### 22. Durable VectorStore conformance suite
**What:** pin the VectorStore port contract (ordering, dedup, dimension
checks, delete semantics) with an exported conformance suite (like llm-test)
so persistent backends — sqlite-vec, HNSW, pgvector — are drop-in consumer
adapters that provably match MemoryVectorStore semantics. The backends
themselves stay consumer-side (dependency-heavy; kit stays stdlib-only).
**Subscribe when:** the first persistent semantic index (throughline's
semantic search is the likely subscriber).

### 23. evidence-graph migration to canonical Span
**Partially done — Wave 3 (v0.5.0), graph queries deferred.** `kits/investigation`
migrated to canonical `evidencespan.Span` this wave (findings carry
`SupportingEvidenceIDs`; `EvidenceDegraded` concern flag). The `evidence-graph`
package's own TTL-coupled graph queries assessed NOT CHEAP (TTL coupling +
250+ line ripple); migration of the graph internals is deferred with the honest
assessment recorded in `foundations/evidence/README.md`. Marked `[~]` in
TRACKING.md.
**Subscribe when:** a consumer needs graph queries over *graded* evidence —
that is the trigger for completing the migration.

### 24. RAG truth-layer bridge
**✅ Graduated — Wave 3 (v0.5.0).** Landed as three packages:
`foundations/retrieval/truth-bridge` (`#ChunkLink` span↔chunk link;
`Graded()` with loud `GradeUnknown`, never silent); `foundations/retrieval/retrieval-metrics`
(Precision/Recall/MRR + `FalseAbsentRate` from canary records +
`CoverageFraction`; per-archetype applicability matrix; typed not-applicable,
never NaN); `foundations/retrieval/evidence-ranker` (LLM-free deterministic
feature ranker: pattern strength / path specificity / hit density / ref
stability / grade tier; caller-config weights; lexicographic tie-break;
`ErrSimilarityRefusedForAbsence` for absence archetypes). The notes' garbled
modes re-derived from first principles. See TRACKING.md §1.17.

### 25. Claim case-file aggregation
**✅ Graduated — Wave 3 (v0.5.0).** Landed as `foundations/evidence/case-file`
(package `casefile`): pure deterministic projection over assertions by type
(stance buckets), tier-rank weighted support/against scores with documented
math, net classification bands; byte-identical golden-tested. Throughline UI
is the named subscriber. See TRACKING.md §1.5.

## Parked from Update-2 / H5 (subscribe-when, verbatim from uplift plan §7)

### P1. judgepool / judgepanel
**What:** central judge vetting + cross-family jury for contested calls.
**Exists today:** kits/eval-runner single-judge lanes; convergence-detector agreement math; model-catalog family/lineage data.
**Subscribe when:** "first contested/irreversible verdict class".

### P2. escalation (judge model ladder)
**What:** confidence-gated cheapest-first judge escalation.
**Exists today:** llmclient.Ladder + NewFailover; eval-runner journals per-lane spend.
**Subscribe when:** "judge spend measurable and material; composes llmclient.Ladder + failover, never a new wrapper".

### P3. conviction-metrics statistical suite
**What:** signal-vs-noise scoring over eval-journal record series (DFR, noise bands, per-concern regression significance).
**Exists today:** eval-journal score series; diff-reporter structural deltas; determinism-gate variance bands.
**Subscribe when:** "score-trend noise actually observed; thresholds re-derived with units — the scanned 0.5/DFR numbers are unusable".

### P4. leak-detector
**What:** detects a producer echoing test/fixture source into answers.
**Exists today:** determinism-gate flips; scenario-bank provenance tags.
**Subscribe when:** "first false-green incident; canary tokens as the primary signal when built".

---

*Maintained under VISION.md's practice-watch: new field-level best practices
and newly observed anti-patterns add entries here (or land directly with a
subscriber). Last reviewed: 2026-06-10.*
