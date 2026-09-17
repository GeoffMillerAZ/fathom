# LCA run 1: instrument-truth reading

Run `lca-run-1`, Long Code Arena bug-localization corpus (40 tasks, Python/Java/Kotlin +
one Go and one Shell file each in an otherwise non-Python repo), Sonnet, single-session,
15-turn cap, 3 trials/arm, `-parallel-arms`. Two driver invocations: 00:24:05Z (aborted,
`exit=2`, disk full) and 05:30:12Z (completed, `exit=1` because 6 cells stayed lost).
This reading covers the aggregate ledger (verified against the raw log, exact match) plus
a hand-audited sample of 10 of the 39 scoreable tasks (60 of 234 cells, 25.6%). It is
about **instrument truth and mechanism** — was the tool actually wired, called, and did
its result drive the answer — not a re-score of hit@1/recall, which the oracle already
computed deterministically.

## 1. Plain-language verdict (VB-27 / BC-15)

**Is there lift? Barely, and it does not survive the mechanism check. Is there a cost?
Yes, a real one.**

Aggregate: fathom passed 94/117 scored cells (80.3%), control 93/117 (79.5%) — a
one-cell, 0.8-point edge. Fathom cost 29% more ($13.54 vs $10.49) and used 64% more
tokens (27.4M vs 16.7M) to get there. At the task level — the unit VB-27's paired
statistic actually uses — fathom did better on 7 of 39 comparable tasks, worse on 6, and
tied on 26. That is close to a coin flip, and every non-tied task differs by only ±1 or
±2 passes out of 3.

Reading the mechanism behind the extremes narrows this further, not wider:

- Of fathom's two biggest wins (`bumble-tech/appyx`, +2; `rapptz/discord.py`, +2), only
  one of six winning fathom trials actually used a fathom tool's own result as the
  answer's evidence (a `mcp__fathom__search` hit on `appyx`); the discord.py "win" traces
  to `find_symbol` genuinely surfacing the gold file first in all three trials, but the
  recorded 0-vs-2 control tally is itself partly a scoring-harness parser bug (§5) that,
  fixed, would make it 1-vs-2 or better for control.
- Of fathom's two biggest losses (`dokku/dokku`, −2; `theopenconversationkit/tock`, −2),
  every fathom FAIL is a session that had already located the gold file — via plain
  Bash/Grep in dokku, via fathom's own tools in tock — and then burned its 15-turn budget
  on redundant re-verification or (in tock) a broken Kotlin index, hitting
  `error_max_turns` before ever emitting a scoreable answer. Zero fathom tool calls
  occurred in any of dokku's three fathom trials despite MCP being connected.
- `pypa/pip` (−1) shows the same shape again: fathom's extra ~20 no-op hook round-trips
  plus a 25-tool schema pushed two of three trials past the same fixed turn cap that
  control's leaner 3-tool surface did not hit, even though the fathom sessions had
  already found the same evidence the passing control sessions used.

So the raw one-cell aggregate edge is consistent with noise plus a turn-budget artifact
that cuts against fathom about as often as scoring artifacts cut against control. This
run does not establish lift for fathom's tool surface on this task shape at this turn
budget.

**Paired 95 % CI (operator bootstrap over the 39 paired tasks, seed 1, 5000 resamples): mean success difference fathom − control +0.009, CI [−0.077, +0.094] — includes zero; token ratio fathom/control 2.00, CI [1.71, 2.34]** — computed here only as raw
counts (7 better / 6 worse / 26 tie of 39 tasks); the bootstrap itself is the operator's
step (`scenariobench ledger` + M12/M16). Given how thin and symmetric the task-level
diffs are, expect an interval that includes zero.

## 2. Per-arm table

| | control | fathom | ratio |
|---|---|---|---|
| Scored cells | 117 | 117 | — |
| Passes | 93 | 94 | +1 |
| Hit@1 rate | 79.5% | 80.3% | +0.8 pt |
| Cost | $10.49 | $13.54 | 1.29x |
| Tokens | 16,709,757 | 27,397,095 | 1.64x |
| Cost / session | $0.0897 | $0.1157 | 1.29x |
| Tokens / session | 142,819 | 234,163 | 1.64x |
| Cost / success | $0.1128 | $0.1440 | 1.28x |
| Tokens / success | 179,675 | 291,458 | 1.62x |
| Paired tasks (of 39) | better 7 · worse 6 · tie 26 | | |
| Fully-lost tasks | 1 (`apache/eventmesh`, all 6 cells) | | |

Both per-arm totals (93/117, 94/117, cost, tokens) were recomputed directly from the raw
log and match the given figures exactly, resolving the two "borrowed" cell blocks (run 1's
clean `fasterxml/jackson-module-kotlin` cells carried into the run-2 ledger unchanged, and
`square/moshi/791/775/control/t1` carried the same way).

By language (scored cells only; n small per cell, read as texture not signal):

| language | tasks | control | fathom |
|---|---|---|---|
| Java | 8 | 22/24 (92%) | 22/24 (92%) |
| Kotlin | 12 | 30/36 (83%) | 31/36 (86%) |
| Python | 19 | 38/51 (75%) | 39/51 (76%) |
| Go (1 file in docker/compose) | 1 | 0/3 (0%) | 1/3 (33%) |
| Shell (1 file in dokku/dokku) | 1 | 3/3 (100%) | 1/3 (33%) |

Go and Shell are single tasks each (n=3 trials), not language-level signal — they are the
`docker/compose` and `dokku/dokku` cells discussed in §1 and §5, both dominated by
turn-budget and answer-formatting mechanics rather than by language support.

## 3. Instrument truth (from the 10-task, 60-cell sample)

**MCP wiring and hooks — clean, no leakage across arms.** All 30 sampled fathom cells
show `mcp_connected:true` and every fathom hook (`SessionStart`/`UserPromptSubmit`/
`PreToolUse`/`PostToolUse`, `Stop` where the session survived to a stoppable turn) firing.
All 30 sampled control cells show no MCP config and zero hooks. This is the expected,
correct arm separation — not a finding, a confirmation.

**Fathom tools were called in most, but not all, connected trials — and drove the
answer in fewer still.** Across the 30 fathom cells: a `mcp__fathom__*` tool was called
in 22 (73%); in the other 8 — all three `dokku/dokku` trials, all three
`archivebox/archivebox` trials, `pypa/pip` trial 3, and `beetbox/beets` trial 2 — the
session never touched the MCP surface despite being connected, and reached its answer (or
ran out of turns) on plain Bash/Grep/Read alone. Of the 22 trials that did call a fathom
tool, the tool's own result was the thing that named the gold file first
(`gold_source_tool` = an `mcp__fathom__*` tool) in only 9 — 30% of all 30 fathom cells,
41% of the 22 that actually called a tool. The rest called a fathom tool, got a miss or an
empty result, and fell back to native Grep/Bash, which is what actually found the file.
Tool mix in the sample: `mcp__fathom__search` (grep-with-receipt) and `find_symbol`
dominate; `query_symbols`, `outline`, `symbol_context`, and `localize` each appear once or
twice. No sampled trial called `references`, `blast_radius`, `verify_plan`, or
`obligations` — unsurprising, since every task here is read-only localization with no
edits, so the edit-triggered and governance tools have nothing to attach to.

**Tree-sitter-only indexing: functional for Python, degraded for Kotlin in this
sample.** Java/Kotlin/Python get no typed call-graph pass in this indexer (Go-only per
project docs), so every `tier` reported here is `verified` (a tree-sitter-confirmed
definition site) or `heuristic`, never a resolved call edge. Python held up:
`find_symbol` resolved correctly at `verified` tier in `rapptz/discord.py` (1,664
symbols), `pypa/pip` (11,261 symbols), `beetbox/beets`, and `edgedb/edgedb` — the last
case correctly disambiguating a same-named symbol split across a Rust struct and a Python
class. Kotlin did not: 3 of the 4 sampled Kotlin-repo tasks show a broken or
near-empty Kotlin index — `bumble-tech/appyx` reports only 165 symbols for a real Kotlin
app; `theopenconversationkit/tock`'s `session_start_context` mislabels the repo's primary
language as **javascript** (it is Kotlin), `find_symbol` returns `did_you_mean` with zero
Kotlin candidates in all three fathom trials, and `outline` returns an empty symbol list
for a real, non-empty `.kt` file; `square/kotlinpoet` reports only 131 symbols tagged
`java/javascript` with the Kotlin source apparently unindexed, again producing
`find_symbol` misses in all three trials. Only the grep-backed `mcp__fathom__search` tool
(not the symbol-aware tools) reliably found anything in these three repos. Whether this is
a per-task scratch-daemon ingestion defect specific to this bench harness or a genuine gap
in the tree-sitter Kotlin grammar is not established by this sample — worth an ingest-log
check on one of these three repos before drawing a language conclusion.

**Other instrument defects surfaced in the sample (harness/scorer, not the tools'
localization quality):**
- `mcp__fathom__localize` returned an 882,577-character single payload in
  `tock`-fathom-t2, blowing through the CLI's own max-tool-output limit and forcing a
  disk-spill-and-reread — the tool's own "budgeted" framing did not hold.
- The same session later saw `mcp__fathom__search` fail outright with "the daemon is up,
  but this repo is not resolvable as an indexed service" moments after an earlier search
  against the identical service had succeeded — an index-registry stability question for
  runs using `-parallel-arms`, not investigated further here.
- Two defects live in the scoring harness's `ParseAnswerFiles`
  (`internal/scenariobench/localization.go`), not in fathom: its numbered-list regex does
  not match Claude's own common `"1. **`path`**"` markdown (bold-wrapped backtick span),
  silently discarding the model's explicit ranking in favor of noisy prose-order fallback
  in at least the `rapptz/discord.py` cell (affecting both a fathom and a control trial
  there); and a bare-filename allow-list credits a colloquial repo name that happens to
  carry a code extension (`discord.py`) as an answered file, which cost `discord.py`
  fathom-t1 its hit@1 despite the model's own list correctly ranking the real fix file
  first. Neither is fathom-specific, but both are corpus-consequential and neither is
  fixed as of this run.
- `gold_source_tool` (the diagnostic naming which tool "found" the file first) has two
  known false-attribution modes that do **not** affect the real hit@1/recall scoring:
  crediting a later `Read` when an earlier Grep used an absolute `path` param (so its
  output never literally contains the full gold path string), and crediting `Bash` when a
  plain `ls`'s visible bare-filename output happens to sit next to an OSC-8
  terminal-hyperlink escape code that does contain the full path. Treat this field as
  "first *structured* naming," not "first access."

**Leaks: none found.** Every sampled session's tool calls (Bash, Grep, Read, and every
`mcp__fathom__*` call) resolved to that trial's own worktree; each arm's three trials for
a task share one worktree per arm (safe here because every task is read-only, zero
Edit/Write calls anywhere in the sample). The only absolute paths appearing in any raw
transcript were benign: Claude Code's own >36KB tool-output spill path under
`~/.claude/projects/...`, OSC-8 hyperlink escape codes wrapping an `ls` listing's bare
filenames, and the harness's own scratch-directory name (containing "sextant") — none of
these named a gold file across the arm boundary or exposed scoring internals (no session
in the sample emitted the strings `gold`, `bugloc`, `answered_files`, or `hit_at_1`).

## 4. The 12 LOST cells

**6 cells, `apache/eventmesh/3432/3431`, both arms, all 3 trials each** — instrument:
`preparing subject: base_commit 8aa97ac2df2fde85e8fbd5b0c51132f41f6447f4 not found in
subject repo after fetch: exit status 128`. This is a bad/rewritten base SHA in the
dataset, not a driver bug; it was never re-run, so the task is dropped whole from both
arms — 39, not 40, tasks are paired, and 117, not 120, cells are scored per arm. Symmetric
across arms (both lost identically), so it does not bias the comparison, but it does mean
the corpus actually delivers one fewer Java data point than advertised.

**6 cells, `tootallnate/java-websocket/329/259`, both arms, all 3 trials each** — lost in
run 1 only (00:24Z): `git clone` failed with `zeroPaddedFilemode: contains zero-padded
file modes` / `fatal: fsck error in packed object` / `fatal: fetch-pack: invalid
index-pack output` — an old-repo git-object quirk tripping the clone's own fsck, not an
instrument or driver defect. Recovered cleanly on the relaunch (05:30Z): control went
3/3 PASS, fathom went 1 FAIL + 2 PASS — a real, freshly-run task outcome, not a residual
instrument failure. Both arms lost and recovered identically (symmetric), but two things
are worth flagging as texture, not bias: (1) the same run-1 abort that left these 6 cells
lost also hit a *separate* fatal error moments later — `square/moshi/791/775/fathom/t1`
failed to write its transcript with "no space left on device," which is what actually
ended run 1 (`exit=2`) — and (2) the relaunch carried forward run 1's already-clean
`fasterxml/jackson-module-kotlin` cells unchanged ("borrowed") while java-websocket and
`square/moshi/...control/t2` needed genuinely fresh sessions in run 2, meaning the
corpus mixes cells executed ~5 hours apart (00:24Z vs 05:30Z) without any check in this
reading for whether that gap correlates with anything (model load, routing). Unknown, not
ruled out.

No cell outside these 12 is marked LOST in the sampled task data; the "12 LOST" set is
exhaustive for this run.

## 5. What this corpus can and cannot say

**Can say:** the exact aggregate counts (93/117 vs 94/117, cost, tokens) are solid — they
were independently recomputed from the raw log and match to the cell. The MCP/hook wiring
is clean and correctly separated by arm with no cross-contamination in every cell checked.
No gold-file or scoring-internals leak was found anywhere in the sample.

**Cannot yet say:** that fathom's tool surface produces a real localization lift on this
task shape. Three things stand in the way, in addition to the mechanism findings in §1:

1. **Ceiling / floor effects mean a lot of the 26 ties are ties by construction, not by
   tool equivalence.** Combined hit@1 across both arms is 187/234 = 80% — close to the
   ~80% figure the brief flagged as a plausible ceiling for Sonnet-plus-grep on file-level
   localization. Some sampled ties are genuinely uninformative: `archivebox/archivebox`'s
   gold file is spelled out verbatim in the issue's own GitHub blob URL (0-1 tool calls,
   both arms 3/3), and `edgedb/edgedb` hit a hard `max_turns=15` cap in **all six** trials
   before either arm ever produced a scoreable answer (both arms 0/3) — a null result from
   budget exhaustion, not evidence the arms performed equally. A corpus with more tasks in
   the discriminating middle (hard enough that plain grep sometimes fails, easy enough
   that a 15-turn budget is not the binding constraint) would separate the arms better
   than this one does.
2. **Turn budget interacts with fathom's added overhead in a way this run does not
   correct for.** Fathom sessions carry ~19-27 hook round-trips and a 25-tool schema that
   control's 3-tool surface does not; against a fixed 15-turn cap, this pushed at least
   three of fathom's six "worse" tasks (`dokku`, `tock`, `pip`) into `error_max_turns`
   after the session had already found the same evidence a passing control session used.
   Whether raising the cap, trimming fathom's hook chatter, or reporting turns-to-answer
   separately from pass/fail would change the task-level tally is untested here.
3. **The sample is 10 of 39 tasks (26%), hand-picked to include the extremes (the two
   biggest wins, the two biggest losses, and a spread of ties/near-ties).** The aggregate
   counts are exact; the *why* behind the other 29 tasks' outcomes is unknown. A
   systematic pass over all 234 cells — even just extracting `gold_source_tool`,
   `fathom_tools_called`, and `hooks_fired`/`mcp_connected` mechanically — would tell you
   whether the patterns found here (Kotlin-index gaps, turn-budget asymmetry, the two
   `ParseAnswerFiles` bugs) generalize or were cherry-picked by the sampling.

**Explicitly unknown, not investigated further:**
- Whether the `ParseAnswerFiles` markdown-list and bare-filename-allowlist bugs are
  corpus-wide (they would move the task-level diff distribution, not just the
  `rapptz/discord.py` and `docker/compose` cells they were caught in) — fixing them and
  rescoring the full 234 cells has not been done.
- Whether the Kotlin indexing gaps (mislabeled primary language, near-zero symbol counts)
  are a per-task scratch-daemon ingestion defect or a genuine tree-sitter-Kotlin
  extraction gap — no ingest log was read to distinguish these.
- Whether the ~5-hour gap between the borrowed run-1 cells and the fresh run-2 cells
  introduced any systematic difference.
- The paired bootstrap CI itself — not computed in this reading; see the placeholder in
  §1. Given the observed 7-better/6-worse/26-tie split and the ±1/±2 magnitude of every
  non-tied task, expect it to include zero, but that is an expectation, not the result.

## What changes before a rerun

- Fix `ParseAnswerFiles`' numbered-list regex to match `"N. **`path`**"` and drop (or
  tighten) the bare-filename allow-list before trusting task-level diffs built on it.
- Decide whether Kotlin's near-empty index in 3 of 12 Kotlin tasks is a bench-harness
  ingestion bug or a real product gap, and fix or document accordingly — right now it is
  silently suppressing whatever lift fathom's Kotlin support might otherwise show.
- Either raise the 15-turn cap, or reduce fathom's per-turn hook/schema overhead, or both,
  so a fixed budget does not structurally cost the fathom arm trials it would otherwise
  win on evidence alone (dokku and tock both show this exact shape).
- Cap or paginate `mcp__fathom__localize`'s payload; investigate the one observed
  mid-session "repo not resolvable as an indexed service" transient under
  `-parallel-arms`.
- Consider filtering the corpus toward tasks where plain grep is not already close to
  100% (drop or flag the archivebox-shaped "gold file named in the issue text" tasks and
  the edgedb-shaped "unsolvable in the turn budget for either arm" tasks), since both
  currently manufacture ties that say nothing about the tool surface.
- Run the full 234-cell mechanical version of this instrument-truth check (not a 10-task
  hand sample) before computing VB-27's paired statistic, so the CI is computed over a
  corpus whose known scorer and indexing defects have either been fixed or explicitly
  priced in.
