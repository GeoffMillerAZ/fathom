# RAC theme review: real badge contrast and actual 200% zoom

G1/G4 follow-up to [initial guide delivery](2026-09-15-resume-theme-guide.md).
I8/I12/I15, ADR-0029/0031 and RAC ADR-028 govern this synthetic-only work. The
default theme, normal app UI, CSS, vendor closure and dependencies are unchanged.
No commit, deployment, real data or global Chrome settings were changed.

## Product defect and durable repair

Visual inspection of the actual Today preview found that the ten-check candidate
made the Email proposal badge difficult to read. The real shared ApprovalCard
uses `callout.Badge(..., "accent")`: `text-accent` on `bg-accent-soft`, without alpha.
The guide checked `accent-ink` on `accent` for actions but omitted the badge pair.

| Pair | Source | Earlier candidate | Joint-token candidate |
|---|---:|---:|---:|
| Badge: accent / accent-soft | 3.047:1 | 1.469:1 | 6.324:1 |
| Action: accent-ink / accent | 4.326:1 | 8.972:1 | 7.897:1 |

Only changing accent cannot satisfy both 4.5:1 checks with the existing companion
values. The review candidate now changes accent to `#b69aff`, accent-ink to
`#180f2e`, and ink-faint to `#c9bfe8` in both branches. This is an exported example,
not an applied preference. The original Synthwave bytes remain immutable.

RAC adds `badge-accent-light/dark` through the existing engine API: twelve selected
checks, with six source failures now exposed (action, faint table text and badge,
each in both branches). No generic editor API or new token was needed.
`TestThemeGuideRejectsBadgeContrastRegression` failed before the repair: missing
named checks and acknowledged export returning HTTP 200. It now requires refusal
and binds the check to the actual Today badge classes. The export test verifies
the exact five ZIP members, all twelve definitions/roles/schemes/4.5 thresholds,
unique passing results, replay and verbatim original bytes. RAC INC-16 records
the outcome. The [kit guide](../design/app-owned-styleguide.md) returns the lesson.

## Native journey and capture integrity

Maintained `fixtures/resume-theme-guide/zoom.mjs` uses fresh isolated Chrome
152.0.7977.84 with actual Appearance zoom 2, not CSS zoom. Its original metrics
report 1440 DIP / 720 CSS width, DPR2, outerWidth1440 and visualViewport.scale1.
The script-disable CDP session stays attached until browser teardown; selected
HTML CSP also prohibits scripts. Only synthetic guide/CSS routes are admitted.

62 checks pass across source, invalid radius, repaired radius, refused badge
candidate, joint repair, stale export, fresh acknowledgement, source comparison
and reset. Native Tab, typing and Enter perform the edits and exports. The
error summary and repair link transfer focus; invalid/stale edits survive. Nine
complete visible-control inventories have no horizontal overflow and primary
targets remain at least44 CSS px. Actual badge computed paint agrees with both
the refused and repaired role values. This is not every control/input/browser.

An earlier Element.screenshot produced a blank crop. Correcting CSS-to-DIP bounds
alone still captured an unpainted offscreen iframe; the new canary correctly failed.
Explicitly scrolling the iframe into view for screenshot preparation, without
changing focus/style/content, produces painted1376x1200 captures. The bounded
helper checks the actual2x geometry; the decoder requires opaque pixels with both
dark and light populations. Fifteen Node tests cover geometry, setup cleanup and
white/black/transparent/malformed/oversized-image refusal. These are capture guards,
not pixel-difference tolerances or accessibility certification. Root visually
inspected candidate/reset captures and the matching focused-error capture.

The separate no-script normal-browser matrix passes105 checks at390/1440px, nine
specimen states and both token branches. Both probes require app.css and brand.css,
bounded ZIP inspection and cleanup error reporting. Full default-theme pixel
equivalence remains inconclusive under the earlier unchanged-source control;
no mask, tolerance or baseline replacement was introduced.

## Independent delivery and evidence

The refreshed2,065-file/21,863,254-byte build closure differs from its predecessor
only in `cmd/serve/themeguide.go` (the added pair). Two offline vendor builds with
Go1.26.3 produce identical binaries; the first uses fresh task-specific caches.
Both browser suites above run against that standalone binary outside the checkout,
with an explicitly nonexistent config path. Environment settings are not an OS sandbox.
Full `cmd/serve` race tests pass; after test-only oracle tightening, the two
focused regression/export race tests pass again. Kit MCP/styleguide and15 Node
tests pass. The retained checker passes all 31 static adoption checks against the
updated RAC contract; its identity and unchanged catalog selection are recorded in
the initial assessment. This is separate from runtime proof. The first redirected
Devbox output included its welcome banner and was not valid JSON; the clean receipt
captures only the checker's output, without changing Devbox configuration.

Local artifact root: `/tmp/appkit-rac-guide.vABdjd`. Snapshot: `standalone-contrast`;
receipts: `zoom-contrast-final` and `browser-contrast-final`. Earlier blank runs
(`zoom-complete`, `zoom-capture-fixed`) remain failed visual evidence. The earlier
54-check `zoom-visible-capture` exposed the badge defect; it is not theme approval.

| Artifact | SHA-256 |
|---|---|
| Build-closure manifest | `0820bd0c7189e871b91754b538bcf853f68f3781e7b26a5fe04fa0cc3202872a` |
| Both standalone binaries | `ee3ac71947ece750767a2608fcebb9ef5e840df586935f857d3036eb13a5ed0f` |
| Zoom probe | `59f3f134da57770cbfcda29a1f9e3c3d328433baaeb71025cdb9204b78b5206d` |
| Zoom receipt,62 checks | `639f22e1787d028133ae9cfb3bc7ddc25213c5dbd0f5f8db0f5a8474e6f322f5` |
| Zoom candidate capture | `bf484ed95262a637e426ce1929381a317379009be53ab328686bb89f4e1e5a08` |
| Zoom reset capture | `750e847330eda524c53834bdf806d4121bea80bf96eac8899c1eb5de54555a25` |
| Normal probe | `fee0c12a520553c5a517bcf1ffa151fe277fcd2201b01365b4a84516aaf3c8c5` |
| Normal receipt,105 checks | `fba649eacdf42cb1d31983b541ed33a985fc09b13cbf65d7ef19dfc0e10ff5a1` |
| Zoom exported ZIP,17,650 bytes | `f639f124f1aa61cf1feb3926e7421f77621ed268699818f692ed44efd262f729` |
| Static adoption receipt,31 checks | `adeb7bb1e8927369ba3ade7417235e7208c3024e97649febba2bd7cea9b6c794` |

The zoom run records45 responses/771,034 decoded body bytes across the entire
journey, not a first-view/network-transfer budget. Seven local submissions take
37.25–67.26ms on an AppleM4/Mac16,10 with32GiB RAM. Concurrent normal-browser
testing ran on the same host; this is not an isolated performance benchmark,
human task-time measurement or production p95 acceptance.

Replay the maintained probes against the printed URL from `racli theme-guide`.
They require the explicitly approved local harness path and Chrome executable.
Artifacts are temporary local evidence, not a released distribution. Full visual,
assistive-technology, font and performance acceptance, tracked release and second
guide adoption remain open. G1/G4 are not complete.
