---
name: intent-design
description: >
  Before material product, design, domain, or architecture work in a repo that has
  docs/intent/: read the router, load only what it routes to, state the invariants,
  decisions, non-goals, futures, and any fired revisit trigger that apply BEFORE
  designing, then apply the free-option test. Use for feature design, new subsystems,
  refactors that cross a boundary, or whenever a design might conflict with documented
  intent. Not for trivial changes.
---

# intent-design

Part of the `intent-*` family (`/intent-…` in Claude Code, `$intent-…` in Codex).
Rationale: `agentic-kit/docs/project-intent/README.md`. The router is
`docs/intent/README.md`; it names where vision and decisions live.

Governing rule: **build for now; foreclose nothing documented when keeping it open is
free.** Never implement a future. Never pay complexity for one.

## Detect

- Walk up from the working directory to the nearest `docs/intent/README.md` → read it
  first, always. If it is not the repository root's, also read the root router's
  `invariants.md` and `repo.md` (the charter): nearest for scope and decisions, root for
  invariants and the container, never the siblings.
- No router but `docs/adr/`, `VISION.md`, `ARCHITECTURE.md` exist → prior art; use it,
  and offer `intent-init` only for substantial long-lived work.
- Nothing → do not invent intent. Ordinary engineering judgment.

## Protocol

1. **Classify** the task with the router's routing table. Trivial (typo, test fix,
   refactor inside one package, no behavior change) → stop, load nothing, say
   "no intent context needed".
2. **Load only what is routed.** Scan the decisions index by title and `Revisit when`.
   Open a record only if its title or trigger touches the task. Grep `futures.md` for
   the task's keywords.
3. **Extract what applies**: invariants (and whether each is enforced or only stated),
   principles and tensions, accepted or deferred decisions, non-goals, assumptions,
   matching futures, the compatibility surface, the evidence the sensitive surface
   requires.
4. **State conflicts and fired triggers before designing.** Out loud, first:
   - violates an invariant → the design is wrong, not the invariant; redesign.
   - contradicts an accepted decision → do not route around it; propose reopening or
     superseding via `intent-decide`, with the new evidence.
   - crosses a non-goal → surface it; the human decides.
   - matches `Revisit when` / `Promote when` / `Drop if` → "trigger fired: <record>
     <condition>", then evaluate whether now is the time.
   - touches the compatibility surface → it is breaking; say so (`changes.md`).
5. **Two horizons.** Simplest design for the present requirement. Then: does it
   foreclose a future marked likely or plausible? Preserve only if all three hold:
   (a) the future is documented, (b) the preserving choice adds no complexity today,
   (c) the alternative makes the future destructive to add later. Any part fails →
   build the simple thing.
6. **Owner questions**, briefly, skipping those that do not apply: right capability or a
   symptom; existing capability covers part of it; simpler product model; an assumption
   stopped being true; a future should become a goal or be dropped; every abstraction
   has one present need.
7. **Check autonomy** (`docs/intent/autonomy.md`): does the work include an ask-first or
   never action? Ask before, not after.
8. **Design and implement.** Produce the evidence the surface requires.
9. **Record** what passes the threshold (`intent-decide`). Update `scope.md`,
   `futures.md`, `questions.md` only if the project's truth materially changed.
10. **Report** in one line each: records consulted, triggers fired, records created or
    updated, breaking changes declared.
