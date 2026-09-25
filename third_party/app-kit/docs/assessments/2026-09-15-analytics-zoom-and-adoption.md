# Analytics: actual browser zoom and usable adoption anchors

G1/G3 follow-up to the [analytics journey](../design/analytics-journey.md).
No production chart, metric, layout, CSS, dependency, consumer or runtime change.
The shared catalog guidance is corrected; browser acceptance gains a scoped zoom lane.

## Shared adoption repair

`analytics.comparison` recommended `RenderBars` and `WriteCSV`, but its primary
source only located the dataset/contract types. Exact get now also names
`appkit/ui/analytics/analytics.go#Dataset.Validate`, `render.go#RenderBars` and
`csv.go#WriteCSV` using complete repository-relative paths in the shared usage text.
It identifies the README, `get_kit_css_setup` and both CSS source scans relative
to the selected kit source root. No second metadata schema or duplicate MCP guide.

`TestAnalyticsUsageMapsPublicAPIsToExactFiles` fails on the missing old anchors and
guidance, then passes after repair. Go AST checks prove declarations exist in the
named files. The primary source, experimental maturity, empty consumer history,
Gallery-versus-RenderBars distinction and all evidence gaps remain unchanged.
The final built Gallery's JSON and HTML exact-get routes both return 200 and all six
API/scan/tool anchors. The corresponding catalog/MCP/Gallery race suites pass.

## Browser task and result

The existing [quality runner](fixtures/gallery-analytics/README.md) accepts an explicit
`--zoom=200` mode. Each lane uses a fresh private Chrome profile, the browser's actual
Appearance setting, then an `about:blank` handoff before application restrictions.
Application scripts are disabled. Only selected loopback GET/HEAD paths are admitted;
WebSockets and outside/write requests fail. No signed-in profile or portal is opened.

Two light/dark tasks retain the original keyboard path: skip, native select type-ahead,
partial August/API/no-comparison selection, Apply, Gateway detail, Back and actual CSV
download. Independent expectations remain Gateway 4/Search 2, two 31-column records,
explicit partial coverage and the original reporting window. Both downloads are 2,712 B.
Normal-size mode retains its six 390/768/1440 light/dark cases and original 1,002 checks.

Observed Chrome 152.0.7977.84, Playwright 1.63.0 and Node 26.3.0 on macOS arm64.
At 200%, outer width 1440 maps to 720 CSS pixels, DPR 2 and visualViewport.scale 1.
Overview, partial and detail pass scoped target/contrast/overflow checks. Four complete
PNG captures pass exact dimensions, CRC and decoded-row checks. The light overview
and dark partial detail were visually inspected: controls, labels, table, source and
last-page content remain readable. No source-fidelity or full accessibility claim.

| Retained temporary run | Result | Receipt SHA256 |
|---|---|---|
| `buyxRL` | Initial strict capture geometry refuses both lanes: 177 passes / 2 failures | `3bc2bab43d624ca2bf4bb676938a5aeec9e2b5b8d36dc8f62e9429a9af5fe640` |
| `oR40Dd` | Geometry correction: 361 passes; before extra detail inspection/setup cleanup | `1dd4a7dbf4dba3b395bcb602a0cd94f071c217f16a385a832d1b6da360e2c342` |
| `ypLWcJ` | Final zoom runner: 459 passes / 0 failures | `26137183f7361e8bbad454e89c2d8b502adbc1bb09976fa695907e35a100de42` |
| `ojbzAI` | Final normal runner: 1,002 passes / 0 failures | `aa1f06a6c03de65a0a5219f34973d66a54e7f83af67b7b8c72097985a28a3a4b` |

Directories are system-temporary `appkit-analytics-quality-<suffix>/`; receipts,
screenshots and CSVs remain local. Seven synthetic-only profiles per zoom run are
retained, closed, with no running browser process observed. They are not user profiles.
The five-sample local DOM-ready p95 is 10.4 ms zoom / 46.1 ms normal, below the existing 250 ms
ceiling. Concurrent local runs and tiny samples establish neither speedup nor regression.

## Oracle repairs and provenance

The first collector assumed exact 2x integer heights. A bounded diagnostic observed
CDP full DIP height 4051 and independently rounded CSS height 2025. `zoomCaptureClip`
permits at most one DIP of coordinate-field rounding but captures the exact full DIP
rectangle: 1440 × 4051 overview and 1440 × 3907 detail. This is not an image-difference
tolerance; a missing PNG row still fails. The original failed receipt stays intact.

Review found setup errors after persistent-context creation could strand Chrome.
`prepareContext` now closes on every setup failure and preserves both errors if close
also fails. Five new tests cover geometry, missing/shifted/non-zoom/unbounded metrics,
normal ownership, six setup failure stages and error preservation; all 11 tests pass
with the existing six PNG/capture tests. Independent review cleared both repairs.

Final runner SHA256 `1863c07df3a6cc5e710f20d6a5cf92fae84e1f1c0bbeb6c01f07cd0d79c4163b`;
zoom helper `4aaa24c657fce654f1bef84f32adcfa4e57ab020cf2289c1c00db02e6806eaf3`.
Browser-tested Gallery binary: `eaf413ffb70ab567130d16b7deb36626ca93d8a63dfabf93abd2695f48ab8e94`.
Later catalog-only rebuild: `81d62b1b39b362d01827ddcf298aa10cc5d618cac029c9271b2ce4dbf9dd7177`;
its metadata changed, not the analytics renderer/CSS tested above. Exact-get JSON/HTML
body hashes respectively `dd1e1e6da4947ee8cd9df56992b0c61fa84d8d86dfaabbc247109e7f14f50d0f`
and `0d4ab5b052bbc35909dc296827d3c4ee85eee3119b1f7ad1f98644c854663d09`.
Receipts bind runner/helpers, response bodies and the reviewed Playwright entry/package,
not a full transitive toolchain or permanent artifact archive.

## Limits and intent

I8/I11/I12 and ADR-0020/0025 remain unchanged; INC-28 links the capture follow-up.
Native popup-arrow behavior, other zoom/browser/device combinations, screen readers,
human task-time/error, production-scale performance and two real consumer acceptance
journeys remain open. No goal, conversion target or recipe is promoted by these checks.
Four selected Go race packages and vet pass. No commit, release, global configuration,
real data, Fathom mutation or deferred Windows/CA/GGB work occurs in this slice.
