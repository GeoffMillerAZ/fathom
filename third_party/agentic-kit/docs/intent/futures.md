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
- Promote when: "an assistant must remember across sessions." Drop if: consumers keep memory in their own stores.

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

## F9. MCP client foundation, consuming side (#14)
- Confidence: plausible. Horizon: later. Do not build: before an agent needs tools its owner did not write.
- Promote when: "an agent needs tools the owner didn't write." Drop if: never.

## F10. MCP server: SSE, sessions, prompts and resources (#15, remainder)
- Confidence: likely. Horizon: next. Evidence: Streamable HTTP built 2026-07-15 (`designs/mcp-streamable-http.md`); the rest explicitly deferred there.
- Do not build: the deprecated HTTP+SSE transport (non-goal).
- Promote when: "an MCP server must serve non-local clients" needing sessions or resources. Drop if: never.

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
