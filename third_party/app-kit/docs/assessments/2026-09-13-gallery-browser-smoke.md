# Gallery browser smoke evidence

This is a dated, local browser-smoke receipt for selected Gallery comparison and
theme surfaces. It is not a current rerun or a platform-support claim. The applicable
evidence boundaries remain in [invariants](../intent/invariants.md),
[prepared-comparison ADR-0028](../adr/0028-prepared-private-static-comparisons.md),
[comparison-controls ADR-0030](../adr/0030-isolated-comparison-workspace-controls.md),
and [runtime selection](../design/runtime-selection.md).

## Recorded environment

- Observed 2026-09-13 America/Phoenix (2026-09-14 UTC), from
  `2026-09-14T00:01:56.340Z` through `2026-09-14T00:02:05.745Z`.
- Headed Chrome `152.0.7977.84` driven by Playwright `1.63.0`.
- Local origin: `http://127.0.0.1:18766`.
- Fresh isolated Playwright browser context; no existing browser profile, cookies,
  signed-in portal session, external app, or global dependency was used.
- Harness allowed only same-origin `GET` and `HEAD`, blocked WebSockets and would have
  blocked foreign origins or other methods. No blocked or failed request was recorded;
  no `POST` or foreign request was attempted.

## Result

The report contains 98 assertions: 96 passed and 2 failed. Sixteen screenshots were
captured across 1440x1000 and 390x844 viewports for the P02 comparison, theme lab, and
themes surfaces. Screenshots remain in the temporary harness directory and were not
copied into the repository.

Observed passes include selected-page status, no outer horizontal overflow at sampled
states, empty iframe sandboxes with `no-referrer`, original/converted selection, fit and
actual-pixel modes, side/stack layouts, dark preview, fullscreen entry, and explicit
fullscreen exit. The explicit Exit fullscreen control passed at both viewports.

Both failures were the automated `Escape exits` check, one per viewport. Playwright's
keyboard dispatch did not exit browser fullscreen; the harness recovered through the
explicit exit control. This does not prove that a trusted OS Escape fails, and trusted
keyboard behavior remains unverified.

The narrow flip/fit transition also exposed a genuine layout shift: the
available comparison width changed from 334 px to 349 px when the root scrollbar state
changed. Individual sampled overflow assertions passed, but that discontinuity is not
accepted as a stable narrow-layout result.

The follow-up below repairs and rechecks this specific shift; it does not change the
original run's recorded failures or certify other viewing/interaction conditions.

Ten page errors reported an attempted read of `navigator.serviceWorker` from sandboxed
opaque frames lacking `allow-same-origin`. Their shape is consistent with Playwright's
`serviceWorkers: "block"` instrumentation interacting with the intentionally empty
sandbox. The run recorded zero console messages, but this correlation is not proof that
application code is error-free.

## Evidence identity and limits

- Report: `/tmp/appkit-browser-review.xBrDCp/report-complete.json`
- Report SHA-256: `4c0080a9d1188d20eb52f26e7b40804c4bbba7ee429d986a6174fa3e30ac2eb4`
- Harness: `/tmp/appkit-browser-review.xBrDCp/smoke.mjs`
- Harness SHA-256: `d595a4ab2d848613805f56ad7613b909fed2ed6539b85c7640edeeb2a1c19fc1`

The report is temporary-path evidence and was not copied wholesale into this checkout.
This smoke run does not certify visual fidelity, a complete shell or analytics task,
mutation/CSRF behavior, streaming/reconnect, authentication, remote hosting, native
capabilities, Wails/WebView behavior, packaging, or macOS/Windows platform support.
The [Wails experiment](../../spikes/wails-gallery/README.md) retains its independent
native and delivery blockers.

## Follow-up: stable root gutter

An unchanged isolated harness ran against the old Gallery on port 18766 and the patched
build on port 18767. Headed Chrome 152 / Playwright 1.63.0 used fresh contexts, only local
GET/HEAD, both parent themes and 390x844 / 1440x1000. Flip → side → stack → flip reproduced
the mobile workspace changing 351→366 px and the fitted preview 334→349 px before repair.
After reserving the root gutter, all 115 targeted assertions pass; the workspace remains
351 px and the fitted full-width preview 334 px across the same mobile transitions.

The before run fails six assertions: four computed-gutter checks and two actual mobile
layout-stability checks. The after run fails none. Fixed 1440 px iframe layout, fitted
geometry, no outer horizontal overflow and sandbox/referrer attributes remain intact.
Original-light and compiled-light response hashes are unchanged across builds; only
the parent stylesheet changes. No reference source, prepared baseline or script privilege changed.

`TestPricingComparisonRootGutterIsStable` failed before repair; Go/race comparison tests
and all seven Node controller tests pass afterward. Four screenshots per run were captured;
mobile-dark and desktop-light after images were visually inspected. Eight instrumentation-
shaped serviceWorker page errors remain in each run; no blocked/failed requests occurred.
This run does not re-test fullscreen Escape or certify all accessibility/target behavior.

Temporary artifact identities (same harness directory as above):

- `gutter-smoke.mjs`: `90349baf558021da2f5f224fab7e02e586806f353f59815997f32ae987f6e10d`
- `gutter-v2-before.json`: `dcd5c5039b05fce476c5a69c46f4d0eab5a5855c9056ffd75153d2c5cc77bf4b`
- `gutter-v2-after.json`: `1d7224c6f67f630dabfe15c35d17c5c4797a7e3bd16b48b1e0f1d1a13682b141`
- Patched binary `/tmp/appkit-gallery-gutter.RMxe9o/gallery`:
  `7e908a672bc43659631e6afa115abe70c53aad4aba177846bd9c0dea8f0d7721`.

## Analytics journey follow-up

The existing synthetic `/analytics` workflow now has local browser task evidence.
Fresh **headless** Chrome 152.0.7977.84 / Playwright 1.63.0, macOS arm64 Mac16,10
(10 logical CPUs, 32 GiB), exercised builds on ports 18778 (before) and 18779 (after).
Only local analytics/CSS GET/HEAD was allowed; WebSockets and service workers were
blocked. No signed-in profile, provider data, app mutation or dependency install.

Final unchanged harness: **107/107 after**, versus **105 pass / 2 fail before**.
Both failures are truthful-copy assertions for partial data with `compare=none`:
overview and detail promised previous-month values despite omitting them from the
table/export. Visual review exposed what the first numerical checks missed. The
renderer now conditions prose on the actual selected reference; values and comparison
policy are unchanged. `TestAnalyticsPartialStateDescriptionMatchesSelectedComparison`
guards both routes and comparison branches; all analytics race tests pass (4.789s).

Covered: six overview viewport/theme combinations (390/768/1440, light/dark), no outer
horizontal overflow, focusable chart/table regions, native disclosure activation,
programmatic select changes followed by Enter submission, detail/Back Enter activation,
preserved selection, visible detail provenance and four downloaded CSV fixtures.
CSV checks require rectangular 31-column data and exact independent values: August
partial API `4,2`; Gateway September/August `4,6`; worker zero stays numeric `0` and
missing current data stays empty with the previous reference intact. Partial coverage,
reporting windows and the no-full-month-delta rule remain explicit.

Ten screenshots per run; phone overview light, desktop overview dark, phone partial
dark and before/after detail dark were visually inspected. The phone table deliberately
scrolls horizontally; the selected filters/button measure 44px tall. Programmatic focus
and `selectOption` do **not** prove full Tab traversal or native arrow-key selection.
Real 200% browser zoom, computed contrast and complete accessibility remain untested.
No page/console errors or rejected requests occurred in the final run.

Thirteen directly navigated HTML responses are 13,621–21,841 bytes, below the existing
32 KiB bound. Observed load-plus-body reads were 18–54ms on loopback; no latency budget,
human task-performance improvement, cold-network or production-scale claim follows.
The complete Go suite, vet and 46 Node tests also pass; G3 and consumer proof remain open.

Temporary evidence under `/tmp/appkit-browser-review.xBrDCp/`:

- Harness `gallery-analytics-smoke.mjs`: `a4dacab69675820240b33e2b1e147a50d53d487b76fd2b14456c39a1cc33441f`.
- Before `gallery-analytics-before-report.json`: `9f7b09b8f9aeef19a08beef9ce493b7c344c657efa99108e7b13271a747eee32`.
- After `gallery-analytics-after-report.json`: `229b7dfb831f8a9cbe57cd0c4e482597963cbdcd8296ca8689247ddd2e178691`.
- Corresponding `gallery-analytics-before-screenshots/` and `gallery-analytics-after-screenshots/`; CSV files have distinct before/after names.
- After binary `/tmp/appkit-gallery-analytics.x2YnFR/gallery-after`: `824c350f227f9fb5b72b07ef5b361960b967b00d86276946b4a1fdf21d4e90a8`.
- Renderer: `0c61599a1560b6986b54435a4630091cb8246989806ed71778921c905a1c4ff7`; template: `a6441176ea6bd11f73e4dd940537e84c116771d1626c6cc44cbfea8d0942ae34`.

## Guidance delivery check

A fresh isolated compiled MCP process returns the final `analytics-journey` document
byte-for-byte through `get_kit_decision` and `kit://analytics-journey`; `get_kit_guidance`
routes to it. Probe: `/tmp/appkit-mcp-analytics.89XPnv/probe`; candidate `appkit-mcp`
SHA-256 `2516872da3dd372529b8ea4a6b4a3e758faca0ae4a1a3d069699948e02ab79ac`.
Document: 7,122 bytes, `76054a5530006795db0fe9a4eea909fe4f3c2e891bd47ba7c3bfc2e38dd2acfd`.
Existing installed MCP processes were not rebuilt/restarted and may serve older startup
snapshots. This is fresh-process protocol proof, not current consumer-host delivery.
