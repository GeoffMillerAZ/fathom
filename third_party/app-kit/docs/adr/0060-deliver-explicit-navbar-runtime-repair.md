# ADR-0060: Deliver the navbar repair with its exact runtime

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-15
- **Extends:** ADR-0036, ADR-0058 and ADR-0059

## Context

The explicit runtime guard now passes527 normal-size,466 actual200% and16 separate
pointer/lifecycle/isolation checks. It fixes the selected scrollable menu without
changing the original module or the existing base/repair artifacts. ADR-0059's
runtime-bearing delivery trigger is ready for implementation and delivery proof.
Loose copied companions would leave runtime selection and upgrade identity ambiguous.

## Decision

Add explicit `assets-adapt-navbar -repair -runtime ABSOLUTE_ORIGINAL_ESM` selection
for private JSON, `-out` and `-check`. Reject a runtime flag without repair selection
and repair without its original module. Read only the exact pinned original from an
absolute path: bounded single-link regular file, real immediate parent, no-follow/
nonblocking open and identity verification. No download or dependency installation.

The distinct CUE-validated `p09-navbar-repair-delivery-v1` envelope nests the unchanged
repair and runtime artifacts and binds its own rule/schema. Replay verifies original
HTML, base, companion, original ESM and derivative before output filesystem access.
Caller-supplied rehashing cannot authorize new bytes or a different runtime.

The nine-file inventory is navbar.templ, navbar.go, defaults.json, navbar_repair.css,
navbar_routes.go, navbar_routes.templ, elements-navbar.js, USAGE.md and artifact.json.
This replaces the prospective eight-file inventory in ADR-0058, not an existing export.
Use ADR-0036's private fixed-inventory writer/checker; artifact.json is written last.
Retain no-clobber, incomplete-output preservation and exit0/1/2 contracts. Base and
repair exports reject cross-checks and never silently upgrade one another.

Keep default JSON and all five base delivery files byte-identical. The app compiles a
private copy, scans both templates for Tailwind CSS, loads source CSS then repair CSS,
and explicitly serves the delivered module once. Never load original and derivative
Elements modules together. No runtime bootstrap, extra listener or second DOM owner.

The app binds NewNavbarWithRoutes data, summary, safe GET destinations, fonts/images,
routes and any backend. The native directory stays available regardless of script
readiness. The kit does not supply account policy, persistence, sign-out mutations or
an auth provider. Consumer integration remains deliberate and separately verified.

## Consequences

The experimental repair becomes privately obtainable and replayable, not a certified
target. Delivery requires exact byte/mode/no-clobber/drift/base-identity evidence.
Cold standalone compilation, CSS/runtime delivery and browser evidence must identify
the delivered inputs; earlier internal browser passes are not automatic adoption proof.
No consumer, existing preview, root dependency, release or global setting changes.

## Alternatives considered

- Export CSS/templates without runtime: fails the known maximum-menu entry contract.
- Replace default runtime or base files: invalidates original behavior and replay.
- Implicitly search/download the module: hides an executable dependency selection.
- Introduce a general runtime/export plugin registry: no present need beyond this pair.

## Revisit when

- An upstream pin removes the patch or changes its source/focus behavior.
- Exact delivery accepts drift, the wrong variant or a non-original runtime input.
- A consumer needs another lifecycle or composition boundary.
- A new platform or atomic publication requirement exceeds ADR-0036's guarantees.
