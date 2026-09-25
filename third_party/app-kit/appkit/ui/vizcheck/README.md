# Static chart composition checks

`vizcheck.CheckHTML(markup, spec)` is an opt-in Go test helper, not a renderer or a
chart-certification service. It adapts RAC's valuable printed-value/legend/current
annotation checks to actual static output, with explicit scope and coverage.
[ADR-0025](../../../docs/adr/0025-static-chart-composition-evidence.md) owns the boundary.

## Use in a consumer test

Render a synthetic, fixed-selection page with the real application renderer. Supply
an independently authored `Spec` and fail on **either** an error or any finding:

```go
report, err := vizcheck.CheckHTML(pageHTML, vizcheck.Spec{
    RootID: "report",
    Charts: []vizcheck.Chart{{
        ID: "counts", Name: "Failed operations by month",
        Labels: []string{"September", "5", "August", "9"},
    }},
    Table: vizcheck.Table{
        ID: "values", Caption: "Failed operations by month",
        Rows: [][]string{{"Month", "Count"}, {"September", "5"}, {"August", "9"}},
    },
    Source: vizcheck.Text{ID: "source", Value: "Synthetic monthly ledger."},
})
if err != nil || len(report.Findings) != 0 {
    t.Fatalf("chart composition: error=%v findings=%+v", err, report.Findings)
}
```

IDs identify actual elements. A chart ID can be the SVG or a wrapper containing
exactly one SVG. A wrapper also scopes its `Annotations`, such as a sparkline's
printed last value; a number elsewhere on the page cannot satisfy that check.
`Legend` binds optional exact-ID text inside the root. Do not require a separate
legend when direct labels already explain the encoding. These fields assert the
chosen composition; they do not infer the meaning of colors or charts.

The table must have one direct caption, a first row of `th scope="col"` cells and
the complete expected rectangular matrix. Extra/missing/reordered rows or values
fail. Nested tables and any `rowspan`/`colspan` attribute are unsupported, not skipped.
Source text proves statement presence, not reference relevance or calculation truth.

Every SVG under `RootID` must have one chart owner or an explicit `Decoration`
with the SVG's own ID and a reason. `aria-hidden` does not excuse inventory: a
meaningful chart can deliberately rely on an adjacent table for assistive access.
For a missing/empty/table-only state, supply zero charts and a real table/source;
unexpected SVGs still fail. Assert expected case/chart totals in your test so a
fixture/route disappearing cannot turn the suite vacuous.

## Exact scope of the result

- Labels compare the full ordered SVG `<text>` sequence. Nested `<tspan>` text is
  aggregated within its parent label. SVG title/description/definitions and script,
  template or HTML-hidden content cannot substitute for a printed value.
- Only HTML ASCII whitespace is collapsed. Unicode spaces, signs and punctuation
  remain distinct. This is DOM text content, not browser `innerText` or layout.
- Name checks compare `aria-label` or root-contained resolving `aria-labelledby`
  declarations. They do not implement the accessible-name algorithm or certify ARIA.
- Caption/legend/source content may require a native disclosure or use CSS such as
  `sr-only`. CSS, closed-disclosure interaction and computed visibility are not
  evaluated. Neither stylesheets nor JavaScript are executed or fetched.
- Known HTML `hidden` ancestors fail for required evidence. CSS can still hide
  correct text; wrong SVG geometry can still accompany correct labels. Browser
  checks and independently expected geometry remain separate work.
- A report counts inspected declarations, not verified apps. Scope is exactly the
  selected root; outside charts, routes and runtime-injected charts are not covered.

Inputs are bounded to 1 MiB UTF-8 HTML, 50,000 nodes, depth 128, 4,096 HTML cells,
64 chart/decorative declarations and 256 KiB of expected text/IDs. Expected table
cells are also bounded to 4,096. At most 128 findings plus an explicit truncation
failure are returned. Duplicate IDs/attributes, ambiguous specs and bounds return
errors. The HTML parser repairs malformed markup; parsing is not HTML validation.

## Keep four independent evidence layers

1. Validate units, windows, freshness, completeness and missingness with the app's
   contract or `analytics.Dataset.Validate` where that recipe fits.
2. Test the app's aggregation/derivation against literal source fixtures. A resolving
   reference or a present source sentence cannot establish that a number is supported.
3. Compare chart/table/export against those independently established values, run
   this helper and maintain positive plus planted-negative cases. Never generate
   the expected strings from the HTML under test. Review changes to the oracle.
4. Exercise the rendered task: geometry/scales, keyboard/focus, disclosure, zoom,
   narrow layouts, contrast, long labels and realistic update/performance behavior.

Run `go test ./appkit/ui/vizcheck ./apps/gallery`
for the reusable failure canaries and Gallery integration. The full `task check`
runs these tests too. No test here establishes real consumer adoption or browser
quality. Low-level chart APIs, CSS scanning and runtime profiles remain unchanged.
