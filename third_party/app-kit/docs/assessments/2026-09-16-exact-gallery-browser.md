# Exact independently built Gallery: browser workflow acceptance

Date: 2026-09-16. G2/G4/G6 progress under I8/I9/I12/I15/I18/I19 and
ADR-0026/0033/0045/0047. The initial exact-candidate rehearsal changed no application
binary/source; the later working-tree renderer repair is labeled separately below.
Neither phase changed consumers, dependencies, global configuration, signed-in profiles,
deployment, native windows or releases.

## Result and scope

The [maintained opt-in runner](../../tools/releasecandidate/README.md#exact-output-browser-tasks)
executes the exact server from the [independent two-cold-build rehearsal](2026-09-15-independent-gallery-host.md).
It retains and runs its own six-file source/test closure, parent receipt and server
outside both fleet source roots. Server networking permits only localhost and source
reads are denied; actual source-read and external-network refusal canaries pass.
The original server and embedded candidate CSS are unchanged.

Final V7 passes **176 assertions**, including 64 served-CSS identity checks, across
four fresh synthetic sessions. All 24 PNGs have retained hashes; twelve full-document
captures are paired with twelve task-viewport captures. No converted target is certified.

| Variant | Viewport, CSS pixels | Page scripts | Assertions | Exact CSS responses |
|---|---|---|---:|---:|
| Desktop light | 1440 × 1000 | enabled | 48 | 17 |
| Narrow dark | 390 × 1000 | enabled | 48 | 17 |
| Plain desktop light | 1440 × 1000 | disabled | 40 | 15 |
| Plain narrow dark | 390 × 1000 | disabled | 40 | 15 |

All variants exchange a one-use launch capability and refuse replay; use Tab/Enter
for the skip link, profile typing and saves; retain invalid 81-character input with
422 feedback; refuse a stale write with409 and show both the retained draft and newer
saved value; apply an explicitly reviewed draft; retain a503 draft and successfully
retry; save UTC/Activity settings and observe them after the native POST redirect.
Settings fits the selected viewport with one main landmark. Fixture setup and select
controls use Playwright APIs; this is not a claim of exclusively keyboard operation.

Script-enabled variants connect the real Datastar Activity observer, receive another
window's save, stop/reconnect explicitly, then disclose potentially stale data after
SIGINT shuts down the exact server. Separate same-session windows keep the observer
visible; the application intentionally pauses hidden tabs. All four server processes
exit0 without a signal after graceful shutdown. No native companion is launched.

Application requests are allowlisted; unexpected requests, console errors, page errors,
dialogs and WebSockets are zero, including the post-context-close checks. Each variant
has no handoff/staging residue. Managed browser cleanup is confirmed, its private temp
directory is empty, and retained binary/harness bytes remain unchanged.

Host: Darwin25.6.0 arm64; Chrome152.0.7977.84; Node26.3.0; the previously installed
isolated Playwright1.63.0. The 22 Node tests pass in both checkout and frozen V7 harness.
An independent audit rehashed the parent receipt/server, six source/test files and all
24 PNGs (2,632,428 bytes), and confirmed checkout/frozen equality and cleanup records.
Focused documentation checks pass375 relative links across16 selected files, all root
intent budgets and the60-line Unreleased budget; scoped diff whitespace is clean.
This is the link/budget subset of intent-doctor, not a full wiring or semantic audit.

## Visual inspection and limitation

Seven final task-viewport PNGs were inspected: desktop conflict/settings/live, narrow
settings/stopped observer, plain-desktop settings and plain-narrow conflict. They show
the retained draft, saved preferences and truthful stopped-observer status with the
original fixed navigation intact. This is sampled inspection, not whole-UI certification.
Full-document screenshots can place fixed mobile navigation over intermediate content;
paired viewport captures center the named task control and check its unobscured hit
target. No CSS is injected to hide app chrome or improve the comparison artificially.

**Historical visual finding:** plain-desktop Settings retains “Appearance controls are
loading” alongside the no-script explanation that appearance controls are unavailable.
Core saving works, but this permanently unavailable enhancement should not imply that
waiting will load it. The V7 workflow assertions do not cover that copy contract.
The shared source is [appearance.go](../../appkit/ui/appearance/appearance.go), whose
initial live status and adjacent no-script message currently render together.
Do not describe the present result as complete no-script UX acceptance.

### Working-tree appearance repair

The shared `appearance.RenderControl` now supplies one initial status: “Appearance
controls require JavaScript.” The duplicate no-script paragraph is removed. Native
select/retry state, role/ARIA hooks, API, CSS, storage keys, theme defaults and JavaScript
runtime bytes are unchanged. Once mounted, the existing controller replaces the fallback
with actual saved/session/error status. No consumer is silently upgraded.

`TestRenderControlHasOneTruthfulScriptFallback` fails against the retained old source
through a Go overlay (`fallback status = "Appearance controls are loading."`) and
passes the repair. The controller's13 Node tests include mounting from this fallback
into saved/session/unavailable states. Race tests pass for appearance and Gallery.

The existing `shell_forms.mjs` now arms form navigation before submission and asserts
the initialized status plus four no-script width/theme combinations. A fresh working-tree
Gallery passes143 browser checks with no failures; its maintained broader save/conflict/
retry/permission/theme/no-script tasks still pass. Desktop-light and narrow-dark native
viewport captures were inspected: the single fallback is visible without fixed-navigation
occlusion. This is a separate local-build repair, not a replacement of the frozen V7
candidate or its historical screenshots and not a new cold-build/adoption proof.

Private repair root: `/Volumes/m2ext/appkit-appearance-status.DPvXp3F5/`.
It retains the Gallery binary, six tested source/config files, old-source overlay, source
manifest, browser report and14 PNGs. Source/harness hashes match before/after browser use.
Independent review rehashed every selected source, binary, report and image. The owned
test Gallery was stopped by SIGINT and exited0; existing previews were untouched.

| Repair artifact | SHA-256 |
|---|---|
| `receipt.json` | `14e1b3920d35dff3718d0664e409baa601c7c3b7defd254715e0df2a8c7c2c08` |
| Gallery executable | `7015c6250b0b215cb7724372dcbbf0883d242890f6428702412b99adfbb974e6` |
| `appkit-shell-browser-dcPcAN/shell-forms-truthful-status-report.json` | `e0186c13adb38e64451d10e0fa9318216fa6cd79f3d55a0a23704bc2e15bbc2e` |
| `appearance.go` | `4fb9b67b3b52c1959a35c9a1968d8aa2c48c233c04aea46abf5c94d90cbbb199` |
| Unchanged `appearance.js` | `6f0345b057fcd1d94e5502ac766d23cb266640c2e0fb1fb1a35e39a835eb8aff` |

The frozen candidate still contains the old fallback. Rebuild or update a selected kit
artifact to adopt the repair. No dependency update or CSS rebuild is required by this
copy-only change. INC-44 records the defect and its narrow durable checks.

## Exact retained evidence

Private root: `/Volumes/m2ext/appkit-exact-browser.zRMlUOGI/replay-v7/`.
Candidate: `v0.0.0-rehearsal.h6b98f004a2a97fa0d7ea3b19d0e29ec2aba45b2d757a77e3ca8a3a108c0af705`.

| Artifact | SHA-256 |
|---|---|
| `browser-receipt.json` | `b083c870c2669a24061b26852c6031f3db0738feda6421746965d82e9e1df641` |
| Parent `host-receipt.json` | `640e557e06dde2591c39e9ad4030f1b034396fc56dd27fd8a82cd42f75fcd309` |
| Executed server | `95cdc86056847a3beb18b9a9d7fa327041c39ad81adb9140b6f702a8b39e4e71` |
| Frozen `harness/run.mjs` | `346dbe97ad2225576d4f868b4f781cb75811bc6b65b492e7634cdaa1a2627941` |

The receipt names all other harness/image identities. Browser version is observed,
not a complete cryptographic inventory of Chrome/Playwright or the operating system.

## Retained failures and remaining gates

Sibling V1 failed before application checks because the Python network canary inherited
a source-denied working directory. Private cwd plus minimal environment fixes the test,
not the sandbox policy. V2/V3 passed desktop but failed narrow startup: the test opener's
published-before-fsync file could be consumed before writer verification. Atomic completed
inode publication with no-clobber hard links fixes that race; immediate-unlink and collision
regressions protect it. V3 also retains URL-free connection/server-exit diagnostics.

V4 passed both script-enabled variants but raced a plain-form redirect: waiting on an
old loaded document is insufficient. Navigation is now armed before submit and unit-tested.
V5 passes all176 assertions with full-document captures. V6's added viewport capture
waited for a page animation callback that cannot run with page scripts disabled; V7 uses
synchronous geometry and browser screenshot synchronization instead. Earlier runs and
their exact harnesses remain retained; failures are not retroactively marked passed.

This proves representative independent browser tasks, not real-consumer adoption, full
accessibility/IME/zoom/performance, native input/media, macOS14 runtime, installation,
signing/update recovery or OS-selected browser launch. The test-only opener is not
LaunchServices proof. Chrome is not process-network-sandboxed and may temporarily persist
the launch URL inside its private profile; passing requires confirmed profile cleanup.
G1–G6 remain open. Windows and CA/GGB adoption stay owner-deferred.
