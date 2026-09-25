# ADR-0050: Repair menu geometry without taking over its interactions

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-15
- **Extends:** ADR-0049; immutable v1 delivery and source-reference contracts remain

## Context

INC-41 fires ADR-0049's runtime/focus review trigger. At actual200% zoom, the valid
long-label menu extends below the viewport and container focus can override Elements'
selected item. Normal captures and successful compilation did not expose this.
Elements exposes no available-height variable in its pinned positioning path.

## Decision

Build and test an explicit, source-specific companion as an internal candidate first.
Do not alter the original, pinned Elements bytes, source-styled constructor or existing
six-file export. Browser proof precedes promotion into a new explicit delivery revision.

The companion may measure the trigger and visual viewport, reserve a declared gap/edge,
and bound menu height to the larger available above/below space. Elements retains
placement and flipping; the companion never writes top/left or substitutes a fixed
overlay. Source gap changes must be reflected in the explicit mounting options.
Scoped observers and one scheduled frame update size for resize, scrolling, opening
and the runtime's position-only inline-style updates; no document subtree is observed.

If deferred container focus displaces an existing selection, the companion may restore
only the single enabled item Elements already marked tabindex0, and only while the
open menu still owns focus. It cannot choose an index, consume input, manage selection,
open/close a menu, submit an action or retry a mutation. Stale callbacks must not
steal focus after dismissal, navigation, disconnection or disposal.

Mounting is explicit per connected wrapper, with duplicate mounts refused and an
idempotent disposer. Owned styles/markers restore their prior values. No global DOM
observer, automatic remount, persisted selection or Datastar lifecycle is implied.
This is a private source-specific repair, not a new public menu framework.

## Evidence and promotion

Pure geometry and synthetic lifecycle tests are necessary but not browser proof.
The original failing zoom oracle stays intact. An explicitly labeled repair variant
must exercise it with hash-bound companion inputs, including long-label every-item
focus, native actions, light/dark, narrow/normal widths, resizing and multiple instances.
Test close/dispose during deferred work and confirm the unchanged reference still
matches. Insufficient room or unproven platform behavior remains a failure, not a
silent fallback or weaker threshold. Real consumer acceptance remains separate.

## Alternatives rejected

- Fixed viewport overlay: fits but discards the purchased anchored interaction.
- Replacement keyboard state machine: duplicates the selected runtime's authority.
- Implicit patch to every Elements menu: changes source references and other consumers.
- Silently adding JavaScript to v1 export: invalidates its six-file and identity contract.

## Revisit when

- The companion cannot bound the menu without overriding runtime placement.
- A pinned upstream fix makes focus reconciliation or geometry measurement unnecessary.
- A named consumer needs different gap/placement, nested actions or DOM morphing.
- Browser/assistive input shows focus theft, observer feedback loops or unreadable items.
- Promotion needs a delivery/schema change: specify migration and replay before export.
