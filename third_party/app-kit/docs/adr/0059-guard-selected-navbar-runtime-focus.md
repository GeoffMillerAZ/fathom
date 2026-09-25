# ADR-0059: Guard deferred focus in an explicitly selected navbar runtime

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-15
- **Extends:** ADR-0023 and ADR-0058; existing base/repair identities stay unchanged

## Context

The P09 maximum-bound acceptance suite records 523 passes and four first-item focus
failures. A separate passive trace shows Elements1.0.22 selects the first link, then
its deferred container-focus callback moves DOM focus to the scrollable menu while
the internal item index remains zero. Small menus retain link focus. This fires
ADR-0023's essential-behavior review trigger; CSS repair alone is insufficient.

## Decision

Keep the original module, base adapter and existing repair envelope unchanged. Generate
a separate private runtime derivative from the exact pinned ESM bytes. Require exactly
one match of the reviewed deferred-focus expression; any input drift refuses replay.
An independent CUE-validated envelope binds the paired repair, original/output ESM,
rule/schema and wrapper identities. Its verification remains not-verified.

Capture whether the menu is inside `data-appkit-navbar-repair` when scheduling. For
selected menus, deferred container focus is allowed only while connected, still open,
and without an active item. Unselected menus run the original unconditional focus.
Elements remains the sole owner of menu state, keyboard handling and lifecycle. No
new listeners, polling, timeout delay, item-selection rule or fallback state machine.

The unchanged acceptance suite must pass on the explicit derivative without a Home
workaround, relaxed focus threshold or replacement visual baseline. Additional tests
cover pointer-open container focus, close/removal before the timer and unselected
behavior. These do not imply full browser, assistive-technology or consumer acceptance.

Consumers must select the derivative and its paired companion deliberately; never load
both original and derivative modules in one document. Unselected original references
remain independently available. CLI repair delivery stays unavailable until verified
delivery binds this runtime too; ADR-0058's proposed eight-file inventory alone is no
longer sufficient for the enhanced maximum-menu keyboard contract. A subsequent
delivery decision must specify that complete inventory and migration instructions.

## Alternatives considered

- Restore focus with a second listener/timer: creates another interaction owner/race.
- Require Home after opening: changes the person's task to accommodate the defect.
- Replace the whole menu implementation: current evidence requires one focus guard,
  not another maintained keyboard, placement and lifecycle implementation.
- Update the upstream dependency: a new pin needs its own complete family replay;
  no current evidence establishes that it fixes this defect without other changes.

## Consequences

We own a small private source-specific runtime patch and its replay tests. Original
availability, source provenance and failed evidence stay intact. Existing exports and
consumer pins do not change. This decision does not certify or publish a target.

## Revisit when

- A selected lifecycle, pointer or keyboard task regresses or an unselected task changes.
- A new pinned Elements release changes the focus site or removes the need for it.
- Two consumers require a permanent shared interaction or broader runtime derivative.
- Verified CLI delivery is ready to define the complete runtime-bearing inventory.
