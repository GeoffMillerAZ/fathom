# The bench ledger

Every context-bench run, in git, one line each. `ledger.jsonl` is the record;
this file renders it (`task bench-ledger`, free) and explains how to read it.

Raw transcripts and per-task scores stay under `workspace/artifacts/context-bench/`
per the repo's workspace convention. Only the per-run summary is committed —
small enough to review, durable enough to show a trend, which is the part a
benchmark needs if it is to be evidence rather than a claim.

## What a run is

Two arms answer the same corpus of questions about the subject repo, scored
against hand-verified ground truth; a third measures what needs no tools:

- **fathom** — the read-only MCP companion surface only; no Bash, no Read, no Grep.
- **grep** — `grep`/`find`/`cat` via Bash only; no MCP tools.
- **none** — no tools at all: the memorisation floor (PB-9). A task this
  arm passes in 2 of 3 trials is `leaked` under the corpus's committed
  `leak_rule`, leaves the verdict, and is named on the row (`leaked=`).

`memory_search` is **deliberately absent** from the fathom arm's allowlist
even though it is on the live agent surface. It recalls conversational
continuity — prior sessions' notes — not code facts (ADR-028's authority
boundary: memory may recall, never assert a fact about code). A bench of
code discovery that let the treatment arm answer from an earlier session's
transcript would be measuring memory, not the index. `ingest` is absent for
the older reason: a bench never mutates the store it is pointed at.

Corpora: `testdata/contextbench/tasks.json` (grep-native lookups, the B1
corpus) and `testdata/contextbench/tasks-home.json` (index-native governance
and enforcement questions, the home-turf corpus). Both are committed data;
every ground truth is re-derivable by hand at the commit its `source` names,
without a running daemon. The home corpus goes further: every ground-truth
token carries a `truth_at_head` anchor that `TestHomeCorpusTruthAtHead`
re-checks in `task validate`, so a file edit that makes a ground truth false
fails the gate in the same change (PB-1), and `truth_at_head_revision`
names the commit of the last full re-sweep.

Tasks whose premise is what the LLM-compiled obligations layer serves
declare `requires: [compiled-layer]` (PB-12). The runner skips them — never
scores them as misses — when its environment lacks the capability
(`--env`), which is what a `--scratch-daemon` run against a fresh store is.

### Two criteria, one bar

`machine_criterion` is **H2-1**: strictly more successes than grep, aggregate
tokens ≤ 1×, replication floor 3, scored by substring match of each answer.
**H2-2** is registered beside it (`registered_criteria`) with the same bar
and a stricter success definition: the answer must be a fenced code block
whose lines number at most the task's `max_predicted` (default
`max(5, 2·|gold|)`), test-file paths are dropped from gold, and every cell
records precision and recall beside pass/fail (`ScoreWithPrecision`). It
exists because all-of substring scoring is passable by dumping the tree.
It is a *new* criterion, not an edit to H2-1, because it changes what
"success" means after H2-1 has recorded data — H2-1's verdicts stand as
history and are never re-scored. `--scoring precision --criterion H2-2`
runs under it.

## How to read a row

| Column | Means |
|---|---|
| `n` | trials per cell — the thinnest one. A corpus is only as replicated as its thinnest cell. |
| `success (t/c)` | successful trials, treatment vs control, summed across tasks |
| `tokens agg` | total-over-total ratio, per trial. **This is what the criterion reads.** |
| `tokens median` | the middle per-task ratio. Interpretation only, never the gate. |
| `Δsuccess 95% CI` | `point [low, high] p=… A12=…`: the mean per-task success-rate difference (treatment − control) with its seeded, task-clustered paired bootstrap 95% CI; `p` is the two-sided Wilcoxon signed-rank on per-task mean tokens (absent when no task differs); `A12` is Vargha–Delaney's A over per-task success rates (0.5 = no effect). The row also carries per-arm `pass_at_1` and `pass_pow_3` and the `bootstrap_seed`. **Reported, never gating** — from agentic-kit `reliability-stats`, whose non-guarantees apply (percentile interval, no multiple-comparison correction, coarse exact `p` at small n). `n/r` for rows written before PB-10. |
| `verdict` | `pass` · `fail` · `indeterminate` |
| `why` | the sentence the verdict was computed from, plus `absent=` / `flaky=` / `cells_lost=` / `leaked=` caveats |

Not rendered but on every row since 2026-09-02 (`ledger.jsonl`): `per_task`
(the paired outcomes the verdict was computed over — what the regression
gate compares), `corpus_sha256` (the exact corpus bytes; `shasum -a 256`
reproduces it), `model_id` (the dated id the alias resolved to), `cost_usd`,
`dirty_tree` (the tree had uncommitted changes, so `commit` alone does not
name the subject), `cells_lost` (intended cells that never scored — a
watchdog kill or an empty envelope; `--resume <run-dir>` fills them without
re-spending the rest), `leaked_tasks`. Older rows lack them and read as
not recorded, never as zero.

Two ratios are reported because they disagree in the way that matters. On
2026-07-24 the fathom arm was cheaper on 8 of 13 tasks and still lost the
aggregate at 1.071×, because five expensive tasks — one at 5.20× — carried
the total. The median says "usually cheaper"; the aggregate says "not cheaper
overall". Both are true, and a run that reported only the friendlier one
would be lying by selection. The pre-registered criterion is written over the
aggregate, so the gate cannot be softened by preferring the median after the
fact.

`flaky=` counts tasks whose outcome varied across trials in either arm. At
n=1 that number is unreachable by construction, which is the whole reason the
replication floor exists.

## Verdicts

`pass` and `fail` are both measured results and both belong in the ledger; a
recorded refutation is the instrument working. `indeterminate` is different —
it means the run did not measure enough to decide, and it is returned when
any task ran in only one arm, or when the thinnest cell fell below the
corpus's registered replication floor. **An incomplete run can never report a
win** (ADR-027: an instrument may not degrade toward "healthy").

The criterion lives in the corpus file as `machine_criterion`, committed
before the run, pinned by a test so loosening it is a visible edit rather
than a quiet one. Nothing else may declare a win.

## Running one

`task bench-home` — **this spends real LLM tokens**, roughly three times the
2026-07-24 run at the default `--repeat 3` (~13M tokens, ~$13 at 2026-09
prices; add a third for the `none` arm). It is operator-initiated: never in
`validate`, never in a hook, never in CI (`docs/intent/autonomy.md`: ask
before LLM spend).

Against another repo: `scripts/context-bench.sh --subject-root <dir>
--service <id> --prompt-subject 'the acme codebase' --scratch-daemon` builds
`bin/fathom` from this tree, ingests the subject into a store under
`workspace/tmp/` (never the fleet store, never a post-fix tree), serves it
on a free loopback port for the run, and stops it at exit. `--fathom-url`
points the fathom arm at a daemon you already run.

## Regression

`task bench-regress` (free) compares the newest ledger row against the
previous row with the same `corpus_sha256`, task by task on the treatment
arm's success rate: a task that fell is a regression (exit 1); a task flaky
in either row is listed, not failed; tasks added since the baseline are not
regressions; no comparable baseline is exit 3, not a pass. The semantics
mirror agentic-kit's eval-runner `Regression` so the fleet has one
convention. It joins `validate` only after two same-corpus rows exist
(ADR-031) — today the ledger holds one backfilled row with no corpus hash,
so it reports exactly that.

## Runs

<!-- BEGIN ledger table (task bench-ledger) -->

| run | commit | corpus | model | tasks | n | success (t/c) | tokens agg | tokens median | Δsuccess 95% CI | verdict | why |
|---|---|---|---|---|---|---|---|---|---|---|---|
| 20260724T170152Z | bfb8010 | tasks-home.json | sonnet | 13 | 1 | 13/11 | 0.570× | n/r | n/r | indeterminate | thinnest cell ran 1 trial(s), below the pre-registered floor of 3 |

<!-- END ledger table -->

### History before the ledger

The ledger begins 2026-09-01. Two earlier runs predate it, and both were
**n=1 per cell** — indeterminate under the replication floor registered on
2026-09-01, though their verdicts stand as accurate history of the
instrument that produced them:

- **2026-07-24, home corpus, 14-tool surface** — `docs/plans/home-bench.md`
  H3-1. Honest negative: success 12/13 vs 12/13 (equal, not strictly
  greater) at 1.071× tokens. Both halves of the criterion missed. Not in
  the ledger: the later run on the same day superseded it.
- **2026-07-24, home corpus, 16-tool surface with `read_source`** —
  `docs/plans/trust-and-tokenomics.md` T5-8. Same untouched criterion:
  success 13/13 vs 11/13 at 0.570×. Both halves met at the time. This is
  the ledger's one **backfilled** row (`contextbenchscore backfill`,
  PB-2): its numbers come from the plan record because the run's artifacts
  survive in no checkout, it is evaluated under today's criterion (so it
  reads `indeterminate`, n=1), and it carries no per-task data and no
  corpus hash — nothing can pair a regression check against it. The
  median column shows `n/r` (not recorded) for the same reason.
