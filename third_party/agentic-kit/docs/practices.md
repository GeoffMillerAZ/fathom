# practices.md — the anti-pattern ledger

This is the document [VISION.md](../VISION.md) points to: each known
agentic-workload anti-pattern → why it bites → the kit's opinion (the practice
it enforces instead) → the packages that implement it → an honest status
(**shipped** | **partial** | **roadmap**). Every package claim below was
verified against the code; a practices doc that overclaims is itself the
anti-pattern.

## 1. Lossy progressive summarization

**The anti-pattern.** Context fills up, so the workload compacts: old turns
become a model-written summary and the originals are thrown away. The evidence
link is severed. This is the canonical failure the kit exists to prevent.

**The opinion.** Summaries are *projections over* retained ground truth, never
replacements. Compaction may shrink what the model sees; it must never shrink
what the system knows. Ground truth lives in append-only journals, hash-chained
event stores, and evidence spans that survive any windowing decision.

| Package | Role |
|---|---|
| `foundations/agent/conversation` | `Window(budget)` returns an API-valid suffix and the dropped count; tool_use/tool_result pairs are never split; `Options.Compactor` is the hook that splices a role-safe summary at the cut point |
| `foundations/prose/context-renderer` | deterministic budget-bounded assembly; priority-sorted segments, lowest priority truncated first |
| `foundations/investigation/investigation-journal` | append-only step journal with sinks (`AttachSink`/`JSONLSink`) — the human-readable ground truth that survives compaction |
| `foundations/provenance/event-store` | append-only hash-chained system of record; `Verify` detects edits by anyone who cannot recompute the chain, plus reordering and gaps (NOT holder-recomputed rewrites or tail truncation — anchor `Head().Hash` and entry count externally for both) |
| `foundations/evidence/evidence-span` | the atomic evidence unit; spans persist independently of any conversation window |

**STATUS: shipped.** The retention spine and the `Compactor` hook were always
shipped; the missing half — an opinionated evidence-preserving summarizer
*implementation* — is `foundations/agent/evidence-compactor` (roadmap #1,
Wave 2), and as of 2026-09-02 (FB-6) the kit also *installs* it:
`agentharness.WithEvidenceCompactor(cfg, ec)` sets it as the conversation
`Compactor` (refusing a budget-less config), and `examples/agent-cli` runs it
behind `-context-budget`. Between Wave 2 and FB-6 the package existed but
nothing in the kit wired it — this ledger said "roadmap" while `compat.md`
said "wired", and both were wrong in different directions.

## 2. Unsourced claims / broken evidence linking

**The anti-pattern.** The agent asserts; nobody can say where the claim came
from, who produced it, or how much to trust it. Fabricated references pass.

**The opinion.** Evidence before assertion. Every span carries `Grade`
(explicit epistemic tier), `Author` (human or AI with reproduction key), and
`Locator` (click-traceable source pointer); `ValidateAsEvidence()` enforces *no
provenance → not storable as evidence*. Confidence is capped by tier:
`review/scoring` refuses confidence 3+ (of 5) to any claim not at least
pre-scanned — an unverified tier CANNOT exceed its ceiling, by code.

| Package | Role |
|---|---|
| `foundations/evidence/evidence-span` | atomic claim with Grade/Author/Locator + record hashes; `ValidateAsEvidence` |
| `foundations/evidence/evidence-grade` | tier taxonomy; `GradeClaim` weakest-link promotion |
| `foundations/evidence/assertion` | separates WHAT is claimed from WHO asserted it, how, when |
| `foundations/evidence/source-linker` / `source-registry` | bidirectional span↔source links over a governed source registry |
| `foundations/prose/evidence-citation` | deterministic, sorted citation rendering for governed prose |
| `foundations/quality/integrity-evaluator` | deterministic check: no fabricated or unresolvable references |
| `foundations/quality/honesty-gate` | deterministic Allowed/Hedge/Block verdict on overclaiming |
| `foundations/review/scoring` + `confidence-scoring` | tier confidence ceilings (`TierCeiling`/`ApplyCeiling`); confidence derived from evidence count + agreement |

**STATUS: shipped.** Start at `foundations/evidence/README.md` — it documents
the canonical pipeline and the one trap: `evidence-graph.Span` is a legacy shape
(no Grade/Author/Locator) whose migration is tracked deferred work.

## 3. Improvised escalation

**The anti-pattern.** "Try the cheap model, and if it feels wrong, hack in a
retry with the big one." Escalation logic accretes as ad-hoc control flow:
unauditable, untestable, different in every workload.

**The opinion.** Escalation paths are declared data, not ad-hoc control flow.
Model escalation is a `Ladder` of `Rung`s climbed only on an explicit `Signal`
(`Escalate` + audit `Reason`); the rung's `Model` is injected into requests,
which is how a ladder composes with any `LLMClient`. `StopToolUse` is success,
never an escalation signal — escalate on measured quality. Refinement loops are
bounded; humans are reached through a typed channel; termination is a value.

| Package | Role |
|---|---|
| `foundations/llmclient` (`Ladder`/`Rung`/`Signal`) | cheap→capable rungs, climbed on declared signals; rung Model injection |
| `foundations/orchestration/planning-refinement` | bounded refine rounds (`Policy.MaxRounds`), then `Escalated=true` |
| `foundations/orchestration/operator-elicitation` | typed human-in-the-loop: `Ask`/`Answer`/`Pending` |
| `kits/agent-harness` | stop causes as values: `StopCompleted/MaxTurns/Budget/Refusal/MaxTokens/Canceled` in `RunResult.Stop` — results, not exceptions |

**STATUS: shipped.**

## 4. Unbounded loops and spend

*OWASP Agentic Top 10 (2026): ASI10 rogue agents — the lifecycle half: a run nobody bounded keeps spending.*

**The anti-pattern.** An agent loop with no turn cap, calls with no budget,
caches and seen-sets that grow forever. The failure mode is a bill or an OOM,
discovered after the fact.

**The opinion.** Fail closed, spend bounded, before the money leaves. The
agent-harness charges the budget guard before EVERY model call and ends the run
with `StopBudget` on denial; endpoint-guard charges even its topic-judge call
before the judge fires (refused charge → no judge call). Every cache, queue, seen-set,
and buffer on the request path has an explicit bound and a stated behavior at
the bound; append-only ground-truth stores (journals, ledgers, event stores)
are deliberately unbounded — that is their job, and its retention consequence
is section 12's entry.

| Package | Role |
|---|---|
| `foundations/governance/budget-guard` | per-minute/per-day request + per-day USD quotas; deterministic, fail-closed `Charge` |
| `kits/agent-harness` | pre-call `Charge`, `Config.MaxTurns` |
| `kits/endpoint-guard` | judge calls budget-charged before they happen; budget trips surfaced as metrics |
| `foundations/governance/circuit-breaker` / `rate-limiter` | open = refuse; exhausted tokens = refuse |
| bounded structures everywhere | `CachingEmbedder` LRU (default 4096), `result-cache` TTL+LRU+single-flight, proof-of-work `SeenSet` (fails closed at `MaxSeen`), MCP `MaxMessageBytes` (4 MiB default), `bounded-exec` output byte caps |

**STATUS: shipped**, with one previously-open gap now closed. The `KeyedLimiter`
(per-identity token buckets in `endpoint-guard`) has a `GC` method that evicts
idle buckets, but it was never called — the per-key map grew without bound for
the life of the process. `cmd/endpoint-guard` now runs a periodic GC goroutine
(every minute, bounded to server shutdown) so the map stays bounded in
production. Callers embedding the kit's `Guard` directly must call
`Guard.GCRateLimiter()` on their own schedule; the kit cannot own the ticker
without importing a runtime dependency. This is the one structure on the hot
path that required explicit wiring rather than being self-bounding.

## 5. Trusting model and tool output

*OWASP Agentic Top 10 (2026): ASI01 goal hijack (injected content steering the agent), ASI02 tool misuse.*

**The anti-pattern.** Tool results flow back into the conversation unexamined;
fetched content is pasted next to instructions; "we run it in a subprocess" is
mistaken for containment.

**The opinion.** Defense-in-depth, honestly framed — mitigations, not a
boundary. The harness scans every tool result before it re-enters the
conversation: flagged results are blocked (replaced by an `is_error` tool_result
naming the detection; the raw payload never enters the transcript) or annotated.
Untrusted content is fenced between per-wrap nonce delimiters — explicitly NOT a
boundary; the model can still be persuaded by the content. Tool execution is
deny-by-default where a policy is wired. And `bounded-exec` is NOT a sandbox:
the child runs with the host's full ambient authority (filesystem, network,
memory, CPU, privileges), bounded only in wall-clock time and captured output;
real isolation requires OS primitives, out of scope for a stdlib-only kit.

| Package | Role |
|---|---|
| `foundations/security/injection-detector` | data-driven injection rules; wired into agent-harness tool-result screening (block/annotate) |
| `foundations/security/untrusted-content-wrapper` | nonce fences; defense-in-depth, not a boundary |
| `foundations/security/instruction-hierarchy` | system > developer > user > untrusted precedence |
| `foundations/environment/tool-execution-policy` + `agent/tool-registry` | deny-by-default policy, consulted before every handler via the registry's `Options.Policy` hook |
| `foundations/agent/bounded-exec` | resource confinement only; non-guarantees stated in its package doc |

**STATUS: shipped** (as defense-in-depth; nothing here claims containment).

## 6. Live-API-only testing

**The anti-pattern.** The only way to test the agent is against the real API:
slow, flaky, expensive, unrunnable in CI — so it stops being run.

**The opinion.** A test ladder, deterministic below the top rung: `MockClient`
(scriptable; rejects requests violating documented contract limits with a typed
error — tests cannot pass requests a live client would refuse) → replay
cassettes (scrubbed by default; opting out requires the explicit, dangerous
`NoScrub`; entries consumed ordinally with opt-in request-hash matching) →
`llm-test` conformance for any new client (httptest, no network) →
`task smoke-live`, the one paid rung, gated on `ANTHROPIC_API_KEY` and skipped
without it. Underneath: injected clocks and randomness, no network in tests,
everything `-race`-clean.

| Package | Role |
|---|---|
| `foundations/llmclient` (`MockClient`) | deterministic scripted double with pre-HTTP contract validation |
| `foundations/llmclient/replay` | record/replay cassettes; scrub-by-default; ordinal + optional hash matching |
| `foundations/llmclient/llm-test` | `RunConformance` suite any `LLMClient` implementation must pass |
| `task smoke-live` | the only live rung, explicit and skippable |

**STATUS: shipped.**

## 7. Invisible runs

*OWASP Agentic Top 10 (2026): ASI10 rogue agents — a behavioral baseline needs a run record to baseline against.*

**The anti-pattern.** The agent ran; nobody can say what it did, what it cost,
or which model call belonged to which run.

**The opinion.** One canonical event shape, trace-correlated. The harness mints
a `RunID` when the context carries none, so harness step events and `llm_call`
events from an instrument-wrapped client share one run identity. Instrumentation
emits exactly one event per *logical* call — retry sits innermost, attempts
surface as an event attribute — so dashboards and budgets keep a stable shape
during retry storms, exactly when you read them.

| Package | Role |
|---|---|
| `foundations/telemetry/event` | canonical `Event` + `TraceContext` (`RunID`/`SpanID`, `TraceFrom`/`Apply`) |
| `foundations/telemetry/sink` | JSONL, slog, memory, multi, filter sinks |
| `foundations/telemetry/metrics` | stdlib Prometheus text-format registry |
| `foundations/cost/cost-tracker` | config-driven price table (NO prices shipped — stale prices are worse than none); spend by label and model |
| `foundations/llmclient/instrument` | `WithTelemetry`/`WithCost` middleware; failure-isolated (telemetry never breaks the call path) |
| `foundations/investigation/investigation-journal` | the human-readable run trace, with JSONL sinks |

**STATUS: shipped.**

## 8. Unaudited prompt drift

*OWASP Agentic Top 10 (2026): ASI04 agentic supply chain — a prompt is an unversioned build input.*

**The anti-pattern.** Prompts are edited in place; outputs change quietly;
nobody can say which prompt version produced which artifact, or rerun the old
one.

**The opinion.** Every AI-authored artifact carries its reproduction key.
`prompt-template.Render` produces `PromptHash`, `PromptTemplateVersion`, and
`ParamsHash`; `ai-provenance.AIAuthor` carries them — plus model id and input
digest — on every span, decision, and prose node. The review ledgers preserve
authorship: every `claim-lifecycle` event carries an `Author`;
`correction-ledger` records model id and prompt/params hashes on AI authors.

| Package | Role |
|---|---|
| `foundations/prompt/prompt-template` | versioned template rendering → PromptHash/ParamsHash |
| `foundations/provenance/ai-provenance` | the authorship record those hashes live on |
| `foundations/review/claim-lifecycle` / `correction-ledger` | hash-chained / append-only ledgers that carry AI provenance through review |

**STATUS: partial.** The hashes are shipped end to end. A versioned prompt
*registry* is deliberately absent (`prompt-template` makes callers own storage
and lookup), and the eval-runner regression harness that would catch quality
drift is deliberate roadmap — TRACKING.md defers it until a real eval corpus
exists to shape it. Drift is *attributable* today, not yet *auto-detected*.

## 9. Tool definition sprawl

*OWASP Agentic Top 10 (2026): ASI02 tool misuse — least-agency tool scoping.*

**The anti-pattern.** The same tool is defined three times — for the API
request, the dispatch switch, and the MCP listing — and the three drift.

**The opinion.** Define a tool ONCE. One `toolregistry.Tool` (an
`llmclient.ToolDef` plus a Go handler) feeds `Request.Tools`, harness dispatch,
and `kits/mcp-server` `tools/list` from the same registration. Policy, per-tool
timeouts, and panic recovery are owned once, in the registry: every failure mode
(unknown tool, policy denial, handler error/panic, timeout, cancellation)
surfaces in-band as an `is_error` tool_result, never as a Go error.

| Package | Role |
|---|---|
| `foundations/agent/tool-registry` | the single tool table: `Defs` + `Dispatch`/`DispatchAll` |
| `kits/agent-harness` / `kits/mcp-server` | both consume the registry rather than invent their own tables |

**STATUS: shipped.**

## 10. Schema/doc drift

**The anti-pattern.** The docs describe one shape, the code another, the stored
data a third. Drift is discovered by an outage, not a gate.

**The opinion.** CUE owns shape, Go owns behavior, and the gate makes drift a
test failure: `task cue-vet` (inside `task validate`) vets every `schema.cue`
*and its testdata fixtures*; `docs/catalog.md` is generated from package doc
comments (`task docs-index`), never hand-edited; TRACKING.md is the single
source of build status with deferred decisions recorded, not lost; conformance
suites (`llm-test`) pin semantics across implementations so a new provider
adapter cannot quietly mean something different.

**STATUS: shipped** — as enforced process; it holds only while the gate stays
non-negotiable.

## 11. Silent truncation and degradation

**The anti-pattern.** Output was cut, context was dropped, the fancy code path
quietly fell back to the dumb one — and nothing said so.

**The opinion.** Degrade loudly or refuse. `bounded-exec` returns explicit
`StdoutTruncated`/`StderrTruncated` flags; `conversation.Window` returns the
dropped-message count; an oversized MCP message gets an explicit `-32700` error
and a resync to the next message boundary — when clean resync is impossible, the
server fails closed rather than misparse. Degraded modes are documented costed
trade-offs, not hidden: `topic-gate`'s `JudgeOnly` mode states that every
non-prefiltered input costs one judge LLM call; `answer-cache` without an
`Embedder` degrades to exact-match-only — documented in its API, though that
fallback (and an embed error becoming a miss) is not telemetered today.

**STATUS: shipped** posture, with the noted gap: degraded-mode telemetry for
answer-cache is not built.

## 12. PII into append-only stores

**The anti-pattern.** Journals, ledgers, cassettes, and call logs are
append-only by design — personal data written into them can never be removed.

**The opinion — stated as a limitation first.** The kit's append-only stores
(event-store, journals, claim/correction ledgers, replay cassettes, MCP call
logs) have NO deletion or retention story today — an honest, documented gap.
What ships are ingress mitigations: replay scrubbing is ON by default
(`DefaultScrubber`, best-effort secret-shape redaction, gaps stated in the
package doc), and the MCP call log NEVER records tool arguments or results
unless `LogPayloads` opts in. Keep PII out; it cannot be taken back out.

**STATUS: shipped (crypto-shred).** EncryptedVault + KeyStore give append-only
stores a deletion story via crypto-erasure: `Shred(namespace)` destroys the
per-namespace AES-256-GCM envelope key, making stored ciphertexts permanently
unrecoverable while the token log stays intact; `retentionpolicy.Due` evaluates
caller-supplied retention rules and prescribes `shred|flag|report` actions —
the foundation decides, callers execute; `RunShredConformance` is the
conformance suite durable adapters must pass.

## 13. The lethal trifecta

*OWASP Agentic Top 10 (2026): ASI01 goal hijack, ASI02 tool misuse, ASI03 identity and privilege abuse.*

**The anti-pattern.** An agent is given three things at once: a tool that reads
untrusted content (web pages, user files, email), a tool that reads private data
(credentials, PII, internal docs), and a tool that writes externally (email,
Slack, HTTP). No individual tool is dangerous. Together they are the "lethal
trifecta": a single prompt-injection payload in the untrusted content can
instruct the agent to read private data and exfiltrate it via the write channel,
all within one turn, with no human checkpoint. This is the canonical 2026
agent-security failure mode.

**The opinion.** Capability taxonomy at registration time, fail-closed by
default. Every tool declares its capabilities — `CapReadsUntrusted`,
`CapReadsPrivate`, `CapWritesExternal` — as a `CapabilitySet` on the `Tool`
struct. A nil set is UNKNOWN, not clean: unknown tools are treated as
potentially exercising all three capabilities under `TrifectaStrict` mode.
When the registered toolset's union of capabilities completes the trifecta,
`TrifectaStrict` blocks the completing tool's `Register` call with a typed
`*ErrTrifectaBlocked` error — refused before any token is spent. Overriding
requires a named `TrifectaOverride` struct with `AuthorizedBy` and `Reason`
fields: who reviewed the risk and why it is acceptable. A bare boolean cannot
override; provenance is required.

Taxonomy is self-declared. A lying tool defeats detection. This is
defense-in-depth, not a security boundary — its value is making the risk
auditable and forcing explicit, named authorization.

| Package | Role |
|---|---|
| `foundations/agent/tool-registry` | `CapabilitySet`, `Capability` constants, `TrifectaOptions`, `TrifectaOverride`, `ErrTrifectaBlocked` — registration-time enforcement |
| `foundations/agent/mcp-client` | `BridgeOptions.CapabilityMap` — declare capabilities for foreign MCP tools at bridge time; absent = unknown |

**STATUS: shipped.** Registration-time detection and `TrifectaStrict` blocking
are implemented and tested across all six registration-order permutations.
Dispatch-time re-check is deliberately absent — the trifecta is a property of
the toolset, not a per-call property; blocking at registration is the correct
seam (before any token is spent). The residual risk of self-declared taxonomy is
honestly documented.

## 14. Tool poisoning

*OWASP Agentic Top 10 (2026): ASI04 agentic supply chain, ASI02 tool misuse.*

**The anti-pattern.** An MCP server's tool descriptions are fetched over the
network and injected into the agent's context on every tools/list. An attacker
who can modify those descriptions between calls — or who operates a malicious
server — can hide instructions ("ignore previous instructions…") in a
description that the agent re-reads silently. The description looks like
metadata; the model treats it as instructions.

**The opinion.** Hash-pin tool descriptions at first listing; verify on every
subsequent listing. `mcpclient.Client` computes a SHA-256 of (name ‖ description ‖
inputSchema) for each tool during `New`, stores it in a guarded pin table, and
re-verifies on every subsequent `ListTools` — the path by which a description
enters the model's context. A changed description returns a typed
`ToolChangedError` rather than silently forwarding modified instructions to the
model. **`CallTool` does not re-verify the pin at call time** — drift between a
listing and a call is caught at the next `ListTools`; a pre-call pin check is a
proposed hardening item in TRACKING.md's pull queue, decision the owner's.
Rebinding a changed tool requires an explicit `Repin` call — drift is visible
and auditable. Re-pinning discipline: repin only after a human or automated
gate has reviewed the new description; a CI check on description changes in
the server's source is the intended upstream control.

| Package | Role |
|---|---|
| `foundations/agent/mcp-client` | SHA-256 pin table in `Client`; `ToolChangedError`; `Repin` for intentional repin |

**STATUS: shipped.** Hash-pinning is implemented and covered by the conformance
suite. The upstream CI hook (diffing server-side description changes) is the
caller's responsibility; the kit enforces the check but cannot own the server.

## 15. Memory poisoning

*OWASP Agentic Top 10 (2026): ASI06 memory and context poisoning.*

**The anti-pattern.** Untrusted content flows into a persisted memory or
knowledge store — a vector database, a conversation summary, a retrieved context
chunk — and re-emerges later as trusted context. The model cannot distinguish
"I was told this by the user" from "this was injected by a web page I fetched
three turns ago and then summarized into my working memory." The attack surface
is temporal: the injection and the exfiltration happen in different turns.

**The opinion.** Fence untrusted content at ingestion; stamp provenance before
storage; screen retrieval before re-injection. The kit's current answer:
`untrusted-content-wrapper` fences untrusted payloads between per-wrap nonce
delimiters before they touch any store or summary path (explicitly not a model
boundary — a persuasive payload can still influence the model, but the fences
are a visible marker for downstream screening). `injection-detector` screens
content for injection patterns; the agent-harness wires it to tool results
before they re-enter the conversation. `evidence-span` provenance stamps
survive compaction — a retrieved chunk carries its grade and author forward, so
the model (and human reviewers) can see whether a claim originated from a
primary source or from a previously-injected tool result.

| Package | Role |
|---|---|
| `foundations/security/untrusted-content-wrapper` | nonce fences at ingestion; defense-in-depth marker, not a model boundary |
| `foundations/security/injection-detector` | pattern-based screening; wired in agent-harness to tool results |
| `foundations/evidence/evidence-span` | provenance stamps (`Grade`/`Author`/`Locator`) that survive conversation compaction |

**STATUS: partial.** Fencing and injection screening at tool-result ingestion
are shipped. What is NOT built: a provenance-aware retrieval gate that refuses
to re-inject evidence below a minimum grade threshold, and a store-layer policy
that rejects untrusted-fenced content from entering long-term memory without an
explicit declassification step. Those are the missing seams; the anti-pattern is
mitigated at the edges but not closed end-to-end.

## 16. Intent amnesia

**The anti-pattern.** The agent knows the code and nothing else: not why the project
exists, what must never break, which futures are real, which alternatives already lost
and why, or what would make an old decision worth reopening. It then either optimizes
the requirement in front of it and walls off a future the humans were counting on, or
hedges against every imaginable future with abstractions nobody asked for. Both failures
come from the same gap. Loading the whole history into every session is not the fix; it
is a second anti-pattern (context bloat) wearing the first one's clothes.

**The opinion.** Durable intent lives in the repo at `docs/intent/` as seven short
living documents plus decision records, and always-loaded instructions only *route* to
it. Every entry that can go stale carries its own expiry condition (`Done when`,
`Promote when`, `Drop if`, `Revisit when`) so one grep is the trigger registry. The
governing rule for the future: build for now; foreclose nothing documented when keeping
it open is free. Full practice: `docs/project-intent/README.md`.

| Component | Role |
|---|---|
| `docs/project-intent/` | methodology, agent integration (Claude Code, Codex, generic `AGENTS.md`), examples of good and bad future-proofing |
| `.agents/skills/intent-*/` | the on-demand skills — `intent-init`, `intent-design`, `intent-decide`, `intent-audit`, `intent-doctor`, `intent-prune`, `intent-release`, `intent-incident` — and the templates they copy; `install-intent.sh` puts them on a harness's path |

**STATUS: shipped** as docs and a skill; no Go package, by design. Revisit when a second
adopting repo shows index drift the `refresh` walk did not catch (that is the day a
checker earns its keep).

## 17. Instruments that degrade toward healthy

**The anti-pattern.** A check cannot read part of what it was asked to check —
a file is unreadable, a glob matches nothing, a directory walk hits an error —
and it reports a *pass*, a *skip*, or a *smaller number*. The most broken state
of a repository is reported as its healthiest-looking one, because eight
readers each did `if err != nil { continue }` and collapsed "absent",
"unreadable" and "empty" into one fact. Measured, not theoretical: `chmod 000`
on one file made fathom's intent audit report 0 invariants instead of 37 and 85
unproven claims instead of 169, naming no file (fathom ADR-027).

**The opinion.** An instrument that cannot see must say so, in the number.
Every deterministic read is one of four outcomes — `read`, `absent`,
`unreadable(errno)`, `skipped(category)` — recorded in a manifest that only
`unreadable` makes incomplete; recording is first-outcome-wins, so a later
retry cannot erase a recorded blindness; and every count taken over an
incomplete manifest renders as a NAMED lower bound (`≥ N (k unreadable: …)`),
never a bare total. "Absent" and "could not look" are different facts and a
rung asserted by a check that cannot tell them apart is *stated*, not *tested*.

| Package | Role |
|---|---|
| `foundations/telemetry/read-manifest` | `Outcome` / `NewSet` / `Set.Record` (first-outcome-wins) / `Manifest.Complete` / `LowerBound`; `schema.cue` with fixtures |
| `cmd/constitution-lint` | the kit's own consumer: `walkWithManifest` routes every continue-on-error walk through a manifest, the `read-coverage` check FAILS the constitution on an unreadable file or directory, and the summary line reports a lower bound when the manifest is incomplete |

**STATUS: shipped.** Two consumers, per D13: `cmd/constitution-lint` in this
repo, and fathom's `internal/readset` (ADR-027), which is the shape this
package was promoted from and which collapses onto it at its next pin bump.
Closes `docs/plans/fathom-uplift-asks.md` FA-20 (the mechanical counterpart to
D10's coverage discipline) and FA-21 (the practice rule itself).

## 18. Irreversible actions without a structural gate

*OWASP Agentic Top 10 (2026): ASI02 tool misuse; ASI09 human-agent trust exploitation on the confirmation itself.*

**The anti-pattern.** A tool that deletes, force-pushes, pays, emails or drops
a volume is dispatched exactly like `read_file`. The only thing between the
agent and the irreversible effect is prose in a system prompt — which the agent
can recite correctly *after* breaking it.

**The opinion.** Encode the boundary in the tooling, not the prompt. A tool
carries a typed effect using the MCP annotation vocabulary verbatim
(`ReadOnlyHint`, `DestructiveHint`, `IdempotentHint`, `OpenWorldHint`), and an
UNDECLARED effect is not "probably fine" — under a strict policy it reads as
destructive, the same nil-is-unknown posture §13 takes for capabilities
(decision `0002`). The gate is structural and per call: a tool approved once is
not approved for its next call, an absent or errored approver is a denial, and
a denial is an `is_error` tool_result the loop can continue from rather than a
crash. The effect is host metadata — `Defs()` is unchanged, so the gate cannot
be talked out of by the model that reads the tool definitions.

| Package | Role |
|---|---|
| `foundations/agent/tool-registry` | `Effect` (the four MCP hints) + `EffectOf`; additive — no existing call site changes |
| `kits/agent-harness` | `Config.EffectPolicy` (`permissive` default, `strict`); under strict an undeclared or destructive dispatch goes through `RequireApproval` → `Deps.Approver`, emitting `gate_denied` / `gate_approved` events plus a journal record; sequential even under `ParallelTools` |
| `kits/hitl` | `Preview` supplies the approval card the approver is shown — the impact of the real op, not the model's description of it |
| `kits/mcp-server` | `annotationsFromEffect` emits the annotations object in `tools/list` for tools with a declared `Effect`, all four hints explicit |
| `foundations/agent/mcp-client` | `effectFromAnnotations` maps a server's annotations IN as untrusted unless `TrustedServer` — a server's self-declared "read only" is a claim, not a fact |

**STATUS: shipped.** Decision `0002` records the three calls (annotation
vocabulary verbatim; nil means destructive under strict; approval composes the
existing `PreDispatch` seam and `kits/hitl` rather than a new gate). Consumers:
`scenarios/governed-investigation` dispatches a destructive tool through the
strict gate; `cmd/kit-new`'s generated harness declares effects.

## 19. Protocol-revision drift

*OWASP Agentic Top 10 (2026): ASI04 agentic supply chain.*

**The anti-pattern.** An implementation pins a protocol revision, the protocol
moves, and nobody in the tree owns the delta. Deprecated features stay
implemented past their removal floor because the deprecation lived in a
release note nobody re-reads; or the opposite failure, a chase — the
implementation churns after every upstream revision with no consumer asking.

**The opinion.** Track revisions with a clock; never chase them. The revision
is a named constant, the deprecations are a table of
`{Feature, DeprecatedIn, RemovableAfter, StillImplemented}`, and a test with an
injected clock goes red the moment a still-implemented feature passes its
twelve-month floor — the existence-check doctrine (I19) applied to protocol
time. Moving to a new revision is then a dated decision with a floor, not a
silent drift; a revision that deprecates something the kit never built costs
one table row and no code.

| Package | Role |
|---|---|
| `kits/mcp-server` | `ProtocolRevision` (`2025-11-25`), `Deprecations()`, `DeprecationsDue(now)`; CUE `#ProtocolRevision` / `#Deprecation`; `TestDeprecationFloorNotReached` reads the wall clock on purpose |
| `foundations/agent/mcp-client` | the same three exports over the same feature set; `TestDeprecationsMatchClient` pins both ledgers to one list so the two halves cannot drift apart |

**STATUS: shipped** for the clock. Whether to MOVE to the 2026-07-28 revision
(stateless, `_meta` on every request, Tasks as an extension) is deliberately a
separate, deferred decision with its own floor — decision `0003`, futures F10.

## 20. Fail-open gates mistaken for gates

*OWASP Agentic Top 10 (2026): ASI02 tool misuse — the control that was supposed to scope agency does not.*

**The anti-pattern.** A guard is advisory — it times out, its infrastructure is
down, its process is not installed — and the effect proceeds anyway. That is
often the CORRECT engineering choice for a hook whose failure would wedge an
editor. The anti-pattern is not fail-open; it is fail-open *presented as a
boundary*, so nobody can say which of an agent's guards would actually stop an
irreversible action and which merely usually do. A behavioral classifier
counted as a boundary is the same error one layer up (belief 6 applied to
guards).

**The opinion.** Every gate declares its posture: structural or behavioral,
fail-closed or fail-open, and which effects it covers. Then the honest question
— "is any irreversible effect covered only by advisory gates?" — is answerable
rather than assumed.

| Component | Role |
|---|---|
| `kits/agent-harness` | fail-closed by construction: `Config.Hooks` are synchronous and a hook error ends the run with `StopHook`; the budget is charged BEFORE every model call and a refusal is `StopBudget`; under `EffectStrict` an absent, errored or denied approver is a denial, never a pass-through |
| `foundations/agent/tool-registry` | `Options.Policy` is deny-by-default when wired, consulted before every handler; `TrifectaStrict` refuses at registration (§13) |
| `kits/mcp-server` | an oversized message is an explicit `-32700` and a resync; when clean resync is impossible the server fails closed rather than misparse |

**STATUS: partial**, and the split matters. Shipped HERE: the kit's own harness
gates are fail-closed by construction (the rows above) — there is no advisory
rung inside the loop to mislabel. NOT shipped here: the declared-posture type
itself (`GatePosture{Kind, FailOpen, Covers}`) and an assurance report over it.
The failure this entry describes lives at the CONSUMER edge, where a harness's
hooks are fail-open by doctrine for infrastructure errors — correct, and
undocumented to the operator. That row is fathom's: a doctor `gate_posture`
check, landing as `HP-15` in its harness-port plan. The kit will absorb the type
when a second consumer needs it; until then this section is a boundary
description, not a package claim.

## 21. Self-derived ground truth, n=1 evidence, and precision-less set grading

*OWASP Agentic Top 10 (2026): ASI09 human-agent trust exploitation — a self-graded claim is read as a measurement.*

**The anti-pattern.** Three failures that travel together in every agent
benchmark. The system grades itself against ground truth it derived (the
generator writes the answer key). A single run per condition is reported as a
result, so a 90% single-attempt success rate is quoted for a workload that
actually succeeds 43% of the time at k = 8. And a set-valued answer is graded
on recall alone, so returning everything scores perfectly.

**The opinion.** Reliability is a distribution, not a number, and the
statistics belong in one place rather than hand-built per project. Report
`pass^k` next to `pass@k`; compare paired conditions with a seeded,
task-clustered bootstrap interval and a rank test rather than a difference of
means; state the effect size. Every primitive states its non-guarantees:
no multiple-comparison correction is applied, the caller pre-registers its
comparisons, the interval is percentile (not BCa), and byte-stability holds
only under a fixed seed.

| Package | Role |
|---|---|
| `foundations/quality/reliability-stats` | `PassAtK` (the unbiased Chen et al. estimator), `PassPowK` (τ-bench p^k), `PairedBootstrapCI` (seeded, task-clustered, percentile), `Wilcoxon` (exact to n = 25, tie-corrected normal approximation above), `A12` (Vargha–Delaney) |
| `kits/eval-runner` | the kit's own consumer: `Regression` reports per-scenario pass^3 (`RegressionResult.PassPowK`) — reported, never gated |
| `foundations/quality/eval-journal` | every judge call becomes a record, so a verdict is attributable to the call that produced it rather than to a summary of it |

**STATUS: shipped for the primitives; the discipline lives downstream.** What
this kit owns is the arithmetic and its stated non-guarantees. What it
deliberately does NOT own is a corpus: derived third-party ground truth, a
memorisation floor and a precision term are properties of a benchmark, and the
named consumer is fathom's bench (ADR-031, with the n ≥ 3 floor and paired
statistics of ADR-028). Judge-bias controls — position swapping, cross-family
requirements, panel escalation — stay parked as F14; this section is about
counting runs honestly, not about judging them.

## 22. Handoff-less stops

**The anti-pattern.** A run ends on a budget ceiling, a token ceiling or a turn
ceiling and leaves nothing structured behind. The next context window starts
blind: it re-derives what was already established, or — worse — trusts a prose
summary as if it were state.

**The opinion.** A stop that was not "done" produces a receipt. What was tried,
which tool ran last, where the checkpoint is, and how many turns it took are
values on the result, emitted once to the sink, the stream and the journal —
and never stored by the kit. This is deliberately the narrow shape: a handoff
is a receipt of ONE run, not memory. Cross-session memory remains F2 ("do not
build a memory store"); fathom's own ADR-028 reaches the same boundary from the
other side, keeping recall in a peer process that may recall but never assert.

| Package | Role |
|---|---|
| `kits/agent-harness` | `Handoff{StopCause, Turns, LastTool, CheckpointRef, Tried, Summary}` via `HandoffFor`/`CheckpointRef`, on `RunResult.Handoff`; emitted once on `StopBudget`, `StopMaxTokens` and `StopMaxTurns` (sink `step=handoff`, `HarnessEventHandoff`, journal); CUE `#Handoff` |
| `examples/agent-cli` | prints the record — the consumer that proves the shape is usable without a store |

**STATUS: shipped.** A stop cause the harness does not emit a handoff for
(`StopCompleted`, `StopRefusal`, `StopHook`, `StopCanceled`) is a deliberate
choice, not a gap: those stops either finished or already carry their reason.

## Using this ledger

Starting a new workload: walk the sections above and wire what applies — each
table is the import list. When a new anti-pattern is identified in the field, it
gets an entry here *and* an opinionated primitive in the kit (VISION.md
practice-watch: absorb, don't annotate). For self-audit,
`foundations/quality/agentic-practices-evaluator` is the scorecard seam: encode
the practices your workload wired as deterministic `Practice` predicates over an
`AgentRun` and score runs — the checklist is caller data by design, and this
ledger is its natural source.
