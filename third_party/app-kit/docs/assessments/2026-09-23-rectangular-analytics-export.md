# Rectangular single-dataset analytics export

## Prospective packet

Owner-funded R4/core analytics maintenance; baseline `9f7d95f`.
Gallery and Receipts already reshape sectioned analytics CSV for their domain
reports. A simple single-dataset spreadsheet import should not need a private parser
or a second implementation of formatting, missingness and text safety.

Add experimental `analytics.WriteTableCSV(io.Writer, Dataset, time.Time) error` and
one opt-in Gallery recipe download. Preserve `WriteCSV`, existing download paths,
paired/domain exports, renderer output and source semantics. No spreadsheet engine,
new dependency, consumer mutation, installed binary or release is included.
ADR-0037 keeps multi-series composition private; this is a second bounded writer,
not promotion of that parser or an inferred external-consumer requirement.
I7/I8/I11/I15/I17 and ADR-0056/0069/0072 apply. Full quality gates stay open.

## Acceptance fixed before implementation

- New format: one 27-column header, then one row per selected observation. The
  existing 17 context keys gain `context_` prefixes and precede the existing 10 data
  keys. Every value matches the existing writer's decoded presentation cells;
  formula shielding occurs exactly once. Numeric negatives remain numeric.
- Validate the complete dataset before writing any bytes; preserve missing versus
  zero, rounding, bounds, role meaning, source/window/freshness/coverage and row order.
  Retain the 200-row bound; no mutation, implicit clock or fabricated empty data row.
  Empty valid input emits only headers, explicitly not empty-selection provenance.
- Refactor only shared cell construction. Existing exports must remain byte-identical
  across the frozen Gallery selection matrix and focused Receipts regression tests.
- `/analytics/recipe/table.csv` supports native GET/HEAD and the existing canonical
  selection/theme context, strict response policy, validation and download behavior.
  Expose a secondary native link with a clear format explanation; no new CSS/JS.
- Independent literal field/value tests, invalid-input/no-output and writer-error
  tests, protocol/HTTP checks, and scoped race suites. Retain baseline endpoint
  unavailability separately from any new-function compilation diagnostic.
- Browser task: 1440/light and 390/dark, complete and missing selections, native
  keyboard download, exact file bytes, filter context and both export choices.
  Inspect captures; do not infer spreadsheet-application or full accessibility proof.
- Freeze source, harness and executable identities before acquisition. Bound this
  packet to 50 minutes from 09:48 UTC, at most one separately retained browser harness
  correction. Preserve failures and stop acquisition at the cap without relaxing it.

Evidence root: `/Volumes/m2ext/appkit-evidence/appkit-rectangular-csv.4hVJLr3X`.

## Results

The baseline and candidate are independent offline Go 1.26.3 source builds, with
source/archive/executable identities retained before execution. The initial baseline
fixture asked for `/assets/app.css`, which is not the Gallery stylesheet. Its 404
remains diagnostic evidence. The corrected `baseline-reviewed` uses `/static/app.css`;
the entire corrected input is frozen before the separate build. No browser used
the incorrect asset. The new endpoint returns 404 in all 48 baseline selections.

- First focused run: 32 passing test/subtest events, no source drift. There was no
  pre-implementation new-API test run; no compile-failure or red-test claim is made.
- Independent literal schema/context/value checks cover formula protection exactly
  once, negative rounding, bounds/roles, missing versus zero, explicit clock, source,
  partial coverage, unchanged inputs, writer errors, empty and 200/201-row boundaries.
  The old sectioned writer also has an independent literal byte fixture.
- `compare-candidate`: all 48 old CSV bodies and headers are byte-identical; all 48
  new table downloads return 200; all 48 recipe pages retain both choices and stay
  below 32 KiB. CSS bytes and headers are identical. This is not whole-page visual
  identity: only the declared export controls/help text change in recipe HTML.
- `candidate-browser`: 64 checks pass in four complete/missing, 1440/light and
  390/dark cells. Native Tab/Enter downloads both exact files, retains filters and
  the report, with visible focus and targets at least 44px high. Completed HTML/CSS
  and independent visible values are checked, with no application scripts or
  unexpected requests. Chrome exits/drains cleanly; no browser correction was needed.
  Node 26.3.0, Playwright 1.62.1 and Chrome 153.0.8010.53 are identity-bound. Root
  review inspected all four full-page captures: export explanation remains readable,
  no page overflow, and existing native chart/table scrolling remains necessary.
- Six-package regression/race run: 1,573 events pass, including Gallery/Receipts
  existing exports, legacy writer bytes, catalog discovery and MCP protocols. The
  frozen source remains unchanged during execution. Independent scoped review found
  no correctness/security/compatibility defects; documentation spacing was corrected.

The tested handler bundle SHA-256 is
`285916cebceb3a96ba1b0a8fa8a539644cc167b2c357c7f9a84e3a8c1723d905`.
This is a Gallery/kit integration, not an external consumer migration, spreadsheet
application test, raw-data export, full accessibility review or core/G3 promotion.
The private domain composers remain unchanged. [ADR-0077](../adr/0077-incubate-rectangular-analytics-export.md)
records the experimental API and empty-selection tradeoff. Review changed capability
metadata pins when adopting; no installed MCP server or consumer pin is refreshed here.
