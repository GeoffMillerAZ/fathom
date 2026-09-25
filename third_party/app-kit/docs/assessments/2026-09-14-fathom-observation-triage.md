# Fathom: truthful observation triage

Owner-selected G3/G4 consumer follow-up to the [Observatory uplift](2026-09-13-fathom-observatory-uplift.md).
This repairs app-owned evidence projection and composition, not observation collection,
the JSON model, a generic aggregate API, a dependency upgrade or a whole-app redesign.
Concurrent source/config work remains untouched. No live records, provider calls or deployment.

## Outcome and design

| Available evidence | Fleet, impact, story and home |
|---|---|
| No observation counts | Named absence, never a fabricated zero |
| Some counts, incomplete coverage | `≥ N`, reporting-source coverage and explicit lower-bound label |
| Complete counts, including measured zero | Exact sum with coverage |
| Registry unavailable/incomplete | Absence or a lower bound; listed rows never become the whole fleet |

One app-owned projection feeds all four readers. Story identifies the backlog as a
**fleet snapshot**, separately from weekly session totals. Home no longer zero-fills
unavailable verify catches or logged calls; simultaneous registry/call-log diagnostics
remain visible and escaped. Existing per-service count semantics are preserved.

Impeccable hardening kept the role-token identity and both themes. The pinned kit's
five-column band clipped at 768px beside its sidebar; existing APIs now compose three
indexing metrics plus two evidence-status metrics. Story's snapshot has its own band.
Lower-bound signs stay with their numbers. Service-detail links have 44px targets and
keyboard outlines. The absence dash uses existing `ink-muted` to meet this slice's
chosen 4.5:1 text target; the old large dash was about 3.86:1, not a blanket WCAG failure.
Paid description-list composition was consulted; no licensed markup entered Fathom.

## Evidence

| Check | Result | Boundary |
|---|---|---|
| Initial observation regressions | Actual behavioral RED, then GREEN | Nil, partial-zero/positive, exact-zero/positive, degraded and escaped diagnostics |
| Landing regressions | Actual RED, then GREEN | Missing call log, missing catches, simultaneous source failures; bounded Q2/Q3 assertions |
| Initial three-page browser probe | 256 failures / 832 checks | Caught fabricated totals, tablet clipping and 18 undersized detail targets |
| Expanded baseline, final oracle | 100 failures / 1,504 checks | 42 nil-catch headlines, 24 stricter contrast misses, 16 split inequalities, 18 landing-context mismatches |
| Final Chrome run | **1,504 / 1,504 pass** | 168 aggregate page cases, 18 partial-fleet detail/return journeys, 24 screenshots |
| Visual inspection | Final phone-light fleet/story and desktop-dark home reviewed | Earlier dark impact review retained; not every screen or state visually certified |
| Focused Go suites, `-race` | Workspace and `GOWORK=off` pass | Observatory, landing and preview policy; no installed daemon or full-repo gate claim |
| `go vet ./cmd/fathom` | Pass | Consumer package static checks |
| `task ui:css` repeated | Same CSS/token hashes | Uses the existing sibling-based generator, not a sibling-free rebuild |
| Adoption checker / config audit | 15 pass / 8 hold, 0 violated or unable | Declared hashes/routing, not host invocation or fleet rollout |
| App-kit MCP tests | Pass | Guidance/protocol suite; not semantic adoption proof |

Final oracle exact-matches values and full reasons; `10` cannot satisfy expected `0`.
It checks observation glyph bounds, inequality grouping, sampled text contrast,
page overflow, inertness, explicit themes, keyboard detail navigation/focus/return,
and fixed-route/method refusal. Registry reasons are exact, not keyword matches.
Chrome resolves OKLCH into opaque 8-bit sRGB samples for contrast; unsupported alpha
does not pass. An earlier RGB-only instrument rejected all 168 color checks as unknown;
that diagnostic is retained, not described as an application regression.

Apple M4 / macOS arm64; Chrome 152.0.7977.84; isolated Playwright 1.63.0; fresh profile,
320/768/1440 × 1000 CSS-pixel viewports, DPR 1, reduced motion, light and dark.
Predetermined budgets: HTML ≤163,840 B, CSS ≤102,400 B, DOM ≤2,500 elements, local
DOMContentLoaded ≤1,500ms. Recorded maxima: **15,731 B HTML**, **52,698 B CSS**,
**180 elements**, **28.7ms**; sampled metric-text contrast minimum **6.13:1**.
These are inert local-delivery readings, not production render/input/update latency.

## Replay and identities

Consumer test `TestExportAppkitObservationPreview`, with
`APPKIT_OBSERVATION_PREVIEW_DIR` naming an existing empty `workspace/tmp` child,
exports seven fixed models: unknown, partial, complete-zero/positive, registry-unavailable,
registry-partial and call-log-unavailable. Home/fleet/impact/story and only existing
alpha/beta/gamma service rows are exported: 92 HTML documents and 14 CSS copies.
It invokes pure renderers, not operational handlers, the cache, providers or the daemon.
Scripts/forms are disabled under restrictive CSP; this is not a generic HTML sanitizer.

Run [browser.mjs](fixtures/fathom-observations/browser.mjs) with the export directory,
a separate empty `workspace/tmp` report directory and the approved isolated Playwright
module path. The loopback server maps only fixed GET/HEAD documents/CSS; arbitrary
services, operational endpoints and mutation methods are refused. It closes on exit.

Artifacts below are under `sextant/workspace/tmp/` (local, ignored, not release evidence):

- Original red: `appkit-observations-before.yEXDc9`, `appkit-observations-browser-before.rrbYOz`.
- Expanded canonical red: `appkit-observations-expanded-red.9pmZ4V`, `appkit-observations-browser-canonical-red.oqUjng`.
- Final reviewed: `appkit-observations-reviewed.6tfkuE`, `appkit-observations-browser-reviewed.D9VHhf`.
- Final combined-source repair replay: `appkit-observations-replay.w4mIl5`; all 106 exported files match the reviewed fixture byte-for-byte.
- Render tests/logs: `observatory-observation-coverage-red/`; combined-source red is also recorded in the session transcript.
- Final report SHA-256: `745fccaf7b6ddacac8345d2fc82efcb18025e57590d9a04bbbe774f2961aaa7b`.
- Canonical red report: `1ec3f66bcd90dba3cd80212a2c50f447e71af7109b0896c88182d6093f430cc5`.
- Shared browser oracle: `c74a2e984d88de91e55bfcc80305ba9a6374b2dd55d34851fe331d6617476c83`.
- Final renderer: `d56eeb31c479e14746cf281f18c45190d83e4b725eb33eb9cb8c5ce1e875e52f`.
- Story/projection: `616a8336d64c26d87509128a2a6afedd7d1f521dcc9f4d2c63df2fcfc7a1261f`.
- Generated CSS: `f3e5a59c20136302d2b4c23c3eb71f56057eea665dba84c3e774facfb6ba1d00`.

## Return and remaining work

The [analytics guide](../design/experience-system.md#analytics-excellence) now requires
coverage-aware aggregation and shared app-owned projections, snapshot/period separation,
and glyph/inequality checks. Fathom's INC-18 and adoption contract retain the failure
and enforcement. Existing agent routers already reach those contracts; no global
configuration, dependency, stamped mirror or new MCP server was needed.
Targeted intent checks resolve all 56 links in changed documents; app-kit remains at
77 router/150 scope lines and its Unreleased block is within 60 lines after folding
related entries. Fathom's already-oversized scope (186 lines) and Unreleased block
(now 434 lines) remain budget debt; this is not a full intent-doctor or semantic-audit pass.

The original browser detail assertions covered only the **partial** scenario. The
post-navigation follow-up below closes the remaining service-distribution gap;
self-recorded file hashes are still identity, not correctness proof.
The later [evidence-journey repair](2026-09-14-fathom-evidence-journey.md) adds browser
cases for combined registry/call-log failure and nonnil landing catches. Full accessibility/zoom, every metric's contrast,
human task time/error, live mutations and production performance remain open.
At the original observation-repair baseline the mirror was `d22871195401594f1083e2952d938ec5dfd655d3`.
The subsequent [navigation handoff](2026-09-14-navigation-delivery.md#fathom-owner-handoff--2026-09-14)
advances it to `63b3a4f`; sibling-free CSS and tracked-only delivery remain separate gates.
No recipe/target certification or G1–G6 completion follows. Conjure remains F2.

## Post-navigation detail follow-up

The shared browser harness now exercises all six existing service-bearing scenarios,
using literal expected vectors independent of the rendered pages: unknown/unknown/unknown;
0/4/unknown; 0/0/0; and 0/4/2 for complete-positive, registry-partial and call-log-unavailable.
Each journey checks the exact service URL, heading, observation value, visible keyboard
focus, 44px target, and return to the exact scenario total/coverage and theme. In an
unavailable registry, there are no detail links and even the fixed alpha route is 404.
Detail pages receive the same inertness, overflow, DOM and local timing budgets.
Existing aggregate value, coverage, glyph, contrast and network oracles remain unchanged.

| Observed gate | Result and boundary |
|---|---|
| Fresh fixed export | Pass from current Fathom working tree, `GOWORK=off`, cached dependencies; 92 HTML +14 CSS, no operational handlers |
| Preview policy | `TestAppkitObservationPreviewPolicy` passes with `-race`; not the full consumer test suite |
| Chrome matrix | **2,398/2,398 checks pass**; 168 aggregate page cases, 108 service-detail/return journeys and six registry-refusal cases |
| Captures/network | 28 screenshots, 1,044 loopback requests; zero page errors or unexpected blocked requests |
| Observed maxima | 17,557 B HTML, 52,776 B CSS, 194 DOM elements, approximately 30.6ms local DOMContentLoaded; same preset budgets |
| Contrast | Aggregate metric-text minimum remains approximately 6.13:1; detail text is not included in this contrast sample |
| Visual inspection | Phone light/unknown and dark/registry-partial detail captures inspected; labels, absence and registry warning wrap without page overflow |

Chrome152.0.7977.84 / Playwright1.63.0, macOS arm64, 320/768/1440 ×1000, light/dark,
reduced motion, DPR1. Keyboard journeys explicitly focus the intended link, then use real
Enter navigation; this is not an exhaustive Tab-order, screen-reader or physical-input test.
No application source, consumer pins, paused Claude work, global configuration or deployment
changed. Only the app-kit harness and its evidence/docs changed. This closes one bounded
G3 fixture gap, not full analytics acceptance or human task-time measurement.

Private retained artifacts under `sextant/workspace/tmp/`:

- Export: `appkit-observation-detail-export.Hbe1xO`; all 106 files unchanged after browsing.
- Report/captures/source snapshots: `appkit-observation-detail-browser.CHXRSH`.
- Report SHA256: `551cea1044dc24ddf62a8685ab4eba0a867fbf3ee87469e8fef7440c86927dda`.
- Harness SHA256: `9d2bfb9a002457946b608a9bbeed64110cd46fd195c4f986739ad863dd6530e9`.
- Exporter SHA256: `839186f05323a672a8c5893810d57cc67a9c8c316dbc277a7a6d1e95bd8ba2c1`.

Replay uses the same exporter and browser commands above, always with new empty output
directories. The [subsequent cost/evidence repair](2026-09-14-fathom-evidence-journey.md)
extends these fixtures to ten scenarios. Full accessibility, true zoom, broader
performance and real human task acceptance remain open work.
