PE-M5 reading: before the 2026-09-29 cleanup, 14,631 of 19,972 verify-ledger rows (13,619 under an OS temp dir) and 94 of 180 intent-ledger rows sat under unregistered roots; filtering them moves every figure, and it empties most of the row-level proof history, because the fleet's gate runs are recorded outside the registered roots.

# PE-M5: ledger pollution, counted and recomputed

**Plain verdict.** The rule PE-24 and PE-25 ship (a ledger reader keeps only rows whose root is a
registered service root) is necessary: most of both ledgers described throwaway clones, and a
view that counted them reported a claimed-unproven share of 6.0% where the registered roots' own
readings show 15.6%.
But on this machine the rule also removes almost all of the per-row proof history (2,951 row
histories fall to 61), because the pre-push gate runs in a push worktree and most `fathom verify`
runs happen in clones. The filtered views are honest and nearly empty. Burn-up and regressions
need the gate to record its runs under the registered root before they measure the fleet again
(observation O195). Scratch rows came back within hours of the cleanup (O196).

Scope: this machine, 2026-09-29, snapshot at 15:30 local (UTC-7). Statistics only; roots are
named by class (registered, scratch, worktree, external-volume build clone, other), never by path.

## Method

- **Inputs, read only.** The live `verify-ledger.jsonl` (6,710 rows) and `intent-ledger.jsonl`
  (86 rows), and the owner's archive of 2026-09-29 (`verify-ledger-scratch-2026-09-29.jsonl`,
  15,666 rows; `intent-ledger-scratch-2026-09-29.jsonl`, 94 rows). The archive files match their
  `SHA256SUMS` (5 of 5 OK). Every file was copied to scratch first; `FATHOM_VERIFY_LEDGER` and
  `FATHOM_INTENT_LEDGER` pointed at the copies. The archive's rows were moved out of the live
  ledgers at the cleanup, so live plus archive is the population before it (22,376 verify rows,
  180 intent rows; the verify count includes rows appended after the cleanup).
- **Roots.** One read-only `list_services` call to the shared daemon: 19 services, 19 canonical
  roots (no ingest was run). The live ledger's registered rows name 9 root spellings that
  canonicalize to 7 roots (symlinked spellings of the same checkouts).
- **Product functions.** Classification by `ledgerroots.RegisteredRoots` and `Roots.Class` at
  commit 016caff6. Proof history by `console.ProofHistory` with the registry unread (unfiltered)
  and with the roots (filtered); burn-up by `console.BurnUp`; claim honesty by
  `console.ClaimHonesty` over the session ledger's agent sessions (`agentSessionsOf`); cost per
  proven row by `/work`'s own computation (`computeWork`: the session join, then the scorecard's
  tokens per proven row); intent rows by `intent.ReadRegisteredLedger`. The harness was a
  throwaway test in a scratch copy, not committed.

## Rows and roots under unregistered roots

| ledger | rows | registered | OS temp dir (scratch) | worktree of a registered root | other unregistered |
|---|---|---|---|---|---|
| verify, population dated before 2026-09-29 (UTC) | 19,972 | 5,341 | 13,619 (173 roots) | 783 (37 roots) | 229 (22 roots) |
| verify, population, all rows | 22,376 | 5,353 | 15,311 (198 roots) | 783 (37 roots) | 929 (38 roots) |
| verify, live at the snapshot (all post-cleanup unregistered rows) | 6,710 | 5,353 | 660 (10 roots) | 0 | 697 (14 roots) |
| intent, population | 180 | 86 | 91 (18 roots) | 3 (1 root) | 0 |

Against the 2026-09-28 reading in O121 (13,514 of 19,639 verify rows and 91 of 180 intent rows
under `/private/tmp` style roots): the scratch class reproduces it, 13,619 of 19,972 (68.2%) with
the rest of that day's rows included, and 91 of 180 exactly. The registered-roots rule excludes
more than the scratch class: 14,631 of 19,972 verify rows (73.3%) and 94 of 180 intent rows
(52.2%), adding worktrees of registered roots (the owner's archive rule) and other unregistered
roots. No verify row lacks a `repo_root`.

## Figures recomputed with and without those rows

Proof history over the population (22,376 readings):

| figure | unfiltered | filtered | note |
|---|---|---|---|
| readings kept | 22,376 | 5,353 | 17,023 readings under 273 unregistered roots (15,311 scratch) ignored |
| row histories (schema-3 per-row verdicts) | 2,951 | 61 | most per-row readings were written outside the registered roots |
| burn-up, fleet, latest day (2026-09-29) | 1,803 of 2,622 weight proven (68.8%) | 43 of 61 (70.5%) | 2 days of schema-3 history; projection absent in both (needs 7 days) |
| burn-up, fathom, latest day | 1,651 of 2,202 (75.0%) | 20 of 36 (55.6%) | 1 day at the registered root; projection absent |
| claim honesty, session-joined (fleet), claimed rows the gate did not prove | 12,400 of 62,341 (0.20) over 4,110 readings | 11,960 of 57,323 (0.21) over 2,885 readings | one identity: the session ledger records no harness or model |
| claim honesty, session-joined (fathom) | 1,034 of 26,324 (0.04) | 740 of 21,563 (0.03) | |
| unattributed readings (no session window covers them) | 18,266 of 22,376 | 2,468 of 5,353 | |
| claimed-unproven share over every reading (fleet) | 22,071 of 367,717 (6.0%) | 13,791 of 88,177 (15.6%) | `PlanReading.Unproven` summed; scratch reruns of proven plans diluted it |
| claimed-unproven share over every reading (fathom) | 7,621 of 316,291 (2.4%) | 1,446 of 44,442 (3.3%) | |
| cost per proven row (`/work`, fathom) | not measured: 0 proven changes joined to a session | not measured: 0 proven changes joined | unjoined proven transitions: 2,166 unfiltered, 20 filtered |

The live ledger alone (6,710 rows) gives the same filtered figures (its registered rows are the
population's), and ignores 1,357 readings under 24 roots.

The intent readers keep 86 of 180 rows: the audit's debt and baseline, the clock and the
learning announcements of the 18 scratch roots and the 1 worktree no longer reach a registered
root's views.

## EW-M1's pre-registered baseline (law 12)

EW-M1's baseline is 49.69M sub-agent tool_result tokens over 671 proven rows, about 74,054 tokens
per row (ADR.md, EW-M1 in `docs/plans/efficiency-wave.md`). Its denominator is git plan state,
not the ledger: the row defines a proven row as one "`[x]` at a window's end and not at its
start, on the integrated line", and ADR.md's decision 6 counts "671 rows proven across the lines".
The filter does not touch git plan state, so the baseline stands unchanged in both arms:

| | unfiltered | filtered |
|---|---|---|
| EW-M1 baseline (git plan state, not re-based) | 49.69M / 671 = 74,054 tokens per row | 49.69M / 671 = 74,054 tokens per row |
| ledger rows claimed-proven at the window's end and not at its start (2026-09-14T18:08:47 to 2026-09-28T18:08:47 local), fathom | 1,486 | 0 |
| the same, every service | 1,503 | 16 |

The ledger could not stand in for EW-M1's denominator under the filter: the registered fathom
checkout recorded no per-row reading before 2026-09-29, so its filtered count in the window is 0.
Unfiltered, the ledger counted 1,486 fathom rows, more than twice git's 671, because scratch
clones and push worktrees re-proved the same rows.

## What this changes

- Every reader now names what it left out (the ledger's rows, roots and scratch share), so a
  figure never travels without its denominator.
- The per-row proof history needs the gate's runs recorded at the registered root: until then
  burn-up, regressions, time to proof and the session join read nearly nothing (O195).
- Scratch and build clones still write the real ledgers after the cleanup: 660 rows under the OS
  temp dir from 11:52 UTC and 697 under external-volume build clones from 16:10 UTC, all
  `origin: cli` (O196). PE-29's `ledger-unregistered` gc class archives them on the owner's
  command; the readers already ignore them.
