# Progress and economics: the plan (vision 38, ADR-049)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/progress-and-economics.md` proves it.
Design: `docs/design/progress-and-economics.md` (section numbers below are its). Goal G22
(PE-M1..M4). Builds on what exists: the verify ledger (SD-1, schema 2, about 17,000
readings), the session ledger (3,359 sessions), the Stop hook's incremental transcript fold,
the console's SSE channel (ADR-045). Every view is read-only; no row here writes a plan or a
status.

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

- [x] Transcript cost fields from the incremental fold (tokens in and out, cache reads and writes, tool calls), Claude Code, library: `foldTranscriptCost`; the Stop hook already records tokens and cache fields, but not the tool-call count (PE-13w) {id: PE-13, symbol: foldTranscriptCost, package: cmd/fathom, reachable_from: TestFoldTranscriptCostIncremental}
- [ ] The Stop hook's session-ledger row carries the tool-call count from the same fold, beside the token and cache fields it already records {id: PE-13w, symbol: buildSessionLedgerEntry, package: cmd/fathom, reachable_from: TestSessionLedgerEntryCarriesToolCalls, needs: "PE-13"}
- [ ] Codex session files read by the same fold on the harness port {id: PE-14, symbol: codexSessionCost, package: internal/harness, reachable_from: TestCodexSessionCost}
- [x] The join: a session's proven outcome from verify readings whose commit it produced; unjoined share named {id: PE-15, symbol: JoinSessionProof, package: internal/console, reachable_from: TestJoinSessionProofUnjoinedNamed, needs: "PE-2,PE-13"}
- [x] A user-editable price table (`~/.fathom/prices.toml`), no network, library: `loadPriceTable`; nothing prices a session with it yet (PE-16w) {id: PE-16, symbol: loadPriceTable, package: internal/cost, reachable_from: TestPriceTableNoNetwork}
- [ ] The console prices sessions through the loaded table: cost per proven row per plan and language on `/progress` {id: PE-16w, symbol: buildProgressPage, package: cmd/fathom, reachable_from: TestProgressPagePricesSessionsThroughPriceTable, needs: "PE-16"}
- [x] G2's admissibility test, library: `admissibleComparison` withholds a compared pair unless admissible; only the model scorecard's causal gate uses it today (PE-17w) {id: PE-17, symbol: admissibleComparison, package: internal/console, reachable_from: TestOnOffWithheldUnlessAdmissible, needs: "PE-15"}
- [ ] Fathom-on versus off shown only when G2's admissibility test passes for the compared units; otherwise the panel names why it is withheld, and `fathom rate`'s on/off verdict goes through the same test {id: PE-17w, symbol: evaluateABCriterion, package: cmd/fathom, reachable_from: TestOnOffVerdictWithheldUnlessAdmissible, needs: "PE-17,PE-15"}
- [x] Tool-call mix and cache hit rate, library: `ToolMix` per session and plan; no page shows it yet (PE-18w) {id: PE-18, symbol: ToolMix, package: internal/console, reachable_from: TestToolMixAndCacheHit, needs: "PE-13"}
- [ ] The session page shows each session's tool-call mix and cache hit rate, and the plan page shows them per plan {id: PE-18w, symbol: buildSessionPage, package: cmd/fathom, reachable_from: TestSessionPageShowsToolMixAndCacheHit, needs: "PE-18"}
- [x] The live session feed, library: `renderSessionFeed` renders the read-only feed table; no route serves it yet (PE-19w) {id: PE-19, symbol: renderSessionFeed, package: cmd/fathom, reachable_from: TestConsoleSessionFeed, needs: "PE-15"}
- [ ] The live session feed is served on `/sessions`, read-only over the SSE channel {id: PE-19w, symbol: buildSessionsPage, package: cmd/fathom, reachable_from: TestSessionsPageServesLiveFeed, needs: "PE-19"}

## Refused (recorded so they are not re-proposed)

- [-] Kanban write-back, status set by a drag or click, assignment or due dates {id: PE-X1, disposition: refused, reason: "the no-tracker non-goal; status is computed by verify (ADR-049 decision 4)"}
- [-] A generic token-usage dashboard (tokens per day, quotas, spend alerts) {id: PE-X2, disposition: refused, reason: "scope non-goal; the clients own raw usage"}
- [-] A second ledger file for proof history {id: PE-X3, disposition: refused, reason: "the verify ledger gains per-row verdicts additively (schema 3)"}
- [-] Per-person figures {id: PE-X4, disposition: refused, reason: "I58"}

## Measurements (op)

- [ ] (op) PE-M1: every number on the Progress and Economics pages traced to an instrument with a truth state {id: PE-M1, oracle: lexical, path: docs/bench/readings/progress/instrument-audit.md, pattern: "^PE-M1 verdict: PASS", needs: "PE-12,PE-19"}
- [ ] (op) PE-M2: the ledger reconstructs burn-up and proof regressions for 30 consecutive days, matching `plans status` at each day's last reading {id: PE-M2, oracle: lexical, path: docs/bench/readings/progress/ledger-reconstruction.md, pattern: "^PE-M2 verdict: PASS", needs: "PE-6,PE-8"}
- [ ] (op) PE-M3: tokens per proven row joined for Claude Code and Codex sessions, the unjoined share named {id: PE-M3, oracle: lexical, path: docs/bench/readings/progress/cost-per-row.md, pattern: "^PE-M3 verdict: PASS", needs: "PE-15,PE-14"}
- [ ] (op) PE-M4: the pages measured like G13's UX-M1 {id: PE-M4, oracle: lexical, path: docs/bench/readings/progress/ux.md, pattern: "^PE-M4 verdict: PASS", needs: "PE-12"}
