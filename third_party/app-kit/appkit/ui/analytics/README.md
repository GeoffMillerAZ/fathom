# Analytics composition

Experimental `WriteTableCSV(writer, dataset, now)` exports one rectangular 27-column
table for spreadsheet import: the 17 existing context fields prefixed `context_`,
followed by the 10 value columns. Every observation carries its source/window/context;
values, order, rounding and text protection match `WriteCSV` exactly. Validation
precedes output. An empty selection emits only headers: use `WriteCSV` to retain
empty-selection context. No metadata observation is invented. Context repetition
increases payload; this is not a lossless raw export or a multi-series report builder.
Gallery `/analytics/recipe/table.csv` demonstrates the new format; the old download
is unchanged. [ADR-0077](../../../docs/adr/0077-incubate-rectangular-analytics-export.md)
owns the experimental boundary; broader adoption and quality remain unproven.

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
integer-overflow-sized label columns; row-height behavior is described below.

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
refuses them because the underlying primitive clamps negatives to zero. Its error
now names the existing `RenderDivergingBars` alternative; the refusal itself is
unchanged. A supplied chart maximum must not clip data. Empty data has an explicit message.

For signed observations, opt into experimental `RenderDivergingBars(dataset,
viz.BarsOptions{}, now)`. Zero is centered in a symmetric domain; negative marks
extend left and positive marks right, without assigning a good/bad role. `Max > 0`
is the absolute extent and must cover every rounded magnitude. Automatic extent
uses the largest rounded magnitude (one unit for an all-zero selection). Zero gets
an explicit mark; missing gets no mark. Domain endpoints, direct values, meanings,
truth context and the existing table/CSV remain visible. Both plot halves must fit
at least 48px; labels/value reservation can require a wider chart or table fallback.
Exact domain labels must also fit. A nonzero mark under one logical pixel triggers
a separate scale-resolution explanation, without exaggerating its magnitude or
claiming that label space is the problem. A narrower selection or an app-owned
alternative representation can help; every selected value remains in the table/CSV.
Intrinsic SVG size preserves native scrolling. The [guidance regression pair](../../../docs/assessments/2026-09-23-adopter-guidance.md)
checks both signs and unchanged label-space/ordinary-chart controls.
Open `/analytics/signed` for the fixed synthetic specimen and matching CSV. This is
one dataset, not paired series or a calculation engine; old renderers are unchanged.
[ADR-0074](../../../docs/adr/0074-incubate-signed-analytics-comparisons.md) owns the boundary.

If the selected labels or full values/units exceed the conservative SVG column
budget, the renderer explicitly explains a table-only fallback. It does not turn
positive values into zero-width marks or silently truncate labels. Wider `W` and
`LabelW` options can recover a fitting plot; a wrapping app-owned composition is
another option. This deterministic fit guard is not actual font measurement.
Both high-level renderers also use an explained, non-error table fallback for
positive `RowH < 16`: tiny rows overlap labels or clip the first/last label even
with a large `Gap`. Choose `RowH >= 16`, keep the default 28, or use an app-owned
layout with verified text geometry. Nonpositive `RowH` still selects the default.
All selected values, bounds, meanings and exports remain; an omitted chart has no
chart legend. Empty/all-missing selections retain their existing empty-state copy.
The 16px minimum reserves space for the current 11px labels, not arbitrary fonts;
verify the actual consumer font and zoom. Low-level `viz.Bars` is unchanged.
Chart and table regions have names, keyboard scrolling and focus outlines. The
table retains a readable minimum width instead of crushing service names on phones.
Wrapped labels, values and meanings are top-aligned; long text remains fully available.
Missing-reason cells reserve 12rem of width, avoiding one-word columns on phones;
the existing scroll region keeps the complete explanation reachable. This is an
intentional missing-row markup change, linked or unlinked. Rebuild scanned CSS and
review snapshots and selected capability pins. All-present HTML, values and CSV
are unchanged by this repair. [ADR-0076](../../../docs/adr/0076-preserve-readable-missing-state-columns.md)
records the measured tradeoff; verify actual app fonts, zoom and translations.

### App-owned detail links

Set optional `Row.DetailHref` to a root-relative destination, for example
`/services/gateway?period=2026-09#observations`. Both renderers link that row's
table label, including zero/missing rows and table-only fallback. Empty keeps
the plain label; the original link-only delta preserved unlinked markup, independently
of the later ADR-0076 missing-row layout repair. Links are native, underlined, keyboard
focusable and at least 44px high; long labels wrap without hiding their text.
The app owns route existence, authorization, selection/return context and recovery.

URLs are bounded to 2048 bytes of valid UTF-8. External/protocol-relative URLs,
literal ASCII whitespace, Unicode controls or backslashes, malformed escapes and controls or
backslashes after one decoding pass are refused. A decoded path starting `//`
is refused too. Encoded spaces, query parameters, fragments and Unicode are allowed.
No normalization, fetching, authorization or recursive decoding is performed.
Invalid supplied links fail validation before either renderer or CSV writes output;
accepted links never enter or change CSV. The destination cannot strengthen data truth.

**Compatibility:** adding the exported field breaks positional `Row{...}` literals;
migrate them to named fields, such as `Row{ID: "gateway", Label: "Gateway", Value: n}`.
Keyed callers may omit the field. Rebuild L3b CSS, review selected
`analytics.comparison` v2 metadata pins, and verify the app's actual detail/back task.
[ADR-0075](../../../docs/adr/0075-incubate-app-owned-analytics-detail-links.md)
admits this experiment, not permanent promotion or consumer adoption.

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
