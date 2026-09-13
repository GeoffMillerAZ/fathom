# Receipts — instrument truth, budgets on every answer, discoveries, evidence anchors, coverage

Design: `docs/vision/28-receipts.md`. Decision: ADR-034. The second reading
of the 2026-09-13 round (facts F1–F12 in the vision) is the evidence for
every wave-0 row; the paid reruns are wave 5 and stay inside vision 28 §6's
pilot tier. Free layers join `task validate`.

Verify: `fathom verify --service fathom --plan docs/plans/receipts.md`.

## Wave 0 — instrument truth and a clean room (F1–F6, F10)

- [ ] The harness runs the rendered arm: `newSessionRequest` carries the rendered MCP config path and allow/deny lists from `wt.Arm`, the harness copies that config into the raw dir as the session's `mcp.json` and passes the copy — never a config rebuilt from the corpus arm {id: RC-1, symbol: newSessionRequest, package: internal/scenariobench}
- [ ] Every rendered MCP entry carries `alwaysLoad: true` unless the corpus set it, and an arm with an allow-list gains `mcp__<server>` for each of its servers {id: RC-2, symbol: normalizeMCPServers, package: internal/scenariobench}
- [ ] The stream parser reads the init event into `MCPServers` (name → status) and `MCPTools`, and counts every `tool_use` into `ToolCalls` {id: RC-3, symbol: ParseSessionStream, package: internal/scenariobench}
- [ ] Instrument truth: a session whose arm expects a server that is absent or not `connected` is an instrument error and its cell is LOST — never stored {id: RC-4, symbol: instrumentTruthError, package: internal/scenariobench}
- [ ] Readings carry `tool_calls` and `surface_calls`; a surface arm with zero surface calls is flagged `surface_unused` on the row and warned by `score` {id: RC-5, symbol: AggregateReadings, package: internal/scenariobench}
- [ ] The clean room: the harness runs every session under `--restricted --settings <the arm's rendered settings.json> --tools <the arm's tools> --disable-slash-commands` (a scratch `CLAUDE_CONFIG_DIR` loses the login), `cleanRoomEnv` sets `FATHOM_THREADS` to a per-cell file, and `--permission-mode default` is passed whenever the arm has an allow-list, so a restriction denies instead of decorating {id: RC-6, symbol: cleanRoomEnv, package: internal/scenariobench}
- [ ] The home bench runs in the same clean room (`--restricted` with the subject's settings file, permission mode) so no operator hook reaches a cell on either arm {id: RC-7, oracle: lexical, path: scripts/context-bench.sh, pattern: --restricted}
- [ ] A competitor arm gets a routing stanza of the same shape fathom's has: `Arm.InstructionsText` rendered into the subject's `CLAUDE.md` for `instructions: "text"`; `lookups-vs.json` carries one per competitor naming its verbs by question shape {id: RC-8, symbol: RenderArm, package: internal/scenariobench}
- [ ] The graphify arm runs graphify's documented server (`python -m graphify.serve <graph.json>` under uv) instead of a command that does not exist {id: RC-9, oracle: lexical, path: testdata/scenariobench/lookups-vs.json, pattern: graphify.serve}
- [ ] The compile cost of a wired cell's obligations layer is on the cell and rolled into the row's cost {id: RC-10, symbol: readingsCompileCost, package: internal/scenariobench}
- [ ] The re-exploration overlap weighs `Read`, `read_source`, `Grep` results and shell reads (`cat`, `sed -n`, `head`, `tail`, `awk` with a path) — a worker discovering through grep is no longer invisible {id: RC-11, symbol: SessionReadBytes, package: internal/scenariobench}
- [ ] Guards cells carry `rule_served_before_edit` (the served rule's text appears in the transcript before the first edit of the governed file) and `violation` as row fields, so M1 is a rate and not prose {id: RC-12, symbol: RuleServedBeforeEdit, package: internal/scenariobench}
- [ ] `tokens_to_first_edit` is read for every session, and `facts_reused` counts hand-off pointers a session referenced before its first edit {id: RC-13, symbol: FactsReused, package: internal/scenariobench}
- [ ] Trial escalation: `scenariobench plan --escalate` reads the ledger and lists the cells whose treatment/control delta sits within the criterion's margin at n=2, so n≥3 is spent only where it decides {id: RC-14, symbol: EscalationPlan, package: internal/scenariobench}
- [ ] Turn budgets follow utilization and an unwinnable oracle is dropped with its reason: guards-i11 and guards-i31 to 80, the lookups to 20, `SB-cli__cli-10009` carries `disposition: dropped` (its fail_to_pass names a package that does not exist at base) and the driver skips it {id: RC-15, oracle: lexical, path: testdata/scenariobench/public-go.json, pattern: dropped}
- [ ] The treatment and control cells of one task run concurrently on their own worktrees and daemons {id: RC-16, symbol: RunArmsConcurrently, package: internal/scenariobench}
- [ ] The home bench reads `trusted_without_recheck`: fathom calls not followed within three turns by a Grep/Read/Bash on the same file or symbol, per task and per tool {id: RC-17, symbol: TrustedWithoutRecheck, package: internal/contextbench}
- [ ] The home corpus carries `min_trials` per task — five on the tasks flaky in two of three runs (H1-T02, T07, T08, T13), three elsewhere, leaked tasks excluded from the criterion as before {id: RC-18, oracle: lexical, path: testdata/contextbench/tasks-home.json, pattern: min_trials}

## Wave 1 — receipts on the answer (F7–F9)

- [ ] `find_callers`, `outline`, `symbol_context`, `repo_map` and `staleness_report` render under the same 16 KB budget as `references`: elide, count, and name the refinement in the elision line {id: RC-19, symbol: fitToolBudget, package: internal/mcp}
- [ ] A caller or reference query whose fan-in exceeds the budget returns the count, the top packages and the scoping arguments instead of the rows {id: RC-20, symbol: scopeRequired, package: internal/mcp}
- [ ] `references` on an interface method lists the implementing types first, from the typed edges {id: RC-21, symbol: implementationsForMethod, package: internal/mcp}
- [ ] `obligations` with no path answers "what governs this repository": the root conventions, budgeted {id: RC-22, symbol: rootObligations, package: internal/mcp}
- [ ] The routing text — stanza, primer, skills, guides, adoption docs — names `staleness_report` for "is this still true / what was known", root `obligations` for repository conventions, and grep for a document-versus-document contradiction {id: RC-23, oracle: lexical, path: docs/guides/fleet-index.md, pattern: staleness_report}
- [ ] Search with a receipt (SB-48 in `docs/plans/strengths-bench.md`) is on the agent surface with its pre-registered win condition beside it {id: RC-24, symbol: registerSearch, package: internal/mcp}

## Wave 2 — discoveries (F5, F11)

- [ ] Every agent-surface call-log row carries the answer's top pointers (symbol ids, paths) in the `ctx:` grammar `fathom plans context` resolves — eight at most, never rendered text {id: RC-25, symbol: resultPointers, package: internal/mcp}
- [ ] `fathom discoveries [--since T] [--budget N]` derives this repo's discoveries from the call log and filed thread pointers, resolves each fresh against the index, marks the stale, and renders within the budget {id: RC-26, symbol: cmdDiscoveries, package: cmd/fathom}
- [ ] `discoveries` is on the agent surface as the pull form of the same render {id: RC-27, symbol: registerDiscoveries, package: internal/mcp}
- [ ] SessionStart records the session's start (id, cwd, time) under `workspace/`; SubagentStart renders the parent session's discoveries in this repo since that start, within 1 200 bytes, or the constant primer when there are none {id: RC-28, symbol: subagentDiscoveries, package: cmd/fathom}
- [ ] The session-end note carries the session's top pointers and the thread brief renders them resolved fresh inside its existing budget {id: RC-29, symbol: sessionEndPointers, package: cmd/fathom}
- [ ] `fathom routes [package|concern]` derives the pointers established in two or more sessions over thirty days, resolved fresh and dropped when the index no longer has them; the primer names routes in one line only when one exists; `thread promote` can write a route onto a plan row's `ctx:` {id: RC-30, symbol: cmdRoutes, package: cmd/fathom}
- [ ] The thread scope is the repo's identity — the origin URL normalized, else the path — with the basename as a read-side fallback so no existing deposit is lost {id: RC-31, symbol: repoIdentity, package: cmd/fathom}
- [ ] The scenario driver gives a wired arm's workers the same discoveries brief SubagentStart renders, and `facts_reused` is read from it {id: RC-32, symbol: workerDiscoveryBrief, package: internal/scenariobench}
- [ ] `docs/mcp-tools.md`, the fleet guide, the adoption docs and the skills describe discoveries and routes with their kill criteria {id: RC-33, oracle: lexical, path: docs/mcp-tools.md, pattern: discoveries}

## Wave 3 — receipts on readings

- [ ] The plan grammar gains `oracle: ledger` (keys `path`, `run_id`, `task`, `arm`, optional `min_trials`): verify resolves the row in the ledger and refuses one that is missing, LOST, `surface_unused`, or under the trial floor {id: RC-34, symbol: ledgerOracle, package: internal/verify}
- [ ] `scenariobench ledger` and the home render emit a "Where fathom stands" block from the row fields (better/worse than which arm, on which rows, at what cost) above the tables, and the README's authored verdict sits above it in plain language {id: RC-35, symbol: renderVerdict, package: internal/scenariobench}
- [ ] The 74 rows of the first round carry `instrument_defect` with the fact that voids them; vision 27 §0, the bench README, scope G10 and the changelog are restated to say the round measured prompt text, not tools {id: RC-36, oracle: lexical, path: docs/bench/scenarios-ledger.jsonl, pattern: instrument_defect}

## Wave 4 — coverage as a code fact

- [ ] Go cover profiles parse into blocks {id: RC-37, symbol: ParseProfile, package: internal/analysis/coverage}
- [ ] Blocks join to indexed symbols: statements, covered, and the covering test names when the profile carries them {id: RC-38, symbol: JoinSymbols, package: internal/analysis/coverage}
- [ ] A repo-declared, open-vocabulary layer config classifies each test (default: `_test.go` is `unit`; build tags and directory names as declared) {id: RC-39, symbol: Classify, package: internal/testlayer}
- [ ] `fathom tests coverage --profile <file> [--diff]` prints per-symbol coverage by layer and the changed symbols with no covering test {id: RC-40, symbol: cmdTestsCoverage, package: cmd/fathom}
- [ ] `review_diff` and `fathom review --json` carry a coverage block for the changed symbols when a profile is supplied — the one surface the review kit consumes {id: RC-41, symbol: DimTestCoverage, package: internal/analysis/review}
- [-] An obligation of the shape "a change to X needs a test at layer Y" {id: RC-42, disposition: deferred, reason: "waits for a consumer — the review kit parks coverage as context and test-kit owns floors", trigger: "the review kit promotes coverage out of context, or a second repo declares layers"}

## Wave 5 — the reruns (pilot tier) and the second reading

- [ ] (op) Dry cells on every arm show `surface_calls > 0` for fathom, gopls, Serena and graphify before any spend; the runbook's dry-cell section records them {id: RC-43, skip_test: true, skip: "operator spend; evidence is docs/bench/scenarios.md's dry-cell section"}
- [ ] (op) vs-pilot-2: the lookups across five arms with tools present and stanzas, and the repaired freshness task (SB-51); M6 and M7 written from it {id: RC-44, skip_test: true, skip: "operator spend; evidence is the ledger rows with surface_calls > 0", needs: RC-43}
- [ ] (op) guards-pilot-4 on both corpora with tools present, n=2, rule readings on the rows; M1 written as a rate {id: RC-45, skip_test: true, skip: "operator spend; evidence is the ledger rows", needs: RC-43}
- [ ] (op) public-pilot-3 on 10016, 10043 and 10072 with the plan-grammar instruction (SB-53); M2, M3 and M4 written from it {id: RC-46, skip_test: true, skip: "operator spend; evidence is the ledger rows", needs: RC-43}
- [ ] (op) reuse-pilot-2 at K=3 with the worker discoveries brief; M5 and the discoveries kill criteria decided {id: RC-47, skip_test: true, skip: "operator spend; evidence is the ledger rows and facts_reused", needs: RC-32}
- [ ] (op) The home bench reruns in the clean room after wave 1, with the search receipt in the full arm; H2-3 decided on the targeted tasks and the corpus {id: RC-48, skip_test: true, skip: "operator spend; evidence is docs/bench/ledger.jsonl", needs: RC-24}
- [ ] (op) The second reading: transcripts read before numbers on every rerun, observations filed, the artifact and the README verdict rewritten in plain language {id: RC-49, skip_test: true, skip: "reading; evidence is the observations and the artifact", needs: RC-48}
- [ ] A redesigned M1 corpus of six tasks whose rules live far from the edit site with mechanical detectors (a determinism rule stated only in a package guide, an org-agnostic rule with the package unnamed, a CGO build-tag rule with a decoy in the file, a served-only binding with no file to read, a multi-file change governed once, a process rule at a commit) {id: RC-50, oracle: lexical, path: testdata/scenariobench/guards-m1.json, pattern: served-only}
- [ ] (op) guards-m1 pilot at n=3; the M1 row written from it {id: RC-51, skip_test: true, skip: "operator spend; evidence is the ledger rows", needs: RC-50}
