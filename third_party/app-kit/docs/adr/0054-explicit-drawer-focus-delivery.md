# ADR-0054: Select drawer focus repair through an explicit CSS companion

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-15
- **Extends:** ADR-0036 and ADR-0053; existing five-file delivery remains unchanged

## Context

P08's [actual200% diagnostic](../assessments/2026-09-15-drawer-typed-adapter.md)
finds unpainted nested menu links and weaker browser-default cues. This fires
ADR-0053's focus review trigger. Task navigation and Elements' nested Escape still
work; neither a new runtime nor a replacement keyboard state machine is justified.
The unchanged base also reproduces in two isolated cold consumer render builds.

## Decision

Keep the base artifact, five-file export and DrawerReference byte-identical. Prepare
a separate source-specific focus stylesheet as an internal candidate. Scoped browser
proof precedes CLI availability; certification and real adoption remain separate.

Use an explicit app-owned `data-appkit-drawer-focus` wrapper around the selected
NewDrawer output and load the companion after source CSS. Both choices are required.
Do not wrap the original specimen or unrelated components. The wrapper contributes
no interactive role or focus target; no extra constructor or JavaScript is needed.

The repair may enlarge selected targets, wrap their text and paint contrast-tested
inset outlines that survive the menu's own overflow clipping. Preserve source colors
where they pass; inherited colors and themes require actual local-backdrop review.
Do not change the already-passing close control or steal legitimate dialog-entry
focus merely because its noninteractive container lacks an outline. Elements retains
modality, selection, Escape, focus return and placement without a second state machine.

The explicit `assets-adapt-drawer -repair` variant has a new nested envelope and exact
six-file inventory: unchanged drawer.templ, drawer.go and defaults.json, plus
drawer_focus.css, variant USAGE.md and artifact.json written last. Schema, rule, CSS
and wrapper identities are bound; the nested base is exact replay, not a new base ID.
Default JSON/output/check behavior stays unchanged. A base check rejects repair output
and vice versa. No implicit migration, dependency install or asset/runtime delivery.

Reuse private filesystem assurance: replay original source before I/O, never overwrite,
require exact members/modes, and reject a changed CSS body even with a recomputed hash.
Retain failed source/candidate results and review oracle corrections independently.
Paint verification includes opaque/translucent backdrop composition and inset/outset
clipping; test helpers must exercise the actual measurement calculation.

## Alternatives considered

- New enhanced constructor: convenient, but an app-owned wrapper already expresses
  the entire selection contract without duplicated validation or generated code.
  Wins if repeated consumers demonstrate a more substantial composition boundary.
- Reuse P07's repair runtime: its geometry/focus reconciliation addresses different
  measured failures and would introduce unnecessary ownership and lifecycle costs.
- Patch the base or every Elements menu: silently changes preserved references and
  existing delivery. A named opt-in companion keeps comparison and fallback honest.

## Consequences

Apps can select the measured improvement without changing typed data or interaction
ownership. They must provide CSS ordering and verify their own backdrop, narrow/zoom
states and runtime-failure access. No universal accessibility or performance promise.

## Revisit when

- A selected app's CSS/theme changes make rings unreadable or targets clip content.
- Touch, assistive input, resizing or dynamic lifecycle reveals a task regression.
- An upstream source/runtime fix makes the companion unnecessary.
- Two consumers need a shared wrapper/constructor beyond this private adapter.
- Default replay changes or an unselected reference receives repair styling.
