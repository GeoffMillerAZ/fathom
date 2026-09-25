# Signed single-dataset analytics: bounded incubation

Base revision: `d285ebf`. [ADR-0074](../adr/0074-incubate-signed-analytics-comparisons.md)
admits one opt-in public renderer and Gallery specimen, not a multi-series engine.
No consumer, installed application, dependency or licensed source is changed.

## Delivered boundary

`analytics.RenderDivergingBars` renders rounded signed observations on a symmetric
scale with centered zero, direct values and exact domain endpoints. Missing rows
remain in the table/CSV; measured zero has a mark. Roles and meanings stay app-owned.
Explicit maxima cannot clip. Intrinsic SVG size supports native scrolling; long
labels, colliding scale text and subpixel nonzero marks produce an explained table
fallback without truncation or exaggerated geometry. Existing `RenderBars` and
`viz.Bars` source/behavior are unchanged. One dataset means one reporting window.

`/analytics/signed` and `/analytics/signed/export.csv` share six fixed synthetic rows:
`-2,-1,0,1,2,missing`, points, partial coverage and explicitly unknown source age.
The fixed specimen admits appearance changes, plus the exact inert default filter
values emitted by the shared frame; changed filters and unknown keys fail. Apps
retain real selection, calculations, authorization and paired-series/detail seams.

The existing `analytics.comparison` capability exposes both renderers. Its selected
v2 metadata pin changes and needs review; do not automatically repin consumers.
Its maturity remains experimental. Readiness keeps all existing partial gates.

## Verification and retained failures

Private packet: `/Volumes/m2ext/appkit-evidence/appkit-signed-analytics.QXiyaW`.
The tracked [browser driver](fixtures/signed-analytics/browser.mjs) uses the existing
synthetic `TestAnalyticsAdoptionBrowserFixture`, installed Playwright/Chrome, a fresh
browser, exact local routes and authenticated fixture shutdown. No signed-in profile.

- Independent tests cover signed proportions/direction, zero/missing omission,
  rounding/clipping, exact small domains, intrinsic dimensions, extreme scale
  fallback, escaping, query refusal and table/CSV truth. Old negative refusal stays.
- Race suites pass for analytics, viz, Gallery, Receipts, catalog, qualityplan,
  adoption and MCP. Two later unrelated volume-read failures are retained; the
  unchanged Gallery/MCP suites pass on bounded retry. Vet and catalog CUE pass.
- Initial absent-API test and executed tests are retained. An initial test oracle
  incorrectly expected fixed decimal zeros; existing `number`/`WriteCSV` trim them.
  Only those expectations changed, retaining the old tests/log and numeric values.
- The catalog regression caught dropped integration wording; production guidance
  restored qualified API names and independent-fixture/static-only limits. No test
  changed to accept the omission.
- Browser-01 retains all four screenshots/CSV and original harness. Its contrast
  calculation incorrectly included alpha in luminance; that harness fault is not
  contrast evidence. Browser acquisition is separate from Go/source assertions.

After that isolated harness repair, **browser-02 remains failed solely on the
unchanged20KiB HTML target in all four cases**. Geometry/2:1 magnitudes, glyph bounds,
body containment, 2px focus outlines, actual narrow-pane ArrowRight scrolling,
native Enter downloads and exact context/data CSV checks pass. Text/axis contrast
minima are7.275 light/6.808 dark; mark minima6.130/6.202. HTML+CSS reaches80,513B
(78.6KiB), within128KiB. Browser close reported no error; authenticated shutdown
returned204, the fixture exited PASS and its loopback port refused connections.
These observations do not establish OS network containment or full accessibility.

Final synthetic binary SHA-256:
`a7fc480ead129992cd4201dcacc03a0ee58075c363c6b4ca04be0ff78c5f997e`.
CSS SHA-256: `60db385940a53cca76b8a0ef717be304cb2342bbb1eb23e63f59965a42a29987`.
`browser-02-inputs.json` records the exact harness, producer and tool identities;
`browser-02/report.json` retains false per-case and overall acceptance. The four
1,317-byte CSV downloads are identical; both acquisitions remain distinct.
Final report SHA-256: `87278537d7538212bfa14d1cd9eb4939d8140a69759c8ff06c3b061a8771acf8`.
Frozen inputs SHA-256: `ee7137ee9743b5be3a9bb22d91eb2f81248d104e54ade6ccf664c29e0676c0bc`.

## Catalog discovery follow-up (2026-09-22)

The existing70-case selection evidence remains preserved. Three additive cases bring
the shared source-reviewed corpus to73: an unfiltered exact-text `negative value
chart` query selects `analytics.comparison` and forbids legacy `analytics.bars`; a
Go-web experimental `signed diverging bars` query selects the same recipe and forbids
the legacy primitive; and the corresponding portable recipe query explicitly refuses
both. `TestTaskSelectionEvaluation` and
`TestCatalogTaskCorpusAcrossMCPAndGallery` pass, covering the shared catalog evaluator
and MCP/Gallery JSON/HTML ordering. Inventory regressions also pin
`analytics.RenderDivergingBars`, `/analytics/signed`, maximum-domain guidance and the
legacy negative-value refusal. These are exact authored lexical cases, not evidence of
semantic search, paraphrase handling or generalized language understanding. Production
catalog metadata and ranking behavior are unchanged.

## Open acceptance

The predeclared HTML target is **under 20 KiB**, with HTML+CSS under128KiB, for
wide1280/narrow390 × light/dark. The initial full Gallery document measured25,346B light
and25,305B dark: that retained HTML acquisition fails, unchanged. It is not a renderer-only byte
measurement or a demonstrated user-visible performance defect. Investigate the
shared-page overhead before a named adopter needs a smaller delivery budget; no
speculative caching or runtime dependency is admitted to hide this failure.

The same retained light HTML was attributed on 2026-09-22: shell header 14,131B
(desktop navigation 5,199B, mobile disclosure 7,249B, theme controls 1,155B), main
10,471B, footer 243B and remaining head/wrappers 501B. Whitespace-only compaction
would save just 354B. The signed frame retains meaningful SVG/table/context content;
removing accessibility, provenance or native navigation is not an accepted optimization.
The retained `payload-analysis/main.go` and `signed-light.html` bind this attribution
to HTML SHA-256 `791d5e4b901337d28f14d38aa3c19608e9c8d688e3f093aea2fd813ba0ccd48d`.
This explains the byte target, not observed task latency. Shared navigation/style
deduplication was subsequently admitted as owner-funded maintenance, without changing
the target or this failed receipt. The separate [Gallery navigation repair](2026-09-22-gallery-nav-payload.md)
reduces the same full documents to20,382B/20,341B and passes all four unchanged signed
browser cases. Its added CSS, narrow payload margin and paired shell checks are recorded
there; this closes that scoped current-payload failure, not the remaining acceptance.

No human A/B, actual zoom, assistive-technology, broad theme, production source or
two-consumer promotion is claimed. Receipts supplies the need witness but retains
its app-owned paired-series renderer. Full G3 and converted-core-v1 remain open.

## Reproduce

Build CSS with `task gallery:css`, then `go test -c -o /private/owned/gallery.test
./apps/gallery`. From `apps/gallery`, run that binary with
`APPKIT_ANALYTICS_BROWSER=1` and
`-test.run '^TestAnalyticsAdoptionBrowserFixture$' -test.v`.
Pass its printed random loopback origin, installed Playwright entry, Chrome path
and a new private output directory to the tracked driver (argument order in file).
Keep the180s fixture watchdog and explicit shutdown; retain each failed acquisition
before changing any harness, input, budget or source. Record exact binary/CSS hashes.
