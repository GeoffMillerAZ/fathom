# Console wave F — the browser proof (2026-09-25)

The doctrine's rule 4 (`cmd/fathom/CLAUDE.md`: a 200 is not a rendering) and the
plan's gate (`docs/plans/observatory-console.md`: every page row needs a browser proof
in both themes at 1440 and 390 px, recorded in the wave's reading) for wave F:
`/impact` (OC-41, OC-42), `/sessions` and `/sessions/{id}` (OC-43), `/operate`
(OC-44, SH-9) and `/decide` (OC-45).

## Verdict, plainly

**Wave F renders, on real data, in both themes at both widths, and every number on it
names its instrument.** All five pages were opened at 1440×900 and 390×844, light and
dark — 20 views — and every view passed: exactly one visible `h1`, the skip link is the
first Tab stop and Enter moves focus to `main#observatory-main`, the System or Evidence
nav item is marked current, the breadcrumb trail ends at the page, no horizontal page
overflow (`scrollWidth == clientWidth`), the theme's ground applied (light
`oklch(0.985 0.005 260)`, dark `oklch(0.16 0.012 260)`), and no console error, page
error or ≥400 response.

What the pages now say about the fleet, from a copy of its real ledgers and call log:

- **Impact.** The arms are ranges: ON 850 admitted sessions, 367–715 real tokens per
  turn (median 497); OFF 11 sessions, 1,077–1,477 (median 1,228). 30 of the 39 called
  tools record no outcome and say "not instrumented" in every outcome cell — the survey
  saw them as green and red zeros. The catches timeline holds 573 catches: 538 verify
  refusals, 34 conserve losses and 1 integrity catch from `fathom review` rows in the
  call log (see the first finding below), and 0 guard refusals. **That zero was wrong**
  (corrected in the fix round below): the guard lane read only `hooks.jsonl`, never its
  rotated generation `hooks.jsonl.1`, which held all 8 refusals (2026-09-21).
- **Sessions.** 1,888 sessions from 3,278 ledger rows, one row each, 50 to a page.
  This session's replay holds 39,314 events: it renders as 197 windows of 200 (the
  survey's replay of one long session shipped 9.3 MB of JSON).
- **Operate.** 13,940 usage calls in the last 7 days (91,924 in the whole log): agent
  2,824, edit hook 5,362, gate 1,551, refresh 334, plumbing 3,589, unattributed 280.
  The heatmap hatches 2026-09-20 — no call-log row that day — instead of drawing it as
  zero. Time-to-remedy over the three fleet report snapshots: a median of 128 h over 7
  closed gaps, 4 still open, drawn **stale** because the newest snapshot is 43 days old.
- **Decide.** Provider none; 120 rulings in the log beside the call log (86 applied, 0
  failed, median 587 ms), all `certify_suggest`; the calibration table from
  `decision-port-calibration-2026-09-24.md`: 16/35 right at confidence ≥ 0.9.

Two things this run found that are not wave F's defects, recorded so they are not lost:

1. **Test runs write to the live call log.** The live `workspace/logs/mcp-calls.jsonl`
   holds `fathom review` rows for a service named `svc` from 2026-09-25T23:15Z on —
   the name `cmd/fathom`'s review tests pass (`review_mode_test.go`). All 34 conserve
   losses on the timeline are those rows. Which run resolved the live log was not
   traced here; it is worth its own observation. The timeline shows the rows because
   they are in the log, labelled with the service they name.
2. **The EPERM wedge reaches the daemon.** On one restart the scratch daemon's first
   model build logged `operation not permitted` for three repo roots (the external
   volume's TCC wedge, `~/.claude/CLAUDE.md`); `tccutil reset` and a restart cleared it.
   A wedged model shows those repos degraded, correctly, but it is not the fleet.

## How it was run

- **Binary:** `bin/fathom` built from branch `cp-wave-F` at `12752973` (the wave's
  code), module Go env (`GOWORK=off`, CGO on).
- **Daemon:** `serve-http` on `127.0.0.1:7675`, `--auto-refresh=false`,
  `FATHOM_SELF_RESTART=off`, `HOME` a scratch home. The live daemon on 7575 and its
  store were not touched and no LLM profile was loaded (the scratch home has no
  `config.toml`).
- **Data:** a `sqlite3 .backup` of `workspace/db/fleet.sqlite` opened read-only; copies
  of the call log (`--call-log`, 91,924 usage rows) with `~/.fathom/logs/decisions.jsonl`
  beside it, and of the session ledger, verify ledger, hooks log, threads and the fleet
  report snapshots under the scratch home (`FATHOM_SESSION_LEDGER`,
  `FATHOM_VERIFY_LEDGER`, `FATHOM_HOOKS_LOG` pointed at them).
- **Browser:** Chromium through the Playwright MCP. The theme was set with the page's
  own appearance control (the sidebar select at 1440, the one inside the Menu
  disclosure at 390), and every view was checked in the page itself, then saved as a
  full-page PNG.

## Keyboard paths

Each page's own control, by keyboard alone from the top of the page:

| Page | Journey | Result |
|---|---|---|
| `/impact` | Tab ×23 to the "verify refusals" kind chip, Enter | `?kind=verify-refusal`; the table reads "20 of 538" |
| `/sessions` | Tab ×18 to the Arm select, type "off", Tab to Filter, Enter | `?arm=off`; 11 of 1,888 sessions match |
| `/sessions/{id}` | Tab ×19 to the first channel chip, Enter | `?channel=custodian`; "window 1 of 1 · 3 events of 3" |
| `/operate` | Tab ×34 to the capabilities Project select, type "conjure", Tab to Show, Enter | `?svc=conjure#capabilities` |
| `/decide` | Tab ×21 to the `certify_suggest` site chip, Enter | `?site=certify_suggest#rulings` |

Arrow keys did not change a closed native select in headless Chromium on macOS (the kit's
own record of native-select keys); typing the option's name did.

## Budgets (real data, scratch daemon)

| Page | HTML | Warm render (3 runs) | First render, model already warm |
|---|---|---|---|
| `/impact` | 124 KB | 3–7 ms | 0.21 s |
| `/sessions` | 180 KB | 16–20 ms | 0.02 s |
| `/sessions/{id}` (39,314 events) | 162 KB | 10–14 ms | 0.15 s |
| `/operate` | 136 KB | 31–39 ms | 0.82 s |
| `/decide` | 80 KB | 2 ms | 4 ms |

Every page is within 250 KB and 150 ms warm. `/operate`'s first render after its 30 s
projection expires reads the 20 MB call log once for every section and runs the flywheel
over the model's impact fold: 0.82 s here, but 1.2 s in an earlier run while other
waves' test runs loaded the machine — the design's 1 s cold-paint budget holds only on
a quiet machine. The replay keeps a session's events for 30 s, so paging a window reads
nothing. The observatory model itself (not wave F's) still takes 6–11 s cold.
`TestConsoleBudgets` holds the fixture fleet to both limits for every page.

## Screenshots

`<page>-<width>-<theme>.png` for `impact`, `sessions`, `session`, `operate`, `decide` at
1440 and 390, light and dark — 20 full-page PNGs — kept outside the repository in the
wave's scratch directory `cp-F/shots/`, with the draft crops the layout was tuned from.

## Fix round (2026-09-26)

A review of this wave confirmed 14 defects; each is fixed and pinned by a test
(`console_wavef_truth_test.go`, `internal/console/evidence_test.go`,
`TestConsolePagesRenderTheirVerbs`' filtered pages, `TestConsoleBudgets`' filtered
paths). The re-run used a binary built from the fix, `serve-http` on `127.0.0.1:7675`
(`--auto-refresh=false`, `FATHOM_SELF_RESTART=off`, scratch `HOME`), a `sqlite3 .backup`
of the store and fresh copies of the call log, `decisions.jsonl`, the session, verify and
hooks ledgers (both generations) and the three fleet report snapshots. The internal disk
was full during the run, so the copies, the build's temp space and the tests' temp space
lived on the external volume.

What the real data now says:

- **Guard refusals: 8**, all 2026-09-21 in scenariobench subject worktrees, from
  `hooks.jsonl.1`; `fathom metrics catches` lists the same 8. The lane's source names
  both generations. Totals: 538 verify refusals, 55 conserve losses, 2 integrity
  catches (the call log grew since the first run).
- **Filters scope every lane.** `?svc=conjure`: 17 refusals of conjure's own 190 verify
  runs (was "of 15,361"); with `&days=7`, 2 of 71. Conserve and integrity lanes read
  "not instrumented for conjure" instead of a zero. `?svc=no-such-project` is the
  shell's 404 (HTML and JSON).
- **Unreadable sources are errors, never idle or empty.** With the hooks log, call log
  and decisions log copies at mode 000 (and, by accident, the external volume's TCC
  wedge mid-run): the replay reads "could not be read — hooks log unreadable";
  `/operate`'s traffic, surfaces, callers, tools and zero-call panels are
  `data-truth-state="error"`, the p95 tiles read "unreadable", and none of "no calls in
  window", "no usage call", "0 of 0 tools" appears; both `metrics traffic` sections
  carry `state: error` and no `total_calls`; every `/decide` site's rulings, applied
  and failed counts are errors. `metrics sessions` with no ledger prints "not measured"
  and a warning, and its JSON carries no count.
- **Time to remedy:** `median_hours` 128, state stale, `lower_bound: true`, its reason
  naming both; the tile reads "≥ 128"; closed gaps that opened on or before the first
  snapshot read "≥ … h"; no repository serializes `episodes: null`.
- **Every panel names the verb that prints it.** `/operate` renders the new
  `fathom metrics load --json` (calls per day per surface; tools by caller class —
  equal to the page's section on real data), gives `--by caller` and the zero-call list
  (`instrument --adoption`) their own panels, and adds the harness matrix (six
  harnesses; the chip carries `--url` for this daemon, and the pasted chip prints the
  section). A pasted `--q 'e865 x'` chip prints the page's section. The replay names
  its verb in its heading and on both panels.

Budgets on real data (HTML; best of three warm renders): `/impact` 147 KB, 4 ms;
`/sessions` 182 KB, 16 ms; `/sessions/{id}` 164 KB, 17 ms (and 164 KB at `?per=2000`,
where it was 1.19 MB); `/operate` 168 KB, 31 ms; `/decide` 112 KB, 2 ms. Each page has
one `h1`; the served `app.css` carries `.min-w-\[560px\]` and no `%dpx` rule.

**Not re-run: the browser pass.** Chrome aborted at launch (SIGABRT, crashpad
`bootstrap_check_in` error 141) through the Playwright MCP and when started directly,
after the internal disk filled; Playwright's own Chromium could not be downloaded. The
first browser run of the fix (20 views, before the crash) hung on `networkidle`, which
never settles on these pages: each holds its `/events` stream open. So the both-themes,
390 px and keyboard checks for this round rest on the page tests and the markup (a
stacked list below `md` for each answer-carrying table, a sort control outside the
md-only table) and still need a browser pass.

## Cleanup

The scratch daemon on 7675 was stopped; the store copy, the ledger and log copies, the
scratch home and the scratch gate store were deleted after the gate ran. The
screenshots were kept.
