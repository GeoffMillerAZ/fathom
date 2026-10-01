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
(`--env`). `--env` defaults to `compiled-layer` (VB-36) — a default run
covers all thirteen home tasks, eight deterministic and five
`requires: [compiled-layer]` — unless the operator passes `--env raw` to
opt out; see "Scratch daemon by default" below.

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
| `gold_source_tool` | The tool whose result FIRST carried a gold token (`answer` when only the final answer did). Summarised on the row as `gold_source_rate` per arm per tool (`RenderGoldSourceRate` lists every tool seen in ANY arm, at 0 where an arm never won with it — VB-5), and as `gold_source_by_shape` per question shape (task `category`) per arm per tool, so "obligations wins on governance, grep wins on lookups" is not one number hiding two findings. |
| `trusted_without_recheck` | RC-17 (F8, vision 28 §2.4). A fathom tool call is *trusted* unless, within the next 3 turns, a Grep/Read/Bash targets the same file path or symbol name that appeared in the call's own input or result. Reported per run and per tool (`{calls, trusted, rate, by_tool}`) on the results artifact, the rendered summary, and the ledger row's caveat column (`trusted=NN%(n=…)`). **Never gated** — it is the search-receipt (SB-48) falsifier: a rerun with a receipt on search that still shows low trust says the lookup tools are not believed on their own. |

`gold_source_rate` is the reading that separates *the task passed* from *the
tool that was supposed to answer it did*. On the stopped 2026-09-02 run,
re-scored offline under H2-3, the fathom arm passed H1-T01 three times out of
three and `obligations` was the gold source **zero** of those three —
`read_source` was, every time. That is E1/O23 as a number.

`trusted_without_recheck` is the companion reading, from the other end: the
home bench's first pass at F8 found a fathom answer stood without a
Grep/Read/Bash re-check 85-90% of the time it was called, while 27% of plain
Greps were followed by a Read of the same file — the search-receipt case for
SB-48, not just a token-ratio one.

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
| **L0** index oracle | Is the index right? callers, references, blast-radius closure vs `go/types` | `internal/contextbench/publicloc` walker (loctruth) | 0 | every commit (`task validate` → `bench-l0-gate`, fixture fleet); `task bench-l0-self` on demand | `internal/indexbench` |
| **L1** payload replay | Is each tool's answer small and stable? bytes, count, latency per (tool, query) | committed goldens against the fixture store | 0 | every commit (`task validate` → `bench-l0-gate`) | `internal/indexbench` |
| **L2** corpus gate | Is the corpus clean? truth at HEAD; no gold token in an auto-loaded file; no prompt naming its own answer | static walk | 0 | every commit | `TestHomeCorpusTruthAtHead` |
| **L3** targeted agent | Did this change move the tasks it could move? fathom arm only, tasks by `exercises:` | H2-3 per task, no verdict | 1–4M tokens | a commit touching a tool handler (`task bench-targeted RANGE=…`) | `contextbenchscore select`, `--only`, `--scope targeted` |
| **L4** full agent | Does fathom beat grep on its home turf at n≥3? | H2-1 (history), H2-3 (registered) | ~15M tokens | release tag; corpus, criterion or surface change | `task bench-home` |
| **L5** sessions | Do real sessions go better with the channels on? | A/B arm, verify-gate catches, thread bet | ambient | continuously | `fathom flywheel`, `fathom research report` |

L0 and L1 now run on every commit as their own named `task validate`
dependency, `bench-l0-gate` (ADR-035 decision 5, VB-6/VB-7), rather than
only being swept in by the package-wide test run. `TestIndexOracleGoldens`
gates the per-tool precision/recall floor at
`internal/indexbench/testdata/l0-goldens.json`; `TestPayloadReplay` gates
bytes (±5%), result counts and `obligations`' `displaced` count at
`internal/indexbench/testdata/l1-replay.json` — both scored on the
fauxtown fixture fleet in under a second.

The same `bench-l0-gate` run also carries the L0 freshness probe (VB-17,
`TestFreshnessProbe`, `internal/indexbench/testdata/freshness-probe.json`,
`task bench-freshness-probe`; `docs/bench/scenarios.md` "Freshness (M7)"):
five scripted mechanical edits against a mutable fixture copy, each scored
before and after the hash-gated re-ingest with no model in the loop, so the
free half of M7 ("the answer reflects HEAD") runs continuously — the paid
five-arm task and a graphify `--watch`-mode arm are the reading still to
buy.

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
| `why` | the sentence the verdict was computed from, plus `absent=` / `flaky=` / `cells_lost=` / `leaked=` / `out_of_root=` / `flagged=` caveats |

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
`toolsearch_turns` means and `gold_source_rate`. Since 2026-09-14 (VB-36,
O47): `out_of_root_reads` (the total count of Bash/Read tool inputs across
every cell that named an absolute path outside their session's own root)
and `flagged_cells` (the `task/arm/trial` ids of every cell carrying a
`path_leak` or an `out_of_root_reads > 0` — see "Scratch daemon by
default" above). Older rows lack them and read as not recorded, never as
zero.

Per cell (`workspace/artifacts/context-bench/<run>/results/*.json`, and the
content-addressed store under `cells/`), not aggregated onto the ledger row
itself: `path_leak` / `path_leak_paths` (a served payload named the live
checkout's own absolute path) and `out_of_root_reads` / `out_of_root_paths`
(a Bash/Read tool input named an absolute path outside the session root,
named as `"<tool>: <path>"`) — VB-36/O47.

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

### Where fathom stands (authored)

This paragraph is hand-written, never touched by a render (RC-35): the
rendered "Where fathom stands" blocks above the ledger tables — `scenariobench
ledger` and `benchledgerrender` — insert only between their own marker
comments, exactly as `bench-ledger-render.sh` does today, so this slot is
where a reader gets the plain-language reading a machine render cannot make:
which claims are actually load-bearing right now, and which are not.

As of 2026-09-17, evening (O62–O77): every corpus has now been run on the
repaired instrument, with tool-neutral prompts, and read transcript by
transcript before it was believed; the readings live under
`docs/bench/readings/`. Lookups (vs-pilot-6, 140 cells, 5 lost to the
harness's own guards): after five voids serena leads at 26 of 27, fathom's
arm 24 of 28, graphify 22 of 27, gopls 21 of 26, grep 20 of 27; fathom's arm
ran at 0.76 times grep's tokens and the same cost, and the verify-gate task
is still the one question only the index answers cleanly. The freshness
task went 2 of 2 on every arm by live grep, so with the graphify watch probe
at 4 of 5 the M7 kill clause fired and freshness leaves positioning.
Governance at the edit (guards-pilot-8, 32 cells, 2 lost): the served rule
now reaches the agent before the edit on 4 of 8 tasks (2 of 8 in pilot 6,
after ADR-037's raw-guide fallback and the relevance ranking of waves O1–O3),
and fathom's arm ties control on violations (4 and 4) for the first time —
but agents violate rules they were shown, clean successes trail (7 of 11
adjudicable against 10 of 14) and each clean success costs 1.56 times the
tokens, so M1 still fails; the measured cause moved from "the rule was never
served" to "serving the rule does not make a capable model obey it", which is
a different product problem. Change tasks stay a null: reuse-pilot-4 4 of 6
on both arms at 1.7 times the tokens (M5 fails on cost, as in pilot 3), and
the Long Code Arena bug-localization run (40 public tasks, 234 cells) put
fathom's arm at 94 of 117 against control's 93 with a paired interval that
includes zero at twice the tokens — the null is published and BC-15 closes
on it. The receipts held (50 of 50 re-derived, envelope 7.6 % of median
payload); cross-process blast radius held on the blind route table (P 1.00,
R 1.00 after the AST extractor); the review verb named a caller or test gap
the fix later touched in 5 of 42 historical PRs with no false findings, so
M15 stays open; test selection and the code-fact audit did not meet their
bars and stay advisory. On this evidence fathom is level with the best
lookup tools at lower cost, ahead only on verification and receipts, not
yet ahead on governance or on change tasks, and the case for continuing
rests on the verify gate, the falsifiable receipts, and a governance channel
that now reaches the edit and must next be made to bite. This build round
(2026-09-17 evening) shipped enforcement at the edit, answer-first `summary`
envelopes, Kotlin extraction, ingest honesty and the compile budget as
instrument truth, and the next guards run measures them — nothing above has
been re-measured yet.

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
prices; add a third for the `none` arm), **plus a small, one-time,
cheap/haiku-tier spend to compile the obligations layer** for the export
being measured (see "Scratch daemon by default" below; cached thereafter,
so only the FIRST run against a given commit pays it). It is
operator-initiated: never in `validate`, never in a hook, never in CI
(`docs/intent/autonomy.md`: ask before LLM spend).

This section covers the home bench (`task bench-home` /
`scripts/context-bench.sh`). Running a **scenario-bench pilot**
(`cmd/scenariobench` — guards/vs/lca corpora, paid) instead? That runbook
lives in `docs/bench/scenarios.md`, whose "Operating lessons" section
carries the operational rules a pilot round this size actually needs
(worktree/raw-artifact lifetime, the push gate's own Go cache, subject
git isolation, `-rescore-lost`, `-strict-borrow`, monitoring a run) —
read it before the first paid cell, not after losing one.

### Scratch daemon by default (VB-36, O47)

The wired arms ("fathom", "full") talk to a **scratch daemon by default**
now — never the fleet store, and never the live checkout. The subject is
`git archive HEAD` exported to a scratch dir (the same export
`prepare_session_root` gives the sessions themselves as their cwd), a
scratch fathom binary is built FROM THAT EXPORT with `-buildvcs=false`
(an export carries no `.git` for vcs stamping), and a scratch store is
ingested from it — never from the live dev tree. This closes O47: a daemon
built or ingested from the live checkout can serve an answer that cites
the live checkout's own absolute path, a path a live editor may be
changing mid-run.

The export, and the scratch binary/store, live under
`${TMPDIR:-/tmp}/fathom-context-bench/` — OUTSIDE this checkout, a
deliberate DIVERGENCE from AGENTS.md's workspace/ convention (the
`scripts/stranger-check.sh` precedent). A first cut put them under
`workspace/tmp/`, nested inside the checkout; every honest served path
inside a nested export also carries the live checkout's own path as a
string PREFIX, so the path-leak check below (naively implemented) flagged
every answer as a leak. Moving the export out fixes the common case, and
`DetectPathLeak`/`DetectOutOfRootReads` also canonicalize both roots
(`realpath`, so a symlinked temp dir or checkout compares equal) and
exclude anything under the session's own root regardless of layout — so
either arrangement scores correctly. Keyed by the export's own commit
sha, and REUSED across cells, across `--repeat`, and across separate
invocations of this script for the same commit — the obligations-layer
compile below is the expensive part this protects, so a repeated run pays
it once. `--no-scratch-daemon` (or `--daemon-url`, an alias of
`--fathom-url`) opts back into the old fleet-daemon behavior for a
deliberate operator run against a daemon you already run.

**`--env` defaults to `compiled-layer`** (VB-36) — restored as the
default for every wired-arm run, scratch daemon or fleet, so a default
`task bench-home` covers all thirteen home tasks (eight deterministic,
five `requires: [compiled-layer]`), matching the pre-scratch-default
behavior. With the (default) scratch daemon, admitting compiled-layer
tasks also means COMPILING THE LAYER FOR REAL when none is cached yet:
`fathom compile -dry-run=false` against the cheap/haiku-tier model
(`claude-haiku-4-5-...`, `cmd/fathom/compile.go`'s `defaultCompileModel`)
— **this spends a small amount of real LLM money**, once per export,
cached exactly like the binary/store above (a second cell, a `--repeat`,
or a later run against the same export reuses it). `--env raw` opts out:
deterministic tasks only, no compile, no spend — the prior default. The
run's final summary prints the env in effect and whether the compile was
cached, paid just now, failed, or not applicable (`--env raw` /
`--no-scratch-daemon`).

Two safety checks run on every wired-arm cell now, from the stream-json
transcript every cell captures (see "How to read a row" for the columns):
a served tool_result payload naming the live checkout's own absolute
path, EXCLUDING anything under the session's own root (an honest
reference into the export is not a leak), sets `path_leak: true`; a
Bash/Read tool input naming an absolute path outside the session's own
root (`--restricted` confines Read/Glob/Grep to it, but a shell command
naming an absolute path escapes completely — the exact O47 repro) is
counted on `out_of_root_reads` and named in `out_of_root_paths`. Both are
recorded on the cell — never silently — and summed/listed on the ledger
row (`out_of_root_reads`, `flagged_cells`).

Against another repo: `scripts/context-bench.sh --subject-root <dir>
--service <id> --prompt-subject 'the acme codebase'` builds the scratch
fathom binary from THIS tree, ingests the subject into a store under
`${TMPDIR:-/tmp}/fathom-context-bench/` (never the fleet store, never a
post-fix tree), serves it on a free loopback port for the run, and stops
it at exit. `--fathom-url` (or `--daemon-url`) points the fathom arm at a
daemon you already run instead, opting out of the scratch default.

**Clean room (RC-7, vision 28 §2.2).** A smoke test proved the earlier
scratch `CLAUDE_CONFIG_DIR` approach loses the login ("Not logged in"), so
every session instead runs under Claude Code 2.1.270's own isolation:
`--restricted` (ignores the operator's user/project/local settings files,
confines file tools to the working directory) plus `--disable-slash-commands`,
`--settings <path>` naming a settings.json the script renders per arm under
the run dir, and `--tools <list>` naming exactly that arm's own built-in
tools (derived from its allow-list, mcp__ entries stripped). Every arm's
settings.json is `{}` today — the home bench runs the subject hook-free
(unchanged) and, for now, the harness side hook-free too; every call also
passes `--permission-mode default` so an arm's
`--allowedTools`/`--disallowedTools` restriction actually denies a
disallowed tool instead of decorating it. This closes F3/F4 (§1): before it,
every session — control included — inherited the operator's own ~90 slash
commands, plugins, and fathom's own SessionStart primer, so the control arm
was not a control. It runs alongside `prepare_session_root`'s existing
hook-free export of the *subject* tree (`.claude/settings.json` stripped
from the checkout being asked about) —
together, no operator configuration reaches either side of a cell.

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

### Where fathom stands

**20260724T170152Z** — fathom vs grep: success 13/13 vs 11/13, tokens 0.570×.
On 20260724T170152Z, fathom's arm did not run enough trials to decide against grep on success (13 v 11) at 0.570× the tokens; registered criterion H2-1: indeterminate (thinnest cell ran 1 trial(s), below the pre-registered floor of 3).

**20260913T002832Z** — fathom vs grep: success 31/33 vs 28/33, tokens 1.444×, cost 1.465×.
On 20260913T002832Z, fathom's arm beat grep on success (31 v 28) at 1.444× the tokens; registered criterion H2-1: fail (aggregate token ratio 1.444× exceeds the pre-registered cap of 1.000×).

**20260913T002832Z** — fathom vs grep: success 31/33 vs 28/33, tokens 1.444×, cost 1.465×.
On 20260913T002832Z, fathom's arm beat grep on success (31 v 28) at 1.444× the tokens; registered criterion H2-3: fail (aggregate payload_bytes ratio 1.901× exceeds the pre-registered cap of 1.000×).

**20260913T094546Z** — fathom vs grep: success 28/33 vs 28/33, tokens 1.375×, cost 1.539×.
On 20260913T094546Z, fathom's arm matched grep on success (28 v 28) at 1.375× the tokens; registered criterion H2-3: fail (success 28 vs 28 — not strictly greater).

**20260913T154217Z** — full vs grep: success 35/39 vs 34/39, tokens 1.810×, cost 1.604×.
On 20260913T154217Z, fathom's arm beat grep on success (35 v 34) at 1.810× the tokens; registered criterion H2-3: fail (aggregate payload_bytes ratio 1.615× exceeds the pre-registered cap of 1.000×).

**20260914T035806Z** — full vs grep: success 41/47 vs 44/47, tokens 1.808×, cost 1.095×.
On 20260914T035806Z, fathom's arm lost to grep on success (41 v 44) at 1.808× the tokens; registered criterion H2-3: fail (success 41 vs 44 — not strictly greater).


| run | commit | corpus | model | tasks | n | success (t/c) | tokens agg | tokens median | Δsuccess 95% CI | criterion | verdict | why |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 20260724T170152Z | bfb8010 | tasks-home.json | sonnet | 13 | 1 | 13/11 | 0.570× | n/r | n/r | H2-1 | indeterminate | thinnest cell ran 1 trial(s), below the pre-registered floor of 3 |
| 20260913T002832Z | a75e64e | tasks-home.json | sonnet | 11 | 3 | 31/28 | 1.444× | 1.542× | +0.091 [-0.061, +0.242] p=0.042 A12=0.60 | H2-1 | fail | aggregate token ratio 1.444× exceeds the pre-registered cap of 1.000× flaky=5 leaked=2 payload=1.901× |
| 20260913T002832Z | a75e64e | tasks-home.json | sonnet | 11 | 3 | 31/28 | 1.444× | 1.542× | +0.091 [-0.061, +0.242] p=0.042 A12=0.60 | H2-3 | fail | aggregate payload_bytes ratio 1.901× exceeds the pre-registered cap of 1.000× flaky=5 leaked=2 payload=1.901× |
| 20260913T094546Z | 68a5348 | tasks-home.json | sonnet | 11 | 3 | 28/28 | 1.375× | 1.193× | +0.000 [-0.121, +0.152] p=0.240 A12=0.48 | H2-3 | fail | success 28 vs 28 — not strictly greater flaky=6 leaked=2 borrowed=66 payload=2.656× |
| 20260913T154217Z | 47ee05f | tasks-home.json | sonnet | 13 | 3 | 35/34 | 1.810× | 1.750× | +0.026 [-0.103, +0.154] p=0.013 A12=0.54 | H2-3 | fail | aggregate payload_bytes ratio 1.615× exceeds the pre-registered cap of 1.000× flaky=5 borrowed=39 payload=1.615× |
| 20260914T035806Z | 13161ff | tasks-home.json | sonnet | 13 | 3 | 41/44 | 1.808× | 2.972× | -0.067 [-0.190, +0.031] p=0.027 A12=0.45 | H2-3 | fail | success 41 vs 44 — not strictly greater flaky=4 payload=0.864× trusted=93%(n=122) |

<!-- END ledger table -->

### The 2026-09-13 rerun on the wave-1 tools (BP-44)

`run-20260913T094546Z` re-ran only the fathom cells (grep and none borrowed
by key, 78 of 156) after the compact references, the read_source budget,
the implementations field and the stop-when-answered descriptions landed:
successes 28 vs 28 (equal, from 31 vs 28), tokens 1.375× (from 1.444×),
payload **2.66×** (from 1.90×), cost 1.54× — FAIL under H2-3, and worse on
payload. `read_source` is now the arm's first gold source (0.24): a smaller
`references` answer sent the agent to read more source. The lookup tools
are the wrong shape for these questions, exactly the falsifier vision 27
§10 registered; SB-48 (search with a receipt) is the reply, not another
trim. ToolSearch turns: 0.00 — `alwaysLoad` removed the harness tax.

The realistic arm (`full`: fathom plus Read, Grep, Glob and read-only
shell forms, in a hook-free export) ran the same corpus against the same
borrowed grep cells: successes 35 vs 34, tokens 1.81×, payload 1.62×, cost
1.60× — FAIL under H2-3. Its gold sources were Grep 0.23 and Bash 0.18
before any fathom tool (obligations 0.15): given everything, the agent
greps first, reaches for the index second, and pays for both. That is the
cost story of the whole matrix in one row, and it is why SB-48 exists.

### The losing tasks, re-read per call (BP-41..46, 2026-09-25, no spend)

The fathom-arm transcripts of the same rerun, read per call for the three
tasks BP-41 named. It is the one fathom-only run since the compact
`references`, the `read_source` budget and the stop sentences landed
(fathom cells at 3655eaa). Payload against the borrowed grep cells: H1-T06
32.3×, H1-T08 3.53×, H1-T11 3.73×. BP-41's bar, ≤ 1.0 on each task, is
not met, and no targeted run has measured it since: the one targeted row,
`run-20260913T034838Z`, carries no fathom payload bytes.

- `references` is no longer where the bytes go. A 50-row call the agent
  asked for (the default is 25) cost 3.6–3.8 KB, and a free-text query
  (`"workspace/ convention"`, `"shell fathom ingest"`) was refused in about
  200 bytes, where the 2026-09-13 run paid 6 KB for `func (s *`.
- The bytes moved past the answer, and each cell paid for something
  different. On H1-T06, t1 and t3 called `find_symbol` for
  `PutObservations`, and its 492-byte hit named both implementations the
  task asks for. t1 then spent 12.2 KB on `repo_map` and 6.6 KB on
  `query_symbols`; t3 spent 3.6 KB on `references`, 3.4 KB on
  `read_source` and 1.6 KB on `symbol_context`. t2 never looked the
  method up with `find_symbol`: it opened with `references` (3.8 KB) and
  `query_symbols` (6.6 KB). `repo_map` appears in one cell of three. On
  H1-T08 every cell spent 5.9 KB on an `outline` of `docs/usage.md`. On
  H1-T11 every cell read the fleet-index guide through `read_source` from
  line 1: t1 lines 1–120 (9.6 KB) and t2 lines 1–200 (11.2 KB, stopped at
  line 136 by the byte cap) at the default budget; only t3 read the whole
  162-line file, at `max_bytes` 16000 (12.9 KB).
- `read_source`'s 8000-byte budget counted raw text, so default reads
  reached the model over it. t2's 136 lines, 7.9 KB of raw text, arrived as
  11,189 bytes, 10,849 of them the `lines` array; t1's 120 lines arrived as
  9,618 bytes without reaching the cap at all. The per-line
  `{line, text}` envelope and its JSON escaping were left out. Since BP-42
  the budget bounds the `lines` array as sent.

BP-45 re-measures these three tasks on a binary carrying BP-42. BP-46
measures `pack_context` against the bare tools on the same task shapes:
no fathom cell here called it, so there is nothing to read yet. Both need
operator spend.

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

### Beyond Go: the file-resolved tier, hand-verified (2026-09-24, CV-M7)

`docs/bench/readings/l0-ts-py-2026-09-24.md`: real calibration clones (zod
4.4.3 TypeScript, flask 3.1.3 Python), real `scip-typescript`/`scip-python`
runs. A precision/recall read against an independent SCIP truth could not
be computed at first — every `SymbolInformation` in both real indexes
carries an empty `DisplayName` (0 of 23,583 / 0 of 5,551), a live gap in
`internal/scip.ImportIndex`'s name resolution against the only published
releases of both producers, filed as **O115**. What COULD be measured
honestly at that point: a stride-sampled hand spot-check of the
file-resolved tier (SU-11) itself — **26 of 26 correct** (13/13 zod, 13/13
flask), including one case requiring real disambiguation between two
same-named nested functions.

**O115 is now fixed** (same reading, "O115 fixed — CV-M7 re-run" section):
`ImportIndex` maps each SCIP definition against the service's own
extracted symbol table by position instead of trusting `DisplayName`, and
two further mapping bugs the re-run surfaced (a function parameter
collapsing onto its enclosing function; a `local <id>` symbol — document-
scoped per SCIP's own spec — treated as globally unique) are closed too,
both hand-verified against these same real clones. With the scip tier now
genuinely fathom-keyed, the file-resolved tier's precision/recall against
an independent truth is real for the first time: **precision 0.87 TS /
0.95 Python, recall 0.03 TS / 0.12 Python** — high precision, low recall,
exactly what a heuristic import/module-scope resolver (never a type
checker) should read. The scip tier's own pooled numbers (precision 0.65
TS / 0.21 Python, recall 1.00 both — recall is trivial by construction)
remain a weaker, self-referential comparison and are reported for
completeness only; the file-resolved-vs-truth numbers above are this
reading's real answer.

### Refactor-conservation kill criteria (vision 34, ADR-044, 2026-09-24)

`docs/bench/readings/cv-m1-guards-pilot-8-replay-2026-09-24.md`: replaying
guards-pilot-8's i28 cells (both trials — inverted assertions, deleted
helper — as real git ranges against the recorded subject commit) and the
i36 self-check as a reconstructed range both come back exactly as design
§1.2/§1.3 says they should — both i28 trials `deleted` (not merely
`renamed`: the body changed too), both i36 rows `self-declared: skip`.
**CV-M1 passes.**

`docs/bench/readings/cv-m2-guarded-test-commit-replay-2026-09-24.md`: the
last 50 commits on this repo's own history touching a guarded test file —
three days of this very program's own development, the hardest population
to stay quiet on — produce 9 `edited` (informational) findings and **zero**
true loss-kind flags. **CV-M2 passes, 0% against a 10% bar.**

`docs/bench/readings/cv-m4-apidiff-calibration-2026-09-24.md`: 38 of the
requested 50 consecutive release pairs (cobra, cli/cli, prometheus; time
budget, not method, capped the count) against `apidiff`. Precision 0.615,
recall 0.941 — both under the 0.95 bar, precision badly. Three clean
disagreement classes: `interface{}`↔`any` modernization (fathom's textual
signature diff is not type-identity-aware — the dominant false-positive
driver), Go's additive-struct-is-compatible rule (fathom flags any textual
struct diff, apidiff only a narrowing one), and a `type X T` → `type X = T`
alias change (invisible to a textual diff, apidiff's largest incompatible
change in the sample — the sole false negative). **CV-M4 fails and fires
ADR-044's own named revisit trigger** (amendment below).

**Wave 3 (gap-closing): CV-M4 fixed and passes.**
`docs/bench/readings/cv-m4-apidiff-full50-2026-09-24.md`: the full 50
consecutive release pairs the bar asks for (23 cobra, 17 cli/cli, 10
prometheus), after `internal/analysis/inventory/apicompat.go` made the
`api:` delta apply apidiff's own struct/interface/alias compatibility
rules instead of a whole-text diff. Precision 0.960, recall 1.000 — both
clear the 0.95 bar. One residual false positive (a dependency's own
cross-package type alias, out of this fix's scope) is named, not fixed.

`docs/bench/readings/cv-m5-blindness-replay-2026-09-24.md`: seeded copies
of fathom's own two migrations (DuckDB removal ADR-025, vector withdrawal
ADR-041) both flag correctly at the design's 5-statement floor. The vector
migration's genuine range was always clean. The DuckDB migration's genuine
range — untouched, nothing seeded — first produced 24 `body_resurrected`
findings: real code that `DuckStore` and `SQLiteStore` had independently,
convergently implemented for years, mistaken for a copy the instant one
twin's file is deleted, because the check never asked whether the MATCHING
head function was itself new in the diff. **Fixed and re-run (2026-09-24
update): `conserve.Resurrected` now requires a matching head function to be
itself new-or-changed in the range, and CV-M5 passes 2/2** — 0 findings on
both genuine ranges, both seeded copies still flag.

## The strengths matrix (vision 27, ADR-033)

Every claim fathom makes, as a row: what it is compared against, the oracle
fathom does not own, and the prediction written before spend. Ledger
pointers fill in as rows run; a row that refutes its own prediction is a
product finding and stays on the table.

| Row | Claim | Versus | Oracle | Predicted win / no-effect | Reading |
|---|---|---|---|---|---|
| M1 | a rule served at the edit prevents the violation | the same rule as CLAUDE.md prose | the invariant's own test or lint | far-from-edit rules / short-file rules | `scenarios-ledger.jsonl` run `guards-pilot-3` (2026-09-13, 8 tasks × 2 arms × n=2, $31.62): complied 6/16 on BOTH arms; violations wired 0, control 1; wired arm 0.85× the control's tokens and 0.92× its cost; M1-1 **pass** on the registered bar (success ≥, cost ≤) — but the predicted wins on i11 (tool without its doc) and i31 (artifact outside workspace/) did not appear: 0/2 vs 0/2 and 1/2 vs 0/2. The no-effect condition dominated: every rule lives in a CLAUDE.md both arms read, and a careful agent declines the rule-breaking asks either way (5–6 of 16 cells per arm declined). Nothing here is a cost story for obligations; the correctness story needs rules that are NOT in the instruction files. Run `guards-adr-pilot-1` (2026-09-13, 8 ADR-only rules × 2 arms × n=2, $31.45) tried exactly that: **zero rule violations on either arm again** across 30 cells that edited code; wired arm 0.77× the control's tokens and 0.86× its cost; M1-2 passes its bar but the success half is **not measurable** — the change-present detectors diffed one named file against the index and read a change made in a sibling file, or committed, as absent (repaired for the next run, SB-54). Two corpora, 64 cells, no violation: on this repo a careful agent does not break documented rules whether or not the rule is served at the edit |
| M2 | the verify gate lowers false "done" | a self-ticked checklist | the PR's tests + a coverage profile | cross-package, governance-bound / single-file fixes | **false-done not measurable yet** (no adjudicable row on either arm, SB-52/SB-53); run `public-pilot-2` (cli/cli, 4 tasks × 2 arms × n=2 × 2 sessions, $20.61): upstream tests passed wired **2/8**, control **3/8**; M2-1 **fail** on the bar. Both arms passed the cross-package task (10043, 2/2 each) and failed the signature change (10009) and both bug fixes but one — the predicted category shape is real, the predicted arm effect is not (see the section below for the false-done reading) |
| M3 | index-grounded plans cover the real change | plans from grep | the gold patch's functions | multi-package / one-function | `public-pilot-2`: plan coverage of the gold functions was sparse and equal — 0–2 of 1–6 named per cell on both arms, with 0–5 spurious rows; no arm effect at n=2 |
| M4 | continuity resumes without re-discovery | a notes file | tokens to first edit, duplicated edits | multi-session / one session | `public-pilot-2`: tokens before the first edit of session 2 were HIGHER on the wired arm on every task (e.g. 10016: 1.2M/3.0M vs 383k/493k; 10009: 831k/1.27M vs 287k/226k) — the prediction is refuted: the primer, the obligations and the index invitation cost more re-discovery than a notes file; duplicated edits 0 on both |
| M5 | exploration once, reused by K workers | each worker explores | bytes re-read vs the planner | K ≥ 3 / K = 1 | run `reuse-pilot-1` (cli/cli 10043, K=3 and K=1, n=2, $5.93): at K=3 both arms passed 2/2 and the workers' re-read overlap was **0.34 / 0.13 wired vs 1.00 / 0.61 control** — the predicted win, as measured; total tokens were still higher wired (1.9M/1.4M vs 1.7M/0.8M). At K=1 both arms failed 2/2 and overlap read 1.0/0.29 vs 1.0/1.0 — the registered no-effect cell held on success, not on overlap |
| M6 | the index is right | graphify, Serena, gopls | `go/types` | resolved callers / single-language references | `competitors-ledger.jsonl` 2026-09-13: callers **fathom 1.00/1.00**, graphify 0.95/0.93; references gopls 1.00/0.96, Serena 1.00/0.79, fathom 0.33/1.00 as text occurrences and **1.00/0.997 in `exact` mode** (SB-45, same day). Paid lookups (`vs-pilot-1`, 5 valid tasks × 5 arms × n=2, every arm with Read/Grep/Bash): fathom 8/10, graphify 8/10, Serena 8/10, grep 8/10, gopls 7/10 — indistinguishable on accuracy; grep cost $1.82 against $4.52–5.03 for the index arms. Every arm failed the verify-reachability task with grep available; the exclusive fathom arm passes it 3/3 |
| M7 | the answer reflects HEAD | a static artifact | truth at HEAD | changed code / unchanged code | **not measurable in `vs-pilot-1`**: the task's setup command wrote a truncated file (a quoting defect in the corpus), so every arm truthfully answered that the function did not exist; the setup is fixed and the task re-runs as SB-51 |
| M8 | fleet blast radius | grep over every repo | consumers' builds and tests | ≥ 3 consumers / one | registered, after the pilots |
| M9 | breadth beyond Go | the same rows on TS and Python | the language's type checker | none claimed | registered, after the pilots |

The free rows (M6, M7's artifact half) run in `task validate` on the in-tree
fixture and on demand against any repo with `task bench-compare-edges`;
`docs/bench/competitors.md` names each tool's version and the exact
commands. The paid rows are one driver, `cmd/scenariobench`, with arms as
data — `docs/bench/scenarios.md` is the runbook a stranger repeats them from.

### The guards pilot, read (M1, 2026-09-13)

**Voided 2026-09-13 (O41):** this reading's cells ran with no MCP tools
connected on the wired arm — it measured prompt text, not fathom's surface.
Read what follows as history, not proof; the rerun is wave 5
(`docs/plans/receipts.md`).

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

### The change bench and the readings it carries (M2–M5, 2026-09-13)

**Voided 2026-09-13 (O41):** `public-pilot-2`, `reuse-pilot-1` and
`vs-pilot-1`'s cells all ran with no MCP tools connected on the wired arm —
they measured prompt text, not fathom's surface. Read what follows as
history, not proof; the reruns are wave 5 (`docs/plans/receipts.md`).

`public-pilot-2` ran four cli/cli changes (two bug fixes, one cross-package
feature, one signature change) on the wired and stripped arms, two trials,
two sessions of forty turns each, with the hidden upstream test patch
applied over the agent's tree after the test files it names are reset to
base, and pass-to-pass calibrated per environment. The wired arm passed 2 of
8, the control 3 of 8; both passed the cross-package task twice and failed
the signature change twice. Tokens per cell ran 1.2M–4.2M on both arms.
The SDD readings on every cell: plan coverage of the gold functions was
sparse on both arms; false-done, re-scored with rows that carry no symbol
contract marked not-adjudicable (SB-52), is **not measurable on this run**:
0 adjudicable rows on either arm (control 53 rows, wired 35, all plain
checklist lines in NOTES.md). The wired arm never wrote a plan-grammar row
the gate could check, so M2 has no reading yet; its plan-first instruction
must name the grammar and the verify verb (SB-53) before the next run. Resumption refuted M4's prediction outright: the wired
arm spent more tokens before its first edit of the second session on every
task. `reuse-pilot-1` confirmed M5's overlap prediction at K=3 (workers on
the wired arm re-read 13–34% of what the planner read, the control's
61–100%) without a token win.

The honest sum so far: the index is right (M6), the wired arm never
violates a rule (M1) and its workers re-explore less (M5), and none of that
has yet turned into fewer tokens or more passing changes at this sample
size. The rows that predicted a token win (M1's cost, M4) are refuted as
run; the rows that predicted an accuracy win are equal or unmeasured.

## Scenario runs

The scenario-run ledger table and its prose moved to [scenario-runs.md](scenario-runs.md) (rendered by `scenariobench ledger`).

## Next round (2026-09-19, not run)

SU-24: every corpus below passes `scenariobench plan` (`TestAllCorporaPlan`,
internal/scenariobench) — one cell per (task, arm, trial), keyed, no error — and
none has run this round. Cost = planned cells x the ledger's median $/cell for the
nearest PRIOR corpus with real data (grouped by `criterion_id`); "no prior" falls
back to `lookups-vs`, the nearest lookup-shaped corpus that has one.

| Order | Corpus | Cells (tasks×arms×trials) | Nearest prior ($/cell, tokens/cell) | Est. cost |
|---|---|---|---|---|
| 1 | guards-fathom.json | 32 (8×2×2) | own history, M1-1: $0.7594, 1,603,632 tok | ~$24 |
| 2 | findings-loop.json | 30 (5×2×3) | no prior: guards-fathom M1-1, $0.7594, 1,603,632 tok | ~$23 |
| 3 | lookups-vs.json | 140 (14×5×2) | own history, M6-1: $0.1091, 159,310 tok | ~$15 |
| 4 | lookups-ts.json | 24 (6×2×2) | no prior: estimate from lookups-vs M6-1, $0.1091, 159,310 tok | ~$3 |
| 5 | lookups-py.json | 24 (6×2×2) | no prior: estimate from lookups-vs M6-1, $0.1091, 159,310 tok | ~$3 |
| 6 | LCA (lca-bugloc.json) | 240 (40×2×3) | own history, lca-bug-localization-m12-m16: $0.0795, 166,171 tok | ~$19 |

**Total: ~$87 across 490 cells.** Re-derive: group `docs/bench/scenarios-ledger.jsonl`
by `criterion_id`, take median(`readings.total_tokens`/`trials`) and median(`cost_usd`/`trials`).
