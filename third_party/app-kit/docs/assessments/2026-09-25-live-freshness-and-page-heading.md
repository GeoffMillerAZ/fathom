# Live freshness and page heading: scoped Gallery evidence (2026-09-25)

Fathom's observatory console (vision 35; `docs/design/observatory-console.md` at Fathom
`f8b40609`, sha256 `b9630e3d2b6ec4667362cfd23e8911025093d2af3af923fce66f2b351e676272`)
selects two just-in-time pieces: FATHOM-KIT-06 live freshness (L2) and FATHOM-KIT-07
page heading (L3b). This records what one local browser run proved for the new
experimental `appkit/ui/live` package and `chrome.RenderPageHeading`, and what it did not.
Fathom has integrated neither; this is kit-side evidence only.

A same-day review found nine defects the first run (below) did not catch. They are fixed
and the current evidence is the [corrected run](#corrected-run-45-of-45-checks-pass).
The first run's record stays as it was measured.

## Corrected run: 45 of 45 checks pass

| Artifact | sha256 |
|---|---|
| Gallery binary (`go build ./apps/gallery/cmd/gallery`, go1.26.3 darwin/arm64) | `cb8ff9637e48f57252479fe1eb30e67e5063b94dfeb5ed1c59d75043dc47f59f` |
| `appkit/ui/live/live.js` | `9aeef7ae80a4703858bdec3973d2b6ccb6e8820d60e9b544fc92d4a578837330` |
| `appkit/ui/live/live.go` | `bd5f9c2cb0eb66c339732e6960820250bc414e9d05549084554566f60daccc1f` |
| `appkit/ui/chrome/heading.go` | `aeaed880d6bd7ffc9939db9f8a73d14e6504caef130328a60bd4b98c02c75246` |
| `apps/gallery/live_freshness.go`, `apps/gallery/page_heading.go` | unchanged from the first run |
| `appkit/catalog/catalog.cue` / `internal/qualityplan/inventory.cue` (embedded) | `64fc3b7039ce9d466ed644834f38ba7e447347b99b400480edbbbf7bee720fa1` / `0f4155cecd1e3d1195e0bdf11c7cfa61d1809716a9825753c3c2c30a851f952e` |
| `apps/gallery/ui/app.css` (pinned Tailwind 4.3.0) | `38544e101a4cfd3b80529da0033030ff90de5f3f1d5f15e1bf05ac7187ab7a28` |
| [Harness](fixtures/live-freshness/browser-run.js) | `f34be1a7cdffcab121de965a5f36ffe276270b08f27f6ff9f63178e34aad8f15` |

The binary was built from the uncommitted working tree whose embedded sources are the
hashes above; its build info stamps `e7d66c9` plus local modifications, so a rebuild from
this commit differs in build info only. Same browser, host and loopback fixture as the
first run, on a freshly started binary; the harness differs from the executed text only
in its output path. It keeps every first-run
check (A8 now asserts that marking leaves the connection line unchanged; B2, B5 and B6
read the control's rendered label) and adds:

| Check | Observed |
|---|---|
| A13 region focus ring | After Enter on refresh and the reload, `#live-1` matches `:focus-visible` and paints `solid 2px` |
| B2 first retry | 1.19 s after the drop (floor 1 s, jitter up to 1.5 s) |
| B5a control name | Named "Stop live updates" only; the reserved wider label is outside the accessible name |
| F1 UTF-8 cursor | `start({lastEventId: 'plan-ω1'})` reaches the server as the id's UTF-8 bytes; the fixture refuses the non-decimal id with 400 (`refused-400`, one request), not a fetch `TypeError` |
| F2 narrow stability | 320 light, 340 dark, 360 light: page status and all three panels keep identical document boxes through marking, stop, resume and a refused reconnect; no overflow |
| E3 script-free height | At 320 px the no-JavaScript status box is 92 px, the running height |

One check outside the harness, on the same binary: a mouse click inside `#live-2` focuses
the region without `:focus-visible` and paints no outline, so dropping `focus:outline-none`
adds no ring for pointer users.

Node checks (`live_test.mjs`, 27, two against a real local server with Node's `fetch`) cover
what the fixture cannot produce: an accept-then-close server exhausts the attempts (4
requests for `maxAttempts: 3`), explicit starts adopt the page cursor and wait for the parsed
document, a refused configuration or unsupported browser offers no control, and the first
delay spreads over 1–1.5 s. Ten script mutations, one per fix, each fail at least one of
them.

## First run: identities

| Artifact | sha256 |
|---|---|
| Gallery binary (`go build ./apps/gallery/cmd/gallery`, go1.26.3 darwin/arm64) | `5d008e9466b118aef68bcd58a3d580b0308b5da2c7234ab392f2d07669f6de3f` |
| `appkit/ui/live/live.js` | `2044884c437a8196e0be04fc49ccaf98edf9d8fcfaae35ff527bf316cef907ff` |
| `appkit/ui/live/live.go` | `f8c23d133e579ee091f7db02f6c23d412761ba03655811d173ff40e81013d72f` |
| `appkit/ui/chrome/heading.go` | `af9df9a9a59eefc34b6d06436deef4edc9be3a5a54ce4e54f400680d9b13cfd2` |
| `apps/gallery/live_freshness.go` | `3dc67d32145e04909b412f7f49ef2eb7d9d68b693da2545580cb301a97d2fc94` |
| `apps/gallery/page_heading.go` | `cd6c3f55c30bbf39c50f6af4bce48bc26840947958849fd28e4d9672944fad91` |
| `apps/gallery/ui/app.css` (pinned Tailwind 4.3.0 via `task gallery:css`) | `34e5f6ccdf3c8227dd8b3cc9cbb34c3d1f5deaca0b5db23e495eb94c466b8913` |
| Harness as committed in `e7d66c9` (Playwright MCP `browser_run_code`) | `bc2b3705129e972d1fa40a8b64d847d9166db4022362a4f98469a24aff6fef39` |

The tested binary predates the quality-inventory records added in the same change; its
live, heading, fixture and CSS sources are byte-identical to the commit (hashes above),
but a binary built from the commit differs because the Gallery embeds that inventory.

Browser: Playwright Chromium 153.0.8010.53, macOS 26.6.2, loopback `127.0.0.1:8775`,
synthetic fixture only. Screenshots stayed in a private scratch directory and are not
retained here. The harness file differs from the executed text only in its output path.

## First run: 38 of 38 checks passed on the identities above

| Area | Viewport / theme / input | Observed |
|---|---|---|
| Start | 1280×900 light | Auto-start from markup; first request sends the render-time `Last-Event-ID` (`0`); "Live updates on." |
| Mark without moving | 1280 light, keyboard Tab + Enter on a fixture control | Only the Plans region marked; 20 region/child/status/section boxes identical in document coordinates; region HTML identical; as-of and updated layers start at the same point |
| Announce | same | One polite message after 1 s: "Plan status has newer data. Use its refresh link to see it."; connection line counts 1 panel |
| Refresh | keyboard Enter on the refresh link | Accessible name "refresh this panel (Plan status)"; 2px focus outline; 45px target; reload at `#live-1` shows revision 1, focus on the region, scroll position unchanged (215) |
| Replay | dropped stream, event published during the gap | Reconnect sends the page cursor `1`; missed event marks Index |
| Backoff | three refused (503) connections | Attempts at +1.0 s, +1.7 s, +3.5 s, recovery at +5.3 s after the last refusal; status events per attempt |
| Bounded give-up | explicit 100–200 ms configuration, eight refusals | Stops at `unavailable` (attempt 8, `http-503`), honest copy, polite announcement, "Resume live updates" |
| Stop hook | keyboard Enter on the one control | Stop keeps focus; a stopped page is not marked; Resume (Enter) replays the missed Sessions event |
| Hidden | synthetic `visibilityState` override, not a real tab switch | Still live after 4 s hidden; paused after 5 s; visible again reconnects with `Last-Event-ID` and replays |
| Narrow | 390×844 dark | No horizontal overflow; heading actions below the title; a `*` gap marks all three panels with zero box movement and one coalesced announcement |
| Contrast | canvas-converted computed colors | Light: as-of 7.27, updated 17.48, link 6.13, connection 7.27, eyebrow 5.97, meta 7.09. Dark: updated 14.77, link 6.20, connection 6.81, eyebrow 6.65, meta 7.30 |
| Page heading | 1280 light and 390 dark × full / title-only / long | One `h1`; no overflow; actions beside (wide) or below (narrow) the title; every action 44px; long title wraps (96/128px) |
| Heading keyboard | 1280 light, Tab | Breadcrumbs → actions → variant links, focus shown at each stop; unsafe action is an inert `role=link` label, skipped |
| No JavaScript | separate context | "Live updates are off. Reload for newer data.", no control, no updated claim; fixture POST returns 303 to a page with the newer revision |
| Errors | whole run | No script errors; 11 expected resource errors are the deliberate 503 refusals |

## Retained failures and corrections

- A first layout check compared viewport coordinates after focus scrolled the page and
  reported every box moved; the instrument was wrong. Document coordinates replaced it.
- **Product defect found and fixed:** a freshly loaded page had no `Last-Event-ID`, so an
  event published while its first stream was down was lost. `Page.LastEventID` now
  renders the server's position and the first connection resumes from it (Go and Node
  regressions added).
- The next run's backoff check failed because an older tab shared the fixture's refusal
  budget, and status events were not emitted for repeated attempts with the same error.
  The script now emits one status event per attempt; the old tab was blanked.
- An earlier dark contrast computation parsed `oklch()` strings as RGB and is not
  evidence. The retained harness converts colors through a canvas.
- The first status line used a 44px `inline-flex` link, which made the reserved cell tall
  and shifted the as-of text vertically when the updated text wrapped. The link is now
  inline with padding inside the line's padding; the alignment check is part of A6.

### Review findings on the first run, all fixed

1. **Invisible region focus.** The region carried `focus:outline-none` with its
   `focus-visible:outline-2` ring; under Tailwind v4 the first sets the outline-style
   variable the second reads, so no ring painted after refresh. A11 checked only where
   focus landed. The class is gone, a Go test refuses the pair and A13 checks the
   computed outline.
2. **Movement at 320–340 px.** Marking appended "· N panels updated" to the connection
   line, which wrapped and pushed every panel down 16 px; the no-movement claim held only
   at 390 and 1280 px. The count is gone and the line now stacks an invisible copy of
   every message (a Go test keeps it equal to `live.js`), the control keeps its box and
   reserves its wider label, and it wraps below the text from a fixed basis (F2, E3).
3. **Non-Latin-1 ids.** `fetch` refuses a header character above U+00FF, so such an id
   ended reconnection; U+0080–U+00FF went out as Latin-1. The header now carries the id's
   UTF-8 bytes; cursors must be unpadded and at most 256 UTF-8 bytes on both sides.
4. **Explicit start lost the cursor.** `start({url, events, sources})` ignored the
   rendered cursor. It now adopts the cursor of the page status with the same URL and
   connects once the document is parsed; another URL starts without the old cursor.
5. **Unbounded accept-then-close loop.** Any line reset the attempt budget, so a server
   that sent a preamble and closed was retried every second forever. Only 10 s open or an
   event that advances the cursor resets it now.
6. **Unsafe breadcrumb links.** `PageHeading` rendered a `javascript:` crumb as a live
   link; `Validate` now refuses crumb destinations that are not relative or HTTP(S).
7. **No first-retry jitter.** The first delay was clamped back to exactly 1 s; it now
   spreads over 1–1.5 s, never below the floor or a larger server `retry`.
8. **Dead control and false announcement.** After a configuration failure the Resume
   button did nothing and the line said "Reload to retry."; resuming in an unsupported
   browser announced "Live updates resumed." No control is offered in either state, the
   line says "Live updates unavailable. Reload for newer data.", only a resume that runs
   is announced, and `Page`/`Region` refuse ports above 65535.
9. **"SSE" found nothing.** The catalog entry now lists SSE and event-stream tasks;
   selection cases cover both queries.

## Not established

Screen-reader speech (the announcer's DOM text only), other engines, real tab switches,
real network loss, long-running memory, CSP-restricted hosts, forced colors, actual
browser zoom (narrow widths were emulated by viewport), human task success and Fathom's
actual `/events` integration. The accept-then-close bound and first-retry spread were
measured in Node, not against a misbehaving server in a browser. Performance was not
measured. Consumers without the scanned L3b CSS would see the updated layer; the Gallery
CSS closure test guards only the Gallery.
