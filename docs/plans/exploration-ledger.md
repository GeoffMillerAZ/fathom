# exploration-ledger — vision 19 promoted: the 90x measured, the dashboard readable

> Governing source: docs/vision/19-exploration-ledger.md. **Covenant
> (ADR-0070 class):** failing test first; `task validate` green before
> commit; dated CHANGELOG entry; boxes flip only on `fathom verify`.

## A — The instrument

- [x] the transcript parser extracts the exploration ledger: raw-exploration calls and result-token estimates (Grep/Glob/Read), fathom-call counts and result-token estimates (mcp fathom tool_use), sidechain count, and duplicated reads across sidechains {id: EX-1, symbol: summarizeTranscriptBytes, package: cmd/fathom, reachable_from: TestExplorationLedger}
- [x] the session ledger carries the exploration fields; every estimate is labeled an estimate and absences carry notes {id: EX-2, symbol: buildSessionLedgerEntry, package: cmd/fathom, reachable_from: TestLedgerExplorationFields}
- [x] the impact model folds exploration across sessions: per-question token cost for raw vs indexed discovery, the observed ratio as a labeled distribution (median plus count), and the duplication counter {id: EX-3, symbol: buildImpactModel, package: cmd/fathom, reachable_from: TestImpactExplorationFold}

## B — The story

- [x] the impact page leads with the exploration story: raw vs indexed cost per discovery question, the observed ratio labeled OBSERVATIONAL with the arm pilot named as the causal instrument, and the subagent duplication number {id: EX-4, symbol: renderObsImpact, package: cmd/fathom, reachable_from: TestImpactExplorationStory}
- [x] every observatory page opens with a plain-English lead and carries a no-JS explainer; jargon terms carry inline plain meanings {id: EX-5, symbol: obsPage, package: cmd/fathom, reachable_from: TestPlainEnglishLayer}
- [x] GET /observatory/brief renders the management-ready brief as a page with a markdown download; same numbers, framed for a reader who has never heard of fathom {id: EX-6, symbol: observatoryBriefPage, package: cmd/fathom, reachable_from: TestObservatoryBriefPage}

## C — Configuration observability

- [x] the service page gains the configuration panel: layers installed with eval state, guard scopes, binding file, and rule-injection counts; an unconfigured repo sees the install command {id: EX-7, symbol: buildConfigPanel, package: cmd/fathom, reachable_from: TestConfigPanel}

## D — Honesty upkeep

- [x] the wiring grade stops reading invisible-from-here as absent: signals that cannot be seen leave the denominator and the chip says what it could not see {id: EX-8, symbol: wiringGrade, package: cmd/fathom, reachable_from: TestWiringGradeUnknowns}

## F — Field report 2026-08-08 (the resume_as_code session; each row is a filed defect or its fix)

- [x] one store: CLI verbs that read (verify, plans, query, instrument) fall back to the daemon's registered store when no cwd-local store exists, loudly naming which store answered — the cwd-relative default silently proved nothing from consumer repos {id: FR-1, symbol: discoverPlansForServiceRoot, package: cmd/fathom, reachable_from: TestResolveReadStore}
- [x] the verify-catch aggregate stops double-counting: the 264,487-catches artifact is found and fixed; gate-shaped folds count events, never cumulative re-reads {id: FR-2, symbol: gateCatchStatsFromLedger, package: cmd/fathom, reachable_from: TestAdoptionVerifyFigureMatchesLedger}
- [x] skills ship with wiring: fleet reconcile detects missing shipped skills per repo and --apply installs them (the Unknown-skill failure) {id: FR-3, symbol: analyzeWiring, package: cmd/fathom, reachable_from: TestReconcileSkills}
- [x] the session hook teaches the first query with a LIVE example from the repo's own index (a real symbol, the exact call) — the first-query experience decides adoption; one miss loses the session to grep {id: FR-4, symbol: liveExampleLine, package: cmd/fathom, reachable_from: TestSessionHookLiveExample}
- [x] the observatory config panel states the obligations-compile condition per service (never compiled / no daemon key / last cost) so 6,985 staleness-warning serves cannot hide {id: FR-5, symbol: buildConfigPanel, package: cmd/fathom, reachable_from: TestConfigPanel}

## G — The tool diet (owner question 2026-08-08: 88 tools is an anti-pattern)

- [x] the context tax is measured: serialized schema bytes of each surface as an observatory panel — the standing per-session cost fathom imposes before any work happens {id: TD-1, symbol: surfaceSchemaTax, package: cmd/fathom, reachable_from: TestSurfaceSchemaTax}
- [x] the agent surface goes on a diet: at most 16 intent-named tools on /rpc/agent (the current golden set plus batch; shrinking below needs zero-call evidence) (the discovery quartet, obligations, verify_plan, read_source, ingest, batch); the long tail stays on /rpc; every removal is a measured zero-call row, never a guess {id: TD-2, symbol: AgentSurfaceTools, package: internal/mcp, reachable_from: TestAgentSurfaceDiet}
- [-] the 72 registered-but-not-exposed tools get a prune-or-expose verdict each {id: TD-3, disposition: deferred, reason: "needs TD-1's measurements plus one owner review pass over the list", trigger: "TD-1 panel live for a week"}

## E — Deferred with triggers

- [-] CompileLayer speaks the provider registry: a claude-cli profile compiles obligations without a raw API key (O13) {id: FR-6, disposition: deferred, reason: "needs the provider abstraction extended into the daemon compile path", trigger: "next daemon-LLM wave, or the ANTHROPIC_API_KEY workaround proving too brittle"}

- [-] fathom bench exploration: the reproducible 90x harness with published methodology over a public corpus {id: EX-9, disposition: deferred, reason: "the in-session ledger carries the internal story; a bench harness is for citable external claims", trigger: "first external-facing claim that needs a reproducible artifact"}
- [-] exploration-aware session-start guidance: nudge an agent toward the index when raw exploration passes a threshold mid-session {id: EX-10, disposition: deferred, reason: "measure first - the ledger must show where the threshold lives", trigger: "two weeks of exploration-ledger data"}
