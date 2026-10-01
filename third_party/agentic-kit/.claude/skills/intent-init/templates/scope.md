# Scope

<!-- Where the edges are right now. Changes often; keep it short and present tense.
     Under 100 lines. Done goals are deleted, not archived; git remembers. -->

## Current goals

<!-- What we are building now, each with an exit test. Sequence, not dates. -->

**G1. <Goal.>** Done when: <observable exit test>. <One line of why, if not obvious.>

## Current boundaries

<!-- What the system is today: the surfaces, providers, platforms, users it serves.
     Evidence from code, not aspiration. -->

- <Boundary.> Evidence: <path or test>.

## Compatibility surface

<!-- What this project promises not to change without ceremony. A change to any item is
     breaking: commit `!`, `### Breaking` in CHANGELOG.md, migration note if needed. -->

- <API, schema, on-disk format, CLI verb/flag, exit code, config key...> Evidence: <path>.

Pre-1.0: breaking changes bump minor. The line still gets written.

## Non-goals

<!-- Things we will not do, with the reason. Each carries the condition under which
     the answer would change. Long-lived "what this is not" belongs in vision.md. -->

- **<Non-goal.>** Why: <reason>. Revisit when: <observable condition, or "never">.
