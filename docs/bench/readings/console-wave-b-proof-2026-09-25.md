# Console wave B — the browser proof (2026-09-25)

The doctrine's rule 4 (`cmd/fathom/CLAUDE.md`: a 200 is not a rendering) and the
plan's gate (`docs/plans/observatory-console.md`: every page row needs a browser
proof in both themes at 1440 and 390 px, recorded in the wave's reading) for
section B: the Now page (OC-17, OC-18) and the Fleet page (OC-19, OC-20).

## Verdict, plainly

**Both pages render, in a real browser, on a copy of the real fleet.** All eight
views (Now and Fleet × 1440 and 390 px × light and dark) passed every shell check:
one visible `h1`, the skip link is the first Tab stop and Enter moves focus to
`main`, the current page is marked in the nav and the trail, no horizontal page
overflow at either width, the theme set through the page's own control, no
console error, page error or failed request, and no visible interactive target in
`main` under 44 px. The keyboard path works without a pointer: a health group
opens with Enter, the fleet filters by name and sorts by a column with the
keyboard alone, the rest of the attention queue opens with Enter, and an evidence
link lands on its project.

**The numbers are the verbs'.** On the same daemon, `fathom brief --json` (run from
the CLI, reading the store the daemon recorded, read-only) equals the Now page's
`fathom brief --json` section in its attention queue and its pulse; the summary
and the since list differ only in the two instants each run was taken (the
sessions metric's `as_of` and the window's `from`).

**What the real fleet shows.** 27 attention items across 18 projects: 2 gate
failures (fathom's `receipts.md` and `toolident.md`, from their newest runs at the
registered root, 2026-09-19), 3 projects with claimed rows the gate cannot prove
(platform-engineering 304 rows across 19 plans, agentic-kit 98 across 9, fathom
23 across 11 — each run dated in its item), 2 stale indexes (a store snapshot:
expected), 13 projects with certificate gaps (511 declared targets, 0 live) and 7
with open observations. "Since you last looked" lists 136 changes in the week:
18 pushes (from the remote-tracking reflogs; pushes made from other clones read
"Landed", dated when this clone fetched them), 15 deploys (the first call each
daemon binary answered, from the call log's binary stamps), 16 bench readings
and this daemon's restart. The fleet rolls up 1 failed, 5 degraded,
0 unknown and 12 healthy; the pulse and the table share one scale (0–3,809
calls a day) and show one absent day, 2026-09-20, when the call log holds no row.

## How it was run

- **Binary:** `bin/fathom` built from branch `cp-wave-B` (the commits below the
  proof), `GOWORK=off`, CGO on; `task ui:css` regenerated `app.css` first.
- **Daemon:** `serve-http` on `127.0.0.1:7671`, `--auto-refresh=false`,
  `FATHOM_SELF_RESTART=off`, `HOME` pointed at a scratch home. The live daemon on
  7575 and its store were not touched.
- **Data:** a `sqlite3 .backup` of the fleet store (18 services) opened through a
  read-only URI; copies of the call log (91,666 rows), the session, verify, hooks,
  intent and console-views ledgers, each pointed at by `--call-log` and its
  `FATHOM_*` variable. The projects' own repositories were read, never written:
  `.git/logs/refs/remotes/origin/*` for pushes and `git log --diff-filter=A` over
  `docs/bench/readings` for measurements.
- **Browser:** Chromium through the Playwright MCP, every check in a page of its
  own (the MCP's shared page was in use by the other waves). Theme set with the
  page's appearance control (the sidebar select at 1440, the one inside the Menu
  disclosure at 390), then reloaded; the checks ran after one Tab and one Enter,
  so the shots show `main` focused.

## Per-view results

All rows: 1 `h1` (visible) · skip link first and moves focus to `main` · nav and
trail current · `scrollWidth == clientWidth` · `data-theme` and the ground as
chosen (light `oklch(0.985 0.005 260)`, dark `oklch(0.16 0.012 260)`) · 0 console
errors · 0 failed requests · 0 visible interactive targets under 44 px.

| Page | Width | Height | Panels | Band columns | Warm render | HTML |
|---|---|---|---|---|---|---|
| Now `/` | 1440 | 3,913 | 3 | 3 | 3.8 ms | 155 KB |
| Now `/` | 390 | 7,592 | 3 | 2 (the phone composition) | | |
| Fleet `/fleet` | 1440 | 3,797 | 2 | 4 | 2.1 ms | 176 KB |
| Fleet `/fleet` | 390 | 11,291 | 2 | 1 | | |

Cold renders take 11–17 s: the observatory model's first build (the registry's
first `list_services` and the per-service freshness reads), unchanged by this
wave and cached for 30 s after it.

**Keyboard path (1440, keyboard only):** Tab 18 reaches the Degraded health group,
Enter opens it; Tab 12 more reaches the name filter, typing `kit` and Enter lands
on `/fleet?…&q=kit…` showing "2 of 18 projects"; Tab 32 reaches the Symbols header
link, Enter sorts (`/fleet?dir=desc&q=kit&sort=symbols`), and the focused link
draws a solid focus outline. On Now, Tab 39 reaches "Show the other 17 items",
Enter opens it, two more Tabs reach an evidence link, and Enter lands on
`/projects/agentic-kit?tab=proof`.

Screenshots are `<page>-<width>-<theme>.png` (8 views) plus
`keyboard-evidence-landed.png`, kept outside the repository in the wave's scratch
directory `cp-B/shots/`, not committed.

## What the proof changed

The first pass found three defects, fixed before the views above were taken
(`fix(console): the browser proof's items …`):

1. Below `xl` the fleet table reads as cards and its header row is hidden, so it
   had no sort control on a phone; the filter form now carries Sort by and Order.
2. A stale index cell read "stale stale (6c1f9d73 → 2beec118)".
3. Now's stat band stacked one tile per row at 390 px; it keeps two columns, as the
   phone mockup does.

Earlier drafts also ran the Fleet page past 240 KB by rendering every row twice (a
table and a card list); one rendering now reads as cards below `xl`
(FATHOM-B-KIT-04), and the realization pips and sparklines shrank (176 KB for 18
projects).

**An unreadable repository, seen honestly.** Mid-proof the external volume's
macOS TCC verdict wedged for the daemon's process (the known EPERM wedge): git and
`.fathom/observations.md` reads failed. The pages did what I38 asks — the stat
band read "≥ 1 wired and fresh, freshness unknown for 17 of 18 projects", the
queue turned partial ("not read: observations files (unreadable in fathom,
platform-engineering, resume_as_code and 3 more) — the queue is a lower bound"),
its index items said "freshness cannot be determined — unknown is never read as
fresh", and the open-observations tile read "unreadable", never a smaller total.
The views above were retaken after `tccutil reset`.

## Minor, and what is not measured

- Evidence links name the project workspace's tabs (`?tab=plans`, `?tab=proof`,
  `?tab=governance`) that wave E builds; until then they land on the project page.
- Restarts: only the serving daemon's own record is durable, so the list shows its
  latest start; earlier restarts of the same binary are not recorded anywhere.
- Plan proof is the verify ledger's newest run at each registered root. Many
  unproven counts come from runs dated 2026-09-02 to 09-14; each item names its
  dates, and its remedy re-runs the gate.
- Between 1,280 and about 1,400 px the fleet's ten columns scroll inside the
  table's own box (allowed by the responsive rule); a long underscore-joined
  project name wraps inside its cell.
- This is not UX-M2's certification: contrast was not measured page by page.

## Cleanup

The daemon on 7671 was stopped; the scratch store copy, the ledger copies and the
scratch home stay in the wave's scratch directory with the shots.
