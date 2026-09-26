# Heatmap truth states (FATHOM-KIT-03) and catalog records (FATHOM-KIT-08)

## Selected task and acceptance

The work was requested for Fathom's project × layer coverage matrix (FATHOM-KIT-03),
which must show "not compiled" as absence, never as a faint low value. That request
lives in a draft Fathom design that is not in any published Fathom revision, so no
consumer revision is pinned here or in the quality inventory; the selection is
unrecorded until Fathom publishes it. Baseline `dc77e66`: `viz.Heatmap` draws NaN
as a dashed hole with no reason, legend or distinction between missing, unknown and
failed cells, and it is absent from the `docs/ui-components.md` viz table and the
capability catalog.

Acceptance: a cell is a value, a measured zero, missing, unknown or error; the three
absent states are never filled and never stretch the value range; each cell has a
reason title; a legend names the states present; existing callers get identical
bytes. L3a rules (I4, I5): no class attribute, colors only through role variables.
KIT-08 adds catalog records for shipped packages without changing any existing
entry or its ADR-0065 capability pin. No consumer, dependency or pin is changed.
Quality inventory: `candidate.fathom-heatmap-truth-states` (queued; no consumer
pinned, not integrated).

## Implementation

`HeatmapOptions.States [][]HeatmapCell` or `HeatmapOptions.TruthStates` selects the
truth-state path; otherwise the original code path runs. Missing cells are dashed
and `/`-hatched, unknown cells dotted, cross-hatched and marked `?` (plated on the
surface role in cells ≥ 28px), error cells carry a `danger` outline and cross, and a
measured zero keeps its ramp fill plus a `0` glyph. Hatches are line geometry, not
`<pattern>` references, so no document-unique IDs are needed and several charts can
share a page or be injected into a client framework. `HeatmapOptions.Validate`
reports contradictory input; rendering stays total. The Gallery specimen
(`/components/heatmap`) derives the chart and its table alternative from one fixture.

## Results

Package tests (`go test ./appkit/ui/viz`, `./apps/gallery`):

- Legacy goldens `heatmap_legacy` and `heatmap_default` were generated from the
  unmodified renderer before any change and still match byte for byte
  (sha256 `0b3a68d3…` and `c27cc935…`).
- Goldens for mixed, all-missing (declared and NaN-derived), hostile-text and
  large-cell fixtures; the all-missing grids contain no `fill-opacity`, no value
  range and no zero glyph; hostile labels, reasons and states stay escaped and the
  output parses as XML.
- A declared missing/unknown/error cell with a numeric entry draws no ramp fill and
  does not widen the range; ±Inf and unrecognized states render as unknown.
- `TestVizEmitsNoClassAttribute` and `TestVizEmitsNoLiteralColor` cover a
  truth-state specimen; `ink-muted` and `danger` marks clear 3:1 and the `ink`
  glyph clears 4.5:1 on `surface` and `surface-raised` in both default schemes.
- Gallery: every chart cell title equals its table cell text; the route keeps the
  Components section current and 404s unknown sub-paths.

Browser (Playwright MCP, Chromium, synthetic fixture, Gallery built from this
working tree, `appkit/ui/viz/heatmap.go` sha256 `cfba08de…`, `apps/gallery/heatmap.go`
`ab7b57d3…`, served on loopback port 8773, pointer and keyboard input):

| Viewport / theme | Expected | Observed |
|---|---|---|
| 1440×1000 light and dark | Five states distinguishable without color alone; legend lists value range, zero, missing reason, unknown, error | As expected; captures inspected. Legacy comparison shows unknown/error as indistinguishable holes |
| 390×844 light and dark | No page overflow; chart and table reachable | Document 375/375px; chart region 301px scrolls to 340px, table 341px scrolls to 457px |
| 390×844 dark, keyboard | Tab reaches each scroll region with visible focus; arrows scroll | Order: chart region, code, table region, all-missing region, code, legacy region; 2px focus-role outline; ArrowRight/End reach the scroll maximum (39px, 116px) |
| 390×844 dark, pointer | Unfilled absent cells still hit-test to their title | A point inside the first missing cell resolves to "atlas, concern: missing — not compiled" |

The first capture showed x labels overlapping at the default 26px cell and the
`?` glyph obscured by dense cross-hatching in large cells. The specimen now sets
`Cell: 56`; hatch spacing scales with cell size and the glyph gains a surface
plate. Label overlap remains a renderer limit, recorded as a catalog gap.

KIT-08 adds 13 records (`viz.heatmap`, `viz.trend-sparkline`, `viz.composition`,
`viz.scores`, `viz.stages`, `viz.outcome`, `viz.provenance-frame`,
`work.agent-trace`, `foundation.motion`, `ops.topology`, `content.explainer`,
`content.export-toolbar`, `content.details`). Maturity is `available` only where
package tests and a Gallery specimen both exist; `viz.heatmap` (new truth states),
`ops.topology`, `content.explainer` and `content.export-toolbar` (no specimen) are
`experimental`. Twenty authored selection cases cover each record, including
refusals: `heatmap` at L3b, `topology` for portable, `node link graph` (nothing
draws one; the topology tree has no edges) and `fleet health rollup` (the tree shows
app-supplied status and rolls nothing up). The catalog, MCP and Gallery tests pass.
Consumers name only apps that call the package in a published revision: Fathom's
observatory uses `viz.Sparkline`, `viz.Gauge`, `vizframe.MustRender` and
`hitl.DiffView`, so `viz.trend-sparkline`, `viz.scores`, `viz.provenance-frame` and
`work.agent-trace` list it; no app renders heatmap truth states yet.

## Review corrections

An independent review of `a80d967` found six defects. Five fixes were written
test-first, and each new test failed on `a80d967` before its fix; the inventory
correction is data only.

| Finding | Fix | Regression check |
|---|---|---|
| With `ShowValues`, `0.004` printed `0` and `-0.004` printed `-0`, so the cell matched a measured zero; titles, the legend range and `Validate` messages rounded the same way | Truth-state text keeps two significant digits when two decimals would round a nonzero value to `0` (`0.004`, `-0.004`); a negative-zero bound prints `0`. Legacy output is unchanged | `TestHeatmapTruth_SmallValuesNeverPrintAsZero`; Gallery `TestHeatmapTableTextMatchesChartForFractions` (table text equals chart title for ratios) |
| An all-zero grid has no span, so zeros and the legend swatch sat mid-ramp (`0.54`), identical below 14px to equal positive values | A zero with no span sits at the ramp floor (`0.12`); equal nonzero values stay mid-ramp | `TestHeatmapTruth_AllZeroGridSitsAtTheRampFloor` |
| The inventory pinned a Fathom selection revision nobody outside the authoring session could check | Consumer pin removed (`consumers: []`), with the unpublished selection stated in the owner, gap and next step | `cue vet` of the inventory; this section |
| The new Components card broke the INC-39 `components_reflow.mjs` exact-caption list | Caption added in card order; README says 18 | `TestComponentsReflowHarnessPinsEveryCard` ties the harness list to `vizCards()`; harness run below |
| `viz.heatmap` named Fathom as a consumer while four packages Fathom ships against listed none | Consumers corrected as above | `TestKit08RecordsNameOnlyActualConsumers` (also refuses qualified consumer names) |
| `ops.topology` declared "fleet health rollup" but rolls nothing up | Task phrase removed; summary says the badges are app-supplied | selection case `health rollup unavailable` |

A value still shares the ramp floor with a measured zero when it is the range
minimum; below 14px without `ShowValues` nothing else separates them. That limit is
stated in the `CellZero` doc, `docs/ui-components.md` and the catalog gaps. In
browser captures of `ShowValues` at the default 26px, `-0.004` runs into its
neighbor's label, as a long value such as `-12.25` already did; the catalog gap and
component docs now say value labels are not fitted to the cell.

Browser re-check (Chrome 153.0.8010.53 through Playwright 1.59.1, Gallery built from
this working tree on `127.0.0.1:8773` with `-theme-source cue/tokens/kit.cue`):

- `components_reflow.mjs` could not run as committed: its pinned Playwright
  `package.json` identity (1.63.0) is not installed on this machine. A scratch copy
  that skips only that package check and imports the same configuration ran the six
  width/scheme cases. With the 17-caption list: 45 checks, 18 failures (exact
  specimen, figure fit and SVG containment in every case), reproducing the review.
  With the 18-caption list: 45 checks, 0 failures; the heatmap figure and SVG stay
  inside the 286, 356 and 388px frames. Receipts sha256 `c7edcdf5…` (baseline) and
  `91af8c69…` (candidate); scratch probe `531745dc…`. Screenshots were not reviewed.
- `/components/heatmap` (light, dark) and `/components` are byte-identical to the
  `a80d967` build: the Gallery fixture has integer values and a nonzero span, so the
  fixes do not change it. At 390×844 dark, Tab order is unchanged (chart region, code,
  table region, all-missing region, code, legacy region, 2px focus outline);
  ArrowRight scrolls the chart and table regions to their maximum (24px and 101px in
  a scrollbar-free headless viewport); the missing cell still hit-tests to its title.
  No page overflow at 390 or 1440px in either theme; the only console error was the
  browser's automatic `/favicon.ico` 404.
- Specimens rendered outside the Gallery with the Gallery's CSS at 390px, light and
  dark: a 12px all-zero grid draws floor fills (`0.12`) against mid-ramp (`0.54`)
  equal fives, and the `ShowValues` row labels `0`, `0.004`, `-0.004`, `1`.
  Captures inspected; kept outside the repository.

## Limits

Evidence is scoped: synthetic data, one Chromium lane, interactive MCP steps and
scratch scripts rather than a retained replay harness, and captures kept outside
the repository.
Cell titles are pointer tooltips inside `role="img"`; no screen-reader, zoom,
forced-colors, print or performance measurement was made, and Fathom has not yet
integrated this revision. `internal/adoption` `TestCheckSocket` fails only when the
test temp path exceeds the macOS socket-path limit and passes with a short
`GOTMPDIR`; it is unrelated to this change.
