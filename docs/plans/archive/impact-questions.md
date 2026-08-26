# impact-questions — vision 18 promoted: instruments, stories, the v2 observatory

> **Archived** 2026-08-26T00:16:59Z · HEAD `6cd778f3d90aff98871bfb3253a5274da48831fd` · 15 tasks total; 12 claimed (12 verified, 0 failing); 3 skipped

> Governing source: docs/vision/18-impact-questions.md. **Covenant
> (ADR-0070 class):** failing test first; `task validate` green before
> commit; dated CHANGELOG entry; boxes flip only on `fathom verify`.

## A — Instruments (the data v1 could not see)

- [x] call-log entries carry result_count and matched_via: hits, did_you_mean consolations and empty misses are distinguishable per tool per service {id: IQ-1, symbol: NewCallLogger, package: internal/mcp, reachable_from: TestCallLogOutcomes}
- [x] the ledger carries first_edit_turn: how many turns a session spent before its first edit; nil with a note when no transcript or no edit {id: IQ-2, symbol: summarizeTranscriptBytes, package: cmd/fathom, reachable_from: TestFirstEditTurn}
- [x] the miss fold: per-tool hit/miss/consolation rates computed from call outcomes into the observatory model; rows logged before IQ-1 count as outcome-not-recorded, never as hits (query TEXT stays out of the log deliberately - it is user-global and metadata-only) {id: IQ-3, symbol: buildMissModel, package: cmd/fathom, reachable_from: TestMissModel}

## B — The story layer

- [x] GET /observatory/session?id=S renders one session as a viz.Timeline: beats, tool calls with outcomes, story rows, the ledger row's verdict — the session replay {id: IQ-4, symbol: observatorySessionPage, package: cmd/fathom, reachable_from: TestObservatorySessionPage}
- [x] GET /observatory/story composes the trailing 7 days from ledger, catches, misses and observations — template-composed, lens-labeled; the LLM weekly stays a deferred row {id: IQ-5, symbol: observatoryStoryPage, package: cmd/fathom, reachable_from: TestObservatoryStoryPage}
- [x] the landing page leads with the four questions, each answered with its live number and linking to its screen {id: IQ-6, symbol: landingHandler, package: cmd/fathom, reachable_from: TestLandingFourQuestions}

## C — The v2 impact screen

- [x] /observatory/impact restructures into the four questions: Prevented (catches + served rules), Saved (arm ranges or the DARK estimator with its unlock command), Friction (miss table + open observations), Compounding (first-edit-turn trend, served-rules growth) {id: IQ-7, symbol: renderObsImpact, package: cmd/fathom, reachable_from: TestImpactFourQuestions}
- [x] fleet tiles gain per-service 14-day traffic sparklines from the call log, the grid sorts by activity, and a top-tools bar chart answers which tools carry the load (owner ask 2026-08-08) {id: IQ-8, symbol: renderObsFleet, package: cmd/fathom, reachable_from: TestFleetSparklines}
- [x] every service carries a wiring grade — hooks, stanza, freshness, traffic and guard file folded to one letter with the one command that raises it; the continuous form of the doctor/rewire audit (owner ask 2026-08-08) {id: IQ-15, symbol: wiringGrade, package: cmd/fathom, reachable_from: TestWiringGrade}

## D — The tool improves itself

- [x] narrate and arm accept humane durations: --for 14d parses (days), with the Go-duration forms still valid {id: IQ-9, symbol: parseHumaneDuration, package: cmd/fathom, reachable_from: TestParseHumaneDuration}
- [x] fathom init arms a bounded auto-narrate window (loudly announced, self-expiring) so adoption's first sessions are captured {id: IQ-10, symbol: armAutoNarrate, package: cmd/fathom, reachable_from: TestInitAutoNarrate}
- [x] the brief tells the four-question story: sections reorganized to Prevented / Saved / Friction / Compounding with the same honesty contract {id: IQ-11, symbol: renderBriefMD, package: cmd/fathom, reachable_from: TestBriefFourQuestions}

## E — Deferred with triggers (named, not vaporware)

- [-] the session-start context pack: inject a repo brief and open plan rows at SessionStart to collapse time-to-context directly {id: IQ-12, disposition: deferred, reason: "changes what every session sees - needs IQ-2 baselines first so the effect is measurable", trigger: "two weeks of first_edit_turn data"}
- [-] the obligation-conformity checker: served rule then edit-conformed, measured not assumed {id: IQ-13, disposition: deferred, reason: "needs a per-rule machine-checkable predicate design", trigger: "first layer whose rules carry checkable predicates"}
- [-] the LLM weekly narrative on /observatory/story {id: IQ-14, disposition: deferred, reason: "template composition first - the LLM adds prose, not truth", trigger: "daemon LLM configured and two weeks of story rows"}
