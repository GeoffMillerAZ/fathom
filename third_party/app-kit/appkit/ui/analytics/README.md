# Analytics composition

An opt-in recipe for a small, ordered comparison such as application stage counts
or monthly cash outflow. The app selects and aggregates one `Dataset`; the kit
validates its declared meaning and derives chart, visible captioned table and CSV
from those rows. No query engine, persistence or application state is introduced.

`Dataset.Validate(now)` requires an explicit clock, unit, grain, aggregation,
snapshot or half-open range with timezone, provenance, source freshness,
completeness and uncertainty. Source freshness refers to an input observation,
not a newly computed forecast's as-of date. Use `Freshness.UnknownReason` when
input age is not known. A freshness age equal to `MaxAge` is fresh; greater is stale.
The app remains responsible for whether those declarations are true.

`RenderBars(dataset, viz.BarsOptions{}, now)` returns `(string, error)`, forces
printed values and generates truthful context plus a data table. Missing values
are `nil` with a reason; a pointer to zero is a real zero. Missing rows remain in
the table/CSV and do not become SVG zeroes. Partial/unknown coverage needs a note.
Bounds are app-authored low/high limits, not an inferred confidence percentage.
Multi-role charts require a role-to-meaning legend. When selected rows have declared
labels, the table adds **Meaning**, including missing rows and table-only fallback;
unmapped rows say **Not specified**. No labels means no extra column. The app retains order.
Legend swatches are explicitly decorative SVGs (`data-analytics-legend-swatch`), using
presentation attributes so strict style CSP needs no inline-style exception. Scoped
SVG audits should declare these swatches separately from the data chart.

This recipe accepts at most 200 rows, 2048 bytes per text field and numeric
magnitudes up to 1e12. Larger inputs must be selected or aggregated in the app;
the kit never truncates them silently. `Decimals` is 0..2, matching the existing
`viz.Bars` formatter and the current count/USD use cases. The same rounding rule
is used in every output; CSV is a presentation export, not a
lossless replacement for the source data. No currency formatting is guessed.

Text must be valid UTF-8 without NUL. This tightens earlier admission: repair or
reject malformed upstream text explicitly; do not depend on HTML replacement
characters or CSV preserving bytes that HTML cannot represent. Validation fails
before `RenderBars` returns markup or `WriteCSV` writes any bytes. Valid Unicode
and the 2048-byte (not rune) limit are unchanged. Dimension admission also rejects
integer-overflow-sized label columns without changing defaults or row-height policy.

The 200-row ceiling is a bounded input contract, not a recommended dashboard density.
With 189 present rows the default SVG is 6,420px tall, even though rendering is cheap.
Select a useful cohort or aggregate at the app boundary before rendering a dashboard;
keep the exact selection, coverage and export scope visible. Do not present a top-N
subset as a complete total or silently remove missing rows. A full-table review can
retain all selected rows with native keyboard scrolling. This recipe does not provide
pagination, virtualization or a large-data chart engine.
The [200-row assessment](../../../docs/assessments/2026-09-15-analytics-maximum-rows.md)
records scoped browser/native-key/export proof and diagnostic budgets, not consumer acceptance.

Negative values are valid semantic data and can be exported, but `RenderBars`
refuses them because the underlying primitive clamps negatives to zero. A
supplied chart maximum must not clip data. Empty data has an explicit message.
If the selected labels or full values/units exceed the conservative SVG column
budget, the renderer explicitly explains a table-only fallback. It does not turn
positive values into zero-width marks or silently truncate labels. Wider `W` and
`LabelW` options can recover a fitting plot; a wrapping app-owned composition is
another option. This deterministic fit guard is not actual font measurement.
Chart and table regions have names, keyboard scrolling and focus outlines. The
table retains a readable minimum width instead of crushing service names on phones.
Wrapped labels, values and meanings are top-aligned; long text remains fully available.

`WriteCSV(writer, dataset, now)` emits context key/value rows followed by the
`id,label,value,unit,state,missing_reason,lower,upper,role,role_label` table. Formula/control-led
text is prefixed with an apostrophe, including hostile metadata; typed numeric
cells stay numeric. The text transform is deliberate and not byte-preserving.
Use `encoding/csv.Reader.FieldsPerRecord = -1` while reading the context block.
CSV quoting is handled independently from spreadsheet formula neutralization.
`role` is the effective token identifier (empty input defaults to `accent`);
`role_label` is the app's exact legend label, or empty when not declared. Neither field
is inferred from a number or color. [ADR-0056](../../../docs/adr/0056-preserve-analytics-role-meaning.md)
records this **breaking** eight-to-ten-column migration; update exact-header parsers
and composed report widths with the kit pin. Existing context and first eight fields stay unchanged.

Go renderers are L3b: include `ui/analytics` and `ui/vizframe` in the consumer's
Tailwind scan. No JavaScript is required. Existing low-level renderers are
unchanged. Package tests prove contracts and output fixtures; browser, keyboard,
contrast and named-consumer adoption remain separate evidence.

The [Gallery reference journey](../../../docs/design/analytics-journey.md) demonstrates
period/category/comparison selection, service detail and rectangular CSV. It composes
this contract with low-level bars and remains experimental until its browser and
consumer evidence is earned.

The separate `/analytics/recipe` specimen calls `RenderBars` itself and exports
the same current-period dataset directly through `WriteCSV`. Its native form
selects period, service group and truth state, without paired-period semantics.
See the [scoped browser evidence](../../../docs/assessments/2026-09-15-analytics-renderer-specimen.md).
The [role-meaning follow-up](../../../docs/assessments/2026-09-15-analytics-role-meaning.md)
checks color-free interpretation, CSV migration, CSP-safe swatches and wrapped-row alignment.

Use [vizcheck](../vizcheck/README.md) in application tests for explicitly scoped
chart labels, annotations/legend, table cells and source text against an independent
fixture. It does not replace this semantic contract, source derivation or browser proof.
