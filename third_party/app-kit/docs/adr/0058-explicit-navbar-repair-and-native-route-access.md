# ADR-0058: Keep navbar repair explicit and native route access persistent

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-15
- **Extends:** ADR-0036 and ADR-0057; the five-file P09 base remains unchanged

## Context

P09's [actual200% diagnostic](../assessments/2026-09-15-navbar-typed-adapter.md#actual-200-zoom-focus-and-runtime-outage)
records38 failures: focus cues, long desktop text and destinations hidden by a module
outage. This fires ADR-0057's first revisit condition. Enhanced keyboard interaction
still works when Elements is available; a replacement runtime is not justified.

A separate synthetic browser experiment exposes unregistered hidden/popover links
using `:not(:defined)` CSS. Native links work, but registration hides a focused link;
by the next animation frame focus moves to BODY. This is not actual Elements timing
proof, but it disproves automatic hiding as a generally focus-preserving fallback.

## Decision

Keep exact NavbarReference, NewNavbar, base artifact and five-file delivery unchanged.
Add an explicit source-specific repair candidate with a scoped stylesheet and
`NewNavbarWithRoutes(Navbar, summary)` companion constructor. Validate/snapshot the
same route model using the base contract; summary is bounded, escaped app-owned text.
The companion renders NewNavbar inside `data-appkit-navbar-repair` and a native
details/summary route directory. Primary current-key semantics remain exact; optional
notifications/account destinations are native GET links, not inferred mutations.

The directory stays available with or without JavaScript. It is not labeled as an
error and is never automatically hidden by runtime readiness. Browser-native details
owns its own distinct subtree; Elements retains the original menu/disclosure subtree.
No polling, duplicate interaction listeners, fallback timeout or state-transfer runtime.
Apps choose the summary/landmark name and place this deliberate additional entrypoint.

Load repair CSS after the source CSS. Permit the header and route groups to grow/wrap
instead of clipping long labels; paint contained, contrast-tested focus on the failing
links. Preserve source colors and already-working controls where possible. The scoped
wrapper must not style unselected references or unrelated navigation instances.

Bind base, rule/schema, CSS, companion Go/templ and wrapper identities separately.
No existing output gains the companion implicitly. Browser/consumer evidence precedes
CLI repair selection; a later explicit eight-file variant can reuse private filesystem
assurance. It will keep three base files, add CSS and companion Go/templ, and write its
own usage/artifact envelope. This decision alone does not ship that CLI or certify it.

## Alternatives considered

- `noscript` alone: misses module503 and other failures with JavaScript enabled.
- Automatically reveal/hide the original undefined elements: late upgrade loses focus
  without another readiness/transfer state machine; synthetic evidence above is red.
- Replace Elements with a new navbar runtime: duplicates working interaction behavior.
- A native-only navbar: remains a valid future explicit target if a consumer chooses
  that different visual/interaction contract; it is not a silent source-faithful repair.

## Consequences

The extra native directory costs markup and a visible entrypoint, but needs no script
or backend and remains useful during delayed/failed loading. App summary and route
labels need real UX review. Static, runtime-failure, late-load focus, repeated-instance,
long-label, zoom and selected/unselected styling gates remain distinct.

## Revisit when

- The extra entrypoint confuses a real consumer or fails its density/task budget.
- Runtime registration, resize or update changes focus in the native directory.
- Source/theme drift invalidates the scoped selectors, geometry or contrast.
- A pinned native-only alternative preserves the desired task/design at lower cost.
- Two consumers justify a permanent shared recipe or a reviewed readiness handoff.
