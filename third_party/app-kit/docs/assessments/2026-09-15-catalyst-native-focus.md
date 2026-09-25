# P10 painted native focus and bounded accessibility evidence

Follow-up to [recovery/zoom](2026-09-15-catalyst-recovery-zoom.md), within
[ADR-0044](../adr/0044-native-catalyst-settings-composition.md). Private experimental
form only; no catalog-target promotion, consumer migration or WCAG certification.

## Defect and repair

The retained Headless styles depended on framework focus attributes that native
controls never supplied. All twelve controls accepted keyboard focus, but88 of96
focused states had no outline or changed wrapper ring; eight checkbox states used
only a1px browser outline. Matching `:focus-visible` did not prove a visible indicator.
The initial prospective collector `p10-focus-browser-IzJUW2` recorded560 checks and96
failures; its computed-style evidence alone was insufficient to prove actual paint.

A generated first-party `catalyst_native.go` embeds `catalyst_native.css` and exposes
`SettingsNativeStyles()` within the private fixture module. This is not an exported
app-kit UI API. The fixture serves it at `/native.css`, after retained source CSS.
Four form-scoped rules own44px control/checkbox-label minimums, a2px CanvasText outline
with2px offset on keyboard focus, and a focused retry-status outline. No source palette,
default layout, controller, canonical options or native POST policy was changed.
GET/HEAD/method/host and reserved-asset-route tests cover stylesheet delivery.

## Frozen pixel oracle and replay

Independent review required local screenshot evidence, form-owned accessibility nodes
and honest text-sampling limits before accepting the focus oracle. The strengthened
collector samples twelve straight-edge outline/background pairs around each focused
control, checks expected ink and local contrast, and rejects off-capture coordinates.
Three source-only tests include missing/occluded outline, low local contrast and invalid
RGBA canaries. The pinned PNG decoder was already present in the isolated test harness;
no application dependency was added.

| Private directory under `data/design-assets/` | Result |
|---|---|
| `p10-focus-browser-YvkYP2` | Old binary:656 checks,192 failures (96 computed outlines plus96 pixel checks). |
| `p10-focus-browser-TNf4Im` | CSS companion:all656 checks pass, no reported browser errors. |

Both use plan SHA256 `babae3478629feba09823bfaeb87b5fc9581a114901e4799be5d297cc36ccddf`.
Old report: `5e07379d7cd109739fb22b288c926dbf469f8f942ea25527ac83fee43ef556ed`.
Repaired report: `92b01cc26be47d3af662690a423d0760a36b7662adf523312c672f968acdbb52`.
Initial computed-only report: `46acd58af6626e15b3e05a87a264cc362401ad452fcfb8d0bb2ffd877ce30ea6`.

Chrome152.0.7977.84, Playwright1.63.0, Node26.3.0; eight390/1440 light/dark native/
enhanced lanes, reduced motion. Native application JavaScript is disabled. The96
keyboard states supply1,152 pixel pairs, minimum local contrast18.7337:1. All twelve
form-owned accessible names/roles and selected invalid/error descriptions are checked.
The240 computed heading/description/label/count/error/status text samples have minimum
contrast4.770997:1 against their measured flat backgrounds. Comparisons use unrounded
ratios; unsupported background layers fail measurement.

This samples straight outline edges, not every corner or pixel. Computed text-color
contrast is not glyph visibility, occlusion, placeholder/value contrast, popup contrast,
screen-reader operation or full WCAG conformance. See the standards for
[text contrast](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html) and
[visible focus](https://www.w3.org/WAI/WCAG22/Understanding/focus-visible.html).
The2px/offset contract is this pilot's chosen repair, not a claim about every AA rule.

## Standalone and regression evidence

Modules `p10-go-2558988874` and `p10-go-3821576029` match all sixteen recorded files;
both manifests and every actual file digest were independently compared.
Manifest SHA256: `d61a98176df20b397ec13061725c7aa81da8aab9081854a64d081a1c2e418f7f`.
Binary SHA256: `fdafadab6c096feb37861c3de2564ae33ed203dd6d171e79b13d640a8341346b`.
The source packet remains `p10-source-ohWeRq`, reference SHA256
`94d3ab4c145e33bf29af750918429833d833fff3f9ac8820e1314c7f2c67f53d`.

| Regression directory | Checks | Report SHA256 |
|---|---:|---|
| `p10-native-browser-5mocrj` | 78 | `a305eae9092ea87246fa87c280599a1b515030c2ea9f6c053b0392301fcf4f9a` |
| `p10-enhanced-browser-9hOG3B` | 81 | `2162455662fbe4ebba3ba14aa7c793e6c55d5fd1fa49654be2753119d6d226e6` |
| `p10-seam-browser-l7HnWf` | 23 | `89af921bb1c925729fa117cafd560494ccebf516782ab0f3020c2220ade2f68e` |
| `p10-zoom-browser-n5DC5V` | 76 | `0d4dd1f1293def77c59c5706519212b3172ddc28b60d08f6909a85179b86343e` |
| `p10-recovery-browser-KuEZyl` | 114 | `4d4ffaf3d4247d2830dfea47cf77e68aecc66c4867537a8aa48d3c9c5113e7cc` |

These372 checks pass on the recorded candidate. Each native/enhanced/zoom suite's four
default PNGs matches its previous baseline exactly (twelve pairs total). Focus/error
images intentionally change. Mobile native-name and desktop enhanced-Validate outlines,
plus mobile dark and desktop light503 frames, were visually inspected. Form-cropped503
images cannot prove outline pixels outside that crop. All eight recovery processes exit0.
Source-only race tests passed for designadapt28.708s and MCP23.878s, plus vet both;
fourteen focus/capture/controller Node tests pass. The complete `task check` was not run.

## Retained concurrency failures and remaining work

Five browser suites initially ran concurrently. Zoom `p10-zoom-browser-WDMeL0` failed
at13 checks and recovery `p10-recovery-browser-PilWlt` at6: native422 retained the exact
draft/errors but the fixed50ms focus observation saw the body. Reports remain retained:
`65e9d9e12cb43c4f56655a35b1f958cb40cbee65e3bdf5e6513c1172464c712f` and
`7ed11ee41e2f4c2315ee05a6cb6620b2e2db248f82589d1459ecf7e632a9b800`.
Unchanged serial reruns above pass. No delay, autofocus assertion, source or CSS was
changed between those failed/passed runs. Scheduling/foreground gating is a hypothesis,
not a confirmed cause; the failures do not become environment passes. A paired
foreground/lifecycle diagnostic remains open. No concurrency-stability claim follows.

Visual review also found a native/enhanced multiline-bio count mismatch. The separate
newline regression investigation retains these focus receipts as historical candidate
evidence; they do not imply complete form correctness. Full source fidelity, physical
input/accessibility, [measured performance](../design/catalyst-settings-performance.md),
private delivery and consumer adoption remain open. Counts remain439/657 compiler/DOM,
five catalogued typed adapters and zero certified targets; G1–G6 remain active.
