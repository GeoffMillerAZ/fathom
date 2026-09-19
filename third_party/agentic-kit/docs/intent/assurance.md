# Workflow assurance

The target is to make good agentic engineering easy to inherit and possible to verify.
This is the acceptance contract for that target, not a claim that every workflow below
already exists. Decisions [0005](decisions/0005-workflow-assurance.md) and
[0006](decisions/0006-evidence-and-lessons.md) govern it. Current evidence and gaps:
[delivery assessment](../assessments/2026-09-13-governed-workflows.md).

## What a claim must identify

For each practice: stable ID, failure mode, applicable workflow, required configuration,
enforcing component and execution paths, error/timeout behavior, independent check,
evidence snapshot, supported host/version, and residual consumer responsibility.
`agent-harness.CoverageReceipt` reports controlled Go runs; `kit-new host` reports
native configuration and observed/unsupported entrypoints. They are separate scopes,
not interchangeable credentials or claims that every listed control ran.

Keep two axes separate:

| Implementation | Meaning |
|---|---|
| designed | a hypothesis and acceptance criteria exist |
| implemented | the component and its local tests exist |
| integrated | a composed entry point exercises it, including failure cases |
| adopted | a consumer uses that composition at a recorded revision |
| measured | consumer outcomes and cost have been compared against a baseline |

| Enforcement | What it can establish |
|---|---|
| structural | a controlled path refuses a prohibited action before its effect |
| deterministic check | specified input/output/state properties hold for the checked scope |
| heuristic | a model or rule estimates quality/risk; calibration and misses are reported |
| advisory | instructions, warnings, or reviews can influence behavior |
| unknown / unsupported | the surface lacks evidence or cannot provide the control |

`stable` / `experimental` / `internal-ish` still mean API compatibility, not these axes.
Record `pass`, `fail`, `unknown`, and `not-applicable` separately; the latter needs an
applicability reason. A required check that did not run cannot yield a green receipt.
Versioned assessment profiles (0012) keep required constraints separate from optional
scoring. Retain criterion/subject identities, evidence method and freshness, the rule
trace, and inputs needed to reconstruct it. A caller-declared pass is not verified fact.
For model screens, measure false refusals and missed violations alongside held/error
rates and cost; assert actual blocked effects, not only emitted classification labels.

## Workflow profiles to prove

Names describe product workflows, not additional packages to scaffold automatically.

| Workflow | Compose from existing pieces | Required proof |
|---|---|---|
| Governed engineering | agent-harness, tool-registry, budgets, compactor, replay, intent, Fathom | installed policy covers the declared tools; missing wiring refuses; completion is checked against the task's acceptance evidence |
| Evidence-backed prose | evidence-span/resolver, prose-plan, traceable-prose, governed-prose-kit, review | each factual claim resolves to its exact source version; unsupported and stale claims block the strict publication path |
| Human-approved changes | hitl, effect gate, operator-elicitation, correction ledger, checkpoints | identity, operation, target version and expiry bind the approval; stale approval and replay cannot duplicate an effect |
| Shared lessons | lesson-transfer, receiver-owned policy/reproducer and regression evidence | a redacted lesson reproduces and prevents a seeded false pass in another repo; current revocation prevents re-adoption |

## Acceptance dimensions

- **Authority:** permitted actions, resources, identities, budget, and stop/revoke paths
  are explicit. A model-generated rationale cannot grant more authority.
- **Evidence:** facts, source records, derived claims, and generated text remain
  distinguishable. A hash verifies bytes; a judge assesses support; neither makes a
  source true. Unresolved references and incomplete coverage remain visible.
- **Reliability:** assert final state as well as transcripts. Use deterministic
  adversarial cases for controls; use held-out cases and repeated independent live
  trials for model quality. Replaying one cassette repeatedly is one observation.
- **Recovery:** interruption preserves state and evidence. Checkpoint restoration does
  not by itself make external actions idempotent; ambiguous outcomes require reconciliation.
- **Cost:** measure adoption effort, overhead, actual usage, retries and operator time.
  Bound execution and storage growth; state how retention affects later replay.
- **Portability:** provider and host capabilities are explicit, versioned and tested.
  Unsupported functionality is a typed result, not a silently dropped parameter.
- **Learning:** incidents become reusable checks and reviewed remedies. Measure repeated
  failure and successful transfer, not how many lessons were written.

## Targets and measurement boundaries

- A fresh Go consumer reaches an offline governed scenario within 15 minutes using the
  guide alone; a Claude Code and a Codex install each produce a capability receipt.
- All required controls in the selected profile reject their planted failure cases;
  removal, parse failure, timeout and missing prerequisites cannot count as a pass.
- The prose pilot resolves every factual claim in its publication fixture; corruption,
  unsupported claims and source revision each fail the publication gate independently.
- One approved change survives interruption without duplication; stale state, changed
  arguments, expired approval and the wrong actor each prevent commit.
- One sanitized consumer incident becomes a regression check successfully adopted by a
  second repo, with a recorded before/after outcome and a tested revocation path.
- Report task success, unsupported-claim rate, false blocks, repeated-run reliability,
  latency, tokens/cost and human review time separately. Choose quality thresholds with
  the pilot's baseline; do not present synthetic scores as production reliability.

Review these targets at each incubation review (0004). More packages or more rules are
not evidence of a better outcome. Add a control when its demonstrated benefit exceeds
its maintenance, latency, context, and approval burden.

Synthetic scenario counts and agent-assisted setup time are not production quality,
human usability or saved operator time. Native configuration-only receipts make no
tool-enforcement claim; global trust, permissions and unhooked entrypoints remain
outside the adapter. Use the delivery evidence before promoting any of these axes.
