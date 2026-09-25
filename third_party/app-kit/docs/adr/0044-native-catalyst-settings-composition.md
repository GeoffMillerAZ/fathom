# ADR-0044: Adapt the complete Catalyst form with native controls and explicit authority

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-14
- **Extends:** ADR-0023 and ADR-0043; no existing adapter or profile contract changes

## Context

P10 is a complete React settings composition, not an HTML block. Its separately
reviewed hardened-v3 derivative already fixes dependent values, reset and retained
over-limit drafts. A smaller generic form would discard much of the purchased design.
Recreating Headless UI in Go, or forcing a request on every keystroke, is unnecessary.

## Decision

Develop an explicit private native-form adaptation of the complete P10 settings form:
organization name, bio, email/visibility, address/country/region, currency and actions.
Retain source section order, descriptions, typography, spacing and light/dark styling.
The surrounding application navigation/shell and business persistence remain app-owned.
This is a composition port, not an arbitrary TSX compiler or a source-faithful badge.

Prepare a pinned rendered form from the verified hardened source and selected build,
with its exact CSS/font/flag closure, defaults, canonical options and original lineage.
Keep the original, hardened source, rendered reference and generated target separate.
Normal Go/templ generation replays retained private inputs without React, Node or an
LLM; initial source preparation may use its already-selected private React toolchain.
Any prepared-input normalization is named and checked; raw captures remain available.

Native selects/checkboxes replace custom Headless controls. Submit only country code
and region, never a caller-authored country object or authoritative option inventory.
Typed app-selected canonical data validates submissions in Go. Invalid drafts are
retained within explicit input bounds, never silently shortened or normalized.
Biography counts 240 Unicode code points; invalid UTF-8 is rejected before counting.
An example email policy is explicitly narrower than arbitrary address acceptance.

Ordinary HTTP works without application JavaScript. An optional small local controller
provides immediate bio/count and country-region feedback, clears stale success, and
restores a complete initial snapshot without remounting the focused Reset control.
It owns only that form's transient state; it does not authorize or persist anything.
Without it, country/region choices remain available and server validation/recovery is
explicit; reset restores the app-selected initial snapshot through a native request.
Record that navigation difference instead of claiming same-node reset without scripts.
The native Reset button belongs to a separate hidden transport form; Enter in an
edit field must validate, never discard the draft (INC-27).

Native popup appearance, minimum control sizes and correction/focus states are explicit
adaptation differences. Prove default/error/reset states at narrow/wide widths, both
themes, keyboard and actual zoom, with scripts both available and withheld. Keep source
failures and target defects separate. Freeze required gates before repairs; no renderer
availability, catalog-target count or certification is earned merely by this decision.

## Consequences and alternatives

Paid markup, classes, assets and generated bodies remain private. First-party parsers,
policy helpers, controllers and synthetic tests may be tracked without vendor bodies.
The existing React reference remains the choice for source-native custom interactions.
A first-party generic form renderer would lose fidelity; a general React transpiler
would add unbounded behavior mapping. This source-specific bridge has one present need.
No new dependency, backend, public API, consumer migration or host registration follows.

## Revisit when

- Native behavior cannot preserve an essential task or meet the frozen quality budget.
- Retained rendering cannot replay after a selected source/runtime upgrade.
- A second real form needs the same controller/model boundary and supplies adoption proof.
- An app needs offline save, rich editing or new fields that invalidate the current seam.
