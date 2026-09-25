# ADR-0077: Offer a bounded rectangular analytics export

- **Status:** Accepted
- **Type:** data
- **Date:** 2026-09-23
- **Extends:** ADR-0037, ADR-0056 and ADR-0069

## Context

The owner resumed bounded analytics/core improvements. The public single-dataset
writer emits a context section before its table, requiring variable-width CSV readers.
Gallery and Receipts demonstrate a separate need for rectangular domain reports.
Spreadsheet import of one selected dataset should not require that domain composition
or a consumer copy of the kit's numeric, missing-value and formula-safe encoding.

## Decision

Incubate `analytics.WriteTableCSV(io.Writer, Dataset, time.Time) error` alongside
the unchanged `WriteCSV`. Emit one 27-column header: the 17 existing context names
prefixed `context_`, then the 10 established value columns. Each selected observation
repeats its dataset context. Share private cell construction with the existing writer;
never parse and re-escape previously encoded text. Validate before writing bytes.
Keep explicit time, bounded rows/text/values, deterministic ordering and IO errors.

An empty selection emits only headers, not a fabricated observation or metadata row.
Callers needing empty-selection provenance must use the existing sectioned format.
Document this loss explicitly beside the API and Gallery example. Repeated context
costs more bytes; this is an import convenience, not a payload optimization or raw ledger.

Add a secondary native spreadsheet-CSV download to the exact Gallery recipe. Preserve
its old export route, filenames and bytes, current filters, strict response policy,
and all renderers. Surface the new choice through the existing experimental capability.
Review changed metadata pins; do not update consumers or installed servers implicitly.

ADR-0037's multi-dataset/domain composition remains private and unchanged. No new
query engine, report builder, spreadsheet runtime, generic option framework or
dependency. This owner-funded single-dataset API is experimental; permanent promotion
still requires independent consumer evidence. Existing signatures/formats are not broken.

## Evidence and limits

Require literal schema/value expectations, shared-cell agreement, hostile text,
missing/zero/negative/bounds/role states, validation before output, IO failure and
empty/max-row boundaries. Retain old exports byte-for-byte across frozen inputs.
Prove native Gallery selection/download in light/wide and dark/narrow states and
inspect its output. No claim of Excel/Numbers execution, complete accessibility,
consumer adoption or G3/core certification follows.

## Revisit when

- A named adopter needs multi-series fields, a different schema or empty-set metadata.
- Context repetition violates a selected export budget.
- Either writer disagrees with the shared presentation cells or re-escapes text.
- Two independent adopters support permanent promotion, or two releases pass unused.
