# The bench ledger

Every context-bench run, in git, one line each. `ledger.jsonl` is the record;
this file renders it (`task bench-ledger`, free) and explains how to read it.

Raw transcripts and per-task scores stay under `workspace/artifacts/context-bench/`
per the repo's workspace convention. Only the per-run summary is committed —
small enough to review, durable enough to show a trend, which is the part a
benchmark needs if it is to be evidence rather than a claim.

## What a run is

Two arms answer the same corpus of questions about this repo, scored by
substring match against hand-verified ground truth:

- **fathom** — the read-only MCP companion surface only; no Bash, no Read, no Grep.
- **grep** — `grep`/`find`/`cat` via Bash only; no MCP tools.

Corpora: `testdata/contextbench/tasks.json` (grep-native lookups, the B1
corpus) and `testdata/contextbench/tasks-home.json` (index-native governance
and enforcement questions, the home-turf corpus). Both are committed data;
every ground truth is re-derivable by hand at the commit its `source` names,
without a running daemon.

## How to read a row

| Column | Means |
|---|---|
| `n` | trials per cell — the thinnest one. A corpus is only as replicated as its thinnest cell. |
| `success (t/c)` | successful trials, treatment vs control, summed across tasks |
| `tokens agg` | total-over-total ratio, per trial. **This is what the criterion reads.** |
| `tokens median` | the middle per-task ratio. Interpretation only, never the gate. |
| `verdict` | `pass` · `fail` · `indeterminate` |
| `why` | the sentence the verdict was computed from, plus `absent=` / `flaky=` caveats |

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
2026-07-24 run at the default `--repeat 3`. It is operator-initiated: never
in `validate`, never in a hook, never in CI (`docs/intent/autonomy.md`: ask
before LLM spend).

## Runs

<!-- BEGIN ledger table (task bench-ledger) -->

_No runs recorded yet. `task bench-home` produces the first row (it spends real tokens)._

<!-- END ledger table -->

### History before the ledger

The ledger begins 2026-09-01. Two earlier runs are recorded in their plans
rather than here, and both were **n=1 per cell** — indeterminate under the
replication floor registered on 2026-09-01, though their verdicts stand as
accurate history of the instrument that produced them:

- **2026-07-24, home corpus, 14-tool surface** — `docs/plans/home-bench.md`
  H3-1. Honest negative: success 12/13 vs 12/13 (equal, not strictly
  greater) at 1.071× tokens. Both halves of the criterion missed.
- **2026-07-24, home corpus, 16-tool surface with `read_source`** —
  `docs/plans/trust-and-tokenomics.md` T5-8. Same untouched criterion:
  success 13/13 vs 11/13 at 0.570×. Both halves met.
