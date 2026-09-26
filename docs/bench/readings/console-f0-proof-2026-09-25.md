# Console wave F0 — the browser proof (2026-09-25)

The doctrine's rule 4 (`cmd/fathom/CLAUDE.md`: a 200 is not a rendering) and the
plan's gate (`docs/plans/observatory-console.md`: every page row needs a browser
proof in both themes at 1440 and 390 px, recorded in the wave's reading).

## Verdict, plainly

**F0 renders. All 11 console pages were opened in a real browser at 1440×900
and 390×844, in light and dark, and every shell check passed on all 44 views:**
exactly one `h1`, the skip link is the first Tab stop and moves focus into
`main`, the four nav groups (Now, Fleet, Evidence, System) with the current page
marked, breadcrumbs with `aria-current="page"`, no horizontal page overflow at
either width, and no console errors or failed requests. The one console error
recorded is on the 404 page: Chromium logs the document's own 404 status. That
is the page working as designed.

**Two things are wrong, and neither is a rendering failure.** Both are about
what the numbers say:

1. `/brief` reports a different "tool calls in the current call log" figure
   from `/fleet` and Now (51,814 against 90,258). In the same paragraph it
   calls the agent surface "16-tool" while also saying it has 25 tools.
2. The console's own page renders write `list_services` rows into the call log
   it reports as usage. That inflates the tool-call counts on Fleet, Impact
   and Now.

The minor items further down are cosmetic: mid-word path breaks, a chart and a
table that scroll at 390 px, and a nav label that differs from its page title.

## How it was run

- **Binary:** `bin/fathom` built from branch `console-f0` at `e2c2b19f` (the
  commit under proof), using the module's own Go env (`GOWORK=off`, CGO on).
- **Daemon:** `serve-http` on `127.0.0.1:7650`, `--auto-refresh=false`,
  `FATHOM_SELF_RESTART=off`. `HOME` pointed at a scratch home, so the daemon
  record, config and page-view log never touched `~/.fathom`. The live daemon
  on 7575 and its store were not touched, and no LLM profile was loaded.
- **Data:** a scratch copy of the fleet store, made with `sqlite3 .backup`
  against a read-only URI (18 services, 136,946 symbols), plus copies of the
  call log (90,252 rows), the session ledger (3,270 rows, 1,888 sessions) and
  the verify ledger. `--call-log`, `FATHOM_SESSION_LEDGER`,
  `FATHOM_VERIFY_LEDGER` and `FATHOM_CONSOLE_VIEWS` all pointed at scratch
  paths.
- **Browser:** Chromium through the Playwright MCP. Every page got a full-page
  PNG. The per-page checks ran in the page itself: `h1` count and visibility,
  `body.firstElementChild` as the skip link and whether its target exists,
  `document.activeElement` after one Tab, `nav[aria-label^="Primary"]` groups
  and the current item, the breadcrumb items and `aria-current`,
  `scrollWidth` against `clientWidth` (with offending elements listed if any),
  `data-theme` on `<html>`, the body background, and the console, page-error,
  failed-request and ≥400 subresource listeners.
- **Theme:** set with the page's own appearance control, never by script. At
  1440 px that is the sidebar select; at 390 px it is the select inside the
  mobile Menu disclosure. The choice persisted under `fathom-theme` ("Saved on
  this device.") and carried across navigations. Every view confirmed
  `data-theme` plus the matching ground: light `oklch(0.985 0.005 260)`, dark
  `oklch(0.16 0.012 260)`.
- **Skip link, activated:** on `/fleet` at both widths, Tab then Enter moves
  focus to `main#observatory-main` (`tabindex="-1"`). The next Tab lands on the
  first breadcrumb link inside `main`.
- **Mobile nav:** at 390 px the sidebar is hidden and the grouped nav sits in a
  Menu `<details>`. On every page the proof opened it and checked that the same
  four groups show, with the current page marked and still no overflow.

Every shot was taken after the one Tab press, so the focused "Skip to content"
link appears top-left over the brand. That is the keyboard proof showing in the
image, not an overlap bug.

## Per-page results

All rows: 1 `h1` (visible) · skip link first, with its target present · nav
groups Now/Fleet/Evidence/System · breadcrumbs visible with the last crumb
`aria-current` · `scrollWidth == clientWidth` at 1440 and at 390 · light and
dark confirmed · 0 console errors unless noted.

| Page | Route | Status | h1 | Nav current | Breadcrumbs | Height 1440 / 390 | Notes |
|---|---|---|---|---|---|---|---|
| Now | `/` | 200 | Now | Home | Observatory / Now | 1360 / 2716 | |
| Fleet | `/fleet` | 200 | Fleet | Fleet | Observatory / Fleet | 3494 / 6953 | tools chart scrolls at 390 (item 4) |
| Project | `/projects/fathom` | 200 | fathom | Fleet | Observatory / Fleet / fathom | 1698 / 3350 | path breaks mid-word (item 3) |
| Impact | `/impact` | 200 | Impact | Impact | Observatory / Impact | 2748 / 4524 | tables scroll in their own boxes at 390 |
| Sessions | `/sessions` | 200 | Sessions | Sessions | Observatory / Sessions | 1350 / 1734 | |
| Session | `/sessions/8c867d31-40ca-4619-b77b-f8f6b680fc2f` | 200 | Session 8c867d31-… | Sessions | Observatory / Sessions / Session … | 900 / 856 | h1 ellipsizes at 390 (item 7) |
| Brief | `/brief` | 200 | Brief | Brief | Observatory / Brief | 4013 / 7097 | **wrong call count (item 1)** |
| Wiring | `/wiring` | 200 | Wiring | Wiring | Observatory / Wiring | 1579 / 3768 | root paths break mid-word (item 3) |
| Statusz | `/statusz` | 200 | Daemon status | Statusz | Observatory / Daemon status | 2340 / 3262 | table scrolls at 390 (item 5); label (item 6) |
| Search | `/search?q=conserve` | 200 | Search: conserve | none (correct) | Observatory / Search: conserve | 2262 / 3052 | 379 matches: 1 verb, 360 symbols, 3 records, 15 plan rows |
| 404 | `/no-such-page` | 404 | Not found | none (correct) | Observatory / Not found | 900 / 844 | 1 console error: the document's own 404; reason and remedy render on the shell |

Screenshots are `<page>-<width>-<theme>.png`: 44 views plus three extras (47
PNGs). They are kept outside the repository, in the proof's scratch directory
`console-proof/shots/`, and are not committed:

- `home-{1440,390}-{light,dark}.png`, `home-390-{light,dark}-menu-open.png`
- `fleet-{1440,390}-{light,dark}.png`, `fleet-390-light-tools-chart.png`
- `project-fathom-{1440,390}-{light,dark}.png`
- `impact-{1440,390}-{light,dark}.png`
- `sessions-{1440,390}-{light,dark}.png`
- `session-detail-{1440,390}-{light,dark}.png`
- `brief-{1440,390}-{light,dark}.png`
- `wiring-{1440,390}-{light,dark}.png`
- `statusz-{1440,390}-{light,dark}.png`
- `search-conserve-{1440,390}-{light,dark}.png`
- `not-found-{1440,390}-{light,dark}.png`

## What is broken

### 1. `/brief` disagrees with `/fleet` on tool calls, and contradicts itself on the surface size

The brief says: "51,814 recorded tool calls (13,835 of them through the curated
16-tool agent surface)" and "Tool calls recorded: 51,814 in the current call
log". The same render of `/fleet` and Now says "tool calls (log) 90,258 —
every row in the log; 55,098 attributed to a service".

The 51,814 figure is the sum of the 18 listed services' per-service
`CallsTotal` (`foldBriefFleet`, `cmd/fathom/brief.go:97`), so it counts calls
attributed to a registered service, not the log. The label is wrong. Three
bullets later the brief says "the agent surface is 25 tools". A `tools/list`
on `/rpc/agent` returns 25, and Impact says 25. "16-tool" is a literal in
`brief.go:244`, unchanged since `ec50190e` (2026-08-08).

This is pre-existing brief text, not F0 code. But F0 serves it at `/brief`, and
on screen it breaks OC-9's rule (one projection per metric, every page agrees).
`TestConsoleMetricsAgreeAcrossPages` cannot catch it because the brief's call
count is not a registered shared metric; the page declares only
`MetricObservations`. **Fix:** read `calls.total` through the shared metric and
name the attributed subset as a subset. Derive the surface size from the tool
list, or drop it.

### 2. The console counts its own reads as tool traffic

During the proof nothing but this browser and a few `curl` probes talked to the
daemon. It still appended 11 `list_services` rows to its call log, all from
binary `e2c2b19f07a5`, `group: platform`, no `origin`. The first row (latency
5,264 ms) lines up with the cold 6.3 s render of `/`. Across page loads
`calls.total` moved 90,258 → 90,262 and `list_services` 33,709 → 33,715.

Those rows look exactly like agent calls. So Fleet's "tool calls", its "which
tools carry the load" chart, Impact's per-tool table (where `list_services` is
the top tool) and Now's "logged calls" all include the console reading itself.
That is a measurement-honesty defect (every number carries its instrument), not
a rendering one. **Fix:** tag console-originated calls with an origin and leave
them out of usage, or read the registry without going through the logged tool
path.

## Minor (cosmetic; no check failed)

3. **Paths break mid-word with a right-aligned tail.** Wiring cards, and the
   project page's `path` row, render each root as `font-mono text-xs break-all`
   in a right-aligned value cell. At both widths this gives a line ending
   `…/sextan` with a lone `t` below it, pushed to the right. Fleet's remedy
   commands break mid-token the same way (`--timeou` / `t 2m`).
4. **The fleet tools chart at 390 px.** The SVG is 560 px wide inside a 324 px
   `overflow-x-auto` box. It scrolls, which the responsive rule allows, but at
   rest the three biggest bars all run off the card edge, look the same length,
   and hide every count (`fleet-390-light-tools-chart.png`). The "calls per
   tool as a table" disclosure below it has the numbers.
5. **`/statusz` at 390 px.** The services table is 874 px wide in a 324 px
   scroll box, so the typed-edges and last-compile columns start off-screen.
   The rule allows it, but it reads poorly on a phone. At 1440 px the "Open →"
   link wraps its arrow onto a second line.
6. **Nav label.** The nav says "Statusz"; the page's `h1` and breadcrumb say
   "Daemon status".
7. **Session page.** At 390 px the `h1` ellipsizes the session id (the
   breadcrumb has it in full). The section label uppercases the hex id
   (`SESSION 8C867D31-…`), so it no longer matches the id. The ledger row's
   "repo" is an opaque key (`f67f06a-27d7f17caa49`), not a name. The law-14
   caveat appears twice, in the meta line and in the callout.
8. **No brand on mobile.** At 390 px the top bar shows the page `h1` and search
   but no brand, because the brand lives in the hidden sidebar.

Not defects, just what this data looks like: "3 stale repos" is expected
because the store copy is a snapshot and auto-refresh was off. The empty
session timeline ("no beats recorded") is expected because narrate is off.

## Cleanup

The daemon on 7650 was stopped. The scratch store copy, the call-log and ledger
copies, the page-view log and the scratch home were deleted. The shots and the
server's stdout/stderr (no warnings, no errors) were kept.
