# Project intent — router

<!-- Entry point. Under 80 lines. Routes; does not explain. Delete these comments. -->

Durable intent for <project>: why it exists, what must never break, what is in and out
of scope, what may come later, what has been decided and when to reopen it, who may do
what. Practice: `agentic-kit/docs/project-intent/`. Skills: `intent-design` before
design work, then `intent-decide`, `intent-doctor`, `intent-audit`, `intent-prune`,
`intent-release`, `intent-incident`, `intent-init`.

## Docs map

| Question | Read |
|---|---|
| Why this exists, what it should feel like, what it is not | `vision.md` <!-- or ../../VISION.md --> |
| How to resolve a tradeoff; known tensions | `principles.md` |
| What must always / never be true; how each is enforced | `invariants.md` |
| Vocabulary, domain truths, platform facts, assumptions | `domain.md` |
| Current goals and exit tests, boundaries, compatibility surface, non-goals | `scope.md` |
| Plausible futures and how to keep them open for free | `futures.md` |
| Decisions, alternatives, consequences, revisit triggers | `decisions/README.md` <!-- or ../adr/README.md --> |
| What an agent may do unasked, must ask first, never does | `autonomy.md` |
| What broke, what would have caught it, what it produced | `incidents.md` |
| What we do not know yet | `questions.md` |
| What changed for users since the last release | `../../CHANGELOG.md` |
| How to cross a breaking change | `../upgrades/` |
| Work orders and their proof | `../plans/` <!-- specs, research, ledgers: add rows --> |

## Routing: what to read for which task

| Task | Read first |
|---|---|
| Typo, test fix, refactor inside one package, no behavior change | nothing |
| Implementation touching a sensitive surface (below) | `invariants.md`, decisions index, the surface's "Done needs" |
| Feature or behavior change | `scope.md`, `invariants.md`, `principles.md`, decisions index, matching `futures.md` entries |
| New subsystem, architecture change, major refactor | the above plus `vision.md`, `domain.md`, `futures.md`, the relevant decisions in full |
| Any push, tag, delete, release, or global-config change | `autonomy.md` |
| Strategy or roadmap work | everything |

## Sensitive surfaces and the evidence "done" needs

| Surface | Why it is sensitive | Done needs |
|---|---|---|
| `<path/**>` | <the invariant it can silently violate> | <test name, gate, benchmark, review step> |

## Before designing

Say out loud, before proposing a design: which invariant, decision, non-goal, or future
applies, and whether any `Revisit when` / `Promote when` / `Drop if` condition has
fired. Build for now. Foreclose nothing documented when keeping it open is free.

## When to update

- A decision passed the threshold in the decisions index → write the record.
- A goal met its `Done when` → remove it. A future met `Promote when` → move to
  `scope.md` and record the decision. A future met `Drop if` → delete it.
- An assumption, invariant, non-goal, or autonomy rule changed → edit in place, same change.
- Something broke → `intent-incident`, and it ends as an invariant, a test, or a decision.
- A trigger fired → say so; update after the human decides.
- Trivial implementation work never touches this directory.

List every condition: `grep -rn -E "(Revisit when|Promote when|Drop if|Done when)" docs/intent/ <decisions dir>`

Verified against: Claude Code <version>, Codex CLI <version> (<YYYY-MM-DD>). <!-- intent-doctor compares -->
