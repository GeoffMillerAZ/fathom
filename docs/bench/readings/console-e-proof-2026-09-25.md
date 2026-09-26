# Console wave E — the project workspace, browser proof (2026-09-25)

The doctrine's rule 4 (`cmd/fathom/CLAUDE.md`: a 200 is not a rendering) and the
plan's gate (`docs/plans/observatory-console.md`: every page row needs a browser
proof in both themes at 1440 and 390 px, recorded in the wave's reading) for rows
OC-35..OC-40, and the read-only half of SP-10 (`docs/plans/ambient-fathom.md`).

## Verdict, plainly

**The project workspace renders.** Five workspace pages (Overview, Index, Intent,
Activity, Wiring) plus the Index tab's Scope preview state were opened on a scratch
daemon over a copy of the real fleet, at 1440×900 and 390×844, in light and dark:
**24 views, every one with exactly one `h1`, the skip link first, the current tab
marked, no horizontal page overflow and no console errors or failed requests from the
pages.** The keyboard path works at 390 px: Tab, Enter on the skip link lands in `main`,
the next Tabs walk the trail, the heading's verb chips, the live-updates control and
then the nine tabs — each a 44 px link, the tab bar scrolling itself into view — and the
Scope panel's "Danger zone" disclosure opens with Enter to show the purge's dry run and
command.

**What it is worth.** The page shows, for the first time, what the index holds for a
repository (6 languages, 46,783 typed call edges at a 91.5 % typed share, 322 inventory
items for `fathom`), who calls a symbol (the kit's layered graph, FATHOM-KIT-02's first
integration), what its intent corpus promises and what nothing proves, its calls and
sessions from the resolved log, and its wiring drift — each panel naming the CLI verb
that prints the same JSON.

**What it costs.** Warm renders are 1–15 ms. Cold renders are not: the Intent tab's
first view of `fathom` took 37–51 s (the read-only audit itself — see below), the
Index tab's 2.7–12 s (the symbol list, the typed edges, the call graph and the corpus's
inventory), Wiring 1.4–2.1 s (the preview plan), Activity 0.4–0.8 s (two reads of a
21 MB call log). Each is memoized (per store watermark, per commit, or for 20–60 s) and
says when its data was read.

## How it was run

- **Binary:** `bin/fathom` built from branch `cp-wave-E` at `fa529b86` (the commit
  under proof), with the module's own Go env (`GOWORK=off`, CGO on).
- **Daemon:** `serve-http --addr 127.0.0.1:7674 --auto-refresh=false`,
  `FATHOM_SELF_RESTART=off`, `HOME` a scratch home. The live daemon on 7575 and its
  store were not touched.
- **Data:** a `sqlite3 .backup` of `workspace/db/fleet.sqlite` through a read-only URI
  (18 services); copies of the call log (21 MB), the session ledger, the verify ledger,
  the hooks log and the intent ledger, every one named to the daemon through its
  `FATHOM_*` variable; `FATHOM_SUPPRESSIONS` and `FATHOM_PURGES` pointed at scratch
  paths. The services' recorded roots are the real checkouts, which the tabs READ
  (the intent corpus, the wiring files, the corpus for the inventory); nothing was
  written into them — no Apply was pressed.
- **Browser:** Chromium through the Playwright MCP. The theme was set with the page's
  own appearance control and persisted under `fathom-theme`; every view confirmed
  `data-theme` and the ground (light `oklch(0.985 0.005 260)`, dark
  `oklch(0.16 0.012 260)`). Per view: `h1` count, `body.firstElementChild` as the skip
  link, the current tab, `scrollWidth − clientWidth`, error truth lines, console,
  page-error, failed-request and ≥ 400 listeners.

## Per-page results

All 24 rows: 1 `h1` · skip link first · current tab marked · overflow 0 at 1440 and 390 ·
light and dark confirmed · 0 console errors · 0 failed requests.

| Page | Route | h1 | Height 1440 / 390 | Panels | Notes |
|---|---|---|---|---|---|
| Overview | `/projects/fathom` | fathom | 1878 / 3604 | F0's body | the tab bar above F0's service detail |
| Index | `/projects/fathom/index?symbol=fathom::internal/console/console.Search` | fathom · Index | 2983 / 4624 | 5 | 3 callers (all tests), 1 callee, 30 at depth 2: 5 drawn + "+25 more callees" |
| Scope preview | `/projects/fathom/index?scope=docs/intent/**` | fathom · Index | 2775 / 4508 | 5 | 11 files, 80 symbols; the purge dry run behind the Danger zone disclosure |
| Intent | `/projects/fathom/intent` | fathom · Intent | 14547 / 18662 | 4 | 45 invariants in full; conditions grouped in disclosures |
| Activity | `/projects/fathom/activity` | fathom · Activity | 4250 / 7396 | 4 | 12,299 calls in 14 days; 235 distinct sessions |
| Wiring | `/projects/fathom/wiring` | fathom · Wiring | 3807 / 6786 | 4 | 3 doctor rows failing, 15 drift items, 0 receipts |

Screenshots (`<page>-<width>-<theme>.png`, 24 views, plus `keyboard-tabs-390-dark.png`
and `keyboard-danger-zone-390-dark.png`) are kept outside the repository, in the wave's
scratch directory `cp-E/shots/`, and are not committed.

**Timings on the real fleet copy** (`curl`, the daemon warm from a first request):

| Route | Cold | Warm | HTML |
|---|---|---|---|
| `/projects/fathom/index` | 2.7–12.1 s | 1 ms | 58 KB |
| `…/index?symbol=…` | 22 ms (graph memoized) | 4 ms | 73 KB |
| `/projects/fathom/activity` | 0.43–0.78 s | 1 ms inside 20 s | 83 KB |
| `/projects/fathom/wiring` | 1.4–2.1 s | 0.8 ms inside 60 s | 60 KB |
| `/projects/fathom/intent` | 34.7–51.5 s | 15 ms inside 5 min | 183 KB |

The fixture fleet's budget test (`TestConsoleBudgets`) holds every tab under 150 ms and
250 KB warm.

## Truth states seen on real data

- **Inventory, EPERM.** Mid-proof the daemon's reads of `/Volumes/m2ext` hit the known
  TCC wedge: the Inventory panel rendered "could not be read — … operation not
  permitted" in the error style, never "0 items". After `tccutil reset` it read 322
  items. A failed read is no longer memoized (it was, for one run: fixed before the
  final pass).
- **Intent, U4 not computed.** The audit's index oracle asks this daemon (as the
  console — every such row in the call log carries `"origin":"console"`) whether the
  invariants' named tests exist; against a cold daemon its `verify_plan` exceeded the
  oracle's 30 s timeout, and the page shows "not computed — U4 (named tests): index
  lookup failed", the unproven count a lower bound. The same audit from the command line
  with no daemon takes 18.8 s on this checkout (13.6 s of it system time walking the
  tree), so the tab's cold cost is the verb's.
- **Read-only held.** Across five intent audits the scratch intent ledger and hooks log
  kept the byte size and mtime of their copy (116,268 and 14,230,955 bytes, 20:12:15).

## What is not done, and why

- **SP-10 stays open.** The Scope panel ships read-only (suppressions, a glob's preview,
  the purge's per-location dry run and command, shown after the file list). No mutation
  is wired: the daemon has no purge path that flushes its own live caches (result cache,
  call-graph memo, vectors), so a page-driven purge would verify the store while the
  process kept serving answers built from the deleted rows. The note is on the row.
- **Governance, Plans, Proof and Refactor** link to `/layers`, `/plans`,
  `/certifications` and `/refactors` with `?svc=` — pages waves C and D build in
  parallel. On this branch alone those links reach the console's 404 page.
- **Cold renders** of the Intent and Index tabs exceed the design's 1 s first paint on
  a large repository; a background warm-up (or the verbs themselves getting faster)
  is the fix, not in this wave.
- **Kit returns** FATHOM-E-KIT-01 (`chrome.Tabs` on a phone: no overflow handling, ~36 px
  targets, tab roles without panels) and FATHOM-E-KIT-02 (the widget header row never
  wraps) are recorded in `docs/app-kit.md` with their local wrappers.
- **`report evidence` prints `inventory_total: 0`** for a corpus it could not read (its
  own best-effort rule); the Index tab shows that case as not measured. The verb is
  unchanged here — an observation for its owner.

## Gate

`fathom verify --strict --service fathom --plan docs/plans/observatory-console.md
--store <scratch ingest of fa529b86>`: 59 tasks, 23 claimed, 23 verified, 0 failing
(exit 0), OC-35..OC-40 among them. `docs/plans/ambient-fathom.md` on the same store:
77 claimed, 77 verified (exit 0); SP-10 unclaimed.

## Cleanup

The daemon on 7674 was stopped. The store copy, the ledger and log copies, the scratch
home and the scratch ingest store were deleted; the screenshots were kept.

## Survey defects fixed, re-run (2026-09-26)

The wave's survey confirmed ten defects in the workspace; commit `a97d1bd5` fixes all ten,
each with a test that failed on `c5d69dc4` first. Re-run on a scratch daemon at
127.0.0.1:7674 (binary `a97d1bd5`, scratch `HOME`, a `sqlite3 .backup` of the real
fleet store through a read-only URI, copies of the call log and the session, verify,
hooks and intent ledgers, `--auto-refresh=false`, `FATHOM_SELF_RESTART=off`).

| # | Defect | Before | After, on the fleet copy |
|---|---|---|---|
| 1 | An unreadable intent corpus rendered "no intent corpus … a choice, not a defect" | the clean absence | `view.corpus` is an **error** naming the files and errno; the tab renders it (fixture test with the router and `ADR.md` at mode 000) |
| 2 | File-resolved edges drawn solid and counted resolved | `console.announcer`: 1 resolved caller; app-kit `browser.check`: 11 resolved | 0 resolved / 1 heuristic; 0 resolved / 11 heuristic; the links `resolved: false` |
| 3 | One global lock per tab memo held across the computation | a kept `cue-wasm` Intent tab took 28–35 s behind a cold `fathom` audit | 0.019 s during `fathom`'s 51.6 s cold audit; a kept `pulumi-eks` Index 0.043 s during `platform-engineering`'s cold read |
| 4 | `project.sessions` computed twice on the Activity tab | band 235 over "the newest 20 of 236" | one value: both said 235 before a ledger append and both 236 after the next model read; the appended session is listed only once it is counted |
| 5 | "Updated — refresh this panel" re-served the memoized audit | `audited 04:04:10` before and after the refresh | a `fathom` verify row → `audit_at` 05:51:32 → 05:52:52, the probe plan's 3 rows present |
| 6 | Wiring drift counted doctor summaries and their items | `fathom` 15 | `fathom` 13: 6 apply items (named on `hooks_installed`), 6 BLOCKED (named on `wiring_drift`), `skills_installed` |
| 7 | Languages panel borrowed the scope-list verb with no daemon record | `fathom scope list …` | no chip; the footer says why |
| 8 | Activity's `call_log.note` differed from the verb run out of process | two notes | the page section and `fathom metrics traffic …` run from a terminal are equal in every key |
| 9 | At 390 px the current tab sat past the strip's edge; doctor's Status off-screen | Wiring at x 623–689 in a strip ending at 359 | `ui/console.js` scrolls the strip (never the page) to the `aria-current` tab; the doctor table leads with Status |
| 10 | A hand-written box for the unreadable-root note | `<p … data-project-root-note>` | `callout.Render` (info for no local root, caution for an unreadable one) |

Also on the copy, per page (`/projects/fathom` and its Index, Intent, Activity and Wiring
tabs, and `/projects/cue-wasm/wiring`): one `h1`, the skip link first, the current tab
marked, verb chips present, no `dark:` class, HTML 56–184 KB, warm renders 0.7–14 ms.
Gate: a scratch ingest of `a97d1bd5`, then `fathom verify --strict --service fathom
--plan docs/plans/observatory-console.md --store <it>`: 59 tasks, 23 claimed, 23
verified, 0 failing (exit 0).

**Not re-run: the browser pass.** Chromium would not start on this machine for the
re-run — the Playwright MCP's Chrome and a directly launched headless Chrome both
aborted at launch (SIGABRT; crashpad `bootstrap_check_in` error 141; `open` could not
find the application and `whoami` could not name uid 501), so the both-themes,
1440/390 px screenshots and the console-error listeners were not repeated for this
commit. What they would have measured for #9 is pinned by
`TestProjectWorkspace_PhoneKeepsTheCurrentTabAndStatusInView` only as far as a Go test
can (the script's presence and the Status-first table); the geometry needs the browser.

