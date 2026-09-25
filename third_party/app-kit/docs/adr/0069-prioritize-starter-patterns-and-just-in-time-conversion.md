# ADR-0069: Finish starter patterns and convert the long tail on demand

- **Status:** Partly superseded by 0072 (immediate completion priority only)
- **Type:** strategy
- **Date:** 2026-09-16
- **Extends:** ADR-0023, ADR-0042, ADR-0043 and ADR-0066

## Context

The owner prioritizes complete useful patterns and a strong core over immediately
optimizing every purchased component. Inventory, syntax translation, tested behavior
and certified target support are different measures. The legacy 439/657 syntax,
compiler and DOM result never meant that the entire catalog was converted.
Requiring every pilot and variant to clear every target gate before this milestone
would defer real reuse and turn optimization research into an unbounded prerequisite.

## Decision

Bound the starter conversion/readiness work to five complete pattern areas:
application shell/settings; analytics/table/detail/export; static content; a declared
client-heavy integration boundary; and themes/style-guide tooling. Prefer existing
kit primitives and recipes; a useful pattern need not port every commercial exemplar.
G5 terminal and G6 native delivery keep their separately declared acceptance gates.

The remaining commercial variants and deeper source-specific optimization become
just-in-time work or explicitly selected spare-capacity batches. Existing originals,
adapters, failure receipts and frozen oracles remain intact and discoverable. Deferred
does not mean passed, unsupported forever, or exempt from a consuming app's needs.
The twelve historical pilots remain research evidence, not twelve mandatory full
certifications for starter completion. This supersedes that breadth of G1/G4's pilot
work order, not ADR-0023's compatibility, fidelity or evidence rules.

Use a CUE-authored priority/readiness manifest beside the existing capability catalog
and revision-bound evidence contracts. Link capability IDs, tasks, profiles, gate
requirements, current scoped evidence, remaining work and an owner. Keep authored
planning/gate claims distinct from executed checks and semantic certification.
Do not copy paid source into this manifest or introduce another server/database.

Before just-in-time work, select one real task, target, original and affordable batch.
Freeze applicable acceptance, visual intent and performance budgets; reuse native or
existing kit behavior before writing a new adapter. Repair without weakening tests,
retain provenance, verify the consumer and return the smallest reusable improvement.
Permanent promotion still follows the layer-specific consumer bar. No blanket pass
can be inferred from a similar component, successful compilation or another profile.

Optimize to declared task budgets first: explicit CSS/assets, local interaction,
bounded/cancelled work, truthful data and failure recovery. Wasm, NATS, virtualized
rendering and more elaborate caches require a measured named need, not default adoption.
User-selected theme expansion is bounded to three additional light/dark families;
personal-site component extraction is assessed separately before admitting a slice.

## Deferred and rejected alternatives

- Full-catalog conversion now: resume only for a named consumer or owner-selected batch.
- Maximum optimization before reuse: costs need a failed budget or measured benefit.
- Remove failed gates to finish: rejected; change priority, not historical truth.
- Another catalog/MCP service: existing metadata/evidence surfaces provide the boundary.

## Revisit when

- A starter pattern lacks a critical task, recovery path or applicable acceptance check.
- An adopter needs a deferred component, profile or source revision.
- The owner selects a bounded spare-capacity batch with a stopping budget.
- Repeated conversions justify a shared rule, or a claimed pass becomes stale.
- A measured performance or stability budget cannot be met by the simpler profile.
