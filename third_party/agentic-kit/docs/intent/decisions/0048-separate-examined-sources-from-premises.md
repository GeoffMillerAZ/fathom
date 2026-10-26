# 0048: Separate examined sources from supporting premises

Status: accepted
Type: data
Date: 2026-10-26

## Context

HP-22's [protocol 3 trial](../../evidence/prose-headless/2026-10-26-qualified-review-v3/README.md)
stopped at 67/85, with 66 matches and 18 unexecuted positions. The model correctly
identified a citation gap: the cited procedural definition did not establish the
asserted event, while an uncited observation contained the assertion. It reported
Unsupported and no supporting premise or contradictory fact. The closed response
contract required at least one premise/counter reference and rejected the reply.
The native response and its known cost remain retained; the site is unqualified.

## Decision

Preserve that failed protocol 3 trial, every denominator and the source-reviewed
criterion 2 floor. Add a separately selected response protocol 4 with required
`examined_fact_ids`, distinct from `premise_fact_ids` and `counter_fact_ids`.
Examined IDs are a nonempty unique subset of exposed source facts. Premise and
counter lists are unique within each list and subsets of the examined set.
Positive premises remain restricted to cited facts; Supported needs at least one.
A citation/missing-premise gap may have no premise or counter while retaining
explicit examined-source provenance. Overlap between lists is permitted.

These are declared source relationships, not authenticated attention or truth.
Reason, criterion, source and native request/response bindings remain mandatory.
Separate source authority, source truth, semantic support and effect readiness.
No model answer authenticates the evidence or grants acceptance/publication.

Version the trial intake and prompt/codec together. Keep old /1 trial selection,
protocol 3 requests, decoder, artifacts and native replay byte-compatible. A new
trial /2 explicitly selects protocol 4. Missing/foreign fields and unknown versions
refuse. Qualify all 85 unchanged cases again, under a new freeze and prior-accounting
receipt; do not resume the failed journal or reinterpret its invalid result.
This stays command-local inside0045's existing incubation and remaining budget.

## Alternatives rejected

- Relax protocol 3 after observing its reply: erases a frozen contract failure.
  Wins if: never.
- Invent a positive premise or counter just to satisfy shape: corrupts evidence
  semantics. Wins if: never.
- Accept rationale text without structured provenance: loses deterministic source
  binding. Wins if: never for this strict profile.
- Extract a universal evidence engine now: one lab does not establish the API.
  Wins if: two concrete consumers need the same qualified relationship contract.

## Revisit when

A structured judgment cannot represent its source basis, an unsupported assertion
is cleared, a prior native replay changes, a second consumer needs this contract,
or0045's budget/review conditions fire. Human UI and document acceptance remain open.
