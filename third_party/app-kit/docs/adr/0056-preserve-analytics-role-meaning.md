# ADR-0056: Preserve declared analytical meaning beyond color

- **Status:** Accepted
- **Type:** data
- **Date:** 2026-09-15
- **Decision drivers:** color-independent interpretation, complete exports, explicit migration

## Context

`analytics.Dataset` already associates app-defined token roles with legend labels.
Multi-role plotted data requires those labels. `RenderBars` previously offered only
colored swatches, while its table and `WriteCSV` omitted the mapping. A table-only
layout removed the legend altogether, losing distinctions such as Scheduled/Recorded.
I11/I12 and G3 require meaning to survive color loss, fallback and export. ADR-0037's
shared-writer migration trigger applies; its exact-schema refusal remains binding.

## Decision

Resolve each row's effective role (empty means `accent`) against the validated,
app-authored legend by role identity, never list position or inferred color meaning.
The visible data table includes a **Meaning** column when any selected row has a
declared label. Unmapped rows say **Not specified**. Missing observations retain
their reason and any declared label; presence and meaning remain separate concepts.
No matching labels means the existing compact table. A table-only view still does
not claim a chart legend or render absent marks.
Rendered legend swatches use explicitly decorative SVG presentation attributes;
strict style CSP is preserved, not relaxed to allow inline style attributes.
Table cells align to the top so wrapped labels do not detach values and meanings
halfway down a tall row. Native scrolling and full, untruncated text remain available.

`WriteCSV` appends `role` and `role_label` after its existing eight data columns.
The seventeen context records and original eight values remain unchanged. Unmapped
labels are empty, not inferred; both added fields receive the existing spreadsheet
text protection exactly once. Row labels, numeric values and missingness are unaltered.
The internal CSV checker and both in-repo report projections adopt the explicit shape.

## Compatibility and migration

This is **breaking CSV output**, despite unchanged Go signatures. The data section is
now ten columns; Gallery comparison exports grow from31 to33 columns and Receipts
evidence exports from33 to35. Consumers must update exact-header/width checks together
with their selected kit revision. Prefer named columns, retaining refusal of unknown
schemas. Do not silently discard the new meaning fields. Numeric indexes in the first
eight data columns and existing context keys retain their meaning.

The Gallery's synthetic service groups supply explicit API/Worker labels to the public
renderer and export. These are categories, not reliability ratings. The paired-period
chart keeps its own current/previous encoding. No external consumer upgrade is implied.
Rebuild consumer CSS after upgrading the renderer; no runtime JS or token change.

## Alternatives considered

- Only retain a detached color legend: still requires color to associate rows.
- Append meaning to row labels: changes app-owned labels and conflates distinct fields.
- Infer meaning from `danger`/`positive`: token names are not app-owned domain truth.
- Keep the eight-column export: knowingly drops information preserved on screen.

## Evidence and limits

Require per-row mapping, missing/default/unmapped cases, escaping, unchanged established
fields, old-schema refusal, both report projections and narrow keyboard/browser evidence.
Preserve the historical144-export digest after removing only the two reviewed new
columns, plus independent assertions for their values. No blind golden replacement.
This fixes declared meaning transfer, not the truth of the app's classifications,
assistive-technology conformance, target certification or full G3 completion.

## Revisit when

- A consumer cannot preserve meaning with this explicit mapping or needs richer dimensions.
- A role label diverges between table and export, or an obsolete schema is silently accepted.
- A consumer upgrade needs a versioned legacy export boundary; do not invent one speculatively.
