# 0001: Condition lines may carry an optional `{oracle: …}` annotation

**Status:** rejected, 2026-10-17: its Revisit when fired. Thirty days from 2026-08-27 passed on
2026-09-26 with no hand-written annotation; fathom's intent audit evaluates 0 of 154 conditions.
An author-only annotation cannot appear in a corpus agents mostly write ([0021](0021-intent-practice-v2-enforces-its-homes-adds-no-record-kind.md)).
**Type:** process
**Date:** 2026-08-27

## Context

The four condition labels are prose, deliberately. README §13 rejected a
generated condition registry, and the reason holds: a condition is a sentence
a human wrote for other humans, and a machine guessing what it meant will be
confidently wrong exactly when it matters.

Three repos now carry the practice, and with them a real problem: **a
condition can fire and nobody notices.** fathom's audit (ADR-026) can tell you
an invariant names a test that no longer exists; it cannot tell you that
`Revisit when a second repo shows index drift` came true last month, because
nothing in the sentence is machine-readable.

This does not parse prose. It lets an author who WANTS a condition checked say
so explicitly.

## Decision

A condition line MAY end with an annotation block:

    **Revisit when** a second consumer needs a nested package. {oracle: lexical, pattern: "..."}

Rules:

- **Optional, always.** A condition with no annotation is complete and normal.
  The overwhelming majority will never carry one. Nothing in the practice
  requires, prompts for, or grades the presence of an annotation.
- **The author writes it or it does not exist.** No tool generates one, no LLM
  infers one. An annotation is a human saying "this one is worth checking"; a
  generated one is §13's rejected registry wearing a different hat.
- **The prose stays authoritative.** The annotation says how to check, never
  what the condition means. A tool that cannot evaluate one reports
  `unevaluable` and shows the sentence.
- **Tools preserve it verbatim.** `intent-prune` and `intent-decide` must move
  a condition line without touching its `{…}` block.
- **The grammar's home follows its evaluators.** Today fathom is the only
  evaluator and owns the key set. If a second evaluator appears, the grammar
  moves into this repository and evaluators pin a version.

## Rationale

The alternative to an explicit opt-in is not "no machine checking" — it is
checking that guesses, which §13 correctly refused. An explicit opt-in keeps
that refusal intact while letting the few conditions that ARE mechanical (a
file exists, a symbol is gone, a date passed) say so. Cost is bounded because
the feature is inert by default: a repo that writes no annotation is
byte-identical to one where this was rejected.

## Alternatives rejected

- **A separate conditions registry.** Wins if conditions must be queried apart
  from their prose. Loses now: it splits one fact across two files, the drift
  the practice exists to prevent.
- **Infer the oracle with an LLM.** Wins if precision on a labelled corpus
  beats the cost of a human writing it. Loses for the §13 reason, and because
  a wrong inference is invisible.
- **Keep conditions purely prose.** The status quo, honestly defensible; loses
  only if the demand is real, which the revisit condition measures.

## Consequences

- One optional paragraph in the docs, one commented example in the templates.
- `intent-prune` and `intent-decide` gain a preservation obligation and a test.
- Evaluators become possible, not promised. fathom's oracle runtime is gated
  on demand this record does not create: **three hand-written annotations
  across two repositories.** Below that, nobody has asked, and it is not built.

## Revisit when

Three hand-written annotations exist across two repositories (accept and unblock
fathom's evaluator), **or** thirty days pass from this record's date with fewer
(mark rejected rather than leaving the proposal open).

Asked by fathom as FA-15 (`docs/plans/fathom-uplift-asks.md`); consuming design: fathom ADR-026 / Q18.
