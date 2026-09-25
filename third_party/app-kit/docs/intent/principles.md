# Principles

The kit guides, it does not dictate. [Vision](../../VISION.md) defines the ambition;
these rules resolve the tensions that arise while pursuing it.

1. **Usefulness leads; craft makes it effortless.** Start with the decision or task a
   person needs to complete. Give it hierarchy, clear language, discoverable controls,
   responsive feedback and a deliberate visual identity. Polish cannot conceal an
   ambiguous number, missing action or broken keyboard path.
2. **Reuse complete behavior at the smallest honest boundary.** Reuse tokens,
   interactions and typed primitives freely; package a workflow when repeated need
   proves its boundary. A settings page includes validation, saving, failure and
   recovery, not just fields. Domain policy remains in the app.
3. **A shared foundation should increase individuality.** Shared roles, accessibility
   and behavior support different typography, density, palette and composition.
   Avoid making every product the same dashboard of cards. Brand choices belong to
   a consumer profile; token names express meaning.
4. **Opinionated defaults are excellent, explicit and escapable.** Offer a complete
   recommended path with known costs. A documented divergence is valid adoption.
   Repeated divergence is evidence that the kit's default needs work.
5. **Ambition earns implementation through a proof.** The owner can admit a bounded
   incubation for a named workflow without waiting for popularity. Define its user,
   comparison, constraints, exit evidence and kill condition. Generalize only after
   the existing layer promotion bar is met (ADR-0017/0018).
6. **Determinism removes accidental work.** Go and CUE own typed logic and contracts;
   generation, artifact provenance and explicit versions make builds repeatable.
   Runtime capability tests and visual review cover what type checks cannot.
7. **Choose technology for the capability it enables.** Prefer the owner's Go, CUE,
   WASM and Charm strengths. Evaluate new releases early, pin deliberately, measure
   their cost and keep a working fallback. Bleeding edge is a research posture,
   not permission to make every consumer absorb an experiment.
8. **Local interaction deserves local response.** Focus, selection, disclosure and
   other ephemeral view state need not round-trip. Authoritative business state has
   one owner. Choose a renderer/transport profile from actual latency, offline,
   security and interaction needs (ADR-0021).
9. **Every improvement has a return path.** Capture an observed failure or successful
   app pattern, make the smallest reusable improvement and demonstrate it. Upgrade
   another consumer when applicable; check the original problem is gone. Layer-specific
   promotion rules determine when a second consumer is required.
10. **Evidence travels with the claim.** Distinguish available, selected, integrated,
    verified and adopted. A catalog count, instruction file, screenshot or green
    package test alone cannot prove a usable application.

## Tensions we keep visible

| Tension | Resolution |
|---|---|
| High data density vs easy comprehension | progressive disclosure, clear grouping, purposeful density modes; test actual tasks |
| Consistency vs app identity | share semantics and interaction contracts; allow app-authored composition and tokens |
| Fast local iteration vs reproducible delivery | local replace is useful development; release proof uses a resolvable pin and clean checkout |
| Rich interaction vs payload/complexity | begin with HTML/SVG; measured need admits an isolated richer renderer |
| Fast agent autonomy vs reliable outcomes | automate computable checks; require human or adversarial judgment for what they cannot decide |
| Shared behavior vs premature framework | recipes compose existing pieces first; permanent APIs need real consumer evidence |
