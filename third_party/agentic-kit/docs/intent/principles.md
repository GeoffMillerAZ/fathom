# Principles

How to reason when the kit's goals pull against each other. The eight core beliefs are
in `VISION.md` and are not repeated; these are the judgment rules that follow from them
and the tensions they create. Invariants (the tests) are in `invariants.md`.

## Principles

**P1. No consumer, no capability.** A capability lands when a real workload pulls for
it, never because it sounds visionary. Every addition names its subscriber; a kit with
zero consumers after its wave is demoted or retired. Source: VISION §What the kit is NOT;
uplift plan D2, D8.

**P2. Downstream is the oracle.** The three named consumers (fathom, throughline,
growing-gardens-bi) and `task downstream-check` decide whether a change is done, not the
kit's own tests alone. Source: VISION §How it stays current; uplift plan D3.

**P3. Honest contracts only.** State non-guarantees as plainly as guarantees: a wrapper
is defense-in-depth, not a boundary; bounded-exec is confinement, not a sandbox.
Overclaiming is a defect class equal to a failing test. Source: VISION belief 6;
`threat-model.md` §3.

**P4. Absorb, don't annotate.** A field best practice becomes a primitive with a test,
not a footnote in a doc. The practices ledger is the import list, not commentary.
Source: VISION §How it stays current; `practices.md`.

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

## Tensions

**T1. Stdlib-only vs real isolation.** Default: stdlib. The kit will never claim a
sandbox; consumers needing isolation add it themselves. Source: `threat-model.md` §3.

**T2. Consumer-driven vs completeness.** Default: consumer-driven. A roadmap item
without a subscriber stays a future with a subscribe-when trigger, however obvious it
looks. Source: `roadmap.md` §How to read this.

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
