# ADR-0049: Keep the menu reference separate from app-owned actions

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-15
- **Extends:** ADR-0023, ADR-0036, ADR-0040

## Context

Frozen P07 already has source/compiled Elements interaction evidence. Its links
and POST action are placeholders, so constant translation is not a reusable account
workflow. Typed action binding fires ADR-0040's review condition; it does not justify
a second menu state machine or an app-kit identity backend.

## Decision

Add a source-specific private adapter. Retain an unchanged constant `MenuReference`
alongside `NewMenu` and explicit `NewMenuEnhanced`. All use the artifact's pinned
Elements runtime; neither constructor installs or embeds it. The reference remains
an inert design specimen when served with form submission denied.

The app-ready model has a bounded label, zero to twelve ordered local/HTTPS links,
and an optional single POST action. Require at least one link or action. The action
has a same-origin local endpoint, explicit CSRF field/token and optional disabled
presentation. The adapter adds a non-submit trigger, hidden CSRF input and conditional
form; it preserves source styles and icon geometry. These are declared adaptations,
not claims that the original supplied authorization or working account actions.

Snapshot inputs and escape plain text. Validate before rendering. The app owns
authentication, token issuance/checking, authorization, duplicate-effect prevention,
successful redirects and visible rejection/recovery. Disabled presentation is not
authorization. Do not insert a menu containing its own form inside another form.

Enhanced CSS separately owns wrapping, target size, viewport bounds and visible
keyboard focus. It must not leak into the unchanged reference. Local input, selection,
Escape and dismissal stay with Elements. No Datastar morph/persisted-state contract
or no-script menu availability is implied. Essential destinations need an app-owned
fallback if the required runtime cannot load.

Use ADR-0036's no-clobber private export/check. Keep source, runtime, rule, model,
defaults and CSS identities; deliver no licensed runtime or fonts implicitly. The
default JSON output and future catalog metadata remain explicitly not-verified.

## Alternatives rejected

- Native disclosure relabeled as a menu: would silently lose the menu keyboard contract.
- Universal action/event bus: adds an authority model where explicit links/forms suffice.
- Editing the original to add tokens: would erase the comparison and source boundary.

## Consequences

Consumers supply the action backend, runtime/CSS/font closure and recovery behavior.
Compiler, browser, standalone and real-consumer evidence remain distinct obligations.
This additive adapter changes no existing API or certified target; G1/G4 stay active.

## Revisit when

- A consumer needs grouped/checked items, several mutation actions, nested menus,
  persisted selection or DOM morphing.
- The pinned runtime loses focus, submits a disabled action or fails its failure path.
- A real consumer cannot use the explicit endpoint/token or fallback contract.
- Source drift or a rehashed artifact passes replay without the required original.
