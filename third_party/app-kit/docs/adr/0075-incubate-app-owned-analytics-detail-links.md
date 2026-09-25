# ADR-0075: Incubate app-owned analytics row destinations

- **Status:** Partly superseded by 0076 (cross-revision layout-byte preservation only)
- **Type:** architecture
- **Date:** 2026-09-23
- **Extends:** ADR-0069 and ADR-0074

## Context

The core analytical task includes inspecting a row's detail. The public recipe
renders labels as plain text, so adopters must duplicate its table or rewrite HTML
to add navigation. Gallery already has app-owned service destinations. This fires
ADR-0069's missing-critical-task and ADR-0074's richer-navigation review triggers.

## Decision

Add experimental optional `analytics.Row.DetailHref`. Empty preserves the old
plain label and every existing rendered byte. A supplied destination is a bounded
root-relative URL with no external/protocol-relative target, malformed escaping,
control characters or backslash ambiguity. Validate before any renderer/CSV output;
do not silently drop a supplied invalid link. Escape the label and URL separately.

Both existing renderers use the same native table link, including zero, missing,
signed and table-only fallback rows. Preserve focus, wrapping and a 44px-high target;
no client code or whole-row click handler. CSV schema and values stay unchanged,
and the destination is presentation-only, never exported as another data column.

Apps construct destinations and retain authorization, existence, filter/return
context and recovery. No callbacks, route inference, routing engine, navigation
side effects or fetched URL. Gallery supplies its existing synthetic detail URLs
as the immediate reference; this does not prove independent consumer adoption.

## Compatibility and delivery

The optional field is additive for keyed literals, but adding an exported Go struct
field breaks unkeyed positional literals: migrate those to named fields. Existing
render/CSV function signatures and zero-link output stay unchanged. Rebuild selected
L3b CSS and review the changed `analytics.comparison` metadata pin when opting in.
No consumer pins, mirrors or installed MCP binaries are refreshed automatically.

## Alternatives and limits

- App-owned routes remain necessary; app-owned duplicate table rendering does not.
- External and callback-generated destinations stay outside this bounded need.
- A universal data-grid/row-action framework adds no value for this native-link task.
- One Gallery reference earns incubation, not permanent L3b promotion or G3 completion.

## Revisit when

- A named adopter requires external destinations, new tabs or non-navigation actions.
- Two independent consumers establish permanent-promotion evidence.
- Linked and unlinked data/export disagree, or empty links change legacy markup.
- A measured focus, wrapping or target-size failure defeats this native-link shape.
