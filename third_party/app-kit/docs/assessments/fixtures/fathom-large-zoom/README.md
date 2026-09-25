# Fathom large-chart and zoom fixture

This is a test-only overlay for the authorized synthetic Fathom theme guide. It
does not modify Fathom, app-kit mirrors, live Observatory routes or user data.
The ordinary `/dev/theme` handler remains `observatoryGuideMux`; `/large` is a
separate loopback-only route that calls the production `renderObsFleet` with
1,000 synthetic grade-A services and two synthetic call rows (`1,234,567` and
`1,000`). Its exact visible task caption is:

> Synthetic chart-scale task: grade A = 1,000; find_symbol calls = 1,234,567; references calls = 1,000.

The route accepts only `?scheme=light` or `?scheme=dark`, makes renderer links
inert with `disableObservatoryGuideLinks`, emits a script-free CSP and serves the
same embedded `obsAppCSS` used by the Observatory at `/app.css`. That consumer-built
stylesheet already imports Fathom's selected `cmd/fathom/ui/tokens/theme.css`; serving
the app-kit mirror's different default `tokens.ThemeCSS()` would test the wrong palette.
The small fixture document
wrapper is intentional: `obsPage` always adds operational Observatory navigation,
which is outside this isolated task. Production chart, widget and SVG markup is
not replaced. `viz.Bars` has no table fallback, so this fixture makes no table
agreement claim; the exact caption and SVG labels are the available text oracle.

## Prepare in a private copy

Start from the reviewed source closure at
`/Volumes/m2ext/appkit-evidence/appkit-fathom-integration.SOWNalRa/src`; do not copy into the
active Fathom checkout. Copy `fixture_test.go.txt` to
`cmd/fathom/fathom_large_zoom_test.go` in that private copy and freeze the source,
test and tool identities before compiling.

Reuse the prior packet's exact offline build environment:

```sh
env GOWORK=off GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-mod=readonly \
  GOMAXPROCS=2 CGO_ENABLED=1 \
  GOCACHE=/Volumes/m2ext/appkit-evidence/appkit-fathom-guide-only.v7yRsn/go-cache \
  GOMODCACHE=/Users/geoff/.dev-cache/go/modules \
  TMPDIR=ABS_PRIVATE_OUTPUT/tmp \
  GOTMPDIR=ABS_PRIVATE_OUTPUT/tmp \
  MACOSX_DEPLOYMENT_TARGET=14.0 \
  /etc/profiles/per-user/geoff/bin/go test -c -mod=readonly -trimpath \
  -o ABS_PRIVATE_OUTPUT/fathom-large-zoom.test ./cmd/fathom
```

Run the deterministic model test normally. For a browser owner, launch only the
named server test with `APPKIT_LARGE_CHART_FIXTURE=1`; read the printed
`APPKIT_LARGE_CHART_ORIGIN`, append `/large` and the exact scheme query, and terminate the
owned test process after the bounded browser work. The server binds a fresh
`127.0.0.1:0`, exits after at most 180 seconds and performs bounded shutdown.

```sh
ABS_PRIVATE_OUTPUT/fathom-large-zoom.test \
  -test.run '^TestAppKitLargeChartModel$' -test.v

APPKIT_LARGE_CHART_FIXTURE=1 ABS_PRIVATE_OUTPUT/fathom-large-zoom.test \
  -test.run '^TestAppKitLargeChartFixture$' -test.v
```

The browser workload remains separately frozen and identity-pinned. This fixture
does not establish accessibility, human-task, performance, production-data or
whole-guide acceptance.

## Owned browser acquisition

The [owner](owner.mjs) takes an absolute prospective contract JSON and a new output
directory. The contract pins the binary, source CUE/CSS/fixture, owner/driver/helper,
Node, Chrome, Playwright entry and manifest. The helper is reused byte-for-byte from
`../fathom-guide-only-2026-09-22/browser/owned_process.mjs`; keep that relative path
when making a private harness copy. The [driver](browser.mjs) is imported only after
input checks. Preserve executed files before every acquisition or repair.

```sh
EXACT_NODE ABS_PRIVATE_HARNESS/fathom-large-zoom/owner.mjs \
  ABS_PROSPECTIVE_CONTRACT NEW_PRIVATE_OUTPUT
```

The owner starts only the named synthetic fixture and a separately owned Chrome
process with a fresh private profile. Four chart cells cover light/dark at native
100%/200% zoom; one light/200% guide journey covers native validation/repair/reset.
The dark guide journey is explicitly not repeated. Renderer labels and geometry do
not prove chart/table/export agreement. The owner requires graceful fixture exit,
browser process exit, completed output capture and final port refusal.

The [assessment](../../2026-09-22-fathom-large-zoom.md) records preparation failures,
the exact acquired source/runtime identities, results and remaining work. Do not
reuse a prior prospective contract for a different binary or silently repin it.

**Known limits from acquisition:** the automated report passes 118 checks, but native
200% raster capture still omits the call chart. Do not treat `report.passed` as visual
acceptance. The source-pinned `zoom-contract.mjs` handles native scrollbar width with
14 positive/refusal tests. The owner also pins the synthetic server's sandbox policy;
Chrome retains its normal sandbox and a fresh private profile, not the extra Go sandbox.
The scale fixture's service-call totals were not reconciled with its per-tool rows;
it cannot qualify overview/chart agreement. See the assessment before reusing it.

The separate [one-cell paint diagnosis](../../2026-09-22-native-zoom-paint.md)
resolves light/200 call-chart capture using no-clip compositor PNGs, while reproducing
the original Playwright failure on the same page. `paint-browser.mjs` and
`prepare-paint.mjs` replay that bounded comparison; the original driver above remains
historical and its matrix remains failed. Do not infer dark/guide raster acceptance.
