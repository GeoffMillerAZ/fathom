# Strengths bench — the matrix, the scenario driver, the readings, the competitors, the runbook

Design: `docs/vision/27-strengths-bench.md`. Decision: ADR-033. The home
bench's lessons (O31, O32) are wave 1; the paid pilots are wave 6 and stay
within vision 27 §9's targeted and pilot tiers. Free layers join
`task validate`.

Verify: `fathom verify --service fathom --plan docs/plans/strengths-bench.md`.

## Wave 1 — the lessons and the doctrine (O31, O32)

- [ ] `references` renders one occurrence in ≤ 100 bytes on the wire: `file:line`, a trimmed text line, confidence as one character; `service`, `tier` and `depth` lifted to the envelope when uniform; default `max` 25 with the elision block naming how to get more; `TestReferencesOccurrenceUnder100Bytes` pins it {id: SB-1, symbol: renderReferenceOccurrence, package: internal/mcp}
- [ ] `references` refuses a query that is not an identifier, a dotted path or a `regex: true` pattern, naming the three forms — never 27 heuristic lines for `func (s *` {id: SB-2, symbol: refuseNonSymbolQuery, package: internal/mcp}
- [ ] `read_source` carries a per-call byte budget with the elision block and its description says when NOT to reach for it (a `find_symbol` hit already carries file and line; `symbol_context` carries the body) {id: SB-3, symbol: readSourceBudget, package: internal/mcp}
- [ ] Every agent-surface tool whose result can be definitive (`find_symbol` exact, `verify_plan`, `obligations`, `find_callers`, `symbol_context`) says in its description that the result IS the answer and what a follow-up call would add; `TestAgentSurfaceDescriptionsStateWhenToStop` iterates the allowlist {id: SB-4, symbol: TestAgentSurfaceDescriptionsStateWhenToStop, package: internal/mcp, skip_test: true, skip: "the test is the anchor"}
- [ ] `find_symbol` on a Go interface carries `implementations` (types whose method sets satisfy it, from the typed pass when present, else `var _ I = (*T)(nil)` assertions and method-set match), so H1-T06's question is one call {id: SB-5, symbol: implementationsOf, package: internal/mcp}
- [ ] The runner gains the realistic arm `full` (fathom MCP plus Bash/Read/Grep) beside `fathom`, `grep`, `none`; its cells key on the fathom commit like `fathom`'s {id: SB-6, oracle: lexical, path: scripts/context-bench.sh, pattern: "full\\)"}
- [ ] The stanza `fathom init` writes and the session primer route by question shape — index for where / who-calls / what-breaks / what-governs / what-was-known, grep for literal text — replacing "before any Grep/Read sweep"; `TestStanzaRoutesByQuestionShape` pins the wording {id: SB-7, symbol: TestStanzaRoutesByQuestionShape, package: cmd/fathom, skip_test: true, skip: "the test is the anchor; the stanza source is the change"}
- [ ] The nine shipped skills, `docs/adoption/*.md`, `docs/use-cases/coding-agents.md` and root `CLAUDE.md`'s fathom stanza carry the question-shape routing; re-rendered into `.claude/skills` and `.agents/skills` {id: SB-8, oracle: lexical, path: .agents/skills/fathom-onboard/SKILL.md, pattern: "question shape"}
- [ ] Targeted run over H1-T06, H1-T08, H1-T11, H1-T03 after SB-1..5: payload ratio ≤ 1.0× on the targeted ledger, else the row records the number and SB-1..5 are not claimed as the fix {id: SB-9, skip_test: true, skip: "operator spend (targeted tier); evidence is workspace/artifacts/context-bench/targeted-ledger.jsonl", needs: SB-5}

## Wave 2 — one driver, arms as data

- [ ] `LoadScenarioCorpus` parses `testdata/scenariobench/*.json` into the `types.go` contract and refuses a task without a prediction per arm, an oracle, or a derivation; `CorpusSHA256` is the content hash {id: SB-10, symbol: LoadScenarioCorpus, package: internal/scenariobench}
- [ ] `RenderArm` turns an `Arm` into the harness inputs — `.claude/settings.json` (hooks only when wired), the MCP config file, allowed/disallowed tools, the instruction-file variant, the notes-file instruction — and `StripFathom` removes the stanza, hooks and MCP entry from a wired worktree for the control {id: SB-11, symbol: RenderArm, package: internal/scenariobench}
- [ ] `PrepareSubject` clones the pinned subject once under `workspace/scenariobench/subjects/<sha>` and creates one worktree per cell at `base_commit`; a wired arm runs `fathom init --harness claude-code --json` against a scratch daemon that ingests only the worktree (never the fleet store) and compiles layers when the arm asks; the scratch daemon's cost is recorded on the cell {id: SB-12, symbol: PrepareSubject, package: internal/scenariobench}
- [ ] `RunSessions` executes a task's sessions in order under the budget: `single` runs Count sessions; `planner-workers` runs the planner then one worker per plan row up to Workers, each cold, each in the same worktree; the prompt differs between arms only by what the arm declares {id: SB-13, symbol: RunSessions, package: internal/scenariobench}
- [ ] `ClaudeHarness` shells out to `claude -p --output-format stream-json --max-turns N` with the arm's MCP config, `--strict-mcp-config`, allowed tools and settings, captures the stream to `raw/`, and `ParseSessionStream` derives every `SessionResult` field (turns, tokens, cost, first edit, files read with bytes, files edited, box checks, tool calls, payload bytes) {id: SB-14, symbol: ParseSessionStream, package: internal/scenariobench}
- [ ] `FakeHarness` replays committed stream transcripts under `testdata/scenariobench/streams/`, so `TestRunSessions_FakeHarnessReplays` drives the whole loop without spend {id: SB-15, symbol: TestRunSessions_FakeHarnessReplays, package: internal/scenariobench, skip_test: true, skip: "the test is the anchor"}
- [ ] `RunOracle` runs the subject's test command over fail-to-pass and pass-to-pass with a coverprofile when asked, runs every detector, and records exit codes and output refs; the agent never runs the oracle {id: SB-16, symbol: RunOracle, package: internal/scenariobench}
- [ ] `ScenarioCellKey` is `sha256(corpus_sha256 · task · arm · model_id · harness_version · scoring · prompt_subject · arm_surface)` with `arm_surface` the fathom commit for wired arms, the pinned version for competitor arms, empty for controls; `BorrowScenarioCell` and `AcceptBorrowedScenario` mirror ADR-032's contract {id: SB-17, symbol: ScenarioCellKey, package: internal/scenariobench}
- [ ] `AppendScenarioLedger` writes one row per run to `docs/bench/scenarios-ledger.jsonl` (per-task, per-arm success and readings, predictions beside verdicts, borrowed cells, cost) and `RenderScenarioLedger` renders the marker-fenced table in `docs/bench/README.md` {id: SB-18, symbol: AppendScenarioLedger, package: internal/scenariobench}
- [ ] `cmd/scenariobench plan|run|score|ledger`: `plan` lists cells, keys and what would borrow (free, no spend); `run` executes and files cells; `score` recomputes readings from raw; `ledger` evaluates the corpus criterion and appends; exit codes 0 pass / 1 fail / 3 indeterminate {id: SB-19, symbol: cmdPlan, package: cmd/scenariobench}

## Wave 3 — the readings, the guards corpus, exploration reuse

- [ ] `PlanCoverage` reads the agent's plan (v2 grammar rows, or a plain checklist's backticked names) before the first edit and scores named gold functions and spurious rows {id: SB-20, symbol: PlanCoverage, package: internal/scenariobench}
- [ ] `FalseDoneRate` matches every `[x]` box check to the final tree and the coverprofile: the row's symbol exists and a covered block falls in its span (spans via `go/parser`); refuted rows named {id: SB-21, symbol: FalseDoneRate, package: internal/scenariobench}
- [ ] `ResumptionReadings` (tokens to first edit from session 2 on; duplicated edits to rows already proven) and `ContractDrift` (rows whose symbol vanished or is no longer test-reached after the change) {id: SB-22, symbol: ResumptionReadings, package: internal/scenariobench}
- [ ] `ReexplorationOverlap`: for each worker, bytes of files read that the planner had already read, over bytes read in total; `not_measurable` when a session recorded no reads {id: SB-23, symbol: ReexplorationOverlap, package: internal/scenariobench}
- [ ] `testdata/scenariobench/guards-fathom.json`: ≥ 8 governance-bound tasks derived from `docs/intent/invariants.md` (each names the rule, its source line, the detector command that is the invariant's own test or lint, and a prediction per arm); `TestGuardsCorpusDetectorsExist` runs each detector's `--help`/dry form or asserts the named test exists {id: SB-24, symbol: TestGuardsCorpusDetectorsExist, package: internal/scenariobench, skip_test: true, skip: "the test is the anchor"}
- [ ] `RuleServed` reads the wired arm's hook beats: whether an obligation carrying the task's rule id reached the session before its first `[x]` (Q21 as a per-task number) {id: SB-25, symbol: RuleServed, package: internal/scenariobench}
- [ ] `TestEveryScenarioTaskHasPredictions`: every committed scenario corpus has a prediction per arm per task and a criterion; a corpus without a no-effect prediction fails the gate {id: SB-26, symbol: TestEveryScenarioTaskHasPredictions, package: internal/scenariobench, skip_test: true, skip: "the test is the anchor"}

## Wave 4 — competitors, run as documented

- [ ] `docs/bench/competitors.md`: for graphify, Serena and gopls — install, build/run, query surface, output format, staleness model, license, pinned version and the date verified; each arm's commands copied from the tool's own docs {id: SB-27, oracle: lexical, path: docs/bench/competitors.md, pattern: "\\*\\*Verified against\\*\\*"}
- [ ] `CompareEdges` scores any edge set (caller → callee, or reference sites) against the L0 `go/types` truth with the same precision/recall and the same adjudicability rule as `indexbench` {id: SB-28, symbol: CompareEdges, package: internal/indexbench}
- [ ] `LoadGraphifyEdges` reads graphify's output graph into the comparable edge set, dropping edge kinds the truth cannot adjudicate and naming them {id: SB-29, symbol: LoadGraphifyEdges, package: internal/indexbench}
- [ ] `ProbeReferencesViaMCP` queries an MCP server (Serena, gopls) for the references or callers of every truth symbol through agentic-kit's `mcp-client` and builds the comparable edge set; the tool names come from the competitors doc {id: SB-30, symbol: ProbeReferencesViaMCP, package: internal/indexbench}
- [ ] `indexbench compare --graphify <graph> | --mcp <config> --tool <name>` prints precision/recall beside fathom's own row and writes `docs/bench/competitors-ledger.jsonl` {id: SB-31, symbol: cmdCompare, package: cmd/indexbench}
- [ ] `testdata/scenariobench/lookups-vs.json`: the home lookup tasks as scenarios with arms `full`, `graphify`, `serena`, `gopls`, `grep` (every arm with Read and Grep), a freshness task per M7, predictions per arm from vision 27 §6 {id: SB-32, oracle: lexical, path: testdata/scenariobench/lookups-vs.json, pattern: "\"graphify\""}

## Wave 5 — the public corpus and the runbook

- [ ] `ClassifyChange` assigns a category from a patch's shape: files and packages spanned, exported signatures changed (`go/parser` before/after), `go.mod` touched, docs-only rules touched {id: SB-33, symbol: ClassifyChange, package: internal/scenariobench}
- [ ] `DeriveScenarioTasks` derives `testdata/scenariobench/public-go.json` from the pinned Multi-SWE-bench Go and SWE-Explore snapshots (ADR-031's rule: gold touches 1–5 non-test files; sort by instance id) with fail-to-pass / pass-to-pass from the source, gold functions via `EnclosingFuncsForHunks`, category via `ClassifyChange`, `merged_at` kept and post-cutoff preferred; `TestPublicScenarioCorpusIsDerived` re-derives and compares the hash {id: SB-34, symbol: DeriveScenarioTasks, package: internal/scenariobench}
- [ ] `docs/bench/scenarios.md`, the runbook a stranger can follow: pins, one command per tier, cost per tier, how to add a task, how to add an arm, how to publish a row; `task bench-scenario-plan` (free), `bench-scenario-guards`, `bench-scenario-pilot`, `bench-compare-edges` {id: SB-35, oracle: lexical, path: docs/bench/scenarios.md, pattern: "Repeat it yourself"}
- [ ] Free layers in `task validate`: corpus load and prediction gate, fake-harness driver tests, `indexbench compare` against the in-tree fixture fleet {id: SB-36, oracle: lexical, path: Taskfile.yml, pattern: "scenariobench"}

## Wave 6 — the pilots (operator spend; targeted and pilot tiers)

- [ ] (op) Guards pilot on the fathom repo: guards corpus × arms `full`, `control` × n=2; row on the scenarios ledger with rule-served and violation readings beside the predictions {id: SB-37, skip_test: true, skip: "operator spend; evidence is the ledger row", needs: SB-24}
- [ ] (op) Change-bench pilot: 8 public Go tasks × `full`, `control` × n=2 × 2 sessions; SDD readings on the row {id: SB-38, skip_test: true, skip: "operator spend; evidence is the ledger row", needs: SB-34}
- [ ] (op) Exploration-reuse pilot: 3 cross-package tasks in `planner-workers` mode, K=3, both arms {id: SB-39, skip_test: true, skip: "operator spend; evidence is the ledger row", needs: SB-23}
- [ ] Competitor edge comparison (free) on the fathom repo and one public subject: precision/recall per tool on `docs/bench/competitors-ledger.jsonl` {id: SB-40, skip_test: true, skip: "free run; evidence is the competitors ledger", needs: SB-31}
- [ ] (op) Competitor lookup pilot: `lookups-vs.json` × n=2 with the freshness task {id: SB-41, skip_test: true, skip: "operator spend; evidence is the ledger row", needs: SB-32}
- [-] M8 fleet blast radius and M9 breadth rows {id: SB-42, disposition: deferred, reason: "registered in vision 27 §1; scheduled after the pilots so their cost is decided on the pilots' readings", trigger: "SB-37..41 rows are on the ledger"}

## Wave 7 — the record

- [ ] `docs/bench/README.md` gains the strengths matrix with each row's ledger pointer and the pilots' readings; `docs/design/coding-agent-excellence.md` and `docs/adoption/README.md` cite the matrix, not the lookup bench, for fathom's claims {id: SB-43, oracle: lexical, path: docs/bench/README.md, pattern: "strengths matrix"}
- [ ] CHANGELOG lines for the doctrine change, the new tools' contracts and the driver; a Claude artifact with the readings {id: SB-44, skip_test: true, skip: "docs; evidence is the diffs"}
