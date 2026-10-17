# Intent practice: what was measured, what was kept, what waits

Owner-architect assessment, 2026-10-17, summarized into the repository the same day. Read at fathom `a7d18c2f`
(v0.15.0). The research pass (`wf_888f8f2d-57b`) and the adversarial review (`wf_a6ee03dd-707`; lenses evidence,
agents, doctrine, and a judge) sit in gitignored workspace files and are not the record. The decision is ADR-074;
the work order is `docs/plans/intent-v2.md` (IV-); this file is the reading that led to them and what v0.15.0
delivered against it.

## Verdict

Keep the practice: it is cheap and it is read. What was weak was delivery and bookkeeping, not the idea. A loading
session pays a median of about 2.1k tokens of intent, every record was read at least once, and 140 transcript lines
in 37 sessions redirect a design on an invariant, decision, non-goal or fired trigger. That the records caused the
redirects is inferred, because there is no contrast group.

Kept, and built in v0.15.0 (ADR-074): retire `INTENT.md` (nothing read it); a committed view needs a named reader and
a `--check`, or it is not committed; edit-time intent generated from the router instead of hand-pasted; a futures
vet; a lossless plan round-trip; every new rejection names what would make it win.

Rejected for now, each with its wins-if (ADR-074, R1 to R3 and the rest): CUE-as-source records with rendered
markdown views (wins if a record kind is over half fields, or markdown field vet failures exceed five a month); a
per-idea ledger with a multi-dimensional rejection grade (wins if three or more re-proposals a month of ideas with no
home, or the wins-if fill rate passes 50% and reviews ask for a finer grade); a full value, effort, risk and
dependency rubric (wins if the owner sorts or filters by effort or risk in two planning sessions); an `intent.md` file
per change or repository (wins if a harness loads a named intent file); `decisions/README.md` generated from
headings (wins if the parity check fails at two wave merges in one month); retiring the plan CUE shadow (wins if a
lossless round trip needs a schema key no reader uses); an "Applies to" column replacing `guard.cue` (wins if K1
reads KEEP and invariants no surface names still need edit-time delivery); annotations, review stamps and new intent
views (wins if three hand-written annotations across two repositories appear, or R2 or R3 wins).

Fired and recorded: kit decision 0001 (condition annotations; its thirty days ran out 2026-09-26) is marked rejected
and fathom ADR-026's K0 recorded the same way, with CX-21's evaluator kept and no inputs; the `ADR.md` split condition
(an id collision at a wave merge, ADR-058, 2026-09-29) fired and IV-9 read it. Measured, not fired: F9.

The 30-day readings K1 to K6 are due 2026-11-17 and none has been taken: they read **unmeasured** here, with the
baselines below. Only K1 and K6 are gated in fathom (IV-M1); K2 to K5 belong to the kit.

## What was read, graded

| Claim | Grade | Source |
|---|---|---|
| A loading session pays a median of about 2.1k tokens of intent (p90 about 10.7k); every fathom record read at least once | measured (transcripts, 2026-10-17) | `wf_888f8f2d-57b` |
| 140 transcript lines in 37 sessions redirect a design on a record | measured; that records caused them is inferred | `wf_888f8f2d-57b` |
| `INTENT.md`: 0 Read-tool reads, 0 instruction loads, 15 commits of upkeep, three renderer bugs (O660, O661, O670) | measured | `wf_888f8f2d-57b` |
| `decisions/README.md` read in 156 sessions, the most churned file in `docs/intent/` (72 commits) | measured | `wf_888f8f2d-57b` |
| Hook injections cite `ADR.md` 79% of the time and `docs/intent` 1.8%; 52% of wave briefs name governing ids; Claude subagents read the router in 9% of sessions | measured | `wf_888f8f2d-57b` |
| 15 hand-pasted guard entries reach 15 of 66 invariants at edits (the stanza claimed all matched invariants) | measured (O1134) | `.fathom/guard.cue`, `docs/intent/invariants.md` |
| 0 of 154 conditions carry an annotation, so the audit evaluated none | measured | ADR-074 context, T1 |
| 182 rejected-alternative bullets, 4 with "wins if", 0 records with Status rejected | measured | ADR-074 context |
| Futures: 8 Horizons off the list, 2 futures missing fields; Horizon did not predict promotion (3 of 4 promoted were "later") | measured | ADR-074 context |
| `gen-plan-cue` lost data on 49 of 75 active plans (50 of 76 at the v0.15.0 base) | measured | ADR-074 context; O1137 |
| 308 plan trigger values share only 2 three-word stems across 4 repositories | measured (F9 not fired) | ADR-074 context, T2 |
| `intent audit` U4: 181 of 229 unproven claims were a stale daemon index | measured | O1136 |
| A sibling repository is served 6 rules compiled from deleted `CLAUDE.md` files | measured | ADR-074 context, IV-7 |
| The records caused the redirects; Horizon is a poor predictor beyond 4 promotions | inferred / estimate (small n) | none: no contrast group |
| Fewer repeated proposals follow from wins-if lines | hypothesis | K3 reads it |

This assessment does not claim that the practice improved any outcome, only that it is cheap and read, and that the
delivery gaps listed were real. It does not claim any K1 to K6 reading.

## What v0.15.0 delivered

| Piece | Reading | Grade | Record |
|---|---|---|---|
| `INTENT.md` retired; `intent render --playbook` deprecated (prints a notice, removed one minor release later); `intent-render-check` gate step gone | Breaking line in the changelog | measured (shipped) | ADR-074 decision 1, IV-1, IV-10 |
| `fathom intent index --check` | 74 decisions: 74 `ADR.md` headings against 74 index rows, a `task validate` step | measured, 2026-10-17 | IV-2 |
| Guard intent entries generated from the router's surface table | 15 entries name 10 distinct invariants at edits; the other 56 are listed once at session start as repo-wide (10 + 56 = 66) | measured (O1134) | IV-3a to IV-3c, I35 |
| Futures vet (U16) against `schema/intent` | 10 of 84 futures failed before the repair, 0 of 84 now | measured | IV-5, IV-5b |
| Plan round-trip through a completed `schema/plan` | 50 of 76 active plans lossy at base, 0 of 76 now | measured (O1137) | IV-6 |
| U4 reads the repository's own tests | 181 findings to 10 on a scratch copy of HEAD; the 10 are names defined nowhere in the tree | measured (O1136) | IV-8 |
| `obligations` withholds a rule whose source is gone, counts it in `layer_coverage.stale_withheld` | tests named in O1135 | measured | IV-7 |
| ADR.md split investigation | Verdict: no recompile needed for a pure move; all 76 sections meet the carry digest rules, no model spend. A split that edits a body, adds a title or re-levels a heading makes that section pay | measured offline | IV-9, `docs/bench/readings/intent-v2/adr-split.md` |

IV-9's verdict opens the split to its own change, under ADR-074's Revisit when; this assessment does not make that
change.

## The 30-day keep or kill, due 2026-11-17

| Id | Piece | Baseline | Keep if | Else | Reading |
|---|---|---|---|---|---|
| K1 | Guard generation | `docs/intent` in 1.8% of hook injections | 10% or more, injected bytes inside I35 | revert to the hand list | unmeasured (IV-M1) |
| K2 | Brief rule | 52% of briefs cite governing ids | 80% or more | drop the rule | unmeasured (kit) |
| K3 | Wins-if rule | 4 of 182 (2%) | 50% or more of new rejected alternatives | drop the rule | unmeasured (kit) |
| K4 | Value and Basis | 0 | 50% of new or touched futures filled | delete the fields | unmeasured (kit) |
| K5 | Bounded review | 22 firings found from commits in 7 weeks | changes at least one disposition | delete the script step | unmeasured (kit) |
| K6 | Docs search before design | misses E10 and c1 | recalls all 6 known misses | revert the step | unmeasured (IV-M1) |

## Decision models

None applies (ADR-074, ADR-073). The candidate questions, "is this request already decided?" and "has this judged
condition fired?", are predicates, and stage 0 measured judged predicates at or below baseline. With provider `none`
the whole design runs on docs search and the git-derived bounded review. K6 and the kit's K5 would reopen either.

## Decisions it drove, and where they are recorded

- The record: ADR-074, with dated amendment lines on ADR-018 (decision 3's round-trip claim) and ADR-026 (K0 fired);
  the CUE non-goal narrowed in `docs/intent/scope/non-goals.md`; F86 added (component corpora); F9 recorded measured,
  not fired.
- Rows: `docs/plans/intent-v2.md` IV-1 to IV-10 and IV-M1. Observations: O1134 to O1137.
- Not done here, by design: the kit side (doctrine, the bounded review script, the skill steps, kit decision 0001
  marked rejected) lands by a kit change.
