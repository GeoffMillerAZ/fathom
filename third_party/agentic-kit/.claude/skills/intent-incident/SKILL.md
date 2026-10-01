---
name: intent-incident
description: >
  Record an incident (what happened, evidence, what would have caught it) in
  docs/intent/incidents.md and drive it to a durable outcome: an invariant, a test, or a
  decision record, so the same failure cannot recur silently. Use right after a defect
  escapes, a gate is bypassed, a wrong claim of "done", an agent violates documented
  intent, or a platform behavior bites for the first time.
---

# intent-incident

Part of the `intent-*` family. Rationale: `agentic-kit/docs/project-intent/operating.md`
§2. An incident without an outcome is a story; an outcome without an incident is a rule
nobody understands. Both halves, always.

## Record (≤ 10 lines, `docs/intent/incidents.md`)

```
### INC-<n> <YYYY-MM-DD> <title>
What: <one or two lines, observable facts only>
Evidence: <commit, transcript, log line, test output, review finding>
Would have caught it: <the check, test, invariant, or review step that did not exist or did not run>
Class: gate-bypassed | wrong-done-claim | intent-violated | platform-fact | contract-broken | other
Outcome: <pending | I<n> in invariants.md | test <name> | ADR-<n> | platform fact in domain.md>
```

## Procedure

1. Write the record from the facts. No blame, no narrative. If the incident is in a
   predecessor system, point at its catalog instead (a `PAIN.md`-style doc) and record
   only the consequence for this project.
2. Decide the outcome with the smallest durable form that prevents recurrence:
   - a **test** if the failure is mechanical and local;
   - an **invariant** row (with its Enforced-by column honest) if it is cross-cutting;
   - a **decision record** (`intent-decide`) if it changes a boundary or a policy;
   - a **platform fact** in `domain.md` if the cause is the environment;
   - an **autonomy** change if an agent did something it should have asked about;
   - an **evidence requirement** on the router's sensitive-surface table if a gate was
     missing for that surface.
3. Land the outcome in the same change when it is small; otherwise leave
   `Outcome: pending` and a `Refs:` to the plan row that will land it. `intent-doctor`
   flags pending outcomes older than 30 days.
4. Close: set `Outcome:` to the concrete artifact. `intent-prune` later compresses the
   closed record to one line.
5. Report: the record id, the outcome, and whether anything else in `docs/intent/`
   changed because of it.
