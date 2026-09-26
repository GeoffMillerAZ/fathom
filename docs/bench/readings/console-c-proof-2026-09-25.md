# Console wave C — the Proof pages' browser proof (2026-09-25)

The doctrine's rule 4 (`cmd/fathom/CLAUDE.md`: a 200 is not a rendering) and the
plan's gate (`docs/plans/observatory-console.md`: every page row needs a browser
proof in both themes at 1440 and 390 px, recorded in the wave's reading), for
OC-21..OC-26 and the rows they carry (RG-5, SD-2, SD-3).

## Verdict, plainly

**Wave C renders, and it now tells the truth when it cannot read something.**
Ten views of the six new surfaces (`/plans` fleet and one repository, a plan, a
row with a reach path, a row with a lexical oracle, `/certifications` fleet and
one repository, `/refactors` and one comparison, and the project overview's
ladder) were opened in a real browser at 1440×900 and 390×844, light and dark:
40 views, every one a 200 with exactly one `h1`, the skip link as the first Tab
stop, the nav marking the right page, breadcrumbs ending in the current page, no
horizontal page overflow, and no console errors or failed requests. The row
drawer was driven by keyboard alone at 1440 light and 390 dark: Tab reaches a
board card, Enter opens the drawer with that row's evidence, focus stays in the
dialog, Escape closes it and focus returns to the card.

**Every defect the proof found is fixed in this wave (eight, listed under
"What is broken").** The
serious ones were about honesty, not layout. The external volume's reads were
being refused (the machine's recurring TCC wedge) while the proof ran, and three
readers turned "could not read" into "nothing there":

1. `/plans` said **"0 plans across 0 repositories"** and a plan's page said
   **"Unknown plan"** (404). Plan discovery used `filepath.Glob`, which drops a
   directory read error and answers "no matches". The verbs (`plans
   status|next`) had the same hole.
2. `/certifications?svc=fathom` said **"no manifests found — nothing
   declared"** for a repository with 77 targets. `certify.Gaps` silently skipped
   an unreadable intent corpus, `incidents.md` and port source files, so the
   targets vanished and the "nothing declared" skip was claimed.
3. The realization ladder's gating rung would read **not applicable** ("no plans
   here") for a repository whose `docs/plans` could not be read.

Fixed: each now names the failure. An unreadable `docs/plans` is an error (the
plan page answers 503 "Plans unreadable", seen live during a wedge); `certify
gaps|status` carry `unread` and say INCOMPLETE, and the certifications page
shows that repository as an error; the gating rung reads unknown. A console read
that failed is asked again (after 30 s for a plan or a conserve report, on the
next read for a certify report), never cached for good. Before, a plan whose
verification failed during a wedge stayed failed until its file, the index or
HEAD changed.

The rest were cost, layout or wording: the plan cache re-verified every plan
of a repository on each of its commits; the refactor page counted an
informational finding (an edited test) as a loss against a plane conserve called
conserved; a six-lane board hid its last two lanes behind a scroll at 1440; a
row page's sentence-long last breadcrumb wrapped under the trail; a truth
state's reason with a long path overflowed its panel at 390 (the page did not,
the panel did); and the fleet pages' "registry unreachable" error read "project
&nbsp;cannot be looked up". All fixed and each pinned by a test.

**What still does not work, and is not fixed here** (details below): `fathom
conserve`'s own plan discovery and `intent.Load`'s directory listing still use
`filepath.Glob`; the kit's widget header squeezes a panel title beside a long
verb chip at 390; and while the daemon verified all 142 plans from cold, its own
5 s registry read timed out now and then, so a page answered 503 "Registry
unreachable" until the load passed.

## How it was run

- **Binary:** `fathom` built from branch `cp-wave-C`: the tree of `9d087ff3`
  less the last two fixes named under "Binary drift" below, with the module's
  own Go env (`GOWORK=off`, CGO on).
- **Daemon:** `serve-http` on `127.0.0.1:7672`, `--auto-refresh=false`,
  `FATHOM_SELF_RESTART=off`. `HOME` pointed at a scratch home. The live daemon on
  7575 and its store were not touched.
- **Data:** a scratch copy of the fleet store (`sqlite3 .backup`, 18 services),
  copies of the call log (92,831 rows at the end), the session ledger and the
  verify ledger (14,912 rows over 28 days, so the portfolio's trend draws and
  each short-lived plan's own trend is `stat.Absent`). Plans are read from each
  service's registered root, which for `fathom` is the live checkout — other
  sessions committed to it during the proof, so HEAD moved under the pages.
- **Browser:** Chromium through the Playwright MCP, each width and theme in its
  own isolated browser context (other agents share the MCP browser). Theme set
  with the page's own appearance control (at 390 inside the Menu disclosure),
  never by script. Checks ran in the page: `h1` count, `data-theme` and the body
  ground, `scrollWidth` against `clientWidth` with offenders listed, the nav's
  current item, breadcrumbs and `aria-current`, the first Tab stop, and
  console/page-error/failed-request/≥400 listeners. A second pass at 390 listed
  every element in `main` that scrolls or overflows on its own.
- **Waiting for truth:** every page was captured only after the background
  verifier reported all 142 plans verified (`/plans?format=json`: 142 present,
  no pending section). Cold, that took 10–12 minutes: the TCC wedge refused the
  daemon's reads every minute or two, the failed plans were retried, and
  `tccutil reset SystemPolicyRemovableVolumes com.mitchellh.ghostty` restored
  access each time.

## Per-page results

All 40 views: status 200 · 1 visible `h1` · skip link first, target present ·
first Tab = the skip link · light `oklch(0.985 0.005 260)` / dark
`oklch(0.16 0.012 260)` ground with the matching `data-theme` · no page
overflow at 1440 or 390 · 0 console errors, 0 failed requests.

| Page | Route | h1 | Nav | Trail | Height 1440 / 390 | Tables · charts |
|---|---|---|---|---|---|---|
| Plans | `/plans` | Plans | Plans | Observatory / Plans | 6241 / 9252 | 3 · 68 |
| Plans, one repo | `/plans?svc=fathom` | Plans | Plans | Observatory / Plans | 5599 / 8427 | 2 · 65 |
| Plan | `/plans/fathom/observatory-console` | the plan's title | Plans | … / Plans / fathom / the plan | 3960 / 4648 | 2 · 3 |
| Row, reach | `…/rows/OC-2` | OC-2 · title | Plans | … / the plan / OC-2 | 2246 / 3298 | 0 · 0 |
| Row, oracle | `…/rows/OC-12` | OC-12 · title | Plans | … / the plan / OC-12 | 1722 / 2307 | 0 · 0 |
| Certifications | `/certifications` | Certifications | Certifications | Observatory / Certifications | 7648 / 21868 | 4 · 1 |
| Certifications, one repo | `/certifications?svc=fathom` | Certifications | Certifications | Observatory / Certifications | 4474 / 15983 | 3 · 1 |
| Refactors | `/refactors` | Refactors | Refactors | Observatory / Refactors | 900 / 844 | 0 · 0 |
| Refactor | `/refactors?svc=fathom&since=HEAD~5` | Refactor · fathom since HEAD~5 | Refactors | … / Refactors / Refactor | 1722 / 3115 | 1 · 0 |
| Project (ladder) | `/projects/fathom` | fathom | Fleet | Observatory / Fleet / fathom | 2247 / 4561 | 0 · 17 |

What the data showed, and why it is right:

- **Plans:** 142 plans across 7 repositories; the gate proves 1,978 rows and
  428 are claimed but not proven (15 failing, 413 the gate cannot decide); 39
  more are measured stale because the daemon binary is not HEAD, and are not
  counted as unproven (PC-5). The trend draws 14 of 28 ledger days; the rest are
  in the JSON.
- **Plan:** `observatory-console.md` at 14 proven, 35 open, 8 measurement rows
  waiting, 2 deferred. Its own proven trend reads "the verify ledger has rows
  on 2 days; the trend needs 14 — not drawn from too little data" (SD-3). Empty
  lanes are named under the board ("No rows are measured (op) — those lanes are
  not drawn"); claimed-not-proven is drawn even when empty.
- **Row, reach:** OC-2's path from `TestResolveCallLog_PrefersDaemonConfiguredLog`
  to `ResolveCallLog` (1 hop), 477 reaching tests (20 listed, the rest counted),
  and its history (added `a683d602`, claimed `417a7587`). Per-row refutations
  read `stat.Absent`: the ledger records per-plan counts, not per-row verdicts.
- **Row, oracle:** OC-12 on the live checkout reads `path-absent` — its oracle
  names `cmd/fathom/console_palette.go`, which that checkout does not have. The
  page reports the tree it was given; that is not a console defect.
- **Certifications:** 511 targets across 13 declaring repositories, 0 with a
  live certificate — every target is a gap, each with its remedy.
- **Refactor:** six of seven planes conserved; `generated` reads "not run — no
  plane by this name is registered in this build". The reading set classes
  nothing and says why: the index was ingested at `95ffaf5b`, HEAD is newer.
- **Ladder:** wired, queried, preventing and gating met; fresh not met (the
  store copy is a snapshot), so the remedy is the post-commit hook command.

At 390 the only horizontal scrolling is inside tables and the board, each in its
own box (the responsive rule's allowance). Screenshots
`<page>-<width>-<theme>.png` (40, plus `plan-drawer-1440-light.png` and
`plan-drawer-390-dark.png`) are kept outside the repository in the proof's
scratch directory `cp-C/shots/`, not committed.

## Keyboard: the row drawer

| | 1440 light | 390 dark |
|---|---|---|
| Tabs from page top to the first board card | 24 | 14 |
| Enter opens the drawer, evidence for that row | OC-12, yes | OC-12, yes |
| Focus inside the dialog after opening | yes | yes |
| Tab cycles the dialog (evidence, Open as a page, Close, browser chrome, back in) | yes | yes |
| Escape closes; focus returns to the card | yes | yes |
| URL unchanged (the page link is in the footer) | yes | yes |

## Render cost (final binary, curl, first and second read after warming)

| Route | Time | HTML |
|---|---|---|
| `/plans` | 0.07 s, then 0.01 s | 166 KB |
| `/plans?svc=fathom` | 0.007 s | 144 KB |
| `/plans/fathom/observatory-console` | 0.03 s, then 0.002 s | 122 KB |
| `…/rows/OC-2` | 0.65 s, then 0.09 s (the call graph walk, cached) | 44 KB |
| `…/rows/OC-12` | 0.001 s | 41 KB |
| `/certifications` | 2.8 s when a repository's HEAD moved, then 0.002 s | 161 KB |
| `/certifications?svc=fathom` | 0.001 s | 125 KB |
| `/refactors` | 0.0005 s | 29 KB |
| `/refactors?svc=fathom&since=HEAD~5` | 0.01 s warm; a cold read waits its 10 s budget and says it is computing | 46 KB |
| `/projects/fathom` | 0.001 s | 62 KB |

A plan whose repository only moved HEAD is not re-verified through the daemon:
its verdicts are kept and the working-tree overlays re-applied (before this
wave's last change, every commit to the live checkout re-verified all 64 of its
plans, and `/plans` read "2 of 142 plans verified" for minutes).

## What is broken

### Fixed in this wave (found by this proof)

1. **Unreadable read as empty (I38), three readers.** `activePlanFiles`
   (`cmd/fathom/plans.go`), `certify.Gaps` (`internal/certify/gaps.go`, `port.go`)
   and `repoHasPlans` (`cmd/fathom/realization.go`) — described above. Pinned by
   `TestActivePlanFiles_UnreadableIsNeverZeroPlans`,
   `TestGaps_UnreadableSourceIsNamedNeverFewerTargets` and the ladder and
   `repoHasPlans` tests.
2. **Failures cached for good.** The console's plan, conserve and certify
   caches kept an error until the plan, the index or HEAD changed. Pinned by
   `TestConsolePlans_HeadOnlyChangeReusesDaemonVerdicts`.
3. **Re-verifying on every commit.** The plan cache keyed on HEAD, so a busy
   repository never finished verifying. Same test.
4. **An informational finding counted as a loss** on a conserved plane
   (`TestRefactorView_NotRunIsNeverConserved`; `conserve.Finding.Informational`).
5. **Six lanes wider than the screen** (`TestConsolePlanBoard_LanesAreBounded`).
6. **A sentence as the last crumb** (`TestConsolePlanRow_EvidenceReachOracleAndFragment`).
7. **A long reason overflowing a 390 panel** (truth-state lines wrap anywhere).
8. **"project &nbsp;cannot be looked up"** on a fleet page
   (`TestConsoleProofPages_UnreachableRegistryIsNamed`).

### Not fixed

9. **Two more `filepath.Glob` readers.** `discoverPlanPaths`
   (`cmd/fathom/conserve.go:145`, the plans `fathom conserve` reads) and
   `intent.Load`'s listing of `docs/intent/*.md` (`internal/intent/corpus.go:209`)
   still read an unreadable directory as empty. `intent.Load` records unreadable
   *files* in its coverage manifest, but not a directory it could not list.
10. **The daemon's own registry read under load.** While the verifier worked
    through 142 plans from cold, `list_services` over HTTP to itself took 6.2 s
    against the model's 5 s budget, and some page loads answered 503 "Registry
    unreachable". Upstream `95ffaf5b` makes the daemon's registry read
    in-process; re-check after the merge before changing the budget or the
    worker count.
11. **The kit's widget header at 390** (FATHOM-C-KIT-04 in `docs/app-kit.md`):
    a long verb chip squeezes the panel title to one word per line. Nothing
    overflows.
12. **The board scrolls at 390.** Six fixed-width lanes cannot fit a phone; the
    board scrolls in its own box and the table under it lists every row.

### Binary drift

The browser run used a binary built before fixes 7 and 8. Both are pinned by
tests (`TestTruthStateLine_ReasonWrapsAnywhere`,
`TestConsoleProofPages_UnreachableRegistryIsNamed`). Fix 7 was found by the same
run: a second pass at 390 listed every element in `main` wider than its box, and
the only ones outside a table or the board were a truth-state reason line on the
OC-12 row (its drawer's body held 543 px of content in 360 px) and on the refactor
page's reading set (a 40-character SHA). A re-run on the final binary was
attempted and could not start: by then Chrome would not launch in this session
(`bootstrap_check_in … unknown error code (141)`, alongside `pgrep: Cannot get
process list` and `tccutil: No such bundle identifier`), so the final binary
was checked with `curl` only — every route above answered 200 at the sizes and
times in the render-cost table.

## Environment

The machine's internal disk filled to 100 % during the proof (other sessions'
scratch; this lane used about 450 MB). The scratch daemon's SQLite reads failed
with "database or disk is full" and a link failed with ENOSPC, so the last build
and the tests used Go temp and `TMPDIR` on the external volume
(`/Volumes/m2ext/tmp/cp-C-tmp`, deleted afterwards).

## Cleanup

The daemon on 7672 was stopped; the scratch store copy, ledger copies, scratch
home and external temp directory were deleted. The screenshots were kept.
