# The bench pyramid — measure the index for free, spend on the agent only where it can change

> Status: **ACCEPTED 2026-09-12 as ADR-032** (proposed 2026-09-03). Evidence: `run-20260902T211103Z` (17 of 117
> cells before the run was stopped), observations O23–O26, the July runs
> (`home-bench.md` H3-1, `trust-and-tokenomics.md` T5-8), `docs/bench/README.md`.
> Governing intent: `docs/vision/26-proof-credibility.md`, ADR-031 (bench
> credibility), ADR-027 (an instrument may not degrade toward healthy),
> `docs/intent/autonomy.md` (ask before LLM spend). No decision record yet —
> this document is the case for one.

## 1. What seventeen cells taught us

The first replicated run was stopped at cell 17. It had already produced four
findings that no earlier run could have, because every earlier run was n=1
and none carried transcripts.

| | Finding | Observation |
|---|---|---|
| **E1** | The obligations tool, asked what governs `internal/extractor/goextract.go`, served six plan-row spec-bindings about C++, CUE and tree-sitter and displaced 65 more. The `internal/CLAUDE.md` rule the file is actually governed by was not among them. The fathom arm passed the task anyway — by reading `internal/CLAUDE.md` through `read_source`, which is grep's move. **Task-level success hid tool-level failure.** | O23 |
| **E2** | Every fathom cell opens with one or two `ToolSearch` turns to load deferred MCP schemas. Claude Code 2.1.x defers MCP tools; `Bash` is core and is never deferred. July's transcripts contain zero `ToolSearch` calls. The treatment arm now pays an asymmetric harness tax of ~28k tokens per deferred load. | O24 |
| **E3** | H1-T02's ground truth is verbatim in the root `CLAUDE.md`, which every `claude -p` session auto-loads. The `none` arm answered it with no tools in 5.5 s. The committed leak rule excluded it — after six tool-arm cells had been spent on it. | O25 |
| **E4** | Per cell: input 4–12 tokens, output 400–1,600, cache-read 28k–180k. `total_tokens` is cache-read × turns. The 3.1× spread on identical H1-T02 fathom trials is 3 vs 6 vs 5 turns. The payload fathom claims to shrink is invisible under a ~28k/turn baseline both arms re-read. | O26 |

E1 is a product defect. E2 is a harness change that invalidates a cross-run
comparison. E3 and E4 are instrument defects. The bench found all four in
nine minutes and roughly 1.5M tokens — which is the strongest argument that
the *method* is sound and the *shape* is wrong.

## 2. What the current bench is, and is not

The context-bench was built as a falsification instrument for one claim
(×10 tokenomics), and it is a good one: pre-registered, negative-capable,
hand-verified truth, honest degradation. Everything in `docs/bench/README.md`
stands.

As the *only* bench it has four structural limits:

1. **It measures agent × harness × index, and the harness term now
   dominates** (E2, E4). A verdict can flip on a Claude Code release.
2. **It can only test what grep can also do.** Fairness rule 3 excludes
   store-only capabilities; that is correct for a control-arm bench and
   means it structurally cannot measure fathom's differentiated value.
3. **Thirteen author-written tasks, near ceiling.** With 13/13 vs 11/13,
   "strictly more successes" is one draw from a coin. The bootstrap CI will
   say so, but a criterion whose primary metric cannot discriminate is a
   criterion in name.
4. **~$15 a run means it runs quarterly.** An instrument that cannot run on
   a commit cannot guide engineering; it can only audit it.

None of these is fixed by running the same bench harder. They are fixed by
putting cheaper, sharper instruments under it and using the expensive one
only for the question only it can answer.

## 3. The pyramid

Six layers. Each measures one thing, at one cost, on one trigger. A layer
never re-asks a question a cheaper layer already answered.

| Layer | Question | Oracle | LLM cost | Runs on | Exists today |
|---|---|---|---|---|---|
| **L0 Index oracle** | Is the index *right*? callers / references / definitions / blast-radius closure vs `go/types`; extraction coverage per language | `cmd/loctruth`, the lang-truth calibration corpus | 0 | every commit, in `task validate` | **wired (VB-6)**: `IndexOracleGoldens`/`TestIndexOracleGoldens` gate `internal/indexbench/testdata/l0-goldens.json` as `task bench-l0-gate` |
| **L1 Payload replay** | Is each tool's answer *small and stable*? bytes, result count, latency per (tool, fixed query) against a pinned store | committed golden responses | 0 | every commit | **wired (VB-7)**: `PayloadReplay`/`TestPayloadReplay` gate `internal/indexbench/testdata/l1-replay.json` as `task bench-l0-gate` |
| **L2 Corpus gate** | Is the corpus *clean*? truth at HEAD; no truth token in any auto-loaded file; prompts name the target, never the governing file | static walk | 0 | every commit | `TestHomeCorpusTruthAtHead` exists; **baseline-absence check missing** |
| **L3 Targeted agent** | Did *this change* move the tasks it could move? fathom arm only, tasks selected by the tools the diff touched | H2-1 / H2-3 per task, no verdict | 1–4M | a commit touching an MCP tool handler | **missing** (needs `exercises:` per task, `--only`, composed rows) |
| **L4 Full agent** | Does fathom beat grep on its home turf at n≥3? | H2-1 (history), H2-3 (registered next) | ~15M | release tag; corpus, criterion or surface change | exists (BC-14 in flight) |
| **L5 Sessions** | Do *real sessions* go better with the channels on? | A/B arm, retro corpus, verify-gate catches | ~0 (ambient) | continuously | instrumented; ON arm has 0 admissible units |

L0 and L1 are where fathom's own quality lives, and both are free. L4 is the
only layer that needs a control arm, and the only one that answers "versus
grep". L5 is the only layer that answers "is it worth it".

### 3.1 L0 — the index oracle bench

`cmd/loctruth` already computes non-test direct callers with `go/types` for
the public corpus (PB-16). The same tool, over fauxtown plus two or three
pinned public checkouts, gives an independent truth for `find_callers`,
`references` and the transitive closure `blast_radius` walks. Score
precision and recall per repo per tool; commit the numbers as goldens;
regress in `validate`. A missed edge is a failing test, not a benchmark
result a Sonnet agent might or might not notice.

Extraction coverage per language belongs here too, on the calibration
corpus the language program already built.

### 3.2 L1 — payload replay

For each read-only tool, a committed list of (service, args) against a
pinned fixture store, with golden `bytes`, `result_count` and a latency
budget. `obligations` on the fixture fleet's files is the first entry:
O23's `displaced: 65` would have been a red number in CI three weeks ago.

### 3.3 L2 — the corpus gate

`TestHomeCorpusTruthAtHead` walks every ground-truth token and asserts
presence at its anchor. Extend the same walk to assert **absence** from
every file the harness auto-loads — root `CLAUDE.md`, `AGENTS.md`, and the
user-memory index if present — and to refuse a prompt that names the
governing file (E1's task told the agent to look in `internal/CLAUDE.md`,
which is the answer). Both are free and turn the `none` arm from a screen
into a confirmation.

### 3.4 L3 — targeted runs and composed rows

Each task declares `exercises: [obligations, read_source]` — the tools it
was written to exercise. The selector maps a diff to changed MCP handlers
(fathom's own `references` over `internal/mcp` does this) and from handlers
to tasks. The runner gains `--only <ids>`.

A targeted run writes **per-task rows**, never a verdict: `scope:
targeted`, and `Criterion.Evaluate` refuses a targeted scope exactly as it
refuses an absent task today (indeterminate, reason named). `regress`
compares per-task and already ignores tasks absent from the baseline.

**Control-arm invariance is the big saving.** The grep and `none` arms do
not depend on fathom's code. Their cells are valid for as long as
`corpus_sha256`, `model_id`, and the `truth_at_head` anchor files are
unchanged. A composed row borrows them with provenance (`cells[].run_id`,
`cells[].borrowed: true`) and the evaluator checks the three invariants
before accepting a borrowed cell; a violated invariant makes the row
indeterminate with the cell named, never a silent reuse.

Typical cost of a fathom-side change: the tasks its tools touch (call it 5
of 13) × 3 trials × fathom arm only ≈ 15 cells ≈ 2M tokens, against 15M for
a full run.

### 3.5 L4 — the criterion the next run is scored on

H2-1 stands as history and is never re-scored. Register **H2-3** beside it
before the next full run, with the same success bar and a different cost
half:

- `payload_bytes` — the sum of tool-result bytes returned to the model per
  cell. This is the discovery cost fathom controls.
- `turns_excl_toolsearch` — turns minus deferred-schema loads. Reported
  separately as `toolsearch_turns` so the harness tax is a visible line,
  never a hidden penalty or a hidden excuse.
- `cost_usd` — the number a buyer sees; cache-read pricing is what makes
  July's "1.38× fewer tokens" and "$2.43 vs $2.46" both true.
- `gold_source_tool` — the tool whose result first contained a gold token.
  A cell passes the task; a tool passes only if it is the gold source. E1
  becomes a number: obligations was the gold source on 0 of 3 T01 trials.

The token half of H2-3 reads `payload_bytes` and `cost_usd`; `total_tokens`
is recorded, not gated.

### 3.6 Gates

| Trigger | Runs |
|---|---|
| any commit | L0, L1, L2 (in `validate`) |
| commit touching `internal/mcp/**` or `internal/semantic/**` | L3 for the tasks those handlers exercise |
| corpus revision, criterion registration, model change | grep + `none` arms refreshed (invariants broke) |
| release tag, tool added to or removed from the surface | L4, full, three arms, n=3, verdict |
| always | L5 |

## 4. What the results say about fathom, not the bench

**D1 — split the question the obligations tool answers.** "What governs
this file" (human-written `CLAUDE.md`/ADR rules) and "what plan rows claim
this file" (spec-bindings) are different questions with different readers.
Serving them through one ranked list is what let four C++ plan rows
displace the one rule that mattered. Either two tools, or a `layer:` filter
with `obligations` as the default and spec-bindings opt-in.

**D2 — scope plan rows by contract, not prose.** A plan row carries
`{symbol, package}`. Binding it to files by the word "extractor" in its text
is the O14 defect at fleet scale: 71 rules on one Go file. Bind by the
package the row names; a row with no contract binds to nothing.

**D3 — inline the cited lines.** The agent round-tripped `read_source` on
the rule's citation in 5 of 6 fathom cells that got a useful rule. A rule
that carries its cited lines (capped, like `symbol_context`'s `snippet`)
saves one turn per rule the agent trusts — and the T5-6 pattern already
exists for exactly this.

**D4 — the deferral tax is a user cost, not just a bench cost.** Every
session's first fathom call pays a schema load. Find out whether the server
or the plugin manifest can mark the hot-loop tools non-deferred, or whether
a smaller default surface avoids deferral; measure it on the exploration
ledger, where raw-vs-indexed already shows agents choosing grep.

**D5 — stale rules are served as top results.** Both T02 payloads led with
`status: stale` rules. Fresh-first ranking is a one-line change with a
visible effect on every edit hook payload (M3's 47% over-budget figure is
partly stale rules taking slots).

## 5. What does not change

- H2-1's recorded verdicts. History is never re-scored.
- The replication floor, the leak rule, the fairness rules, the ledger
  append-only discipline, the ask-before-spend rule.
- The home corpus's role: it stays the L4 corpus after the L2 gate prunes
  it. The public corpus (PB-19) stays the reproducibility corpus.

## 6. Order of work

1. L2 baseline-absence test and the prompt rule — free, unblocks a clean
   L4 run.
2. H2-3 registered in the corpus; `payload_bytes`, `toolsearch_turns`,
   `gold_source_tool` extracted from the transcripts the runner already
   captures with `--stream`.
3. Finish BC-14 under H2-1 (history) and score the same transcripts under
   H2-3 offline — one spend, two criteria.
4. L0 oracle bench over fauxtown; goldens committed; in `validate`.
5. L1 payload replay, `obligations` first.
6. D1/D2/D5 on the obligations layer, then an L3 run on the obligations
   tasks only — the first targeted run, and the first time a fathom change
   is benchmarked for ~2M tokens instead of 15M.
7. `exercises:` on every task, `--only`, composed rows.

## 7. Revisit when

- A Claude Code release removes or changes MCP tool deferral (E2 disappears
  or changes shape; H2-3's `toolsearch_turns` goes to zero and the line
  item should be retired, not silently kept).
- L0 reports precision or recall under 0.95 on any pinned repo — the index,
  not the agent, is the bottleneck and L4 should not run until it is fixed.
- The `none` arm passes more than half the corpus (ADR-031's own trigger):
  the corpus, not the tool, is the finding.
- Two same-corpus L4 rows exist and `regress` has compared them — the
  composed-row design either earned its complexity or did not.
