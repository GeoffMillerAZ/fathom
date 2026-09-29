# Efficiency wave stops: what each stop's reading found

Statistics and row states only. Each stop's reading is one line that begins with the row id, which the
row's lexical oracle matches (`docs/plans/efficiency-wave.md`); the sections under it say what was read.

## Stage 0: the integration (EW-0)

EW-0 reading: the four lines are each an ancestor of main at `9ea6da5f` (`git merge-base --is-ancestor` exits 0 for all four): the push line carrying the wired audit (`872c939c`), the judgment plans (`adab8048`), the roadmap wave (`67b53780`) and the language wave (`c91741ac`); of the 29 rows `872c939c` reopened, 28 are still `[ ]` at that tip and `plans next` names each of them open (`plans status` counts them), and the 29th, IA-3, is `[-]` refused, a later disposition and not a regression.

### What was read

- **Ancestry.** Read-only git in a clean clone at `9ea6da5f`. All four commits, and the merges that carried them, are ancestors of the tip; 342 commits and 55 merges lie between `872c939c` and the tip.

| line | commit | how it reached main |
|---|---|---|
| push line carrying the wired audit | `872c939c`, "disposition the 105 claimed rows no product entry point reaches" (2026-09-28) | on the first-parent history of main, before the `jev-ideas` merge `d9a3947e` |
| judgment plans | `adab8048`, "seven judgment-program plans with grammar-v2 rows (ADR-056)" (2026-09-28) | on the line `d9a3947e` merged as its second parent (tip `6f5e8ed7`) |
| roadmap wave | `67b53780`, "rm-integ, roadmap wave R4b" (2026-09-28) | first-parent merge, parents `68624033` and `e2604f48` |
| language wave | `c91741ac`, "language wave 2 (lq-integ 88fe67a5) onto main" (2026-09-29) | merge on the push 81 line, parents `f383a64a` and `beb2b232` |

- **Rows the wired audit reopened.** `git show 872c939c -- docs/plans` adds 29 `[ ]` lines. The ids: SH-2, BA-6, RG-4, PB-4, DF-6, DF-7, EP-37, O-13, R-12, R-21, GKE-MH-2, IA-3, LB-F24, P1-6, RC-13, RC-38, CV-5, RT-161, RT-41, SB-18, SM-4, SM-8, SR-67, SR-68, TC-8, VB-25, WI-20, WI-21, WI-22. Each id was read in the plan file the audit edited, at the tip: 28 are `[ ]` and none is `[x]`. `fathom plans next --json`, run with `--plan` over the 19 plan files that hold the 29 rows (taken from the tip, scratch store), names all 28 as open with state `open`; WI-22 is blocked on a `needs` target and is still named. `plans status --json` puts each of the 18 plans that hold an open one at an `open` count at least as large as its share of the 28.
- **The one that is not open.** IA-3 (`docs/plans/init-and-adoption.md`) reads `[-]` with `disposition: refused`, set by AG-25 under ADR-059 (one stanza, in AGENTS.md alone) after the audit. It was reopened by the audit and refused later by a decision; the merge did not lose it.

### What this reading does not say

- It does not say the 28 rows are wired. They are open, each waiting on its own plan's wiring or disposition.
- Rows can leave the open set after `9ea6da5f`: the commit that lands this reading also retires SR-67 and SR-68 as refused (`docs/plans/surface-reckoning.md`), so the tip after it reads 26 open and 3 `[-]`.

## Stop 1: push, deploy, adopt (EW-S1)

EW-S1 reading: push 81 passed the gate on its first run and landed as 087fb5ab (one attempt for one landing), `task deploy` exited 0 and the daemon serves that same commit, `fathom doctor` on the main checkout reads 53 checks: 38 pass, 8 warn, 1 fail, 6 skipped, with no wiring row failing, the agent surface serves the eight tools the stanza names, and the wave dispatched after the push ran under the guide (six worker environments, each with its own fathom home, cache root and temp dir); the row's `needs` list still holds EW-0 and EW-12, which are open, so this reading does not close the row.

### What was read

- **Gate.** One run of `task push` from a clean clone at the integrated tip: validate 470 s, verify 333 s, review 3.8 s, refactor-conserve 25 s. The push replaced f383a64a with 087fb5ab (94 commits on main). No retry, no gate failure, no bypass. `origin/main` still names 087fb5ab when this is written.
- **Deploy.** `task deploy` from the clean clone exited 0 in 77 s. The daemon took 60 health polls to answer (launchd's respawn throttle after a first spawn killed for code signing, the same shape as push 80's deploy at 61 polls), then the assert read the served binary as 087fb5abb033, equal to HEAD. EW-17's last-exit-reason line printed as designed.
- **Index.** The `ingest` call over /rpc answered at its 2-minute write budget with the refresh queue's state (`running`), not an empty body (EW-18). The service's indexed commit moved from f383a64a to 087fb5ab within the following minutes.
- **Doctor, read again after the wave dispatched.** `fathom doctor --service fathom` on the main checkout, read-only: `binary_fresh` and `daemon_fresh` pass (local and daemon binary both at 087fb5abb033, built 2026-09-29T12:25:59Z; the daemon up 22 minutes, 6 starts in 24 h, 0 unclean, 0 over-budget exits). `stanza_present` passes (well-formed stanza in AGENTS.md, no CLAUDE file, Claude Code 2.1.284 loads it natively). `stanza_loaded` also passes, but on 7 older beats that name a root CLAUDE.md this checkout no longer has (O139), so it is not evidence that a session loaded the AGENTS.md stanza. `hooks_installed` (20 of 20), `hooks_firing` (33,428 edit-hook calls and 28,415 verify-cli calls in the call log), `skills_installed` (10 of 10), `wiring_drift` (0 of 34 drifted), `config_valid`, `service_roots` (19 of 19 exist) and `concerns` (17 hold) pass.
- **Served surface.** `tools/list` on the agent endpoint returns 8 tools: blast_radius, find_symbol, obligations, read_source, review_diff, search, symbol_context, verify_plan. The stanza names the same 8; the sets are equal.
- **Plan verify.** `fathom verify --service fathom --plan docs/plans/efficiency-wave.md`, run twice at the stop: exit 0 both times; 36 tasks, 20 claimed, 20 verified, 0 failing. Wall time 15.86 s for the first run and 0.54 s for the second (a cold and a warm product-reach memo, EW-22). The open rows it lists are EW-0, EW-13, EW-29 and the operator readings EW-S1, EW-S2 and EW-M1 to EW-M4.

### Doctor rows that did not pass

| row | state | reading |
|---|---|---|
| providers | FAIL | 1 of 2 profiles failed auth: no API key in the doctor's environment. Expected while the model-spend hold is on; no wiring or freshness row is affected. |
| scratch_daemons | WARN | 1 unmanaged serve-http daemon, leaked (orphaned, 9 minutes old); it is not the managed daemon. `fathom daemon reap` names it; it was not reaped here. |
| hook_budget | WARN | stop-hook p99 8.9 s against its 8.0 s deadline; every other hook inside its deadline (edit-hook p99 0.7 s of 4.0 s over 42,054 calls; session-hook p99 4.5 s of 8.0 s over 426; prompt-hook p99 1.0 s of 8.0 s over 99). This is the work EW-6 and EW-29 read. |
| disk_retention | WARN | 4.3 GiB reclaimable, all in one scratch bench-worktree class; 15.0 GiB free. |
| obligations_served | WARN | the obligations layer is partially compiled: 32 of 89 sections (the layer recompile is on the model-spend hold). |
| typed_edges | WARN | 265 packages resolved, 1 discovery gap flagged. |
| unsupported_languages | WARN | one language with no extractor: 42 files, zero symbols. |
| unproven_claims | WARN | 41 unproven claims [U1 5, U11 22, U12 8, U13 2, U3 2, U6 2]. |
| doctrine | WARN | 40 visions (landed 21, planned 17, accepted 2); 29 goal bets. |

Two rows read differently from a fresh clone than from the main checkout, because they read state that lives
under the checked-out root: `hooks_firing` FAILs in a clone that no session has edited in (its call log
is empty), `stanza_loaded` skips there (no beat names the clone), and `unproven_claims` computes without
its plan-row source (36 claims, 5 sources not computed). The readings above are the main checkout's.

### The next wave used the guide

The wave dispatched after push 81 (six workers, labels `wa-*`) ran under `docs/guides/agent-waves.md`, read
from its worker environments while it was in flight:

- **Isolation (EW-8).** Each of the six has its own clone and its own fathom home, cache root and temp dir under one shared build directory; the Go build cache and the gate lock are shared, as the guide says.
- **Scratch ingests, not the daemon's (guide, "The shared daemon is read-only").** Three of the six hold a scratch verify store; the worker that wrote this reading ingested its committed head into its own store (161 s) and read the shared daemon only.
- **Tests selected first (EW-11).** That worker ran `fathom tests select --safe` on its diff (526 tests selected, 0 touched certificates) and then the selected `go test` commands under `-race` (passed in 139 s) before any whole-package run.
- **Identity (guide, "Isolation").** Each clone sets the maintainer's name and address before its first commit; the 6 commits the wave's fathom clones held ahead of push 81 when read (2 clones) all carry that identity as author and committer, none a host-derived one. On main, 16 authored and 15 committed commits from earlier runs carry a host-derived address; a `.mailmap` maps them.
- **Not read here:** whether every worker kept to every rule of the guide, and the wave's own gate and push, which had not run when this was written.

### What this reading does not say

- It does not close EW-S1. The row's `needs` name EW-0 (the integration reading, not yet landed) and EW-12 (which waits on EW-0), and a row whose needs are open is not eligible; the clause "the next wave used the guide" is read from a wave still in flight.
- One landing is not EW-M2's sample: it needs at least 6 landings after stop 1.
- The doctor's `providers` FAIL and the partial obligations layer are held by the spend hold, not by a defect this stop introduced; neither was read as caused or fixed by the wave.
