# 0015: CUE owns shape, projections included; a consumer's test-pinned Go projection is a recorded divergence, not a rule change

**Status:** accepted
**Type:** architecture
**Date:** 2026-09-09
**DIVERGENCE:** consumer `resume_as_code` (racli) ADR-016 §4 against `docs/why-cue.md` §2 and §3.2 and invariant I5. The kit rule stands; the consumer's departure is recorded here, neither absorbed nor papered over.

## Context

The kit's rule: CUE owns *shape*, Go owns *behavior* (VISION belief 1;
`why-cue.md` §2); org-specific policy is CUE data under a schema, never
hard-coded Go (§3.3); `schema.cue` is canonical and `types.go` its mirror —
"one definition, many projections" (§3.2). A projection, which destination
sees which fields at which depth, is configuration, and configuration is CUE.

racli's ADR-016 "one corpus, many projections"
(`resume_as_code/.agents/adr/016-one-corpus-many-projections.md`, code
`cmd/corpus`) keeps one CUE corpus and makes every destination a projection
of it — and its §4 puts the projections in Go, not CUE. Its reasons are
measured, not aesthetic: (1) `TestATSDesignDropsNothing` pins the projection
VALUE, and a CUE-authored projection could silently disagree with the value
the test guards — "policy a test pins lives where the test can see it";
(2) depth filtering runs in `corpus.Project` before the view is built (§5),
which kept four invariant tests unedited and the drop report literally empty,
where both rival designs that filtered inside the view builder had to weaken
`report.Empty()` into a reason-filtered predicate; (3) the model-facing type
is reflected into a strict JSON schema and never widened with corpus identity
fields (§6), so an LLM never gets the vocabulary to invent or renumber identity.
Only §4 conflicts with the kit: §5 is behavior and §6 a type boundary, both
Go's under the kit's own rule.

## Decision

- The rule stands unchanged: shape and org-specific configuration are CUE
  data under `schema.cue`; I5 and `why-cue.md` are not amended; no kit
  package gains a Go-valued projection or policy table.
- The divergence is recorded, with the consumer named by path, and NOT
  absorbed: no `projection` primitive, no Go-projection convention, no
  "test-pinned policy" exception to `why-cue.md`.
- `why-cue.md` §3.2 carries a one-line pointer to this record, so the rule's
  own document shows the departure instead of hiding it in `docs/intent/`.

## Rationale

The kit wants shape checked by a compiler no test author can forget to call;
the consumer wants a value pinned by a Go test a CUE fixture cannot see. Both
are true. The kit is stdlib-only (I1), mirrors CUE by hand and never loads it
at runtime, so neither argument transfers across the boundary. Two
repositories, one rule each; the honest record is the conflict, not a synthesis.

## Alternatives rejected

- **Absorb: a `projection` primitive with Go-valued tables** — one consumer
  (P1, P5, D13). Wins if: a second consumer pins projection policy in a Go
  test and names the need.
- **Amend `why-cue.md` with a "test-pinned policy" exception** — an
  exception with one beneficiary is a hole in the rule. Wins if: the kit's
  own `schema.cue` + `testdata` discipline cannot pin a value a Go test must
  also see (a Go golden test over a `cue export`ed fixture shows the opposite).
- **Ask the consumer to move its projections into CUE** — its ADR records
  the measurement (four invariant tests unedited; rivals weakened
  `report.Empty()`), and a consumer's tree is ask-first (`autonomy.md`).

## Consequences

A reader who finds Go-valued projections in a consumer knows they are
recorded and does not import the pattern into the kit on the strength of that
consumer's tests. The kit gains no code; `why-cue.md` gains one sentence.

## Revisit when

- A second consumer pins a projection, or any org-specific policy table, as
  a Go value under a Go test — absorb or amend, per D13.
- A kit package needs a projection policy of its own: it goes in CUE; if
  `cue vet` plus a Go mirror test cannot pin it, this record is reopened.
- A Go test that asserts over a `cue`-exported fixture lands in the kit —
  the consumer's reason (1) is answered and the divergence may collapse.
- racli's ADR-016 §3 falsifier fires (a `depth` value naming an audience):
  the consumer re-cuts its model and this record's description is stale.
