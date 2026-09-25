# ADR-0055: Inspect retained PNG evidence in the private Gallery

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-15
- **Supersedes:** ADR-0043 only for Gallery access to declared image bytes

## Context

P06's immutable private evidence record already retains 24 browser captures, but
Gallery shows only metadata and gate counts. The owner needs to inspect converted
designs. ADR-0043's metadata-only boundary currently prevents that useful task.

## Decision

Retain ADR-0043's store, identity, publication, integrity and non-certification rules.
Extend only local Gallery with exact GET access to receipt-declared `role=image`
slots. Reuse the same checked bundle snapshot as metadata reads, including original
membership and all content hashes. Never expose arbitrary input/output/report bytes.

Only PNG is admitted: signature, bounded dimensions and full decoding must pass;
limits are 8 MiB encoded, 8192 pixels per edge and 16 million decoded pixels.
Return exact retained bytes, not a regenerated visual baseline. Invalid images remain
errors. No SVG, HTML, image proxy, external URL, source execution or live preview.

Image routes inherit loopback peer/Host, same-origin, GET-only, no-store, nosniff and
frame denial. A successfully rendered evidence detail may add same-origin `img-src`
to its otherwise unchanged restrictive CSP; errors and other private pages retain
their existing policy. No script permission or new browser runtime is introduced.
Successful raw-PNG responses alone allow inline styles for the browser's generated
image document; strict inherited styling caused observed Chrome CSP failures. PNG
type/decoding and nosniff precede this exception. Default sources, scripts, forms and
frames remain denied; neither HTML pages nor errors receive inline-style permission.

The detail view selects one capture or two stacked captures using native GET controls,
with exact slot IDs, hashes, dimensions and full-size image links. Captured pixels keep
their original theme independently of Gallery chrome. Metadata APIs and MCP remain
byte-free and shape-compatible. Catalog references link by exact subject ID, without
claiming that a record exists or promoting a CLI reference into a web component.

Screenshots are historical authored observations, not current runtime, keyboard,
semantic, visual-equivalence or target certification. Preserve `not-verified` and the
original source recovery path. New adapters need explicit evidence retention; temporary
captures are not silently imported. No consumer, dependency or publication changes.

## Consequences and alternatives

The viewer makes existing captures useful without running vendor code. Full integrity
checks on each request cost bounded decoding and archive reads; no unchecked cache is
introduced. Raw PNG-only links avoid a CSP change but make comparison/context recovery
cumbersome; retain them as the full-size option. A live interactive preview still needs
its own runtime and containment decision, not permission inferred from image access.

## Revisit when

- Named captures exceed the bounds or measured reads impede interactive inspection.
- A viewer presents historical pixels as current behavior or certification.
- Another format, live interaction, image annotation, or public/remote access is needed.
