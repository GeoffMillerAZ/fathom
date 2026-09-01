# Agentic UX — the human side of agentic-kit

Status: envisioning (2026-06-17, owner-directed). This is the design north star
for app-kit's **agentic interaction** surface: the components a human needs to
(1) make decisions in an agent's loop (human-in-the-loop), (2) see what an agent
is doing and did and why ("under the hood"), and (3) converse with an agent
through a world-class chat.

It is grounded in a survey of the real `agentic-kit` capability surface (≈24
concepts across `foundations/` + `kits/`) and the current state of app-kit
(`appkit/ui/hitl` = 4 components; **no chat component exists**) and Throughline's
bespoke Expert Chat (the de-facto reference to learn from and eventually replace).

Principles (binding): D5 loopback (server-rendered HTML, no shell-specific JS;
interactivity via tiny vanilla scripts like `overlay.js`/`router.js`), D2 no Node,
D7 tokens (every color a role utility), pure-Go SVG for any viz, deterministic +
escaped + accessible. Agentic-kit ships typed Go surfaces with no UI — the kit is
where they become human-legible.

---

## What already exists (the seed)

`appkit/ui/hitl`:
- `ApprovalCard` — a proposed action awaiting approve/edit/reject (intent, risk, optional confidence, caller-supplied actions).
- `RunTrace` — vertical status rail of `TraceStep`s (done/active/pending/failed).
- `DiffView` — before→after change rows.
- `ToolCall` — one tool invocation (name/status/args/result, collapsible).

These map cleanly onto agentic-kit but are point components. The vision below
adds the **foundational primitives** that make the rest cheap, the **workbench**
compositions, and the **chat** the kit is missing entirely.

---

## Pillar 0 — Foundational primitives (build first; highest reuse)

These appear in a dozen of the specific components below. Build them once.

1. **`AuthorBadge`** — human avatar vs. AI model chip from `aiprovenance.Author`
   (Kind, AIAuthor{ModelID, ModelVersion, PromptHash, …}). Expands to a popover
   with the full reproduction key (prompt/params/input hashes, timestamp),
   click-to-copy. Used by: evidence spans, corrections, journal entries,
   hypotheses, leads, chat messages.
2. **`EvidenceTierPill`** — color-coded pill for `evidencegrade.Tier`
   (direct/documentary/circumstantial/testimonial/inferred) + confidence. Used by:
   span cards, hypothesis list, claim lifecycle, conflict display, chat citations.
3. **`SeverityChip`** — one chip for the shared `low/medium/high/critical` scale
   (injection-detector, dlp-evaluator, concern-flags, review-queue priority).
4. **`StatusBadge` (generic enum→pill)** — drives `StopCause`, `RunStage`,
   `LeadStatus`, `ClaimState`, `WorkflowState`, convergence `Verdict`, circuit
   breaker state. One token-mapped badge with an icon set, fed any enum.
5. **`FSMActionBar`** — given `currentState` + a `map[state][]event`, render only
   the legal next-state buttons. Drives correction-ledger, claim-lifecycle,
   lead Kanban, plan-refinement — every FSM gets a correct, self-limiting control.
6. **`ConfidenceMeter` / `ScorePips`** — 0..1 meter and 1..5 pips (confidence-
   scoring, honesty-gate, eval-gate, practices scorecard). (viz.Rating already
   covers pips; add the labeled meter.)

These are small, pure, token-themed, and unblock everything else.

---

## Pillar 1 — HITL decision surfaces (the "workbench")

The agent pauses; a human decides. Each is an `ApprovalCard`-class surface but
shaped to its decision. Grouped by agentic-kit source:

| Component | agentic-kit source | The decision |
|---|---|---|
| **PendingQuestionsInbox** | `operator-elicitation.Request`, `clarification-loop.Question` | Answer a free-text question the agent blocked on (inline answer field; resolves in place). |
| **PlanReviewPanel** | `planning-refinement.{Plan,Signal}` | Accept / Needs-work / Reject an iterative plan; shows round counter + escalation badge; diff vs. prior round. |
| **FindingReviewCard** | `correction-ledger.{Finding,Correction,Dismissal}` | Claim → correct (new value + reason + 1-5 confidence) or dismiss (rationale); FSM-gated actions. |
| **ClaimLifecyclePanel** | `claim-lifecycle.{ClaimState,ClaimEvent}` | Advance a claim's epistemic state + link evidence; FSM-gated. |
| **ReviewQueue** | `review-queue.{ReviewItem,ReviewPriority}` | Pick the next item, critical-first; priority-sorted inbox shell. |
| **ImprovementProposalCard** | `improvement-flywheel.Proposal` | Approve/reject an AI-proposed *system* change (pattern type, frequency, affected target, supporting runs). "Human approval before adoption" is a hard rule. |
| **ToolPolicyMatrix** | `tool-execution-policy.{ToolPolicy,Decision}` | Toggle which tools are allowed in which environments (deny-by-default grid). |
| **BudgetConfigPanel** | `budget-guard.{Config,Snapshot,Decision}` | Set req/min, req/day, USD/day caps; live gauges; tripped → red banner + reason. |
| **InjectionPolicySelector** | `agent-harness.InjectionAction` + `injection-detector.Severity` | Off / Annotate / Block + severity threshold. |
| **HonestyGateReview** | `honesty-gate.{Claim,Verdict,Decision}` | Override allow/hedge/block on a low-evidence claim. |
| **DLPReview** | `dlp-evaluator.{Report,Finding,RiskBand}` | Review/redact/allow/block flagged content before it leaves the system. |

The **HITL Workbench** itself is a page pattern: `ReviewQueue` (left) → the
selected decision surface (center) → relevant context/evidence (right). The kit
ships the pieces; an app composes the workbench with its own data + endpoints
(loopback POSTs), exactly as Throughline wires `ApprovalCard` today.

---

## Pillar 2 — Observability surfaces ("under the hood")

What the agent is doing (live) and did (review). `RunTrace`/`ToolCall` are the
seed; the agentic-kit event/journal/ledger types make a real timeline possible.

| Component | agentic-kit source | Shows |
|---|---|---|
| **LiveEventFeed** | `telemetry/event.{Event,Kind}` + harness `Attr*` constants | Streaming log: tool_call / llm_call / error / info, with turn, duration, tool names, severity. Live tail. |
| **RunSummaryBar** | `agent-harness.{RunResult,StopCause}` | Turn count, tool-call count, stop-cause badge, token totals — the "what just happened" strip. |
| **JournalTimeline** | `investigation-journal.Entry` (Step/Note/Author/SpanRefs) | Append-only step timeline with author badges + clickable span refs + as-of scrub. |
| **TokenCostMeter** | `llmclient.Usage` + `cost-tracker.Totals` | Input/output/cache tokens + USD; breakdown by model/label. |
| **QuotaGauges** | `budget-guard.Snapshot` | req/min, req/day, USD/day progress bars ramping green→red. |
| **EventChainExplorer** | `provenance/event-store.Entry` (hash-chained) | Seq/Kind/time table + one-click chain-verify (broken links flagged). |
| **ConversationWindowIndicator** | `agent/conversation.Window` | Which turns are in-context vs. dropped/compacted; "N messages dropped" at the cut. |
| **TaskTreeView** | `coordination/task-tree.TaskNode` | Collapsible decomposition tree with rolled-up status. |
| **WorkflowInstanceViewer** | `workflow-engine.{WorkflowInstance,TransitionRecord,GuardEval}` | FSM diagram (current state highlit) + transition log with guard pass/fail. |
| **HypothesisRankedList** | `hypothesis-ranker.Hypothesis` | Ranked cards: statement, support bar, tier badge, author, span refs. |
| **ConvergenceMeter / DADPhaseIndicator** | `convergence-oracle.Verdict`, `dad-investigator.Phase` | Investigation progress: phase stepper + converged/exploring/needs-sources/contradicted. |
| **ConflictGraph** | `evidence/conflict.Conflict` | Node graph of disagreeing evidence; edges colored by kind, weighted by severity. |
| **EvidenceSpanCard / Popover** | `evidence-span.Span` | Entity/field/value + tier + confidence + locator (click-to-source) + provenance + hash. The hover target for any `SpanRef`. |
| **NarrativeShiftDiff** | `narrative-shift.NarrativeShift` | Side-by-side From→To with shift-kind badge + gap-days. |
| **PracticesScorecard / EvalGateResult** | `agentic-practices-evaluator`, `eval-gate` | Score gauge + per-check pass/fail with weights. |

Note (already learned, [[appkit-mcp-built]]): TailwindPlus has **no charts** — all
of the above viz (graphs, gauges, timelines, feeds) is kit `ui/viz` / pure-Go SVG,
not a TailwindPlus pull. TailwindPlus helps with the *chrome* around them
(cards, tables, lists, headings) which we now mine.

---

## Pillar 3 — Chat (the kit's missing core) — `appkit/ui/chat`

app-kit has **no chat component**. Throughline hand-rolled a strong one; the kit
should own a world-class, reusable, loopback-safe chat so every app gets it. The
package is `appkit/ui/chat`. Server-rendered bubbles + transcript; a single small
`chat.js` (like `overlay.js`) for the streaming/compose creature comforts, kept
strictly within D5 (no framework, app owns the endpoints/SSE).

**Composition:**
- `Transcript` — message list; `Message` (role, author via `AuthorBadge`, rich
  body via trusted HTML, timestamp, actions).
- `MessageBubble` — user / assistant / system variants; markdown body slot;
  **inline `ToolCall` + `RunTrace`** blocks (reuse hitl!) so "under the hood"
  lives *in* the transcript — the bridge between Pillar 2 and chat.
- `Composer` — auto-grow textarea, model picker slot, attach slot, send;
  Enter-to-send / Shift-Enter newline.
- `Citations` / `SafetyFlags` disclosures (from `evidence-citation`,
  `injection-detector`) — the anti-fabrication + safety surface.
- `chat.js` behaviors: SSE delta streaming, autoscroll + "jump to latest",
  thinking indicator, draft persistence, copy, **stop**, **regenerate**.

**Creature-comfort checklist** (✓ = Throughline already proved it; ✦ = gap the kit
should fix as a first-class feature, learned from the Throughline audit):

✓ SSE streaming · ✓ threads (create/rename/pin/archive/switch/search) · ✓ history/
persistence · ✓ markdown + code blocks w/ copy · ✓ citations/sources · ✓ safety
flags · ✓ model picker · ✓ draft persistence · ✓ Enter/Shift-Enter · ✓ auto-grow ·
✓ error+retry · ✓ empty states · ✓ autoscroll + jump-to-latest · ✓ thinking
indicator · ✓ copy message · ✓ dock + full-page modes · ✓ AI authorship mark
✦ **Stop / cancel generation** (server cancel is wired but has no button)
✦ **Regenerate last response**
✦ **Token / cost display** (per-message + thread totals — wire `llmclient.Usage`)
✦ **Tool-calls in the transcript** (`hitl.ToolCall`/`RunTrace` inline — currently absent)
✦ **Attachments → prompt** (Throughline uploads but never injects them; the kit
  contract should carry attachment refs end to end)
✦ **Message actions** beyond copy (thumbs up/down → feeds `improvement-flywheel`;
  edit-and-resend; branch/fork)
✦ **Streaming markdown** (render incrementally, not plain-then-swap, to kill the flash)

Building chat in the kit means Throughline *replaces* its bespoke dock with the
kit component (less code, the ✦ gaps fixed for free), and every future app gets
chat on day one.

---

## Recommended build order

1. **Pillar 0 primitives** — `AuthorBadge`, `EvidenceTierPill`, `SeverityChip`,
   generic `StatusBadge`, `FSMActionBar`, `ConfidenceMeter`. Cheap, unblock the rest.
2. **`appkit/ui/chat` MVP** — Transcript + MessageBubble (with inline ToolCall) +
   Composer + `chat.js` (stream/stop/regenerate/draft/autoscroll) + token meter.
   Highest user-visible value; the owner called it out explicitly. Validate by
   porting Throughline's dock onto it.
3. **Observability quick wins** — `RunSummaryBar`, `LiveEventFeed`,
   `TokenCostMeter`, `JournalTimeline` (compose existing `RunTrace`/`ToolCall`).
4. **HITL workbench surfaces** — `ReviewQueue` + `FindingReviewCard` +
   `PlanReviewPanel` + `PendingQuestionsInbox` (the most common pause points),
   then the policy/budget/honesty/DLP gates.
5. **Investigation viz** — `HypothesisRankedList`, `ConvergenceMeter`,
   `ConflictGraph`, `EvidenceSpanCard` (Throughline + any investigation app).

Each ships to the gallery (D12) as it lands, and proves out in Throughline.

---

## Open questions for the owner

- **Chat first, or primitives first?** Primitives are the cheaper foundation, but
  chat is the headline ask. Recommendation: primitives (1 short pass) → chat MVP.
- **How agent-kit-coupled should the kit be?** Proposal: the kit imports *no*
  agentic-kit types — components take plain view structs (like `hitl.ToolCall`
  does), apps map agentic-kit → kit. Keeps the kit dependency-free and reusable.
- **Where does the workbench page live?** Kit ships the pieces; the composed
  "workbench" layout is an app concern (or a gallery example), not a kit page.
</content>
