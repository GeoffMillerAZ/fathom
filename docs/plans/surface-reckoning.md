# The surface reckoning — the plan (feature scorecard 2026-09-27)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/surface-reckoning.md` proves it.
Design: `docs/design/surface-reckoning.md` (the verdict tables, the deletion-safety rule in
§2, the refinements in §8). Row ids are stable: `SR-<n>` work rows, `SR-M<n>` measurements.

Every removal row obeys design §2 in the commit that claims it: callers evidence
(`safe_delete_check` / `find_callers` on each removed handler) in the `Verified:` line, the
exemption check re-run (in-repo MCP callers rewired, skills and `docs/use-cases/` worked
examples rewritten), dependent claimed rows re-anchored or retired to `[-]` with
`disposition: refused, reason: "retired by SR-<n> (surface reckoning)"`, and the tool named
under `### Breaking` with its replacement in `docs/upgrades/0.9.0-surface-reckoning.md`.
Absence rows are `gone:` contracts: proven only when the index is at HEAD (I44), never
falsely.

## Wave 1 — live bugs the scorecard found (fixed before this plan)

- [x] The edit hook's spec references resolve an unresolved identity to the service the root's fathom stanza names, not the directory basename (170 of 306 `spec_refs` calls failed on the old name; fix `2b7b368d`) {id: SR-1, symbol: serviceCandidates, package: cmd/fathom, reachable_from: TestEditHookSpecRefsUseStanzaNamedService}
- [x] `doctor` asks `tools/list` before calling `list_modules` and never sends the doomed call (338 call-log errors; fix `edcc2352`) {id: SR-2, symbol: moduleListForDoctor, package: cmd/fathom, reachable_from: TestModuleListForDoctorNeverProbesAnUnadvertisedTool}

## Wave 2 — deletions and merges (law 4)

### 2.1 DITCH (design §3)

- [x] F-17, F-18: `cross_repo_blast_radius` and `dep_drift` are unregistered; `withServiceRefHops` stays for blast_radius and review_diff; `internal/depgraph` stays for N1 {id: SR-10, gone: pattern "Name:\s+.(cross_repo_blast_radius|dep_drift).", paths: "internal/mcp/"}
- [ ] F-19: `package_deps` stays deleted and no record still names it as live — the roadmap memo's N1 and vision 15 §3 are corrected {id: SR-11, gone: pattern "package_deps.*exist", paths: "docs/vision/15-reach.md"}
- [ ] F-22: `http_client_calls`, `fleet_drift`, `reachable`, `traverse` and `history` are unregistered; verify's `reachable_from` is unaffected (it never called the tool) {id: SR-12, gone: pattern "Name:\s+.(http_client_calls|fleet_drift|reachable|traverse|history).", paths: "internal/mcp/"}
- [ ] F-45: anchored memory is removed — `memory_search`, `fathom memory`, the edit hook's memory injection and `internal/memory`; AM-1..AM-11, DF-16, DF-17 and RT-36 are retired to `[-]` in the same commit {id: SR-13, gone: import github.com/geoffmilleraz/fathom/internal/memory}
- [x] F-53: the `narrate` and `brief` CLI verbs are removed with story-mode rows; `renderBriefMD` and `briefTuning` stay for the console's `/brief` page; IO-21, IO-22 and OB-7 are retired {id: SR-14, gone: pattern "case .(narrate|brief).:", paths: "cmd/fathom/main.go"}
- [x] F-53 (refined): the MCP `brief` tool (worker pre-pack, 0 lifetime calls) is unregistered; D1-1 and D1-2 are retired {id: SR-15, gone: pattern "Name:\s+.brief.", paths: "internal/mcp/"}
- [x] F-53 (refined to freeze): `fathom research` leaves top-level help and stays callable, because ADR-046 decision 6 routes the retro corpus through its bundle {id: SR-16, symbol: printUsage, package: cmd/fathom, reachable_from: TestUsageOmitsFrozenVerbs}
- [ ] F-68: the four module MCP tools, the `/rpc/modules` surface and hosted-modules mode are removed; the Expert Module v0 manifest, `modules.cue`, `internal/modspec` and the `fathom module` verbs stay; GKE-MH-5 is retired; the `module_query` wire contract leaves the compatibility surface with a Breaking line {id: SR-17, gone: pattern "Name:\s+.(list_modules|module_obligations|module_query|module_staleness).", paths: "internal/mcp/"}
- [x] F-70: `fathom assess` leaves top-level help (frozen under ADR-046, capability default off); `internal/assess` stays {id: SR-18, symbol: printUsage, package: cmd/fathom, reachable_from: TestUsageOmitsFrozenVerbs}
- [ ] F-77: the twelve legacy tools (`complexity`, `god_object`, `community`, `dead_code`, `cloc`, `secret_scan`, `shell_out`, `proto_conformance`, `traceability`, `evidence_summary`, `contradiction_scan`, `batch`) are unregistered; the governor's `batch` branches go; RT-60 is retired; the complexity column stays for U13 {id: SR-19, gone: pattern "Name:\s+.(complexity|god_object|community|dead_code|cloc|secret_scan|shell_out|proto_conformance|traceability|evidence_summary|contradiction_scan|batch).", paths: "internal/mcp/"}
- [x] F-78: the four canned prompts are removed; `prompts/list` answers an empty list; the dead `onboard_service` strings in the governor and tenancy go {id: SR-20, gone: pattern "(investigate_incident|onboard_service|plan_requirement|review_change)", paths: "internal/,cmd/fathom/"}

### 2.2 MERGE (design §4)

- [x] F-05: `find_callees` is unregistered; `symbol_context` is the answer for callees {id: SR-21, gone: pattern "Name:\s+.find_callees.", paths: "internal/mcp/"}
- [x] F-10: `repo_map`'s header carries the entrypoints (capped at 10 with a `more` count) and the `entrypoints` tool is unregistered {id: SR-22, symbol: repoMapEntrypoints, package: internal/mcp, reachable_from: TestRepoMapHeaderCarriesEntrypoints}
- [x] F-11: one `status` tool answers the `tools`, `daemon` and `sources` sections byte-compatibly with `capabilities`, `server_stats` and `list_sources`, which are unregistered; doctor and the console call `status`; AD-1, AD-2, AD-4, A1-1, A1-2, K3-1, K4-1 and P1-7 are re-anchored onto `registerStatus` {id: SR-23, symbol: registerStatus, package: internal/mcp, reachable_from: TestStatusSectionsMatchRetiredTools}
- [x] F-13: `blast_radius` takes `kind: change|rename|delete`; `rename` returns today's rename_impact answer and `delete` today's safe_delete_check answer (proven-zero receipt included); both tools are unregistered and `promotedVerbs()` drops `rename_impact` {id: SR-24, symbol: blastRadiusByKind, package: internal/mcp, reachable_from: TestBlastRadiusKindsMatchRetiredVerbs}
- [x] F-16: `diff_impact` is unregistered; `review_diff` is the verb {id: SR-25, gone: pattern "Name:\s+.diff_impact.", paths: "internal/mcp/"}
- [x] F-20: the `api_diff` tool is unregistered; the calibrated API delta stays in `conserve` {id: SR-26, gone: pattern "Name:\s+.api_diff.", paths: "internal/mcp/"}
- [x] F-35: `arch_conformance`, `spec_coverage` and `spec_refs` are unregistered; the edit hook reads spec references from an additive `spec_refs` section of the `obligations` answer; P1-4, C4-3 and C4-5 are re-anchored onto the review engine's `runArch` {id: SR-27, symbol: obligationsSpecRefs, package: internal/mcp, reachable_from: TestEditHookReadsSpecRefsFromObligations}
- [x] F-56: `staleness_report` carries `doc_drift` (doc spans naming a symbol that no longer exists) and `stale_docs` is unregistered {id: SR-28, symbol: stalenessDocDrift, package: internal/mcp, reachable_from: TestStalenessReportDocDrift}
- [x] F-47: the `discoveries` tool is unregistered and `routes` becomes `prime --routes`; RC-27 is re-anchored onto the session brief's discovery renderer {id: SR-29, gone: pattern "Name:\s+.discoveries.", paths: "internal/mcp/"}
- [x] F-44: `gen-status-cue` and `gen-plan-cue` leave top-level help; the ADR-018 round-trip is unchanged {id: SR-30, symbol: printUsage, package: cmd/fathom, reachable_from: TestUsageOmitsInternalVerbs}
- [x] F-52: `metrics` is the one verb (`instrument`, `flywheel`, `retro`, `rate`, `arm` as subverbs); the old verbs are hidden aliases for one release {id: SR-31, symbol: cmdMetrics, package: cmd/fathom, reachable_from: TestMetricsSubverbsMatchRetiredVerbs}
- [x] F-71: doctor has a `providers` row running the auth and one-token probes; `providers` and `smoke` are hidden aliases {id: SR-32, symbol: doctorProvidersRow, package: cmd/fathom, reachable_from: TestDoctorProvidersRow}

### 2.3 The surface

- [x] F-86: `/rpc/agent` serves the default eight (`find_symbol`, `search`, `read_source`, `symbol_context`, `obligations`, `verify_plan`, `blast_radius`, `review_diff`); `/rpc/extended` serves the 21-tool opt-in set and `/rpc/companion` aliases it; the diet test caps the default at 8 {id: SR-33, symbol: toolOnSurface, package: internal/mcp, reachable_from: TestDefaultSurfaceIsEight}
- [x] The fathom stanza stops telling agents to call `ingest` (auto-refresh queues it) and names only default-surface tools; `fathom init` writes the new stanza {id: SR-34, symbol: fathomStanzaBodyFor, package: cmd/fathom, reachable_from: TestStanzaNamesOnlyDefaultSurfaceTools}
- [x] The registry pin: no tool the reckoning removed is registered on any surface (the `surface_prune_test.go` precedent) {id: SR-35, symbol: NewServer, package: internal/mcp, reachable_from: TestRegistryCarriesNoReckonedTools}
- [ ] The migration note lists every removed tool, verb and prompt with its replacement, and `CHANGELOG.md` carries the `### Breaking` block {id: SR-36, oracle: lexical, path: docs/upgrades/0.9.0-surface-reckoning.md, pattern: "^\| `cross_repo_blast_radius` \|"}

## Wave 3 — IMPROVE (design §5)

- [x] F-02: `references` defaults to exact; a free-text query is answered by `search` with `routed_from: references`, never refused {id: SR-40, symbol: registerReferences, package: internal/mcp, reachable_from: TestReferencesRoutesFreeTextToSearch}
- [x] F-07: `repo_map`'s default budget is cut and its envelope says `orientation-only` {id: SR-41, symbol: registerRepoMap, package: internal/mcp, reachable_from: TestRepoMapDefaultBudgetOrientationOnly}
- [ ] (op) F-23: BP-46 is read; `pack_context` is kept with the reading cited, or folded into `query_symbols` and unregistered {id: SR-42, oracle: lexical, path: docs/bench/readings/bp-46-pack-context.md, pattern: "^Verdict: (keep|fold)"}
- [x] F-24: `query_symbols` opens with a one-line `answer:` and a lower default top-N {id: SR-43, symbol: querySymbolsAnswerLine, package: internal/mcp, reachable_from: TestQuerySymbolsAnswerFirst}
- [x] F-25: the prompt hook routes a stack trace, a failing test name or an error string to `localize` deterministically {id: SR-44, symbol: routeIssueShapedPrompt, package: cmd/fathom, reachable_from: TestPromptHookRoutesIssueShapesToLocalize}
- [ ] F-26: a checkable served rule compiles to an `enforce: true` guard and the `obligations` answer marks it checkable {id: SR-45, symbol: guardFromCheckableRule, package: internal/guardspec, reachable_from: TestCheckableRuleCompilesToGuard}
- [x] F-28: `fathom init` wires the prompt hook by default {id: SR-46, symbol: cmdInit, package: cmd/fathom, reachable_from: TestInitWiresPromptHookByDefault}
- [ ] F-29: guard verdicts are emitted as SARIF 2.1.0 {id: SR-47, symbol: guardVerdictsSARIF, package: internal/guardspec, reachable_from: TestGuardVerdictsSARIFGolden}
- [ ] F-31: when a served rule is violated, the next edit-hook turn asks for a one-key verdict and writes it as refute or corroborate {id: SR-48, symbol: askRuleVerdict, package: cmd/fathom, reachable_from: TestEditHookAsksVerdictOnIgnoredRule}
- [ ] F-34: `review_diff` emits the change risk receipt (U7) — blast radius, certificates, conservation, rules, reviewers — as SARIF and a GitHub check payload {id: SR-49, symbol: changeRiskReceipt, package: internal/analysis/review, reachable_from: TestChangeRiskReceiptGolden}
- [ ] F-15: `tests select` is certifying-first (T4): the minimal set re-proving every touched certificate, scored against a coverage oracle {id: SR-50, symbol: selectCertifyingFirst, package: cmd/fathom, reachable_from: TestTestsSelectCertifyingFirst}
- [ ] F-21: `cross_repo_callers` reads provides/consumes links from the system graph instead of regex detectors {id: SR-51, symbol: registerCrossRepoCallers, package: internal/mcp, reachable_from: TestCrossRepoCallersReadsSystemGraphLinks}
- [ ] (op) F-46: continuity threads reach the 20-unit sample and the flywheel reading keeps or kills them {id: SR-52, oracle: lexical, path: docs/bench/readings/threads-20-unit.md, pattern: "^Verdict: (keep|kill)"}
- [ ] F-51: `doctor --stranger` times install to first proven answer against a 5-minute bar (G16) with an injected clock {id: SR-53, symbol: doctorStrangerCheck, package: cmd/fathom, reachable_from: TestDoctorStrangerCheckTimed}
- [ ] (op) F-54: the plan-row list is reachable in three clicks from Now (UX-M1's one failing task) {id: SR-54, oracle: lexical, path: docs/bench/readings/ux-m1-console-rerun.md, pattern: "^10 of 10 tasks in three clicks"}
- [ ] F-57: reconcile is triggered from the console fleet page (user-triggered, never scheduled) and drift renders there {id: SR-55, symbol: consoleReconcileAction, package: cmd/fathom, reachable_from: TestConsoleFleetReconcileTrigger}
- [ ] (op) F-58: the stranger check is read on a clean machine and the plugin is published (the owner's Q4 call) {id: SR-56, oracle: lexical, path: docs/bench/readings/stranger-check.md, pattern: "^First proven answer in [0-5]:[0-5][0-9]"}
- [ ] F-61: RT-99 and RT-107 land and RT-M1's seven days are read (owned by the responsible-tenant plan) {id: SR-57, oracle: lexical, path: docs/plans/responsible-tenant.md, pattern: "^- \[x\].*id: RT-107,", needs: "responsible-tenant.md#RT-107"}
- [ ] F-69: every decision-port site outside certify is a proposal gated by a passing calibration and an A/B against the deterministic baseline; a site without both refuses to rule {id: SR-58, symbol: requireCalibratedSite, package: internal/decide, reachable_from: TestUncalibratedSiteRefusesToRule}

## Wave 4 — underused delivery (design §6)

- [ ] blast_radius (F-12): `review_diff` includes the blast-radius summary and the pre-edit hook adds one line of callers and tests reached {id: SR-60, symbol: editHookBlastLine, package: cmd/fathom, reachable_from: TestPreEditHookBlastLine}
- [ ] Per-rule counts (F-27, U4): the edit hook records served, complied and violated per claim id; `obligations` and the console show them {id: SR-61, symbol: recordRuleOutcome, package: cmd/fathom, reachable_from: TestEditHookRecordsRuleOutcomes}
- [ ] Certifications (F-41): liveness bound to the commit (O114), service resolution fixed, certificates declared on fathom itself {id: SR-62, symbol: certificateLiveAtCommit, package: internal/certify, reachable_from: TestCertificateLivenessBoundToCommit}
- [ ] Conservation (F-42): `fathom init` wires `conserve` on the pushed range into every adopter's pre-push gate by default (today only this repo's `lefthook.yml` runs it) and review shows its plane verdicts {id: SR-63, symbol: initPrePushConserve, package: cmd/fathom, reachable_from: TestInitWiresConservePrePush}
- [ ] (op) Findings loop (F-33): Review Kit publishing is armed on one real repository and an import receipt shows findings > 0 {id: SR-64, oracle: lexical, path: docs/bench/readings/findings-armed.md, pattern: "^Imported: [1-9]"}
- [ ] SCIP import (F-74): `fathom init` runs `scip-typescript` / `scip-python` when installed and imports the index {id: SR-65, symbol: initRunSCIP, package: cmd/fathom, reachable_from: TestInitRunsSCIPWhenInstalled}
- [ ] (op) ui-appkit pack (F-67): compiled on app-kit and one consumer; UI rule violations measured {id: SR-66, oracle: lexical, path: docs/bench/readings/ui-appkit-pack.md, pattern: "^Compiled claims: [1-9]"}
- [ ] Call log and hook log: one reader over both logs feeds the U1–U5 views {id: SR-67, symbol: ReadActivityRecord, package: internal/metrics, reachable_from: TestActivityRecordJoinsCallAndHookLogs}
- [ ] Verify and session ledgers: the join by session id that N2 burn-up and N3 cost-per-proven-row read {id: SR-68, symbol: JoinLedgersBySession, package: internal/metrics, reachable_from: TestJoinVerifyAndSessionLedgers}

## Measurements

- [ ] SR-M1: `tools/list` schema bytes on `/rpc/agent` before (18,310, 25 tools) and after, and on bare `/rpc` before (34,246, 59 tools) and after; the default is at most 9,000 bytes {id: SR-M1, symbol: surfaceSchemaBytes, package: internal/mcp, reachable_from: TestDefaultSurfaceSchemaBytesBudget}
- [ ] (op) SR-M2: agent tool choice on the next bench round — calls per tool, off-surface calls and turns to first answer, default surface vs the 25-tool agent surface {id: SR-M2, oracle: lexical, path: docs/bench/readings/surface-reckoning-tool-choice.md, pattern: "^Verdict: "}
- [ ] (op) SR-M3: fourteen days of off-surface demand on the default surface read from the call log; a tool called off-surface by agents on at least 5% of sessions is a candidate to return {id: SR-M3, oracle: lexical, path: docs/bench/readings/surface-reckoning-off-surface.md, pattern: "^Sessions: [0-9]+"}
