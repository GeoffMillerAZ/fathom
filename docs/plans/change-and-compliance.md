# Change and compliance: the plan (vision 38 §4, ADR-051)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/change-and-compliance.md` proves it.
Design: `docs/design/change-and-compliance.md` (section numbers below are its). Goal G25
(CX-M1..M4; the catalog's SG-M6 is in `docs/plans/system-graph.md`). Promotes futures F24 and
the computable subset of F9.

## Wave 2: the change risk receipt (U7)

- [x] The receipt's factors for a diff: blast radius, links touched (spec/typed counted, lower tiers listed), certificates affected, conservation, rows proven, rules touched, zone interest, uncertified contracts (§2) {id: CX-1, symbol: BuildRiskReceipt, package: internal/review, reachable_from: TestRiskReceiptFactorsCited}
- [x] The band from a published rule table (`.fathom/risk.cue`, defaults in the binary): the strongest reason, never a summed score {id: CX-2, symbol: riskBand, package: internal/review, reachable_from: TestRiskBandStrongestReason, needs: "CX-1"}
- [x] The risk rule table schema with valid/invalid testdata {id: CX-3, oracle: lexical, path: schema/risk/schema.cue, pattern: "^#RiskTable:"}
- [x] Suggested reviewers from CODEOWNERS and frequent committers to impacted providers, with the files that put them there, never ranked (I58) {id: CX-4, symbol: suggestReviewers, package: internal/review, reachable_from: TestSuggestReviewersFromOwnership, needs: "CX-1"}
- [ ] `review_diff` gains the `receipt` section, additively {id: CX-5, symbol: reviewDiffReceipt, package: internal/mcp, reachable_from: TestReviewDiffCarriesReceipt, needs: "CX-2"}
- [ ] `fathom review --receipt --format sarif|check` through the shared emitter {id: CX-6, symbol: cmdReviewReceipt, package: cmd/fathom, reachable_from: TestCmdReviewReceiptFormats, needs: "CX-5,governance-learning.md#GL-21"}
- [x] Links below `typed` appear in the receipt as listed hints and never move the band (I57) {id: CX-7, symbol: receiptLinkTiers, package: internal/review, reachable_from: TestReceiptHeuristicLinksNeverMoveBand, needs: "CX-1"}
- [x] Fathom never posts, approves or blocks on its own authority; CI decides what a band does (ADR-040's boundary) {id: CX-8, symbol: receiptAuthority, package: internal/review, reachable_from: TestReceiptNeverPostsOrBlocks, needs: "CX-6"}

## Wave 3: the migration tracker (U9)

- [ ] A migration declaration: from and to for symbols, imports, routes, config keys and `pkg:` versions, in a plan or written by `refactor plan` (§4) {id: CX-9, symbol: ParseMigration, package: internal/review, reachable_from: TestParseMigrationForms}
- [ ] Remaining and done sites per repository in the fleet, each with its tier (typed or lexical) {id: CX-10, symbol: MigrationSites, package: internal/review, reachable_from: TestMigrationSitesPerRepo, needs: "CX-9"}
- [ ] Blockers: sites in files governed against the change, or in generated code {id: CX-11, symbol: migrationBlockers, package: internal/review, reachable_from: TestMigrationBlockers, needs: "CX-10"}
- [ ] Burn-down from proof history and completion proven by conservation (`gone:` rows proven, no unexcused loss) {id: CX-12, symbol: MigrationBurnDown, package: internal/review, reachable_from: TestMigrationCompletionByConservation, needs: "CX-10,progress-and-economics.md#PE-2"}
- [ ] `fathom refactor status` shows the fleet-wide migration view; nothing is rewritten {id: CX-13, symbol: refactorStatusMigration, package: cmd/fathom, reachable_from: TestRefactorStatusMigrationNeverRewrites, needs: "CX-12"}

## Wave 4: the evidence pack (U8) and the drift radar (U11)

- [ ] The traceability matrix: requirement, code, test, certificate, attested run, decision, each cell with its live state and proving reading (§3) {id: CX-14, symbol: BuildTraceMatrix, package: internal/review, reachable_from: TestTraceMatrixLiveStates}
- [ ] The standard mapping file schema (`.fathom/trace/<standard>.cue`), no clause text shipped (ADR-011) {id: CX-15, oracle: lexical, path: schema/trace/schema.cue, pattern: "^#StandardMapping:"}
- [ ] `fathom trace export --standard --out` writes the matrix, digests, readings, commit and a sha256 manifest, only on request {id: CX-16, symbol: cmdTraceExport, package: cmd/fathom, reachable_from: TestCmdTraceExportManifest, needs: "CX-14,CX-15"}
- [ ] `fathom trace verify <dir>` recomputes at the pack's commit and must be byte-identical, differences named by file and cell {id: CX-17, symbol: cmdTraceVerify, package: cmd/fathom, reachable_from: TestCmdTraceVerifyByteIdentical, needs: "CX-16"}
- [ ] `accept:` criteria and foreign spec requirements appear as matrix requirements {id: CX-18, symbol: traceRequirements, package: internal/review, reachable_from: TestTraceRequirementsIncludeAccept, needs: "CX-14,spec-interop.md#SI-8"}
- [ ] ADR dependency statements proposed as archconformance rules with citations; a human accepts each (F24 promoted) (§5) {id: CX-19, symbol: ProposeADRRules, package: internal/review, reachable_from: TestProposeADRRulesCited}
- [ ] Violation counts per rule recorded at each ingest; the trend and the commits that added violations {id: CX-20, symbol: recordViolationTrend, package: internal/fusion, reachable_from: TestViolationTrendAtIngest}
- [ ] Computable revisit and promote triggers (bet-block shape: an instrument and a threshold) evaluated by `fathom intent audit`, never by the daemon on its own; prose triggers listed as unevaluated (F9's subset) {id: CX-21, symbol: evalComputableTriggers, package: internal/intent, reachable_from: TestComputableTriggersEvaluatedOnDemand}
- [ ] The radar on the console: decisions due for review and architecture eroding here {id: CX-22, symbol: renderDriftRadar, package: cmd/fathom, reachable_from: TestConsoleDriftRadar, needs: "CX-20,CX-21"}
- [ ] The decision port proposes a rule from an ADR's prose only with `[decide]` configured; kept only if SI-style acceptance reads 8 of 10 {id: CX-23, symbol: proposeRuleFromADR, package: internal/review, reachable_from: TestProposeRuleFromADROnlyWithProvider, needs: "CX-19"}

## Refused (recorded so they are not re-proposed)

- [-] A summed risk score or a model's opinion of risk {id: CX-X1, disposition: refused, reason: "the band is the strongest cited reason (ADR-051 decision 4)"}
- [-] Posting to a forge, approving or blocking on fathom's authority {id: CX-X2, disposition: refused, reason: "ADR-040's authority boundary; CI decides"}
- [-] Shipping a standard's clause text {id: CX-X3, disposition: refused, reason: "ADR-011; the mapping is configuration"}
- [-] Rewriting sites to complete a migration {id: CX-X4, disposition: refused, reason: "the Moderne non-goal: track and prove, never transform"}
- [-] The daemon evaluating triggers on its own schedule {id: CX-X5, disposition: refused, reason: "the dispatch non-goal"}

## Measurements (op)

- [ ] (op) CX-M1: the receipt on every PR in at least 2 repositories for 30 days, SARIF or check output consumed by CI {id: CX-M1, oracle: lexical, path: docs/bench/readings/change/receipts.md, pattern: "^CX-M1 verdict: PASS", needs: "CX-6"}
- [ ] (op) CX-M2: an evidence pack exported and re-verified byte-identically at its commit {id: CX-M2, oracle: lexical, path: docs/bench/readings/change/evidence-pack.md, pattern: "^CX-M2 verdict: PASS", needs: "CX-17"}
- [ ] (op) CX-M3: one real migration tracked from declaration to proven completion {id: CX-M3, oracle: lexical, path: docs/bench/readings/change/migration.md, pattern: "^CX-M3 verdict: PASS", needs: "CX-13"}
- [ ] (op) CX-M4: the radar live with at least 3 compiled ADR rules, a violation trend, and the computable triggers evaluated {id: CX-M4, oracle: lexical, path: docs/bench/readings/change/radar.md, pattern: "^CX-M4 verdict: PASS", needs: "CX-22"}
- [ ] (op) CX-M5: the band against outcomes on 50 historical PRs: high-band PRs carry at least 2x the post-merge fixes of low-band PRs, or the table is revised {id: CX-M5, oracle: lexical, path: docs/bench/readings/change/band-calibration.md, pattern: "^CX-M5 verdict: (PASS|REVISED)", needs: "CX-2"}
