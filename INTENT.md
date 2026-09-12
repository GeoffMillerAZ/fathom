<!-- fathom:intent-playbook begin -->
# Intent

*Rendered by `fathom intent render --playbook` from `docs/intent/`. Everything
between the fathom markers is generated: edit the corpus, not this file. The
durable records are the corpus; this is its one-screen entry document, in the
shape the AI-native SDLC playbook opens with (intent → spec → plan → diff → PR
→ incident).*

## Problem

Durable intent for fathom: why it exists, what must never break, what is in and out of scope, what may come later, what has been decided and when to reopen it, who may do what. Practice: `agentic-kit/docs/project-intent/`. Skills: `intent-design` before design work, then `intent-decide`, `intent-doctor`, `intent-audit`, `intent-prune`, `intent-release`, `intent-incident`, `intent-init`.

Why it exists and what it should feel like: `docs/design/product-vision.md`.

## Proposed outcome

- **G2** — The A/B becomes admissible
- **G3** — The continuity-thread bet reaches its sample
- **G4** — Ambient fathom closes
- **G5** — The custodian closes
- **G6** — Zero claimed-unproven rows fleet-wide, then CI fails closed
- **G9** — A stranger can re-run the bench
- **G7** — The evidence plane's waves 1–2 ship and its oracle gate is read (ADR-026)

Each goal's `Done when` test, and the boundaries around them: `docs/intent/scope.md`.

## Affected users

Coding agents and the people running them, across one or many repos: the
surfaces are the Go client (`pkg/client`), the CLI (`cmd/fathom`) and the MCP
server. The domain vocabulary every one of them shares — what an observation,
an obligation, a layer and a claim ARE — is `docs/intent/domain.md`; who fathom
is for and what it should feel like is `docs/design/product-vision.md`.

## Constraints

38 invariants must always hold. Each one's statement and its enforcement
mechanism: `docs/intent/invariants.md`. An invariant is binding, not advisory;
`fathom intent audit` reports the ones whose enforcement claim no longer holds.

What fathom deliberately does NOT do: `docs/intent/scope.md` §Non-goals.
What an agent may do unasked, must ask first, and never does:
`docs/intent/autonomy.md`. Decisions and their revisit triggers:
`docs/intent/decisions/README.md` (records are the `## ADR-NNN` sections of `ADR.md`).

## Open questions

- **Q3** — Delete the 24 stale worktrees and 22 `worktree-*` branches (INC-1)?
- **Q4** — Force-multiplier or product?
- **Q5** — The threat model
- **Q6** — Has XT-6b's trigger fired?
- **Q16** — Should honesty-debt counts ever leave the machine?
- **Q18** — Does the practice's owner accept `{oracle: …}` annotations on condition lines?
- **Q19** — Where does first-seen live across machines?
- **Q20** — Who owns the `{oracle: …}` grammar if a second evaluator appears?

What each blocks and who can answer it: `docs/intent/questions.md`.

## The rest of the loop

- **spec / plan** — `docs/plans/*.md`, gated: a box flips to `[x]` only when
  `fathom verify --service <svc> --plan <plan>` proves it (`docs/plans/README.md`).
- **diff / PR** — a conventional commit with a `Verified:` line; observable
  behavior also gets a `CHANGELOG.md` line.
- **incident** — `docs/intent/incidents.md`: what happened, the evidence, and
  what would have caught it, driven to an invariant, a test or a decision.

Re-render this file with `fathom intent render --playbook`.
<!-- fathom:intent-playbook end -->

