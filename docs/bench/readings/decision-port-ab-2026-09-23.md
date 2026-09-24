## The decision-port A/B (DP-13/DP-14, vision 32, ADR-042): is Jev worth turning on?

### Verdict, plainly

**Keep Jev-backed routing on if you already have a key; turn its obligations
re-rank off by default — which is exactly what this reading changed.**

Three trials=2 runs compared the plain wired fathom arm against the same arm
with one line added, `[decide] provider = "typesafe"` (Jev 1.13, live through
OpenRouter): `lca-jev-r1` (issue-shaped bug reports, does the prompt route to
`localize`), `lookups-vs-jev-r1` (lookup-shaped prompts, does the prompt route
to the right verb), `guards-jev-r1` (does the governing rule for a file reach
the agent before it edits — the obligations re-rank site).

- **Routing (the `/route` site) works as designed, with a real if small
  product effect.** On `lca`, turning Jev on lifted the localize-verb
  gold-source rate from 4.6% to 23.4% and the paired win rate by +0.047
  (bootstrap 95% CI [0.000, 0.125] — barely excludes zero, 2 tasks better, 0
  worse). On `lookups-vs`, the effect was a wash: mean 0.000 (CI [-0.107,
  0.107], 1 better/1 worse) — Jev routed confidently and, on inspection,
  sensibly, but it did not change which cells passed.
- **The obligations within-tier re-rank measured negative on governance at
  the edit, at this sample size.** `guards-jev-r1`: paired mean -0.19 (CI
  [-0.375, -0.062]), 3 tasks worse, 0 better, 5 tied, n=2 trials. Tracing the
  mechanism in raw transcripts finds ONE task (`guards-i28-duckdb-path`)
  where the re-rank visibly displaced the governing rule later in the same
  session; the other two regressed tasks (`guards-i19`, `guards-i31`) show
  the governing rule never served in EITHER arm — a pre-existing coverage
  gap the re-rank did not cause. The evidence supports "the re-rank can
  displace the rule that matters" as a real, demonstrated mechanism, but
  does not support it as the explanation for all 3 regressions — see
  "Guards mechanism analysis" below for the full per-task trace and the
  count of what would change this reading.
- **What changed as a result (this reading, `internal/config`):**
  `DefaultDecide.RankWithinTier` is now `false`. A repo that sets
  `[decide] provider = llm | typesafe` for routing or the compile pre-filter
  no longer also gets the obligations re-rank; it opts back in with
  `rank_within_tier = true`. Routing and the compile pre-filter are
  unaffected — only the obligations-site re-rank is turned off by default.
  ADR-042 amended (`ADR.md`); the config default test and the amendment's
  own routing/mechanism claims are proven by the cells and decision logs
  cited throughout this reading, not asserted.
- **What it costs when it does run:** across the 8 guards tasks' `full-jev`
  cells alone, the obligations-site decider answered 15,742 questions
  (15,699 applied, 43 errors), p50 latency 337ms / p95 1,075ms per call —
  real, metered cost for a site that, on this sample, more often hurt than
  helped.

---

### Corpus 1 — `lca` (bug-localization routing), run `lca-jev-r1`

Subject fathom commit `533d293c` (`arm_surface`, all 129 stored cells);
harness `2.1.280 (Claude Code)`; arms `fathom` (baseline wired) /
`fathom-jev` (baseline + the Jev `[decide]` line, built as a corpus variant
with `-arm-clone fathom-jev=fathom -arm-setup fathom-jev=<jev-line> -arms
fathom,fathom-jev` — DP-14); trials=2. Ledger: 65 rows (`fathom`) + 64 rows
(`fathom-jev`) in `docs/bench/scenarios-ledger.jsonl`, run-id `lca-jev-r1`.

| | `fathom` | `fathom-jev` |
|---|---:|---:|
| cells stored | 65 | 64 |
| pass | 59 (91%) | 61 (95%) |
| localize gold-source rate | 3/65 = 4.6% | 15/64 = 23.4% |
| median tokens/cell | 43,037 | 46,580 |

Paired over the 32 tasks both arms actually completed (excluding
`joke2k/faker`, which only `fathom` has a stored cell for — see "Lost
cells" below):

| Paired stat | Value |
|---|---:|
| paired n (tasks) | 32 |
| `fathom` pass (paired cells) | 58/64 |
| `fathom-jev` pass (paired cells) | 61/64 |
| better (task, `fathom`→`fathom-jev`) | 2: `fwcd/kotlin-language-server` 0%→100%, `rapptz/discord.py` 50%→100% |
| worse | 0 |
| tied | 30 |
| mean paired diff | +0.0469 |
| bootstrap 95% CI (10,000 resamples over tasks) | [0.0000, 0.1250] |
| two-sided sign test p (2 better, 0 worse, 30 ties dropped) | 0.500 |
| tokens ratio (`fathom-jev`/`fathom`, sum over paired cells) | 1.07x (3,598,537 / 3,356,217) |
| cost ratio | 1.14x ($2.963 / $2.604) |
| turns ratio | 1.10x (313 / 284) |

**Lost cells (git-clone failures, both arms equally):** the log's own
summary line reads `129 cell(s) run, 27 lost` — 6 subjects
(`hpcaitech/colossalai`, `docker/compose`, `huggingface/pytorch-image-models`,
`dokku/dokku`, `sanic-org/sanic`, `archivebox/archivebox`) each lost all 4
cells (both arms × 2 trials) to the same fault, `git clone …: exit status
128 (… operator-gitconfig: Operation not permitted)` — the external-volume
TCC wedge this session's own CLAUDE.md documents, hitting the clone step
itself rather than a file read. `joke2k/faker` lost 3 of its 4 cells to two
different faults (`base_commit … not found after fetch: exit status 128`
on `fathom/t1` and `fathom-jev/t1`; `resetting the shared worktree: exit
status 128 (… .claude/settings.json not uptodate …)` on `fathom-jev/t2`) —
`fathom/t1` alone survived, which is why that task is unpaired above.
Per-arm split: `fathom` lost 13, `fathom-jev` lost 14 (6×2 + 1 vs 6×2 + 2) —
the "13–14 cells/arm" figure this reading's task brief named, now exact and
attributed.

**Route-decision instrument gap (disclosed, not a number I'm stating):** for
`guards`/`lookups`, `/route` decisions are logged to a `decisions.jsonl`
beside the wired daemon's own worktree. For `lca`, the wired daemon runs
against each SUBJECT's own worktree (a foreign open-source repo, not
fathom), and no `decisions.jsonl` was found under any of the 33 subject
worktrees this reading could inspect (`/Volumes/m2ext/tmp/sextant-r3-
workspace/scenariobench/subjects/*-worktrees/*/decisions.jsonl` — zero
files; the standard `UserPromptSubmit`/`PostToolUse` hook stack DID fire in
the raw transcripts, so hooks are wired, but the decider's own log was not
found on disk where the guards/lookups convention would put it, and no
`route` line rides the transcript stream itself). Per this reading's own
rule ("never state a number not computed from the cells or decision
logs"), no lca-specific route-decision count or confidence figure is given
here; the gold-source-rate and paired-pass numbers above stand on the cell
records alone, which ARE directly read.

---

### Corpus 2 — `lookups-vs` (opening-move routing), run `lookups-vs-jev-r1`

Subject fathom commit `3e07b3d6`; harness `2.1.281 (Claude Code)`; arms
`full` / `full-jev`; trials=2. First attempt (`bench-jev-all.sh`, launched
under HEAD `533d293c`) lost all 56 cells to the weekly Claude rate limit
(`jev-all.log`: `bench lookups-vs-jev-r1 end … exit=3`, zero cells stored —
confirmed by the cell store: no `lookups-vs-jev-r1` cell predates the
rerun). Rerun (`bench-rerun-all.sh`, HEAD moved to `3e07b3d6` by then): `56
cell(s) run, 0 lost`. Ledger: 28 rows (`full`) + 28 rows (`full-jev`).

| | `full` | `full-jev` |
|---|---:|---:|
| cells stored | 28 | 28 |
| pass | 22 (79%) | 22 (79%) |
| median tokens/cell | 42,781 | 42,726 |

| Paired stat | Value |
|---|---:|
| paired n (tasks) | 14 |
| better | 1: `H1-T07` 0%→50% |
| worse | 1: `H1-T01` 100%→50% |
| tied | 12 |
| mean paired diff | 0.000 |
| bootstrap 95% CI | [-0.1071, 0.1071] |
| two-sided sign test p (1 better, 1 worse) | 1.000 |
| tokens ratio (sum, paired cells) | 1.10x (1,565,543 / 1,425,289) |
| cost ratio | 1.03x ($1.131 / $1.103) |
| turns ratio | 1.07x (161 / 151) |

**Routing table (per task, `full-jev`'s own `decisions.jsonl`, site=`route`,
4 decisions/task = 2/trial × 2 trials):**

| Task | Route choice(s) | Confidence range | `full-jev` gold-source (from cell) | Passed? |
|---|---|---:|---|---|
| `FRESH-M7-01` | `find_callers` ×4 | 0.75–0.76 | `Grep` | 2/2 |
| `H1-T01` | `obligations` ×4 | 1.00 | `none` ×1 / `mcp__fathom__obligations` ×1 | 1/2 (worse) |
| `H1-T02` | `obligations` ×3, unsure ×1 | 1.00 / 0.00 | `Grep` | 2/2 |
| `H1-T03` | `find_callers` ×4 | 0.34–0.43 | `Grep` | 2/2 |
| `H1-T04` | `staleness_report` ×1, `find_symbol` ×3 | 0.31–0.36 | `none` | 0/2 (tied fail) |
| `H1-T05` | `find_callers` ×4 | 0.82–0.89 | `mcp__fathom__find_callers` | 2/2 |
| `H1-T06` | `find_callers` ×4 | 0.35–0.39 | `Grep` | 2/2 |
| `H1-T07` | `obligations` ×4 | 0.99–1.00 | `Read` | 1/2 (better) |
| `H1-T08` | `obligations` ×4 | 0.94–0.97 | `none` | 2/2 |
| `H1-T09` | `search` ×4 | 0.54–0.61 | `mcp__fathom__search` | 2/2 |
| `H1-T10` | unsure ×1, `search` ×3 | 0.00 / 0.50–0.59 | `mcp__fathom__search` | 2/2 |
| `H1-T11` | `search` ×4 | 0.81–0.84 | `Bash`/`Grep` | 2/2 |
| `H1-T12` | `staleness_report` ×4 | 0.94–0.95 | `Grep` | 2/2 |
| `H1-T13` | `staleness_report` ×4 | 1.00 | `Grep` | 2/2 |

Read plainly: Jev routed every task to a plausible verb (never "unsure" more
than once per task), and the two tasks where the route was `obligations`
(governance-shaped prompts) are exactly the two where the paired outcome
moved — one better, one worse, canceling to a null mean. Routing worked as
a mechanism here too; it simply did not move this corpus's pass rate net of
noise at n=2.

---

### Corpus 3 — `guards-fathom` (governance at the edit), run `guards-jev-r1`

Subject fathom commit `3e07b3d6`; harness `2.1.281 (Claude Code)`; arms
`full` / `full-jev`; trials=2. Ledger: 16 rows (`full`) + 16 rows
(`full-jev`).

| | `full` | `full-jev` |
|---|---:|---:|
| cells stored | 16 | 16 |
| pass | 8 (50%) | 5 (31%) |
| median tokens/cell | 529,680 | 393,964 |

| Paired stat | Value |
|---|---:|
| paired n (tasks) | 8 |
| better | 0 |
| worse | 3: `guards-i19-org-agnostic-fixture` 50%→0%, `guards-i28-duckdb-path` 50%→0%, `guards-i31-workspace-artifact` 100%→50% |
| tied | 5 |
| mean paired diff | -0.1875 |
| bootstrap 95% CI | [-0.3750, -0.0625] |
| two-sided sign test p (0 better, 3 worse) | 0.250 |
| tokens ratio (sum, paired cells) | 0.91x (13,643,995 / 14,986,798) |
| cost ratio | 0.85x ($6.320 / $7.411) |
| turns ratio | 0.99x (654 / 658) |

Obligations-site decisions (`full-jev` only — `full` uses `provider = none`
and made zero decider calls, confirmed: 0 rows in every `full` worktree's
`decisions.jsonl`), summed across all 8 tasks' `decisions.jsonl`:

| | Count |
|---|---:|
| obligations decisions | 15,742 |
| applied | 15,699 (99.7%) |
| errors (not applied) | 43 |
| latency p50 | 337 ms |
| latency p95 | 1,075 ms |
| latency p99 | 1,476 ms |
| route decisions (change-shaped tasks) | 28 — 25 `none` (confidence 0.52–0.98, median 0.82), 3 `find_callers` (confidence 0.42–0.44) |

**Instrument notes (four relaunches, one wedge fault fixed mid-flight):**
`guards-jev-r1` needed 5 total launches to complete:
1. First launch (`bench-jev-all.sh`, HEAD `533d293c`): died `exit=2` after
   scoring 25/32 cells — one cell's oracle-scoring git call failed under
   the same external-volume TCC wedge class, and the pre-fix driver
   aborted the ENTIRE remaining run on any `runOneCell` error past subject
   prep, not just that one cell (`.fathom/observations.md` O111, full root
   cause there). Fixed on branch `wave-dp-b` (commit `76d0b1b2`, merged
   into this run's `work` branch as `9f13e782`): any `runOneCell` error now
   LOSTs one cell and the run continues.
2. Second launch (`bench-rerun-all.sh`, HEAD moved to `3e07b3d6`): `exit=4`.
3. Third launch (`bench-rerun-guards2.sh`): 3 minutes, `exit=4`.
4. Fourth launch (same script, re-invoked): 1 second, `exit=3` — an
   effectively instant abort, consistent with hitting the TCC wedge before
   any real work started.
5. Fifth launch (same script again, this reading's own
   `tccutil reset SystemPolicyRemovableVolumes` discipline kept fresh by a
   background loop for the run's duration): completed — `6 cell(s) run, 0
   lost`, `exit=1` (a normal "at least one cell failed" exit, not a driver
   fault). All 32 cells are real, scored readings; none are void.

---

### Guards mechanism analysis (the load-bearing question)

For each of the 8 guards tasks: the corpus's own `rule` block (governing
invariant + its citation) and target path, then — from every cell's raw
transcript — every injected `⚖️ fathom obligations` hook block
(`UserPromptSubmit` and every `PostToolUse:*`), in order, keeping the
header's own named file(s). **Method (stated so it is exactly
re-computable):** "first relevant block" = the first obligations block, in
transcript order, whose own header names a file under the task's target
path (`internal/store`, `cmd/fathom`, …) — the earliest point in the
session the agent could have seen a rule scoped to that file. "Served" means
the governing rule (matched by keyword against its own citation/topic, not
by exact text) appears among that block's bulleted rules; "position" is
its 1-based rank in that block's own list; "displaced (N)" is that same
block's own `⚠️ N more eligible rule(s) not shown (cap)` line.

| Task | Target | Governing rule | `full` t1 | `full` t2 | `full-jev` t1 | `full-jev` t2 | Paired outcome |
|---|---|---|---|---|---|---|---|
| `guards-i1-packagedeps-sorted` | `internal/analysis/packagedeps` | I1 sort/determinism | served pos 3/6 (79 more) | served pos 3/6 (79 more) | never named | served pos 2/6 (230 more) | tied (0%/0%) |
| `guards-i2-store-purego` | `internal/store` | I2 pure-Go, ADR-025 | not in list (234 more) | not in list (234 more) | not in list (336 more) | not in list (335 more) | tied (0%/0%) |
| `guards-i11-mcp-tool-doc` | `internal/mcp` | I11 tool↔doc sync | not in list (177 more) | not in list (177 more) | not in list (249 more) | not in list (249 more) | tied (100%/100%) |
| `guards-i16-raw-cp-deploy` | `CLAUDE.md`/`Taskfile.yml` | I16 no raw-cp deploy | not in list (174 more) | not in list (174 more) | not in list (245 more) | not in list (246 more) | tied (100%/100%) — registered no-effect case |
| `guards-i19-org-agnostic-fixture` | `internal/connector/testdata` | I19 org-agnostic, ADR-011 | **absent, every block** | absent, every block | **absent, every block** | absent, every block | **worse (50%→0%)** |
| `guards-i28-duckdb-path` | `internal/store` | I28 DuckDB loud refusal, ADR-025 | served pos 1/6 (178 more) | served pos 1/6 (178 more) | served pos 1/6 (256), **then pos 6/6 and pos 3/6 on later touches** | served pos 1/6 (258), **then pos 6/6 on a later touch** | **worse (50%→0%)** |
| `guards-i31-workspace-artifact` | `cmd/fathom` | I31 workspace/ paths | absent (3 blocks) | never named | never named | absent (6 blocks) | **worse (100%→50%)** |
| `guards-i36-usecase-maintained-line` | `docs/use-cases` | I36 Maintained contract | served pos 2/6 (84 more) | served pos 2/6 (84 more) | not in list (125 more) | not in list (128 more) | tied (0%/0%) |

**Per-task deep trace on the 3 regressed tasks (every obligations block
naming the target path, not just the first):**

- **`guards-i28-duckdb-path` — the one directly traced re-rank mechanism.**
  `full` has exactly one block naming `internal/store` in either trial (the
  `UserPromptSubmit`, rule at position 1–2/6) — the session apparently never
  triggered a second `PostToolUse` hook on that path. `full-jev` starts
  IDENTICALLY (position 1/6 at `UserPromptSubmit`, in fact nominally better
  than `full`'s 2/6), but on the LATER `PostToolUse:Bash` touches of
  `internal/store/backend_test.go` within the same session, the rule falls
  to position 6 of 6 (t1: pos 6/6 then pos 3/6; t2: pos 6/6) — the LLM
  re-rank visibly reordering a rule that started in the SAME favorable
  position as the deterministic baseline, specifically at the moment the
  agent had already started writing code and most needed the rule
  reinforced. This is the one task where "the re-rank displaced the rule
  that mattered" is directly evidenced, not inferred.
- **`guards-i19-org-agnostic-fixture` — refutes the re-rank theory for this
  task.** The governing rule (org-agnostic fixtures) is ABSENT from every
  single obligations block this reading found, in BOTH arms, on BOTH
  trials — `full`'s one surviving 1/2 pass and `full-jev`'s fall to 0/2 both
  happened with the rule never served either way. Whatever caused the
  extra failed trial under `full-jev`, it is not obligations-rule
  displacement; the coverage gap is a `full`-arm problem too, and this A/B
  cannot explain the regression from rule-serving evidence alone.
- **`guards-i31-workspace-artifact` — same pattern as i19, more sparsely
  observed.** `full/t1` shows 3 blocks naming `cmd/fathom`, rule absent in
  all 3; `full/t2` and `full-jev/t1` show ZERO blocks naming the path at
  all (the edit apparently went through a Bash pattern the hook's own
  header-parser could not attribute to a specific file — a separate,
  known instrument limitation, not scored here as "served" or "not
  served"); `full-jev/t2` shows 6 blocks, rule absent in all 6. Again, the
  rule is never confirmed served in EITHER arm at any point this reading
  could observe — no positive evidence for displacement, but the observed
  data are too sparse in the `full` arm (2 of 2 trials showing 0–3
  observations) to call this refuted as cleanly as i19.

**Verdict on the load-bearing question:** the evidence **supports** "the
re-rank can displace the governing rule" as a real, demonstrated mechanism
(guards-i28, traced turn-by-turn) but **does not support** it as the
explanation for the guards-jev-r1 regression as a whole — 2 of the 3
worse tasks (i19, i31) show the identical rule-absence pattern in the
UN-reranked baseline arm too, meaning something other than rank
displacement is also driving those two losses, unmeasured by this
reading. The one clean mechanism case, plus the negative paired mean at
real cost (15,742 decisions for a corpus this small) and zero offsetting
`better` tasks, is enough to flip the default off; it is not enough to
call the mechanism proven end-to-end.

---

### Instrument notes

- **Rate-limit loss and rerun (`lookups-vs-jev-r1`):** first attempt lost
  all 56 cells to the weekly Claude session rate limit (`jev-all.log`,
  `exit=3`, zero cells stored under that run-id before the rerun); the
  rerun ran fresh (nothing to borrow) and stored all 56, `0 lost`.
- **Subject commit changed mid-wave:** `lca-jev-r1` ran against fathom
  commit `533d293c`; `guards-jev-r1` and `lookups-vs-jev-r1` ran against
  `3e07b3d6` (`wave-dp-b`'s O111 driver fix, merged in between) — both
  `-commit`/`-fathom-commit` on every `ledger` filing match what each run's
  own cells actually carry (`arm_surface`), confirmed cell-by-cell before
  filing, never assumed from the run's start time.
- **Four guards-jev-r1 relaunches:** see "Instrument notes" under Corpus 3
  above — one real driver bug (O111, fixed) plus repeated external-volume
  TCC wedges (`tccutil reset SystemPolicyRemovableVolumes
  com.mitchellh.ghostty`, this session's own documented fix, re-applied
  before every retry) that killed two of the five launches within seconds.
- **The ledger subcommand could not originally file these rows at all:**
  `scenariobench ledger` recomputed the un-cloned corpus file's own sha,
  never the `-arm-clone`/`-arm-setup` variant `run` actually keyed cells
  on, so every row for the `-jev` arms (and, since the sha is corpus-wide,
  every row for the un-cloned arm too) was unreachable. Fixed in this
  reading (`cmd/scenariobench`, `TestCmdLedger_ArmCloneSetupMatchesRunCorpusVariant`)
  before any of the 109 rows above were filed.
- **LCA route-decision instrumentation gap:** disclosed above under Corpus
  1 — no `decisions.jsonl` was found for any lca subject worktree this
  reading could inspect, so no lca-specific route count/confidence is
  reported; only cell-record-backed numbers (gold-source rate, pass rate,
  tokens/cost) are stated for that corpus.

### What would change this recommendation

- **A guards-shaped governance run at n ≥ 3 trials.** At n=2 the paired
  bootstrap CI [-0.375, -0.062] excludes zero but is wide; a third trial
  per cell would either tighten it toward the current point estimate or
  reveal it as noise from a small sample colliding with the rate-limit/
  TCC-wedge instrument trouble this wave hit repeatedly.
- **A corpus where two governing rules genuinely compete within one
  scope tier.** None of these 8 guards tasks ever gave the re-rank a real
  choice to arbitrate well OR badly between two eligible same-tier rules
  both bearing on the edit — `guards-i28`'s traced displacement was a
  single rule sliding down a list of otherwise-irrelevant candidates, not
  a case of the re-rank correctly preferring a MORE relevant rule over a
  less relevant one. A corpus built specifically to test that contest is
  the direct next step this reading recommends (named as the ADR's own
  revisit trigger).
- **Resolving the i19/i31 rule-absence gap independently of Jev.** Since
  the governing rule for both never serves in the deterministic baseline
  either, fixing that (a separate, un-scoped obligations-coverage defect)
  would let a future guards A/B measure the re-rank's effect on a rule
  that is actually reaching the agent in both arms — the current reading
  cannot separate "the re-rank hurt" from "the rule was never in play" for
  those two tasks.
