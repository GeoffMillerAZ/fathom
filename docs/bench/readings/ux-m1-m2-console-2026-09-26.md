# UX-M1 and UX-M2 on the integrated console (2026-09-26)

The measurement rows of `docs/plans/observatory-console.md` (vision 35 §9), taken on the
integrated branch after waves B–G merged (`92b13169`), against a copy of the real fleet.

## Verdict, plainly

**UX-M1 fails narrowly. Nine of ten tasks take three clicks or fewer at 1440 px, and all
ten on-screen answers equal their verb's JSON.** The miss is the first example in the
vision: *which rows of plan P remain*. The plan page's board shows the lane counts
(Remaining 35, Op waiting 8) at three clicks. It lists only the first 12 cards of each
lane, though: 20 of the 43 rows. The full list is behind a closed "Every row as a table"
disclosure, which makes a fourth click. Search does not shorten the path, because a plan
or row result links to the project page, not to the plan (finding 1). At 390 px every
sidebar destination sits one Menu tap deeper, so 7 of 10 tasks fit in three clicks.

**UX-M2 is not certified, because no browser could be launched.** The Playwright MCP's
Chrome aborts at launch on this machine (crashpad `bootstrap_check_in` error 141,
SIGABRT). That is the same failure the wave-B fix pass hit earlier today. So these were
not measured on any page:

- both themes as rendered;
- horizontal overflow at 390 px as laid out;
- console errors;
- a keyboard path as focused.

No screenshots exist. The shots directory is empty.

What the served HTML alone can show holds on all 31 registered pages:

- one `h1` each;
- the skip link is the first focusable element, and its target exists;
- the seven nav groups appear in both the sidebar and the mobile menu;
- breadcrumbs are present;
- all 349 `th` elements carry `scope`;
- no fixed width wider than a 390 px content box sits outside a scroll container;
- JSON answers on every page;
- 490 of 491 internal links return 200 (the other is a POST form action, 405 to GET);
- the three script assets and all 32 inline scripts parse.

**The HTML budget passes and the render budget does not.**

- **HTML:** every page is 49–193 KB, and so are 16 heavier variants, against the
  250 KB budget.
- **Warm render, typical:** the median is 0.4–82 ms and the p95 is 0.5–88 ms on all 31
  pages (185 samples each over 150 s).
- **Warm render, stalls:** 11 pages had at least one request over 150 ms. Two causes
  (findings 2 and 3):
  - the 30 s observatory model rebuilds on the request path, stalling one request per
    window for 3.6–4.4 s;
  - the intent tab re-runs `intent audit` inline, stalling for 22.5 s.
- **Cold first render, with the model already warm:** over the 1 s budget on 9 pages,
  up to 26 s.

**Also found: this week's "conserve losses" are test-fixture rows.** All 55 of them, on
both `/impact` and `fathom metrics catches`, come from service `svc`. That is a test
fixture's name, written into the live daemon's call log (finding 4). The page and the
verb agree with each other, and both are wrong about what fathom caught.

## How it was run

- **Binary:** `bin/fathom` built at `92b13169` from a clean tree (`GOWORK=off`, CGO on).
  `bash scripts/ui-css.sh` left `app.css` unchanged.
- **Daemon:** `serve-http --addr 127.0.0.1:7690 --auto-refresh=false --store <copy>
  --call-log <copy>`, with `FATHOM_SELF_RESTART=off`.
  - `HOME` was a scratch home. Its `config.toml` holds only `[daemon]` store, addr,
    call_log and auto_refresh (no LLM profile).
  - Scratch copies stood in for every ledger: `FATHOM_VERIFY_LEDGER`,
    `FATHOM_SESSION_LEDGER`, `FATHOM_HOOKS_LOG`, `FATHOM_INTENT_LEDGER`,
    `FATHOM_THREADS`, `FATHOM_CONSOLE_VIEWS`, `FATHOM_SUPPRESSIONS`, `FATHOM_PURGES` and
    `FATHOM_SESSION_STAMPS`.
  - The daemon log holds no warning or error beyond the known broken
    `resume-as-code-standards` layer pack, which `/layers` also names.
  - The live daemon on 7575 and its store were not touched.
- **Data:**
  - a `sqlite3 .backup` of `workspace/db/fleet.sqlite` (188 MB, 18 services listed);
  - copies of:
    - the call log (94,682 rows at start);
    - the session ledger (3,288 rows);
    - the verify ledger (15,361 rows);
    - the hooks log (72,937 rows, plus its rotated file);
    - the intent ledger (159 rows);
    - `~/.local/state/fathom`.

  The fathom service's registered root is the live checkout on `main` (`3d58a0db`). The
  plan answers below therefore describe main's `observatory-console.md` (59 rows, 14
  proven), not this branch's copy. The projects' repositories were read, never written.
- **CLI side:** each verb ran against the same daemon (`--url`) and store (`--store`),
  with the same ledgers and `FATHOM_CALL_LOG` unset. Its call log therefore resolved the
  way an operator's does, by the daemon's record (rule `daemon`).
- **UX-M1 journeys** (`uxm1_tasks.py` in the scratch directory):
  - Each journey starts at `/` and follows only links, or GET forms, present in the
    served HTML of the page it is on.
  - Counted as one click each: following a link, choosing a `<select>` option, pressing
    a submit button, and opening a closed `<details>`. Typing counts as no click.
  - At 390 px the sidebar is `hidden md:flex`, and a nav link costs the Menu disclosure
    too.
  - The on-screen answer is read from the final page's text. It is compared with an
    independent CLI run taken right after the page. Separately, each page's JSON section
    is diffed against that CLI run, ignoring only instants (`generated_at`,
    `since`/`until`, `from`) and the call-log provenance note.
- **UX-M2 static pass** (`uxm2_static.py`): every registered page, one concrete URL per
  parameterized route. Each page's HTML is parsed with Python's `html.parser`.
  - The tab stops are counted in DOM order to the first control in `main` outside the
    heading and live bar. They skip `hidden`, closed-`<details>` and breakpoint-hidden
    elements, and they are counted without JavaScript.
  - The budgets use `curl` on loopback: three warm runs per page, then a 150 s loop of
    all 31 pages (185 rounds).

## Browser: why there is none

The Playwright MCP launches `/Applications/Google Chrome.app` (153.0.8010.53), which
aborts before its first page:

```
bootstrap_check_in org.chromium.crashpad.child_port_handshake…: unknown error code (141)
ReadExactly: expected 4, observed 0
<process did exit: exitCode=null, signal=SIGABRT>
```

Other routes failed or were not available:

- **Headless from the shell:** a launch with crashpad disabled aborts the same way
  (exit 134).
- **The Chrome bundle:** `codesign -v` reports an invalid signature. Its framework
  directory holds three versions (152.0.7977.84, 153.0.8010.50, 153.0.8010.53), which
  suggests an update staged in place.
- **Other browsers:** there is no other build on the machine. The `ms-playwright` caches
  hold profiles only, and npm had no network to fetch Playwright's own Chromium.
- **Outside the sandbox:** a launch outside the tool sandbox was not permitted in this
  session.
- **Reinstalling Chrome:** not done. It is the owner's browser.

Either of two things lets the browser half be taken:

- relaunching Chrome so the staged update completes;
- installing Playwright's Chromium.

The browser half is these checks, at 1440 and 390 px in both themes:

- the theme set through the page's control;
- `scrollWidth == clientWidth`;
- the console, page-error and failed-request listeners;
- the keyboard path;
- 44 px targets;
- screenshots.

## UX-M1 — the ten tasks

The first five are the vision's examples. The last five are drawn from the design's page
list: certifications, the fleet, intent, Operate's tools and sessions.

| # | Task | Path from Now | Clicks 1440 / 390 | On screen | Verb and its answer | Equal |
|---|---|---|---|---|---|---|
| T1 | Which rows of plan P (fathom · observatory-console) remain? | Plans → fathom → observatory-console → open "Every row as a table" | **4 / 5** | 43 rows open or waiting (35 remaining + 8 op), ids listed. At 3 clicks: lane badges 35 and 8 and 20 of the 43 cards | `plans next --plan … --json`: the same 43 ids. `plans status`: open 43, proven 14 of 59 | yes |
| T2 | What governs `internal/conserve/conserve.go`, and why was rule #8 displaced? | Serving explainer → choose project → type the file, press Explain | 3 / 4 | 6 served (ancestor-directory tier, `internal/CLAUDE.md`), 178 displaced of 185. #8 "Sort every output slice…": "rank — same scope tier as the rule served at #6; ranked lower on rule text order" | `obligations --file … --layer rules --explain --json`: the same 6 in order. Rank 8 `displaced.reason: rank`, same `why` | yes |
| T3 | Which semantic layers have stale claims? | Semantic layers | 1 / 2 | adr-rules 1, obligations 10, spec-bindings 19 | `layers strata --json`: the same `statuses.stale` | yes |
| T4 | What did fathom catch this week? | Impact → 7 days | 2 / 3 | 282: verify refusals 217, conserve losses 55, integrity catches 2, guard refusals 8 | `metrics catches --days 7 --json`: the same (but see finding 4) | yes |
| T5 | What is the daemon's p95? | Operate | 1 / 2 | agent surface p95 5.7 s (3,256 calls); unscoped p95 11.4 s | `metrics traffic --json`: agent `p95_ms` 5,678; (unscoped) 11,376 | yes |
| T6 | Which of fathom's certification targets lack a live certificate? | Now's queue ("show the other items") → fathom, or Certifications → fathom | 2 / 2 | 77 targets, 0 live, 77 gaps; the gap table pages 50 at a time | `certify status --json`: declared 77, live 0, gaps 77. `certify gaps`: 77 targets | yes |
| T7 | Which projects' indexes are behind HEAD? | on Now | 0 / 0 | app-kit (54f5fac8 → 406297dc), platform-engineering (6c1f9d73 → 2beec118) | `fleet report --json`: the same two stale, same commits | yes |
| T8 | How many of fathom's intent claims are unproven, over how many invariants? | Now's fleet pulse → fathom → Intent tab | 2 / 2 | invariants 46, unproven claims ≥ 113 | `intent audit --read-only --json`: 46 invariants, 113 unproven (U4 97, U12 10, U1 4, U6 2) | yes |
| T9 | Which registered tools has no one ever called? | Operate | 1 / 2 | 23 of 59, named (`api_diff` … `traverse`) | `instrument --adoption --json`: the same 23 of 59 | yes |
| T10 | Which sessions ran in the OFF arm? | Sessions → Arm: off → Apply | 3 / 4 | 11 sessions, ids in order | `metrics sessions --arm off --json`: matching 11, the same rows | yes |

**Section parity** compares the page's JSON section for each verb with the independent
run:

- **Equal:** T2, T3, T5, T6, T7, T8 and T10.
- **T1:** equal. The only difference is the plan path, which is repo-relative on the page
  and absolute in my run from outside the repository.
- **T4:** differs only in the window's `from` instant, 2 s apart.
- **T9:** differs by exactly the 5 call-log rows my own CLI `intent audit` run appended
  between the two reads. They were counted by timestamp: `query_symbols` ×3,
  `list_services`, `verify_plan`.

**Totals:**

| Width | Tasks within 3 clicks | Answers equal |
|---|---|---|
| 1440 px | 9 of 10 | 10 of 10 |
| 390 px | 7 of 10 | 10 of 10 |

At 390 px, T1 takes 5 clicks, and T2 and T10 take 4 each.

## UX-M2 — every registered page

All 31 rows have:

- one `h1`, and the skip link first with an existing target;
- the seven nav groups in the sidebar and the mobile menu;
- the current destination marked in both navs (except `/search`, which is not a nav
  destination);
- the theme control present (disabled until its script runs);
- 0 `th` without `scope`;
- 0 fixed widths over 358 px outside a scroll container;
- 0 `dark:` classes;
- no literal palette class except the kit-owned `backdrop:bg-black/40`;
- 0 positive `tabindex`;
- `?format=json` 200.

"Crumbs" counts trail entries including separators. The tab stops are counted from the
top of the page. With the skip link it is Tab, then Enter, then the heading's trail and
chips.

| Page | Route | h1 | Crumbs | Tables (th, no scope) | Tab stops 1440 / 390 | HTML | Warm median / p95 / max (ms) | ≤150 ms |
|---|---|---|---|---|---|---|---|---|
| home | `/` | Now | 3 | 20, 0 | 26 / 9 | 171.2 KB | 4.4 / 6.2 / 36.3 | yes |
| search | `/search?q=verify` | Search: verify | 3 | 0, 0 | 23 / 6 | 63.4 KB | 38.4 / 39.3 / 76.9 | yes |
| fleet | `/fleet` | Fleet | 3 | 25, 0 | 24 / 7 | 187.6 KB | 1.2 / 1.4 / 3,955.9 | no — 1 of 185 over |
| project | `/projects/fathom` | fathom | 5 | 0, 0 | 25 / 8 | 70.9 KB | 0.6 / 0.7 / 3.3 | yes |
| project-index | `/projects/fathom/index` | fathom · Index | 7 | 11, 0 | 26 / 9 | 67.2 KB | 0.5 / 0.7 / 2.9 | yes |
| project-intent | `/projects/fathom/intent` | fathom · Intent | 7 | 22, 0 | 26 / 9 | 192.6 KB | 13.1 / 14.6 / 22,452.2 | no — 1 of 185 over |
| project-activity | `/projects/fathom/activity` | fathom · Activity | 7 | 19, 0 | 26 / 9 | 92.6 KB | 0.8 / 2.8 / 4,100.1 | no — 9 of 185 over |
| project-wiring | `/projects/fathom/wiring` | fathom · Wiring | 7 | 5, 0 | 26 / 9 | 68.3 KB | 1.0 / 1.3 / 1,736.4 | no — 3 of 185 over |
| plans | `/plans` | Plans | 3 | 19, 0 | 25 / 8 | 174.8 KB | 11.5 / 43.0 / 71.4 | yes |
| plan | `/plans/fathom/observatory-console` | The Observatory console — the pla… | 7 | 9, 0 | 26 / 9 | 133.0 KB | 1.9 / 2.2 / 6.9 | yes |
| plan-row | `/plans/fathom/observatory-console/rows/OC-1` | OC-1 · Page projections carry a t… | 9 | 0, 0 | 27 / 10 | 49.0 KB | 77.4 / 88.0 / 118.5 | yes |
| certifications | `/certifications` | Certifications | 3 | 26, 0 | 25 / 8 | 175.7 KB | 1.4 / 70.3 / 3,896.3 | no — 1 of 185 over |
| refactors | `/refactors?svc=fathom` | Refactor · fathom since HEAD~1 | 5 | 11, 0 | 25 / 8 | 59.9 KB | 11.9 / 15.0 / 46.0 | yes |
| layers | `/layers` | Semantic layers | 3 | 14, 0 | 27 / 10 | 128.5 KB | 41.6 / 43.9 / 74.5 | yes |
| layer | `/layers/obligations` | obligations | 5 | 7, 0 | 26 / 9 | 127.8 KB | 38.9 / 40.4 / 73.1 | yes |
| claims | `/claims` | Claims | 5 | 6, 0 | 26 / 9 | 112.0 KB | 40.4 / 44.2 / 86.4 | yes |
| claim | `/claims/21aee289…?svc=fathom` | Claim 21aee289 | 5 | 0, 0 | 25 / 8 | 57.2 KB | 8.3 / 8.8 / 23.6 | yes |
| packs | `/packs` | Packs | 3 | 18, 0 | 25 / 8 | 91.8 KB | 81.8 / 83.3 / 239.0 | no — 2 of 185 over |
| explain | `/explain?svc=fathom&file=internal/conserve/conserve.go` | Serving explainer | 3 | 11, 0 | 24 / 7 | 111.3 KB | 14.4 / 14.7 / 332.9 | no — 5 of 185 over |
| impact | `/impact` | Impact | 3 | 17, 0 | 24 / 7 | 155.5 KB | 4.3 / 9.2 / 4,441.2 | no — 5 of 185 over |
| sessions | `/sessions` | Sessions | 3 | 9, 0 | 24 / 7 | 190.3 KB | 16.1 / 16.5 / 32.7 | yes |
| session | `/sessions/001f8dee-…` | Session 001f8dee | 5 | 4, 0 | 25 / 8 | 69.5 KB | 4.6 / 4.9 / 126.0 | yes |
| brief | `/brief` | Brief | 3 | 0, 0 | 25 / 8 | 53.4 KB | 19.6 / 20.5 / 54.4 | yes |
| operate | `/operate` | Operate | 3 | 39, 0 | 26 / 9 | 177.7 KB | 30.3 / 32.5 / 360.2 | no — 5 of 185 over |
| statusz | `/statusz` | Daemon status | 3 | 4, 0 | 24 / 7 | 82.3 KB | 15.8 / 16.0 / 19.8 | yes |
| wiring | `/wiring` | Wiring | 3 | 0, 0 | 24 / 7 | 56.0 KB | 13.7 / 14.1 / 3,880.2 | no — 1 of 185 over |
| decide | `/decide` | Decision port | 3 | 18, 0 | 24 / 7 | 120.4 KB | 1.6 / 1.8 / 6.2 | yes |
| learn | `/learn` | How fathom works | 3 | 19, 0 | 26 / 9 | 85.6 KB | 3.7 / 4.0 / 285.9 | no — 1 of 185 over |
| learn-ask | `/learn/ask` | Ask the right way | 3 | 5, 0 | 26 / 9 | 71.7 KB | 3.1 / 3.3 / 8.0 | yes |
| learn-guides | `/learn/guides` | Guides and glossary | 3 | 2, 0 | 24 / 7 | 60.8 KB | 0.4 / 0.5 / 1.4 | yes |
| learn-guide | `/learn/guides/getting-started` | Getting started: wire fathom into… | 5 | 9, 0 | 24 / 7 | 77.4 KB | 0.8 / 0.9 / 2.7 | yes |

**Cold first renders**, each page's first request after `/` had built the model (9.0 s):

- over 1 s:
  - the plan page, 12.0 s;
  - the intent tab, 26.0 s;
  - `/refactors?svc=fathom`, 10.0 s (a new range: 10 s again);
  - a plan row, 5.3 s;
  - the activity tab, 4.7 s;
  - `/certifications`, 2.1 s;
  - `/plans`, 1.6 s;
  - the index tab, 1.3 s;
  - the wiring tab, 1.2 s;
- under 1 s: every other page.

**Heavier variants**, all within the HTML budget:

- a 1,807-turn session, 172 KB;
- `/claims?page=63`, 86 KB;
- `/?since_page=3`, 166 KB;
- `/projects/platform-engineering/intent`, 140 KB;
- `/fleet?sort=symbols`, 188 KB;
- the rest are under 133 KB.

**Error pages** render on the shell with an `h1`, the skip link and the nav:

- 404 for an unknown route ("Not found"), project ("Unknown project") and plan
  ("Unknown plan");
- 400 for an over-long search ("Search too long").

**Not measured, because each needs a browser:**

- both themes as rendered, and contrast;
- real overflow at 390 px;
- console, page and request errors at runtime;
- focus visibility and the keyboard path as walked;
- 44 px targets as laid out;
- screenshots.

## Findings, not fixed here

1. **Search sends plans and rows to the project page.** `projectSearchDocs`
   (`cmd/fathom/console_search.go`) sets `Href: projectHref(service)` for both kinds.
   - That code predates wave C's plan pages. The ⌘K fragment uses the same links, so
     even an exact row ID (`OC-12`) lands on `/projects/fathom`.
   - Fix: `planHref(service, name)` for a plan result and
     `planRowHref(service, name, id)` for a row result. With that, T1 through search is
     three clicks: search, the plan, the table.
2. **The observatory model rebuilds on the request path.** `cachedObservatoryModel`
   (`cmd/fathom/observatory.go`) rebuilds under its mutex once the 30 s TTL lapses.
   - On the real fleet one request per window waits 3.6–4.4 s, and concurrent requests
     wait with it.
   - It was seen on `/`, `/plans`, `/fleet`, `/certifications`, `/impact`, `/wiring`,
     `/operate` and the project tabs.
   - It reproduced twice. After 31 s idle, `/learn/guides` (which needs no model)
     rendered in 1.9 ms. The next `/fleet` took 3.60 s and 3.64 s, and the one after it
     1.3 ms.
   - Fix: serve the previous model while a rebuild runs in the background.
3. **The intent tab runs `intent audit` inline.** On fathom that takes 21–26 s.
   - It happens every time the audit key moves (the root's HEAD, the plans generation,
     the intent ledger) and every 5 minutes (`intentAuditTTL`). It stalled once in the
     150 s sampling.
   - Fix: as in 2, show the previous audit marked stale while the new one runs.
4. **Tests write `fathom review` rows into the live daemon's call log.**
   - The log holds 205 `review` rows for service `svc`, 148 of them on 2026-09-25/26.
     `svc` is no registered project and is the service name the review and conserve
     tests use.
   - `reviewCallLogPath` resolves the daemon record under the real `HOME` (OC-2), so a
     test that runs the review path without pinning `FATHOM_CALL_LOG` or `HOME` appends
     to the production log.
   - Every one of this week's 55 conserve losses on `/impact` and in `metrics catches`
     is such a row.
   - Fix: pin the call log (or `HOME`) to `t.TempDir()` in those tests. Consider also
     that a catch naming an unregistered service should read as unattributed, not as a
     project.
5. **Minor:**
   - The design's `/intent` route and vision §3's Evidence → Intent entry are not
     registered (404). Intent lives only as the project tab (OC-38).
   - The explainer's project select has no default, so the form costs a choice before
     the file.

## Screenshots and scratch

None were taken, because no browser ran. `cp-proof/shots/` is empty. The journey and
static-pass scripts, their JSON results, the page HTML and the CLI outputs stayed in the
run's scratch directory.

## Cleanup

The daemon on 7690 was stopped. The store copy, the call-log copy and the scratch home
(ledger copies and state) were deleted.
