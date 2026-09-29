# Intent health — 2026-09-13

Read-only `intent-doctor` after the governed-workflow refresh: **21 PASS, 2 SKIP**.
Scope is the owned root intent, excluding templates, vendored snapshots and temporary
worktrees. This checks wiring, not semantic truth or runtime enforcement; those are
reported in the [delivery assessment](2026-09-13-governed-workflows.md).

| # | Check | Result | Evidence |
|---|---|---|---|
| 1 | Router exists | PASS | `docs/intent/README.md` |
| 2 | Router budget | PASS | 79 lines; limit 80 |
| 3 | Living-doc budgets | PASS | Largest is `futures.md`, 111 lines; all within 150/180 limits |
| 4 | Required docs | PASS | All eight required living documents present |
| 5 | Decision membership | PASS | Working tree has 10 unique local records/index rows; committed delivery has 9, excluding the owner's separate uncommitted 0007 in both counts; legacy rows are external pointers |
| 6 | Record shape | PASS | Every local record has `Status` and `Revisit when` |
| 7 | Trigger registry | PASS | Explicit revisit/promote/drop/done conditions present |
| 8 | Goals/futures | PASS | G1 has `Done when`; future entries have confidence/horizon and promote/drop conditions, with F16 defaults and explicit folded pointers |
| 9 | Compatibility surface | PASS | `scope.md` names exported APIs, errors, stable CUE, nested modules and inherited gate semantics |
| 10 | Sensitive surfaces | PASS | Router's `Done needs` cells identify tests/tasks/evidence |
| 11 | Changelog shape | PASS | `[Unreleased]` begins with `Release impact:`; Breaking first |
| 12 | Unreleased budget | PASS | 51 body lines including the owner's separate entry; below 60 |
| 13 | Autonomy | PASS | May/ask/never and protected-main branch policy present |
| 14 | Incidents | PASS | Open INC-7 has an outcome and is 11 days old; generic gate addition remains unauthorized, not silently implemented |
| 15 | Host stanzas | PASS | CLAUDE.md and AGENTS.md Project intent blocks identical after blank-line removal |
| 16 | Claude skills | PASS | All 8 intent skills resolve to project `.agents/skills` |
| 17 | Codex skills | PASS | Same 8 resolved bodies; no separate shadow copies; resolution does not prove invocation |
| 18 | Installed versions | PASS | Claude Code 2.1.270, Codex CLI 0.154.0 match router; live coverage is separately scoped |
| 19 | Relative links | PASS | Root living-doc and decision-index targets resolve; directory links and local anchors checked |
| 20 | Dead router paths | PASS | Named files/directories exist; placeholder pattern excluded |
| 21 | Repository charter | PASS | Root is the only owned intent router; component map inherits root, with stated evolution triggers |
| 22 | Nested stanzas | SKIP | No component-owned intent router |
| 23 | Component hygiene | SKIP | No component-owned intent router |

The owner's in-flight 0007 decision, four modified skill bodies, CLAUDE/Taskfile and
practice-distribution edits were preserved, not incorporated into this delivery.
Both hosts currently resolve those same working-tree skill bodies.

Most useful next action: run the first attended résumé-shaped prose pilot against a
real authorized baseline. A green doctor is not evidence that that pilot has happened.
