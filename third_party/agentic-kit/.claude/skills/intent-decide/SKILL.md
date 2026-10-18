---
name: intent-decide
description: >
  Record, reject, defer, or supersede a decision record in a repo with docs/intent/
  (decisions may live in docs/adr/). Applies the recording threshold, writes the record
  from the template with a required Revisit when, updates the decisions index and any
  living doc the decision changes. Use when a design session settles something
  cross-cutting, expensive to reverse, or likely to be proposed again.
---

# intent-decide

Part of the `intent-*` family. Rationale: `agentic-kit/docs/project-intent/README.md` §5.
Template: `../intent-init/templates/decisions/0001-template.md`. The router
(`docs/intent/README.md`) names the decisions directory and its numbering.

## Threshold

Record when any holds: reversal is expensive; it constrains code outside its own
package; it creates or removes an invariant, non-goal, or boundary; it accepts a notable
tradeoff or dependency; it defers something significant and names what would un-defer
it; it rejects an alternative someone will propose again; without the record a future
reader would rebuild this differently. Otherwise do not record: a comment, a test name,
or a commit body carries it.

## Procedure

1. Id = highest existing + 1, keeping the repo's prefix and width (`ADR-011`, `0011`).
   Check the router or `questions.md` for reserved ids before taking one.
2. File `<decisions dir>/<id>-<slug>.md` from the template. Delete sections that do not
   apply. Keep `Context`, `Decision`, `Revisit when`. Under 80 lines.
3. `Status:` proposed | accepted | rejected | deferred | superseded by <id> | deprecated.
   `Type:` product | ux | domain | architecture | data | security | ops | dependency |
   platform | process | strategy.
4. `## Revisit when` is required. Observable conditions: a metric crossed, a spec
   shipped, a second consumer, a named event. "None identified" is allowed; omission
   is not.
5. Rejected alternatives: one-line reason each, plus `Wins if: <condition>`, or `Wins if:
   never` for a hard rejection. A standalone proposal or an owner's request that was
   declined and passes the threshold gets its own record with `Status: rejected` and the
   request quoted. Never a separate rejections log, never an ideas directory.
   `Request:` carries the deciding words verbatim and dated, `[org]` for organization
   names; a paraphrase says so.
6. Superseding: the new record says `Supersedes: <id>`; edit only the old record's
   `Status:` line to `superseded by <new id>`.
7. Add one row to the decisions index (id, status, type, title, short trigger).
8. Same change: update `scope.md` / `invariants.md` / `futures.md` if the decision moved
   them. If it changes observable behavior, the `CHANGELOG.md` line links to the record,
   never the reverse (`changes.md`). If it changes the compatibility surface, it is
   breaking.
9. Report: id, status, what it supersedes, which living docs changed.
