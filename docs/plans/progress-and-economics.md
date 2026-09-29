# Progress and economics: the plan (vision 38, ADR-049)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/progress-and-economics.md` proves it.
Design: `docs/design/progress-and-economics.md` (section numbers below are its). Goal G22
(PE-M1..M4). Builds on what exists: the verify ledger (SD-1, schema 2, about 17,000
readings), the session ledger (3,359 sessions), the Stop hook's incremental transcript fold,
the console's SSE channel (ADR-045). Every view is read-only; no row here writes a plan or a
status. Weights and cost per weight point are `docs/plans/work-signals.md` (WS-8 to WS-11); PE-22 is the page
that shows them. Waves 4 and 5 were added 2026-09-29 (ADR-056 decision 10, observation O121).

## Wave 1: cost in every answer

- [x] Every receipt carries an estimated token count beside its bytes (`cost.Estimate`, named as an estimate) {id: PE-20, symbol: receiptTokens, package: internal/mcp, reachable_from: TestReceiptCarriesTokenEstimate}
- [x] The console shows the cost of each answer on the call-log pages {id: PE-21, symbol: renderAnswerCost, package: cmd/fathom, reachable_from: TestConsoleAnswerCost, needs: "PE-20"}

## Wave 2: proof history and the progress views

- [x] Verify ledger schema 3: an optional `rows` array (`id`, `state`, `criterion`, `weight`) written only when the plan's row digest changed; v1 and v2 rows still parse (§3) {id: PE-1, symbol: appendVerifyLedgerRows, package: cmd/fathom, reachable_from: TestVerifyLedgerSchema3RowsOnDigestChange}
- [x] Proof history as a projection: per row, its state over time from the ledger, bounded tail reads, cached in a `memo` group {id: PE-2, symbol: ProofHistory, package: internal/console, reachable_from: TestProofHistoryProjection, needs: "PE-1"}
- [x] `plans status --weighted` rolls up by `weight:` (fleet, project, plan, section) {id: PE-3, symbol: weightedRollup, package: cmd/fathom, reachable_from: TestPlansStatusWeighted, needs: "spec-interop.md#SI-7"}
- [x] The weighted table view (§4) {id: PE-4, symbol: renderWeightedTable, package: cmd/fathom, reachable_from: TestConsoleWeightedTable, needs: "PE-3"}
- [x] The derived kanban: Open, Claimed-unproven, Proven, Refused or deferred; read-only, cards link to the failing criterion {id: PE-5, symbol: renderDerivedKanban, package: cmd/fathom, reachable_from: TestConsoleDerivedKanbanReadOnly, needs: "PE-2"}
- [x] Burn-up with a projected finish: a linear fit over 14 days with its interval, hidden below 7 days of data {id: PE-6, symbol: BurnUp, package: internal/console, reachable_from: TestBurnUpProjectionNeedsSevenDays, needs: "PE-2"}
- [x] The frontier view from `plans next --eligible-only` {id: PE-7, symbol: renderFrontier, package: cmd/fathom, reachable_from: TestConsoleFrontier}
- [x] Proof regressions: rows once proven that now read otherwise, with the first changed reading and its commit {id: PE-8, symbol: ProofRegressions, package: internal/console, reachable_from: TestProofRegressionsWithCommit, needs: "PE-2"}
- [x] Time-to-proof per row (first appearance to first proven reading) {id: PE-9, symbol: TimeToProof, package: internal/console, reachable_from: TestTimeToProof, needs: "PE-2"}
- [x] Claim honesty per agent identity (model, harness), never per person (I58) {id: PE-10, symbol: ClaimHonesty, package: internal/console, reachable_from: TestClaimHonestyByAgentIdentity, needs: "PE-2"}
- [x] The heatmap (proven weight by plan and week), streaks and "proved today" as counts {id: PE-11, symbol: renderProofHeatmap, package: cmd/fathom, reachable_from: TestConsoleProofHeatmap, needs: "PE-2"}
- [x] Goal gauges per scope bet and filters on every view (service, plan, wave, goal, agent identity, date range) {id: PE-12, symbol: goalGauges, package: internal/console, reachable_from: TestGoalGaugesFromBets, needs: "PE-2"}

## Wave 3: outcome economics

- [x] Transcript cost fields from the incremental fold (tokens in and out, cache reads and writes, tool calls), Claude Code, library: `foldTranscriptCost`; the Stop hook already records tokens and cache fields, but not the tool-call count (PE-13w) {id: PE-13, symbol: foldTranscriptCost, package: cmd/fathom, reachable_from: TestFoldTranscriptCostIncremental, library: true, reason: "the library half of a split; PE-13w (open) is the row that wires it into the product"}
- [ ] The Stop hook's session-ledger row carries the tool-call count from the same fold, beside the token and cache fields it already records {id: PE-13w, symbol: buildSessionLedgerEntry, package: cmd/fathom, reachable_from: TestSessionLedgerEntryCarriesToolCalls, needs: "PE-13"}
- [ ] Codex session files read by the same fold on the harness port {id: PE-14, symbol: codexSessionCost, package: internal/harness, reachable_from: TestCodexSessionCost}
  <!-- product wiring open (fathom verify --strict reads it library-only, 2026-09-29): codexSessionCost is reached only through harness.CodexSessionCost, which no verb, hook or console page calls yet; `fathom harness capabilities` describes the rollout but does not fold it. Re-opened, not waived, for the reason CX-23 gives. -->
- [x] The join: a session's proven outcome from verify readings whose commit it produced; unjoined share named {id: PE-15, symbol: JoinSessionProof, package: internal/console, reachable_from: TestJoinSessionProofUnjoinedNamed, needs: "PE-2,PE-13"}
- [x] A user-editable price table (`~/.fathom/prices.toml`), no network, library: `loadPriceTable`; nothing prices a session with it yet (PE-16w) {id: PE-16, symbol: loadPriceTable, package: internal/cost, reachable_from: TestPriceTableNoNetwork, library: true, reason: "the library half of a split; PE-16w (open) is the row that wires it into the product"}
- [ ] The console prices sessions through the loaded table: cost per proven row per plan and language on `/progress` {id: PE-16w, symbol: buildProgressPage, package: cmd/fathom, reachable_from: TestProgressPagePricesSessionsThroughPriceTable, needs: "PE-16"}
- [x] G2's admissibility test, library: `admissibleComparison` withholds a compared pair unless admissible; only the model scorecard's causal gate uses it today (PE-17w) {id: PE-17, symbol: admissibleComparison, package: internal/console, reachable_from: TestOnOffWithheldUnlessAdmissible, needs: "PE-15"}
- [ ] Fathom-on versus off shown only when G2's admissibility test passes for the compared units; otherwise the panel names why it is withheld, and `fathom rate`'s on/off verdict goes through the same test {id: PE-17w, symbol: evaluateABCriterion, package: cmd/fathom, reachable_from: TestOnOffVerdictWithheldUnlessAdmissible, needs: "PE-17,PE-15"}
- [x] Tool-call mix and cache hit rate, library: `ToolMix` per session and plan; no page shows it yet (PE-18w) {id: PE-18, symbol: ToolMix, package: internal/console, reachable_from: TestToolMixAndCacheHit, needs: "PE-13", library: true, reason: "the library half of a split; PE-18w (open) is the row that wires it into the product"}
- [ ] The session page shows each session's tool-call mix and cache hit rate, and the plan page shows them per plan {id: PE-18w, symbol: buildSessionPage, package: cmd/fathom, reachable_from: TestSessionPageShowsToolMixAndCacheHit, needs: "PE-18"}
- [x] The live session feed, library: `renderSessionFeed` renders the read-only feed table; no route serves it yet (PE-19w) {id: PE-19, symbol: renderSessionFeed, package: cmd/fathom, reachable_from: TestConsoleSessionFeed, needs: "PE-15", library: true, reason: "the library half of a split; PE-19w (open) is the row that wires it into the product"}
- [ ] The live session feed is served on `/sessions`, read-only over the SSE channel {id: PE-19w, symbol: buildSessionsPage, package: cmd/fathom, reachable_from: TestSessionsPageServesLiveFeed, needs: "PE-19"}

## Wave 4: cost per weight point on the pages (design section 10, ADR-056 decision 10)

The estimator, the one-command acceptance and the computation are `docs/plans/work-signals.md` WS-8 to WS-11
(not repeated here); this row is the page that shows them.

- [ ] The Progress and Economics pages show cost per proven WEIGHT POINT beside cost per proven row, per model, harness and zone, keyed by agent identity (I58), with an estimated weight marked as an estimate and an authored weight as authored {id: PE-22, symbol: renderCostPerWeightPoint, package: cmd/fathom, reachable_from: TestConsoleCostPerWeightPointMarksEstimates, needs: "work-signals.md#WS-10"}

## Wave 5: the ledgers stay honest (design section 11, observation O121)

Scratch-clone runs appended rows keyed by scratch paths to the user-level ledgers (13,514 of 19,639 verify rows
on 2026-09-28), which distorts proof history and economics. Readers filter to registered roots, scratch and test
runs get a documented way to stay out, and an owner-run `fathom gc` class archives what is already there.

- [x] The registered service roots are one function: the roots the fleet index registered, canonical (symlinks resolved, no trailing slash), asked by every ledger reader {id: PE-23, symbol: RegisteredRoots, package: internal/ledgerroots, reachable_from: TestRegisteredRootsCanonicalAndSymlinkResolved}
- [x] Every reader of the verify ledger (the gate figure and catches, proof history and burn-up, the session join, plans trend, the research report, assess gate results) reads only rows whose `repo_root` is a registered service root, ignores unregistered and scratch roots and names the ignored count; the file itself is never rewritten by a reader {id: PE-24, symbol: verifyLedgerRegisteredRows, package: cmd/fathom, reachable_from: TestVerifyLedgerReadersIgnoreUnregisteredRoots, needs: "PE-23", weight: 3}
- [x] Every reader of the intent ledger (the audit's baseline and debt, clocks, learning announcements) does the same {id: PE-25, symbol: registeredLedgerRows, package: internal/intent, reachable_from: TestIntentLedgerReadersIgnoreUnregisteredRoots, needs: "PE-23"}
- [x] `FATHOM_HOME` relocates the whole `~/.fathom` (ledgers, logs, stamps, caches, daemon records) and `--ledger <path>` on `fathom verify` and `fathom intent audit` names one ledger file; the per-file variables (`FATHOM_VERIFY_LEDGER`, `FATHOM_INTENT_LEDGER`, `FATHOM_SESSION_LEDGER`, `FATHOM_HOOKS_LOG`) keep working and win over `FATHOM_HOME` {id: PE-26, symbol: FathomHome, package: internal/fathomhome, reachable_from: TestFathomHomeRelocatesLedgersAndPerFileVarsWin, weight: 3}
- [x] The overrides are documented: `docs/usage.md` names `FATHOM_HOME`, `--ledger` and the per-file variables, and says a scratch clone or a test run sets one {id: PE-27, oracle: lexical, path: docs/usage.md, pattern: "FATHOM_HOME", needs: "PE-26"}
- [x] Test and scratch runs use it: the gate's `Taskfile.yml` and the scratch-clone workflow set `FATHOM_HOME` to a temporary directory, and a scratch `fathom verify` or `fathom intent audit` under it appends nothing to the real `~/.fathom` ledgers {id: PE-28, symbol: appendVerifyLedgerFrom, package: cmd/fathom, reachable_from: TestScratchRunUnderFathomHomeLeavesRealLedgersUntouched, needs: "PE-26"}
- [x] `fathom gc` gains the class `ledger-unregistered`: the dry run counts rows and bytes per unregistered root in the verify and intent ledgers; `--apply`, run by the owner, archives them under ~/.fathom/archive/ in the 2026-09-29 layout (plain JSONL, SHA256SUMS, a README row), moving rows and never deleting them, and neither the daemon's daily pass nor doctor applies the class {id: PE-29, symbol: planLedgerUnregistered, package: cmd/fathom, reachable_from: TestGCLedgerUnregisteredArchivesNeverDeletes, needs: "PE-23", weight: 3}

## Refused (recorded so they are not re-proposed)

- [-] Kanban write-back, status set by a drag or click, assignment or due dates {id: PE-X1, disposition: refused, reason: "the no-tracker non-goal; status is computed by verify (ADR-049 decision 4)"}
- [-] A generic token-usage dashboard (tokens per day, quotas, spend alerts) {id: PE-X2, disposition: refused, reason: "scope non-goal; the clients own raw usage"}
- [-] A second ledger file for proof history {id: PE-X3, disposition: refused, reason: "the verify ledger gains per-row verdicts additively (schema 3)"}
- [-] Per-person figures {id: PE-X4, disposition: refused, reason: "I58"}
- [-] Deleting or rewriting ledger rows to clean up unregistered roots, from a reader, the daemon or doctor {id: PE-X5, disposition: refused, reason: "evidence is archived, never deleted (ADR-047 decision 13); a root unregistered today may be registered tomorrow; the filter is on read and the archive is owner-run and reversible"}

## Measurements (op)

- [ ] (op) PE-M1: every number on the Progress and Economics pages traced to an instrument with a truth state {id: PE-M1, oracle: lexical, path: docs/bench/readings/progress/instrument-audit.md, pattern: "^PE-M1 verdict: PASS", needs: "PE-12,PE-19"}
- [ ] (op) PE-M2: the ledger reconstructs burn-up and proof regressions for 30 consecutive days, matching `plans status` at each day's last reading {id: PE-M2, oracle: lexical, path: docs/bench/readings/progress/ledger-reconstruction.md, pattern: "^PE-M2 verdict: PASS", needs: "PE-6,PE-8"}
- [ ] (op) PE-M3: tokens per proven row joined for Claude Code and Codex sessions, the unjoined share named {id: PE-M3, oracle: lexical, path: docs/bench/readings/progress/cost-per-row.md, pattern: "^PE-M3 verdict: PASS", needs: "PE-15,PE-14"}
- [ ] (op) PE-M4: the pages measured like G13's UX-M1 {id: PE-M4, oracle: lexical, path: docs/bench/readings/progress/ux.md, pattern: "^PE-M4 verdict: PASS", needs: "PE-12"}
- [x] (op) PE-M5: the ledger-pollution reading on this machine: verify-ledger and intent-ledger rows under unregistered roots counted (13,514 of 19,639 and 91 of 180 on 2026-09-28), and burn-up, cost per proven row and claim honesty recomputed with and without them, the difference published {id: PE-M5, oracle: lexical, path: docs/bench/readings/progress/ledger-pollution.md, pattern: "^PE-M5 reading:", needs: "PE-24,PE-25"}
