# Console wave G — the browser proof (2026-09-25)

The doctrine's rule 4 (`cmd/fathom/CLAUDE.md`: a 200 is not a rendering) for wave G of
the console: the Learn group — How fathom works (`/learn`, OC-46), Ask the right way
(`/learn/ask`, OC-47) and the guides (`/learn/guides`, `/learn/guides/{slug}`, OC-48).
Plan: `docs/plans/observatory-console.md`; the readings this follows:
`console-f0-proof-2026-09-25.md` and `console-stage0-proof-2026-09-25.md`.

## Verdict, plainly

**The Learn pages render and work in a real browser on the real fleet's data.** Six
views (the pipeline, the ask page, a try-it answer, the guides index, a long guide and
the laws) passed in both themes at 1440 and 390 px — 24 views: one `h1`, no horizontal
page overflow, the theme applied through the page's own appearance control (the ground
`oklch(0.985 0.005 260)` light, `oklch(0.16 0.012 260)` dark), the skip link first, and
no console error, page error, failed request or response of 400 or more.

**The numbers are the fleet's, and they say where they came from.** For `fathom` the
pipeline drew 2,056 files indexed, 22,214 symbols, 45,155 typed-pass edges (91.4 %
resolved), 1,234 current claims, 4,703 rules served and 14,065 tool calls in 30 days,
12,689 gate runs over 157 plans (1,503 of 1,577 claimed rows verified at each plan's
latest run) and 235 sessions; each stage panel names its instrument, and the switches
panel is the project's own `fathom capabilities` (8 of 10 operating). Because the live
repository kept moving while the store copy did not, the source stage read **Stale
here** (indexed `95ffaf5b680e`, HEAD `437761a63569`) with the daemon-routed refresh as
its remedy — the page told the truth about a snapshot.

**The try-it is one read-only call that never counts as usage.** Every row the console
added to the scratch call log during the run carries `"origin":"console"` (33 rows: 16
`list_services`, 13 `find_callers`, 2 `find_symbol`, 1 `staleness_report`, 1
`obligations`), and the ask page's "asked here" column did not move with them.

**Live freshness works on a stage panel.** A verify-ledger row appended to the scratch
copy for `fathom` marked the gate stage "Updated — refresh this panel" with its
geometry unchanged (top 2,162 px, height 683 px before and after); the source stage,
which does not follow `plans:fathom`, was not marked.

**Found and fixed in the browser passes before the final run** (in the wave's commits): the guide's outline rail
came after the prose in the document, so a keyboard reader met every code block before
the table of contents — the rail now leads the document and the grid places it on the
right; a stale index's reason repeated the word ("stale — … stale: …"); the ask page's
example input picked `Join` (the standard library's `filepath.Join` call sites) — a
call now counts only when it resolves to one of the project's own (package, name)
pairs; indexed files no dialect pack reads showed as a language "unclassified".

**Found and not fixed** (outside the wave): the `index` capability's remedy for a stale
index is `fathom ingest --service S` (`internal/capability/registry.go`), a shell
ingest that writes a store the daemon never reads — the remedy F0 retired from the
project page (INC-3). `/learn` renders it faithfully on the extract and index stages
while the source stage shows the daemon-routed `fathom post-commit-hook`; the verb
should change, and `doctor_typed.go` and `doctor_instrument_identity.go` carry the same
advice. A try-it's latency is the tool's own: `find_callers` on `fathom` answered in
2.2 s on every call.

## How it was run

- **Binary:** `bin/fathom` built at `64c84478` (the wave's three commits, clean tree,
  `GOWORK=off`, CGO on).
- **Daemon:** `serve-http --addr 127.0.0.1:7676 --auto-refresh=false`,
  `FATHOM_SELF_RESTART=off`, `HOME` a scratch home. Store: a `sqlite3 .backup` of the
  fleet store through a read-only URI (21 services in `service_meta`, 18 listed). Call
  log, session ledger, verify ledger and hooks log: scratch copies (the first three
  91,857 / 3,278 / 15,104 rows); `--call-log`, `FATHOM_SESSION_LEDGER`,
  `FATHOM_VERIFY_LEDGER`, `FATHOM_HOOKS_LOG` and `FATHOM_CONSOLE_VIEWS` pointed at them.
  The live daemon on 7575 and its store were not touched.
- **Browser:** Chromium through the Playwright MCP. The theme was set with the page's
  appearance control (the sidebar select at 1440, the Menu disclosure's at 390), then
  each page was loaded and checked in the page: `h1` count, `data-theme`, the body
  ground, `scrollWidth − clientWidth`, the first link, the answer panel's truth state;
  console, page-error, failed-request and ≥ 400 listeners ran throughout.
- **Keyboard:** at 1440 on `/learn`, `/learn/ask` and a guide: Tab reaches the skip
  link first and Enter moves focus to `main`; then the trail, the verb chip's copy
  button, the project select, Redraw, the live stop control, the pipeline region
  (focusable, scrollable), the "pipeline as a table" disclosure and each stage's remedy
  chip and page link. On the ask page every "Try it" link is a tab stop and Enter on
  one opened `/learn/ask?svc=fathom&try=where#answer` ("1 symbol matched (exact
  match)"). On a guide the related links and the outline come before the prose. Every
  stop showed the kit's focus outline or its 2 px focus ring.

## Per-page results

All views: 1 `h1`, overflow 0 at 1440 and 390, light and dark confirmed, 0 errors.

| Page | Route | Warm render | HTML | Height 1440 / 390 | Notes |
|---|---|---|---|---|---|
| How fathom works | `/learn` | 1.0 ms | 81.5 KB | 4,122 / 8,957 | cold 9.5 s: the daemon's first model build plus this project's facts; another project's first draw 0.75 s |
| Ask the right way | `/learn/ask` | 0.8 ms | 64.8 KB | 2,315 / 6,378 | the routing table from `lg` up, one stacked entry per shape below |
| A try-it | `/learn/ask?svc=fathom&try=who-calls` | 2.2 s | 78.3 KB | 3,691 / 9,132 | the tool's own latency; answer `present` |
| Guides and glossary | `/learn/guides` | 0.7 ms | 54.1 KB | 2,393 / 4,426 | 15 guides, 20 terms, 15 laws |
| A guide | `/learn/guides/getting-started` | 1.1 ms | 70.4 KB | 13,959 / 20,301 | the outline panel below `xl` is 1,155 px (FATHOM-G-KIT-02) |
| The laws | `/learn/guides/laws` | 0.7 ms | 40.1 KB | 2,606 / 4,415 | anchors `#law-1` … `#law-15` |

Screenshots are `<page>-<width>-<theme>.png`, 24 views plus
`learn-1440-light-gate-updated.png` and two viewport views, kept outside the
repository in the wave's scratch directory `cp-G/shots/`, not committed.

## Cleanup

The daemon on 7676 was stopped. The store copy, the ledger and call-log copies and the
scratch home's `.fathom` were deleted; the shots and the daemon's log (no warning, no
error) were kept.
