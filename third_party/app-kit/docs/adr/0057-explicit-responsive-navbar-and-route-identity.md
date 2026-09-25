# ADR-0057: Bind responsive navigation to one app-owned route model

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-15
- **Extends:** ADR-0023, ADR-0036 and ADR-0040

## Context

Frozen P09 contains desktop/mobile links, an Elements disclosure and account menu,
remote brand/avatar imagery and inert notification/sign-out examples. The existing
twelve-case pilot calls for typed repeated instances, firing ADR-0040's review trigger.
Duplicated route lists can disagree about current location; demo labels do not grant
authority to invent account or notification behavior.

## Decision

Keep exact `NavbarReference()` beside `NewNavbar(Navbar)`, generated from the verified
private original. Preserve source styles/icons and the pinned Elements 1.0.22 runtime.
One ordered primary-link model drives both layouts. Each link has a unique stable
key, escaped label and app-owned GET URL. An explicit matching `CurrentKey` applies
source current styling and `aria-current="page"` to both presentations; empty means
none current. Do not derive route identity from labels or substring matching.

Require a document-unique ID prefix, a named navigation landmark, mobile-control
label and explicit light/dark brand image URLs with meaningful alt. The paired brand
marks do not imply a home destination. The account image is decorative because the
account trigger has an explicit accessible label. Apps own asset rights and delivery.

Notifications and account navigation are optional. Absent options remove their controls.
An account menu requires one or more actual destinations. No sign-out mutation, POST,
auth provider, notification service or fake placeholder action is inferred. Apps must
not use GET destinations for sensitive mutations. Defaults leave route keys/URLs,
instance identity, landmark name and imagery unbound, with optional actions absent.

Validate bounded UTF-8 labels, safe root-relative/HTTPS URLs and unique keys before
rendering. Snapshot slices and optional objects. Triggers are non-submit buttons;
Elements owns disclosure/menu interaction, dismissal and focus. Do not introduce a
second state machine or Datastar ownership for these subtrees. Runtime-failure access
to essential destinations remains a required app integration responsibility.

Use the existing immutable five-file private export/check assurance. Compile a copy
in an app-owned module using templ and the standard library. No CSS/font/image/runtime
installation or app-kit runtime import is needed by the generated Go component.
Source/compiler/DOM, browser, imagery, accessibility and consumer gates stay separate.
Metadata remains experimental CLI tooling, not certified web-profile compatibility.

## Alternatives considered

- Separate desktop/mobile models: permit contradictory routes and selected states.
- Infer sign-out/notification actions from the source: invents app authority.
- Replace Elements or automatically retheme the source: changes the measured reference
  before demonstrating a benefit. A source-preserving explicit repair can follow evidence.

## Consequences

Apps receive readable route-bound navigation without duplicating its two layouts.
The adapter is source-specific, with asset/runtime and responsive-geometry proof costs.
Original imagery remains unavailable in the local baseline; no fidelity claim is earned
by substituting synthetic logos. No existing adapter output or consumer pin changes.

## Revisit when

- Long labels, zoom, focus, resize, runtime failure or repeated instances block navigation.
- A consumer needs mutation, a linked brand, additional layout states or morph persistence.
- Source/runtime drift invalidates replay or approved image bytes cannot be delivered.
- Two consumers demonstrate a shared permanent recipe boundary beyond this private pilot.

## Observed revisit: 2026-09-15

The [actual200% and module-outage diagnostic](../assessments/2026-09-15-navbar-typed-adapter.md#actual-200-zoom-focus-and-runtime-outage)
fires the first trigger: long-label containment, conservative focus screening and
runtime-owned destination access fail. Preserve the unchanged five-file base and red
receipts. Any repair must remain explicit, replayable and separately measured; this
observation does not introduce a second interaction owner or claim a repair is shipped.
