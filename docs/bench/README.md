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

As of 2026-09-14 (O53): the first clean-room lookup pilot with every tool
present, vs-pilot-3, is filed in full (70 rows, nothing lost) and then read
transcript by transcript before it was believed. The ledger shows fathom's
arm 26 of 28 at $1.57 against gopls 24 ($2.83), grep 23 ($2.67), graphify
22 ($3.11) and serena 21 ($3.08). The second reading found the instrument
decided most of that gap: the competitor arms' worktrees had lost the
repository's own CLAUDE.md, the answer key sat inside every worktree, the
grep baseline could reach the live daemon, the oracle was a substring
check, and the competitor stanzas told the model not to use their tools.
After the conservative correction fathom, gopls and grep tie at 88
percent; what survives is roughly half the cost per pass and one clean
mechanism win, `verify_plan` on the verify-gate task. The earlier
strengths-matrix rows (O41) remain voided. No lookup claim is proven until
vs-pilot-4 runs on the repaired instrument (waves N1-N3); the scenario
pilots on the guards and reuse corpora, whose control arms were never
altered, stand as measured. reuse-pilot-3 (four change tasks on a public
Go CLI, 16 cells, nothing lost) is a second null: fathom's arm 2 of 8
against control 2 of 8 at 1.7 times the tokens, M5-1 failing on cost; its
second reading (O55) found no fathom tool changing an outcome, the gate
bypassed by a worker writing `skip_test` into its own row (O56), and the
subject clone carrying the merged fix in its history. On this evidence
fathom does not help a capable model make a change on a repository it has
never seen, and costs more; the case for the tools rests on the verify
gate, on governance that bites, and on the answers a type-checked index
gives that grep cannot, not on lookup or change speed. guards-pilot-5
(eight governance-bound change tasks on this repository, 32 cells) is the
third null on the ledger, 8 of 16 on both arms at level cost; its second
reading (O57) found one wired pass written by an orphaned agent, twelve
trials that scored a correct refusal as a failure, a wired failure caused
by the arm's own stanza refresh, and served obligations that never named
the governing rule before the edit in any wired trial. On adjudicable
trials fathom's arm violated a rule once in nine and control three times
in eleven, at 1.4 times the tokens per clean success. Governance at the
edit has not yet been measured on a fair instrument; waves N6 and N7
repair the driver and the product before the next run.

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
deliberate DIVERGENCE from CLAUDE.md's workspace/ convention (the
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

The change bench (vision 27, ADR-033): `cmd/scenariobench` over a corpus of
scenarios (`testdata/scenariobench/*.json`) — a change to a pinned subject,
run by an agent across sessions under a turn budget, in arms that differ
only in data, scored by oracles fathom does not own
(`internal/scenariobench`, `docs/plans/strengths-bench.md`). Each row below
is one (run, task, arm); the full per-cell data lives in
`docs/bench/scenarios-ledger.jsonl`, appended by `scenariobench ledger` and
never hand-edited.

<!-- BEGIN scenario ledger (task bench-scenario-ledger) -->

### Where fathom stands

**guards-adr-pilot-1**: every row is voided — nothing sound to measure.
Voided: 16 row(s) carry `instrument_defect` and are excluded from the numbers above — guards-adr-006-raw-query-bypasses-concern/control, guards-adr-006-raw-query-bypasses-concern/full, guards-adr-008-fast-query-skips-staleness/control, guards-adr-008-fast-query-skips-staleness/full, guards-adr-014-harden-surface-hard-gate/control, guards-adr-014-harden-surface-hard-gate/full, guards-adr-017-cache-absolute-source-path/control, guards-adr-017-cache-absolute-source-path/full, guards-adr-018-persist-lease-status/control, guards-adr-018-persist-lease-status/full, guards-adr-027-raw-read-invariants/control, guards-adr-027-raw-read-invariants/full, guards-adr-028-trust-claimed-tier/control, guards-adr-028-trust-claimed-tier/full, guards-adr-032-trim-cell-key-fields/control, guards-adr-032-trim-cell-key-fields/full.

**guards-pilot-3**: every row is voided — nothing sound to measure.
Voided: 16 row(s) carry `instrument_defect` and are excluded from the numbers above — guards-i1-packagedeps-sorted/control, guards-i1-packagedeps-sorted/full, guards-i11-mcp-tool-doc/control, guards-i11-mcp-tool-doc/full, guards-i16-raw-cp-deploy/control, guards-i16-raw-cp-deploy/full, guards-i19-org-agnostic-fixture/control, guards-i19-org-agnostic-fixture/full, guards-i2-store-purego/control, guards-i2-store-purego/full, guards-i28-duckdb-path/control, guards-i28-duckdb-path/full, guards-i31-workspace-artifact/control, guards-i31-workspace-artifact/full, guards-i36-usecase-maintained-line/control, guards-i36-usecase-maintained-line/full.

**guards-pilot-4** (criterion names full/control, not among this run's arms):
control 3/16 (surface calls 0).
Voided: 8 row(s) carry `instrument_defect` and are excluded from the numbers above — guards-i1-packagedeps-sorted/full, guards-i11-mcp-tool-doc/full, guards-i16-raw-cp-deploy/full, guards-i19-org-agnostic-fixture/full, guards-i2-store-purego/full, guards-i28-duckdb-path/full, guards-i31-workspace-artifact/full, guards-i36-usecase-maintained-line/full.

**guards-pilot-5** — full vs control: success 7/8 vs 6/10, tokens 0.756×, cost 0.751×.
Surface calls: control=0, full=45.
On guards-pilot-5, fathom's arm beat control on success (7 v 6) at 0.756× the tokens; the tools were called 45 time(s). Registered criterion M1-1: pass (treatment met the registered bar).
gold source: none earned 7 of 7 passes.
Voided: 7 row(s) carry `instrument_defect` and are excluded from the numbers above — guards-i16-raw-cp-deploy/control, guards-i16-raw-cp-deploy/full, guards-i2-store-purego/control, guards-i2-store-purego/full, guards-i28-duckdb-path/control, guards-i28-duckdb-path/full, guards-i31-workspace-artifact/full.

**public-pilot-2**: every row is voided — nothing sound to measure.
Voided: 8 row(s) carry `instrument_defect` and are excluded from the numbers above — SB-cli__cli-10009/control, SB-cli__cli-10009/full, SB-cli__cli-10016/control, SB-cli__cli-10016/full, SB-cli__cli-10043/control, SB-cli__cli-10043/full, SB-cli__cli-10072/control, SB-cli__cli-10072/full.

**reuse-pilot-1**: every row is voided — nothing sound to measure.
Voided: 4 row(s) carry `instrument_defect` and are excluded from the numbers above — RE-SB-cli__cli-10043-K1/control, RE-SB-cli__cli-10043-K1/full, RE-SB-cli__cli-10043/control, RE-SB-cli__cli-10043/full.

**reuse-pilot-2** (criterion names full/control, not among this run's arms):
control 3/8 (surface calls 0).
Voided: 4 row(s) carry `instrument_defect` and are excluded from the numbers above — RE-SB-cli__cli-10043-K1/full, RE-SB-cli__cli-10043/full, RE-SB-cli__cli-10048/full, RE-SB-cli__cli-10124/full.

**reuse-pilot-3** — full vs control: success 2/8 vs 2/6, tokens 2.696×, cost 2.319×.
Surface calls: control=0, full=29.
On reuse-pilot-3, fathom's arm lost to control on success (2 v 2) at 2.696× the tokens; the tools were called 29 time(s). Registered criterion M5-1: fail (treatment success rate 0.250 is below control's 0.333).
gold source: Bash earned 2 of 2 passes.
Voided: 1 row(s) carry `instrument_defect` and are excluded from the numbers above — RE-SB-cli__cli-10124/control.

**vs-pilot-1**: every row is voided — nothing sound to measure.
Voided: 30 row(s) carry `instrument_defect` and are excluded from the numbers above — FRESH-M7-01/full, FRESH-M7-01/gopls, FRESH-M7-01/graphify, FRESH-M7-01/grep, FRESH-M7-01/serena, H1-T01/full, H1-T01/gopls, H1-T01/graphify, H1-T01/grep, H1-T01/serena, H1-T03/full, H1-T03/gopls, H1-T03/graphify, H1-T03/grep, H1-T03/serena, H1-T06/full, H1-T06/gopls, H1-T06/graphify, H1-T06/grep, H1-T06/serena, H1-T07/full, H1-T07/gopls, H1-T07/graphify, H1-T07/grep, H1-T07/serena, H1-T10/full, H1-T10/gopls, H1-T10/graphify, H1-T10/grep, H1-T10/serena.

**vs-pilot-2**: every row is voided — nothing sound to measure.
Voided: 70 row(s) carry `instrument_defect` and are excluded from the numbers above — FRESH-M7-01/full, FRESH-M7-01/gopls, FRESH-M7-01/graphify, FRESH-M7-01/grep, FRESH-M7-01/serena, H1-T01/full, H1-T01/gopls, H1-T01/graphify, H1-T01/grep, H1-T01/serena, H1-T02/full, H1-T02/gopls, H1-T02/graphify, H1-T02/grep, H1-T02/serena, H1-T03/full, H1-T03/gopls, H1-T03/graphify, H1-T03/grep, H1-T03/serena, H1-T04/full, H1-T04/gopls, H1-T04/graphify, H1-T04/grep, H1-T04/serena, H1-T05/full, H1-T05/gopls, H1-T05/graphify, H1-T05/grep, H1-T05/serena, H1-T06/full, H1-T06/gopls, H1-T06/graphify, H1-T06/grep, H1-T06/serena, H1-T07/full, H1-T07/gopls, H1-T07/graphify, H1-T07/grep, H1-T07/serena, H1-T08/full, H1-T08/gopls, H1-T08/graphify, H1-T08/grep, H1-T08/serena, H1-T09/full, H1-T09/gopls, H1-T09/graphify, H1-T09/grep, H1-T09/serena, H1-T10/full, H1-T10/gopls, H1-T10/graphify, H1-T10/grep, H1-T10/serena, H1-T11/full, H1-T11/gopls, H1-T11/graphify, H1-T11/grep, H1-T11/serena, H1-T12/full, H1-T12/gopls, H1-T12/graphify, H1-T12/grep, H1-T12/serena, H1-T13/full, H1-T13/gopls, H1-T13/graphify, H1-T13/grep, H1-T13/serena.

**vs-pilot-3** (no registered criterion for this corpus):
full 24/26 (surface calls 54); gopls 21/22 (surface calls 0); graphify 22/26 (surface calls 1); grep 23/28 (surface calls 0); serena 20/24 (surface calls 1).
Voided: 7 row(s) carry `instrument_defect` and are excluded from the numbers above — H1-T05/full, H1-T08/gopls, H1-T08/graphify, H1-T08/serena, H1-T12/gopls, H1-T12/serena, H1-T13/gopls.


| run | commit | task | category | arm | model | trials | successes | prediction | surface calls | hooks fired | rule served | violation | overlap ratio | facts reused | resumption | false done | plan coverage | cost | notes |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| guards-pilot-3 | 5b5f5ea | guards-i1-packagedeps-sorted | governance_bound | control | sonnet | 2 | 2 | no-effect: an unsorted map-range read like ordinary idiomatic Go, and nothing in a generic notes file calls that pattern out before the change is claimed done | 0 | – | – | – | – | – | – | – | – | 0.8477 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-pilot-3 | 5b5f5ea | guards-i1-packagedeps-sorted | governance_bound | full | sonnet | 2 | 2 | win: internal/CLAUDE.md's 'sort every output slice; map iteration order must never reach a result' obligation is bound to edits inside internal/analysis, so a new unsorted field is exactly the class of edit it names before TestDeterminism ever runs | 0 | – | – | – | – | – | – | – | – | 0.8961 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-pilot-3 | 5b5f5ea | guards-i11-mcp-tool-doc | governance_bound | control | sonnet | 2 | 0 | no-effect: shipping a new tool and forgetting the doc update is the default outcome of a task phrased purely as 'register it and ship it', with no per-file reminder to catch the omission | 0 | – | – | – | – | – | – | – | – | 4.1108 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-pilot-3 | 5b5f5ea | guards-i11-mcp-tool-doc | governance_bound | full | sonnet | 2 | 0 | win: internal/CLAUDE.md's 'Adding things > MCP tool' section is an obligation bound to internal/mcp edits naming docs/mcp-tools.md directly, which a brand-new registerTool call triggers immediately | 0 | – | – | – | – | – | – | – | – | 3.9624 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-pilot-3 | 5b5f5ea | guards-i16-raw-cp-deploy | governance_bound | control | sonnet | 2 | 0 | no-effect: the control arm reads that exact same short, prominent table directly and needs no served obligation to see the rule it is about to contradict | 0 | – | – | – | – | – | – | – | – | 0.3503 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-pilot-3 | 5b5f5ea | guards-i16-raw-cp-deploy | governance_bound | full | sonnet | 2 | 0 | no-effect: the raw-cp prohibition already sits in CLAUDE.md's Non-negotiables table within the file's first 20 lines, over a real incident (O25) -- the wired arm's obligation adds nothing a short, prominent file did not already surface to a careful reader | 0 | – | – | – | – | – | – | – | – | 0.3715 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-pilot-3 | 5b5f5ea | guards-i19-org-agnostic-fixture | governance_bound | control | sonnet | 2 | 2 | no-effect: a prose reminder sitting in a notes file, unread mid-edit, does not stop a realistic-sounding example from feeling like the natural thing to type into a new golden fixture | 0 | – | – | – | – | – | – | – | – | 3.8953 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-pilot-3 | 5b5f5ea | guards-i19-org-agnostic-fixture | governance_bound | full | sonnet | 2 | 1 | win: the fixture is new, small, and directly named in the diff, so the obligation reaches exactly the file being written before a plausible-sounding real hostname or org name lands in it | 0 | – | – | – | – | – | – | – | – | 3.5774 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-pilot-3 | 5b5f5ea | guards-i2-store-purego | governance_bound | control | sonnet | 2 | 0 | no-effect: a plain notes-file instruction carries no per-file trigger equivalent to an obligation, so a plausible-sounding performance fix has nothing stopping it from reaching for cgo | 0 | – | – | – | – | – | – | – | – | 0.6507 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-pilot-3 | 5b5f5ea | guards-i2-store-purego | governance_bound | full | sonnet | 2 | 0 | win: obligations fire on any edit under internal/store naming ADR-025's purity rule, well before a CGO import or build tag lands in that package | 0 | – | – | – | – | – | – | – | – | 0.4761 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-pilot-3 | 5b5f5ea | guards-i28-duckdb-path | governance_bound | control | sonnet | 2 | 0 | no-effect: nothing in a generic notes file counters the 'be lenient to the user' instinct that a hang-report prompt invites | 0 | – | – | – | – | – | – | – | – | 0.9307 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-pilot-3 | 5b5f5ea | guards-i28-duckdb-path | governance_bound | full | sonnet | 2 | 0 | win: the naive 'just make it work' fix directly contradicts ADR-025's loud-refusal rule, and an obligation bound to internal/store's backend-selection file serves that rule on the exact edit a user-facing hang report tempts an agent toward | 0 | – | – | – | – | – | – | – | – | 0.8231 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-pilot-3 | 5b5f5ea | guards-i31-workspace-artifact | governance_bound | control | sonnet | 2 | 0 | no-effect: a generic notes-file instruction carries no per-file trigger naming defaultStorePath's contract, so the control has no reason to route a brand-new output path through the shared helper instead of the cwd | 0 | – | – | – | – | – | – | – | – | 4.1660 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-pilot-3 | 5b5f5ea | guards-i31-workspace-artifact | governance_bound | full | sonnet | 2 | 1 | win: the workspace/ default-path convention is bound to edits under cmd/fathom's flag/path-default files, so an obligation serves the rule exactly where a new debug-dump default path is being written, well before the naive 'just write it next to cwd' fix lands | 0 | – | – | – | – | – | – | – | – | 3.8664 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-pilot-3 | 5b5f5ea | guards-i36-usecase-maintained-line | governance_bound | control | sonnet | 2 | 2 | no-effect: a prompt that explicitly frames the contract line as optional boilerplate, with no obligation naming it, leaves the control arm with every reason to leave it out | 0 | – | – | – | – | – | – | – | – | 1.5099 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-pilot-3 | 5b5f5ea | guards-i36-usecase-maintained-line | governance_bound | full | sonnet | 2 | 2 | win: docs/CLAUDE.md's use-case upkeep contract is a doc-shaped obligation bound to edits under docs/use-cases/, which fires on a brand-new file there even though the prompt itself invites skipping the contract line | 0 | – | – | – | – | – | – | – | – | 1.1846 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| public-pilot-2 | 4a5fc8b | SB-cli__cli-10009 | signature_change | control | sonnet | 2 | 0 | loses relative to full on this task | 0 | – | – | – | – | – | – | – | – | 3.7837 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| public-pilot-2 | 4a5fc8b | SB-cli__cli-10009 | signature_change | full | sonnet | 2 | 0 | wins — vision 27 M2/M3: a cross-package, signature, or governance-bound change is where index-grounded plans and the verify gate diverge from a plain checklist | 0 | – | – | – | – | – | – | – | – | 3.4311 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| public-pilot-2 | 4a5fc8b | SB-cli__cli-10016 | bug_fix | control | sonnet | 2 | 1 | no effect — same | 0 | – | – | – | – | – | – | – | – | 2.4468 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| public-pilot-2 | 4a5fc8b | SB-cli__cli-10016 | bug_fix | full | sonnet | 2 | 0 | no effect — vision 27 M2: a single-file mechanical change is where a plain checklist already succeeds | 0 | – | – | – | – | – | – | – | – | 3.3017 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| public-pilot-2 | 4a5fc8b | SB-cli__cli-10043 | cross_package_feature | control | sonnet | 2 | 2 | loses relative to full on this task | 0 | – | – | – | – | – | – | – | – | 1.3888 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| public-pilot-2 | 4a5fc8b | SB-cli__cli-10043 | cross_package_feature | full | sonnet | 2 | 2 | wins — vision 27 M2/M3: a cross-package, signature, or governance-bound change is where index-grounded plans and the verify gate diverge from a plain checklist | 0 | – | – | – | – | – | – | – | – | 1.3333 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| public-pilot-2 | 4a5fc8b | SB-cli__cli-10072 | bug_fix | control | sonnet | 2 | 0 | no effect — same | 0 | – | – | – | – | – | – | – | – | 2.2704 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| public-pilot-2 | 4a5fc8b | SB-cli__cli-10072 | bug_fix | full | sonnet | 2 | 0 | no effect — vision 27 M2: a single-file mechanical change is where a plain checklist already succeeds | 0 | – | – | – | – | – | – | – | – | 2.6545 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| reuse-pilot-1 | 4a5fc8b | RE-SB-cli__cli-10043 | cross_package_feature | control | sonnet | 2 | 2 | planner explores with grep/Read and writes prose rows; each worker re-explores the same files — overlap high | 0 | – | – | – | – | – | – | – | – | 2.3758 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| reuse-pilot-1 | 4a5fc8b | RE-SB-cli__cli-10043 | cross_package_feature | full | sonnet | 2 | 2 | planner explores once through repo_map/pack_context/blast radius; workers start from the row's ctx pack — overlap low, workers reach first edit sooner | 0 | – | – | – | – | – | – | – | – | 1.8541 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| reuse-pilot-1 | 4a5fc8b | RE-SB-cli__cli-10043-K1 | cross_package_feature | control | sonnet | 2 | 0 | no effect for the same reason; this variant is the matrix's no-effect cell | 0 | – | – | – | – | – | – | – | – | 0.8295 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| reuse-pilot-1 | 4a5fc8b | RE-SB-cli__cli-10043-K1 | cross_package_feature | full | sonnet | 2 | 0 | no effect: with one worker there is nothing to reuse across workers — the overlap reading is defined only against the planner, and vision 27 M5 predicts the pack advantage vanishes at K = 1 | 0 | – | – | – | – | – | – | – | – | 0.8644 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | FRESH-M7-01 | freshness | full | sonnet | 2 | 0 | wins: the wired arm's worktree is (re-)ingested after this setup step lands, so the index reflects the new file and function at answer time. | 0 | – | – | – | – | – | – | – | – | 0.7525 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | FRESH-M7-01 | freshness | gopls | sonnet | 2 | 0 | predicted ties fathom: same live-query mechanism as serena — no precomputed artifact to go stale. | 0 | – | – | – | – | – | – | – | – | 0.2948 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | FRESH-M7-01 | freshness | graphify | sonnet | 2 | 0 | predicted loses: graph.json is a precomputed artifact built during this arm's own setup, BEFORE this task's Subject.Setup writes the new file; without an --update re-run (not part of this arm's setup), it has no node for FreshnessMarkerValue at all. | 0 | – | – | – | – | – | – | – | – | 0.6657 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | FRESH-M7-01 | freshness | grep | sonnet | 2 | 0 | predicted ties fathom: a direct file read always reflects the file's current on-disk content, so grep is never disadvantaged by staleness. | 0 | – | – | – | – | – | – | – | – | 0.2546 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | FRESH-M7-01 | freshness | serena | sonnet | 2 | 1 | predicted ties fathom: gopls-backed queries re-analyze the file on disk on every call, so a query issued after the setup step sees the new function. | 0 | – | – | – | – | – | – | – | – | 0.3970 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T01 | obligations | full | sonnet | 2 | 2 | wins: answered directly by the obligations tool's compiled rule for this file, which cites internal/CLAUDE.md by line — no competitor tool reads governance docs at all. | 0 | – | – | – | – | – | – | – | – | 0.7529 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T01 | obligations | gopls | sonnet | 2 | 2 | ties grep, below fathom: go_symbol_references answers code structure, not documented conventions. | 0 | – | – | – | – | – | – | – | – | 0.2230 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T01 | obligations | graphify | sonnet | 2 | 2 | ties grep, below fathom: query_graph/get_node answer code structure, not documented conventions. | 0 | – | – | – | – | – | – | – | – | 0.8768 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T01 | obligations | grep | sonnet | 2 | 2 | below fathom: the rule is on disk in internal/CLAUDE.md and grep-findable given the right file, but nothing points the agent at it directly the way obligations() does. | 0 | – | – | – | – | – | – | – | – | 0.3159 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T01 | obligations | serena | sonnet | 2 | 2 | ties grep, below fathom: find_symbol/find_referencing_symbols answer code structure, not documented conventions. | 0 | – | – | – | – | – | – | – | – | 0.6423 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T03 | verify | full | sonnet | 2 | 0 | wins: references/find_callers on run() (or verify_plan's own reachability check) surfaces a qualifying Test* function directly. | 0 | – | – | – | – | – | – | – | – | 0.7525 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T03 | verify | gopls | sonnet | 2 | 0 | ties grep, below fathom: go_symbol_references on run() finds the same candidates — no edge over reading the file. | 0 | – | – | – | – | – | – | – | – | 0.1775 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T03 | verify | graphify | sonnet | 2 | 0 | ties grep, below fathom: its calls-relation graph could in principle surface a caller of run(), but this arm's tool list here is limited to references-style queries the same way serena/gopls are used, so no advantage over reading verify_test.go directly is predicted. | 0 | – | – | – | – | – | – | – | – | 0.7072 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T03 | verify | grep | sonnet | 2 | 0 | below fathom only in directness, not in reachability: `grep -n 'run("verify'` finds a correct answer readily; predicted a near-tie with the competitor arms and slightly below fathom. | 0 | – | – | – | – | – | – | – | – | 0.1690 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T03 | verify | serena | sonnet | 2 | 0 | ties grep, below fathom: find_referencing_symbols on run() finds the same 16 candidates a direct read does — no edge over reading the file. | 0 | – | – | – | – | – | – | – | – | 0.8367 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T06 | blast_radius | full | sonnet | 2 | 2 | wins: find_symbol's implementations field (SB-5) lists both concrete types directly from the typed pass. | 0 | – | – | – | – | – | – | – | – | 0.8057 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T06 | blast_radius | gopls | sonnet | 2 | 2 | plausible tie with fathom: go-to-implementation is native LSP functionality over a real type-checker (no find_implementations-equivalent is in this bench's gopls tool list, so this is optimistic — recorded as a prediction to test, not assumed). | 0 | – | – | – | – | – | – | – | – | 0.4301 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T06 | blast_radius | graphify | sonnet | 2 | 2 | predicted loses: its tree-sitter extraction has no type-checker, so implicit structural interface satisfaction (Go has no 'implements' keyword) is outside what its calls/references/method relations can encode reliably. | 0 | – | – | – | – | – | – | – | – | 0.7582 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T06 | blast_radius | grep | sonnet | 2 | 2 | below fathom/serena: a textual match on the method name+signature risks both false positives (an unrelated same-named method elsewhere) and false negatives (a differently formatted signature), with no type-checking to disambiguate. | 0 | – | – | – | – | – | – | – | – | 0.1762 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T06 | blast_radius | serena | sonnet | 2 | 2 | plausible tie with fathom: find_implementations is backed by gopls's real type-checker, which resolves structural interface satisfaction directly — an LSP's forte. | 0 | – | – | – | – | – | – | – | – | 0.8938 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T07 | claim_provenance | full | sonnet | 2 | 2 | wins: the obligations tool's compiled rule for this file names the exact citation directly. | 0 | – | – | – | – | – | – | – | – | 1.5422 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T07 | claim_provenance | gopls | sonnet | 2 | 2 | ties grep, below fathom: no tool surface for decision-record provenance. | 0 | – | – | – | – | – | – | – | – | 0.4929 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T07 | claim_provenance | graphify | sonnet | 2 | 2 | ties grep, below fathom: no tool surface for decision-record provenance. | 0 | – | – | – | – | – | – | – | – | 1.2275 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T07 | claim_provenance | grep | sonnet | 2 | 2 | below fathom: ADR.md is long; finding the exact line without a citation to start from is a real search cost every non-fathom arm shares equally. | 0 | – | – | – | – | – | – | – | – | 0.7203 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T07 | claim_provenance | serena | sonnet | 2 | 2 | ties grep, below fathom: no tool surface for decision-record provenance. | 0 | – | – | – | – | – | – | – | – | 1.2725 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T10 | concern_search | full | sonnet | 2 | 2 | wins in directness: query_symbols' concern ranking surfaces fitToBudget among its top candidates. | 0 | – | – | – | – | – | – | – | – | 0.4205 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T10 | concern_search | gopls | sonnet | 2 | 1 | ties grep, below fathom: no concern-scoped ranking. | 0 | – | – | – | – | – | – | – | – | 0.1881 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T10 | concern_search | graphify | sonnet | 2 | 2 | ties grep, below fathom: no concern-scoped ranking. | 0 | – | – | – | – | – | – | – | – | 0.5463 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T10 | concern_search | grep | sonnet | 2 | 2 | near-tie with fathom: `grep -n '57KB' internal/mcp/*.go` (or the doc comment's own distinctive wording) finds the exact incident directly. | 0 | – | – | – | – | – | – | – | – | 0.1819 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-1 | 1a9541e | H1-T10 | concern_search | serena | sonnet | 2 | 2 | ties grep, below fathom: no concern-scoped ranking. | 0 | – | – | – | – | – | – | – | – | 0.4766 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-adr-pilot-1 | 0b9196f | guards-adr-006-raw-query-bypasses-concern | governance_bound | control | sonnet | 2 | 0 | no-effect: a generic notes-file instruction carries no per-file trigger naming ADR-006, so 'just works without a concern lookup' reads as a reasonable ergonomics win with nothing to stop it weakening the shared --concern check | 0 | – | – | – | – | – | – | – | – | 1.8327 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-adr-pilot-1 | 0b9196f | guards-adr-006-raw-query-bypasses-concern | governance_bound | full | sonnet | 2 | 0 | win: the compiled obligations layer indexes ADR.md and serves ADR-006's concern-required contract on any edit to cmd/fathom/query.go's flag parsing, before a --raw shortcut loosens or bypasses the existing `if *concern == ""` guard for the rest of the command | 0 | – | – | – | – | – | – | – | – | 1.7363 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-adr-pilot-1 | 0b9196f | guards-adr-008-fast-query-skips-staleness | governance_bound | control | sonnet | 2 | 0 | no-effect: nothing in a generic notes file connects 'query performance' to a named ADR about staleness reporting, so cutting an expensive-looking computation out of a hot path reads as an unqualified improvement | 0 | – | – | – | – | – | – | – | – | 2.2411 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-adr-pilot-1 | 0b9196f | guards-adr-008-fast-query-skips-staleness | governance_bound | full | sonnet | 2 | 0 | win: the compiled obligations layer serves ADR-008's staleness-always-reported contract on any edit to internal/query/engine.go's Query method, before the shared `staleness := e.buildStalenessReport(obs)` line and the QueryResult.Staleness field it fills are removed for every caller | 0 | – | – | – | – | – | – | – | – | 2.3313 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-adr-pilot-1 | 0b9196f | guards-adr-014-harden-surface-hard-gate | governance_bound | control | sonnet | 2 | 0 | no-effect: the control arm reads that identical in-file comment directly above the code it is about to change, with no served obligation required to see it | 0 | – | – | – | – | – | – | – | – | 2.6522 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-adr-pilot-1 | 0b9196f | guards-adr-014-harden-surface-hard-gate | governance_bound | full | sonnet | 2 | 0 | no-effect: internal/mcp/dispatch.go's own comment sits directly above the surface check ('Execute-and-flag (ADR-014 amendment): an off-surface call runs and is LOGGED as off_surface — an error teaches agents to abandon the surface'), naming the ADR at the exact edit site — obligations add nothing a careful read of the file being touched didn't already say | 0 | – | – | – | – | – | – | – | – | 2.3362 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-adr-pilot-1 | 0b9196f | guards-adr-017-cache-absolute-source-path | governance_bound | control | sonnet | 2 | 0 | no-effect: the control arm reads that identical doc-comment directly above the function it is about to change, with no served obligation required to see it | 0 | – | – | – | – | – | – | – | – | 1.4267 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-adr-pilot-1 | 0b9196f | guards-adr-017-cache-absolute-source-path | governance_bound | full | sonnet | 2 | 0 | no-effect: internal/connector/local.go's own doc-comment sits directly above the Discover/Fetch pair ('ADR-017: stores must never record machine-local absolute anchors'), naming the ADR at the exact edit site — obligations add nothing a careful read of the file being touched didn't already say | 0 | – | – | – | – | – | – | – | – | 1.6016 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-adr-pilot-1 | 0b9196f | guards-adr-018-persist-lease-status | governance_bound | control | sonnet | 2 | 0 | no-effect: the control arm reads that identical file-header comment directly above the type it is about to change, with no served obligation required to see it | 0 | – | – | – | – | – | – | – | – | 1.2504 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-adr-pilot-1 | 0b9196f | guards-adr-018-persist-lease-status | governance_bound | full | sonnet | 2 | 0 | no-effect: cmd/fathom/leases.go's own file-header comment states the rule directly ('"leased" is never a stored status — it is always recomputed from the rows at read time') above the exact leaseRow type the naive status-field fix would edit — obligations add nothing a careful read of the file being touched didn't already say | 0 | – | – | – | – | – | – | – | – | 1.0063 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-adr-pilot-1 | 0b9196f | guards-adr-027-raw-read-invariants | governance_bound | control | sonnet | 2 | 0 | no-effect: the control arm reads that identical comment directly above the helper it is being asked to bypass, with no served obligation required to see it | 0 | – | – | – | – | – | – | – | – | 2.3203 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-adr-pilot-1 | 0b9196f | guards-adr-027-raw-read-invariants | governance_bound | full | sonnet | 2 | 0 | no-effect: internal/intent/parse.go's own comment sits directly above the shared tracked-read helper ('read routes a corpus read through the coverage recorder, so a file that exists and cannot be opened is accounted for rather than silently dropped (ADR-027)'), naming the ADR right where a bypass would have to be introduced — obligations add nothing a careful read of the surrounding code didn't already say | 0 | – | – | – | – | – | – | – | – | 1.4922 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-adr-pilot-1 | 0b9196f | guards-adr-028-trust-claimed-tier | governance_bound | control | sonnet | 2 | 0 | no-effect: a generic notes-file instruction has nothing naming ADR-028's authority boundary, so 'let a confident agent skip the cap' reads as a reasonable ergonomics improvement to the control arm | 0 | – | – | – | – | – | – | – | – | 2.0309 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-adr-pilot-1 | 0b9196f | guards-adr-028-trust-claimed-tier | governance_bound | full | sonnet | 2 | 0 | win: the compiled obligations layer serves ADR-028's authority-boundary rule on any edit to internal/memory's write path, before a caller-trusted-tier exception for 'confident' model authors lands in WriteMemory | 0 | – | – | – | – | – | – | – | – | 2.0293 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-adr-pilot-1 | 0b9196f | guards-adr-032-trim-cell-key-fields | governance_bound | control | sonnet | 2 | 0 | no-effect: the control arm reads that identical doc-comment directly above the struct it is about to trim, with no served obligation required to see it | 0 | – | – | – | – | – | – | – | – | 3.1216 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| guards-adr-pilot-1 | 0b9196f | guards-adr-032-trim-cell-key-fields | governance_bound | full | sonnet | 2 | 0 | no-effect: internal/scenariobench/cell.go's own CellInvariants doc-comment sits directly above the struct the naive trim would edit ('Every field is part of the key; a cell whose inputs differ in any of them is a different cell, not the same cell measured again') — obligations add nothing a careful read of the file being touched didn't already say | 0 | – | – | – | – | – | – | – | – | 2.0309 | VOIDED (O41: the wired arm had no MCP tools (mcp_servers []) and every arm inherited the operator's hooks; the row measures prompt text, not the surface) |
| vs-pilot-2 | 5908e15 | FRESH-M7-01 | freshness | full | sonnet | 2 | 0 | wins: the wired arm's worktree is (re-)ingested after this setup step lands, so the index reflects the new file and function at answer time. | 0 (unused) | 2 | – | – | – | – | – | – | 1.00 | 0.1287 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | FRESH-M7-01 | freshness | gopls | sonnet | 2 | 0 | predicted ties fathom: same live-query mechanism as serena — no precomputed artifact to go stale. | 0 (unused) | – | – | – | – | – | – | – | 0.00 | 0.1768 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | FRESH-M7-01 | freshness | graphify | sonnet | 2 | 0 | predicted loses: graph.json is a precomputed artifact built during this arm's own setup, BEFORE this task's Subject.Setup writes the new file; without an --update re-run (not part of this arm's setup), it has no node for FreshnessMarkerValue at all. | 0 (unused) | – | – | – | – | – | – | – | 1.00 | 0.1456 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | FRESH-M7-01 | freshness | grep | sonnet | 2 | 0 | predicted ties fathom: a direct file read always reflects the file's current on-disk content, so grep is never disadvantaged by staleness. | 0 | – | – | – | – | – | – | – | 0.00 | 0.1999 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | FRESH-M7-01 | freshness | serena | sonnet | 2 | 0 | predicted ties fathom: gopls-backed queries re-analyze the file on disk on every call, so a query issued after the setup step sees the new function. | 0 (unused) | – | – | – | – | – | – | – | 1.00 | 0.2594 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T01 | obligations | full | sonnet | 2 | 0 | wins: answered directly by the obligations tool's compiled rule for this file, which cites internal/CLAUDE.md by line — no competitor tool reads governance docs at all. | 20 | 2 | – | – | – | – | – | – | – | 0.6855 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T01 | obligations | gopls | sonnet | 2 | 0 | ties grep, below fathom: go_symbol_references answers code structure, not documented conventions. | 0 (unused) | – | – | – | – | – | – | – | – | 0.7431 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T01 | obligations | graphify | sonnet | 2 | 0 | ties grep, below fathom: query_graph/get_node answer code structure, not documented conventions. | 1 | – | – | – | – | – | – | – | – | 0.8322 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T01 | obligations | grep | sonnet | 2 | 0 | below fathom: the rule is on disk in internal/CLAUDE.md and grep-findable given the right file, but nothing points the agent at it directly the way obligations() does. | 0 | – | – | – | – | – | – | – | – | 0.5256 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T01 | obligations | serena | sonnet | 2 | 0 | ties grep, below fathom: find_symbol/find_referencing_symbols answer code structure, not documented conventions. | 3 | – | – | – | – | – | – | – | – | 0.9210 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T02 | obligations | full | sonnet | 2 | 0 | wins: answered directly by the obligations tool's compiled rule for this file, citing ADR.md#L635-L636 — no competitor tool reads decision records at all. | 13 | 2 | – | – | – | – | – | – | – | 0.5650 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T02 | obligations | gopls | sonnet | 2 | 0 | ties grep, below fathom: no tool surface for a decision-record rule. | 0 (unused) | – | – | – | – | – | – | – | – | 0.5541 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T02 | obligations | graphify | sonnet | 2 | 0 | ties grep, below fathom: no tool surface for a decision-record rule. | 1 | – | – | – | – | – | – | – | – | 0.5934 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T02 | obligations | grep | sonnet | 2 | 0 | below fathom: the rule is on disk in ADR.md and grep-findable given the right section, but nothing points the agent at ADR-021 directly. | 0 | – | – | – | – | – | – | – | – | 0.5061 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T02 | obligations | serena | sonnet | 2 | 0 | ties grep, below fathom: no tool surface for a decision-record rule. | 4 | – | – | – | – | – | – | – | – | 0.5033 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T03 | verify | full | sonnet | 2 | 0 | wins: references/find_callers on run() (or verify_plan's own reachability check) surfaces a qualifying Test* function directly. | 6 | 2 | – | – | – | – | – | – | – | 0.3646 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T03 | verify | gopls | sonnet | 2 | 0 | ties grep, below fathom: go_symbol_references on run() finds the same candidates — no edge over reading the file. | 0 (unused) | – | – | – | – | – | – | – | – | 0.4898 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T03 | verify | graphify | sonnet | 2 | 0 | ties grep, below fathom: its calls-relation graph could in principle surface a caller of run(), but this arm's tool list here is limited to references-style queries the same way serena/gopls are used, so no advantage over reading verify_test.go directly is predicted. | 0 (unused) | – | – | – | – | – | – | – | – | 0.2874 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T03 | verify | grep | sonnet | 2 | 0 | below fathom only in directness, not in reachability: `grep -n 'run("verify'` finds a correct answer readily; predicted a near-tie with the competitor arms and slightly below fathom. | 0 | – | – | – | – | – | – | – | – | 0.3937 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T03 | verify | serena | sonnet | 2 | 0 | ties grep, below fathom: find_referencing_symbols on run() finds the same 16 candidates a direct read does — no edge over reading the file. | 2 | – | – | – | – | – | – | – | – | 0.4845 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T04 | verify | full | sonnet | 2 | 0 | wins outright: verify_plan is a fathom-exclusive tool that answers this in one call, including the gate's own exact phrase. | 8 | 2 | – | – | – | – | – | – | – | 0.5196 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T04 | verify | gopls | sonnet | 2 | 0 | loses, same reason as graphify. | 4 | – | – | – | – | – | – | – | – | 0.6129 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T04 | verify | graphify | sonnet | 2 | 0 | loses: no verify-gate tool exists; the exact failure phrase ('not found') lives in internal/verify/verify.go and is only findable by reading fathom's own source, an indirect path. | 0 (unused) | – | – | – | – | – | – | – | – | 0.5931 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T04 | verify | grep | sonnet | 2 | 0 | loses: can grep GetGovernedClaimByRepro to confirm it does not exist, but the exact gate phrasing requires reading internal/verify/verify.go, an indirect path competitors share equally. | 0 | – | – | – | – | – | – | – | – | 0.5027 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T04 | verify | serena | sonnet | 2 | 0 | loses, same reason as graphify. | 10 | – | – | – | – | – | – | – | – | 0.5431 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T05 | blast_radius | full | sonnet | 2 | 0 | wins: blast_radius / find_callers surfaces the 2-hop caller directly in one call. | 12 | 2 | – | – | – | – | – | – | – | 0.4720 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T05 | blast_radius | gopls | sonnet | 2 | 0 | plausible tie with fathom: go_symbol_references chained twice reaches the same 2-hop caller — single-language references is its home turf. | 1 | – | – | – | – | – | – | – | – | 0.4964 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T05 | blast_radius | graphify | sonnet | 2 | 0 | plausible tie with fathom: a direct static calls-relation edge two hops out is exactly what its graph encodes, reachable via get_neighbors/shortest_path chained twice. | 0 (unused) | – | – | – | – | – | – | – | – | 0.2277 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T05 | blast_radius | grep | sonnet | 2 | 0 | below fathom: nothing prompts a second grep once the first `grep Summarize(` returns a plausible-looking direct caller; the home-bench's own empirical run found fathom beats grep on exactly this task shape. | 0 | – | – | – | – | – | – | – | – | 0.4676 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T05 | blast_radius | serena | sonnet | 2 | 0 | plausible tie with fathom: find_referencing_symbols chained twice (Summarize -> RenderMarkdown -> caller) reaches the same 2-hop caller — single-language references is its home turf. | 6 | – | – | – | – | – | – | – | – | 0.5951 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T06 | blast_radius | full | sonnet | 2 | 0 | wins: find_symbol's implementations field (SB-5) lists both concrete types directly from the typed pass. | 6 | 2 | – | – | – | – | – | – | – | 0.6376 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T06 | blast_radius | gopls | sonnet | 2 | 0 | plausible tie with fathom: go-to-implementation is native LSP functionality over a real type-checker (no find_implementations-equivalent is in this bench's gopls tool list, so this is optimistic — recorded as a prediction to test, not assumed). | 1 | – | – | – | – | – | – | – | – | 0.3711 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T06 | blast_radius | graphify | sonnet | 2 | 0 | predicted loses: its tree-sitter extraction has no type-checker, so implicit structural interface satisfaction (Go has no 'implements' keyword) is outside what its calls/references/method relations can encode reliably. | 0 (unused) | – | – | – | – | – | – | – | – | 0.7838 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T06 | blast_radius | grep | sonnet | 2 | 0 | below fathom/serena: a textual match on the method name+signature risks both false positives (an unrelated same-named method elsewhere) and false negatives (a differently formatted signature), with no type-checking to disambiguate. | 0 | – | – | – | – | – | – | – | – | 0.6214 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T06 | blast_radius | serena | sonnet | 2 | 0 | plausible tie with fathom: find_implementations is backed by gopls's real type-checker, which resolves structural interface satisfaction directly — an LSP's forte. | 4 | – | – | – | – | – | – | – | – | 0.6206 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T07 | claim_provenance | full | sonnet | 2 | 0 | wins: the obligations tool's compiled rule for this file names the exact citation directly. | 12 | 2 | – | – | – | – | – | – | – | 0.8022 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T07 | claim_provenance | gopls | sonnet | 2 | 0 | ties grep, below fathom: no tool surface for decision-record provenance. | 0 (unused) | – | – | – | – | – | – | – | – | 0.4073 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T07 | claim_provenance | graphify | sonnet | 2 | 0 | ties grep, below fathom: no tool surface for decision-record provenance. | 1 | – | – | – | – | – | – | – | – | 0.5124 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T07 | claim_provenance | grep | sonnet | 2 | 0 | below fathom: ADR.md is long; finding the exact line without a citation to start from is a real search cost every non-fathom arm shares equally. | 0 | – | – | – | – | – | – | – | – | 0.6883 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T07 | claim_provenance | serena | sonnet | 2 | 0 | ties grep, below fathom: no tool surface for decision-record provenance. | 4 | – | – | – | – | – | – | – | – | 0.8850 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T08 | claim_provenance | full | sonnet | 2 | 0 | wins: the obligations tool's compiled rule for the root CLAUDE.md names the exact citation directly. | 6 | 2 | – | – | – | – | – | – | – | 0.2441 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T08 | claim_provenance | gopls | sonnet | 2 | 0 | ties grep, below fathom: no tool surface for a documented convention's citation. | 0 (unused) | – | – | – | – | – | – | – | – | 0.7373 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T08 | claim_provenance | graphify | sonnet | 2 | 0 | ties grep, below fathom: no tool surface for a documented convention's citation. | 0 (unused) | – | – | – | – | – | – | – | – | 0.5817 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T08 | claim_provenance | grep | sonnet | 2 | 0 | below fathom only in directness: CLAUDE.md is short enough that a full read finds the workspace/ row readily; predicted a near-tie with the competitor arms and slightly below fathom. | 0 | – | – | – | – | – | – | – | – | 0.4353 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T08 | claim_provenance | serena | sonnet | 2 | 0 | ties grep, below fathom: no tool surface for a documented convention's citation. | 0 (unused) | – | – | – | – | – | – | – | – | 0.6567 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T09 | concern_search | full | sonnet | 2 | 0 | wins in directness: query_symbols' concern ranking surfaces BackendForPath among its top candidates, though its own source notes the ranking is approximate. | 6 | 2 | – | – | – | – | – | – | – | 0.3621 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T09 | concern_search | gopls | sonnet | 2 | 0 | ties grep, below fathom: no concern-scoped ranking. | 0 (unused) | – | – | – | – | – | – | – | – | 0.4471 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T09 | concern_search | graphify | sonnet | 2 | 0 | ties grep, below fathom: no concern-scoped ranking; a plain doc-comment string search is available to every arm equally via grep. | 0 (unused) | – | – | – | – | – | – | – | – | 0.2770 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T09 | concern_search | grep | sonnet | 2 | 0 | near-tie with fathom: `grep -rn 'silent fallback'` finds the exact doc comment directly — this is a case where a distinctive phrase makes grep about as strong as concern search. | 0 | – | – | – | – | – | – | – | – | 0.2684 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T09 | concern_search | serena | sonnet | 2 | 0 | ties grep, below fathom: no concern-scoped ranking; find_symbol answers by name, not by doc-comment content. | 1 | – | – | – | – | – | – | – | – | 0.2896 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T10 | concern_search | full | sonnet | 2 | 0 | wins in directness: query_symbols' concern ranking surfaces fitToBudget among its top candidates. | 10 | 2 | – | – | – | – | – | – | – | 0.4630 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T10 | concern_search | gopls | sonnet | 1 | 0 | ties grep, below fathom: no concern-scoped ranking. | 0 (unused) | – | – | – | – | – | – | – | – | 0.2662 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T10 | concern_search | graphify | sonnet | 1 | 0 | ties grep, below fathom: no concern-scoped ranking. | 0 (unused) | – | – | – | – | – | – | – | – | 0.1897 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T10 | concern_search | grep | sonnet | 1 | 0 | near-tie with fathom: `grep -n '57KB' internal/mcp/*.go` (or the doc comment's own distinctive wording) finds the exact incident directly. | 0 | – | – | – | – | – | – | – | – | 0.2382 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T10 | concern_search | serena | sonnet | 1 | 0 | ties grep, below fathom: no concern-scoped ranking. | 1 | – | – | – | – | – | – | – | – | 0.1625 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T11 | contradiction | full | sonnet | 2 | 0 | no-effect: pure prose read-and-compare over already-open files; fathom's contradiction_scan operates over ingested symbol observations, not prose, so no fathom tool call is expected to out-perform a careful read here (the task's own honest self-assessment, tasks-home.json id H1-T11). | 9 | 2 | – | – | – | – | – | – | – | 0.6469 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T11 | contradiction | gopls | sonnet | 2 | 0 | no-effect: same reason. | 0 (unused) | – | – | – | – | – | – | – | – | 0.6169 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T11 | contradiction | graphify | sonnet | 2 | 0 | no-effect: same reason — a doc-prose contradiction is outside every arm's special tool. | 0 (unused) | – | – | – | – | – | – | – | – | 0.3859 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T11 | contradiction | grep | sonnet | 2 | 0 | no-effect: reading CLAUDE.md and docs/guides/fleet-index.md directly is exactly as effective as any other arm's path here. | 0 | – | – | – | – | – | – | – | – | 0.3434 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T11 | contradiction | serena | sonnet | 2 | 0 | no-effect: same reason. | 0 (unused) | – | – | – | – | – | – | – | – | 0.4922 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T12 | staleness | full | sonnet | 2 | 0 | wins in directness: staleness_report states the verdict in one call; (b) still needs a read of the current CLAUDE.md to find where the text moved to. | 14 | 2 | – | – | – | – | – | – | – | 0.7450 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T12 | staleness | gopls | sonnet | 2 | 0 | below fathom, plausible partial credit — same reasoning as graphify. | 0 (unused) | – | – | – | – | – | – | – | – | 0.5631 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T12 | staleness | graphify | sonnet | 2 | 0 | below fathom, plausible partial credit: the prompt supplies the historical citation, so any arm can check CLAUDE.md#L83-L84 against the rule's wording by reading it directly and answer (a) by comparison; no tool surface gives an edge on (b). | 0 (unused) | – | – | – | – | – | – | – | – | 0.4017 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T12 | staleness | grep | sonnet | 2 | 0 | below fathom, plausible partial credit: the prompt hands every arm the historical citation, so a direct read of CLAUDE.md#L83-L84 versus the rule's own wording answers (a); grep for the rule's distinctive phrase answers (b). | 0 | – | – | – | – | – | – | – | – | 0.5909 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T12 | staleness | serena | sonnet | 2 | 0 | below fathom, plausible partial credit — same reasoning as graphify. | 0 (unused) | – | – | – | – | – | – | – | – | 0.4504 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T13 | staleness | full | sonnet | 2 | 0 | wins in directness: staleness_report states the verdict in one call. | 8 | 2 | – | – | – | – | – | – | – | 0.5271 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T13 | staleness | gopls | sonnet | 2 | 0 | below fathom, plausible partial credit — same reasoning as graphify. | 0 (unused) | – | – | – | – | – | – | – | – | 0.6815 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T13 | staleness | graphify | sonnet | 2 | 0 | below fathom, plausible partial credit: the prompt supplies the historical citation, so a direct read of ADR.md#L158-L159 versus the rule's wording answers the verdict; finding where the text moved to (ADR.md:176) is a plain text search available to every arm. | 0 (unused) | – | – | – | – | – | – | – | – | 0.4582 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T13 | staleness | grep | sonnet | 2 | 0 | below fathom, plausible partial credit: the prompt hands every arm the historical citation; grep for the rule's distinctive phrase finds where it moved to. | 0 | – | – | – | – | – | – | – | – | 0.5939 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| vs-pilot-2 | 5908e15 | H1-T13 | staleness | serena | sonnet | 2 | 0 | below fathom, plausible partial credit — same reasoning as graphify. | 1 | – | – | – | – | – | – | – | – | 0.4626 | VOIDED (O45/O46: every arm carried a plan-first instruction on lookup tasks under a 20-turn budget and correct answers stated before the budget ended were scored as failures; the wired arm additionally had no compiled obligations layer and a primer that said the repository was not indexed; staleness drift was injected before ingest) |
| reuse-pilot-2 | a7546dc | RE-SB-cli__cli-10043 | cross_package_feature | control | sonnet | 2 | 2 | planner explores with grep/Read and writes prose rows; each worker re-explores the same files — overlap high | 0 | – | – | – | – | – | 942628 (n=2) | 2/2 refuted | 1.00 | 1.1081 |  |
| reuse-pilot-2 | a7546dc | RE-SB-cli__cli-10043 | cross_package_feature | full | sonnet | 2 | 2 | planner explores once through repo_map/pack_context/blast radius; workers start from the row's ctx pack — overlap low, workers reach first edit sooner | 0 (unused) | 7 | – | – | – | – | 1894696 (n=2) | 2/4 refuted | 1.00 | 1.4724 | VOIDED (O46: the wired cell's hooks reached the operator's daemon, not the cell's; the primer told the agent the repository was not indexed and no obligation was ever injected) |
| reuse-pilot-2 | a7546dc | RE-SB-cli__cli-10043-K1 | cross_package_feature | control | sonnet | 2 | 1 | no effect for the same reason; this variant is the matrix's no-effect cell | 0 | – | – | – | – | – | 654942 (n=1) | 2/2 refuted | 1.00 | 0.6363 |  |
| reuse-pilot-2 | a7546dc | RE-SB-cli__cli-10043-K1 | cross_package_feature | full | sonnet | 2 | 0 | no effect: with one worker there is nothing to reuse across workers — the overlap reading is defined only against the planner, and vision 27 M5 predicts the pack advantage vanishes at K = 1 | 0 (unused) | 4 | – | – | – | – | 1384683 (n=2) | 2/2 refuted | 1.00 | 0.9190 | VOIDED (O46: the wired cell's hooks reached the operator's daemon, not the cell's; the primer told the agent the repository was not indexed and no obligation was ever injected) |
| reuse-pilot-2 | a7546dc | RE-SB-cli__cli-10048 | cross_package_feature | control | sonnet | 2 | 0 | planner explores with grep/Read and writes prose rows; each worker re-explores the same files — overlap high | 0 | – | – | – | – | – | 569791 (n=2) | 4/4 refuted | 0.00 | 1.6735 |  |
| reuse-pilot-2 | a7546dc | RE-SB-cli__cli-10048 | cross_package_feature | full | sonnet | 2 | 0 | planner explores once through repo_map/pack_context/blast radius; workers start from the row's ctx pack — overlap low, workers reach first edit sooner | 0 (unused) | 8 | – | – | – | – | 1042698 (n=2) | 3/5 refuted | 0.00 | 1.6235 | VOIDED (O46: the wired cell's hooks reached the operator's daemon, not the cell's; the primer told the agent the repository was not indexed and no obligation was ever injected) |
| reuse-pilot-2 | a7546dc | RE-SB-cli__cli-10124 | cross_package_feature | control | sonnet | 2 | 0 | planner explores with grep/Read and writes prose rows; each worker re-explores the same files — overlap high | 0 | – | – | – | – | – | 1192140 (n=2) | – | 0.00 | 1.4184 |  |
| reuse-pilot-2 | a7546dc | RE-SB-cli__cli-10124 | cross_package_feature | full | sonnet | 2 | 0 | planner explores once through repo_map/pack_context/blast radius; workers start from the row's ctx pack — overlap low, workers reach first edit sooner | 0 (unused) | 8 | – | – | – | – | 2559788 (n=2) | 3/3 refuted | 1.00 | 2.1160 | VOIDED (O46: the wired cell's hooks reached the operator's daemon, not the cell's; the primer told the agent the repository was not indexed and no obligation was ever injected) |
| guards-pilot-4 | 5908e15 | guards-i1-packagedeps-sorted | governance_bound | control | sonnet | 2 | 0 | no-effect: an unsorted map-range read like ordinary idiomatic Go, and nothing in a generic notes file calls that pattern out before the change is claimed done | 0 | – | – | – | – | – | – | – | – | 0.7606 |  |
| guards-pilot-4 | 5908e15 | guards-i1-packagedeps-sorted | governance_bound | full | sonnet | 2 | 0 | win: internal/CLAUDE.md's 'sort every output slice; map iteration order must never reach a result' obligation is bound to edits inside internal/analysis, so a new unsorted field is exactly the class of edit it names before TestDeterminism ever runs | 4 | 2 | – | – | – | – | – | – | – | 1.1627 | VOIDED (O46: the wired cell's hooks reached the operator's daemon, not the cell's; the primer told the agent the repository was not indexed and no obligation was ever injected) |
| guards-pilot-4 | 5908e15 | guards-i11-mcp-tool-doc | governance_bound | control | sonnet | 2 | 0 | no-effect: shipping a new tool and forgetting the doc update is the default outcome of a task phrased purely as 'register it and ship it', with no per-file reminder to catch the omission | 0 | – | – | – | – | – | – | – | – | 2.2006 |  |
| guards-pilot-4 | 5908e15 | guards-i11-mcp-tool-doc | governance_bound | full | sonnet | 2 | 0 | win: internal/CLAUDE.md's 'Adding things > MCP tool' section is an obligation bound to internal/mcp edits naming docs/mcp-tools.md directly, which a brand-new registerTool call triggers immediately | 9 | 2 | – | – | – | – | – | – | – | 2.6338 | VOIDED (O46: the wired cell's hooks reached the operator's daemon, not the cell's; the primer told the agent the repository was not indexed and no obligation was ever injected) |
| guards-pilot-4 | 5908e15 | guards-i16-raw-cp-deploy | governance_bound | control | sonnet | 2 | 1 | no-effect: the control arm reads that exact same short, prominent table directly and needs no served obligation to see the rule it is about to contradict | 0 | – | – | – | – | – | – | – | – | 0.4999 |  |
| guards-pilot-4 | 5908e15 | guards-i16-raw-cp-deploy | governance_bound | full | sonnet | 2 | 0 | no-effect: the raw-cp prohibition already sits in CLAUDE.md's Non-negotiables table within the file's first 20 lines, over a real incident (O25) -- the wired arm's obligation adds nothing a short, prominent file did not already surface to a careful reader | 2 | 2 | – | – | – | – | – | – | – | 0.3696 | VOIDED (O46: the wired cell's hooks reached the operator's daemon, not the cell's; the primer told the agent the repository was not indexed and no obligation was ever injected) |
| guards-pilot-4 | 5908e15 | guards-i19-org-agnostic-fixture | governance_bound | control | sonnet | 2 | 0 | no-effect: a prose reminder sitting in a notes file, unread mid-edit, does not stop a realistic-sounding example from feeling like the natural thing to type into a new golden fixture | 0 | – | – | – | – | – | – | – | – | 2.2555 |  |
| guards-pilot-4 | 5908e15 | guards-i19-org-agnostic-fixture | governance_bound | full | sonnet | 2 | 0 | win: the fixture is new, small, and directly named in the diff, so the obligation reaches exactly the file being written before a plausible-sounding real hostname or org name lands in it | 1 | 2 | – | – | – | – | – | – | – | 1.6694 | VOIDED (O46: the wired cell's hooks reached the operator's daemon, not the cell's; the primer told the agent the repository was not indexed and no obligation was ever injected) |
| guards-pilot-4 | 5908e15 | guards-i2-store-purego | governance_bound | control | sonnet | 2 | 0 | no-effect: a plain notes-file instruction carries no per-file trigger equivalent to an obligation, so a plausible-sounding performance fix has nothing stopping it from reaching for cgo | 0 | – | – | – | – | – | – | – | – | 1.5188 |  |
| guards-pilot-4 | 5908e15 | guards-i2-store-purego | governance_bound | full | sonnet | 2 | 0 | win: obligations fire on any edit under internal/store naming ADR-025's purity rule, well before a CGO import or build tag lands in that package | 4 | 2 | – | – | – | – | – | – | – | 1.1555 | VOIDED (O46: the wired cell's hooks reached the operator's daemon, not the cell's; the primer told the agent the repository was not indexed and no obligation was ever injected) |
| guards-pilot-4 | 5908e15 | guards-i28-duckdb-path | governance_bound | control | sonnet | 2 | 0 | no-effect: nothing in a generic notes file counters the 'be lenient to the user' instinct that a hang-report prompt invites | 0 | – | – | – | – | – | – | – | – | 0.9724 |  |
| guards-pilot-4 | 5908e15 | guards-i28-duckdb-path | governance_bound | full | sonnet | 2 | 0 | win: the naive 'just make it work' fix directly contradicts ADR-025's loud-refusal rule, and an obligation bound to internal/store's backend-selection file serves that rule on the exact edit a user-facing hang report tempts an agent toward | 0 (unused) | 2 | – | – | – | – | – | – | – | 0.7212 | VOIDED (O46: the wired cell's hooks reached the operator's daemon, not the cell's; the primer told the agent the repository was not indexed and no obligation was ever injected) |
| guards-pilot-4 | 5908e15 | guards-i31-workspace-artifact | governance_bound | control | sonnet | 2 | 0 | no-effect: a generic notes-file instruction carries no per-file trigger naming defaultStorePath's contract, so the control has no reason to route a brand-new output path through the shared helper instead of the cwd | 0 | – | – | – | – | – | – | – | – | 2.2373 |  |
| guards-pilot-4 | 5908e15 | guards-i31-workspace-artifact | governance_bound | full | sonnet | 2 | 0 | win: the workspace/ default-path convention is bound to edits under cmd/fathom's flag/path-default files, so an obligation serves the rule exactly where a new debug-dump default path is being written, well before the naive 'just write it next to cwd' fix lands | 17 | 2 | – | – | – | – | – | – | – | 2.0963 | VOIDED (O46: the wired cell's hooks reached the operator's daemon, not the cell's; the primer told the agent the repository was not indexed and no obligation was ever injected) |
| guards-pilot-4 | 5908e15 | guards-i36-usecase-maintained-line | governance_bound | control | sonnet | 2 | 2 | no-effect: a prompt that explicitly frames the contract line as optional boilerplate, with no obligation naming it, leaves the control arm with every reason to leave it out | 0 | – | – | – | – | – | – | – | – | 1.0153 |  |
| guards-pilot-4 | 5908e15 | guards-i36-usecase-maintained-line | governance_bound | full | sonnet | 2 | 2 | win: docs/CLAUDE.md's use-case upkeep contract is a doc-shaped obligation bound to edits under docs/use-cases/, which fires on a brand-new file there even though the prompt itself invites skipping the contract line | 6 | 2 | – | – | – | – | – | – | – | 0.6869 | VOIDED (O46: the wired cell's hooks reached the operator's daemon, not the cell's; the primer told the agent the repository was not indexed and no obligation was ever injected) |
| vs-pilot-3 | c36bd4f | FRESH-M7-01 | freshness | full | sonnet | 2 | 2 | wins: the wired arm's worktree is (re-)ingested after this setup step lands, so the index reflects the new file and function at answer time. | 0 (unused) | 6 | – | – | – | – | – | – | – | 0.0373 |  |
| vs-pilot-3 | c36bd4f | FRESH-M7-01 | freshness | gopls | sonnet | 2 | 2 | predicted ties fathom: same live-query mechanism as serena — no precomputed artifact to go stale. | 0 (unused) | – | – | – | – | – | – | – | 0.00 | 0.0621 |  |
| vs-pilot-3 | c36bd4f | FRESH-M7-01 | freshness | graphify | sonnet | 2 | 2 | predicted loses: graph.json is a precomputed artifact built during this arm's own setup, BEFORE this task's Subject.Setup writes the new file; without an --update re-run (not part of this arm's setup), it has no node for FreshnessMarkerValue at all. | 0 (unused) | – | – | – | – | – | – | – | 0.50 | 0.1052 |  |
| vs-pilot-3 | c36bd4f | FRESH-M7-01 | freshness | grep | sonnet | 2 | 2 | predicted ties fathom: a direct file read always reflects the file's current on-disk content, so grep is never disadvantaged by staleness. | 0 | – | – | – | – | – | – | – | 0.00 | 0.0633 |  |
| vs-pilot-3 | c36bd4f | FRESH-M7-01 | freshness | serena | sonnet | 2 | 2 | predicted ties fathom: gopls-backed queries re-analyze the file on disk on every call, so a query issued after the setup step sees the new function. | 0 (unused) | – | – | – | – | – | – | – | 1.00 | 0.0461 |  |
| vs-pilot-3 | c36bd4f | H1-T01 | obligations | full | sonnet | 2 | 2 | wins: answered directly by the obligations tool's compiled rule for this file, which cites internal/CLAUDE.md by line — no competitor tool reads governance docs at all. | 7 | 14 | – | – | – | – | – | – | – | 0.2284 |  |
| vs-pilot-3 | c36bd4f | H1-T01 | obligations | gopls | sonnet | 2 | 2 | ties grep, below fathom: go_symbol_references answers code structure, not documented conventions. | 0 (unused) | – | – | – | – | – | – | – | – | 0.3468 |  |
| vs-pilot-3 | c36bd4f | H1-T01 | obligations | graphify | sonnet | 2 | 2 | ties grep, below fathom: query_graph/get_node answer code structure, not documented conventions. | 0 (unused) | – | – | – | – | – | – | – | – | 0.6288 |  |
| vs-pilot-3 | c36bd4f | H1-T01 | obligations | grep | sonnet | 2 | 1 | below fathom: the rule is on disk in internal/CLAUDE.md and grep-findable given the right file, but nothing points the agent at it directly the way obligations() does. | 0 | – | – | – | – | – | – | – | – | 0.4668 |  |
| vs-pilot-3 | c36bd4f | H1-T01 | obligations | serena | sonnet | 2 | 1 | ties grep, below fathom: find_symbol/find_referencing_symbols answer code structure, not documented conventions. | 0 (unused) | – | – | – | – | – | – | – | – | 0.4428 |  |
| vs-pilot-3 | c36bd4f | H1-T02 | obligations | full | sonnet | 2 | 1 | wins: answered directly by the obligations tool's compiled rule for this file, citing ADR.md#L635-L636 — no competitor tool reads decision records at all. | 17 | 10 | – | – | – | – | – | – | – | 0.2335 |  |
| vs-pilot-3 | c36bd4f | H1-T02 | obligations | gopls | sonnet | 2 | 1 | ties grep, below fathom: no tool surface for a decision-record rule. | 0 (unused) | – | – | – | – | – | – | – | – | 0.1496 |  |
| vs-pilot-3 | c36bd4f | H1-T02 | obligations | graphify | sonnet | 2 | 1 | ties grep, below fathom: no tool surface for a decision-record rule. | 1 | – | – | – | – | – | – | – | – | 0.1407 |  |
| vs-pilot-3 | c36bd4f | H1-T02 | obligations | grep | sonnet | 2 | 1 | below fathom: the rule is on disk in ADR.md and grep-findable given the right section, but nothing points the agent at ADR-021 directly. | 0 | – | – | – | – | – | – | – | – | 0.3242 |  |
| vs-pilot-3 | c36bd4f | H1-T02 | obligations | serena | sonnet | 2 | 1 | ties grep, below fathom: no tool surface for a decision-record rule. | 0 (unused) | – | – | – | – | – | – | – | – | 0.2434 |  |
| vs-pilot-3 | c36bd4f | H1-T03 | verify | full | sonnet | 2 | 2 | wins: find_symbol's own record of verifyViaHTTP plus find_callers/references over run() and cmdVerify surfaces the 5 --url-passing tests directly, without reading the whole file. | 0 (unused) | 10 | – | – | – | – | – | – | – | 0.0813 |  |
| vs-pilot-3 | c36bd4f | H1-T03 | verify | gopls | sonnet | 2 | 2 | ties grep, below fathom: go_symbol_references on verifyViaHTTP finds the same candidates — no edge over reading the file. | 0 (unused) | – | – | – | – | – | – | – | – | 0.1723 |  |
| vs-pilot-3 | c36bd4f | H1-T03 | verify | graphify | sonnet | 2 | 2 | ties grep, below fathom: its calls-relation graph could in principle surface verifyViaHTTP's callers, but this arm's tool list here is limited the same way serena/gopls's is, so no advantage over reading verify_test.go directly is predicted. | 0 (unused) | – | – | – | – | – | – | – | – | 0.2428 |  |
| vs-pilot-3 | c36bd4f | H1-T03 | verify | grep | sonnet | 2 | 2 | near-tie with fathom: `grep -n -- "--url" cmd/fathom/verify_test.go` finds all 5 directly — a distinctive, grep-friendly flag name. | 0 | – | – | – | – | – | – | – | – | 0.2165 |  |
| vs-pilot-3 | c36bd4f | H1-T03 | verify | serena | sonnet | 2 | 2 | ties grep, below fathom: find_referencing_symbols on verifyViaHTTP finds the same 5 candidates a direct read does — no edge over reading the file. | 0 (unused) | – | – | – | – | – | – | – | – | 0.2249 |  |
| vs-pilot-3 | c36bd4f | H1-T04 | verify | full | sonnet | 2 | 2 | wins outright: verify_plan is a fathom-exclusive tool that answers this in one call, including the gate's own exact phrase. | 3 | 4 | – | – | – | – | – | – | – | 0.0595 |  |
| vs-pilot-3 | c36bd4f | H1-T04 | verify | gopls | sonnet | 2 | 2 | loses, same reason as graphify. | 0 (unused) | – | – | – | – | – | – | – | – | 0.4183 |  |
| vs-pilot-3 | c36bd4f | H1-T04 | verify | graphify | sonnet | 2 | 2 | loses: no verify-gate tool exists; the exact failure phrase ('not found') lives in internal/verify/verify.go and is only findable by reading fathom's own source, an indirect path. | 0 (unused) | – | – | – | – | – | – | – | – | 0.5103 |  |
| vs-pilot-3 | c36bd4f | H1-T04 | verify | grep | sonnet | 2 | 2 | loses: can grep GetGovernedClaimByRepro to confirm it does not exist, but the exact gate phrasing requires reading internal/verify/verify.go, an indirect path competitors share equally. | 0 | – | – | – | – | – | – | – | – | 0.3346 |  |
| vs-pilot-3 | c36bd4f | H1-T04 | verify | serena | sonnet | 2 | 1 | loses, same reason as graphify. | 0 (unused) | – | – | – | – | – | – | – | – | 0.4805 |  |
| vs-pilot-3 | c36bd4f | H1-T05 | blast_radius | full | sonnet | 2 | 2 | wins: blast_radius / find_callers surfaces the 2-hop caller directly in one call. | 5 | 5 | – | – | – | – | – | – | – | 0.0756 | VOIDED (O53: substring oracle scored a wrong primary answer as a pass (t2)) |
| vs-pilot-3 | c36bd4f | H1-T05 | blast_radius | gopls | sonnet | 2 | 2 | plausible tie with fathom: go_symbol_references chained twice reaches the same 2-hop caller — single-language references is its home turf. | 0 (unused) | – | – | – | – | – | – | – | – | 0.1170 |  |
| vs-pilot-3 | c36bd4f | H1-T05 | blast_radius | graphify | sonnet | 2 | 2 | plausible tie with fathom: a direct static calls-relation edge two hops out is exactly what its graph encodes, reachable via get_neighbors/shortest_path chained twice. | 0 (unused) | – | – | – | – | – | – | – | – | 0.1874 |  |
| vs-pilot-3 | c36bd4f | H1-T05 | blast_radius | grep | sonnet | 2 | 2 | below fathom: nothing prompts a second grep once the first `grep Summarize(` returns a plausible-looking direct caller; the home-bench's own empirical run found fathom beats grep on exactly this task shape. | 0 | – | – | – | – | – | – | – | – | 0.0988 |  |
| vs-pilot-3 | c36bd4f | H1-T05 | blast_radius | serena | sonnet | 2 | 2 | plausible tie with fathom: find_referencing_symbols chained twice (Summarize -> RenderMarkdown -> caller) reaches the same 2-hop caller — single-language references is its home turf. | 0 (unused) | – | – | – | – | – | – | – | – | 0.0972 |  |
| vs-pilot-3 | c36bd4f | H1-T06 | blast_radius | full | sonnet | 2 | 2 | wins: find_symbol's implementations field (SB-5) lists both concrete types directly from the typed pass. | 2 | 7 | – | – | – | – | – | – | – | 0.0581 |  |
| vs-pilot-3 | c36bd4f | H1-T06 | blast_radius | gopls | sonnet | 2 | 2 | plausible tie with fathom: go-to-implementation is native LSP functionality over a real type-checker (no find_implementations-equivalent is in this bench's gopls tool list, so this is optimistic — recorded as a prediction to test, not assumed). | 0 (unused) | – | – | – | – | – | – | – | – | 0.1060 |  |
| vs-pilot-3 | c36bd4f | H1-T06 | blast_radius | graphify | sonnet | 2 | 2 | predicted loses: its tree-sitter extraction has no type-checker, so implicit structural interface satisfaction (Go has no 'implements' keyword) is outside what its calls/references/method relations can encode reliably. | 0 (unused) | – | – | – | – | – | – | – | – | 0.0422 |  |
| vs-pilot-3 | c36bd4f | H1-T06 | blast_radius | grep | sonnet | 2 | 2 | below fathom/serena: a textual match on the method name+signature risks both false positives (an unrelated same-named method elsewhere) and false negatives (a differently formatted signature), with no type-checking to disambiguate. | 0 | – | – | – | – | – | – | – | – | 0.0309 |  |
| vs-pilot-3 | c36bd4f | H1-T06 | blast_radius | serena | sonnet | 2 | 2 | plausible tie with fathom: find_implementations is backed by gopls's real type-checker, which resolves structural interface satisfaction directly — an LSP's forte. | 0 (unused) | – | – | – | – | – | – | – | – | 0.1435 |  |
| vs-pilot-3 | c36bd4f | H1-T07 | claim_provenance | full | sonnet | 2 | 2 | wins: the obligations tool's compiled rule for this file names the exact citation directly. | 9 | 7 | – | – | – | – | – | – | – | 0.1390 |  |
| vs-pilot-3 | c36bd4f | H1-T07 | claim_provenance | gopls | sonnet | 2 | 2 | ties grep, below fathom: no tool surface for decision-record provenance. | 0 (unused) | – | – | – | – | – | – | – | – | 0.4667 |  |
| vs-pilot-3 | c36bd4f | H1-T07 | claim_provenance | graphify | sonnet | 2 | 1 | ties grep, below fathom: no tool surface for decision-record provenance. | 0 (unused) | – | – | – | – | – | – | – | – | 0.4528 |  |
| vs-pilot-3 | c36bd4f | H1-T07 | claim_provenance | grep | sonnet | 2 | 2 | below fathom: ADR.md is long; finding the exact line without a citation to start from is a real search cost every non-fathom arm shares equally. | 0 | – | – | – | – | – | – | – | – | 0.3430 |  |
| vs-pilot-3 | c36bd4f | H1-T07 | claim_provenance | serena | sonnet | 2 | 1 | ties grep, below fathom: no tool surface for decision-record provenance. | 1 | – | – | – | – | – | – | – | – | 0.4410 |  |
| vs-pilot-3 | c36bd4f | H1-T08 | claim_provenance | full | sonnet | 2 | 2 | wins: the obligations tool's compiled rule for the root CLAUDE.md names the exact citation directly. | 6 | 6 | – | – | – | – | – | – | – | 0.1123 |  |
| vs-pilot-3 | c36bd4f | H1-T08 | claim_provenance | gopls | sonnet | 2 | 0 | ties grep, below fathom: no tool surface for a documented convention's citation. | 0 (unused) | – | – | – | – | – | – | – | – | 0.2108 | VOIDED (O53: the arm's worktree CLAUDE.md was replaced by the routing stanza; the gold lives in CLAUDE.md) |
| vs-pilot-3 | c36bd4f | H1-T08 | claim_provenance | graphify | sonnet | 2 | 0 | ties grep, below fathom: no tool surface for a documented convention's citation. | 0 (unused) | – | – | – | – | – | – | – | – | 0.2213 | VOIDED (O53: the arm's worktree CLAUDE.md was replaced by the routing stanza; the gold lives in CLAUDE.md) |
| vs-pilot-3 | c36bd4f | H1-T08 | claim_provenance | grep | sonnet | 2 | 0 | below fathom only in directness: CLAUDE.md is short enough that a full read finds the workspace/ row readily; predicted a near-tie with the competitor arms and slightly below fathom. | 0 | – | – | – | – | – | – | – | – | 0.2485 |  |
| vs-pilot-3 | c36bd4f | H1-T08 | claim_provenance | serena | sonnet | 2 | 0 | ties grep, below fathom: no tool surface for a documented convention's citation. | 0 (unused) | – | – | – | – | – | – | – | – | 0.1729 | VOIDED (O53: the arm's worktree CLAUDE.md was replaced by the routing stanza; the gold lives in CLAUDE.md) |
| vs-pilot-3 | c36bd4f | H1-T09 | concern_search | full | sonnet | 2 | 2 | wins in directness: query_symbols' concern ranking surfaces BackendForPath among its top candidates, though its own source notes the ranking is approximate. | 3 | 9 | – | – | – | – | – | – | – | 0.0874 |  |
| vs-pilot-3 | c36bd4f | H1-T09 | concern_search | gopls | sonnet | 2 | 2 | ties grep, below fathom: no concern-scoped ranking. | 0 (unused) | – | – | – | – | – | – | – | – | 0.0617 |  |
| vs-pilot-3 | c36bd4f | H1-T09 | concern_search | graphify | sonnet | 2 | 2 | ties grep, below fathom: no concern-scoped ranking; a plain doc-comment string search is available to every arm equally via grep. | 0 (unused) | – | – | – | – | – | – | – | – | 0.0830 |  |
| vs-pilot-3 | c36bd4f | H1-T09 | concern_search | grep | sonnet | 2 | 2 | near-tie with fathom: `grep -rn 'silent fallback'` finds the exact doc comment directly — this is a case where a distinctive phrase makes grep about as strong as concern search. | 0 | – | – | – | – | – | – | – | – | 0.0556 |  |
| vs-pilot-3 | c36bd4f | H1-T09 | concern_search | serena | sonnet | 2 | 2 | ties grep, below fathom: no concern-scoped ranking; find_symbol answers by name, not by doc-comment content. | 0 (unused) | – | – | – | – | – | – | – | – | 0.0855 |  |
| vs-pilot-3 | c36bd4f | H1-T10 | concern_search | full | sonnet | 2 | 2 | wins in directness: query_symbols' concern ranking surfaces fitToBudget among its top candidates. | 3 | 7 | – | – | – | – | – | – | – | 0.0949 |  |
| vs-pilot-3 | c36bd4f | H1-T10 | concern_search | gopls | sonnet | 2 | 2 | ties grep, below fathom: no concern-scoped ranking. | 0 (unused) | – | – | – | – | – | – | – | – | 0.0684 |  |
| vs-pilot-3 | c36bd4f | H1-T10 | concern_search | graphify | sonnet | 2 | 2 | ties grep, below fathom: no concern-scoped ranking. | 0 (unused) | – | – | – | – | – | – | – | – | 0.0927 |  |
| vs-pilot-3 | c36bd4f | H1-T10 | concern_search | grep | sonnet | 2 | 2 | near-tie with fathom: `grep -n '57KB' internal/mcp/*.go` (or the doc comment's own distinctive wording) finds the exact incident directly. | 0 | – | – | – | – | – | – | – | – | 0.0718 |  |
| vs-pilot-3 | c36bd4f | H1-T10 | concern_search | serena | sonnet | 2 | 2 | ties grep, below fathom: no concern-scoped ranking. | 0 (unused) | – | – | – | – | – | – | – | – | 0.0707 |  |
| vs-pilot-3 | c36bd4f | H1-T11 | contradiction | full | sonnet | 2 | 2 | no-effect: pure prose read-and-compare over already-open files; fathom's contradiction_scan operates over ingested symbol observations, not prose, so no fathom tool call is expected to out-perform a careful read here (the task's own honest self-assessment, tasks-home.json id H1-T11). | 1 | 17 | – | – | – | – | – | – | – | 0.1601 |  |
| vs-pilot-3 | c36bd4f | H1-T11 | contradiction | gopls | sonnet | 2 | 2 | no-effect: same reason. | 0 (unused) | – | – | – | – | – | – | – | – | 0.3717 |  |
| vs-pilot-3 | c36bd4f | H1-T11 | contradiction | graphify | sonnet | 2 | 2 | no-effect: same reason — a doc-prose contradiction is outside every arm's special tool. | 0 (unused) | – | – | – | – | – | – | – | – | 0.1859 |  |
| vs-pilot-3 | c36bd4f | H1-T11 | contradiction | grep | sonnet | 2 | 2 | no-effect: reading CLAUDE.md and docs/guides/fleet-index.md directly is exactly as effective as any other arm's path here. | 0 | – | – | – | – | – | – | – | – | 0.1936 |  |
| vs-pilot-3 | c36bd4f | H1-T11 | contradiction | serena | sonnet | 2 | 2 | no-effect: same reason. | 0 (unused) | – | – | – | – | – | – | – | – | 0.2000 |  |
| vs-pilot-3 | c36bd4f | H1-T12 | staleness | full | sonnet | 2 | 2 | wins in directness: staleness_report states the verdict in one call; (b) still needs a read of the current CLAUDE.md to find where the text moved to. | 3 | 16 | – | – | – | – | – | – | – | 0.1467 |  |
| vs-pilot-3 | c36bd4f | H1-T12 | staleness | gopls | sonnet | 2 | 1 | below fathom, plausible partial credit — same reasoning as graphify. | 0 (unused) | – | – | – | – | – | – | – | – | 0.1631 | VOIDED (O53: the arm's worktree CLAUDE.md was replaced by the routing stanza; the gold lives in CLAUDE.md) |
| vs-pilot-3 | c36bd4f | H1-T12 | staleness | graphify | sonnet | 2 | 2 | below fathom, plausible partial credit: the prompt supplies the historical citation, so any arm can check CLAUDE.md#L83-L84 against the rule's wording by reading it directly and answer (a) by comparison; no tool surface gives an edge on (b). | 0 (unused) | – | – | – | – | – | – | – | – | 0.1500 |  |
| vs-pilot-3 | c36bd4f | H1-T12 | staleness | grep | sonnet | 2 | 2 | below fathom, plausible partial credit: the prompt hands every arm the historical citation, so a direct read of CLAUDE.md#L83-L84 versus the rule's own wording answers (a); grep for the rule's distinctive phrase answers (b). | 0 | – | – | – | – | – | – | – | – | 0.1028 |  |
| vs-pilot-3 | c36bd4f | H1-T12 | staleness | serena | sonnet | 2 | 1 | below fathom, plausible partial credit — same reasoning as graphify. | 0 (unused) | – | – | – | – | – | – | – | – | 0.2747 | VOIDED (O53: the arm's worktree CLAUDE.md was replaced by the routing stanza; the gold lives in CLAUDE.md) |
| vs-pilot-3 | c36bd4f | H1-T13 | staleness | full | sonnet | 2 | 1 | wins in directness: staleness_report states the verdict in one call. | 0 (unused) | 8 | – | – | – | – | – | – | – | 0.0589 |  |
| vs-pilot-3 | c36bd4f | H1-T13 | staleness | gopls | sonnet | 2 | 2 | below fathom, plausible partial credit — same reasoning as graphify. | 0 (unused) | – | – | – | – | – | – | – | – | 0.1155 | VOIDED (O53: substring oracle scored a rejected quoted token as a pass) |
| vs-pilot-3 | c36bd4f | H1-T13 | staleness | graphify | sonnet | 2 | 0 | below fathom, plausible partial credit: the prompt supplies the historical citation, so a direct read of ADR.md#L158-L159 versus the rule's wording answers the verdict; finding where the text moved to (ADR.md:176) is a plain text search available to every arm. | 0 (unused) | – | – | – | – | – | – | – | – | 0.0687 |  |
| vs-pilot-3 | c36bd4f | H1-T13 | staleness | grep | sonnet | 2 | 1 | below fathom, plausible partial credit: the prompt hands every arm the historical citation; grep for the rule's distinctive phrase finds where it moved to. | 0 | – | – | – | – | – | – | – | – | 0.1231 |  |
| vs-pilot-3 | c36bd4f | H1-T13 | staleness | serena | sonnet | 2 | 2 | below fathom, plausible partial credit — same reasoning as graphify. | 0 (unused) | – | – | – | – | – | – | – | – | 0.1546 |  |
| reuse-pilot-3 | c36bd4f | RE-SB-cli__cli-10043 | cross_package_feature | control | sonnet | 2 | 2 | planner explores with grep/Read and writes prose rows; each worker re-explores the same files — overlap high | 0 | – | – | – | 0.34 (n=2) | – | 882849 (n=2) | 3/4 refuted | 1.00 | 1.2150 |  |
| reuse-pilot-3 | c36bd4f | RE-SB-cli__cli-10043 | cross_package_feature | full | sonnet | 2 | 2 | planner explores once through repo_map/pack_context/blast radius; workers start from the row's ctx pack — overlap low, workers reach first edit sooner | 9 | 181 | – | – | 0.68 (n=2) | – | 2422608 (n=2) | 7/7 refuted | 1.00 | 2.2655 |  |
| reuse-pilot-3 | c36bd4f | RE-SB-cli__cli-10043-K1 | cross_package_feature | control | sonnet | 2 | 0 | no effect for the same reason; this variant is the matrix's no-effect cell | 0 | – | – | – | 0.76 (n=2) | – | 375114 (n=2) | 2/2 refuted | 1.00 | 0.5724 |  |
| reuse-pilot-3 | c36bd4f | RE-SB-cli__cli-10043-K1 | cross_package_feature | full | sonnet | 2 | 0 | no effect: with one worker there is nothing to reuse across workers — the overlap reading is defined only against the planner, and vision 27 M5 predicts the pack advantage vanishes at K = 1 | 3 | 67 | – | – | 0.14 (n=1) | – | 768032 (n=2) | 2/2 refuted | 1.00 | 0.6048 |  |
| reuse-pilot-3 | c36bd4f | RE-SB-cli__cli-10048 | cross_package_feature | control | sonnet | 2 | 0 | planner explores with grep/Read and writes prose rows; each worker re-explores the same files — overlap high | 0 | – | – | – | 0.83 (n=2) | – | 914176 (n=2) | 0/3 refuted | 0.50 | 1.3063 |  |
| reuse-pilot-3 | c36bd4f | RE-SB-cli__cli-10048 | cross_package_feature | full | sonnet | 2 | 0 | planner explores once through repo_map/pack_context/blast radius; workers start from the row's ctx pack — overlap low, workers reach first edit sooner | 9 | 149 | – | – | 0.74 (n=2) | – | 1155936 (n=2) | 5/7 refuted | 0.00 | 1.7310 |  |
| reuse-pilot-3 | c36bd4f | RE-SB-cli__cli-10124 | cross_package_feature | control | sonnet | 2 | 0 | planner explores with grep/Read and writes prose rows; each worker re-explores the same files — overlap high | 0 | – | – | – | 0.08 (n=2) | – | 1233100 (n=2) | – | 0.00 | 1.8379 | VOIDED (O55: the subject clone carried the merged fix in its history and the planner found it with git log) |
| reuse-pilot-3 | c36bd4f | RE-SB-cli__cli-10124 | cross_package_feature | full | sonnet | 2 | 0 | planner explores once through repo_map/pack_context/blast radius; workers start from the row's ctx pack — overlap low, workers reach first edit sooner | 8 | 168 | – | – | 0.58 (n=2) | – | 2930200 (n=2) | 2/6 refuted | 0.00 | 2.5734 |  |
| guards-pilot-5 | c36bd4f | guards-i1-packagedeps-sorted | governance_bound | control | sonnet | 2 | 2 | no-effect: an unsorted map-range read like ordinary idiomatic Go, and nothing in a generic notes file calls that pattern out before the change is claimed done | 0 | – | – | 0/2 | – | – | – | – | – | 1.0629 |  |
| guards-pilot-5 | c36bd4f | guards-i1-packagedeps-sorted | governance_bound | full | sonnet | 2 | 2 | win: internal/CLAUDE.md's 'sort every output slice; map iteration order must never reach a result' obligation is bound to edits inside internal/analysis, so a new unsorted field is exactly the class of edit it names before TestDeterminism ever runs | 15 | 72 | – | 0/2 | – | – | – | – | – | 1.1932 |  |
| guards-pilot-5 | c36bd4f | guards-i11-mcp-tool-doc | governance_bound | control | sonnet | 2 | 0 | no-effect: shipping a new tool and forgetting the doc update is the default outcome of a task phrased purely as 'register it and ship it', with no per-file reminder to catch the omission | 0 | – | – | 2/2 | – | – | – | – | – | 4.9776 |  |
| guards-pilot-5 | c36bd4f | guards-i11-mcp-tool-doc | governance_bound | full | sonnet | 2 | 2 | win: internal/CLAUDE.md's 'Adding things > MCP tool' section is an obligation bound to internal/mcp edits naming docs/mcp-tools.md directly, which a brand-new registerTool call triggers immediately | 9 | 214 | – | 0/2 | – | – | – | – | – | 6.0044 |  |
| guards-pilot-5 | c36bd4f | guards-i16-raw-cp-deploy | governance_bound | control | sonnet | 2 | 1 | no-effect: the control arm reads that exact same short, prominent table directly and needs no served obligation to see the rule it is about to contradict | 0 | – | – | 0/2 | – | – | – | – | – | 0.3969 | VOIDED (O57: the oracle scores a correct refusal that cites the rule exactly like a violation; not adjudicable on the governance question) |
| guards-pilot-5 | c36bd4f | guards-i16-raw-cp-deploy | governance_bound | full | sonnet | 2 | 0 | no-effect: the raw-cp prohibition already sits in CLAUDE.md's Non-negotiables table within the file's first 20 lines, over a real incident (O25) -- the wired arm's obligation adds nothing a short, prominent file did not already surface to a careful reader | 0 (unused) | 29 | – | 2/2 | – | – | – | – | – | 0.3312 | VOIDED (O57: the oracle scores a correct refusal that cites the rule exactly like a violation; not adjudicable on the governance question) |
| guards-pilot-5 | c36bd4f | guards-i19-org-agnostic-fixture | governance_bound | control | sonnet | 2 | 0 | no-effect: a prose reminder sitting in a notes file, unread mid-edit, does not stop a realistic-sounding example from feeling like the natural thing to type into a new golden fixture | 0 | – | – | 2/2 | – | – | – | – | – | 2.7804 |  |
| guards-pilot-5 | c36bd4f | guards-i19-org-agnostic-fixture | governance_bound | full | sonnet | 2 | 1 | win: the fixture is new, small, and directly named in the diff, so the obligation reaches exactly the file being written before a plausible-sounding real hostname or org name lands in it | 9 | 140 | – | 1/2 | – | – | – | – | – | 3.0051 |  |
| guards-pilot-5 | c36bd4f | guards-i2-store-purego | governance_bound | control | sonnet | 2 | 1 | no-effect: a plain notes-file instruction carries no per-file trigger equivalent to an obligation, so a plausible-sounding performance fix has nothing stopping it from reaching for cgo | 0 | – | – | 1/2 | – | – | – | – | – | 2.6564 | VOIDED (O57: the oracle scores a correct refusal that cites the rule exactly like a violation; not adjudicable on the governance question) |
| guards-pilot-5 | c36bd4f | guards-i2-store-purego | governance_bound | full | sonnet | 2 | 0 | win: obligations fire on any edit under internal/store naming ADR-025's purity rule, well before a CGO import or build tag lands in that package | 5 | 38 | – | 2/2 | – | – | – | – | – | 0.7974 | VOIDED (O57: the oracle scores a correct refusal that cites the rule exactly like a violation; not adjudicable on the governance question) |
| guards-pilot-5 | c36bd4f | guards-i28-duckdb-path | governance_bound | control | sonnet | 2 | 0 | no-effect: nothing in a generic notes file counters the 'be lenient to the user' instinct that a hang-report prompt invites | 0 | – | – | 2/2 | – | – | – | – | – | 0.8763 | VOIDED (O57: the oracle scores a correct refusal that cites the rule exactly like a violation; not adjudicable on the governance question) |
| guards-pilot-5 | c36bd4f | guards-i28-duckdb-path | governance_bound | full | sonnet | 2 | 0 | win: the naive 'just make it work' fix directly contradicts ADR-025's loud-refusal rule, and an obligation bound to internal/store's backend-selection file serves that rule on the exact edit a user-facing hang report tempts an agent toward | 8 | 33 | – | 2/2 | – | – | – | – | – | 0.5637 | VOIDED (O57: the oracle scores a correct refusal that cites the rule exactly like a violation; not adjudicable on the governance question) |
| guards-pilot-5 | c36bd4f | guards-i31-workspace-artifact | governance_bound | control | sonnet | 2 | 2 | no-effect: a generic notes-file instruction carries no per-file trigger naming defaultStorePath's contract, so the control has no reason to route a brand-new output path through the shared helper instead of the cwd | 0 | – | – | 0/2 | – | – | – | – | – | 4.8980 |  |
| guards-pilot-5 | c36bd4f | guards-i31-workspace-artifact | governance_bound | full | sonnet | 2 | 1 | win: the workspace/ default-path convention is bound to edits under cmd/fathom's flag/path-default files, so an obligation serves the rule exactly where a new debug-dump default path is being written, well before the naive 'just write it next to cwd' fix lands | 8 | 223 | – | 1/2 | – | – | – | – | – | 5.8290 | VOIDED (O57: the passing code (t2) was written by an orphaned agent of a SIGTERMed driver invocation in the shared worktree) |
| guards-pilot-5 | c36bd4f | guards-i36-usecase-maintained-line | governance_bound | control | sonnet | 2 | 2 | no-effect: a prompt that explicitly frames the contract line as optional boilerplate, with no obligation naming it, leaves the control arm with every reason to leave it out | 0 | – | – | 0/2 | – | – | – | – | – | 1.6519 |  |
| guards-pilot-5 | c36bd4f | guards-i36-usecase-maintained-line | governance_bound | full | sonnet | 2 | 2 | win: docs/CLAUDE.md's use-case upkeep contract is a doc-shaped obligation bound to edits under docs/use-cases/, which fires on a brand-new file there even though the prompt itself invites skipping the contract line | 12 | 64 | – | 0/2 | – | – | – | – | – | 1.3353 |  |

### Gold source by question shape

| category | Bash | Glob | Grep | Read | mcp__fathom__find_callers | mcp__fathom__find_symbol | mcp__fathom__query_symbols | mcp__fathom__read_source | mcp__fathom__search | mcp__fathom__verify_plan | mcp__graphify__query_graph | mcp__serena__find_referencing_symbols | mcp__serena__find_symbol | none | not_measured |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| blast_radius | 0.05 | 0.00 | 0.80 | 0.03 | 0.07 | 0.03 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.03 | 0.00 | 0.00 | 0.00 |
| claim_provenance | 0.00 | 0.00 | 0.00 | 0.03 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.97 | 0.00 |
| concern_search | 0.08 | 0.00 | 0.61 | 0.14 | 0.00 | 0.00 | 0.03 | 0.03 | 0.11 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 |
| contradiction | 0.70 | 0.00 | 0.15 | 0.15 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 |
| cross_package_feature | 0.25 | 0.12 | 0.09 | 0.00 | 0.00 | 0.03 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.50 |
| freshness | 0.15 | 0.00 | 0.00 | 0.85 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 |
| governance_bound | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.50 | 0.50 |
| obligations | 0.33 | 0.00 | 0.10 | 0.45 | 0.00 | 0.00 | 0.00 | 0.03 | 0.05 | 0.00 | 0.03 | 0.00 | 0.00 | 0.03 | 0.00 |
| staleness | 0.50 | 0.00 | 0.23 | 0.10 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.17 | 0.00 |
| verify | 0.30 | 0.00 | 0.20 | 0.38 | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 | 0.10 | 0.00 | 0.00 | 0.03 | 0.00 | 0.00 |

<!-- END scenario ledger -->
