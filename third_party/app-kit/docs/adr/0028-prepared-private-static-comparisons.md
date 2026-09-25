# ADR-0028: Prepare private static comparisons before serving them

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-13
- **Extends:** ADR-0022 and ADR-0023

## Context

The Gallery compares inert source; compiler/DOM evidence now covers 439 accepted
official HTML inputs. Neither gives the owner a rendered comparison. The HTML archives
omit compiled CSS, font binaries and an exact rendering configuration. Their Tailwind
major does not identify a portal baseline or a reproducible complete page.

ADR-0022's reconstruction trigger has fired for the selected P02 pricing composition.
It is a complete static composition with inline SVG and no external resource/runtime.
The existing pinned Tailwind binary and local Inter assets can support an explicitly
chosen fixture without pretending to recover unspecified vendor defaults.

## Decision

Start with the frozen P02 original and saved templ attempt. Prepare original/compiled
light/dark HTML explicitly outside the web request path. Preserve their literal bodies
and compare namespace-aware DOM before/after templ formatting. Pin identical document,
CSS and font inputs; record chosen defaults separately from unknown portal defaults.
No compatibility, accessibility, performance or visual certification follows.

Keep generated pages in the ignored private asset store, with an immutable metadata
receipt. First-party CUE metadata admits exact reviewed receipt/file/style identities,
not arbitrary user-uploaded HTML. Viewing never builds, downloads or executes vendor
projects. Missing, stale or corrupt inputs refuse the comparison; original-source
routes remain independent and keep their existing inert-source policy.

Serve a dedicated comparison page with matched 390/1440-width and light/dark controls.
Only dedicated prepared-document routes may be framed by the same-origin Gallery.
Use opaque-origin sandboxed frames plus a response sandbox, no scripts/forms/remote
resources, exact inline-style hashes and data-font permission. Existing source routes
remain unframeable. Keep origin/loopback/GET-only/no-store boundaries.

Self-contained pages duplicate a pinned upright InterVariable font. Accept this
bounded inspection cost (at most 1 MiB per document) to avoid a new credentialed asset
server or runtime dependency graph. It is not the consumer production-font strategy.
Retain the font license and exact bytes. Browser enforcement and font/layout fidelity
remain separate checks on the real browser, not conclusions from response headers.

## Alternatives considered

- Compile or execute on a Gallery request: expands a source reader into an executor.
  Wins only after a named interactive need earns a separately reviewed execution boundary.
- Use Gallery CSS: its scanner does not cover private runtime-loaded source and its
  semantic theme would change the reference. Build the fixture's own pinned stylesheet.
- Call a chosen font the portal font: unsupported by the archives. Capture the actual
  portal configuration when access returns; record any new baseline explicitly.
- Build a general interactive preview host now: the fixed static case does not need it.
  Elements/React/MatDash retain their own runtime and lifecycle requirements.

## Revisit when

- Browser testing finds containment, font loading or meaningful layout differences.
- Another useful case needs external media, scripts, navigation or form submission.
- The duplicated-font cost prevents useful inspection at the declared document budget.
- A changed source/rule/toolchain invalidates the prepared receipt.
- Portal evidence establishes different defaults that need a separately named baseline.
