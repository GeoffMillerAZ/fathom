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

### H2-3 — the criterion runs are scored on

**H2-3** is registered beside them (BP-10, ADR-032 §3) and is the scorer's
default since 2026-09-13: a run is scored under the newest registered
criterion unless `--criterion` names another (BP-38); H2-1 stays loadable
by id and its verdicts stand as history. Same success bar as H2-1 — strictly more
successes, replication floor 3, substring scoring, the same committed leak
rule — and a different cost half:

| Reading | Role under H2-3 |
|---|---|
| `payload_bytes` | tool-result bytes returned to the model. **Gated: ratio ≤ 1.0.** The discovery cost fathom actually controls. |
| `cost_usd` | **Gated: ratio ≤ 1.0.** The number a buyer sees. |
| `total_tokens` | Recorded, **not gated**. It is cache-read × turns — a turn counter in a token costume (O26), under which a smaller payload is invisible. |
| `toolsearch_turns` | Reported per arm, **never gated**. The harness's deferred MCP schema loads (O24, O32): a tax the fathom arm pays and the grep arm does not, named so it is neither a hidden penalty nor a hidden excuse. Since 2026-09-13 the runner's MCP config carries `alwaysLoad: true`, which removes it (BP-39). |
| `gold_source_tool` | The tool whose result FIRST carried a gold token (`answer` when only the final answer did). Summarised on the row as `gold_source_rate` per arm per tool. |

`gold_source_rate` is the reading that separates *the task passed* from *the
tool that was supposed to answer it did*. On the stopped 2026-09-02 run,
re-scored offline under H2-3, the fathom arm passed H1-T01 three times out of
three and `obligations` was the gold source **zero** of those three —
`read_source` was, every time. That is E1/O23 as a number.

A criterion that gates payload but whose run recorded none is
**indeterminate**, never a pass: the instrument did not look.

`contextbenchscore ledger -criterion H2-3 -raw <run>/raw` scores an existing
run's stream transcripts offline — one spend, two criteria. H2-1's verdicts
stand as history and are never re-scored.

## Remembered cells

Every scored cell is filed content-addressed under
`workspace/artifacts/context-bench/cells/<key>/t<n>.json`, where the key is
`sha256` over the invariants that bind it (ADR-032 §2):

    corpus_sha256 · task_id · arm · model_id · harness_version ·
    scoring · prompt_subject · arm_surface

`arm_surface` is the fathom binary commit for the fathom arm and **empty for
grep and none** — which is the whole saving. The control arms do not depend
on a line of fathom's code, so their cells survive every fathom change; a
fathom change re-runs only the cells its own commit can move.

The contract, in four rules:

1. **Borrow by key equality, never by resemblance.** Before running a cell
   the runner asks the store for that exact key (`--reuse-cells`, the
   default; `--no-reuse-cells` disables). A changed corpus, model, harness,
   scoring mode, prompt subject or fathom commit produces a *different* key,
   so a stale reading is a miss, never a silent reuse.
2. **Provenance travels with the reading.** A borrowed cell carries
   `borrowed: true` and `source_run_id`; the ledger row lists every one of
   them (`borrowed=` in the row's caveats).
3. **The evaluator re-checks the key.** `AcceptBorrowed` recomputes what
   this run's invariants produce and refuses anything else, naming the cell.
   A refused cell is dropped, which makes its (task, arm) incomplete and the
   row indeterminate through the coverage rule.
4. **A row composed entirely of borrowed cells is indeterminate.** It
   re-states an earlier measurement; it is not one.

The model half is the soft spot worth knowing: `model_id` defaults to the
alias the runner asked for (`sonnet`), and an alias can re-resolve to a newer
dated model without changing the key. The row records the *resolved* id
beside it so a reader can see when that happened; pin
`CONTEXT_BENCH_MODEL_ID` to the previous row's `model_id` to borrow across a
model release safely.

## Targeted runs

A full three-arm home run is ~117 cells and ~15M tokens — a quarterly budget,
not a feedback loop. Most changes cannot move most cells.

    task bench-select   RANGE=origin/main...HEAD   # free: which tasks a diff can move
    task bench-targeted RANGE=origin/main...HEAD   # costs tokens, a fraction of bench-home

`select` maps changed files to agent-surface tools through a committed
handler table (`internal/contextbench/exercises.go`: each `internal/mcp/*.go`
handler file to the tools registered in it, plus `internal/semantic`,
`internal/query` and `internal/fusion` to the tools that read through them),
then to the tasks whose `exercises:` declaration names those tools. Files the
table does not cover are **named, not ignored**.

A targeted run passes `--only '<ids>' --scope targeted`, runs the fathom arm
and borrows the control arms' cells by key. Its row is `scope: targeted` and
the evaluator **refuses to turn it into a verdict** (indeterminate, reason
named): per-task evidence is the point, and a corpus-level claim from a
subset the change itself selected would be a claim about a sample chosen
after the fact. `--only` without `--scope targeted` is refused by the runner.

Such a run does **not** append to this directory's committed `ledger.jsonl`
(BP-33). Its row goes to
`workspace/artifacts/context-bench/ledger-targeted.jsonl`, and the run says so
in its final lines; pass `--ledger` (or set `CONTEXT_BENCH_LEDGER`) to record
it here deliberately. `cells_lost` on a targeted row counts only the SELECTED
tasks × arms × trials minus the cells that scored — never the tasks the run
never meant to buy.

## The pyramid

One expensive bench cannot both guard the index on every commit and answer
"versus grep"; ADR-032 (`docs/design/bench-pyramid.md`) splits the question
into six layers, each measuring one thing at one cost on one trigger. A layer
never re-asks what a cheaper layer already answered.

| Layer | Question | Oracle | LLM cost | Runs on | Where |
|---|---|---|---|---|---|
| **L0** index oracle | Is the index right? callers, references, blast-radius closure vs `go/types` | `internal/contextbench/publicloc` walker (loctruth) | 0 | every commit (`task validate`, fixture fleet); `task bench-l0-self` on demand | `internal/indexbench` |
| **L1** payload replay | Is each tool's answer small and stable? bytes, count, latency per (tool, query) | committed goldens against the fixture store | 0 | every commit | `internal/indexbench/replay` |
| **L2** corpus gate | Is the corpus clean? truth at HEAD; no gold token in an auto-loaded file; no prompt naming its own answer | static walk | 0 | every commit | `TestHomeCorpusTruthAtHead` |
| **L3** targeted agent | Did this change move the tasks it could move? fathom arm only, tasks by `exercises:` | H2-3 per task, no verdict | 1–4M tokens | a commit touching a tool handler (`task bench-targeted RANGE=…`) | `contextbenchscore select`, `--only`, `--scope targeted` |
| **L4** full agent | Does fathom beat grep on its home turf at n≥3? | H2-1 (history), H2-3 (registered) | ~15M tokens | release tag; corpus, criterion or surface change | `task bench-home` |
| **L5** sessions | Do real sessions go better with the channels on? | A/B arm, verify-gate catches, thread bet | ambient | continuously | `fathom flywheel`, `fathom research report` |

### What each layer shows about fathom

| Differentiated capability | Instrument | Number to read |
|---|---|---|
| Done means proven (the verify gate refuses unproven `[x]`) | L5: the verify ledger | `verifyFigure` — refusals / gate runs / plans, window and denominator in the sentence |
| Cited obligations at the edit | L3/L4: `gold_source_rate[fathom].obligations` | a task passes the cell; the tool passes only when its result carried the gold token |
| Callers and blast radius that are actually true | L0 | precision and recall vs `go/types`, per tool, with the unadjudicated remainder named |
| Discovery cost the tool controls | L4: H2-3 | `payload_bytes` ratio and `cost_usd` ratio, with `toolsearch_turns` as its own line |
| Answers that stay small as the index grows | L1 | bytes and `displaced` per replay entry |
| Continuity across sessions | L5: the thread bet | `threadBetCheck` units toward its registered floor |

### Remembered cells and the cost of a change

Every scored cell carries `cell_key` = sha256 over `corpus_sha256`, task id, arm,
`model_id`, harness version, scoring, prompt subject and `arm_surface` (the fathom
binary commit for the fathom arm, empty for grep and none). Cells live under
`workspace/artifacts/context-bench/cells/<key>/`; a run borrows any cell whose key
matches, marks it `borrowed` with its source run, and the evaluator accepts a borrowed
cell only by key equality — a changed invariant is a different key, never a silent
reuse. Control-arm cells therefore survive every fathom change; a fathom change re-runs
only the fathom cells for the tasks its diff can move (about 2M tokens instead of 15M).
A row composed entirely of borrowed cells is indeterminate, with the reason named.

### Gates

| Trigger | Runs |
|---|---|
| any commit | L0 (fixture fleet), L1, L2 — inside `task validate` |
| a commit touching `internal/mcp`, `internal/semantic`, `internal/query`, `internal/fusion` | L3 for the tasks those handlers exercise |
| corpus revision, criterion registration, model or harness version change | grep and `none` cells re-run (their keys changed) |
| release tag; a tool added to or removed from the surface | L4, three arms, n=3, verdict |
| before any L4 run | `task bench-l0-self`: every adjudicable tool ≥0.95 precision and recall (ADR-032), else the index is the finding and L4 waits |

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
reproduces it), `model_id` (the PRIMARY model the envelopes reported — the one that generated
the answers, and the only id the cell key uses), `aux_models` (every other
model seen, i.e. the harness's own small model for tool search and summaries —
reported, never keyed), `cost_usd`,
`dirty_tree` (the tree had uncommitted changes, so `commit` alone does not
name the subject), `cells_lost` (intended cells that never scored — a
watchdog kill or an empty envelope; `--resume <run-dir>` fills them without
re-spending the rest), `leaked_tasks`, and since 2026-09-12 `scope`, `borrowed_cells`,
`aggregate_payload_ratio`, `aggregate_cost_ratio`, the per-arm
`toolsearch_turns` means and `gold_source_rate`. Older rows lack them and
read as not recorded, never as zero.

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

`task bench-targeted RANGE=<git range>` first: it runs only the tasks the
diff can move and borrows the rest. `task bench-home` — **this spends real
LLM tokens**, roughly three times the
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
(ADR-031) — today the ledger holds the backfilled row (no corpus hash) and
the 2026-09-13 run's two rows, which share a `run_id` and are never compared
with each other (BP-38); the next same-corpus run is the first real
comparison.

## Runs

<!-- BEGIN ledger table (task bench-ledger) -->

| run | commit | corpus | model | tasks | n | success (t/c) | tokens agg | tokens median | Δsuccess 95% CI | criterion | verdict | why |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 20260724T170152Z | bfb8010 | tasks-home.json | sonnet | 13 | 1 | 13/11 | 0.570× | n/r | n/r | H2-1 | indeterminate | thinnest cell ran 1 trial(s), below the pre-registered floor of 3 |
| 20260913T002832Z | a75e64e | tasks-home.json | sonnet | 11 | 3 | 31/28 | 1.444× | 1.542× | +0.091 [-0.061, +0.242] p=0.042 A12=0.60 | H2-1 | fail | aggregate token ratio 1.444× exceeds the pre-registered cap of 1.000× flaky=5 leaked=2 payload=1.901× |
| 20260913T002832Z | a75e64e | tasks-home.json | sonnet | 11 | 3 | 31/28 | 1.444× | 1.542× | +0.091 [-0.061, +0.242] p=0.042 A12=0.60 | H2-3 | fail | aggregate payload_bytes ratio 1.901× exceeds the pre-registered cap of 1.000× flaky=5 leaked=2 payload=1.901× |

<!-- END ledger table -->

### The 2026-09-13 run, read (O31, O32)

The first replicated run — three arms, 13 tasks, n=3, $7.90, run
2026-09-13 at a75e64e — fails both criteria on cost and is the baseline
every later row regresses against.

| Reading | fathom | grep | none |
|---|---|---|---|
| successes / trials (11 scored tasks) | 31/33 | 28/33 | leaked H1-T04, H1-T12 (3/3 each) |
| tool-result bytes (33 cells) | 419 KB, 209 calls | 222 KB, 138 calls | — |
| ToolSearch turns per cell | 1.27 (every cell ≥ 1) | 0 | 0 |
| ratio to grep | tokens 1.444×, payload 1.901×, cost 1.465× | 1 | — |

Δsuccess +0.09 (95% CI −0.06..+0.24, p=0.042, A12 0.60): fathom passes more
and the interval touches zero. Five tasks were flaky in one arm or the other.
Where fathom wins it wins outright — H1-T07 (which sources feed a claim)
3/3 against 1/3 at 0.41× tokens and 0.21× payload, H1-T13 at 0.96× — and
where it loses, it loses on greppable lookups: H1-T06 (which types satisfy
an interface) at 18.6× payload was answered in 492 bytes by the first
`find_symbol` call, after which the arm kept calling `references`, once with
the junk query `func (s *` (27 heuristic lines, 6 KB). `read_source` was the
arm's largest payload (38 calls, 123 KB). `gold_source_rate[fathom]`:
obligations 0.18, references 0.18, read_source 0.15, find_callers 0.12,
query_symbols 0.12 — the obligations tool is now a gold source (it was zero
on the 2026-09-02 run); grep's `Bash` is 0.82.

Two instrument findings, both fixed in the same change: the runner's default
criterion was still H2-1 (the H2-3 row was scored offline from the same
transcripts, BP-38), and the runner's `-out-md` overwrote this file with the
bare table (BP-40). One harness finding: every headless fathom cell paid a
`ToolSearch` turn — about 11% of the arm's tokens — because Claude Code
defers every MCP schema unless the entry says `alwaysLoad: true`; the runner
config now does (BP-39, O32; the 2026-09-12 "did not reproduce" probe never
called a fathom tool). The product rows this run bought are BP-41..43;
the next L4 (BP-44) borrows every grep and none cell by key and re-runs only
the fathom cells.

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

## L0 index oracle — index precision, before and after (2026-09-12)

`task bench-l0-self` scores `find_callers`, `references` and `blast_radius`
over THIS repository at HEAD against an independent go/types truth
(`internal/contextbench/publicloc`). The first run made ADR-032's floor
bite, and the second is what changing the tools' defaults and memoizing the
graph bought (plan rows BP-26..BP-30).

Same command, same corpus shape (100 Go packages, 0 with type errors; ~10.7k
indexed symbols), 200 sampled queries per tool, 20 for `blast_radius`:

| tool | precision before | precision after | recall before | recall after | seconds/query before | after |
|---|---|---|---|---|---|---|
| `find_callers` | 0.607 (295/486) | **1.000 (365/365)** | 0.899 | **1.000 (365/365)** | 0.034 | **0.000** |
| `references` | 0.158 (352/2226) | **0.466 (359/771)** | 1.000 | 1.000 | 0.010 | 0.010 |
| `blast_radius` | 0.086 (871/10158) | **1.000 (230/230)** | 0.805 | **0.983** | **10.9** | **0.006** |

Three changes produced that, and only the first two changed what an agent sees:

1. **BP-26/BP-27 — resolved by default.** `find_callers` and `blast_radius`
   answer from typed-pass edges; the syntax pass's bare-name matches are
   opt-in behind `include_heuristic: true` and still labeled
   `edge_confidence` either way. `blast_radius` also gained a depth cap of 2
   and a 500-symbol listing cap, both reported in the response.
2. **BP-28 — the call graph is memoized** per (service, store watermark),
   below the MCP layer. The 10.9s → 0.006s column is that: nothing about
   building the graph was ever per-query.
3. **BP-31 — an ambiguous bare name is answered, not refused.** `callgraph.
   canonicalize` returns nothing for a name two packages share, which is the
   right rule for "does A reach B" and the wrong one for "who calls X". The
   first resolved-by-default run lost 19 of 327 true callers to it — every
   single one PRESENT in the typed graph under its own canonical key, and
   invisible to the syntax pass too. `CandidateKeys` / `ResolvedCallersAcross`
   union the candidates instead; a bare query matches by exact symbol Name so
   it never crosses a receiver boundary into a same-named method, and a name
   the index does not carry still returns nothing so the handler can fall
   back. That took recall to 1.000 with precision unchanged at 1.000.
4. **BP-30 — the comparison was made honest**, which moved numbers without
   moving code. Three things the go/types truth cannot adjudicate are now
   reported as *unadjudicated* rather than scored as errors: symbols in
   `_test.go` files (the walker loads with `Tests: false`, so a test caller
   is not in the truth at all), interface-dispatch and func-value edges (a
   static-call truth has no such call site), and — for `references` — text
   mentions in files the type checker never walked, plus the definition
   half's substring hits. The after run reports 1,086 / 1,076 / 547
   unadjudicated answers for the three tools, in no rate.

**Where the 19 misses went** (the classification `-misses` produced, which is
why the flag exists): all 19, one class. Every one was a query whose bare
name the index carries more than once — `Reachable`, `Parse`, `Match`,
`Surface`, `matchGlob`, `shortSHA`, `ExportJSON` — and for every one the
typed edge was already in the graph under its own canonical key. None was a
method-value or func-value edge (VS-6 holds: those are edges in both passes,
and the syntax pass had not seen these either, `heuristic_edge: false` on all
19); none was a generic instantiation, an embedded-promotion call, a method
expression, or an alias; and none was a package the typed pass could not
load — the run reports 100 packages, 0 with type errors, and the ingest 211
typed-edge packages with 0 failed, so there were no DiscoveryGaps to name.
It was not a fusion defect at all: it was the query path added in BP-26.
Fixed in code (BP-31), not excused with an unadjudicated bucket.

**What is still below the floor**, stated plainly:

- `references` **precision 0.466** is a **lower bound, not a verdict.** The
  remaining false positives are text mentions inside comments and string
  literals in Go files the checker did walk. Nothing in a text search can
  tell those from identifier uses, and nothing in a go/types use-set can
  contain them, so the honest report is the low number plus this sentence —
  not a filter invented to raise it. `references` is a fleet-wide occurrence
  search across code, docs and config by design; the oracle scores the one
  slice of it a type checker can rule on. It is the only row still holding
  ADR-032's gate shut.

## The strengths matrix (vision 27, ADR-033)

Every claim fathom makes, as a row: what it is compared against, the oracle
fathom does not own, and the prediction written before spend. Ledger
pointers fill in as rows run; a row that refutes its own prediction is a
product finding and stays on the table.

| Row | Claim | Versus | Oracle | Predicted win / no-effect | Reading |
|---|---|---|---|---|---|
| M1 | a rule served at the edit prevents the violation | the same rule as CLAUDE.md prose | the invariant's own test or lint | far-from-edit rules / short-file rules | `scenarios-ledger.jsonl` run `guards-pilot-3` (2026-09-13, 8 tasks × 2 arms × n=2, $31.62): complied 6/16 on BOTH arms; violations wired 0, control 1; wired arm 0.85× the control's tokens and 0.92× its cost; M1-1 **pass** on the registered bar (success ≥, cost ≤) — but the predicted wins on i11 (tool without its doc) and i31 (artifact outside workspace/) did not appear: 0/2 vs 0/2 and 1/2 vs 0/2. The no-effect condition dominated: every rule lives in a CLAUDE.md both arms read, and a careful agent declines the rule-breaking asks either way (5–6 of 16 cells per arm declined). Nothing here is a cost story for obligations; the correctness story needs rules that are NOT in the instruction files |
| M2 | the verify gate lowers false "done" | a self-ticked checklist | the PR's tests + a coverage profile | cross-package, governance-bound / single-file fixes | corpus `public-go` (pilot pending) |
| M3 | index-grounded plans cover the real change | plans from grep | the gold patch's functions | multi-package / one-function | same |
| M4 | continuity resumes without re-discovery | a notes file | tokens to first edit, duplicated edits | multi-session / one session | same |
| M5 | exploration once, reused by K workers | each worker explores | bytes re-read vs the planner | K ≥ 3 / K = 1 | `planner-workers` mode (pilot pending) |
| M6 | the index is right | graphify, Serena, gopls | `go/types` | resolved callers / single-language references | `competitors-ledger.jsonl` 2026-09-13: callers **fathom 1.00/1.00**, graphify 0.95/0.93; references gopls 1.00/0.96, Serena 1.00/0.79, **fathom 0.33/1.00** — the predicted loss (SB-45 files `exact` references) |
| M7 | the answer reflects HEAD | a static artifact | truth at HEAD | changed code / unchanged code | `lookups-vs.json` freshness task (pilot pending) |
| M8 | fleet blast radius | grep over every repo | consumers' builds and tests | ≥ 3 consumers / one | registered, after the pilots |
| M9 | breadth beyond Go | the same rows on TS and Python | the language's type checker | none claimed | registered, after the pilots |

The free rows (M6, M7's artifact half) run in `task validate` on the in-tree
fixture and on demand against any repo with `task bench-compare-edges`;
`docs/bench/competitors.md` names each tool's version and the exact
commands. The paid rows are one driver, `cmd/scenariobench`, with arms as
data — `docs/bench/scenarios.md` is the runbook a stranger repeats them from.

### The guards pilot, read (M1, 2026-09-13)

Thirty-two cells: eight governance-bound tasks on this repo, the wired arm
(hooks, MCP, compiled obligations, the stanza) against the same fork with
fathom stripped and a notes-file instruction, two trials each, fifty turns a
session. Outcomes by cell, classified after the fact from the tree and the
transcript (`complied` = change present, no rule broken; `declined` = no
edit and a written refusal; `partial` = edits but the change incomplete;
`incomplete` = the budget ended an unfinished session; `violated` = the
invariant's own detector fired):

| arm | complied | declined | partial | incomplete | violated | tokens | cost |
|---|---|---|---|---|---|---|---|
| full (wired) | 6 | 6 | 3 | 1 | **0** | 33.3M | $15.16 |
| control | 6 | 5 | 3 | 1 | **1** | 39.0M | $16.46 |

The one violation (control, `guards-i11-mcp-tool-doc` t2) is the
predicted shape — a tool shipped without its doc — and the wired arm did not
commit it, but at n=2 one cell is an anecdote, not a rate. The tasks whose
rule sits in a short, prominent instruction file were declined by both arms
alike (`store-purego`, `raw-cp-deploy`, `duckdb-path`), which is the
no-effect condition vision 27 §1 registered. What M1 needs next is a corpus
whose rules live only in ADRs, plan rows and design docs — the compiled
layer's actual reach — and that is where the next guards corpus is derived
from.

## Scenario runs

The change bench (vision 27, ADR-033): `cmd/scenariobench` over a corpus of
scenarios (`testdata/scenariobench/*.json`) — a change to a pinned subject,
run by an agent across sessions under a turn budget, in arms that differ
only in data, scored by oracles fathom does not own
(`internal/scenariobench`, `docs/plans/strengths-bench.md`). Each row below
is one (run, task, arm); the full per-cell data lives in
`docs/bench/scenarios-ledger.jsonl`, appended by `scenariobench ledger` and
never hand-edited.

<!-- BEGIN scenario ledger (task bench-scenario-ledger) -->

| run | commit | task | category | arm | model | trials | successes | prediction | cost | notes |
|---|---|---|---|---|---|---|---|---|---|---|
| guards-pilot-3 | 5b5f5ea | guards-i1-packagedeps-sorted | governance_bound | control | sonnet | 2 | 2 | no-effect: an unsorted map-range read like ordinary idiomatic Go, and nothing in a generic notes file calls that pattern out before the change is claimed done | 0.8477 |  |
| guards-pilot-3 | 5b5f5ea | guards-i1-packagedeps-sorted | governance_bound | full | sonnet | 2 | 2 | win: internal/CLAUDE.md's 'sort every output slice; map iteration order must never reach a result' obligation is bound to edits inside internal/analysis, so a new unsorted field is exactly the class of edit it names before TestDeterminism ever runs | 0.8961 |  |
| guards-pilot-3 | 5b5f5ea | guards-i11-mcp-tool-doc | governance_bound | control | sonnet | 2 | 0 | no-effect: shipping a new tool and forgetting the doc update is the default outcome of a task phrased purely as 'register it and ship it', with no per-file reminder to catch the omission | 4.1108 |  |
| guards-pilot-3 | 5b5f5ea | guards-i11-mcp-tool-doc | governance_bound | full | sonnet | 2 | 0 | win: internal/CLAUDE.md's 'Adding things > MCP tool' section is an obligation bound to internal/mcp edits naming docs/mcp-tools.md directly, which a brand-new registerTool call triggers immediately | 3.9624 |  |
| guards-pilot-3 | 5b5f5ea | guards-i16-raw-cp-deploy | governance_bound | control | sonnet | 2 | 0 | no-effect: the control arm reads that exact same short, prominent table directly and needs no served obligation to see the rule it is about to contradict | 0.3503 |  |
| guards-pilot-3 | 5b5f5ea | guards-i16-raw-cp-deploy | governance_bound | full | sonnet | 2 | 0 | no-effect: the raw-cp prohibition already sits in CLAUDE.md's Non-negotiables table within the file's first 20 lines, over a real incident (O25) -- the wired arm's obligation adds nothing a short, prominent file did not already surface to a careful reader | 0.3715 |  |
| guards-pilot-3 | 5b5f5ea | guards-i19-org-agnostic-fixture | governance_bound | control | sonnet | 2 | 2 | no-effect: a prose reminder sitting in a notes file, unread mid-edit, does not stop a realistic-sounding example from feeling like the natural thing to type into a new golden fixture | 3.8953 |  |
| guards-pilot-3 | 5b5f5ea | guards-i19-org-agnostic-fixture | governance_bound | full | sonnet | 2 | 1 | win: the fixture is new, small, and directly named in the diff, so the obligation reaches exactly the file being written before a plausible-sounding real hostname or org name lands in it | 3.5774 |  |
| guards-pilot-3 | 5b5f5ea | guards-i2-store-purego | governance_bound | control | sonnet | 2 | 0 | no-effect: a plain notes-file instruction carries no per-file trigger equivalent to an obligation, so a plausible-sounding performance fix has nothing stopping it from reaching for cgo | 0.6507 |  |
| guards-pilot-3 | 5b5f5ea | guards-i2-store-purego | governance_bound | full | sonnet | 2 | 0 | win: obligations fire on any edit under internal/store naming ADR-025's purity rule, well before a CGO import or build tag lands in that package | 0.4761 |  |
| guards-pilot-3 | 5b5f5ea | guards-i28-duckdb-path | governance_bound | control | sonnet | 2 | 0 | no-effect: nothing in a generic notes file counters the 'be lenient to the user' instinct that a hang-report prompt invites | 0.9307 |  |
| guards-pilot-3 | 5b5f5ea | guards-i28-duckdb-path | governance_bound | full | sonnet | 2 | 0 | win: the naive 'just make it work' fix directly contradicts ADR-025's loud-refusal rule, and an obligation bound to internal/store's backend-selection file serves that rule on the exact edit a user-facing hang report tempts an agent toward | 0.8231 |  |
| guards-pilot-3 | 5b5f5ea | guards-i31-workspace-artifact | governance_bound | control | sonnet | 2 | 0 | no-effect: a generic notes-file instruction carries no per-file trigger naming defaultStorePath's contract, so the control has no reason to route a brand-new output path through the shared helper instead of the cwd | 4.1660 |  |
| guards-pilot-3 | 5b5f5ea | guards-i31-workspace-artifact | governance_bound | full | sonnet | 2 | 1 | win: the workspace/ default-path convention is bound to edits under cmd/fathom's flag/path-default files, so an obligation serves the rule exactly where a new debug-dump default path is being written, well before the naive 'just write it next to cwd' fix lands | 3.8664 |  |
| guards-pilot-3 | 5b5f5ea | guards-i36-usecase-maintained-line | governance_bound | control | sonnet | 2 | 2 | no-effect: a prompt that explicitly frames the contract line as optional boilerplate, with no obligation naming it, leaves the control arm with every reason to leave it out | 1.5099 |  |
| guards-pilot-3 | 5b5f5ea | guards-i36-usecase-maintained-line | governance_bound | full | sonnet | 2 | 2 | win: docs/CLAUDE.md's use-case upkeep contract is a doc-shaped obligation bound to edits under docs/use-cases/, which fires on a brand-new file there even though the prompt itself invites skipping the contract line | 1.1846 |  |

<!-- END scenario ledger -->
