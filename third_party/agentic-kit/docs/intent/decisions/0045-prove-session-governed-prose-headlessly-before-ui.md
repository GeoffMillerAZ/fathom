# 0045: Prove session-governed prose headlessly before UI

Status: accepted
Type: architecture
Date: 2026-10-24

## Context

The owner explicitly authorizes a separate Anthropic API-backed Claude Code lane,
real synthetic prose experiments and OpenRouter/Jev evaluation. Session-specific
goals and reusable document types are missing runtime contracts. Existing sourced
prose and HITL are useful substrates, not the hierarchical engine described in PH.
The owner makes headless proof a prerequisite for further workbench UI work.

## Decision

Admit the [headless prose proof](../../designs/prose-headless-proof.md) as the active
reference workflow in the existing second incubation slot. Command-tool expansion,
including the unadmitted GT15 journal, waits; its full objective remains open.
This is a change of focus, not a third incubation. Preserve 0004/0038 and the first
quality-gate maintenance slot. Budget: five engineering days, <=64 MiB synthetic
corpus, first paid stage <=$25 ($15 Claude Code, $10 direct experiments including
Jev), maintainer ownership, review on budget exhaustion/next release/2026-11-21.
Reserve spend before dispatch and retain failed/canceled/unknown attempts.

Use isolated worktrees and explicit ownership. The owner controls independent
scope/oracles, study execution and integration. The Claude lane implements the
prose kit and its headless scenario. Merge reviewable units through full gates and
actual affected-consumer checks. No independent push to main by the worker.

Add immutable session intent and document-type profile bindings to an additive
strict document composition. Catalog/Assignment/HITL remain separate owners,
composed by the consuming scenario/lab. Generic freeform work uses explicit base
requirements and task-local goals; it is not an evidence-policy bypass.

Separate generated-content reuse, evidence rebinding, review invalidation and
deterministic rendering. Exposed context is a dependency. A decision model may
widen impact or recommend escalation, never shrink a required closure, clear a
hard hold, authenticate a source, certify truth or authorize publication.

Run representative real generation and bounded decision experiments with public
synthetic inputs. Independent expected claims, conflicts and affected-node sets
must exist before model calls. Measure fidelity, false acceptance, completeness,
reuse, actual calls/tokens/cost and failures. Preserve source-integrity, semantic
support, source truth and effect authority as separate results. Do not infer
general accuracy, calibrated confidence or production readiness from this cohort.

Only after the headless entry gate passes may a new app-kit Herdr lane implement
the reusable UI. Use the existing PH/HW/PX/AK packets and app-kit contribution
loop. Headless proof cannot satisfy browser, accessibility or attended-task gates.
No current UI implementation, private-input disclosure, deployment, global hook
change, new dependency, command-classifier paid run or real publication is admitted.

## Rejected alternatives

- More visual polish before semantic proof: reverses the owner's requested order.
- One large generation call after every correction: loses locality and cost control.
- Citation presence or model confidence as factual acceptance: contradicts 0026.
- A model-selected smaller closure: can silently preserve stale dependent text.
- Adopt Jev because it is fast: retain it only where paired evidence improves the
  selected task without new critical misses; record negative results explicitly.
- A new universal goals/catalog/editor engine: compose the current owners instead.

## Revisit when

A required fault escapes, closure/context is incomplete, session/profile changes
reuse stale consent, costs exceed reservations, a model degrades the paired task,
the headless gate fails or budget/review date arrives. The full G12/G13, PH/HW/PX
and governed-command objectives stay open until their own evidence exists.
