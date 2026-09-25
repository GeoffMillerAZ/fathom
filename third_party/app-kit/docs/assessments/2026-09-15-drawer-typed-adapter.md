# P08 typed profile drawer: source, delivery and browser evidence

## Result and boundary

Frozen P08 now generates an unchanged `DrawerReference` and typed `NewDrawer` with
app-owned identity, image semantics, definition-list details and optional navigation.
No messaging/call/clipboard/presence backend is inferred. [Guide](../design/profile-drawer-adapter.md)
and [ADR-0053](../adr/0053-explicit-profile-drawer-and-app-owned-navigation.md) define
the private adapter. Compiler/replay, CLI checks and 229 scoped Chrome checks pass; full target certification
and actual consumer adoption remain open. Legacy 439/657 coverage is unchanged.

## Retained compiler and delivery evidence

Private root: `/Volumes/m2ext/appkit-evidence/appkit-drawer-proof.o8ATJ5/probe.vD9UBX`.
The command-scoped TMPDIR is its parent; no global cache/temp setting changed.

| Evidence | SHA-256 |
|---|---|
| Original member | `1fbde54a0be874618cc295bf3b837fee4d89f38a1543dd9a6e2374bbd1d13415` |
| `delivery/artifact.json` | `34644520c31c87de479b4c61472b6ea3fb0e759f7c757db2fbb291fd9b2883ca` |
| Composite adapter rule | `f05f3bfce1e8743fa4652e41ba9fab4c951ff07591400eab3a5295a754e6a04a` |
| Generated template | `6c7a5cbd3430df1e63e6b18a50bc9e2ea799780e6235919c905265a370d4d832` |
| Generated model | `6bd90ebd0d764682cc3127eb867293229e1e84fb4bd2d332d156a04a03cb962b` |
| `compile-receipt.json` | `ca83dcba88f43e5145d672b83fe182ad49040417800f8abd1688426a0e42bc59` |

`TestAdaptDrawerLive` passes with original archive/catalog hashes checked before and
after. Both unformatted/formatted templ compilation phases pass exact original/
reference DOM comparison and typed model/DOM race tests. Five rendered specimens
cover typed, read-only, long and multiple-instance states plus the reference.
The receipt binds source, artifact, runtime, models and rendered files. No original
copy, class list or icon path is pasted into tracked adapter implementation.

`TestDrawer*` synthetic race checks pass, including unknown source/shape/text-binding
refusals, invalid URLs/calendar dates, optional states, escaping/snapshotting,
writer/cancellation behavior and immutable delivery inventory/drift controls.
`TestAssetsAdaptDrawer*` live CLI tests pass: repeated JSON, five 0600 files in a
0700 directory, silent export/check, no-overwrite refusal, missing/drift exit 1 and
invalid/source-unavailable exit 2. This is in-process dispatcher coverage; the
separate compiled-command replay below adds execution evidence, not a clean consumer.

Logs retained in the parent: `synthetic.jsonl`, `live-final.jsonl`, `cli.jsonl`,
with empty matching stderr files. `live-initial.jsonl` and `probe.6F20kN` preserve
the earlier implementation; the identities above select the final compiler run.

## Compiled command and shared discovery

The privately built `appkit-drawer-probe` binary in the parent has SHA-256
`cd2950ee15ef9672e3481c1bc0558878a643ef65328859f82e3959a2657f5533`.
Its `assets-adapt-drawer -out` produces `cli-a` and `cli-b`, each silent exit 0.
Both `-check` commands return 0 with empty stdout/stderr. Exact file inventories,
0600 single-link regular files and all five content hashes match one another and
the compiler probe's immutable delivery. No global installation or app build occurs.

The shared CUE catalog now has 33 records, including eight experimental private
typed adapters and the separate Elements translator. MCP/Gallery tests prove exact
metadata and revision parity without paid archives, bounded search, private-source
exclusion and refusal of unsupported web/available/proven promotion. The new guide
and ADR resolve to their exact current documents through MCP.

`packages-final.jsonl` records race-test passes for internal/designadapt, cmd/appkit,
appkit/catalog, cmd/appkit-mcp and apps/gallery: 1,362 passing test/subtest events,
21 explicit skips and zero failures. The skipped private/live gates are not passes;
P08 source/compiler and CLI live tests above ran separately. The first discovery
run retained outdated Gallery count and incorrect ADR-resource-address test failures;
the final tests use eight entries and the actual ADR tool contract, not a new URI.
Selected document checks cover 424 local links across ten files, router/scope/new
ADR/index budgets, equal Fathom stanzas and 60 trimmed Unreleased lines. This is a
structural subset, not a full intent-doctor or host-routing claim.

## Browser evidence

The maintained [browser harness](fixtures/drawer-adapter/browser.mjs) uses the
existing isolated Playwright install, a fresh owned Chrome process, pinned Elements/
Inter/Tailwind inputs, loopback-only synthetic routes and four 390/1440 light/dark
lanes. Original external profile imagery is deliberately unavailable through CSP;
the typed fixture uses an authored local image. No photo-fidelity claim is possible.
Final receipt: `browser-fXfoC6/receipt.json`, SHA-256
`2d7869414fbdbfac61200ae47d9caeaec85fc96c52947718466fa73f04618c2b`.
Harness SHA-256: `f939dfd904dae4920958a9f4bedfb114e5926c1fe6f24121e18186717d6b93d6`.
Chrome 152.0.7977.84, 390/1440 × 1000 CSS pixels, light/dark and reduced-motion
preference. Input is browser-protocol keyboard/pointer/wheel, not OS or physical input.

All 229 scoped checks pass with zero page/probe errors. Twenty-four captures include
eight exact original/reference closed/open pairs under the same missing-image
condition. Sixteen file inputs, 38 retained outputs and all captures were independently
rehashed after the run. The owned Chrome PID is absent and the loopback listener
closed. The owner's existing RAC preview still returns HTTP 200.

- Original/reference and typed Tab/Enter entry, nested ArrowDown, menu Escape returning
  to its trigger without closing the dialog, then dialog Escape returning to its opener.
- Modal Tab containment, pointer close, four real native GET navigations to explicit
  synthetic result pages, read-only action absence and multiple-instance targeting.
- Long-content panel bounds plus selected heading/name/action/final-detail element
  and Range text rectangles inside their horizontal scroll clip. Normal browser wheel
  input fully reveals the final detail text. This does not measure every glyph/state.
- Eight original-image CSP refusals with no external route requests. All 272 recorded
  loopback responses use the pinned fixture/asset closure. No backend effects occur.

Desktop dark typed output and narrow light long-content/end-of-scroll captures were
visually inspected. Source styling and image cropping remain deliberate references;
an authored synthetic photo cannot establish fidelity to the original photography.
No P07 repair module, product rewrite or relaxed visual threshold was injected.

Earlier runs remain retained: `browser-R2Dn15` stopped prelaunch because a generic
input size guard rejected the pinned compiler; `browser-MZN0uQ` sampled unfinished
drawer animation; `browser-fJxhKW` included a data favicon in the image-refusal canary.
The harness now bounds the compiler separately, waits for three settled animation
samples and serves its favicon locally. `browser-FPRrQz` (181 checks) and
`browser-SSJpXB` (229) predate the final text-range coverage. None replaces the final
receipt or hides a product failure. Original and delivered component bytes stayed fixed.

## Remaining gates

Original-photo fidelity, complete focus/contrast, touch/screen readers,
adverse network/runtime failure, dynamic lifecycle, performance, standalone
CSS/asset delivery and real consumer integration remain open. Artifacts stay not-verified;
private CLI availability must never be mistaken for certified renderer support.
The unchanged base's actual200% task checks pass below, but its visible-focus screening fails.

## Standalone consumer render replay

The maintained [rehearsal](fixtures/drawer-adapter/standalone.mjs) and
[synthetic consumer](fixtures/drawer-adapter/consumer_test.go.txt) pass in
`/Volumes/m2ext/appkit-evidence/appkit-drawer-proof.o8ATJ5/drawer-standalone-Y1Gg9N`.
Receipt SHA-256: `2296b2083a8d046993aa46574fdfab2e5dad7cb19851c20ff17166a1e425b68c`.
Harness SHA-256: `b3f373527fd1c11068d2dee127f922d8d284067bb2a66b48174deadd785f97b2`.
Consumer test SHA-256: `a9cac81e1567ffc7fa784f42dcd5b6637c193ab137dc3d0b74b2e55f0359644c`.

Two independent empty module/build caches use Go1.26.3, Node26.3.0, templ0.3.1020
and the existing pinned public-module fixture. The macOS sandbox denies network,
kit-parent source reads, original probe reads and public ZIP-cache writes. Source/
delivery-read controls fail with permission denial; an independently reachable owned
HTTP canary fails under the sandbox. Public-cache write denial is configured, not
tested by attempting to write the owner's cache. Local ZIPs verify against go.sum.

All 26 commands complete with their expected results. Both named consumer tests run,
compare typed output byte-for-byte with the compiler/browser oracle, refuse unbound
IDs/images and invalid calendar dates, preserve read-only output and snapshots, and
isolate repeated IDs. Withholding generated code makes compilation fail with undefined
symbols and produces no binary. Renderer module discovery contains only the consumer
and templ; the test's HTML tokenizer and compiler dependency closures are recorded
separately. No app-kit module or local replacement enters any selected closure.

| Reproduced output, each run | SHA-256 |
|---|---|
| Generated templ Go, 233,931 bytes | `4c2a5a5193e58cfaf501e6ce227799f19ec0403bd15d5f78d59a7d3a8fadd4b7` |
| Typed HTML, 7,335 bytes | `12b5c12c3aad0dd52f80e7ee62e1373962d992ee4cce5cab8c1485a14aaada72` |
| Read-only HTML, 4,545 bytes | `5857a073078cb56729ee0b418c2170fca97ce40b91d9910b284e3b571dc615f6` |
| Compiled consumer test, 7,146,354 bytes | `bc5e0c9380d476b0f6ee0a0eec89dbbb6b91c3eb53ae9d2872d037fb1d97ed40` |

Delivery checks remain silent/matching before and after. Input/tool/fixture/output
and command-log hashes were independently rechecked; the owned HTTP canary closes.
This is compiled synthetic rendering, not CSS/runtime delivery or a real app migration.

Retained `drawer-standalone-Dq1gDf` failed because a substring test mistook a preserved
HTML comment's example script tag for an executable element. The final oracle uses
HTML tokenization, with canaries distinguishing comments/escaped text from actual
script/form tags. Component, source, defaults and expected rendered bytes did not
change to satisfy that test.

## Actual200% zoom and retained focus failures

The separate [zoom diagnostic](fixtures/drawer-adapter/zoom.mjs), SHA-256
`ef39958d6ab627f592eebdd9db1d7d6c0d6862d71337359949805bb9ed8bedec`,
uses fresh Chrome Appearance zoom2, not CSS zoom or viewport emulation. The normal
receipt pins its source/rendered inputs. Typed/long light/dark lanes observe
innerWidth720, outerWidth1440, DPR2 and visualViewport.scale1 throughout.

Retained `probe.vD9UBX/zoom-o8SYrW/failure.json`, SHA-256
`6c4fe2312cdc0473a2ed5f1c220db8cf4a03baa4d6e1b8b03c9d37b56a16d297`,
records144 passes,30 focus-screening failures, zero runtime errors and eight captures.
All measured zoom, text-reflow/reachability, nested interaction and native GET tasks
pass. A noninteractive full-dialog DIV is a legitimate contained entry-focus target,
recorded separately rather than misclassified as an unpainted interactive control.

Twelve nested menu-link observations have no outline. Eighteen opener/menu-trigger/
secondary-link observations use thinner1px browser-default cues and fail the declared
solid2px/3:1/unclipped kit screening policy. Those cues are not all invisible; this is
not a WCAG certification result. Close controls pass all four lanes. The earlier
`zoom-hLeteJ` diagnostic incorrectly compared outward primary-link outlines against
their own fill; the corrected run uses the adjacent parent backdrop and accounts for
popover top-layer clipping. Both failures remain retained. Owned Chrome PIDs exited
and the local listener closed.

This fires ADR-0053's nested-focus review trigger. A separate internal scoped CSS
candidate uses explicit app-owned wrapper selection; the five-file base and Elements
interaction ownership remain unchanged. The explicit repair below must not replace
this baseline or imply source-photo fidelity or consumer acceptance.

## Explicit CSS-only focus repair

[ADR-0054](../adr/0054-explicit-drawer-focus-delivery.md) records a source-specific
stylesheet selected by an app-owned wrapper, without a new constructor or JavaScript.
It gives selected controls 44px minima, wrapping and inset 2px outlines. The nested
trigger uses the source's gray900/100 tokens and `prefers-color-scheme: dark` authority;
its text/icons, the already-passing close control and Elements behavior stay unchanged.

Final CSS SHA-256: `9b1fcd1b50999d5d82878e0444c55ad097a4755fd57df2057f2ab25bb453ffe8`
(1,173 bytes). The same maintained [repair harness](fixtures/drawer-adapter/repair-zoom.mjs),
SHA-256 `824f4f35f5419ed211f106dcdca928d6118355e4ef6b2bac4925db29a5b74709`,
binds both final runs under the original `probe.vD9UBX`:

| Run | Scoped result | Receipt SHA-256 |
|---|---|---|
| `repair-zoom-rkOoJM/receipt.json` | 182/182, four focus captures, actual Appearance200% | `d0340da7e92c7cd2dd520814d81c3473c125ffe7b559eabd5b2078a7d1151ccc` |
| `repair-normal-MCn4as/receipt.json` | 360/360, eight focus captures, 390/1440 CSS-pixel widths | `d7a60f1743214f19fd62a7e47bf4850008972d10592efd4975b87d8415dd953e` |

Both cover typed/long fixtures in light/dark, keyboard entry and nested Escape/focus
return, selected text bounds/reachability, four native GET routes, touched-target size,
solid-outline thickness/contrast and clipping. Short content must be visibly contained;
scrollable content must reach its final detail through wheel input. No hidden overflow
or shortened fixture substitutes for that task. Trigger-ring contrast measures17.747:1
light and9.689:1 dark. These are scoped screening results, not WCAG certification.

Raw browser facts feed the same pure extent, source-over and clipping functions as
the canaries: inset outlines use their painted element backdrop, outward outlines use
the adjacent parent; translucent descendants paint over ancestors; a top-layer menu
escapes ancestor clipping but retains its own clip. No assertion threshold was relaxed.
Original baseline source/receipts/harnesses remain untouched. All twelve inputs per run,
output/capture hashes and final receipt hashes were independently checked. Selected
zoom-long/light and narrow-typed/dark captures were visually inspected. Installed Chrome
metadata remains152.0.7977.84; this is headless browser-protocol input, not physical input.
All four owned Chrome PIDs exited, listeners refused connections after cleanup, and
neither run records a runtime error. The repair CSS has16/32 recorded requests respectively.

Retained failures, not substituted baselines:

- `repair-zoom-2xssMl/failure.json`, SHA `417514ff380c85ac58a0ccbd7f00ebd0485809e0569db4f008c4b3127e3b12f9`:174/8, zero runtime errors. The first CSS inherited black into the dark trigger; two additional Escape checks sampled incomplete closure. Paint and wait sequencing were corrected separately.
- `repair-zoom-zTCmQn/failure.json`, SHA `aae0ca8ad75b8f8db86d02bcb953e12889c20aec05a6ba0709c0fbeed03db96c`:178/4, zero runtime errors. Source gray400 still gave only2.602:1 on the light trigger; explicit source-token outline colors fix this measured gap.
- `repair-zoom-xdNgjq/failure.json`, SHA `36accb1005f0cf8fffa5abdcaaa4f25c915beebd2e1570a68be39654e5000d31`:178/4 and an input-changed error. CSS changed during the run; it is invalid candidate evidence.
- `repair-normal-tqXUgz/failure.json`, SHA `9dac156294b5a0f076b6a6eeb246342915ca40f7abf20c5c3fb8e5cf9a0088cd`:352/8, zero runtime errors. The zoom-oriented oracle wrongly required scrolling for short, fully visible normal-view content. Final checks retain both containment and actual-scroll branches.

## Repair delivery and unchanged default

`assets-adapt-drawer -repair` now selects the nested not-verified artifact and explicit
six-file export/check. Default JSON, explicit `-repair=false` and the five-file base
remain byte-identical to the previously recorded artifact. No generated Go/templ/default
content changes. No dependency, source archive, runtime, consumer or global setting changes.

The [maintained CLI rehearsal](fixtures/drawer-adapter/focus-delivery.mjs), SHA-256
`49f265b0194a9fc6be882e866b2a51fffcc3368159e8fe2e9a917be940f43165`, runs the actual
binary SHA-256 `7ea28663c1d7d7979098ba8dbc3707b993ad7b61357abc2c8785dc6a6c104369`.
Private receipt: `/Volumes/m2ext/appkit-evidence/appkit-drawer-focus-delivery.I4u61jyb/focus-cli-CTgXcJ/receipt.json`,
SHA-256 `3fde6b27fd963d2109d9d6d4f64e0a718561f45c784f6d7b4bc218b981824f8d`.
All22 commands return expected exits/output. Two private exports match exactly; wrong
variant, changed/rehashed CSS, extra-member, permission drift and no-overwrite controls
are refused. `repair-a` and `base` remain valid; `repair-b` deliberately retains
`extra.txt` as a negative control and must not be used as a matching delivery.

Repair artifact SHA-256: `d19692bceab358def7124107d39ec25f2512d214d41e0f4c3c78ad1690d16256`.
Rule/schema SHA-256: `326294218671e7ef7a61d687062bf3fe79f20ee0164989fb3b6330ed4dc616db`.
Usage SHA-256: `3dcb9c294a05c85e14f7ef9ee8913e1f7ff10d8bc982048b800e461823b772a2`.
The CSS identity exactly matches the browser pair; nested base identity remains
`34644520c31c87de479b4c61472b6ea3fb0e759f7c757db2fbb291fd9b2883ca`.
Inputs, command logs, valid exported members and embedded rule/schema identity were
independently rechecked. Focus/replay/private-source and CLI live tests pass with race
detection; direct CUE schema vet and selected MCP guidance/index/protocol tests pass.
Independent read-only review found no delivery/style ownership defect. These checks
do not prove a standalone CSS/runtime build or real app integration. Original-photo
fidelity, other engines/input methods, complete accessibility/lifecycle/performance
and consumer acceptance remain open; eight typed adapters and zero certified targets
are unchanged counts.
