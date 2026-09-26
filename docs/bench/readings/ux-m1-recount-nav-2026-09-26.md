# UX-M1 re-counted after the navigation fixes (2026-09-26)

A re-count of the ten UX-M1 journeys from `ux-m1-m2-console-2026-09-26.md`, after OC-55
(search opens plan and row pages) and OC-56/OC-57 (the plan page lists every remaining
row). Same method as that reading: HTTP-level, no browser.

## Verdict, plainly

**All ten tasks now answer in three clicks or fewer at 1440 px, and eight of ten at
390 px. All ten answers still equal their verb's JSON.** The fix was for the miss the
first reading found, "which rows of plan P remain". That task now takes 3 clicks at both
widths, down from 4 at 1440 and 5 at 390. The plan page's Remaining panel lists all 43
of the observatory plan's open rows, the same IDs as `fathom plans next`. Before, the
board showed 20 of them and hid the rest behind a closed table.

The two 390 px misses are the serving explainer (T2) and sessions by arm (T10). Both take
3 clicks at 1440. At 390 each pays the Menu tap on top of a form that costs two clicks
by itself: a choice and a submit. `docs/design/observatory-console.md` §2 records why Now
does not repeat the navigation to hide that tap.

UX-M1 stays open. Vision 35 §9 asks for scripted *browser* journeys, and Chrome still
aborts at launch on this machine.

## How it was run

- **Binary:** `bin/fathom` built from branch `fx-nav` at `0962d319`.
- **Daemon:** `serve-http` on 127.0.0.1:7702 with `--auto-refresh=false` and
  `FATHOM_SELF_RESTART=off`, under a scratch `HOME`.
- **Data:** a `sqlite3 .backup` of `workspace/db/fleet.sqlite`, and copies of the call
  log, the session, verify, hooks, intent and threads ledgers, `console-views.jsonl` and
  `~/.local/state/fathom`.
  - The fathom service's registered root is the live checkout on main (`3d58a0db`), so
    the plan answers describe main's `observatory-console.md`: 59 rows, 43 open.
  - The live daemon and its store were not touched.
- **Journeys:** the first reading's scripts (`uxm1.py`, `uxm1_tasks.py`) with the same
  counting rules. T1 now tries three paths from Now and counts the cheapest:
  - the sidebar: Plans → fathom → the plan;
  - search: the top bar's Search link → type the plan's name → Search → the plan result;
  - the fleet pulse: fathom → its Plans tab → the plan.
- **Baseline:** the same scripts against a build of `d1d92441` (before the fixes) on the
  same copies. T1 took 4 clicks at 1440 and 5 at 390, T2 and T10 took 3 and 4, and every
  other task matched the first reading.

## The ten tasks

| # | Task | Clicks 1440 / 390, before | Clicks 1440 / 390, now | Path now | Answer = verb |
|---|---|---|---|---|---|
| T1 | Which rows of plan P remain? | 4 / 5 | **3 / 3** | the fleet pulse or search, then the plan; the Remaining panel (43 rows) | yes: `plans next`, 43 IDs; `plans status` open 43 |
| T2 | What governs a file, and why was rule #8 displaced? | 3 / 4 | 3 / 4 | Serving explainer → project → file → Explain | yes |
| T3 | Which layers have stale claims? | 1 / 2 | 1 / 2 | Semantic layers | yes |
| T4 | What did fathom catch this week? | 2 / 3 | 2 / 3 | Impact → 7 days | yes |
| T5 | The daemon's p95 | 1 / 2 | 1 / 2 | Operate | yes |
| T6 | fathom's certification gaps | 2 / 2 | 2 / 2 | Now's queue → fathom | yes |
| T7 | Indexes behind HEAD | 0 / 0 | 0 / 0 | on Now | yes |
| T8 | fathom's unproven intent claims | 2 / 2 | 2 / 2 | the fleet pulse → fathom → Intent | yes |
| T9 | Tools never called | 1 / 2 | 1 / 2 | Operate | yes |
| T10 | Sessions in the OFF arm | 3 / 4 | 3 / 4 | Sessions → Arm: off → Apply | yes |

**Totals:**

| | Within 3 clicks at 1440 | Within 3 clicks at 390 | Answers equal |
|---|---|---|---|
| Before | 9 of 10 | 7 of 10 | 10 of 10 |
| Now | 10 of 10 | 8 of 10 | 10 of 10 |

**Section parity:** each page's JSON section equals an independent CLI run on nine of
the ten tasks. T4 differs only in the window's `from` instant, 2 s apart. T1's plan
section also agrees: `model.remaining` holds the same IDs as `plans next` and as the
page's own `plans next` section.

## Search on the real fleet

- An exact row ID leads in all three renderings. For `OC-12`, the JSON, the search page
  and the ⌘K fragment all put `/plans/fathom/observatory-console/rows/OC-12` first.
  The plan's file name puts its plan page first.
- Every plan result opens a page that answers 200: all 142 plans across the fleet.
  So do 300 row results, sampled from six random pages of the 2,887 plans-and-rows
  matches for `verify`, a word every row's verb chip carries.

## Page weight

After the verdicts arrived, all 142 plan pages were fetched once. Every page is under the
250 KB budget:

| Plan | Before | Now | Remaining rows now |
|---|---|---|---|
| fathom · ambient-fathom (the largest page) | 202 KB | 205 KB | 6 |
| platform-engineering · phase-3b-ci-kit (the most rows, 117) | 184 KB | 188 KB | 13 |
| fathom · observatory-console | 133 KB | 137 KB | 43 |
| platform-engineering · env-kit (the most remaining) | not measured | 132 KB | 53 |

Each row sits in exactly one table, either the Remaining table or the board's
disclosure, so a page grows by the panel and not per row. Listing the remaining rows a
second time would have cost about 205 KB of body alone for a 120-row plan with every
row open.

## Cleanup

The daemon on 7702 was stopped, and the store copy, call-log copy and scratch home were
deleted.
