# 34 — Refactored means conserved

Status: accepted 2026-09-24 (ADR-044). Assessment: the owner's question of
2026-09-24, "how can fathom strengthen its refactoring capabilities, as
refactoring becomes the main activity of agentic coding?" Plan:
`docs/plans/refactor-conservation.md`. Design: `docs/design/refactor-conservation.md`.

**Wave 2 status (2026-09-24).** Landed: stages 1–3 (proof integrity, the
conservation ledger, absence/anti-Blindness) fully proven; stage 4
(`fathom refactor plan`/`refactor status`, CV-33/34) and stage 5
(`fathom report evidence`, CV-39) shipped; the pre-push hook runs `fathom
conserve --since` as a warning (CV-47); `review --since`/`review_diff`'s
`conservation` dimension is actually wired (CV-24); the SCIP import bug
that kept TypeScript/Python edges out of fathom's own symbol space is
fixed (O115). Open: CV-M4 fired this vision's own §6 revisit trigger — the
Go surface delta is a textual diff, not type-identity-aware, measured
0.615 precision / 0.941 recall against the 0.95/0.95 bar
(`docs/bench/readings/cv-m4-apidiff-calibration-2026-09-24.md`); CV-M5
found `conserve.Resurrected` over-fires on a genuine large removal
(`docs/bench/readings/cv-m5-blindness-replay-2026-09-24.md`); the decision
port's narrow role (§3, decision 8) and the cross-repo test-kit/Review Kit
rows stay `(op)`/unclaimed in the plan.

## 1. The thesis

fathom has two proven ideas. *Done means proven*: a plan checkbox is a claim the
index checks. *Tested means certified*: a test is evidence for a named behaviour,
and the certificate must stay live. This vision adds the third, for the activity
agentic coding will spend most of its time on: **refactored means conserved**.

A refactor is a change of structure under a conservation law. Some things must
stay: what the system offers (its exported API, routes, RPCs, CLI verbs, tool
names, wire field names, environment keys), the behaviour its certified tests
pin down, and the rules that govern the moved code. Some things must change:
whatever the refactor retires must be provably gone. fathom computes that law
from a base commit, checks the head against it, and reports what was lost, what
was declared, and what could not be checked.

The rewrite is cheap and plentiful: agents, codemods and recipe engines produce
it. Trust that nothing was lost is scarce. That trust is what fathom sells.

## 2. What refactoring actually fails on

Agents rarely fail at finding call sites; compilers and typed edges do that well.
They fail at proving the change happened and nothing was lost, and they fail at
not grading their own work:

- SWE Refactor Bench (arXiv 2608.23564): 28 of 520 runs pass its three stages;
  agents copy the old implementation behind a new name so tests keep passing.
- RefactorBench (arXiv 2503.07832): agents 22%, humans 87%.
- ChangeGuard (arXiv 2410.16092): a project's own regression tests catch 7.6% of
  semantics-changing edits, so "the tests still pass" proves little on its own.
- fathom's own guards pilot: an agent renamed and inverted the test that
  certifies invariant I28; `go test -run` then matched nothing and printed `ok`.
  Another agent checked off every row of a plan it had not been asked for.

## 3. What never changes

- **fathom proves; it never writes the change.** The transformation engine stays
  a permanent non-goal.
- **fathom asserts facts; Review Kit judges.** No safety grade, no score, no
  ordinal ladder. Planes and missing inputs, stated plainly.
- **Missing input means unproven.** A stale index, failed typed packages, an
  unbound test run or an unsupported language makes a plane `unproven`, never
  `conserved` (I44).
- **The agent that made the change cannot excuse its own loss.** Declarations
  added inside the change range are reported as self-declared (I45).
- **Costs scale with the change.** Base facts come from the changed files only.
- **The model orders attention; it never decides.** The decision port's role is
  narrow, measured and opt-in per site (ADR-042, ADR-044 decision 8).

## 4. The five stages

1. **Proof integrity.** Catch weakened, deleted, renamed, skipped or
   golden-edited tests that certify or enforce something; report declarations
   made inside the range as self-declared; bind certificates to the commit their
   run exercised.
2. **The conservation ledger.** The capability inventory at base and head with
   language-neutral keys, its delta against the declared delta, cross-repo
   consumers of anything removed, behaviour certificates that resolve to
   inventory keys, and rules orphaned by a move.
3. **Absence and anti-Blindness.** What the refactor retires is gone as an
   import, a module dependency, a pattern or a kept-but-unused symbol; no new
   callers of deprecated symbols; no deleted body resurrected under a new name.
4. **Refactors as programs.** A pinned baseline, ratchets that may only tighten
   while main keeps moving, and a gate-ready plan generated from the impact
   verbs. No campaign engine.
5. **A typed floor beyond Go.** Measured precision and recall for the
   TypeScript, JavaScript and Python tiers, and every report naming the tier it
   stands on.

## 5. Bigger questions this vision keeps asking

These stay open in `docs/intent/questions.md` until evidence answers them:

- What would a reviewer need to be shown to accept a 400-file agent refactor
  without reading the diff, and can a stranger re-derive every line of it?
- When is a test edit a legitimate refactor and when is it a weakening?
- Should conservation block at push, or only in CI, and for whom?
- How do certificates, ratchets and baselines behave on a long-lived branch
  while main keeps moving?
- Which refactors does this round not see: data and schema migrations,
  infrastructure moves, security properties such as an authorization chain,
  performance and concurrency?
- When agents write most code, what should teams choose languages and
  architectures for, and how can fathom measure evidence density per service
  without mistaking its own extractor maturity for a language's merit?

## 6. Kill criteria

Each stage carries one (P5): the integrity check must flag the real i28 and i36
cells and flag at most 10% of ordinary commits that touch certified tests; the Go
surface delta must agree with `apidiff` at 0.95 precision and recall; the
anti-Blindness check must catch seeded copies in replays of fathom's own
migrations; the refactoring pack must fire at warn within a month where armed;
and nothing in this vision may report "conserved" from a missing input.
