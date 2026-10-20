---
name: intent-prune
description: >
  Shrink a repo's Project Intent and changelog back to budget: delete goals whose Done
  when is met, futures that were dropped or promoted, answered questions; compress closed
  incidents to one line; collapse [Unreleased] to the delta from the last release; split
  or trim living docs over budget; fix the decisions index. Git keeps history; nothing is
  archived. Use when intent-doctor reports a budget failure or the docs feel heavy.
---

# intent-prune

Part of the `intent-*` family. Prune removes what is no longer true or no longer
useful. It never changes a decision record's substance and never touches a released
changelog block. Read the router first; check `autonomy.md` for whether pruning may
proceed without asking (it usually may: everything removed is in git).

## Passes

1. **Goals** (`scope.md`). `Done when` met → delete the goal. If the goal produced a
   durable rule, that rule is already an invariant or a decision; if not, write one
   (`intent-decide`) before deleting.
2. **Futures** (`futures.md`). `Promote when` met → move to `scope.md` as a goal and
   record why now. `Drop if` met → delete. Speculative entries older than a year with
   no evidence change → delete. A deleted future's reason, with its `Wins if` or
   `never`, moves into the record that dropped it (a decision's Alternatives rejected or
   the plan's disposition row); there is no ideas directory.
3. **Questions** (`questions.md`). Answered → delete; the answer lives in the doc or
   record it changed.
4. **Incidents** (`incidents.md`). Closed (has `Outcome:` pointing at an existing
   invariant, test, or record) → compress to one line: `id date title → outcome`.
5. **Changelog** (`CHANGELOG.md`, `[Unreleased]` only). Collapse to the delta: a fix to an
   unreleased addition folds into the addition; an extension rewrites the line; a
   removal deletes the line; a line over two sentences loses its explanation (link the
   record instead). Recompute `Release impact:`. In a fragments repo (pointer line in
   `[Unreleased]`) apply the same collapse to the fragment files, deleting a fragment
   left empty; `Release impact:` is computed by the roll.
6. **Budgets.** Any living doc over budget: first remove restatement (point instead),
   then move detail to the doc that owns it (a domain fact into `domain.md`, a rule
   into `invariants.md`), then split only if the doc genuinely holds two subjects.
7. **Decisions index.** Rows equal records; statuses match the `**Status:**` lines;
   superseded records show their successor.
8. **Router.** Every path still exists; the docs map has no dead rows.

## Output

What was removed or moved, one line each, with the commit that still holds it
(`git log -1 --format=%h -- <path>`). Then `intent-doctor` to prove the budget.
