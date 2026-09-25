# Optional Gallery analytics quality probe

`quality.mjs` records a bounded, synthetic browser task and local timing observations.
It is not an app dependency, browser automation service or accessibility certification.
The [assessment](../../2026-09-14-gallery-analytics-quality.md) owns the recorded
before/after result; the [journey](../../../design/analytics-journey.md) owns behavior.

## Run

Use the approved existing Playwright **1.63.0** installation and installed Google
Chrome on macOS. The probe checks package name/version and hashes the resolved entry
and package before importing; it does not establish full transitive tool identity.
It never installs or updates dependencies or opens the signed-in browser profile.

Build/run the Gallery from the repository root (CSS must match selected source):

```sh
go run ./apps/gallery/cmd/gallery -addr 127.0.0.1:0 -design-assets ''
```

Pass its printed, bare loopback origin, including its actual port:

```sh
APPKIT_PLAYWRIGHT_ENTRY=/absolute/approved/node_modules/playwright/index.mjs \
  node docs/assessments/fixtures/gallery-analytics/quality.mjs http://127.0.0.1:PORT
```

Actual 200% Chrome zoom, light/dark at 1440 outer pixels (720 CSS pixels):

```sh
node --test docs/assessments/fixtures/gallery-analytics/zoom-contract_test.mjs \
  docs/assessments/fixtures/catalyst-settings-adapter/zoom-capture_test.mjs
APPKIT_PLAYWRIGHT_ENTRY=/absolute/approved/node_modules/playwright/index.mjs \
  node docs/assessments/fixtures/gallery-analytics/quality.mjs http://127.0.0.1:PORT --zoom=200
```

Zoom mode uses the actual Appearance setting in fresh persistent profiles, then
disables application scripts. It preserves the same keyboard/CSV task and additionally
inspects detail geometry/contrast. Complete CDP DIP screenshots reuse the CRC/decoded-row
checker; the zoom helper allows only one DIP of independently rounded height metrics,
never a cropped image or visual-difference tolerance. Setup failures close the context.
Synthetic-only profiles remain in the temporary output after normal browser close.
[Recorded scope, failed initial capture and final 459-check result](../../2026-09-15-analytics-zoom-and-adoption.md).

The default entry points to the session's approved temporary installation; it may no
longer exist. Supply a reviewed existing entry rather than silently installing one.
Only selected analytics paths and `/static/app.css` are allowed, with GET/HEAD;
outside/write requests and WebSockets are refused and retained as failures. Each run
creates its own temporary output directory. A successful ordinary run retains
`receipt.json`, 20 PNGs and 14 downloaded CSVs; a successful zoom run retains 12 PNGs
and ten CSVs. A failed state may add a named failure capture. Browser contexts close on
completion. The Gallery process remains caller-owned; stop only the process you
launched when finished.

## What is checked

The separate exact-renderer probe exercises `/analytics/recipe`, not the custom
paired-period composition. Run against the same explicitly started Gallery:

```sh
node docs/assessments/fixtures/gallery-analytics/recipe.mjs http://127.0.0.1:PORT
```

It uses four fresh 390/1440 light/dark contexts with application scripts disabled;
checks glyph/mark overlap, SVG bounds, readable table columns, named scroll regions,
keyboard scrolling/native missing-state selection and the actual missing-value CSV.
Independent complete values are September's 4/1/0 fixture ledger, not inferred from
the output. Evidence and limitations: [renderer assessment](../../2026-09-15-analytics-renderer-specimen.md).

The opt-in role-meaning check additionally mounts a test-only long-label fallback:

```sh
APPKIT_ANALYTICS_ROLE_OUTPUT_PARENT=/absolute/existing/private-0700-directory \
  go test ./apps/gallery -run '^TestAnalyticsRolesBrowser$' -count=1 -v
```

The launcher uses the reviewed temporary Playwright installation named in its source;
it installs nothing and stops its owned server/browser afterward. `roles.mjs` verifies
per-row Meaning, missingness, native table scrolling, exact ten-column downloads and
top-aligned fully wrapped labels in four light/dark narrow/wide cases. Four private PNGs
and CSVs accompany each receipt. [Red/green evidence and migration](../../2026-09-15-analytics-role-meaning.md)
retain CSP, collector and alignment failures separately; this is not target certification.

The original `quality.mjs` journey checks:

- Six cases: 390/768/1440 CSS pixels × light/dark, reduced motion and fresh contexts.
- Key-only skip, native select type-ahead, disclosure, apply, detail, back and CSV.
  Actual focus and native focus outlines are checked; no scripted value selection.
- Selected native control/table-link bounds, outer overflow, no production scripts,
  sampled opaque text/background contrast and HTML ≤32 KiB.
- Actual CSV download: two exact August/API/partial/no-comparison records, rectangular
  unique 33-column headers including role/meaning, independent values and reporting-window expectations
  (ADR-0056; historical receipts used31 columns).
- Five fresh-context DOM-ready observations, local p95 ≤250ms, response sizes/hashes,
  page/console errors and non-download response status. CSV parsing/download errors fail.

Those six August/API/partial/no-comparison cases and their budgets remain unchanged.
The same invocation also runs four separately statused September 2026/API/previous-month
cases: complete, missing, stale and partial. Each case starts in a fresh context and uses
native keyboard type-ahead and submit; no script sets a control value or focus. It checks:

- exact overview narrative, two service rows, current/previous values, count differences,
  state labels and source freshness;
- paired charts for complete/stale, no chart for missing, and current-period-only charts
  for partial (the previous full month remains tabular but is not plotted);
- Gateway detail semantics and exact preservation through the Back link;
- overview and Gateway-detail downloads against a predeclared, ordered 33-column schema,
  with every cell independently expected: values, windows, missing reasons, freshness,
  completeness, source/provenance, evaluation time, coverage, uncertainty, aggregation,
  precision, role and `API services` meaning; and
- overview/detail captures plus the same containment, geometry, contrast, payload,
  browser-error, service-worker and cleanup checks in ordinary and actual-200%-zoom mode.

`receipt.json` initializes every comparison state and phase as `not-run`, then records
`executing`, `passed` or `failed` independently. One state failure does not suppress the
remaining states; a setup failure that precedes them leaves explicit `not-run` entries and
fails the aggregate assertion. These are synthetic Gallery checks, not consumer adoption.
The [retained four-state acquisition](../../2026-09-17-analytics-truth-state-journeys.md)
distinguishes the first selected-field CSV oracle from the later all33-cell validation.

Run reproducible in-process handler measurements separately:

```sh
go test ./apps/gallery -run '^$' -bench '^BenchmarkAnalyticsHTTPHandler$' \
  -benchmem -benchtime=200ms -count=5
```

Six handlers include selection validation, fixture construction, rendering/CSV and
response copying. Request construction, networking, browser work and first buffer
growth are excluded. Reported 5ms/op and 512KiB/op ceilings are local diagnostic
budgets, not timing-sensitive CI gates. The HTML budget and semantic canaries are tests.

## Public renderer's 200-row boundary

`scale.mjs` runs only through a test-owned loopback handler, never a production route.
Set an existing real absolute 0700 output parent and an explicitly reviewed Playwright
entry. Use an external `TMPDIR`/`GOCACHE` when internal disk space is constrained:

```sh
APPKIT_ANALYTICS_SCALE_OUTPUT_PARENT=/absolute/existing/private-0700-directory \
APPKIT_PLAYWRIGHT_ENTRY=/absolute/approved/node_modules/playwright/index.mjs \
  go test ./apps/gallery -run '^TestAnalyticsScaleBrowser$' -count=1 -v
go test ./appkit/ui/analytics -run '^$' \
  -bench 'BenchmarkMaximumRows(Render|CSV)$' -benchmem -benchtime=100x -count=1
node --test docs/assessments/fixtures/gallery-analytics/scale_test.mjs
```

Four fresh 390/1440 light/dark contexts exercise 200 ordered rows, eleven missing
sources, real zero, sixteen explicit intervals and API/Worker meanings. Native keys
reach the first/last table rows, scroll to Meaning, export all rows and switch to a
long-label table fallback. Independent expected values and pinned HTML/CSS/CSV agree;
201 rows are refused in Go before output. No scripted focus, scrolling or selection
is used to stand in for those keyboard actions.

The Go controller pins its executable and selected inputs before launching the copied
harness. Each private run retains the manifest, source/CSS/harness snapshots, persisted
report, exact CSV downloads and named viewport PNGs. Browser requests are limited to
the two fixed GET/HEAD test paths and Gallery CSS. Owned Chrome gets an aggregate
watchdog and bounded shutdown; parent-process absence is distinct from descendant
inventory. PNG dimensions, complete decoded rows and nonblank pixels are checked;
captures show task regions, not an entire 6,420px chart or fidelity certification.
Named targets must remain completely on screen across capture after native scrolling
settles; X-only bounds and nonblank pixels alone are insufficient. [Retained results](../../2026-09-15-analytics-maximum-rows.md)
include the earlier failed target capture and the corrected268-check run.

Predeclared budgets: 256KiB HTML, 128KiB CSS, 3,500 DOM nodes; render/export 25ms and
4MiB per operation; local DOM-ready p95 500ms. Timing remains diagnostic, not a flaky
default CI gate. The eight browser observations include four initial contexts and
four native GET navigations, not eight cold-cache trials. The 200-row API ceiling is
not a recommended dashboard density; apps own meaningful selection/aggregation.

## Limits and evidence discipline

Assert counts include repeated samples, not independent requirements. A five-sample
p95 does not establish a speedup. Normal mode has no real browser zoom; the explicit
zoom mode covers only 200% at 1440 outer pixels in the recorded Chrome version.
Neither mode proves native popup-arrow selection,
screen reader, touch device, LCP/INP, cold/remote network, production-scale dataset or
human task-time/error is measured. Contrast does not model overlapping elements or
antialiasing; unsupported sampled opacity/backgrounds fail rather than being skipped.
The root Gallery navigation/footer are outside the scoped target/contrast sample.
Keep red receipts, use the exact same final oracle for both candidates, and inspect
rendered captures before making a visual claim. Do not turn a green local receipt
into consumer adoption, whole-page conformance or target promotion.
