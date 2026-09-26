# Console stage 0 — the browser proof (2026-09-25)

The doctrine's rule 4 (`cmd/fathom/CLAUDE.md`: a 200 is not a rendering) for stage 0
of the console pages: the app-kit mirror moved to `d9dd789`, the ⌘K palette (OC-12),
live freshness (OC-14), and wave F0's carry-overs. Plan: `docs/plans/observatory-console.md`;
the F0 reading this follows: `docs/bench/readings/console-f0-proof-2026-09-25.md`.

## Verdict, plainly

**Stage 0 renders and works in a real browser.** All 11 console pages passed the shell
checks in both themes at 1440 and 390 px (44 views): one `h1`, no horizontal page
overflow, the palette trigger with its ⌘K hint, the brand in the top bar at 390 px, and
no console errors other than the 404 page's own status. The palette opened on ⌘K, found
results as it was typed, moved with the arrow keys, opened the selected result on Enter,
and returned focus to its trigger on Escape, at 1440 light and 390 dark. A verify-ledger
row appended to the scratch copy marked `/impact` "Updated — refresh this panel" within
6 to 11 s with nothing on the page moving (identical geometry before and after), and the
refresh link reloaded the page focused on the updated region.

**The F0 proof's two defects are fixed on the real fleet.** `/brief` now says 91,386 tool
calls in the call log (the shared total) and "a 25-tool surface"; the console's own reads
land in the call log with `"origin":"console"` and every figure leaves them out (the
fleet tile names "N console page reads excluded").

**One thing this run found and fixed, one it found and did not.** `/statusz`'s table,
once its minimum widths were removed, fit 390 px only by squeezing two columns to a word
per line; it now stacks one entry per service below `lg` (commit `9280a3f6`). The session
page for a long, instrumented session is 6.1 MB and takes 2.1 s: its timeline draws every
beat. That is OC-43's windowed replay (wave F), not a stage-0 regression; every other
page rendered warm in 0.5 to 25 ms at 6 to 99 KB.

## How it was run

- **Binary:** `bin/fathom` built from branch `cp-stage0` (the stage's commits, GOWORK=off,
  CGO on).
- **Daemon:** `serve-http --addr 127.0.0.1:7660 --auto-refresh=false`,
  `FATHOM_SELF_RESTART=off`, `HOME` a scratch home. Store: a `sqlite3 .backup` of the fleet
  store through a read-only URI (21 services in `service_meta`, 18 listed). Call log,
  session ledger, verify ledger and hooks log: scratch copies (91,386 / 3,275 / 14,715 /
  40,409 rows); `FATHOM_VERIFY_LEDGER`, `FATHOM_SESSION_LEDGER`, `FATHOM_HOOKS_LOG` and
  `FATHOM_CONSOLE_VIEWS` pointed at them. The live daemon on 7575, its store and
  `~/.fathom` were not touched.
- **Browser:** Chromium through the Playwright MCP (`navigator.platform` MacIntel, so the
  palette's Mod is ⌘). Theme set with the page's own appearance control, persisted under
  `fathom-theme`; every view confirmed `data-theme` and the token ground (light
  `oklch(0.985 0.005 260)`, dark `oklch(0.16 0.012 260)`).
- **Live event:** a script appended one verify-ledger row to the scratch copy after a
  delay; the daemon's 2 s ledger poll published a `verify` event naming `plans`, which
  `/impact`'s page region follows.

## Per-page results

Every row: one visible `h1` · no document overflow at either width · both themes ·
palette trigger with a revealed ⌘K hint · brand visible at 390 px and hidden at 1440 px ·
0 console errors (the 404 page logs its own status, by design).

| Page | Route | h1 | Live | Warm render, HTML |
|---|---|---|---|---|
| Now | `/` | Now | page region (index, sessions, plans) | 0.5 ms, 36 KB |
| Fleet | `/fleet` | Fleet | page region (index, layers, sessions) | 0.9 ms, 99 KB |
| Project | `/projects/fathom` | fathom | page region (fathom's four keys) | 0.7 ms, 54 KB |
| Impact | `/impact` | Impact | page region (sessions, plans) | 0.6 ms, 51 KB |
| Sessions | `/sessions` | Sessions | page region (sessions) | 20 ms, 34 KB |
| Session | `/sessions/e8652946-…` | Session e8652946 | page region (sessions) | **2.1 s, 6.1 MB** (OC-43) |
| Brief | `/brief` | Brief | page region (index, sessions, plans) | 21 ms, 44 KB |
| Wiring | `/wiring` | Wiring | page region (index) | 9 ms, 47 KB |
| Daemon status | `/statusz` | Daemon status | two panels (daemon; index, layers) | 11 ms, 73 KB |
| Search | `/search?q=conserve` | Search: conserve | none | 25 ms, 46 KB |
| 404 | `/no-such-page` | Not found | none | — |

The fleet's tool-load chart at 390 px shows its narrow drawing (320 px viewBox, 11 px
labels at native size, every count visible); the wide drawing is `display: none`. The
`/statusz` table at 1440 px fits its box (scroll width 1,092 = client width 1,092).

## Palette journey (OC-12)

At 1440 light and 390 dark, on `/fleet`: ⌘K from the page body opened `#console-palette`
with focus in its input (idle state); typing "conserve" produced the kit fragment for
exactly that query (22 options: the verb, the symbols group's top 8 "of 360" with its
"Show all" link, rows and records); ArrowDown twice then ArrowUp moved
`aria-activedescendant`; Enter opened the selected result's own page
(`/projects/fathom`). On `/impact`, clicking the trigger opened the dialog and Escape
closed it with focus back on the trigger. No page overflow with the dialog open at
390 px. Without JavaScript, `/search` leads with the kit's GET search form.

The kit's summary line counts the options it shows ("22 results"), not the search's
totals; the group headings carry the exact totals. Noted, not changed.

## Live journey (OC-14)

`/impact` at 1440 dark and 390 light, rendered at cursor `<epoch>-3` and `-4`: the
connection line read "Live updates on."; after the appended row the page region's status
line read "Rendered … UTC · Updated — refresh this panel", `AppKitLive.state()` listed
`page-live` as updated, and the region's box, its first five children's boxes and the
document height were identical before and after. Enter on the refresh link reloaded
`/impact#page-live` with focus on the region and the status line reset.

## Cleanup

The daemon on 7660 was stopped. The scratch store, ledger copies and scratch home stay in
the stage's scratch directory, outside the repository, as do the screenshots
(`shots/<page>-<width>-<theme>.png`, `palette-*`, `live-updated-*`, `statusz-*`).
