# Proof credibility — instruments that cannot lie, proof for non-code rows, the claim-moment gate, wiring drift

Design: `docs/vision/26-proof-credibility.md` §1 and §4. Decisions: ADR-030
(lexical oracle), ADR-027 (coverage before conclusion) applied to gate
verdicts. Bench rows live in `bench-public.md`; product rows in `blast-depth.md`.

Verify: `fathom verify --service fathom --plan docs/plans/proof-credibility.md`.

## Wave 1 — store provenance on gate verdicts (O20, O5)

- [x] `/statusz` carries `binary_commit`, `binary_dirty`, `built_at` (additive; `schema_version` bumped) from the daemon's own `internal/buildinfo` (`debug.BuildInfo` `vcs.*`) {id: PC-1, symbol: statuszBuildInfo, package: cmd/fathom}
- [x] Every verify-ledger row carries `daemon_commit` (from `/statusz`) and `store_commit` (the watermark the store was built at) {id: PC-2, symbol: verifyLedgerProvenance, package: cmd/fathom}
- [x] `fathom verify` prints one provenance line when the daemon's commit differs from the local binary's `vcs.revision` — never compared to the cwd repo's HEAD (O5) {id: PC-3, symbol: provenanceNotice, package: cmd/fathom}
- [x] Doctor `daemon_fresh` compares the daemon's `binary_commit` to the local binary's; `binary_fresh` is re-scoped to the local binary vs its own source stamp and says so {id: PC-4, symbol: checkDaemonFresh, package: cmd/fathom}
- [x] `plans status` and `intent audit` mark rows whose newest reading came from a stale daemon `measured-stale`, counted as coverage gaps, never as failing (ADR-027) {id: PC-5, symbol: markMeasuredStale, package: cmd/fathom}
- [x] O20 committed with root cause and fix; status `fixed` once PC-1..5 verify {id: PC-6, skip_test: true, skip: "observation log; evidence is .fathom/observations.md O20's status line"}

## Wave 1 — the honesty sweep

- [x] The U2 remedy text says what the counter does: a `skip:` reason does not lift U2; only a symbol or a lexical oracle does. `TestU2RemedyMatchesCounter` pins remedy and counter to each other {id: PC-7, symbol: TestU2RemedyMatchesCounter, package: cmd/fathom, skip_test: true, skip: "the test is the anchor"}
- [ ] Anchors: OB-3, OB-12 (observatory), EP-13 (evidence-plane), DO-4 (ambient-fathom) re-anchored to the tests their skip reasons already name (done 2026-09-02); the three U11 ADR citations corrected (`ADR.md` bench ledger wording; the two deleted DuckDB paths — pending, owned by the ADR.md unit); the six router surfaces with no `Test*` in "Done needs" name one (done 2026-09-02) {id: PC-8, skip_test: true, skip: "plan and intent edits; evidence is fathom intent audit's U2/U11/U12 counts in the Verified: line"}
- [x] `scope.md` G6: the blocker names ADR-030's path, not the closed thread `th-c214dd26`; DP-2 dispositioned superseded by EP-9 {id: PC-9, oracle: lexical, path: docs/intent/scope.md, pattern: "the lexical oracle shipped in 0\.8\.0 \(ADR-030"}
- [x] `needs:` may reference a row in another plan (`custodian.md#SD-2`): `resolveCrossPlanNeed` reads the sibling plan's row state; EP-44's lint line is gone {id: PC-10, symbol: resolveCrossPlanNeed, package: internal/verify, reachable_from: TestAnsweredBeatCarriesDisplacedCount}
- [x] Doctor's module rows call `list_modules` on the surface that serves it, or name the real reason; "tool not found" for a tool that exists is gone {id: PC-11, symbol: moduleListForDoctor, package: cmd/fathom}
- [ ] The eight handoff threads whose every item shipped are closed with a reason (`fathom thread close`); the A/B arm state is read and recorded in the thread note (vision 24 §7: ON once instrument-truth landed) {id: PC-12, skip_test: true, skip: "operator verbs; evidence is fathom thread list"}
- [x] One canonical verify figure: `verifyFigure` computes catches / gate runs / plans from the ledger with its window and denominator in the sentence; `fathom research report` and `docs/adoption/README.md` use it, retiring the three circulating figures (O6 family) {id: PC-13, symbol: verifyFigure, package: cmd/fathom}
- [x] Rung lifts named by `invariants.md`: I19 gets a denylist test over code, docs, tests and fixtures (`TestNoOrgSpecificNames`); I36 gets a `## Maintained` parser test (`TestMaintainedLinesParse`); `fathom config audit` joins `task validate` (I16/I34/I37) {id: PC-14, symbol: TestNoOrgSpecificNames, package: cmd/fathom, skip_test: true, skip: "the tests are the anchors; the validate wiring is the Taskfile diff"}
- [x] I31: `-store` and `-call-log` defaults resolve under the repo's `workspace/` (or `~/.fathom` when no repo), never the cwd (O11/O12); CHANGELOG line {id: PC-15, symbol: defaultStorePath, package: cmd/fathom}
- [ ] Q21 drafted as a proposed ADR-016 amendment: a measured obligation ceiling with the 2026-08-27 numbers, Status `proposed`, the owner decides {id: PC-16, skip_test: true, skip: "a proposed record; evidence is the ADR.md section"}

## Wave 2 — the lexical oracle (ADR-030)

- [x] `{oracle: lexical, path: <repo path>, pattern: <regex>}` on a plan row: `evalLexicalOracle` checks the path exists in the working tree the plan itself is read from (the same moment, never `git show HEAD:`), is non-empty, and the pattern matches at least once; the row verifies and leaves U2 {id: PC-17, symbol: evalLexicalOracle, package: internal/verify, reachable_from: TestAdoptionServiceScoped}
- [x] `LintPlan` refuses a Markdown path without a `pattern` and a lexical row that also carries `symbol` (one contract per row) {id: PC-18, symbol: lintLexicalOracle, package: internal/verify, reachable_from: TestAnsweredBeatCarriesDisplacedCount}
- [x] `schema/plan/schema.cue` gains the keys with valid/invalid testdata; `docs/plans/README.md` gains the grammar row; `CHANGELOG.md` gains the line {id: PC-19, oracle: lexical, path: schema/plan/schema.cue, pattern: "^\s*oracle\?: \"lexical\""}
- [ ] The non-code rows across active plans carry lexical oracles or a typed disposition; `fathom verify --strict` exits 0 for every top-level plan (scope G6) {id: PC-20, skip_test: true, skip: "plan edits; evidence is the strict verify sweep in the Verified: line"}
- [x] `fathom verify --format sarif` emits SARIF 2.1.0 for the refusals (rule id per criterion, location = the plan line), so code-scanning and editor problem panes render them; `intent audit` and `config audit` share the emitter {id: PC-21, symbol: renderSARIF, package: internal/verify, reachable_from: TestAnsweredBeatCarriesDisplacedCount}

## Wave 3 — done-means-proven at the claim moment

- [x] Claim-moment verify on the existing edit hook: when a PostToolUse edit touches a depth-1 `docs/plans/*.md` and adds a `[x]`, `claimMomentVerify` runs the verify path for that plan and injects the refusal (if any) as `additionalContext`; silent on a clean claim; verify-ledger rows carry `origin: claim-moment` {id: PC-22, symbol: claimMomentVerify, package: cmd/fathom}
- [x] `InstructionsLoaded` hook row → `fathom session-hook --event instructions-loaded`: a silent beat naming `file_path`, `memory_type`, `load_reason` (the first harness-side evidence for I37) {id: PC-23, symbol: recordInstructionsLoaded, package: cmd/fathom}
- [x] `ConfigChange` hook row → `fathom config audit --hook`: an attention line on the next session line, never a block (a block surfaces no message in the harness) {id: PC-24, symbol: configChangeHook, package: cmd/fathom}
- [x] Doctor `stanza_loaded` reads the InstructionsLoaded beats: PASS when a session loaded the stanza-bearing file, SKIP (never FAIL) with the reason when no beat exists {id: PC-25, symbol: checkStanzaLoaded, package: cmd/fathom}
- [ ] The two new rows are carried by init, plugin render, reconcile and doctor from the one hook table; one deliberate `fleet reconcile --apply` (ask-first) rolls them out; pre-registered: ≥1 ledger row with `origin: claim-moment` that refused within 14 days, else PC-22 is dispositioned deferred {id: PC-26, skip_test: true, skip: "fleet operation and a pre-registered reading; evidence is the verify ledger"}
- [-] `TaskCompleted` hook row {id: PC-27, disposition: deferred, reason: "the event belongs to the built-in task list, absent by default on current models; the claim moment in fathom's model is the [x] edit (PC-22)", trigger: "the task-list tools are enabled fleet-wide"}

## Wave 4 — wiring drift covers what an agent could be made to change

- [x] `fleet reconcile` and the law-15 digest cover MCP server entries (argv and env NAMES, never values) in the project and user MCP files each descriptor names, and fathom-owned skill trees; a drifted entry is reported by kind and path, never merged (`mcpEntryDigest`) {id: PC-28, symbol: mcpEntryDigest, package: internal/wiring}
- [x] Doctor `wiring_drift`: one row over hooks, stanza regions, MCP entries and owned skills — PASS at zero drift, FAIL naming each drift, SKIP with the reason when a file is unreadable (I38) {id: PC-29, symbol: checkWiringDrift, package: cmd/fathom}
- [x] Custodian concern `wiring_current` and a session attention line on drift (`wiringDriftAttention`) {id: PC-30, symbol: wiringDriftAttention, package: cmd/fathom}
- [-] A separate `.fathom/trust.lock` file and an `internal/trustlock` package {id: PC-31, disposition: refused, reason: "the law-15 digests already cover hooks and stanzas; a second lock file widens the committed file list for no new evidence; the principal check is heuristic (P3)", trigger: "an incident where a drifted MCP entry or skill was not caught by reconcile"}

## Wave 5 — index truth (O21, O22, found by the 2026-09-02 integration)

- [x] Symbol keys carry the package path, not only the package name: two `package main` binaries with a same-named function never collapse to one node (`callgraph.Build` is last-writer-wins today, and the collision silently refuted five unrelated rows); verify's `package:` filter resolves against the qualified key; the reachfix harness gains a two-mains fixture {id: PC-32, symbol: TestTwoMainsSameFuncNameNeverCollapse, package: internal/fusion, skip_test: true, skip: "the test is the anchor; the code change is in internal/core and internal/fusion"}
- [x] Ingest refuses to re-root an existing service to a path that is a git worktree of that service's recorded root (or any path under `.claude/worktrees/`), naming the refusal (I30 extended); a post-commit hook run from such a worktree refreshes nothing and says so on stderr; `list_services` is the record of truth {id: PC-33, symbol: TestIngestRefusesWorktreeReroot, package: internal/mcp, skip_test: true, skip: "the test is the anchor; the CLI ingest path shares the check"}

## Wave 6 — one read manifest

- [x] `internal/readset` is deleted onto agentic-kit's `foundations/telemetry/read-manifest` (the kit package generalised from it, D13's second consumer being the kit's own constitution-lint); every reporting-layer read routes through the kit package; ADR-027's four Done-needs tests keep their names and stay green {id: PC-34, symbol: TestUnreadableInputNeverLooksHealthier, package: internal/intent, skip_test: true, skip: "the test is the anchor; the deletion is the evidence (git ls-files internal/readset is empty)"}
