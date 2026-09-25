# Complete Catalyst settings adaptation fixture

Private preparation for [ADR-0044](../../../adr/0044-native-catalyst-settings-composition.md).
Tracked files contain first-party collection/policy/fixture logic, not purchased markup.
The source-native reference stays separate and usable. No target certification follows.

## Source capture

Requires the explicitly selected, already-built hardened-v3 private project and its
existing loopback server. Uses the approved isolated Playwright installation and
fresh Chrome contexts; no dependency installation or signed-in browser profile.

```sh
APPKIT_PLAYWRIGHT_MODULE=/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs \
node docs/assessments/fixtures/catalyst-settings-adapter/capture.mjs \
  /tmp/appkit-catalyst-hardened.DdYTWU http://127.0.0.1:55516 \
  /Volumes/m2ext/code/geoffmilleraz/app-kit/data/design-assets
```

The collector creates a new private `p10-source-*` directory. It pins the source
manifest, original ZIP digest, selected CSS, full font/flag closure and tool versions;
captures the raw hydrated five-section form at 390/1440 light/dark; refuses duplicate
FormData names before serializing defaults; records over-limit bio and cleared-region
state. Font license text is retained separately from render assets. No normalization.
`reference.cue` checks the packet shape; the compiled-form rehearsal additionally pins
the exact reference file and verifies every declared retained input before use.

On 2026-09-14, `p10-source-wigFjt` and `p10-source-ohWeRq` each passed 20 checks,
with zero browser errors/forbidden requests and 43 render assets. All 54 retained
form/options/assets/license/screenshot files compared byte-identically. Plans and
reports are not compared as equal: the second collector added duplicate-name refusal.
Latest reference SHA256: `94d3ab4c145e33bf29af750918429833d833fff3f9ac8820e1314c7f2c67f53d`.
Default form SHA256: `5c79ace9bb6c337a1651d615754a1c5e9cd8e9784f4de25bee389cbc9976064d`.
The 390px dark source form was visually inspected; this is not target visual review.

Earlier `p10-source-c6ySq0` failed during setup because the collector initially
treated the font license as a render asset. Its partial directory/plan remains;
it contains no success reference and is not counted as a passed capture.

## Native implementation checks

Source-only policy/view tests run without paid files. Their authored view stub proves
helpers, not the complete form. The separate opt-in test compiles the actual private
transformation with the real model/view and tests complete native controls, options,
escaping, retained invalid drafts, unique IDs and resolved accessibility references.

```sh
go test -race ./internal/designadapt -run 'TestCatalyst(ModelStandalonePolicy|ViewStandalonePolicy)' -count=1
APPKIT_CATALYST_SOURCE_PACKET=/Volumes/m2ext/code/geoffmilleraz/app-kit/data/design-assets/p10-source-ohWeRq \
APPKIT_CATALYST_BUILD_PARENT=/Volumes/m2ext/code/geoffmilleraz/app-kit/data/design-assets \
go test ./internal/designadapt -run '^TestCatalystPrivateCompiledForm$' -count=1 -v
```

Missing opt-in input skips only that private rehearsal. Explicit missing/drifted input
fails. An optional existing private build parent retains generated output for local
inspection; otherwise temporary output is test-owned. Never copy generated bodies
into tracked files or publicly distributable artifacts. A test command is not a pass.

The generated module now includes `cmd/server`: a synthetic loopback-only host for
the real native form and retained CSS/fonts. Launch its `fixture` binary with the
explicit source-packet path; it prints its bound URL. Run `browser.mjs URL PRIVATE_PARENT`
with the same approved Playwright module. See the [assessment](../../2026-09-14-catalyst-native-form.md)
for two final78-check runs, eight identical screenshot pairs and retained failures.
Enter validates; native Reset uses a separate transport form. Never return it to the
edit form's default-submit path (INC-27). The source-only synthetic HTTP test uses
the real handler/policy/view with authored markup, independent of licensed files.

## Optional enhancement

The same fixture exposes `/light/enhanced/settings` and `/dark/enhanced/settings`.
These opt-in routes add inert snapshot configuration and one local module; the native
routes remain module/config-free. Local bio/country feedback and same-node Reset never
authorize a submission. See [enhancement/replay evidence](../../2026-09-14-catalyst-enhancement.md).
Run `enhanced-browser.mjs URL PRIVATE_PARENT` for four responsive/theme lanes and
withheld-module fallback; `seam-browser.mjs URL PRIVATE_PARENT` for adversarial
configuration, repeated instances and lifecycle. Both use `APPKIT_PLAYWRIGHT_MODULE`
as above, freeze plans before interaction and retain failed receipts.

```sh
node --test internal/designadapt/catalyst_enhance_test.mjs
```

Generation resolves `go tool -modfile <KIT_GO_MOD> templ` inside the private output
module. This keeps pinned tooling and module-relative error locations together;
generating from the kit root leaks temporary directory names into Go and binary bytes.
The recovery-only modules matched fourteen files. The later native CSS companion adds
`catalyst_native.go` and `catalyst_native.css`; its two builds match all sixteen files.
Serve `SettingsNativeStyles()` after source CSS, as `/native.css` in the fixture.
It owns native focus/size rules; do not discard it when adopting generated form output.

## Backend refusal and real browser zoom

The fixture optionally accepts `--refuse-first-validation`; normal launches never
enable this synthetic failure. Only the first valid validation refuses503. Invalid
fields, framing/origin/CSRF failures and Reset do not consume it. The response retains
all ten fields and focuses the retry status; native Enter can retry after correction.

```sh
APPKIT_PLAYWRIGHT_MODULE=/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs \
node docs/assessments/fixtures/catalyst-settings-adapter/recovery-browser.mjs \
  /Volumes/m2ext/code/geoffmilleraz/app-kit/data/design-assets/p10-go-1747876674/fixture \
  /Volumes/m2ext/code/geoffmilleraz/app-kit/data/design-assets/p10-source-ohWeRq \
  /Volumes/m2ext/code/geoffmilleraz/app-kit/data/design-assets

APPKIT_PLAYWRIGHT_MODULE=/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs \
node docs/assessments/fixtures/catalyst-settings-adapter/zoom-browser.mjs \
  http://127.0.0.1:49653 /Volumes/m2ext/code/geoffmilleraz/app-kit/data/design-assets

node --test docs/assessments/fixtures/catalyst-settings-adapter/zoom-capture_test.mjs
```

Revalidate the actual bound URL before use. Recovery owns one isolated fixture process
per case; zoom uses the selected normal server and fresh Chrome profiles. The zoom
runner selects Chrome Appearance200%, not CSS/CDP page scaling. `zoom-capture.mjs`
requires measured2x CSS-to-DIP bounds, complete form/final-action containment, exact
PNG dimensions, CRC integrity and complete pixel rows. Requires Node's `zlib.crc32`
(tested Node26.3.0). No additional package is installed. See [current evidence and
retained failures](../../2026-09-15-catalyst-recovery-zoom.md):114 recovery,76 zoom and
182 regression checks. Captures remain private and are not source-fidelity certification.

## Painted focus and multiline transport

`focus-browser.mjs URL PRIVATE_PARENT` freezes scoped pixel/text/AX checks across
390/1440, both themes and native/enhanced modes. It uses the same approved isolated
Playwright module and its existing PNG decoder. [Focus evidence](../../2026-09-15-catalyst-native-focus.md)
keeps old/new receipts, sixteen-file replay, text/pixel sampling limits and concurrent
autofocus-observation failures separate. Run browser suites serially for investigation;
serial passes do not prove that the unresolved concurrency failure was environmental.

`newline-browser.mjs URL TARGET_BINARY PRIVATE_PARENT` checks native POST at multiline
Unicode limits, CRLF transport, exact leading/trailing-newline retention and count/error
agreement. Its output stays private; [INC-30 evidence](../../2026-09-15-catalyst-textarea-transport.md)
retains the160-check/52-failure baseline and160-check repaired pass, plus122 recovery
checks. Source-only focus/capture tests run without paid assets.

## Descriptive payload acquisition

`payload-browser.mjs SOURCE_ORIGIN SOURCE_ROOT TARGET_ORIGIN TARGET_BINARY SOURCE_PACKET
PRIVATE_PARENT SOURCE_PID TARGET_PID` uses existing verified processes only. It neither
starts a server nor installs dependencies. Use the same approved Playwright module.
The collector verifies packet/source-file identities and live listener/executable paths,
keeps resource encodings/cache/decoded-body observations separate, and writes each
sample privately. All cold lanes precede warm attempts; first failure stops acquisition
and leaves unvisited samples not run. It is not a latency or comparative-speed tool.
The [first partial evidence](../../2026-09-15-catalyst-payload-observations.md) records90
usable phone cold samples, source no-store and desktop prefetch-cancellation gaps.
Do not change cache headers, disable source prefetch or ignore failures to make it pass.

Append `--diagnostic-desktop` to the same command for a separately identified maximum
of five cold1440px/light source observations, stopping at the first failure. This does
not replace the default360 matrix. The collector now retains phase/event chronology,
post-close resource totals and distinct policy/transport failure channels. It does not
reinterpret cancellations as passes. The [first diagnostic](../../2026-09-15-catalyst-payload-phases.md)
observes22 cancellations before teardown; the source cause remains unproven.

`--diagnostic-stream-cancel` is a separate instrumented lane, never a passive
performance sample. It observes only bounded stream/reader cancel calls, not per-request
causality; the [first failed trace](../../2026-09-15-catalyst-stream-cancel.md) remains retained.

## Separate cached-static delivery experiment

The generated fixture accepts `SOURCE_PACKET --cached-static-delivery=p10-cached-static-v1`;
it cannot combine that selection with backend-refusal mode. The source-side companion
is `../catalyst-native/cached-static-reference-server.mjs SOURCE_ROOT SOURCE_PACKET`.
Use explicit absolute paths and new listeners; never replace the no-store reference.

`cached-static-smoke.mjs` takes the same eight positional inputs as `payload-browser.mjs`,
but requires the separately selected cached source/target commands. It verifies the
16 generated artifact hashes and43 source pins, checks raw HTTP identity/gzip/HEAD/304
and refusal behavior, then collects at most six390px/dark cold/warm samples serially.
It saves metadata only and stops on the first failure; no default360-matrix substitution.
The [first smoke](../../2026-09-15-catalyst-cached-static.md) passes688 HTTP/32 exclusions,
then fails source font cache membership because actual CSS URLs include `?v=4.1`.
The query-free packet and v1 receipt remain immutable. Select the separate Go
`--cached-static-delivery=p10-cached-static-v2`, source
`../catalyst-native/cached-static-font-alias-reference-server.mjs SOURCE_ROOT SOURCE_PACKET`,
and smoke `--font-query-aliases` for38 exact CSS-declared aliases. New source/target
listeners must expose the same alias-manifest SHA;43 assets provide81 delivery URLs.
The [first v2 smoke](../../2026-09-15-catalyst-font-alias-delivery.md) passes1,296 HTTP/62
exclusions and three cold samples, but fails source warm cached-font body observation.
Actual cache events do not justify claiming unavailable font bodies were retrieved;
two remaining warm samples were not run. No full warm/performance gate is earned.
V2 smoke now also emits separate `warm-identity-*.json` diagnostic records from exact
prime/reload lineage, cache events, representation validators and pinned-wrapper errors.
[The first result](../../2026-09-15-catalyst-warm-font-identity.md) links two source fonts;
strict samples remain failed and their byte/body totals unchanged. Missing wrapper or
linkage evidence yields supplemental `unavailable`, not a replacement pass or a new
base-smoke requirement. Replay uses `warm-font-identity.mjs`; it never refetches a body.

## Boundaries

Capture checks selected render-asset responses by digest. Other JavaScript/dynamic
responses are checked for status only, not attested as an entire reproducible build.
The original ZIP digest is checked; the original member identity comes from the
previously frozen pilot, not a fresh extraction in this collector. Setup can fail
before a browser report exists; a directory/plan alone is never success evidence.
The packet is not a delivery API, full vendor license package or certified target.
Broader native browser tasks, physical input/contrast/performance, complete fidelity,
CLI/catalog exposure and real consumer integration remain separate acceptance work.
