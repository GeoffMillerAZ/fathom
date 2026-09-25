# ADR-0051: Deliver the menu repair as an explicit private variant

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-15
- **Extends:** ADR-0036, ADR-0049 and ADR-0050

## Context

ADR-0050's delivery-promotion trigger fires after the companion passes 314 normal
and 210 actual-zoom/resize/lifecycle browser checks. The unchanged v1 still fails
focus and containment. [Evidence](../assessments/2026-09-15-menu-typed-adapter.md)
retains both outcomes. Copying loose internal source files is not a durable
consumer delivery contract.

## Decision

Add an explicit `assets-adapt-menu -repair` selection for JSON, `-out` and `-check`.
Its distinct `p07-menu-repair-v1` envelope contains the unchanged base artifact,
repair implementation identity, module/CSS bytes and hashes, and the reviewed
mount defaults (gap 8px, edge 16px). Replay verifies original source, base and
companion before filesystem I/O; caller-provided hashes are not trusted evidence.

The private fixed inventory has eight files: existing template/model/base CSS and
defaults, `menu_repair.mjs`, `menu_repair.css`, variant-specific `USAGE.md` and
`artifact.json`. Use ADR-0036's secure no-clobber write/check helpers and existing
exit semantics. Base and repair inventories must reject cross-checks. No overwrite,
automatic upgrade, tool/runtime installation or publication is introduced.

Keep default JSON, six-file export and its replay identity exactly unchanged.
Do not edit the base implementation merely to insert the new variant. The existing
source/reference and typed renderers retain their behavior; repair is an optional
companion for `NewMenuEnhanced`, not a silently modified constructor.

The app loads repair CSS after source/menu CSS and explicitly imports and mounts
the module on each connected selected wrapper after Elements is defined. It owns
the returned disposer before removal/remount. Do not deliver an auto-mount bootstrap
or the browser probe's test handle. Gap/edge changes require consumer evidence.
Elements retains input/placement/selection; auth, actions and fallback stay app-owned.

## Consequences

This makes a tested private repair obtainable and replayable, not a certified target
or real-consumer integration. Preserve source/runtime/license and proof boundaries.
Delivering eight files must additionally prove exact bytes, modes, drift/no-clobber
refusal, base compatibility and browser input identity. Existing standalone Go proof
applies only to unchanged generated Go, not automatically to the new full delivery.

## Alternatives rejected

- Change v1 in place: invalidates previously retained exports and exact replay.
- Separate unbound companion directory: makes source/repair selection easy to mix up.
- Automatic global mounting: takes lifecycle ownership away from the consuming app.

## Revisit when

- A consumer needs automatic lifecycle, different spacing, nested or stateful actions.
- The source or Elements pin invalidates the repair or a browser containment/focus gate.
- New delivery loses default byte identity or can be checked as the wrong variant.
- Upstream fixes make the companion unnecessary; retain reproducible historical replay.
