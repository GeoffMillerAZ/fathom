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

**What the real fleet shows.** *Corrected 2026-09-26 (see "The survey's defects,
fixed" below): the two gate failures this paragraph first reported were false.* They
came from runs at the registered root on 2026-09-19 at `d8770983`, while the pre-push
gate's runs at the commit the root is on (in its detached clone) passed. After the fix,
on a fresh copy of the fleet: 27 attention items across 18 projects and **no gate
failure**. Six proof items: geoffmiller-cloud's 11 plans have no recorded verify run;
agentic-kit and platform-engineering each have one plan never verified; and
platform-engineering (24 plans), agentic-kit (28) and resume_as_code (4) were last
verified before their HEAD, each stated as history with its commits and dates. One
stale index, 13 projects with certificate gaps (512 declared targets, 0 live) and 7
with open observations. "Since you last looked" lists 136 changes in the week:
18 pushes (from the remote-tracking reflogs; pushes made from other clones read
"Landed", dated when this clone fetched them), 15 deploys (the first call each
daemon binary answered, from the call log's binary stamps), 16 bench readings
and this daemon's restart. The fleet first rolled up 1 failed (fathom, on
the stale runs above), 5 degraded, 0 unknown and 12 healthy; after the fix it is 0
failed, 4 degraded, 1 unknown (agentic-kit: every plan last verified before HEAD) and
13 healthy; the pulse and the table share one scale (0–3,809
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
first `list_services` and the per-service freshness reads), cached for 30 s after it.
*Corrected 2026-09-26:* this wave did add cost past the model. Its project gather
(claims, certify status with its port scan, compile runs, for every project) was keyed
on the model's build time, so it re-ran after every 30 s rebuild: 1.8–6.4 s for the
first Now or Fleet render with the model already warm. It is now cached by its inputs'
watermarks, and that first render takes 24–28 ms.

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
The views above were retaken after `tccutil reset`. *Corrected 2026-09-26:* the
certificates did not degrade honestly. `certify.Gaps` dropped an unreadable intent
corpus without saying so, so the tile fell from "of 511 declared targets in 13 repos"
to "of 284 in 5" as a plain zero, the queue lost 8 certificate items without turning
partial, and one render read "no project declares a certification target". Fixed:
the corpus files that could not be read are named, and the figure is a lower bound.

## Minor, and what is not measured

- Evidence links name the project workspace's tabs (`?tab=plans`, `?tab=proof`,
  `?tab=governance`) that wave E builds; until then they land on the project page.
- Restarts: only the serving daemon's own record is durable, so the list shows its
  latest start; earlier restarts of the same binary are not recorded anywhere.
- *Superseded 2026-09-26:* plan proof was the verify ledger's newest run at each
  registered root. It is now the run at the project's HEAD from any clone, else the
  root's newest run marked stale (below).
- Between 1,280 and about 1,400 px the fleet's ten columns scroll inside the
  table's own box (allowed by the responsive rule); a long underscore-joined
  project name wraps inside its cell.
- This is not UX-M2's certification: contrast was not measured page by page.

## The survey's defects, fixed (2026-09-26)

A survey of this wave found nine defects on the real fleet (commit `8909a5fe`, each
with a test that failed first):

1. **Stale gate failures stated as current.** A plan's proof is now the newest run at
   the project's HEAD for the same service, from the registered root or any clone.
   Only without one does the page fall back to the root's newest run, and then it is
   stale: it names the commit it proved, its date and its age. Now also declares
   `plans status` and `certify status` per project, and the parity test runs both
   verbs against the page (`plans status` after a real `fathom verify` at HEAD).
2. **An unreadable intent corpus shrank the certificates silently.** It is now a
   named lower bound (or unknown, when nothing was declared) in the tile, the queue
   (which turns partial), the Fleet cell and `certify status`.
3. **Sort by Project sorted by calls.** It now sorts by name, in both directions.
4. **Plans never verified were not on the queue.** They are now listed there, so the
   queue can no longer say "nothing needs attention" where Fleet says unknown.
5. **"Since you last looked" cut at 50.** It is now paged: every change in the window
   is on a page, and "new" counts all of them.
6. **The project gather re-ran every 30 s.** It is now cached by its inputs'
   watermarks.
7. **The unread-ledger reason was an unbounded dump.** It is now one bounded line,
   and every truth line wraps anywhere.
8. **An unknown count was reported as 0.** Attention items now carry a truth state,
   and an unknown count is omitted.
9. **"Showing N of M" counted every match.** It now names the page's row range.

**Verified on a fresh copy of the fleet** (scratch daemon on `127.0.0.1:7671`, a
`.backup` of the store, copies of the ledgers):

- `/?format=json` has no gate item. fathom's 64 plans read from the gate runs at
  `3d58a0db`: receipts.md 45 proven and 0 unproven, toolident.md 8 and 0.
  `fathom plans status` for the two plans, run in the checkout, agrees:
  claimed_unproven 0 for both.
- `plans.verified` reads 1,966 of 2,368, stale, naming its 56 plans in 3 repos
  verified before HEAD and its 3 projects with plans no run checked.
- `/fleet` rolls up 0 failed.
- `sort=service` orders the table by name in both directions.
- The since list pages 137 changes as 1–50, 51–100 and 101–137.
- Every page has one `h1`.
- `go test ./cmd/fathom/ ./internal/...` passes (94 packages).

**Not re-proven in a browser.** The post-fix views (both themes at 1440 and 390 px,
the keyboard path) could not be taken. Chrome aborts at launch on this machine in
this session: crashpad's `bootstrap_check_in` fails with error 141 and the process
gets SIGABRT, through the Playwright MCP and headless alike, over ten minutes of
retries. The installed bundle also fails `codesign -v`. That leaves this change's
doctrine rule 4 check outstanding. The markup changes are small: truth lines gain
`min-w-0 [overflow-wrap:anywhere]`, the since panel gains a kit pager, and a stale
cell reads "N stale". `task ui:css` produced no CSS change.

## Cleanup

The daemon on 7671 was stopped; the scratch store copy, the ledger copies and the
scratch home stay in the wave's scratch directory with the shots.
