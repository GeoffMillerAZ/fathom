# Future directions

Things that might happen. **Nothing here is a requirement.** Promote through a named
subscriber or a bounded incubation under decision 0004; move it into scope before
implementation. `Promote when` below states the evidence for review. Numbers in parentheses are
roadmap item numbers; P1–P4 are the parked Update-2 items.

Confidence: **likely** (a consumer has named it or a design exists), **plausible**
(fits the vision, no subscriber), **speculative** (listed to stop re-proposal). Horizon:
**next** / **later** / **someday**.

## F1. Generalized context assembler (#2)
- Confidence: plausible. Horizon: later. Motivation: ranked "what goes in the prompt".
- Do not build: a ranking framework; `context-renderer` already assembles by priority.
- Preserve: keep `prose/context-renderer` segment-based; nothing else.
- Promote when: "the first RAG-grade workload where what goes in the prompt is a ranked decision." Drop if: no such workload within two releases.

## F2. Adapters to consumer-owned memory (#3)
- Confidence: plausible. Horizon: later. Evidence: consumers own recall; 0006 keeps that boundary.
- Do not build: a central memory store or summaries that replace sources.
- Preserve: independent evidence/journals and the run Handoff receipt.
- Promote when: two memory integrations need the same provenance-aware retrieval contract. Drop if: their existing evidence-provider interfaces suffice at that review.

F3's first shared-learning protocol is implemented as `kits/lesson-transfer` (0009);
its two-repository evidence and limits are in the delivery assessment. It does not depend on F2.

## F4. Trajectory-level regression (#6)
- Confidence: plausible. Horizon: later. Evidence: eval-runner (H5) is the substrate; uplift D6 says golden-diff needs no corpus yet.
- Do not build: a second eval runner. Preserve: eval-runner's replay corpus format.
- Promote when: "an agent's behavior must not regress" for a named consumer. Drop if: eval-runner's scenario lane proves sufficient.

## F5. Canned practices catalog (#7)
- Confidence: plausible. Horizon: later. Do not build: before a second workload self-audits.
- Preserve: `quality/agentic-practices-evaluator` keeps the checklist as caller data.
- Promote when: "the second workload wants the same self-audit the first hand-wrote." Drop if: never; the ledger is the seed.

## F6. Batch API support (#11)
- Confidence: plausible. Horizon: later. Do not build: a batch abstraction over the `LLMClient` contract.
- Promote when: an eval/enrichment consumer measures a worthwhile cost saving and can tolerate asynchronous results. The old "eval-runner lands" trigger has fired; the offline runner alone does not establish batch value. Drop if: the measured saving no longer pays for the adapter.

## F7. Multi-provider routing and failover (#13)
- Confidence: plausible. Horizon: later. Ordered failover already ships as `llmclient.NewFailover`; only health/cost-aware routing remains. Do not build: a router without measurements.
- Preserve: keep every adapter behind `llmclient.LLMClient` with typed errors.
- Promote when: "uptime across providers matters" to a consumer. Drop if: never.

## F8. Worker-pool executor for swarm waves (#8, uplift D7)
- Confidence: plausible. Horizon: later. Evidence: `designs/eval-waves-d7.md` decided not to fire it.
- Do not build: any executor; `coordination/*` stays plan-only (I20).
- Preserve: agent-harness as the worker unit and live-graph as the planner remain the named shape.
- Promote when: "the corpus grows past what a serial run completes in interactive time, or cross-family multi-wave agreement gains a real subscriber." Drop if: single-wave eval-runner stays sufficient for a year.

## F10. MCP server: sessions, prompts and resources (#15, remainder)
- Confidence: likely. Horizon: next. Evidence: Streamable HTTP built 2026-07-15 (`designs/mcp-streamable-http.md`); the rest explicitly deferred there.
- Do not build: the deprecated HTTP+SSE transport (non-goal) — and note the target MOVED: MCP 2026-07-28 retires protocol-level sessions outright, so "sessions" here now means the 2025-11-25 shape this kit would be adopting late. Decision `0003` (track revisions with a clock, never chase) governs whether the kit moves to that revision at all; the deprecation floor is 2027-07-28 and `DeprecationsDue` will say so.
- Also parked here: `statehandle`-style principal-bound, expiring handles — the shape a resources/sessions surface needs so a handle is not a bearer token — land it inside `kits/mcp-server` when this fires, never as a package.
- Promote when: "an MCP server must serve non-local clients" needing sessions or resources. Drop if: the kit adopts 2026-07-28, which retires the session half.

## F11. Per-principal budgets and authenticated callers (#17)
- Confidence: likely. Horizon: next. Evidence: `governance` per-identity rolling-day quota and `security/httpauth` landed (D13-scoped).
- Do not build: an identity provider. Preserve: identity as a caller-supplied string.
- Promote when: "the first endpoint with paying or known users." Drop if: never.

## F12. Trace propagation across additional transports (#21)
- Confidence: plausible. Horizon: later. HTTP/MCP trace propagation is implemented (`event/traceparent.go` and MCP trace tests); subprocess/environment conventions remain a separate need.
- Promote when: a consumer's subprocess boundary loses run correlation. Drop if: its HTTP/MCP paths cover the topology. This does not depend on a worker-pool executor.

## F13. Durable VectorStore conformance suite (#22)
- Confidence: plausible. Horizon: someday. Promote when: "the first persistent semantic index." Drop if: consumers stay in-memory.

## F14. Judge pool and panel (P1), judge-model ladder (P2), conviction statistics (P3), leak detector (P4)
- Confidence: speculative. Horizon: someday. Evidence: parked from Update-2 with named triggers (`roadmap.md` §Parked).
- Do not build: any of them. Preserve: nothing.
- Promote when: respectively "first contested or irreversible verdict class"; "judge spend measurable and material"; "score-trend noise actually observed"; "first false-green incident." Drop if: a year passes with no trigger.

## F15. Public release and 1.0 (uplift D4)
- Confidence: likely. Horizon: next. Evidence: H8 Stage A hygiene complete; Stage B blocked on ratification.
- Do not build: `/v1` module path (decided against, `compat.md` §v1 record).
- Preserve: strict semver on `stable` at 1.0; nested modules tagged independently.
- Promote when: Geoff ratifies D4 (`scope.md` G1). Drop if: the kit stays private by decision.

## F16. Field anti-patterns awaiting admission (2026-09)

The 2026-09-01 field sweep (fathom lanes E3/E4/I3) named twenty-one failure modes.
Six were absorbed as practices §17–§22 because a package or a named consumer carried
them. These are the rest: one line each; a fired trigger or 0004 incubation prompts
admission review. Confidence: **plausible** unless noted. Horizon:
**later** unless noted.

For every remaining F16 subentry: Drop if the named failure is covered by an existing
composition or the relevant upstream feature retires; assess at the next annual intent
audit (2027-09-12). No entry authorizes implementation merely by being listed.

- **F16.1 Ambient credentials in tool results.** A secret-shaped string in a tool result enters the conversation; DLP today is egress-only. Promote when: "a consumer reports a credential reaching a transcript through a tool result, or asks for ingress DLP on `PostToolResult`."
- **F16.6 Cascading failure without blast-radius isolation.** Failure domains on swarm tasks; aggregation refusing results from a tripped domain. Promote when: "F8 fires" — `coordination/*` is plan-only by I20 and this is aggregation policy, not execution. Horizon: someday.
- **F16.7 Immortal runs.** `RunLease{Owner, ExpiresAt, MaxUSD, MaxTurns}` checked beside the budget charge, plus a revoke flag. Promote when: "a consumer runs an agent long enough that ctx cancellation is not in the operator's hands." Do not conflate with fathom's advisory TTL leases on the fact side.
- **F16.8 Always-loaded context.** A progressive-disclosure ladder (L1 name/description → L2 body → L3 detail) assembled under a budget, deterministically ordered for prompt-cache hits. Promote when: "a consumer's tool or skill descriptions no longer fit the budget they are given." Not F1: a ladder is not a ranker.
- **F16.9 Vague subagent briefs and duplicated fan-out.** A `Brief{Objective, OutputSchema, Tools, Boundaries, Effort}` validated at plan time (duplicate boundaries are an error). Promote when: "F8 fires, or a consumer's planner emits briefs the kit is asked to validate." Horizon: someday.
- **F16.10 Judge bias.** Position swapping, cross-family requirements, reference-anchored rubrics, `Inconclusive` on swap disagreement. Promote when: F14's own trigger — "the first contested or irreversible verdict class." Already parked as F14; listed here so the field's vocabulary maps onto it.
- **F16.12 Agent-writable trust configuration.** A `trustconfig.Manifest` hash-locking MCP argv, hook commands, skill trees and deny rules, with typed drift. Promote when: "a second consumer needs the hash-lock primitive — fathom's doctor and custodian are the first, and fathom owns the verbs." Confidence: likely.
- **F16.13 First-match instruction files.** A harness that loads the first matching instruction file and silently ignores the rest, so a nested rule never fires. Promote when: "a consumer reports a harness whose instruction-file resolution the kit is asked to model." Confidence: speculative.
- **F16.14 Parse-failure-means-allow.** A guard whose config fails to parse and whose caller treats the error as a pass. Adjacent to §17 and §20; no separate primitive proposed. Promote when: "a consumer's guard config parse error is observed reaching production as an allow."
- **F16.15 State handles without ownership or expiry.** Folded into F10 above — it belongs inside `kits/mcp-server`, not beside it.
- **F16.16 Per-model compensations.** A `Compensation{Name, ForModel, Reason, RevisitWhen}` list emitting a review-due event when the model changes under it. Promote when: "a consumer keeps harness scaffolding for a model it no longer runs." The kit's own `claudecli`/`codexcli` `Ping` tripwires are the precedent, not the general form.
- **F16.18 Firehose tool results.** A result shaper with a bounded size and a steering notice ("showing 1–50 of 1,204; pass `cursor=`"), plus an actionable `is_error` convention. Complements §11: §11 is about SAYING a result was cut, this is about its shape. Promote when: "a consumer's tool returns a payload big enough that the kit is asked to shape it — fathom's MCP surface already caps `pack_context`/`read_source` by hand."

F16.2/.3/.4's bounded profile work is implemented: exact registered-tool scope,
independent completion checks and coverage-aware native configuration. This is not
filesystem isolation or universal native completion enforcement. F16.5/.17's bounded
replay/approval work is implemented with `hitl.Committer` and the sourced-prose scenario;
destination adapters remain consumer-owned. The synchronous effect gate's preview is
descriptive, not a measured remote dry run. Production effectiveness remains unmeasured.

## F17. Incident-readiness declaration and facet values (practices §27)
- Confidence: plausible. Horizon: someday. Evidence: one consumer's reviewed design (2026-09-29); nothing built, nothing measured. Guidance: [`incident-readiness.md`](../incident-readiness.md).
- Do not build: a declaration package, a probe renderer, an event-attach endpoint, an incident command center, or any connector that fetches or polls operations systems.
- Preserve: `quality/structural-refusal` and `telemetry/read-manifest` as the shapes for "not computed: why"; effect annotations (§18) as the only gate a rendered command may rely on; the consumer owns its declaration schema. The probe, next-query and attach triggers live in the guide (sections 6 and 7).
- Promote when: "a consumer's replay over at least 10 change-caused incidents reads BUILD (two arms at equal list length, bars fixed first) and a second consumer asks for the same declaration schema or facet-value type." Drop if: neither happens by the next annual intent audit (2027-09-12).

## F18. Commit-time lost-pin check (practices §28)
- Confidence: plausible. Horizon: someday. Evidence: one consumer's accepted design (2026-09-29); its replay flagged both known violations with no false loss over 50 commits; nothing built here, behavior change unmeasured. Guidance: [`serving-rules.md`](../serving-rules.md).
- Do not build: a promise store or open set, a state ledger, waivers, a model that sets kept or broken, a bypass in the failure message, a warn mode no rule uses, or a per-edit single-file check (a moved test reads as deleted).
- Preserve: `quality/structural-refusal` and `telemetry/read-manifest` as the shapes for a check that could not run ("skipped", never "kept"); the loss kinds (deleted, renamed, weakened, skipped, golden changed, marker moved) as the only shapes; the consumer owns which sources name a pin.
- Promote when: "a second consumer runs a commit-time lost-pin check in record mode with no adjudicated false fire in 30 days and asks for the same loss-check type." Drop if: neither happens by the next annual intent audit (2027-09-12).

## F19. Incident responder over a code-evidence engine that reports an answer basis (practices §27, guide section 11)
- Confidence: likely (owner direction 2026-10-11; fathom's assessments place it here). Horizon: later. Evidence: a design only. fathom serves no answer basis, fact sheet or fact receipt yet ([fathom plan](../plans/fathom.md) sections 2 and 3). Guidance: [incident-readiness section 11](../incident-readiness.md).
- Do not build: a kit package for the fact sheet, the answer basis or receipts; an engine-specific skill in the kit (proposed [0019](decisions/0019-incident-evidence-doctrine-in-kit-engine-binding-in-engine.md)); anything that mitigates, runs a probe, fetches or names a cause.
- Preserve: section 11's basis table and receipt rule; `quality/structural-refusal` and `telemetry/read-manifest` as the shapes for `unknown` and lower bounds; decision 0002's declared effects for an agent that registers the engine's tools.
- Promote when: "fathom serves answers with a basis and one service's fact sheet, and a named consumer with an on-call owner runs one incident or replay session through them", or fathom's incident replay (IM-M0) reads BUILD. Drop if: no fact sheet ships by fathom's program kill date (2027-01-31) or the next annual intent audit (2027-09-12), whichever comes first.

## F20. A git-native anchor on the evidence span (fathom ADR-063 O9, ADR-068 decision 12)
- Confidence: plausible. Horizon: later. Evidence: fathom aligns its receipt anchor with a review tool's `SpanRef` "and its planned evidence-span successor". `evidence-span` has a line `Locator` and a 64-hex `RecordHash`, but no commit, no git blob id, no anchor state, and an `Author` that is only human or ai, so a deterministic tool's fact cannot pass `ValidateAsEvidence` honestly ([fathom plan](../plans/fathom.md) section 3.4).
- Do not build: a receipt or result format of the kit's own (fathom ADR-063); a Merkle tree over facts; an import of fathom code.
- Preserve: `Span` grows only by optional fields (backward compatible); `RecordHash` stays sha256; the two vocabularies stay separate types under the naming audit.
- Promote when: "two consumers commit to reading it: fathom ships `export receipt` (its EVX-9) and the review tool swaps `SpanRef` for `evidence-span`" (D13). Drop if: the review tool keeps `SpanRef` at its next release and fathom names no kit consumer for its receipts, or at the 2027-09-12 audit.
