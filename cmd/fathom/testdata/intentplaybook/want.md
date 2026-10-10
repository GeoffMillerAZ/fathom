<!-- fathom:intent-playbook begin -->
# Intent

*Generated from `docs/intent/` by `fathom intent render --playbook`: edit the corpus, not this file.*

Durable intent for the fixture: why it exists and what must never break. It wraps onto a second line.

Why it exists: `docs/design/product-vision.md`. Vocabulary: `docs/intent/domain.md`.

## Open goals

- **G1** — A single-line goal
- **G2** — A wrapped goal title that runs onto a second line and a third line (ADR-1)
- **G3** — A title with no trailing period

Each goal's `Done when`, and the non-goals: `docs/intent/scope.md`.

## Unanswered questions

2 open, titles and blockers in `docs/intent/questions.md`: Q1, Q3.

## Constraints

3 invariants must always hold and are binding: `docs/intent/invariants.md`.
Autonomy: `docs/intent/autonomy.md`. Decisions: `docs/intent/decisions/README.md`.

## Changing intent

Edit `docs/intent/{README,scope,invariants,questions}.md`, then run
`fathom intent render --playbook` and commit INTENT.md in the same change.
`fathom intent render --playbook --check` (the `intent-render-check` gate)
exits 1 naming the first stale line.

corpus sha256: 4e1a973d8da78a8ef1124199e53d25c90781e57b462cdd94ff2d7b4acaadcca1
<!-- fathom:intent-playbook end -->
