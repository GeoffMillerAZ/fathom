# Bench pyramid — free layers in the gate, remembered cells, targeted spend, and the fixes the bench found

Design: `docs/design/bench-pyramid.md`. Decision: ADR-032 (extends ADR-031; H2-1 is
history). Runs that spend tokens stay operator-initiated; everything else here is free
and lands in `task validate`.

Verify: `fathom verify --service fathom --plan docs/plans/bench-pyramid.md`.

## L2 — the corpus gate (free, first)

- [x] `TestHomeCorpusTruthAtHead` also asserts every gold token is ABSENT from every file the harness auto-loads (root `CLAUDE.md`, `AGENTS.md`, `.claude/CLAUDE.md`, the user memory index when present) and refuses a prompt that names the file its own ground truth cites (`corpusBaselineFiles`) {id: BP-1, symbol: corpusBaselineFiles, package: internal/contextbench}
- [x] H1-T02's truth and any other leaked task re-derived so the gate passes without weakening the task; the corpus revision bumped {id: BP-2, skip_test: true, skip: "corpus edit; evidence is TestHomeCorpusTruthAtHead green after BP-1"}

## Remembered cells and targeted runs

- [x] `CellKey(inputs)`: sha256 over corpus_sha256, task id, arm, model id, harness version, scoring, prompt subject and `arm_surface` (the fathom binary commit for the fathom arm, empty for grep and none); every scored cell carries `cell_key` and `arm_surface` {id: BP-3, symbol: CellKey, package: internal/contextbench}
- [x] Content-addressed cell store under `workspace/artifacts/context-bench/cells/<key>/t<n>.json`: the runner writes every scored cell there and, before running a cell, borrows one whose key matches (`BorrowCell`), copying it into the run with `borrowed: true` and `source_run_id` {id: BP-4, symbol: BorrowCell, package: internal/contextbench}
- [x] The ledger row lists borrowed cells with provenance and `Criterion.Evaluate` accepts a borrowed cell only by key equality (`AcceptBorrowed`); a row composed entirely of borrowed cells is indeterminate with the reason named {id: BP-5, symbol: AcceptBorrowed, package: internal/contextbench}
- [x] `exercises: [tool, …]` on every home task (the tools it was written to exercise); `Task.Validate` refuses an unknown tool name {id: BP-6, symbol: TestTasksDeclareExercises, package: internal/contextbench, skip_test: true, skip: "the test is the anchor: it walks tasks-home.json"}
- [x] `contextbenchscore select -diff <git range>`: changed files under `internal/mcp`, `internal/semantic`, `internal/query`, `internal/fusion` map to agent-surface tools by a committed handler table, then to the tasks that exercise them; prints task ids (`SelectTasks`) {id: BP-7, symbol: SelectTasks, package: internal/contextbench}
- [x] Runner `--only <ids>` and `--scope targeted`: a targeted run writes per-task cells and a row with `scope: targeted` that the evaluator refuses to turn into a verdict (indeterminate, reason named); `task bench-targeted RANGE=<git range>` chains select → run {id: BP-8, skip_test: true, skip: "shell + Taskfile; bash -n via scripts-check; the Go side is BP-5/BP-7"}

## H2-3 — the criterion the next run is scored on

- [x] `TranscriptMetrics(stream)`: `payload_bytes` (sum of tool-result bytes returned to the model), `toolsearch_turns`, `turns_excl_toolsearch`, `gold_source_tool` (the tool whose result first carried a gold token, or `answer` when none did); parsed from the `--stream` transcript the runner already captures {id: BP-9, symbol: TranscriptMetrics, package: internal/contextbench}
- [x] H2-3 registered in `tasks-home.json` beside H2-1 and H2-2: same success bar; cost half reads `payload_bytes` ratio ≤1.0 and `cost_usd` ratio ≤1.0; `toolsearch_turns` reported never gated; `LoadCriterionByID("H2-3")` evaluates it and its values are pinned by a test {id: BP-10, symbol: TestCriterionH23Pinned, package: internal/contextbench, skip_test: true, skip: "the test is the anchor"}
- [x] `contextbenchscore ledger -criterion H2-3` scores an existing run's transcripts offline (one spend, two criteria); the row carries per-arm `gold_source_rate` per tool {id: BP-11, symbol: GoldSourceRate, package: internal/contextbench}

## L0 — the index oracle (free)

- [x] `cmd/loctruth` runs over the fixture fleet (`internal/connector/testdata/fauxtown` Go services) and the index built from the same tree; `IndexOracleReport` scores precision and recall for `find_callers`, `references` and the transitive closure `blast_radius` walks, per service {id: BP-12, symbol: IndexOracleReport, package: internal/indexbench}
- [x] Goldens committed (`internal/indexbench/testdata/fauxtown.golden.json`); `TestIndexOracleMatchesGolden` fails on any precision or recall drop; in `task validate` {id: BP-13, symbol: TestIndexOracleMatchesGolden, package: internal/indexbench, skip_test: true, skip: "the test is the anchor"}
- [x] `task bench-l0-self`: the same oracle over this repository at HEAD (slower, on demand and before a release); the report is written under `workspace/` and its precision/recall printed with the denominator {id: BP-14, oracle: lexical, path: Taskfile.yml, pattern: "^  bench-l0-self:$"}

## L1 — payload replay (free)

- [x] `internal/indexbench/replay`: a committed list of (tool, args) against the fixture store with golden `bytes`, `result_count` and a latency budget; `obligations` on fixture files first, then `find_callers`, `references`, `blast_radius`, `symbol_context`, `pack_context`, `repo_map` (`ReplayGolden`) {id: BP-15, symbol: ReplayGolden, package: internal/indexbench}
- [x] `TestPayloadReplayMatchesGolden` in `task validate`; a byte-count growth over 10% or a `displaced` count on any fixture file is a failing test naming the tool {id: BP-16, symbol: TestPayloadReplayMatchesGolden, package: internal/indexbench, skip_test: true, skip: "the test is the anchor"}

## The fixes the bench found (D1, D2, D3, D5 of the design)

- [x] Fresh-first: `ObligationsFor` orders served claims fresh before non-fresh, then by layer role, then specificity, then rule text; pinned by `TestObligationsFreshFirst` {id: BP-17, symbol: TestObligationsFreshFirst, package: internal/semantic, skip_test: true, skip: "the test is the anchor; ObligationsFor already carries its own anchor in earlier plans"}
- [x] Layer roles: `#Layer` gains `role: "governs" | "claims"` (obligations and adr-rules govern; spec-bindings and findings claim); `LayerRole(layer)` reads it; `claims` rules fill only the budget `governs` rules leave {id: BP-18, symbol: LayerRole, package: internal/semantic}
- [x] Contract scoping at serve time: a spec-bindings claim's scope is re-derived from the cited plan row's `{package, symbol}` annotation (`ContractScope`); a row with no contract binds to nothing; the LLM-proposed scope is kept only as a fallback for non-plan corpora {id: BP-19, symbol: ContractScope, package: internal/semantic}
- [x] The `obligations` MCP tool inlines each rule's cited lines as `snippet` (capped at 240 bytes, `include_snippets` defaults true on the tool and false on the edit hook so the hook payload budget is unchanged) {id: BP-20, symbol: obligationSnippet, package: internal/mcp}
- [x] Payload replay golden for `obligations` on `internal/extractor/goextract.go`-shaped fixture files shows the governing rule first and `displaced` 0 after BP-17..19 {id: BP-21, skip_test: true, skip: "evidence is the BP-15 golden diff: re-running go test ./internal/indexbench/... -update after BP-17..19 landed left internal/indexbench/replay/golden.json byte-identical (displaced 0 on all three obligations entries), and the served order on ledger.go is fx-balance then fx-validate then fx-sorted (obligations, fresh, specific-first) with the stale spec-bindings claim last"}

## The deferral tax (D4)

- [-] The agent-surface per-session schema bytes are measured by `capabilities` and held under the harness's documented MCP deferral threshold (or, when undocumented, under the size that produced zero `ToolSearch` turns in July: compact `outputSchema` by reference, drop redundant descriptions); `TestNarrowProfileSchemaBytes` re-pinned with the reason {id: BP-22, disposition: refused, reason: "measured 2026-09-12: zero ToolSearch turns at 28,715 schema bytes on Claude Code 2.1.270 with and without alwaysLoad (O27); the deferral tax of 2.1.258 does not reproduce, so shrinking the surface would trade the outputSchema contract for nothing", trigger: "BP-23's toolsearch_turns line on the exploration ledger rises above zero"}
- [x] `toolsearch_turns` per session on the exploration ledger (from the session transcript parser) as the tripwire named in ADR-032 §5 {id: BP-23, symbol: toolSearchTurns, package: cmd/fathom}

## Records and the run

- [ ] `docs/bench/README.md` gains the pyramid table, the strengths map (which layer shows which capability), the cell-cache contract and `task bench-targeted`; scope G9 restated; `docs/design/coding-agent-excellence.md` §0 sentence 5 states the July figure as a turn-count result and names H2-3 as the standing bar {id: BP-24, skip_test: true, skip: "docs; evidence is the diffs and TestNoStaleHelpPhrasesInDocs"}
- [ ] (op) Full L4 run on the home corpus at n=3, three arms, scored under H2-1 (history) and H2-3, ledger rows committed, `gold_source_rate` per tool in the row {id: BP-25, symbol: TestLedgerHasReplicatedRow, package: internal/contextbench, skip_test: true, skip: "operator spend; the test asserts a replicated row exists"}

## L0 findings — index precision (2026-09-12)

The first `task bench-l0-self` run at HEAD (10,651 symbols, 100 Go packages, 0 with type
errors) measured what the nav tools actually return on this repository: `find_callers`
precision 0.607 (295/486) recall 0.899, `references` precision 0.158 (352/2,226) recall
1.000, `blast_radius` precision 0.086 (871/10,158) recall 0.805 at ~10.9 s a query. High
recall with low precision is the signature of the syntax pass's tolerant bare-name
matching, and ADR-032 holds L4 closed while L0 is under 0.95. Before/after numbers:
`docs/bench/README.md`.

- [x] `find_callers` and `references` return resolved (typed-pass) edges by default and heuristic (syntax-only) edges only with `include_heuristic: true`, each row still carrying `edge_confidence` (BD-2); precision on the self oracle ≥0.95 for find_callers {id: BP-26, symbol: resolvedCallersOnly, package: internal/mcp}
- [x] `blast_radius` walks resolved edges by default with a default depth cap (2) and a named result cap, heuristic edges opt-in; precision on the self oracle ≥0.95 at depth 1 {id: BP-27, symbol: blastRadiusResolved, package: internal/mcp}
- [x] The call graph is memoized per (service, store watermark) below the MCP layer and invalidated on ingest; I21 in `docs/intent/invariants.md` amended to "derived graphs may be memoized keyed by the store watermark only, never across ingests", with the test that pins invalidation {id: BP-28, symbol: CachedCallGraph, package: internal/analysis/callgraph, skip_test: true, skip: "CachedCallGraph is a type; no test CALLS it, so test_reaches cannot see the tests that prove it — `TestCachedCallGraphInvalidatesOnReIngest`, `TestCachedCallGraphEmptyWatermarkNeverCaches`, `TestCachedCallGraphBoundedLRU` and the MCP-level `TestNavGraphMemoIsWatermarkKeyed`, all in `task validate`"}
- [x] `task bench-l0-self` prints a per-tool verdict against the 0.95 floor and the report path, and records every truth caller the index missed (`-misses`) so a recall number becomes a list of defects; `find_callers` now PASSES the floor on both halves (P 1.000 / R 1.000), `blast_radius` passes (1.000 / 0.983), `references` precision stays below it as a stated lower bound (BP-30); `docs/bench/README.md` carries the before/after numbers {id: BP-29, oracle: lexical, path: Taskfile.yml, pattern: "^  bench-l0-self:$", skip_test: true, skip: "Taskfile + docs; the Go side is the verdict printer in cmd/indexbench"}
- [x] The `references` oracle comparison is made honest: which reference kinds the go/types truth can adjudicate (identifier uses in walked Go files, unambiguously-named functions) versus what `references` returns (every text mention, docs and config included) is defined, only the comparable subset is scored, and the rest is reported as unadjudicated — never forced into a number {id: BP-30, symbol: ReferencesOracleSubset, package: internal/indexbench, skip_test: true, skip: "ReferencesOracleSubset is a type; the tests that prove it reach its constructor, not it — `TestReferencesOracleSubsetPartitionsQueries`, `TestReferencesOracleSubsetAdjudicableFiles`, both in `task validate`"}
- [x] A bare symbol name that two packages share is answered from resolved edges — `CandidateKeys` unions the candidates instead of `canonicalize` refusing them and `find_callers` degrading to the syntax answer; a bare query never crosses a receiver boundary into a same-named method, and a name the index does not carry still returns nothing so the caller can fall back {id: BP-31, symbol: ResolvedCallersAcross, package: internal/analysis/callgraph}
