# The advertised analytics renderer now has a working Gallery specimen

G1/G3 follow-up under I4/I5/I7/I8/I11/I12, ADR-0020 and ADR-0025.
This is first-party runtime integration and scoped fixture proof, not a consumer
promotion, converted component, new metric backend or completed G3 acceptance.

## What changed

Previously every resolved `analytics.RenderBars` caller was a test. The Gallery's
successful `/analytics` journey used a separate low-level bar composition. Its
browser evidence therefore did not cover the renderer recommended to consumers.

`/analytics/recipe` now calls the public renderer over the selected current-period
dataset, with native period/category/state controls and canonical `compare=none`.
Its `/analytics/recipe/export.csv` route calls `analytics.WriteCSV` directly. This
context/value CSV is intentionally different from the comparison journey's rectangular
31-column adapter. Theme and selection survive navigation; no script or backend is
added. The existing comparison/detail/export workflow remains intact and links here.
The shared CUE catalog exposes this distinction to Gallery and MCP consumers.

Actual Chrome review found the fixed 56px value column overlapped unit-bearing text,
chart/table scroll regions lacked explicit keyboard access, and mobile table labels
were crushed into fragments. The repairs are reusable, not specimen-only CSS:

- `viz.Bars` reserves the full printed number and suffix using a conservative
  11px-per-display-cell budget, including grouped large numbers and Unicode units.
  Its short-value golden remains unchanged. It remains a permissive low-level SVG.
- `analytics.RenderBars` gives chart/table scroll regions distinct names, keyboard
  focus and focus outlines. Tables retain a 32rem minimum and 8rem row-label column.
- Otherwise valid labels or value columns that cannot fit the declared plot receive
  an explicit table-only explanation, not clipped labels or positive zero-width marks.
  Full values/source/CSV remain available; larger caller-selected dimensions can
  restore the plot. Invalid geometry, clipped scale and negative-bar refusals remain.
- Table-only output does not show a color legend with no corresponding marks.
  Independent review caught this before final verification.

This does not supply automatic wrapping SVG labels or a responsive chart engine.
The conservative fit guard is not a font measurement. Native horizontal scrolling
is deliberate at narrow widths; blanket typography or accessibility is not claimed.
Consumers must rebuild CSS scanning `appkit/ui/analytics/*.go` and
`appkit/ui/vizframe/*.go` to receive the new width/focus utilities.

## Browser evidence and oracle corrections

[recipe.mjs](fixtures/gallery-analytics/recipe.mjs) uses the approved existing
Playwright 1.63.0 installation and Chrome 152.0.7977.84 on macOS arm64, without a
signed-in profile. Four fresh contexts cover 390/1440 CSS pixels and light/dark,
100% zoom, reduced motion, and disabled application scripts. Only named loopback
GET/HEAD paths are admitted; outside/write/socket requests and browser errors fail.

The task checks exact complete values, skips to main, keyboard-scrolls the wide
chart, uses native type-ahead to select missing reports, submits the form and
downloads the real missing-value CSV. SVG glyph bounds and glyph/mark overlap are
measured independently of screenshot similarity. Table-column widths are measured.
The 32KiB HTML budget remains. The final phone dark and desktop light captures were
visually inspected; offscreen columns remain scrollable, not squeezed or removed.

The initial collector incorrectly expected September counts 3/2/0. The existing
`analyticsServices` ledger and `TestAnalyticsFixtureExactValuesWindowsAndNarrative` expectations
instead specify 4/1/0. That oracle error was corrected before changing the renderer,
and the old binary was rerun. No production fixture or calculation was changed.
Visual review later added a 128px minimum row-header-width check to catch the table
fragmentation. The final strengthened oracle was run against both binaries.

| Run under system temp | Result | Receipt SHA256 |
|---|---|---|
| `appkit-analytics-recipe-APPlzw` | Initial collector: 118 passes / 24 failures, including four wrong-expectation failures | `9e3c9f9d8f8758886a2f918051ecae0c24137e674841573ccd1f869cab8351be` |
| `appkit-analytics-recipe-PFeh7B` | Corrected counts, old renderer: 122 / 20 | `0e783a08908360e2c8a9076eec1a6cf40f62cd5f21b4e75a867fd6ac8d3d0c91` |
| `appkit-analytics-recipe-szxeDv` | Intermediate repair, before column-width oracle: 150 / 0 | `12d48a030ecf477411d376263b4469e4da3d7ed592071819239d7c59f7f27a4f` |
| `appkit-analytics-recipe-ds698M` | Final oracle, old renderer: 124 / 26 | `6c90f63180a83fcfeacda76bc68366747e1f90894e32059b67920573b07d0b6c` |
| `appkit-analytics-recipe-RyiDe3` | Final oracle, reviewed renderer: 158 / 0 | `d6f89435e2d607a2bd35f74c635216dec144fe12cba15278bbf5de67cbdd3a68` |
| `appkit-analytics-quality-yLNKHD` | Existing six-case comparison task: 1,026 / 0 | `229e3daeadb8e621465c415e1b3a7e4c60a5ed0ba3240315f42733bf373fe878` |

The final baseline's 26 failures are 12 label/mark overlaps, eight missing named
scroll regions and six squeezed row-label columns. Added focus-path successes mean
pass/fail totals are not a fixed independent-requirements denominator. The existing
journey's count rises because its new integration link is included in repeated layout
checks, not because the task gained 24 new requirements. Its five local DOM-ready
samples give p95 22.9ms against the predeclared 250ms ceiling, not a speedup claim.

Baseline binary SHA256: `f7011ce7ef57c91aa1a90b567c5466c1364fda61fd03995382864da578d6ac23`.
Final browser-tested binary: `25e7f8ff6d4b4bc6c7c12114426a4c374b4c6eb04643ea65f33891ce501fb815`.
Final recipe collector: `df15f53091e215d061a9368f01af41deb98ede07f55b38f594d4d0381c0ee420`.
Final served CSS: `9e2d822864db965307afe0d87828208d63c7082773cb8e57a1aecc7c1bc60d46`.
Receipts retain response-body, CSV, screenshot and selected Playwright entry/package
hashes; they do not bind the entire transitive toolchain or create a permanent archive.

## Deterministic checks and remaining work

`go test ./...` passes. Selected `viz`, `analytics`, catalog, Gallery and MCP race
tests and vet pass. Twenty-four recipe selections require the exact `RenderBars`
output once and byte-identical direct `WriteCSV` output; the real Gallery mux is tested.
New value-column and named-region tests failed before repair. Layout fallback tests
cover long/multilingual labels, maximum values/long units, wider-option recovery,
unchanged CSV and absent misleading legends. Existing 48-selection analytical
agreement tests remain. A fallback's static tests are not arbitrary-label browser proof.

INC-36 records the defect and outcome; scope, integration guidance and changelog are
updated. No public Go signature, profile or catalog ID changes. No plan checkbox or
recipe maturity is promoted. No dependencies, consumer files, global configuration,
commits, release or real data are changed; Windows/CA/GGB stay deferred.
The focused intent-doctor subset passes ten living-doc budgets, 206 relative links,
identical intent stanzas and a 58-line Unreleased block, not full host/config wiring.
Three owned Gallery servers exited cleanly after verification; temporary synthetic
evidence remains local, with no user browser/profile opened.
Other browsers, actual zoom for this renderer, screen readers, pointer/native input,
large datasets, human tasks and two-app acceptance remain open. Repeatedly expanding
this small fixture does not substitute for those goal exit criteria.
