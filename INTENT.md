<!-- fathom:intent-playbook begin -->
# Intent

*Generated from `docs/intent/` by `fathom intent render --playbook`: edit the corpus, not this file.*

Durable intent for fathom: why it exists, what must never break, what is in and out of scope, what may come later, what has been decided and when to reopen it, who may do what. Practice: `third_party/agentic-kit/docs/project-intent/`. Skills: `intent-design` before design work, then `intent-decide`, `intent-doctor`, `intent-audit`, `intent-prune`, `intent-release`, `intent-incident`, `intent-init`.

Why it exists: `docs/design/product-vision.md`. Vocabulary: `docs/intent/domain.md`.

## Open goals

- **G2** — The A/B becomes admissible
- **G3** — The continuity-thread bet reaches its sample
- **G4** — Ambient fathom closes
- **G9** — The bench is honest, cheap to re-run, and the 2026-09-13 reading is the baseline
- **G13** — The Observatory console closes
- **G14** — Fathom is a responsible tenant (ADR-047, vision 36)
- **G15** — Language breadth and depth, measured (ADR-048, vision 37)
- **G16** — Reach: fathom installs in one step and serves the clients people use (ADR-049, vision 38)
- **G17** — The system graph: proof crosses service boundaries (ADR-050)
- **G18** — Dependencies: upgrade impact from what is already on disk (ADR-050)
- **G19** — Spec interop and the certified proof tier (ADR-049)
- **G20** — Knowledge retrieval: doc questions answered at grep's rate or better (ADR-049)
- **G21** — Governance that learns and reaches reviewers (ADR-049)
- **G22** — Progress and economics, computed (ADR-049)
- **G23** — Test intelligence: the tests that matter, ranked and selected (ADR-051)
- **G24** — Work intelligence: effort, friction and cost joined to proof (ADR-051)
- **G25** — Change and compliance receipts (ADR-051)
- ...and 18 more: `docs/intent/scope.md` §Current goals

Each goal's `Done when`, and the non-goals: `docs/intent/scope.md`.

## Unanswered questions

20 open, titles and blockers in `docs/intent/questions.md`: Q3, Q4, Q5, Q16, Q18, Q19, Q20, Q23, Q24, Q25, Q26, Q27, Q28, Q29, Q30, Q31, Q33, Q35, Q36, Q37.

## Constraints

66 invariants must always hold and are binding: `docs/intent/invariants.md`.
Autonomy: `docs/intent/autonomy.md`. Decisions: `docs/intent/decisions/README.md`.

## Changing intent

Edit `docs/intent/{README,scope,invariants,questions}.md`, then run
`fathom intent render --playbook` and commit INTENT.md in the same change.
`fathom intent render --playbook --check` (the `intent-render-check` gate)
exits 1 naming the first stale line.

corpus sha256: 0ae18c0687f566445e5d37166ebb87298a8ad9994cd8f7d6da5410dee18e1927
<!-- fathom:intent-playbook end -->

