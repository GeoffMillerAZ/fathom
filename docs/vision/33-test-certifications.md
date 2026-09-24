# 33 — Tested means certified

Status: accepted 2026-09-23 (ADR-043); stages 0-3 landed 2026-09-23 (the
`certifications` layer, `verify certified_by`, the review dimension, the
`certify` verb family, receipts, fathom's own declarations — TC-1..24 except
the TC-M op measurements). Assessment: the owner's question of 2026-09-23,
"should fathom track test coverage, test kinds and what tests are for, on a
standard defined in platform-engineering?" Plan:
`docs/plans/test-certifications.md`. Design: `docs/design/test-certifications.md`.

## 1. The thesis

"Done means proven" is fathom's one proven capability: a plan checkbox is a
claim, and `fathom verify` checks it against the index. This vision extends it
one step: **tested means certified**. A test is evidence for a behaviour the
organization has decided matters — an invariant, a spec, a governing rule, an
incident, a port — and that evidence has the same shape as every claim fathom
already keeps: a statement about a place in the code, made by a named producer
with a stated method, that must be re-checked when the place moves.

A **certificate** says: *test T, of kind K and strength S, certifies target X*.
fathom does not decide whether that is enough. It asserts whether the
certificate is **live**: the test exists, reaches the symbols the target names,
covers them in the layer the operator labelled, kills mutants in them when a
report exists, and passed in the last attested run without flaking. It lists
what has **no certificate at all**, with the remedy on the line. Adequacy is
Review Kit's requirement, with the organization's own remedy text and
enforcement; the vocabulary is the platform standard's (test-kit, ADR-0042).

## 2. Why now, and why fathom

Three repositories already hold two thirds of this and agree on the boundary:

- **test-kit** declares suites (class, floors, tier, a required port on a
  conformance suite) and attests results per case (pass, fail, skip, flake,
  attempts, coverage, artifact references). Its own open list says coverage is
  "carried but nothing measures it yet".
- **Review Kit** judges requirements through evidence checks with adapters,
  minimum tiers and enforcement, already calls fathom for impact, obligations,
  architecture and spec checks, and already publishes findings into fathom's
  findings layer.
- **fathom** knows which tests reach which symbols (`test_impact`), which
  symbols each labelled layer covers (`fathom tests coverage`), which mutants
  survive, and it already says "fathom asserts a fact. The review kit judges
  it."

Nothing links a test to *what it is for*, and nothing computes "which
invariants, incidents, specs and ports have no certifying test". That join is
claim-shaped, and fathom is the claim engine.

## 3. What never changes

- **fathom asserts a fact; the review kit judges it.** No threshold, no
  percentage, no "adequate" lives in fathom. A certificate's state is a fact
  with an evidence tier.
- **No model in the default path.** Declarations plus deterministic joins
  compile the certifications layer; no tokens are spent. The decision port may
  *suggest* certificates for unregistered tests (a choice over the strength
  ladder and a noul "does this test assert behaviour"), in the proceed band only,
  and a suggestion is never a claim (ADR-042's authority boundary).
- **State is computed, never stored by hand** (ADR-018, P17). The manifest
  artifact is regenerated from declarations and joins; a hand edit is a defect.
- **fathom never runs tests** (scope non-goal: no dispatch loop). The attested
  run is test-kit's record; fathom ingests it as an evidence producer.
- **A reached test is not a passing test.** Without an attested run a
  certificate is `unproven`, never `live`. A stranger's receipt says "pass:
  unverified (no run)" until they run it.
- **Costs scale with the change** (P19): the layer recompiles from the
  declarations and runs that changed, hash-gated like everything else.

## 4. The four places a certificate is served

1. **The gate.** A plan row gains `certified_by: <Test>`; `fathom verify` fails
   the row until the test exists, reaches the row's symbol and passed in the
   last attested run. An agent can no longer write "tested" any more than it can
   write "done".
2. **The review.** `review_diff` and `fathom review` gain a `certifications`
   dimension: live certificates per changed symbol, and targets in the changed
   files that lost one.
3. **The list.** `fathom certify gaps`: every declared target with no live
   certificate; `fathom certify manifest`: the regenerated artifact.
4. **The judge.** A Review Kit requirement with a fathom evidence adapter, for
   example "every deny-severity invariant has a live certificate of strength at
   least `asserts`", decides and enforces.

The edit hook does not serve certificates in this round: its payload is capped
at 150 tokens and already over budget on half of edits (Q21). A one-line "no
live certificate governs this file" at the edit is a measured row with its own
kill criterion, not a default.

## 5. What it opens

- **Incident to regression.** `docs/intent/incidents.md` promises "every
  incident yields an invariant, a test, or a decision"; the gap list makes
  "a test" checkable.
- **Invariant to enforcement.** The intent audit's `test` label gains a
  certificate, a strength and a state.
- **Port to conformance, fleet-wide.** test-kit requires a port on a
  conformance suite; fathom lists an interface's implementers with evidence;
  the join names every adapter with no conformance certificate across every
  indexed repo.
- **Flakiness as staleness**, contract certificates on cross-repo edges,
  certificate receipts, certifying-tests-first selection, orphan tests: each a
  row in the plan with its own bar.

## 6. Kill criteria

Each stage carries one (P5): an envelope-to-symbol join rate under 80 percent
kills stage 0's design; fewer than half of fathom's deny-severity invariants
certified after two weeks of dogfood kills the declaration path; a Review Kit
requirement that never fires deny on a real change in a month is theatre;
suggested certificates accepted less than half the time are noise.
