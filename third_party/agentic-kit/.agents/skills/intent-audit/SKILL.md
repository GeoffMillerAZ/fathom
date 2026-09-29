---
name: intent-audit
description: >
  Truth check of a repo's Project Intent: which Revisit when / Promote when / Drop if /
  Done when conditions have fired, which assumptions are now false, whether each
  invariant's enforcement claim still holds, which open questions have been answered,
  which incidents lack an outcome, and whether scope still matches the code. Reports
  findings with evidence and proposed edits. Use at a milestone, a release, or when the
  intent smells stale.
---

# intent-audit

Part of the `intent-*` family. `intent-doctor` checks shape; this checks truth. Read
the router first. Report before editing; edit only what `docs/intent/autonomy.md`
allows without asking, and say what you changed.

Resolve affected component paths first, including a task launched at root. Audit the
root's applicable constraints and the component's own claims; cross-component work
includes both contract owners. Inherited goals have one canonical owner. Preserve prior
user authorization when proposing and making the scoped edits.

## Walk

1. **Triggers.** `grep -rn -E "(Revisit when|Promote when|Drop if|Done when)" docs/intent/ <decisions dir>`.
   For each condition: `fired | not fired | cannot tell`, with the evidence (a file, a
   test, a metric, a shipped feature, a date passed). For each fired one: what the
   record says to do, and the recommendation (reopen via `intent-decide`, promote the
   future into `scope.md`, drop it, delete the goal).
2. **Assumptions** (`domain.md`). Any now false? Name the records its `If false` line
   points at.
3. **Invariants** (`invariants.md`). For each row marked `test` or `code`, confirm the
   named test or code path still exists (the repo's code index, with grep fallback).
   Existence alone is not enforcement: trace the entrypoint, missing-tool and bypass
   behavior, authority, and the negative test. For rows marked `stated`, check
   whether a test now exists and the column should upgrade. Spot-check three `stated`
   rows against the code for silent violation.
4. **Scope** (`scope.md`). Do the boundaries match the code (verbs, providers, surfaces)?
   Is any goal's `Done when` met? Is any non-goal being built?
5. **Futures** (`futures.md`). Has any `Do not build` item been built? Has any `Preserve`
   choice been undone by a recent change (`git log --since`)?
6. **Questions** (`questions.md`). Any answered by a commit, a record, or a spec change?
7. **Incidents** (`incidents.md`). Open ones without an `Outcome:`; outcomes whose
   invariant, test, or record does not exist.
8. **Changelog** (`CHANGELOG.md`). Do `[Unreleased]` lines match `git log` since the
   last tag (missing observable changes, or lines for changes that were reverted)? Is
   `Release impact:` the max category present?
9. **Decisions elsewhere.** Any ruling in a spec, audit, or plan since the last audit
   that passes the threshold and is not indexed?

## Output

Findings first, ranked by consequence, each with evidence and the proposed edit:

```
F1  FIRED   ADR-009 "notify pipeline clean 60 days" — day-ledger shows 71 clean days → reopen background enforce (intent-decide)
F2  FALSE   assumption "single machine" — second host in migration/ledger → F9 Promote when review
F3  STALE   I10 marked `stated`; TestImportDiscipline landed 2026-09-02 → upgrade to `test`
```

Then the edits made (if allowed) and the edits awaiting the human. Finish with the one
finding that most changes what should happen next.
