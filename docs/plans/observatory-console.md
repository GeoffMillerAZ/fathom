# The Observatory console — the plan (vision 35, ADR-045)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/observatory-console.md` proves it.
Design: `docs/design/observatory-console.md`. Every page row also needs the doctrine's
browser proof (both themes, 1440 and 390 px) recorded in the wave's reading before the
wave is called done.

## A. Foundations

- [x] Page projections carry a truth state per value (present, zero, absent, stale, partial, error) with reason, source, denominator and time {id: OC-1, symbol: Value, package: internal/console, reachable_from: TestValue_TruthStatesRoundTripJSON}
- [x] The call log is resolved once — the daemon's configured log, then config, then the legacy relative path — and every metrics verb and page uses it, naming which rule chose it {id: OC-2, symbol: ResolveCallLog, package: internal/console, reachable_from: TestResolveCallLog_PrefersDaemonConfiguredLog}
- [x] Pages register in one table that drives routing, navigation, search and the JSON parity test {id: OC-3, symbol: registerPage, package: cmd/fathom, reachable_from: TestConsoleRegistry_RoutesNavAndSearchAgree}
- [x] The shell has grouped navigation, breadcrumbs, a page heading with meta and actions, a theme control and a skip link {id: OC-4, symbol: renderConsoleShell, package: cmd/fathom, reachable_from: TestConsoleShell_GroupsBreadcrumbsThemeSkipLink}
- [x] Error pages (404, 400, unknown service, unreadable store) render on the shell with the reason and the remedy {id: OC-5, symbol: renderConsoleError, package: cmd/fathom, reachable_from: TestConsoleErrors_RenderOnShellWithRemedy}
- [x] `/statusz` renders on the shell and shows an unknown typed-edge state as unknown, JSON v3 unchanged {id: OC-6, symbol: renderStatusz, package: cmd/fathom, reachable_from: TestStatusz_OnShellUnknownTypedEdgesVisible}
- [x] Old `/observatory*` HTML routes redirect to their new homes and their `?format=json` keeps observatoryModel v1 {id: OC-7, symbol: legacyObservatoryRedirect, package: cmd/fathom, reachable_from: TestLegacyObservatoryRoutes_RedirectAndKeepJSON}
- [x] Every page's JSON equals the `--json` output of each verb it names, on the fixture fleet {id: OC-8, symbol: registerPage, package: cmd/fathom, reachable_from: TestConsolePagesRenderTheirVerbs}
- [x] A metric shown on several pages has one projection, and every page agrees on it {id: OC-9, symbol: registerPage, package: cmd/fathom, reachable_from: TestConsoleMetricsAgreeAcrossPages}
- [x] Panels carry a copyable verb chip, a question, and a footer naming source, denominator and freshness {id: OC-10, symbol: renderPanel, package: cmd/fathom, reachable_from: TestConsolePanels_CarryVerbQuestionAndSource}
- [x] `/search` returns grouped results (projects, plans and rows, symbols, rules and records, verbs, pages) as a page and as a fragment {id: OC-11, symbol: SearchWith, package: internal/console, reachable_from: TestSearch_GroupsAndRanksResults}
- [x] The ⌘K palette opens over any page, moves with the arrow keys, opens on Enter and returns focus on Esc {id: OC-12, oracle: lexical, path: cmd/fathom/console_palette.go, pattern: "palette"}
- [x] A plan result in search, the ⌘K palette and the JSON opens its plan page and a row result its row page, an exact row ID first; rows are read with the parser the row page resolves them with {id: OC-55, symbol: projectSearchDocs, package: cmd/fathom, reachable_from: TestConsoleSearch_PlanAndRowResultsOpenTheirPages}
- [x] `/events` streams ingest, compile, verify, session and daemon events with IDs, keepalives, replay and a subscriber bound {id: OC-13, symbol: EventBus, package: internal/console, reachable_from: TestEventBus_ReplayBoundAndKeepalive}
- [x] A panel shows "updated" when an event names its source and never moves content {id: OC-14, oracle: lexical, path: cmd/fathom/console_live.go, pattern: "data-console-source"}
- [x] Console page views are counted per route for the panel-traffic measurement {id: OC-15, symbol: countPageView, package: cmd/fathom, reachable_from: TestConsolePageViews_Counted}
- [x] Warm renders stay within 150 ms and 250 KB on the fixture fleet {id: OC-16, symbol: registerPage, package: cmd/fathom, reachable_from: TestConsoleBudgets}

## B. Now and the fleet

- [x] The attention queue orders problems by what blocks proof first, each with its area, evidence link and remedy {id: OC-17, symbol: AttentionQueue, package: internal/console, reachable_from: TestAttentionQueue_OrdersByProofBlockingFirst}
- [x] "Since you last looked" lists pushes, deploys, measurements and restarts from the ledgers and daemon state {id: OC-18, symbol: SinceLastLook, package: internal/console, reachable_from: TestSinceLastLook_FromLedgers}
- [x] The fleet page is a sortable, filterable table with a health rollup where unknown is never green {id: OC-19, symbol: FleetTable, package: internal/console, reachable_from: TestFleetTable_SortFilterAndUnknownNotGreen}
- [x] Sparklines across projects share one scale and never draw across an absent day {id: OC-20, symbol: FleetTable, package: internal/console, reachable_from: TestFleetTable_SharedScaleAndAbsentDays}

## C. Proof

- [x] The plans portfolio shows a proof bar per plan across the fleet {id: OC-21, symbol: PlanPortfolio, package: internal/console, reachable_from: TestPlanPortfolio_ProofBarsPerPlan}
- [x] A plan page shows its proof bar, a board of rows by state with each row's contract kind, and a burn-up from the plan's history {id: OC-22, symbol: PlanLedger, package: internal/console, reachable_from: TestPlanLedger_BoardAndBurnup}
- [x] A row's evidence names its contract, verify result, the reach path or oracle match, and its history {id: OC-23, symbol: RowEvidence, package: internal/console, reachable_from: TestRowEvidence_ReachPathAndOracleMatch}
- [x] A plan page lists every remaining row in one table of its own, never capped by the board's lanes nor behind a disclosure, each row in exactly one of the page's tables {id: OC-56, symbol: renderPlanRemaining, package: cmd/fathom, reachable_from: TestConsolePlanRemaining_ListsEveryRow}
- [x] A plan page's remaining rows are the rows `plans next` lists, in document order, counted as `plans status` counts them, a lower bound while claims await their verdicts, and its JSON carries them {id: OC-57, symbol: Remains, package: internal/console, reachable_from: TestPlanLedger_RemainingIsWhatPlansNextLists}
- [x] The certifications page shows targets × certificates with states and the gap list {id: OC-24, symbol: CertMatrix, package: internal/console, reachable_from: TestCertMatrix_TargetsByState}
- [x] The project overview renders the realization ladder with evidence per rung and the remedy on the first unmet rung (RG-5) {id: OC-25, symbol: RealizationLadder, package: internal/console, reachable_from: TestRealizationLadder_RemedyOnFirstUnmetRung}
- [x] The refactor page shows plane chips (a plane not run reads "not run") and the reading set with a code excerpt per partial item {id: OC-26, symbol: RefactorView, package: internal/console, reachable_from: TestRefactorView_NotRunIsNeverConserved}

## D. Knowledge

- [x] The strata view places every layer in its band with current claims, lifecycle, runs and cost {id: OC-27, symbol: Strata, package: internal/console, reachable_from: TestStrata_BandsAndLifecycle}
- [x] The coverage matrix shows claims per project and layer, missing drawn as not compiled {id: OC-28, symbol: LayerCoverage, package: internal/console, reachable_from: TestLayerCoverage_MissingIsNotZero}
- [x] A layer page shows compile runs with cost, accepted and rejected counts, rejection reasons and uncompiled scopes {id: OC-29, symbol: LayerDetail, package: internal/console, reachable_from: TestLayerDetail_RunsAndRejections}
- [x] A claim page shows the statement, citation excerpt, scope, tier, status and event history {id: OC-30, symbol: ClaimDetail, package: internal/console, reachable_from: TestClaimDetail_EventsAndCitation}
- [x] The packs page shows dialect packs as a language × capability matrix with measured precision where a reading exists, and layer packs with where each is active {id: OC-31, symbol: PackMatrix, package: internal/console, reachable_from: TestPackMatrix_MeasuredOrSaidUnmeasured}
- [x] `obligations --explain` returns every candidate with its scope tier, the ranking, the cap, the served list and each displaced claim's reason; the default output is unchanged {id: OC-32, symbol: ExplainObligations, package: internal/semantic, reachable_from: TestExplainObligations_DisplacedReasons}
- [x] Serving never serves two claims whose normalized statements are identical {id: OC-33, symbol: ExplainObligations, package: internal/semantic, reachable_from: TestObligations_DedupesIdenticalStatements}
- [x] The serving explainer draws the scope columns, the cap and the served list for a chosen file {id: OC-34, oracle: lexical, path: cmd/fathom/console_explain.go, pattern: "ExplainObligations"}

## E. The project workspace

- [x] The project workspace has tabs that are URLs: overview, index, governance, plans, proof, refactor, intent, activity, wiring {id: OC-35, symbol: projectTabs, package: cmd/fathom, reachable_from: TestProjectWorkspace_TabsAreURLs}
- [x] The index tab shows languages with engine and tier, typed share, edge kinds and inventory keys {id: OC-36, symbol: IndexProfile, package: internal/console, reachable_from: TestIndexProfile_TiersEdgesInventory}
- [x] The code neighborhood shows a symbol's callers and callees with resolved and heuristic edges distinct, aggregating beyond a bound {id: OC-37, symbol: Neighborhood, package: internal/console, reachable_from: TestNeighborhood_BoundedAndAggregated}
- [x] `intent audit --read-only` renders the audit without appending ledger rows, and the intent tab uses it {id: OC-38, symbol: IntentView, package: internal/console, reachable_from: TestIntentView_ReadOnlyAuditWritesNothing}
- [x] The activity tab lists a project's sessions and tool calls from the resolved call log {id: OC-39, symbol: ProjectActivity, package: internal/console, reachable_from: TestProjectActivity_FromResolvedLog}
- [x] The wiring tab shows doctor checks, drift and receipts beside the existing preview, apply and undo {id: OC-40, symbol: WiringView, package: internal/console, reachable_from: TestWiringView_DoctorDriftReceipts}

## F. Evidence and system

- [x] Impact orders its questions, draws arms as ranges with counts, and says "not instrumented" instead of a colored zero {id: OC-41, symbol: ImpactView, package: internal/console, reachable_from: TestImpactView_NotInstrumentedNeverZero}
- [x] Catches (verify refusals, conserve losses, integrity catches, guard refusals) render on one filterable timeline {id: OC-42, symbol: CatchTimeline, package: internal/console, reachable_from: TestCatchTimeline_KindsAndFilters}
- [x] The sessions page is a table of sessions, and replay is windowed to a bounded number of events per response {id: OC-43, symbol: SessionWindow, package: internal/console, reachable_from: TestSessionWindow_Bounded}
- [x] Operate shows the daemon, per-surface traffic with p50, p95 and errors, tools by caller class, the zero-call list, the flywheel, config explain and the harness matrix {id: OC-44, symbol: OperateView, package: internal/console, reachable_from: TestOperateView_SurfacesAndCallerClasses}
- [x] The decision-port page shows the provider, its sites, the ruling log and the latest calibration table {id: OC-45, symbol: DecideView, package: internal/console, reachable_from: TestDecideView_LogAndCalibration}

## G. Learn

- [x] How fathom works draws the pipeline with live counts for a chosen project, each stage linking to its page {id: OC-46, symbol: PipelineView, package: internal/console, reachable_from: TestPipelineView_StagesAndCounts}
- [x] Ask the right way maps question shapes to tools, each with a read-only try-it answer on the chosen project {id: OC-47, symbol: AskGuide, package: internal/console, reachable_from: TestAskGuide_QuestionShapesToTools}
- [x] The use-case guides render with a table of contents, beside a glossary and the laws {id: OC-48, oracle: lexical, path: cmd/fathom/console_learn.go, pattern: "RenderToCRail"}

## H. App-kit contributions (in app-kit, fathom as originating consumer)

- [ ] (op) OC-49: FATHOM-KIT-01 the command palette lands in app-kit as an experimental capability with a Gallery specimen and keyboard evidence, and fathom uses it {id: OC-49, skip: "cross-repo: app-kit"}
- [ ] (op) OC-50: FATHOM-KIT-02 the layered graph renderer lands in app-kit (L3a, golden SVG, no class attributes, bounded nodes) and fathom uses it {id: OC-50, skip: "cross-repo: app-kit"}
- [ ] (op) OC-51: FATHOM-KIT-03 heatmap truth states land in app-kit and fathom's coverage matrix uses them {id: OC-51, skip: "cross-repo: app-kit"}
- [ ] (op) OC-52: FATHOM-KIT-04 the code excerpt lands in app-kit and fathom's refactor and claim pages use it {id: OC-52, skip: "cross-repo: app-kit"}
- [ ] (op) OC-53: FATHOM-KIT-05..08 unified diff, live freshness, page heading and catalog entries land in app-kit {id: OC-53, skip: "cross-repo: app-kit"}
- [x] Fathom's mirror is pinned to the app-kit revision carrying its contributions, and each contribution record in docs/app-kit.md names its delivery {id: OC-54, oracle: lexical, path: docs/app-kit.md, pattern: "FATHOM-KIT-01"}

## I. Measurements

- [ ] (op) UX-M1: ten operator tasks answered in three clicks or fewer from Now, each answer equal to its verb's JSON {id: UX-M1, skip: "operator measurement 2026-09-26 (docs/bench/readings/ux-m1-m2-console-2026-09-26.md): fails narrowly — 9 of 10 tasks in three clicks or fewer at 1440 px (7 of 10 at 390 px), all 10 answers equal their verb; plan rows remaining takes 4 because the board lists 12 cards per lane and search links plans and rows to the project page"}
- [ ] (op) UX-M2: every page passes both themes, 390 px with no horizontal overflow, a keyboard path, no console errors and its budgets {id: UX-M2, skip: "operator measurement 2026-09-26 (docs/bench/readings/ux-m1-m2-console-2026-09-26.md): not certified — Chrome aborts at launch, so themes, 390 px overflow, console errors and focus are unmeasured; the static pass holds on all 31 pages and HTML is 49-193 KB, but warm renders stall 3.6-4.4 s once per 30 s model rebuild and 22 s on the intent tab"}
- [ ] (op) UX-M4: after 30 days of page-view counting, pages nobody opened are reviewed under law 8 {id: UX-M4, skip: "operator measurement: 30-day window"}

## Deferred (trigger on file)

- [-] Hosted or shared views of any page {id: OC-60, disposition: deferred, reason: "law 13; non-goal", trigger: "the hosted-view revisit condition in scope.md fires"}
- [-] A client-side charting or graph library {id: OC-61, disposition: deferred, reason: "kit SVG primitives carry every current view (vision 17, F7)", trigger: "a real dataset misses a render budget with the SVG renderer"}
