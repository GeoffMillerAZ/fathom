# Receipts — instrument truth, budgets on every answer, discoveries, evidence anchors, coverage

Design: `docs/vision/28-receipts.md`. Decision: ADR-034. The second reading
of the 2026-09-13 round (facts F1–F12 in the vision) is the evidence for
every wave-0 row; the paid reruns are wave 5 and stay inside vision 28 §6's
pilot tier. Free layers join `task validate`.

Verify: `fathom verify --service fathom --plan docs/plans/receipts.md`.

## Wave 0 — instrument truth and a clean room (F1–F6, F10)

- [x] The harness runs the rendered arm: `newSessionRequest` carries the rendered MCP config path and allow/deny lists from `wt.Arm`, the harness copies that config into the raw dir as the session's `mcp.json` and passes the copy — never a config rebuilt from the corpus arm {id: RC-1, symbol: newSessionRequest, package: internal/scenariobench}
- [x] Every rendered MCP entry carries `alwaysLoad: true` unless the corpus set it, and an arm with an allow-list gains `mcp__<server>` for each of its servers {id: RC-2, symbol: normalizeMCPServers, package: internal/scenariobench}
- [x] The stream parser reads the init event into `MCPServers` (name → status) and `MCPTools`, and counts every `tool_use` into `ToolCalls` {id: RC-3, symbol: ParseSessionStream, package: internal/scenariobench}
- [x] Instrument truth: a session whose arm expects a server that is absent or not `connected` is an instrument error and its cell is LOST — never stored {id: RC-4, symbol: instrumentTruthError, package: internal/scenariobench}
- [x] Readings carry `tool_calls` and `surface_calls`; a surface arm with zero surface calls is flagged `surface_unused` on the row and warned by `score` {id: RC-5, symbol: AggregateReadings, package: internal/scenariobench}
- [x] The clean room: the harness runs every session under `--restricted --settings <the arm's rendered settings.json> --tools <the arm's tools> --disable-slash-commands` (a scratch `CLAUDE_CONFIG_DIR` loses the login), `cleanRoomEnv` sets `FATHOM_THREADS` to a per-cell file, and `--permission-mode default` is passed whenever the arm has an allow-list, so a restriction denies instead of decorating {id: RC-6, symbol: cleanRoomEnv, package: internal/scenariobench}
- [x] The home bench runs in the same clean room (`--restricted` with the subject's settings file, permission mode) so no operator hook reaches a cell on either arm {id: RC-7, oracle: lexical, path: scripts/context-bench.sh, pattern: --restricted}
- [x] A competitor arm gets a routing stanza of the same shape fathom's has: `Arm.InstructionsText` rendered into the subject's `CLAUDE.md` for `instructions: "text"`; `lookups-vs.json` carries one per competitor naming its verbs by question shape {id: RC-8, symbol: RenderArm, package: internal/scenariobench}
- [x] The graphify arm runs graphify's documented server (`python -m graphify.serve <graph.json>` under uv) instead of a command that does not exist {id: RC-9, oracle: lexical, path: testdata/scenariobench/lookups-vs.json, pattern: graphify.serve}
- [x] The compile cost of a wired cell's obligations layer is on the cell and rolled into the row's cost {id: RC-10, symbol: readingsCompileCost, package: internal/scenariobench}
- [x] The re-exploration overlap weighs `Read`, `read_source`, `Grep` results and shell reads (`cat`, `sed -n`, `head`, `tail`, `awk` with a path) — a worker discovering through grep is no longer invisible {id: RC-11, symbol: SessionReadBytes, package: internal/scenariobench}
- [x] Guards cells carry `rule_served_before_edit` (the served rule's text appears in the transcript before the first edit of the governed file) and `violation` as row fields, so M1 is a rate and not prose {id: RC-12, symbol: RuleServedBeforeEdit, package: internal/scenariobench}
- [ ] `tokens_to_first_edit` is read for every session, and `facts_reused` counts hand-off pointers a session referenced before its first edit {id: RC-13, symbol: FactsReused, package: internal/scenariobench}
  <!-- product wiring open (fathom verify --strict reads it library-only, 2026-09-28): FactsReused has no production caller: no run computes facts_reused. -->
- [x] Trial escalation: `scenariobench plan --escalate` reads the ledger and lists the cells whose treatment/control delta sits within the criterion's margin at n=2, so n≥3 is spent only where it decides {id: RC-14, symbol: EscalationPlan, package: internal/scenariobench}
- [x] Turn budgets follow utilization and an unwinnable oracle is dropped with its reason: guards-i11 and guards-i31 to 80, the lookups to 20, `SB-cli__cli-10009` carries `disposition: dropped` (its fail_to_pass names a package that does not exist at base) and the driver skips it {id: RC-15, oracle: lexical, path: testdata/scenariobench/public-go.json, pattern: dropped}
- [x] The treatment and control cells of one task run concurrently on their own worktrees and daemons {id: RC-16, symbol: RunArmsConcurrently, package: internal/scenariobench}
- [x] The home bench reads `trusted_without_recheck`: fathom calls not followed within three turns by a Grep/Read/Bash on the same file or symbol, per task and per tool {id: RC-17, symbol: TrustedWithoutRecheck, package: internal/contextbench}
- [x] The home corpus carries `min_trials` per task — five on the tasks flaky in two of three runs (H1-T02, T07, T08, T13), three elsewhere, leaked tasks excluded from the criterion as before {id: RC-18, oracle: lexical, path: testdata/contextbench/tasks-home.json, pattern: min_trials}

## Wave 0 addendum — what the review of the merged fix found (O41 addendum)

- [x] `RenderArm` never replaces the hooks `fathom init` wrote: for a wired arm the rendered settings file is the one the init wrote, asserted to carry a SessionStart entry and absolute hook commands, and the nonexistent `obligations-hook` stanza is gone {id: RC-54, symbol: assertWiredHooksPresent, package: internal/scenariobench}
- [x] Hooks are instrument truth too: `hook_response` events are parsed into `hooks_fired`, and a wired session with no SessionStart hook response is an instrument error — the cell is LOST, as it is when any session (not only every session) carries one {id: RC-53, symbol: hooksInstrumentError, package: internal/scenariobench}
- [x] The index carries a symbol's end line for Go, C, C++, C#, Java, Lua, PHP, Rust, Scala, CUE, HCL, CMake and Templ, so `outline` renders `end_line`, the search receipt attributes a hit to the symbol whose span holds it, and the coverage join no longer guesses a boundary {id: RC-52, symbol: EndLine, package: internal/extractor/treesitter, reachable_from: TestAdoptionServiceScoped}
- [-] End lines for the wazero tree-sitter engine's languages (Python, JavaScript, TypeScript, Ruby), SQL, GraphQL and Protocol Buffers {id: RC-55, disposition: deferred, reason: "the engine's node carries a start row only — an end row needs a guest call on a hot path; the other three record no end position", trigger: "a coverage or search consumer on one of those languages, or the engine gains an end row"}

## Wave 1 — receipts on the answer (F7–F9)

- [x] `find_callers`, `outline`, `symbol_context`, `repo_map` and `staleness_report` render under the same 16 KB budget as `references`: elide, count, and name the refinement in the elision line {id: RC-19, symbol: fitToolBudget, package: internal/mcp, reachable_from: TestAdoptionServiceScoped}
- [x] A caller or reference query whose fan-in exceeds the budget returns the count, the top packages and the scoping arguments instead of the rows {id: RC-20, symbol: scopeRequired, package: internal/mcp, reachable_from: TestAdoptionServiceScoped}
- [x] `references` on an interface method lists the implementing types first, from the typed edges {id: RC-21, symbol: implementationsForMethod, package: internal/mcp, reachable_from: TestAdoptionServiceScoped}
- [x] `obligations` with no path answers "what governs this repository": the root conventions, budgeted {id: RC-22, symbol: rootObligations, package: internal/mcp, reachable_from: TestAdoptionServiceScoped}
- [x] The routing text — stanza, primer, skills, guides, adoption docs — names `staleness_report` for "is this still true / what was known", root `obligations` for repository conventions, and grep for a document-versus-document contradiction {id: RC-23, oracle: lexical, path: docs/guides/fleet-index.md, pattern: staleness_report}
- [x] Search with a receipt (SB-48 in `docs/plans/strengths-bench.md`) is on the agent surface with its pre-registered win condition beside it {id: RC-24, symbol: registerSearch, package: internal/mcp, reachable_from: TestAdoptionServiceScoped}

## Wave 2 — discoveries (F5, F11)

- [x] Every agent-surface call-log row carries the answer's top pointers (symbol ids, paths) in the `ctx:` grammar `fathom plans context` resolves — eight at most, never rendered text {id: RC-25, symbol: resultPointers, package: internal/mcp, reachable_from: TestAdoptionServiceScoped}
- [x] `fathom discoveries [--since T] [--budget N]` derives this repo's discoveries from the call log and filed thread pointers, resolves each fresh against the index, marks the stale, and renders within the budget {id: RC-26, symbol: cmdDiscoveries, package: cmd/fathom}
- [x] The pull form of the same render lives in the session brief (the SubagentStart hook's "last session established" — the `discoveries` MCP tool this row used to anchor was unregistered by SR-29, surface reckoning: 0 lifetime calls, the push form already covers the need) {id: RC-27, symbol: subagentDiscoveries, package: cmd/fathom}
- [x] SessionStart records the session's start (id, cwd, time) under `workspace/`; SubagentStart renders the parent session's discoveries in this repo since that start, within 1 200 bytes, or the constant primer when there are none {id: RC-28, symbol: subagentDiscoveries, package: cmd/fathom}
- [x] The session-end note carries the session's top pointers and the thread brief renders them resolved fresh inside its existing budget {id: RC-29, symbol: sessionEndPointers, package: cmd/fathom}
- [x] `fathom routes [package|concern]` derives the pointers established in two or more sessions over thirty days, resolved fresh and dropped when the index no longer has them; the primer names routes in one line only when one exists; `thread promote` can write a route onto a plan row's `ctx:` {id: RC-30, symbol: cmdRoutes, package: cmd/fathom}
- [x] The thread scope is the repo's identity — the origin URL normalized, else the path — with the basename as a read-side fallback so no existing deposit is lost {id: RC-31, symbol: repoIdentity, package: cmd/fathom}
- [x] The scenario driver gives a wired arm's workers the same discoveries brief SubagentStart renders, and `facts_reused` is read from it {id: RC-32, symbol: workerDiscoveryBrief, package: internal/scenariobench}
- [x] `docs/mcp-tools.md`, the fleet guide, the adoption docs and the skills describe discoveries and routes with their kill criteria {id: RC-33, oracle: lexical, path: docs/mcp-tools.md, pattern: discoveries}
- [x] Discoveries derive from every discovery action, not only fathom answers: the PreToolUse observer that already counts greps, globs and reads (`cmd/fathom/discovery.go`) records their targets as pointers (paths; the enclosing symbol when the index knows it), so a planner that grepped still hands its workers a brief — reuse-pilot-2 found the wired planners called no fathom tool and every worker brief was empty {id: RC-56, symbol: observedDiscoveryPointers, package: cmd/fathom}

## Wave 3 — receipts on readings

- [x] The plan grammar gains `oracle: ledger` (keys `path`, `run_id`, `task`, `arm`, optional `min_trials`): verify resolves the row in the ledger and refuses one that is missing, LOST, `surface_unused`, or under the trial floor {id: RC-34, symbol: ledgerOracle, package: internal/verify, reachable_from: TestAdoptionServiceScoped}
- [x] `scenariobench ledger` and the home render emit a "Where fathom stands" block from the row fields (better/worse than which arm, on which rows, at what cost) above the tables, and the README's authored verdict sits above it in plain language {id: RC-35, symbol: renderVerdict, package: internal/scenariobench}
- [x] The 74 rows of the first round carry `instrument_defect` with the fact that voids them; vision 27 §0, the bench README, scope G10 and the changelog are restated to say the round measured prompt text, not tools {id: RC-36, oracle: lexical, path: docs/bench/scenarios-ledger.jsonl, pattern: instrument_defect}

## Wave 4 — coverage as a code fact

- [x] Go cover profiles parse into blocks {id: RC-37, symbol: ParseProfile, package: internal/analysis/coverage}
- [ ] Blocks join to indexed symbols: statements, covered, and the covering test names when the profile carries them {id: RC-38, symbol: JoinSymbols, package: internal/analysis/coverage}
  <!-- product wiring open (fathom verify --strict reads it library-only, 2026-09-28): JoinSymbols has no production caller: no verb joins coverage blocks to indexed symbols. -->
- [x] A repo-declared, open-vocabulary layer config classifies each test (default: `_test.go` is `unit`; build tags and directory names as declared) {id: RC-39, symbol: Classify, package: internal/testlayer}
- [x] `fathom tests coverage --profile <file> [--diff]` prints per-symbol coverage by layer and the changed symbols with no covering test {id: RC-40, symbol: cmdTestsCoverage, package: cmd/fathom}
- [x] `review_diff` and `fathom review --json` carry a coverage block for the changed symbols when a profile is supplied — the one surface the review kit consumes {id: RC-41, symbol: DimTestCoverage, package: internal/analysis/review}
- [-] An obligation of the shape "a change to X needs a test at layer Y" {id: RC-42, disposition: deferred, reason: "waits for a consumer — the review kit parks coverage as context and test-kit owns floors", trigger: "the review kit promotes coverage out of context, or a second repo declares layers"}

## Wave 5 — the reruns (pilot tier) and the second reading

- [ ] (op) Dry cells before any spend: one real session per arm on a lookup task shows the server connected and, where the agent reaches for it, `surface_calls > 0`; and one real session on a change-shaped task proves the agent can run a shell command, create a directory and write a file inside its worktree (a replayed session catches neither — O43); the runbook's dry-cell section records both {id: RC-43, skip_test: true, skip: "operator spend; evidence is docs/bench/scenarios.md's dry-cell section"}
- [ ] (op) vs-pilot-2: the lookups across five arms with tools present and stanzas, and the repaired freshness task (SB-51); M6 and M7 written from it {id: RC-44, skip_test: true, skip: "operator spend; evidence is the ledger rows with surface_calls > 0", needs: RC-43}
- [ ] (op) guards-pilot-4 on both corpora with tools present, n=2, rule readings on the rows; M1 written as a rate {id: RC-45, skip_test: true, skip: "operator spend; evidence is the ledger rows", needs: RC-43}
- [ ] (op) public-pilot-3 on 10016, 10043 and 10072 with the plan-grammar instruction (SB-53); M2, M3 and M4 written from it {id: RC-46, skip_test: true, skip: "operator spend; evidence is the ledger rows", needs: RC-43}
- [ ] (op) reuse-pilot-2 at K=3 with the worker discoveries brief; M5 and the discoveries kill criteria decided {id: RC-47, skip_test: true, skip: "operator spend; evidence is the ledger rows and facts_reused", needs: RC-32}
- [ ] (op) The home bench reruns in the clean room after wave 1, with the search receipt in the full arm; H2-3 decided on the targeted tasks and the corpus {id: RC-48, skip_test: true, skip: "operator spend; evidence is docs/bench/ledger.jsonl", needs: RC-24}
- [ ] (op) The second reading: transcripts read before numbers on every rerun, observations filed, the artifact and the README verdict rewritten in plain language {id: RC-49, skip_test: true, skip: "reading; evidence is the observations and the artifact", needs: RC-48}
- [ ] A redesigned M1 corpus of six tasks whose rules live far from the edit site with mechanical detectors (a determinism rule stated only in a package guide, an org-agnostic rule with the package unnamed, a CGO build-tag rule with a decoy in the file, a served-only binding with no file to read, a multi-file change governed once, a process rule at a commit) {id: RC-50, oracle: lexical, path: testdata/scenariobench/guards-m1.json, pattern: served-only}
- [ ] (op) guards-m1 pilot at n=3; the M1 row written from it {id: RC-51, skip_test: true, skip: "operator spend; evidence is the ledger rows", needs: RC-50}
