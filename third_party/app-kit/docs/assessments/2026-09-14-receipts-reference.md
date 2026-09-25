# Runnable receipts reference: scoped evidence

Date: 2026-09-14. [Task contract](../design/matdash-analytics-adapter.md),
[ADR-0038](../adr/0038-runnable-receipts-reference.md), [launch guide](../../apps/receipts/README.md).
This is first-party synthetic cash-flow evidence, not a MatDash port or production service.

## Outcome

| Surface | Observed result | Boundary |
|---|---|---|
| Projection and CSV | Literal cents/raw-measure oracles, 72 overview/detail selections, zero/missing/partial/stale semantics | No live facts or accounting backend |
| Signed comparison | One common scale, current solid/previous outlined, negative and zero marks, native detail links | Private renderer, no shared chart-engine promotion |
| Real HTTP journey | Native filters, explanation, product detail/back, theme preservation, actual matching CSV download | Loopback-only fixed fixture |
| Browser | 144/144 checks; six screenshots; exact CSV download | Chrome 152.0.7977.84, isolated Playwright 1.63.0, synthetic data |
| Payload | Maximum 24,235 HTML bytes across 72 overview/theme combinations; zero client scripts; CSS 53,101 bytes | HTML budget 32 KiB; not a network/production benchmark |
| Handler | 0.062–0.085 ms/op; 173,708–271,668 bytes/op across five cases, three runs | Apple M4/darwin-arm64; excludes process/CUE startup and network |
| Build | Two identical 23,133,282-byte `-trimpath` executables; CSS rebuild unchanged | Dirty working-tree evidence, not a released artifact |

September's visible/table/CSV net is $1,550.00 versus August $2,300.00. Atlas contributes
-$600.00 of the -$750.00 change; Cedar is -$150.00, Beacon unchanged. October keeps
known zero separate from missing inputs. A selection with no known values says
“Not known,” not a zero subtotal. November labels the actual partial interval beside
its chart/table values and refuses an exact full-month amount or contribution ranking.
Product-detail headlines/totals name the product scope rather than an overview total.

## Browser and independent review

The harness loads the compiled HTTP server, not a static HTML reconstruction. At
390/1440 CSS pixels in both themes it checks outer geometry, internal table scrolling,
literal current/previous table facts, common-axis signed bar geometry and the actual
80,000:15,000 bar-width ratio. Native Menu and skip-target focus pass. Keyboard
activation of the negative Cedar SVG link reaches its detail and returns to the same
selection; its hit rectangle has a 2px non-scaling focus stroke and 44px height.
The keyboard filter task changes all four native selectors and submits their exact
values. Downloaded October CSV retains 18 literal measure rows, including missing
refunds with known receipts and September's negative net comparison value.

True 200% Chrome zoom is set only in a fresh temporary profile: DPR 2, a 720-CSS-pixel
layout inside the unchanged 1440-pixel window, visual scale 1, no outer overflow.
Detail/back remains functional. A 256-character label stress case checks actual
wrapping in the mobile nav, chart and table; it mutates only the fetched test DOM,
not server input or live records. Its matching Go fixture separately checks escaping.
No unexpected page traffic/errors, persistent selection storage or workers were observed
on the scoped instrumented routes. Screen-reader and physical touch tests were not run.

Review caught and repaired an all-missing zero display, ambiguous detail totals,
partial-period labels, a 38.89px product-link target and touching label/value text.
Go and browser canaries retain these boundaries. Earlier failures also exposed harness
timing/native-select assumptions. Playwright's full-page screenshot clipped true zoom
to 720 physical pixels despite correct layout; the corrected CDP viewport capture is
1440×1000, includes every right-edge control and is visually inspected. Earlier failed
runs and the clipped image remain separate evidence, not passing captures.
Desktop light, narrow dark/long-label and corrected zoom images were visually inspected.
Impeccable's product guidance shaped restrained task-first hierarchy, role-token styling,
native controls, wrapping labels and independently checked focus/target behavior.

## Identity and reproduction

Private evidence is under `data/design-assets/receipts-reference.T3WybH/`:

- `browser-final6/report.json`: `c2c92028abe1ce6644d572f3d4b10997af9bde96d2a56eece5852a8033779bba`.
- Launched and repeated binaries: `b8662109a13521b5670bfdeeb18781ead634c545152971da0d572fb589e1930e`.
- Served/rebuilt CSS: `2be2cbf23dc24dd30da40d2c9052e9925a11a8799d228408306eceb85505548a`.
- `build-receipt.json`: `706381b177764326492b6273eac045318a7ffac388e4bb9bc9727b076837e0bc`.
- 635 non-standard source/embed/module inputs: manifest `d20df58bc9e6290fc47fcf4c466708c121ab6ed1e5bc03ce014bfaa954255cf1`;
  38 app-kit compile inputs copied into `source/`. Standard-library identity is the
  recorded Go 1.26.3 toolchain, not an independently snapshotted compiler.
- `benchmarks.txt`, `payload.txt`, `go-test-final.txt`, build info, earlier browser
  attempts and six final screenshots preserve raw results. These are ignored local
  artifacts, not durable publication or licensed-catalog content.

The main tree's focused receipts/Gallery/internal-CSV/MCP race suites, scoped vet,
CUE validation and whitespace checks pass. A separate compile-source snapshot builds
with `GOWORK=off GOPROXY=off GOSUMDB=off -buildvcs=false` against the existing module
cache; app-kit resolves inside that snapshot, not the sibling checkout. This targeted
build is not a complete clean-install or consumer-source snapshot.
Commands live in the launch guide; browser checks require the explicitly approved
isolated harness, a running local server and a fresh empty evidence directory.

## Remaining gates

The original 13-panel MatDash composition, source/font/asset fidelity, real consumer
acceptance, human task-time/error baselines, broader accessibility and native-host
delivery remain open. Conversion counts stay **three typed candidates, zero certified**.
G1–G6 are not complete. Windows and CA/GGB adoption remain owner-deferred.

Intent-doctor subset:

| Check | Result |
|---|---|
| Router/living docs/new ADR budgets | PASS: router 77/80; living docs within bounds; ADR 60/80 |
| Register and MCP document parity | PASS: focused protocol/index tests |
| Checked relative links | PASS: no broken links in routed intent, index and affected guides |
| Unreleased budget | PASS: 60 lines; redundant uncommitted history note pruned, all change entries retained |
| Host invocation/versions/full semantic audit | SKIP: not re-proven by this structural subset |
