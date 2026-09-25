# Native-zoom chart paint: one-cell capture diagnosis

## Prospective resume contract

- Packet: R4/R5, the unresolved Fathom 200% call-chart raster from the
  [large-value packet](2026-09-22-fathom-large-zoom.md), not another broad matrix.
- Need/hypothesis: geometry says the chart is visible, while Playwright's screenshot
  omits it. A screenshot coordinate conversion may mix CSS and device pixels.
- Deliverable: one light/200% call chart, direct no-clip compositor capture before
  and after the existing Playwright capture on the same page. Inspect the actual
  PNGs for the two exact labels/values and visible chart. No app/CSS/data changes.
- Inputs: app-kit `852b5f0`; reuse the frozen large-value fixture binary
  `3fe1408f016a85535d3289dfd545f90772cb664a03af32b819e6b1e66629e7d8`,
  source archive and consumer CSS from that packet. Validate unchanged inputs before
  preparing a separate prospective contract; record consumer-source drift separately.
- Authority: kit-owned synthetic fixtures/docs, private browser profile, commit/push.
  No consumer writes, dependencies, live data, daemon restart or global changes.
- Acceptance: exact labels, completed same CSS, actual Chrome Appearance 200%,
  unchanged target/scroll/zoom across captures, 1440×1000 physical PNGs; actual pixel
  review remains mandatory and separate from machine success. Bracketed compositor
  images must match. Preserve the failing instrument's image, even if candidate works.
- Cost cap: one ≤180-second acquisition; one economical source reviewer; approximately
  30 minutes. Stop after this acquisition, even on failure. Do not expand to dark or
  guides. Operational startup failure yields a retained handoff, not an automatic retry.
- Return: the maintainer records exact executed source/contract/output identities and
  the narrow successful capture recipe, or the remaining failed hypothesis. Existing
  failures and broader G3/G4, table, accessibility, memory and human gates stay open.

## Result

**The one-cell capture diagnosis passed.** One acquisition completed 19 machine
checks in approximately six seconds, followed by direct inspection of the retained
PNGs. This is the same renderer, source, CSS, fixture values and native zoom as the
earlier failure. All fourteen pinned inputs were checked; eleven selected current
consumer files still matched the frozen source. No rebuild or consumer edit occurred.

| Instrument / observation | Result |
|---|---|
| Direct CDP before/after | Both 1440×1000 PNGs are byte-identical. The inspected image visibly contains the call-chart heading, both bars, `find_symbol` = `1,234,567` and `references` = `1,000`, without clipped labels. |
| Existing Playwright control | Reproduces the old wrong-region screenshot byte-for-byte, including its blank region and missing call chart. Its hash matches the prior light/200 failure. |
| Same-page geometry | Target is x37/y225/w560/h50 in the 720×500 CSS viewport, scrollY713, DPR2 and Appearance200%. These values are unchanged before/after each capture. |
| Inputs/assets | Same 33,879-byte app CSS, completed response bodies/fonts/styles, no application scripts. No blocked page requests, request failures or page/console errors. |
| Owned lifecycle | Chrome and fixture exit0; bounded drains complete, no forced termination, final loopback port refuses. Chrome stderr retains allocator/display-link warnings; this is not an error-free host claim. |

This establishes an instrument-path discrepancy for this exact configuration, not a
rendering omission. Source review of the pinned Playwright `coreBundle.js` shows that
`_originalViewportSize()` prefers the cached emulated1440×1000 size while native zoom
reports720×500 CSS pixels. Playwright's Chromium delegate constructs an explicit
clip from layout metrics before calling CDP. The exact faulty conversion is not independently isolated;
do not claim that an explicit CSS clip or a library upgrade has been tested.

The bounded working recipe is `Page.captureScreenshot` with `format:"png"`,
`fromSurface:true`, `captureBeyondViewport:false` and **no clip**, after real zoom,
target-in-viewport, font/stylesheet/body and stable-state checks. No pixels were
cropped, rescaled or otherwise transformed. It is a fixture-specific measurement
path, not a replacement for Playwright screenshot preparation on arbitrary animated,
editable or loading pages. Keep visual review separate from `report.passed`.

## Replay and retained evidence

The [preparer](fixtures/fathom-large-zoom/prepare-paint.mjs) refuses any changed
historical input/source archive, copies the unchanged owner/zoom/lifecycle helpers,
and copies the [diagnostic driver](fixtures/fathom-large-zoom/paint-browser.mjs) as
private `browser.mjs`. It freezes a new contract, not a replacement historical one.
Run it with the old contract and an empty absolute private staging directory, then
run that directory's `harness/fathom-large-zoom/owner.mjs` with its `contract.json`
and a new output directory. Use the contract's exact Node executable.
The prior packet retains the binary/full source; their locations must still resolve.

Durable evidence root: `/Volumes/m2ext/appkit-evidence/appkit-native-paint.5WfjTbFA`.
Internal staging: `/private/tmp/appkit-native-paint.6tPRg6XL`.
`acquisition-packet.tar` contains the contract, preparation source, executed harness,
all three PNGs and machine report, excluding only the fresh synthetic Chrome profile.
The report/PNGs and executed sources also remain directly accessible in that root.

| Artifact | SHA-256 |
|---|---|
| Pre-execution source/contract-spec archive | `f7e24b1c76272fb527f31a8fdf0474863021a7795e0b0dad62b4ab1f62534672` |
| Acquisition packet | `efa29bbcae4c12a05ebaca172ab80bc1ada3f83866a02c69349d2a93ed90d13a` |
| Prospective runtime contract | `4170752ef8a0ce5982b55c744950e31c4ba050c422e109ac11b35e992a1f1776` |
| Report | `fdb5efabb3fd993680c803e93b7230af61fbe5e82cdfe6ba1eb0c07dd3ee6451` |
| Both direct compositor PNGs | `8a40eff1dbbb12fa56433c67f13307ddbdbe12fb47fb2c66d08194bf72281442` |
| Playwright control PNG, matching historical failure | `a678a796d34ed4964a2a7ab4c5fad40cf1e3e0199b98d7d0a7ed97e82c4e594d` |
| Executed diagnostic driver | `4f8ec606e60bcccd14039bec9d353cc7b4209715884d5092829e78e2afc25724` |
| Executed preparer | `e7e0774a3a2e4315f862d5f548d15be9dae52ce8204a288e5f2000f792d05c2b` |

The return is the reusable diagnostic/recipe and a resolved **light/200 call-chart
capture** gap. The earlier matrix remains failed; dark/200 and guide rasters are not
promoted. Do not repeat this diagnosis with unchanged inputs. A future selected
appearance task can use this recipe, verify its own targets and inspect its images.
The stress fixture's aggregate inconsistency, chart/table alternatives, human A/B,
accessibility and memory/performance remain independent open work. No CUE quality
gate, target certification or weighted completion category is promoted.
