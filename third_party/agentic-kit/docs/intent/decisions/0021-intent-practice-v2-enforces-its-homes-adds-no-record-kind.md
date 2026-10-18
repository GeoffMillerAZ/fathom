# 0021: Intent practice v2 enforces the homes it has; it adds no record kind

**Status:** accepted
**Type:** process
**Date:** 2026-10-17
**Request:** the owner, 2026-10-17 (a paraphrase; the words were not kept): consolidate the
generated intent overview, reconcile the practice with a vendor's per-change `intent.md`, stay
Codex- and monorepo-compatible, grade and periodically review rejected ideas, rate backlog items
on several dimensions, move machine-read records to CUE with rendered views; keep, improve or
revert each practice on evidence.

## Context

fathom measured its corpus and its fleet on 2026-10-17. The practice is cheap and read: a median
of about 2.1k tokens per loading session, every fathom record read, 140 transcript lines in 37
sessions where intent redirected a design (causality inferred). fathom's generated overview had
0 reads against 15 upkeep commits; its hand-kept decisions index was read in 156 sessions. None
of 154 conditions is annotated, so fathom's audit evaluates none; people found 22 firings from
commits in seven weeks. 4 of 182 rejected alternatives carry `wins if`, no record is Status
rejected, and the measured miss (an owner request a fathom record had decided) is one a docs
search finds. Horizon did not predict promotion (3 of 4 promoted futures were `later`); seam value
ratings drove an owner ask. 52% of orchestrator briefs cite governing ids; Claude subagents read
the router in 9% of sessions. In platform-engineering `F1` has 7 homes and `G1` 5, review-kit
links out 60 times, 1,239 lines cite a root ADR by bare id, and Codex at the root loads no nested
`AGENTS.md` (root to cwd, 32 KiB cap). A per-change `intent.md` (secondary sources) is one idea in
the requester's words, the first artifact of a per-change chain; no harness loads one.

## Decision

The kit half; fathom's half (its overview's retirement, guard generation, CUE vet) is fathom's.
1. **Views.** A committed view needs a named reader and a `--check` gate (README §11a).
2. **Per-change intent** maps to existing homes: a plan opens with its request verbatim and
   dated, a record quotes the deciding words on a `Request:` line, `[org]` redacted (§5, §11c).
   No repository intent file is named `INTENT.md` or `intent.md`.
3. **Rejections.** Every new rejected alternative carries `Wins if: <condition>` or `never`; a
   declined owner request past the threshold is a record with Status rejected (§5).
4. **Bounded review.** `intent-audit/scripts/intent-stale.sh` (git only, ships with the skill)
   lists the 10 condition lines whose last change is oldest; `intent-audit` judges them weekly and at wave planning, records a
   fired one in its record, and commits each judged line, which resets its age.
5. **Value and Basis**, optional, on futures and candidate lists; §13 is amended for these two.
6. **Briefs** name the governing record ids (`operating.md` §4).
7. **`intent-design` step 2** runs a docs search (fathom's when present, else grep), ~1.5 KB.
8. **Monorepo:** `<scope> <id>` citations, an extraction checklist, a root routing table naming
   every component, a 32 KiB root-to-deepest `AGENTS.md` chain; `intent-doctor` flags a bare
   id that crosses scope and a chain over budget (`monorepo.md` §4, §5a, §8).
9. **0001 is rejected**: its own Revisit when fired. `intent-prune` writes a dropped future's
   reason into the record that dropped it.

## Alternatives rejected

- **A per-idea ledger with a graded rejection.** Unproven: the miss was a decided record a
  search finds; the one-field form goes unused (4 of 182). Wins if: three re-proposals a month
  of ideas with no home, or wins-if fill passes 50% and reviews then ask for a finer grade.
- **The full value, effort, risk and dependency rubric.** Horizon did not predict promotion.
  Wins if: the owner sorts or filters candidates by effort or risk in two planning sessions.
- **CUE-authored records with rendered views.** An ungated copy rots (fathom's plan shadow loses
  data on 49 of 75 plans), prose is poor in CUE, short records grow 3x, readers are mostly LLMs.
  Wins if: a record kind is over half fields, or markdown field-vet failures exceed five a month.
- **An idea kind, a strength enum, a backfill.** A second place to search. Wins if: the ledger wins.
- **A review skill, review stamps or ledger state.** Git already dates lines. Wins if: the
  bounded review misses a firing a stamp would have caught, twice.
- **Never committing views; a generated decisions index.** The hand index is read. Wins if: it
  drifts from its records at a gate check.
- **Renaming Confidence to likelihood.** Churn. Wins if: two reviews confuse it with Basis.
- **`<scope>:<id>` and parent pointers so a split rewrites one line.** The counts above refute
  the one line. Wins if: an extraction completes by editing one line.
- **Kit-shipped CUE schemas for intent records.** One evaluator exists. Wins if: a second does.

**Decision models.** None applies: "is this already decided?" and "has this condition fired?"
measured at or below baseline in fathom's Stage 0; docs search and the bounded review are the
deterministic paths, and with no port there is nothing to opt out of.

## Revisit when

The 30-day keep-or-kill reading, due 2026-11-17 on fathom's instruments; a piece that misses goes:
- K2 briefs: under 80% cite governing ids (baseline 52%) → drop the rule.
- K3 wins if: under 50% of new rejected alternatives carry one (baseline 4 of 182) → drop it.
- K4 Value and Basis: under 50% of new or touched futures fill them (baseline 0) → delete them.
- K5 bounded review: it changes no disposition (baseline: 22 firings found by hand) → delete it.
- K6 docs search: it recalls fewer than all six measured misses → revert the step.
