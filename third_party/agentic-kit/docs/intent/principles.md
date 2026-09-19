# Principles

How to reason when the kit's goals pull against each other. The eight core beliefs are
in `VISION.md` and are not repeated; these are the judgment rules that follow from them
and the tensions they create. Invariants (the tests) are in `invariants.md`.

## Principles

**P1. Demand or a bounded hypothesis.** A capability needs a consumer or a sponsored
incubation with executable outcomes and a dated review. New composition starts in an
existing kit, scenario, or adapter. Two active incubations at most; consumer regressions
retain priority. Zero consumers is evidence to review, not an automatic retirement.
Source: [0004](decisions/0004-bounded-incubation.md).

**P2. Downstream measures value.** Confirm which consumers and revisions actually ran.
Compilation proves compatibility, a scenario proves integration, and measured use
proves benefit. An absent checkout is untested. Source: VISION; `assurance.md`.

**P3. Honest contracts only.** State non-guarantees as plainly as guarantees: a wrapper
is defense-in-depth, not a boundary; bounded-exec is confinement, not a sandbox.
Overclaiming is a defect class equal to a failing test. Source: VISION belief 6;
`threat-model.md` §3.

**P4. Absorb into the smallest useful composition.** A field practice becomes a
failure case and a tested remedy. Prefer an existing seam, default, scenario, or adapter
before a new primitive. A lesson includes applicability and provenance, so other
workloads can validate it. Source: VISION; [0006](decisions/0006-evidence-and-lessons.md).

**P5. Nest until a second consumer names the need.** A package with one consumer lives
under that consumer; it graduates to a shared foundation only when a second one names
the need (D8). Two consumers make it kit business (D13). Source: `compat.md` §D8;
`designs/catalog-kit.md` §1.

**P6. Mine, don't port.** Reference sources under `_reference/` are read and ported
selectively with a named subscriber each; never imported, never copied wholesale.
Source: uplift plan D8; CLAUDE.md §Where things live.

**P7. Determinism is the default; nondeterminism is injected.** Clocks, randomness,
network, subprocesses, and model calls sit behind seams; tests replay cassettes and
never touch the network. Source: VISION belief 2; CLAUDE.md hard rules.

**P8. Fail closed, spend bounded.** Every paid call is budgeted before it is made;
every buffer, cache, and loop states its bound and its behavior at the bound. Source:
VISION belief 4.

**P9. Match the sibling's shape.** Read two sibling packages before adding one; the
package doc states its concurrency contract; `schema.cue` where shape is data.
Source: CLAUDE.md §Working style; junk gate.

**P10. Owner ratifies what changes the promise.** Going public, 1.0, demoting a
classification, retiring a package with a consumer: Geoff's call, recorded. Source:
uplift plan D4 ("genuinely yours"); `compat.md` §Promise.

**P11. Prove the path consumers run.** Every advertised safeguard names required
wiring, execution surface, failure behavior and tests. Test removal and bypass, not
only the helper's happy path. Advisory guidance stays labelled advisory. Source: 0005.

**P12. Separate integrity, support and truth.** Valid IDs and hashes establish links;
evaluations and review assess whether evidence supports a claim; domain authority
determines what a source can establish. Never collapse these into a confidence badge.
Source: 0006; `assurance.md`.

**P13. Share the evidence lifecycle, not a universal grade.** Domain criteria retain
their applicability, evaluator and limits. Required failures and unknowns cannot be
compensated by soft scores; N/A needs justification. Screens classify, external controls
authorize. Source: [0012](decisions/0012-composable-engineering-assessments.md).

## Tensions

**T1. Stdlib-only vs real isolation.** Default: stdlib. The kit will never claim a
sandbox; consumers needing isolation add it themselves. Source: `threat-model.md` §3.

**T2. Consumer needs vs anticipation.** Default: repair consumer friction first, with
a bounded portfolio for the next important workflow. Advance an incubation on new
evidence; park it when the hypothesis does not survive. Source: 0004.

**T3. Stable promise vs pre-1.0 velocity.** Default: `stable` may still break pre-1.0,
but never silently: a migration note in `CHANGELOG.md` and a same-commit `TRACKING.md`
update. Source: `compat.md` §Promise.

**T4. Org-agnostic vs naming real consumers.** Default: lowercase consumer repo names
are architecture and stay; the historical coded names are denylisted. A waiver file
containing half the tree is the failure to avoid. Source: `cmd/constitution-lint/org-denylist.txt`.

**T5. Agent-developed vs human-owned.** Default: agents build, the gate proves, Geoff
ratifies decisions that change the promise. The gate substitutes for continuous review;
it does not substitute for ratification. Source: CLAUDE.md line 3; uplift plan D4.

**T6. Verbose evidence vs readable change records.** Default: the `CHANGELOG.md` line is
two sentences and links; rationale lives in a decision or the commit. The 500-word
entries of 2026-07 are the counterexample, left as history. Source: `project-intent/changes.md`.

**T7. Automation vs human attention.** Automate collection, validation, previews and
reversible preparation. Ask at the actual authority boundary with the exact proposed
effect and prior authorization intact. Measure false blocks and review time alongside
failure prevention; more prompts are not evidence of more control. Source: 0005/0006.
