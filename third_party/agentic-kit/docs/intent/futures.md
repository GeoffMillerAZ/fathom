# Future directions

Things that might happen. **Nothing here is a requirement.** The kit's own rule is
stricter than the practice's: an item exists only with a subscribe-when trigger, and it
is built only when a real workload pulls (VISION junk gate; `docs/roadmap.md` §How to
read this). `Promote when` below is that trigger, quoted. Numbers in parentheses are
roadmap item numbers; P1–P4 are the parked Update-2 items.

Confidence: **likely** (a consumer has named it or a design exists), **plausible**
(fits the vision, no subscriber), **speculative** (listed to stop re-proposal). Horizon:
**next** / **later** / **someday**.

## F1. Generalized context assembler (#2)
- Confidence: plausible. Horizon: later. Motivation: ranked "what goes in the prompt".
- Do not build: a ranking framework; `context-renderer` already assembles by priority.
- Preserve: keep `prose/context-renderer` segment-based; nothing else.
- Promote when: "the first RAG-grade workload where what goes in the prompt is a ranked decision." Drop if: no such workload within two releases.

## F2. Cross-session agent memory (#3)
- Confidence: plausible. Horizon: later. Evidence: `agentic.md` §8 names it as deliberately absent; each `Run` is a fresh conversation by design.
- Do not build: a memory store, a recall layer, summaries that replace sources.
- Preserve: `evidence-span` and journals already persist independently of any window.
- Promote when: "an assistant must remember across sessions." Drop if: consumers keep memory in their own stores — **drop evidence 2026-09-01**: fathom's ADR-028 decided exactly that, an external agent memory is a peer process and never a dependency, with the authority boundary that memory may recall but never assert a fact. The kit's counter-move was the narrow one instead: a run `Handoff` receipt (practices §22), which is not memory.

## F3. Knowledge consolidation flywheel (#4)
- Confidence: plausible. Horizon: someday. Do not build: anything before F2.
- Promote when: "a workload accumulates runs whose lessons are being re-learned." Drop if: F2 dropped.

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
- Promote when: "eval-runner or any bulk enrichment workload lands." Drop if: providers drop batch pricing.

## F7. Multi-provider routing and failover (#13)
- Confidence: plausible. Horizon: later. Do not build: a router; adapters already share one contract.
- Preserve: keep every adapter behind `llmclient.LLMClient` with typed errors.
- Promote when: "uptime across providers matters" to a consumer. Drop if: never.

## F8. Worker-pool executor for swarm waves (#8, uplift D7)
- Confidence: plausible. Horizon: later. Evidence: `designs/eval-waves-d7.md` decided not to fire it.
- Do not build: any executor; `coordination/*` stays plan-only (I20).
- Preserve: agent-harness as the worker unit and live-graph as the planner remain the named shape.
- Promote when: "the corpus grows past what a serial run completes in interactive time, or cross-family multi-wave agreement gains a real subscriber." Drop if: single-wave eval-runner stays sufficient for a year.

## F9. MCP client foundation, consuming side (#14) — PROMOTED 2026-09-02
- Trigger fired and built: `foundations/agent/mcp-client` ships the consuming side (transport, handshake, hash-pinned tool descriptions, tool-registry bridge, annotation mapping). Roadmap #14 graduated; `TRACKING.md` H8-1 remains the one open hardening item (no pre-call pin check on `CallTool`).
- Removed at the next `intent-prune`: a promoted future is history, and git keeps it.

## F10. MCP server: sessions, prompts and resources (#15, remainder)
- Confidence: likely. Horizon: next. Evidence: Streamable HTTP built 2026-07-15 (`designs/mcp-streamable-http.md`); the rest explicitly deferred there.
- Do not build: the deprecated HTTP+SSE transport (non-goal) — and note the target MOVED: MCP 2026-07-28 retires protocol-level sessions outright, so "sessions" here now means the 2025-11-25 shape this kit would be adopting late. Decision `0003` (track revisions with a clock, never chase) governs whether the kit moves to that revision at all; the deprecation floor is 2027-07-28 and `DeprecationsDue` will say so.
- Also parked here: `statehandle`-style principal-bound, expiring handles — the shape a resources/sessions surface needs so a handle is not a bearer token — land it inside `kits/mcp-server` when this fires, never as a package.
- Promote when: "an MCP server must serve non-local clients" needing sessions or resources. Drop if: the kit adopts 2026-07-28, which retires the session half.

## F11. Per-principal budgets and authenticated callers (#17)
- Confidence: likely. Horizon: next. Evidence: `governance` per-identity rolling-day quota and `security/httpauth` landed (D13-scoped).
- Do not build: an identity provider. Preserve: identity as a caller-supplied string.
- Promote when: "the first endpoint with paying or known users." Drop if: never.

## F12. Cross-process trace propagation (#21)
- Confidence: plausible. Horizon: someday. Promote when: "the first multi-process agent topology." Drop if: F8 dropped.

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

## F16. Field anti-patterns awaiting a consumer (2026-09)

The 2026-09-01 field sweep (fathom lanes E3/E4/I3) named twenty-one failure modes.
Six were absorbed as practices §17–§22 because a package or a named consumer carried
them. These are the rest: recorded so they are not re-proposed, one line each, built
only when the quoted trigger fires. Confidence: **plausible** unless noted. Horizon:
**later** unless noted.

- **F16.1 Ambient credentials in tool results.** A secret-shaped string in a tool result enters the conversation; DLP today is egress-only. Promote when: "a consumer reports a credential reaching a transcript through a tool result, or asks for ingress DLP on `PostToolResult`."
- **F16.2 Scope creep on benign tasks.** A declared `TaskScope` (allowed paths/tools, forbidden patterns) enforced at `PreDispatch` rather than inferred from prose. Promote when: "a consumer's agent edits a file outside its brief and the consumer asks the harness — not the prompt — to stop it."
- **F16.3 Self-graded completion.** `CompletionClaim` + `Config.RequireEvidence` refusing `StopCompleted` for a claim with no evidence ref. Promote when: "a consumer gates a run on evidence the harness can see; fathom's `verify` is the proof engine and its `stop-hook` the harness edge — it asks, or it does not fire."
- **F16.4 Prompt-only guards.** A declared `GatePosture{Kind, FailOpen, Covers}` and an assurance report over it (practices §20's missing type). Promote when: "a second consumer needs the posture as data — fathom's doctor `gate_posture` row (HP-15) is the first."
- **F16.5 Non-idempotent replay.** An idempotency key per dispatch plus a side-effect log, so resuming a checkpoint cannot send the second email. Promote when: "the first consumer resumes a run that has external side effects." (The kit resumes; nothing with side effects does yet.)
- **F16.6 Cascading failure without blast-radius isolation.** Failure domains on swarm tasks; aggregation refusing results from a tripped domain. Promote when: "F8 fires" — `coordination/*` is plan-only by I20 and this is aggregation policy, not execution. Horizon: someday.
- **F16.7 Immortal runs.** `RunLease{Owner, ExpiresAt, MaxUSD, MaxTurns}` checked beside the budget charge, plus a revoke flag. Promote when: "a consumer runs an agent long enough that ctx cancellation is not in the operator's hands." Do not conflate with fathom's advisory TTL leases on the fact side.
- **F16.8 Always-loaded context.** A progressive-disclosure ladder (L1 name/description → L2 body → L3 detail) assembled under a budget, deterministically ordered for prompt-cache hits. Promote when: "a consumer's tool or skill descriptions no longer fit the budget they are given." Not F1: a ladder is not a ranker.
- **F16.9 Vague subagent briefs and duplicated fan-out.** A `Brief{Objective, OutputSchema, Tools, Boundaries, Effort}` validated at plan time (duplicate boundaries are an error). Promote when: "F8 fires, or a consumer's planner emits briefs the kit is asked to validate." Horizon: someday.
- **F16.10 Judge bias.** Position swapping, cross-family requirements, reference-anchored rubrics, `Inconclusive` on swap disagreement. Promote when: F14's own trigger — "the first contested or irreversible verdict class." Already parked as F14; listed here so the field's vocabulary maps onto it.
- **F16.11 pass@1 theater.** ABSORBED — practices §21, `foundations/quality/reliability-stats`. Listed only so the sweep's row resolves; no future remains.
- **F16.12 Agent-writable trust configuration.** A `trustconfig.Manifest` hash-locking MCP argv, hook commands, skill trees and deny rules, with typed drift. Promote when: "a second consumer needs the hash-lock primitive — fathom's doctor and custodian are the first, and fathom owns the verbs." Confidence: likely.
- **F16.13 First-match instruction files.** A harness that loads the first matching instruction file and silently ignores the rest, so a nested rule never fires. Promote when: "a consumer reports a harness whose instruction-file resolution the kit is asked to model." Confidence: speculative.
- **F16.14 Parse-failure-means-allow.** A guard whose config fails to parse and whose caller treats the error as a pass. Adjacent to §17 and §20; no separate primitive proposed. Promote when: "a consumer's guard config parse error is observed reaching production as an allow."
- **F16.15 State handles without ownership or expiry.** Folded into F10 above — it belongs inside `kits/mcp-server`, not beside it.
- **F16.16 Per-model compensations.** A `Compensation{Name, ForModel, Reason, RevisitWhen}` list emitting a review-due event when the model changes under it. Promote when: "a consumer keeps harness scaffolding for a model it no longer runs." The kit's own `claudecli`/`codexcli` `Ping` tripwires are the precedent, not the general form.
- **F16.17 Approval theater.** An `ApprovalRequest` unconstructable without the raw input and the effect, with the model's rationale fenced as untrusted. Partly absorbed: §18's gate already routes through `kits/hitl.Preview`, which shows the op's measured impact. Promote when: "a consumer needs the typed request itself — an approval surface where the rationale is model-authored text."
- **F16.18 Firehose tool results.** A result shaper with a bounded size and a steering notice ("showing 1–50 of 1,204; pass `cursor=`"), plus an actionable `is_error` convention. Complements §11: §11 is about SAYING a result was cut, this is about its shape. Promote when: "a consumer's tool returns a payload big enough that the kit is asked to shape it — fathom's MCP surface already caps `pack_context`/`read_source` by hand."
