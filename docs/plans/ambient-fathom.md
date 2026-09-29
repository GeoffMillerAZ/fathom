# ambient fathom — phases T through Y

> Governing source: `docs/vision/20-ambient-fathom.md` (this plan is its
> binding promotion). Designs: `capability-registry.md`,
> `self-healing-fleet.md`, `scope-and-purge.md`, `anchored-memory.md`,
> `budgeted-answers.md`, `realization-grade.md`, `assess-lanes.md` (DO-3,
> IR-3..5; ADR-046) under `docs/design/`.
>
> **Covenant (ADR-0070 class):** failing test first, `task validate` green
> before commit, dated CHANGELOG entry, boxes flip only on
> `fathom verify --service fathom --plan docs/plans/ambient-fathom.md`.
>
> **Order is load-bearing.** T ships first because every later phase needs
> a toggle. U ships second because it is the only phase that stops the
> fleet decaying while the rest is built.

---

## T — Capability registry (`docs/design/capability-registry.md`)

- [x] one typed registry over the four existing seams; every entry carries id, seam, summary, default, scope rules, a `detect` function and a non-empty `why_off` — enforced by a test that iterates the registry so a capability cannot be added without a reason for its absence {id: CR-1, symbol: Default, package: internal/capability, reachable_from: TestRegistryEveryEntryExplainsItsAbsence}
- [x] `detect` returns operating-or-named-reason, never a bare boolean: a capability configured on but not observably working is off, with the reason (law 3 applied to configuration) {id: CR-2, symbol: DetectAll, package: internal/capability, reachable_from: TestDetectAllRespectsConfiguration}
- [x] scope resolution built-in → user → project returns the value AND the scope that supplied it; a user-scope file setting a project-only key is rejected with the key named, never ignored; an unknown id is rejected with the nearest known id suggested (law 13) {id: CR-3, symbol: ValidateSettings, package: internal/capability, reachable_from: TestResolveProvenanceAndRefusals}
- [x] the CUE schema is a closed struct over known ids and the loader refuses a known field declared optional in an instance — the phase-O trust-boundary inversion cannot recur {id: CR-4, symbol: LoadSettings, package: internal/capability, reachable_from: TestLoaderRefusesOptionalKnownField}
- [x] `fathom capabilities` / `--json` / `--service` / `explain` / `set`; explain states what it does, what it costs when on, what turns it on, and what evidence would appear on the dashboard {id: CR-5, symbol: cmdCapabilities, package: cmd/fathom, reachable_from: TestCmdCapabilities}
- [x] a capability is `switchable` ONLY when an enforcement site reads the resolution and acts on it: `obligations`, `guards` and `verify-gate` are gated before any daemon traffic, every other entry is status and `set` refuses it naming the real control; enforcement fails OPEN so a malformed config never disarms a safety feature {id: CR-7, symbol: capabilityOffAt, package: cmd/fathom, reachable_from: TestSwitchOffStopsGuards}
- [x] doctor gains a capabilities section where every off entry carries a reason and either a command or an explicit nothing-to-run; it produces zero known-false failures in a repository with no fathom configuration at all (the O23 class) {id: CR-6, symbol: checkCapabilities, package: cmd/fathom, reachable_from: TestDoctorCapabilitiesOnVirginRepo}

## T2 — The flywheel checks itself (owner ask, 2026-08-08)

- [x] `fathom flywheel` reports every feedback loop as CLOSED / OPEN / IDLE with its evidence and, when not closed, a remedy; a loop is CLOSED only when data is produced, arrives AND is consumed, so a producer that nobody reads is OPEN rather than healthy-looking {id: FW-1, symbol: runFlywheelChecks, package: cmd/fathom, reachable_from: TestFlywheelEveryCheckCarriesEvidence}
- [x] the arm loop detects BOTH 2026-08-08 failure signatures: armed-but-recording-nothing, and rows-arriving-none-admissible — the second is the subtle one, because the producing side looks alive {id: FW-2, symbol: checkLoopArm, package: cmd/fathom, reachable_from: TestFlywheelSeesRowsArrivingButInadmissible}
- [x] IDLE never fails the exit code and OPEN always does: a loop with nothing to measure yet is not a defect, and failing on it teaches people to ignore the check {id: FW-3, symbol: flywheelExit, package: cmd/fathom, reachable_from: TestFlywheelIdleIsNotFailure}

---

## M — Measurement integrity (found while setting up the A/B run, 2026-08-09)

- [x] the ledger counts SESSIONS, not stops: Claude Code's Stop event fires at every turn boundary, so 200 rows were 8 sessions; deduped at read time keeping the last row per session id, with the raw stop count still reported so the correction is auditable {id: MI-1, symbol: buildImpactModel, package: cmd/fathom, reachable_from: TestLedgerCountsSessionsNotStops}
- [x] `fathom rate <1-5>` records the self-rated outcome — the half of the pre-registered criterion that was never instrumented; joined to the ledger by session id at read time so a late rating still counts, and refused outright when it cannot be joined {id: MI-2, symbol: cmdRate, package: cmd/fathom, reachable_from: TestRateRecordsAndRerating}
- [x] every field the transcript parser computes reaches the ledger row, asserted over the STRUCT rather than a hand-listed set — the hand-copy it replaces silently disabled four instruments (arm coverage, the in/out token split, warm-up, the exploration ledger), each of which reported "rows predate the instrument" while the instrument was wired and its output discarded three lines later {id: MI-4, symbol: transcriptSummaryFrom, package: cmd/fathom, reachable_from: TestTranscriptSummaryCarriesEveryField}
- [x] the registered criterion is evaluated in words and names every unmet part — session floor per arm, missing medians, missing ratings — rather than reporting a bare "not yet" {id: MI-3, symbol: evaluateABCriterion, package: cmd/fathom, reachable_from: TestLedgerCountsSessionsNotStops}

## AU — Autonomy: the deploy stops needing hands (owner ask, 2026-08-12)

- [x] the daemon watches its own executable and, when the bytes change, VERIFIES the replacement runs before exiting so launchd's KeepAlive restarts it on the new code — an agent can build, sign and install, but cannot run `launchctl`, so every committed daemon fix used to wait for a human {id: AU-1, symbol: watchOwnBinary, package: cmd/fathom, reachable_from: TestSelfRestartAdoptsAVerifiedBinary}
- [x] a candidate that cannot execute is REFUSED and named, never adopted — the 2026-08-12 unsigned-binary failure (OS_REASON_CODESIGNING) would otherwise have become an unbounded restart loop against a binary that could never start {id: AU-2, symbol: binaryReports, package: cmd/fathom, reachable_from: TestSelfRestartRefusesABinaryThatCannotRun}
- [x] the bytes must settle across two consecutive polls, so a file still being written is never adopted half-copied; adoption is rate-limited and can be pinned off with FATHOM_SELF_RESTART=off {id: AU-3, symbol: selfRestartDisabled, package: cmd/fathom, reachable_from: TestSelfRestartDisabledByEnv}

---

## U — Self-healing fleet (`docs/design/self-healing-fleet.md`, law 15)

- [x] marker format v3 carries the template version and the sha256 of the region body as fathom last wrote it; a body whose digest does not match is drift {id: SH-1, symbol: parseOwnedRegion, package: internal/wiring, reachable_from: TestOwnedRegionDigestDetectsHandEdit}
- [ ] `.fathom/owned.json` records the keys fathom has added to structured files it does not own (lefthook.yml, .claude/settings.json, .gitignore); reconciliation touches only those keys and a key absent from the ledger is invisible to fathom {id: SH-2, symbol: OwnedKeys, package: internal/wiring, reachable_from: TestOwnedKeysBoundsStructuredEdits}
  <!-- product wiring open (fathom verify --strict reads it library-only, 2026-09-28): OwnedKeys has no production caller: nothing reads .fathom/owned.json when reconciling. -->
- [x] law 15: a drifted region is never merged — the item reports BLOCKED with both versions attached, the file on disk is byte-identical afterwards, and one BLOCKED item does not block the others in the same run {id: SH-3, symbol: PlanWiring, package: internal/wiring, reachable_from: TestDriftStopsAndReportsNeverMerges}
- [x] plan is always computed before any mutation; apply executes only items with verdict apply, and content outside fathom's regions survives byte-for-byte including line endings {id: SH-4, symbol: ApplyWiring, package: internal/wiring, reachable_from: TestApplyPreservesForeignContent}
- [x] applying twice adds fathom's entries exactly once — the `fathom init` hook-duplication regression {id: SH-5, symbol: ApplyWiring, package: internal/wiring, reachable_from: TestApplyIsIdempotent}
- [x] every apply writes a receipt carrying the full prior content of every region touched; `fathom fleet receipts` lists them and `fathom fleet undo` restores exactly, refusing when a file has changed since {id: SH-6, symbol: cmdFleetUndo, package: cmd/fathom, reachable_from: TestUndoRestoresOrRefuses}
- [x] a TORN apply still writes its receipt: any error mid-loop leaves files already rewritten, and discarding the receipt made undo impossible while `fleet receipts` falsely reported nothing was written (found in review) {id: SH-10, symbol: ApplyWiring, package: internal/wiring, reachable_from: TestTornApplyStillWritesAReceipt}
- [x] `fleet receipts --limit N` states the TOTAL and what the limit hid, never the truncated length as the total — law 3 applied to a count, in the verb whose only job is to tell the truth about what fathom changed (found in review) {id: SH-11, symbol: cmdFleetReceipts, package: cmd/fathom, reachable_from: TestReceiptsLimitDisclosesWhatItHid}
- [x] the observatory renders Preview changes → typed diff → Apply; apply is refused if the repository changed between preview and apply (plan digest mismatch) {id: SH-7, symbol: wiringApplyHandler, package: cmd/fathom, reachable_from: TestWiringApplyRequiresFreshPlan}
- [x] the mutating endpoints refuse a non-loopback Host, a cross-site Origin, a service id not in the registry, and any path not already a registered fathom service (law 13) {id: SH-8, symbol: requireLocalMutation, package: cmd/fathom, reachable_from: TestMutationEndpointRefusals}
- [x] time-to-remedy is measured and rendered: interval from a wiring gap first appearing to it closing, per repository, with the 2026-08-08 baseline recorded (9 of 17 below A, unbounded) {id: SH-9, symbol: timeToRemedy, package: cmd/fathom, reachable_from: TestTimeToRemedy}

---

## V — Scope and purge (`docs/design/scope-and-purge.md`)

- [x] suppression model in `~/.fathom/suppressions.jsonl`: pattern, required reason, optional expiry, provenance; expiry is evaluated at read time with no sweep, so a suppression expiring while the daemon is down is inactive on the next query {id: SP-1, symbol: LoadSuppressions, package: internal/scope, reachable_from: TestSuppressionExpiryAtReadTime}
- [x] the filter applies at exactly one READ chokepoint and covers every surface — a table test enumerates MCP tools, CLI verbs and HTTP routes and asserts a suppressed ref appears in none; a surface added without being listed fails {id: SP-2, symbol: ApplySuppressions, package: internal/scope, reachable_from: TestSuppressionCoversEverySurface}
- [x] every response that dropped content discloses the count and reason; a silent filter fails the test {id: SP-3, symbol: ApplySuppressions, package: internal/scope, reachable_from: TestSuppressionIsDisclosed}
- [x] `fathom scope list|suppress|unsuppress|preview`; preview's count matches exactly what suppress then hides {id: SP-4, symbol: cmdScope, package: cmd/fathom, reachable_from: TestCmdScopePreviewMatchesEffect}
- [x] path-scoped purge removes rows from every location enumerated in the design — observations, typed_edges, typed_edge_fingerprints, doc_summaries, fingerprints, knowledge_cards, semantic_claims, claim_links — proven per table {id: SP-5, symbol: PurgePaths, package: internal/store, reachable_from: TestPurgePathsClearsEveryTable}
- [x] purge flushes the result cache wholly (it is query-keyed, so selective removal is unsound) and deletes affected vectors; an answer cached before the purge is not served after it {id: SP-6, symbol: PurgePaths, package: internal/store, reachable_from: TestPurgeFlushesDerivedStores}
- [x] verification is part of the operation: after execute, every location is re-queried and must report zero; an artificially surviving row makes the run FAIL and names the table, never partial success {id: SP-7, symbol: verifyPurge, package: cmd/fathom, reachable_from: TestPurgeFailsLoudlyOnSurvivor}
- [x] purge writes an INGEST-time exclusion as well as a suppression — SERVICE-SCOPED by construction (`~/.fathom/ignore.d/<service>`, unioned at Discover for that service alone; a fleet-wide pattern is refused loudly, so `--all` can never wipe the fleet index) — because a query-layer filter cannot stop auto-refresh re-indexing what was just deleted; proven by a forced full re-ingest after purge leaving zero rows {id: SP-8, symbol: cmdPurge, package: cmd/fathom, reachable_from: TestPurgeSurvivesForcedReingest}
- [x] purge resolves the pattern against EACH location's own path column (observations/fingerprints/doc_summaries source_ref, knowledge_cards.file, typed_edges.from_file, ParseCiteRef over claim_links.target_ref) — `ListSourceRefs` alone misses every row anchored by another key {id: SP-11, symbol: resolvePurgeTargets, package: internal/store, reachable_from: TestPurgeResolvesRowsWithNoSourceRef}
- [x] deletion runs on a `secure_delete` connection followed by `wal_checkpoint(TRUNCATE)` and VACUUM; verification includes a RAW-BYTES probe of the store file, derived independently of the deletion — a re-query can only confirm the delete it came from {id: SP-12, symbol: purgePhysicalResidue, package: internal/store, reachable_from: TestPurgeLeavesNoBytesInStoreFile}
- [x] claim_events retention is disclosed with its count by default and deleted only under `--include-history --force`, which states that claim history integrity is now broken {id: SP-9, symbol: cmdPurge, package: cmd/fathom, reachable_from: TestClaimEventsRetentionDisclosed}
- [ ] the Scope panel on `/observatory/service`: suppressions with preview, and purge behind a second click requiring a displayed file list, a typed reason, and the service name typed to confirm; the verification result is rendered per table {id: SP-10, symbol: observatoryScopePanel, package: cmd/fathom, reachable_from: TestScopePanelPurgeGuards}

> **SP-10, 2026-09-25 (console wave E): the read-only half ships; the row stays open.** The project
> workspace's Index tab (`/projects/{svc}/index#scope`, `projectScopePanel`) lists the active
> suppressions, previews a glob against every location's own path column (`store.ResolvePurge`) and
> shows the purge's per-location dry run with the exact command only after the file list. No
> mutation is wired: the daemon has no purge path that also flushes its own live caches (the result
> cache, the call-graph memo, vectors — `purgecmd.go` leaves them nil and names a daemon purge path
> that does not exist), so a page-driven purge would verify the store while the process kept serving
> answers built from the deleted rows. Suppress and unsuppress behind `requireLocalMutation` land
> with it, so the panel's two operations ship together.

---

## W — Anchored memory (`docs/design/anchored-memory.md`, law 14)

- [-] the memory model: claim, ≥1 anchor, ≥1 evidence, derived tier, derived state; a memory with no anchor is REFUSED with the reason, never stored at a low tier {id: AM-1, disposition: refused, reason: "retired by SR-13 (surface reckoning, ADR-053): anchored memory — internal/memory, memory_search, the fathom memory verbs and the edit hook memory block — is removed; 0 memories in any repo"}
- [-] `ApplyCeiling` is called at the write path and the ceiling is derived structurally from the evidence, not from the caller's claimed tier — a lying caller lands at the ceiling with the cap and its reason recorded (law 14; this is `ApplyCeiling`'s first caller in fathom) {id: AM-2, disposition: refused, reason: "retired by SR-13 (surface reckoning, ADR-053): anchored memory — internal/memory, memory_search, the fathom memory verbs and the edit hook memory block — is removed; 0 memories in any repo"}
- [-] anchor drift is evaluated at READ time from hashes fathom already maintains, with no sweep, cached per index generation: forty surfacings of one memory hash its anchors once {id: AM-3, disposition: refused, reason: "retired by SR-13 (surface reckoning, ADR-053): anchored memory — internal/memory, memory_search, the fathom memory verbs and the edit hook memory block — is removed; 0 memories in any repo"}
- [-] fresh → suspect on anchor content change, labeled with what changed and when, tier dropped one step while suspect; suspect → orphaned on anchor deletion, and an orphaned memory does not appear in an injection {id: AM-4, disposition: refused, reason: "retired by SR-13 (surface reckoning, ADR-053): anchored memory — internal/memory, memory_search, the fathom memory verbs and the edit hook memory block — is removed; 0 memories in any repo"}
- [-] `fathom memory review` resolves a suspect memory to still-true / no-longer-true / rewrite; re-confirmation records who and against which commit, and a model-confirmed memory lands at a lower tier than a human-confirmed one without the caller specifying it {id: AM-5, disposition: refused, reason: "retired by SR-13 (surface reckoning, ADR-053): anchored memory — internal/memory, memory_search, the fathom memory verbs and the edit hook memory block — is removed; 0 memories in any repo"}
- [-] memories reach the agent through the EXISTING obligations injection path in a separate labeled block, ordered deterministically by tier then specificity then id, respecting the phase-X budget {id: AM-6, disposition: refused, reason: "retired by SR-13 (surface reckoning, ADR-053): anchored memory — internal/memory, memory_search, the fathom memory verbs and the edit hook memory block — is removed; 0 memories in any repo"}
- [-] a resolved finding (phase R) becomes a memory with a symbol anchor and verified evidence, with no new instrumentation {id: AM-7, disposition: refused, reason: "retired by SR-13 (surface reckoning, ADR-053): anchored memory — internal/memory, memory_search, the fathom memory verbs and the edit hook memory block — is removed; 0 memories in any repo"}
- [-] `memory_search` on the agent surface; every surfaced memory carries its tier and state — an unlabelled memory is a claim fathom is making on its own authority and is a test failure {id: AM-8, disposition: refused, reason: "retired by SR-13 (surface reckoning, ADR-053): anchored memory — internal/memory, memory_search, the fathom memory verbs and the edit hook memory block — is removed; 0 memories in any repo"}

---

## W2 — Anchored memory, review round (found by the build wave's adversarial reviewer, 2026-08-12)

- [-] a bare symbol name that collides across packages REFUSES to bind, naming the qualified candidates — the store's row order tracks ingestion, not code, so first-wins resolution bound the same anchor differently on identical stores and a per-file re-ingest could flip the winner, fabricating drift about a file the memory never named {id: AM-9, disposition: refused, reason: "retired by SR-13 (surface reckoning, ADR-053): anchored memory — internal/memory, memory_search, the fathom memory verbs and the edit hook memory block — is removed; 0 memories in any repo"}
- [-] drift that cannot be judged is never fresh: an anchor gone ambiguous, or one with no recorded baseline, lands SUSPECT — surfaced, labeled, tier-penalised — because fresh's contract is "every anchor's hash MATCHES its last confirmation" and neither case compared anything (law 3 on the four fields injection and search sort by) {id: AM-10, disposition: refused, reason: "retired by SR-13 (surface reckoning, ADR-053): anchored memory — internal/memory, memory_search, the fathom memory verbs and the edit hook memory block — is removed; 0 memories in any repo"}
- [-] the TierCeiling half of the confidence clamp is pinned for every tier — the reviewer deleted it and the package stayed green, in the branch whose thesis is law 14; the untested branch was exactly the one covering verified and pre-scan, the two tiers a lying caller would claim {id: AM-11, disposition: refused, reason: "retired by SR-13 (surface reckoning, ADR-053): anchored memory — internal/memory, memory_search, the fathom memory verbs and the edit hook memory block — is removed; 0 memories in any repo"}
- [x] `.fathom/memories.jsonl` is gitignored (`.fathom/` is a git-TRACKED directory, so a per-developer file with free-form claim text sat one `git add .` from being published — law 13 makes sharing a human verb, not an accident) and phase V's binding purge enumeration gains the location, since a memory may QUOTE purged content {id: AM-12, oracle: lexical, path: .gitignore, pattern: "^\.fathom/memories\.jsonl$"}

---

## X — Budgeted answers (`docs/design/budgeted-answers.md`)

- [x] every tool that can truncate emits an `elided` block naming count, total, ordering and how to get more — enforced by a registry-iteration test so a new tool cannot skip it; silent truncation is a defect {id: BA-1, symbol: elisionFor, package: internal/mcp, reachable_from: TestEveryTruncatingToolDiscloses}
- [x] `elided.count + len(results) == total`, checked, so the disclosure cannot drift from reality {id: BA-2, symbol: elisionFor, package: internal/mcp, reachable_from: TestElisionArithmetic}
- [x] ordering is deterministic and nested: the same query at a small budget returns a strict prefix of the same query at a large budget {id: BA-3, symbol: applyBudget, package: internal/mcp, reachable_from: TestBudgetPrefixProperty}
- [x] a budget too small for the minimum useful response returns that response and says the budget was exceeded — a budget is a target, never a refusal {id: BA-4, symbol: applyBudget, package: internal/mcp, reachable_from: TestBudgetNeverRefuses}
- [x] per-tool defaults come from the capability registry so a project sets them once rather than at every call site {id: BA-5, symbol: budgetDefaults, package: internal/mcp, reachable_from: TestBudgetDefaultsFromRegistry}
- [ ] response-size accounting is recorded per call and totals per tool across a fixture window {id: BA-6, symbol: recordResponseCost, package: internal/mcp, reachable_from: TestResponseCostAccounting}
  <!-- product wiring open (fathom verify --strict reads it library-only, 2026-09-28): recordCallOutcome records each call's response bytes, and the product totals them per tool through callLogFold (cmd/fathom/console_callfold.go, read by the cost panel); recordResponseCost, the anchor, has no production caller. Re-anchor on callLogFold's per-tool totals and flip once a test pins the product path (2026-09-29 re-audit). -->
- [x] a narrow surface profile carries only its own tools' schema, with the byte count asserted so an accidental widening fails the build {id: BA-7, symbol: surfaceSchemaTax, package: cmd/fathom, reachable_from: TestNarrowProfileSchemaBytes}
- [x] the Cost panel on `/observatory/impact`: tokens fathom put into agent contexts by tool, the standing schema tax and its trend, elision rate per tool, and the tokens-spent against discovery-served ratio labeled observational (law 12) {id: BA-8, symbol: callLogFold.costPanel, package: cmd/fathom, reachable_from: TestCostPanel}
- [x] budget defaults are project-settable: `budgetDefaults` reads a budgets block from the capability settings (project → user → built-in precedence, the law-13 shape BA-5 named) instead of the built-in table alone — the loader half BA-5 shipped without (2026-08-23 deviation, recorded) {id: BA-9, symbol: budgetDefaults, package: internal/mcp, reachable_from: TestBudgetDefaultsLoadFromProjectSettings}

---

## Y — Realization grade (`docs/design/realization-grade.md`)

- [x] the grade folds five dimensions — wired, fresh, queried, preventing, gating — each yes/no/not-applicable with not-applicable leaving the denominator; queried counts calls IN THE WINDOW, not calls ever {id: RG-1, symbol: realizationReportFrom, package: cmd/fathom, reachable_from: TestRealizationGradeDimensions}
- [x] the letter carries the first broken rung in ladder order, and a repository with no sessions in the window grades idle rather than F {id: RG-2, symbol: realizationReportFrom, package: cmd/fathom, reachable_from: TestIdleIsNotFailure}
- [x] grade computation is pure over a service row plus a window with an injected clock — same inputs, same letter {id: RG-3, symbol: realizationReportFrom, package: cmd/fathom, reachable_from: TestGradeIsPure}
- [ ] the fleet page ranks by unrealized value (dimensions unrealized × sessions in window), deterministic and stable under equal keys, replacing the sort by call count {id: RG-4, symbol: rankFleetByUnrealized, package: cmd/fathom, reachable_from: TestFleetPageRanksByUnrealizedValue}
- [x] the service page renders the realization ladder with evidence inline and the first broken rung carrying its remedy — the exact command today (the project overview, `/projects/{svc}`, console wave C; `fathom fleet realization` prints the same ladder), a button after phase U {id: RG-5, symbol: RealizationLadder, package: internal/console, reachable_from: TestRealizationLadder_RemedyOnFirstUnmetRung}
- [x] per-dimension 30-day series built from real call-log rows, rendering `stat.Absent` with a reason for days with no data — never an interpolated line, never a zero-height bar for an unmeasured dimension {id: RG-6, symbol: dimensionSeries, package: cmd/fathom, reachable_from: TestDimensionSeriesAbsenceIsNamed}
- [x] the fleet page's grade distribution over 30 days — the honest scoreboard for phase U: if this does not improve, the buttons did not work {id: RG-7, symbol: gradeDistributionSeries, package: cmd/fathom, reachable_from: TestGradeDistributionSeries}
- [x] the service page renders with EVERY instrument absent and fabricates no zeros (the law-3 regression) {id: RG-8, symbol: observatoryServicePage, package: cmd/fathom, reachable_from: TestServicePageAllAbsent}
- [ ] charts come from `appkit/ui/viz`; any new file rendering kit utilities is listed in `cmd/fathom/ui/input.css`'s @source globs or its classes silently never render {id: RG-9, skip_test: true, skip: "wiring row — proven by the committed input.css @source chain and the kit's own Sparkline/Gauge suites; the render tests anchor their use here"}

---

## TH — Continuity threads (vision 21; specced 2026-08-12, unbuilt)

- [x] the thread store and verbs: user-central JSONL, `fathom thread brief|note|point|list|close`, derived-state composition (plans next + claimed-unproven + git WIP), `completes_when` self-closing on artifact existence; every deposit path is mechanical — an LLM call anywhere in the loop is the kill {id: TH-1, symbol: cmdThread, package: cmd/fathom, reachable_from: TestThreadBriefComposesDerivedAndDeposited}
- [x] SessionEnd (fires on /clear, reason attached) and PreCompact deposit the mechanical SDD delta and in-flight pointers at exactly the moments context dies; the Stop hook adds session pointers incrementally {id: TH-2, symbol: cmdSessionEndHook, package: cmd/fathom, reachable_from: TestSessionEndDepositsOnClear}
- [x] SessionStart injection is source-aware and budget-bounded: a ≤600-token brief on startup/clear, NOTHING on resume/compact (their context survives) — the byte bound enforced by test {id: TH-3, symbol: renderThreadBrief, package: cmd/fathom, reachable_from: TestBriefBudgetAndSourceGating}
- [x] the §2 pre-registered bet is measurable: fresh-with-brief units labeled in the ledger, warm-up (first_edit_turn) and all-in per-turn cost (in+out+cache×0.1) compared against resumed baselines in the flywheel/brief; the registered kill (band unmet after 20 units → the injection is removed) is wired as a named check, not a memory {id: TH-4, symbol: threadBetCheck, package: cmd/fathom, reachable_from: TestThreadBetEvaluation}
- [x] `fathom thread promote` scaffolds a gate-ready plan (fathom-plan grammar) from a thread's items; `fathom thread handoff` renders the brief as a harness-neutral handoff artifact; AGENTS.md carries the Codex instruction {id: TH-5, symbol: cmdThreadPromote, package: cmd/fathom, reachable_from: TestThreadPromoteScaffoldsGateReadyPlan}
- [x] `fathom thread note` files on the current repo's own thread by default — "general" only when no service resolves — and `thread list`/`brief` scope to the current repo first, cross-repo items behind a flag: on 2026-08-23 the open set held 30 items of which ~25 belonged to other repos' handoffs, burying this repo's continuity in every listing (law 13's per-repo tagging exists; the default was the defect) {id: TH-6, symbol: cmdThreadNote, package: cmd/fathom, reachable_from: TestThreadNoteDefaultsToRepoThread}

---

## Deferred with triggers

- [-] fleet-wide one-click apply across all repositories at once {id: DF-1, disposition: deferred, reason: "per-repository apply bounds the blast radius of a bad reconciliation to one repository and keeps undo per-repository", trigger: "per-repository apply has run 50 times with zero BLOCKED-then-regretted outcomes"}
- [-] automatic or scheduled reconciliation {id: DF-2, disposition: refused, reason: "vision 20 X1 — a background process that edits repositories it was not invited into is indistinguishable from malware; the button is the consent"}
- [-] content-based purge matching (purge anything that looks like a secret) {id: DF-3, disposition: refused, reason: "a heuristic that decides what is secret will be wrong in both directions, and being wrong about deletion is unrecoverable; paths only"}
- [-] transcript mining into memories {id: DF-4, disposition: refused, reason: "produces plausible, unfalsifiable, unanchored claims at a volume that would drown the four trusted sources; anchored-memory.md §5"}
- [-] agent prose compression (the caveman ask, literal form) {id: DF-5, disposition: refused, reason: "prose style is a skill and skills already do it; fathom owns the size of what fathom emits, which is phase X"}
- [-] a fifth extension seam {id: DF-6, disposition: deferred, reason: "four typed seams exist; widening one is nearly always cheaper than adding another", trigger: "a proposal that writes down, per seam, why that seam does not fit"}

---

## Preconditions carried from earlier phases

These are owner actions, not code, and they gate the honesty of everything
above. Listed here so they are not lost.

- [ ] (op) the counterfactual pilot is run: `fathom arm off --sessions 20` after pre-registering criteria — the only instrument that can license a savings claim (law 12) {id: OP-1, skip_test: true, skip: "operator action; expert-suite-p.md carries the pre-registered protocol"}
- [ ] (op) the deployed daemon binary matches HEAD before any of the above is judged live — a committed fix reaches the daemon only after a deliberate deploy {id: OP-2, oracle: lexical, path: docs/bench/readings/ambient-fathom/op-2-deploy.md, pattern: "^OP-2 reading:"}

## Phase DO — outcomes derived, not self-reported (docs/design/continuity-threads.md §4.1, owner field report 2026-08-14)

- [x] DO-1 Derived outcome per measured unit — v1: commits landed in the unit beat-window (stated as narrow everywhere it renders; proven-row deltas follow the verify ledger) {id: DO-1, symbol: derivedCommitsInSpan, package: cmd/fathom}
- [x] DO-2 Arm criterion consumes the derived outcome instead of requiring self-ratings; copy stops asking for `fathom rate` as a precondition {id: DO-2, symbol: evaluateABCriterion, package: cmd/fathom}
- [x] DO-3 `fathom assess` — the benchmark lane: LLM judges a unit's on-disk work products (diffs, plan deltas, gate results), NEVER transcripts; explicitly invoked, per-run token cost reported. Design `docs/design/assess-lanes.md` §1: typed noul/choice questions through the decision port, a default-off fail-closed capability, a pre-flight token cap, `--dry-run` spends nothing, cost priced through the compile meter {id: DO-3, symbol: cmdAssess, package: cmd/fathom, reachable_from: TestCmdAssess_JudgesWorkProductsNeverTranscripts}
- [x] DO-4 `fathom rate` demoted to optional human override, named when present, never a verdict precondition {id: DO-4, symbol: evaluateABCriterion, package: cmd/fathom, reachable_from: TestObservatoryImpactPage}

## Phase IR — intervention outcomes (docs/design/intervention-outcomes.md; owner ask 2026-08-14)

- [x] IR-1 Firing beats carry session + file (edit-hook pre/post, all kinds; custodian attention beat) — absent values omitted, never "" {id: IR-1, symbol: firingMeta, package: cmd/fathom}
- [x] IR-2 `fathom retro` — mechanical session retro: firings joined to on-disk aftermath (committed / modified / untouched; concern remediation now-state; discovery counts), unjoinable and silent-by-design absences NAMED {id: IR-2, symbol: cmdRetro, package: cmd/fathom}
- [x] IR-3 `fathom assess --interventions` — LLM lane judging undecidable rows from on-disk artifacts only (never transcripts); per-run cost in output; capability-gated, default off. Only committed / modified-uncommitted firings are asked (one complied/departed/not_applicable/cannot_tell choice per file, guidance re-derived from disk with its provenance); untouched and unstamped firings, and firings whose file has no diff inside the window, are named, never asked (design §1) {id: IR-3, symbol: assessInterventions, package: cmd/fathom, reachable_from: TestCmdAssessInterventions_CapabilityGatedUndecidableOnly}
- [x] IR-4 Spend isolation: retro/assess metering excluded from arm and bet unit totals, pre-registered before the first assess run. Registered as `IR-4@2026-09-25` (design §3, I46): assess meters on its own ledger, its judge inherits `FATHOM_CHANNELS=off` + `FATHOM_SPEND_LABEL`, and a labelled session-ledger row is never an arm, bet or flywheel unit, counted and named {id: IR-4, symbol: spendIsolatedRow, package: cmd/fathom, reachable_from: TestAssessSpendNeverEntersArmOrBetUnits}
- [x] IR-5 Retro corpus under workspace/artifacts/retros/ feeds the research flywheel (sanitized, opt-in bundle path). `fathom research bundle --include-retros` adds a counts-only `retros` group over closed labels to the closed CUE allowlist; absent without the flag or the corpus, never zero; no file, session or verdict (design §5) {id: IR-5, symbol: aggregateRetroCorpus, package: cmd/fathom, reachable_from: TestResearchBundle_RetroCorpusIsOptInAndSanitized}
