# Analytics and adoption: scoped integration, 2026-09-22

RAC's owner-approved channel-yield CSV is integrated in its **working tree**, not
committed or deployed. Shared legend truth and readiness-reader integrity defects
are fixed in app-kit. No whole-goal, conversion or inventory gate is promoted.

## Scope and ownership

- App-kit starting revision: `0dbeccb`; preserve existing failure evidence.
- RAC: `kit-governed-adoption`, HEAD `04be1238bae18e9542b135060233412b4abd72ca`.
  This differs from the prepared packet's HEAD, but all eight selected source
  preimages match that packet. The approved four-file patch applies cleanly.
- Fathom remains parked. Another Codex session owns Conjure adoption; do not
  perform simultaneous consumer edits. Human A/B remains explicitly owner-deferred.
- No real registry/logs, providers, dependency upgrades, global config, deployment,
  consumer commit/push, default-theme change or app-kit vendor refresh.

## Delivered behavior and proof

| Change | Proof | Boundary |
|---|---|---|
| `RenderBars` legends describe only measured roles; zero remains measured, missing rows retain their table/CSV meaning | Missing-only legend regression failed before repair; analytics package/race pass; 50/50 real Gallery browser assertions | Synthetic partial/complete cases, not all themes or certified targets |
| Shared quality reader refuses duplicate app/revision identities within one work item | Identical/conflicting status failures reproduced; two refusals/four legal controls and full reader race pass | Go reader enforcement, not a new CUE uniqueness rule or live adoption verification |
| RAC aggregate export uses existing `ops.ChannelYield`, leaving legacy absent-view CSV/JSON intact | Four-row fixed expected values, sample floor, incomplete/no-data/zero/no-sent states, formula protection, focused race and package vet pass | Domain policy remains in RAC; separate live reads are not snapshot-bound |
| RAC unknown export views refuse rather than returning detailed application data | CSV/JSON typo regressions first returned 200, now 400 before registry read; non-CSV channel view also refuses | Existing method/auth middleware policy unchanged |
| Actual RAC Pipeline native download | 111/111 checks, five synthetic states at 1440/light preference and 390/dark preference; native disclosure and Tab/Enter download | Synthwave remains the same palette in both preferences; not two different app themes, full-app auth, hosted or human evidence |

RAC's five states: four populated channel rows, incomplete sent evidence, measured
zero, no applications, and an unsent application. Checks include all four populated
CSV rows, source/scope/freshness fields, real embedded CSS and bounded analytics-region
geometry. Desktop coherent and phone incomplete screenshots were visually inspected.
The full Pipeline board/aging surfaces are not accepted by this region check.

Before measurement: 20 fictional applications, widget ≤16 KiB, CSV ≤8 KiB;
combined pure render ≤5 ms/op and ≤512 KiB/op. Integrated reading on Apple M4,
Devbox Go 1.26.2: **5,365 bytes / 1,870 bytes; 22,131 ns/op; 35,500 B/op**.
Browser, CSS, networking and production scale are excluded from that timing claim.

Root checks: `go test ./appkit/ui/analytics ./internal/qualityplan ./apps/gallery
./cmd/appkit-mcp -count=1` passes. Strict Fathom verification of the four existing
plans reports **56 total / 40 claimed / 40 verified / 0 failing**: static claims,
not runtime proof of the remaining goals.
Scoped intent-doctor checks pass: root router/living-document budgets, 60-line
Unreleased block, relative targets in 15 selected documents and unchanged identical
RAC app-kit routers. Closed INC-1 and overlapping unreleased entries were compressed;
their longer history remains in `0dbeccb`. This is not a full intent/host audit.
The selected four-package `go vet` and final independent diff/evidence review pass.

## Retained failures and reconstruction

Private scratch: `/Volumes/m2ext/appkit-evidence/appkit-analytics-adoption.KX4Xx1`.
Original failed sources are retained or exactly reconstructible from the named
archive and one stated edit; later passes do not replace their outcomes.

| Artifact (relative to scratch) | SHA-256 |
|---|---|
| `legend-red-inputs.tar` | `daa8283824fb73c889882975eac437d58def7615c2065cf0aaa6e8598409b3ac` |
| `legend-green-inputs.tar` | `b7bab9b35d8e6d89b69e781c745c9faeaac617f071bdc45f179b1888879ba287` |
| `quality-red-inputs.tar` | `f9034dcb88e8f023eb2c839c491b9f67f4d23c2cfa5276ecd082f43597f3c628` |
| `quality-green-inputs.tar` | `b0c16dcfe68d8f7fa18f30bcca2a60e59f42b53d47e62c2084a6822d297d13d4` |
| `kit-browser/inputs-01.tar` | `22525a33f27845e6e0a1e445c18f2c23d731f1915c9bcc2f3206246aa164936b` |
| `kit-browser/run-01/report.json` | `f2c0c80caf341ba44104b8ec8235b0dc70702e8ff0483175e91d9170d2ce9e55` |
| `rac/pre-apply.tar` | `41a5450ea3e95327a691807af79db9b68d7fcf3edc635106d3a2c05b79f3b4f3` |
| `rac/review-red-inputs.tar` | `e609065600c8c11442e4e5d8643401320b02d2f39dc7d596b5c824b528e6c070` |
| `rac/review-green-inputs.tar` | `a4124090f28269b9a87f9059ef5327239aff7716f56ba73aa880288cca1290ff` |
| `rac/browser-support-01.tar` | `7841a1297e98788c953bda9640eea7018c3644879bfd0e632ad18990b571addb` |
| `rac/browser-inputs-01.tar` | `ce67d8cd4df89ec2036c12b0f04966871a48ea59ecf0b6879be3f56586fb1aa0` |
| `rac/browser-inputs-02.tar` | `3d9c0e57ef069de164167eef0e98b7f862b95a0ab1672f8789919ae16749a772` |
| `rac/browser-inputs-03.tar` | `5141afc3d3e4dd55a29529566a12a3f9bda2addc015944982578b289dfd46ade` |
| `rac/browser-03.test` | `c727501e305a091802070e18761d6c8c0ceea8b9efe627be63edb79a86c1e831` |
| `rac/browser-run-03/report.json` | `4ad37ad301db9259480e56c59900354d62505d27dab949ba7af9c3afe2f1bf97` |

Quality-reader intermediate positive control incorrectly chose the existing
`artifact` kind as a different kind. Its exact failed test is the green archive's
test with `"kind": "source"` changed back to `"kind": "artifact"` in that control.
The correction changes only the mistaken control, not duplicate refusal.

RAC browser run-01 failed after two passes: its oracle checked a rate inside a
closed native disclosure. Run-02 opened the actual summary and passed all 111 task
checks, but exited 1 when fixture shutdown closed the connection before its 204
acknowledgment flushed. Its `passed:true` field reports tasks only and **must not be
read as a whole-run pass**. Run-03 adds explicit acknowledgment flushing and makes
cleanup failure set the overall flag false; unchanged task assertions and cleanup
pass. Both earlier reports, screenshots and executable inputs remain. No app change
was needed for those harness defects. All fixture processes exited; no existing
preview, daemon or signed-in browser was touched.

## Portable consumer handoff

Apply the unchanged [September 21 patch](fixtures/adoption-analytics-2026-09-21/rac/channel-yield.patch)
only to its reviewed preimages, then [review corrections](fixtures/adoption-analytics-2026-09-22/rac/review-corrections.patch)
(`1c54dee975edf253d2274a1b668b6fa2fb277f18d5f0aada2e8c88899c926662`).
Private ordered replay reproduced every final file hash:

| RAC path under `cmd/serve/` | SHA-256 |
|---|---|
| `pipeline.go` | `ef820d6b6158924f5ee173c712c4f6b533288b58f998fe91a7ddd27abcbdfe0b` |
| `settings.go` | `4083ed700f8bdc42ff2ae2e677297abb952fc38eee2a01845b00f57b7188b62b` |
| `analytics_export.go` | `78237c5da8dd2f60145fb21d2adef3584a83a9aff646cf5474f31782178afdf2` |
| `analytics_acceptance_test.go` | `aac076eba87cef07baf72e8c0eb35571638117e38912b3aca6d07e1d89015697` |

From RAC, run the [packet's focused test/benchmark command](fixtures/adoption-analytics-2026-09-21/rac/README.md)
without its overlay option; repeat focused tests with `-race` (not benchmark
budgets), then `devbox run -- go vet -mod=vendor ./cmd/serve`.
For browser replay, overlay [the fixture](fixtures/adoption-analytics-2026-09-22/rac/browser_fixture.go.txt)
onto `cmd/serve/appkit_channel_export_browser_test.go`, compile the test binary,
and run only `TestAppKitChannelExportBrowserFixture` with
`APPKIT_RAC_EXPORT_BROWSER=1`; use the printed loopback origin with the
[retained browser driver](fixtures/adoption-analytics-2026-09-22/rac/browser-smoke.mjs).
Driver arguments: Playwright module, Chrome executable, origin, new output directory.
The fixture caps lifetime at 180 seconds and exposes only synthetic read routes plus
authenticated fixture shutdown. Never launch the real app against its user registry.

## Progress accounting

The earlier approximately87–88% campaign reports are planning estimates: no durable
per-category weights/formula were found, so they are not a reproducible weighted
score. Do not award completion credit for deferral, more tests, or this narrow slice.
G2/G5 retain their bounded acceptance; G1/G3/G4/G6 retain their open criteria.
The frozen conversion result remains **439/657 (66.8%) syntax/compiler/DOM**, not
the whole purchased corpus; **zero full target certifications**. Core-first/JIT
remains the agreed scope, not an obligation to convert everything now.
