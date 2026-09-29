# Work intelligence: the plan (vision 38 §4, ADR-051)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/work-intelligence.md` proves it.
Design: `docs/design/work-intelligence.md` (section numbers below are its). Goal G24
(WI-M1, WI-M2, WI-M4). Every product is a projection over existing ledgers and stored facts
(the join rule, ADR-051 decision 1); proof history and the session join come from
`docs/plans/progress-and-economics.md`. Invariant I58 rises from `stated` as WI-14 lands.

Stall detection, estimated weights, cost per proven weight point and the preflight baseline (decisions E3 and E4 of
the 2026-09-29 judgment research) are `docs/plans/work-signals.md`; they read WI-4, WI-5, WI-9 and WI-15 to WI-19
and are not repeated here. WI-16 to WI-19 stay open and are what WS-8 and WS-12 wait for.

## Wave 2: proof regressions generalized (U12)

- [ ] Certificate regressions: live to flaky, broken or unproven, with the reading and commit {id: WI-20, symbol: certificateRegressions, package: internal/console, reachable_from: TestCertificateRegressionsWithCommit}
  <!-- product wiring open (fathom verify --strict reads it library-only, 2026-09-28): certificateRegressions has no production caller: no page renders the merged regressions list. -->
- [ ] Conservation and invariant-enforcement regressions: a plane once `conserved`, an enforcing test that stopped reaching {id: WI-21, symbol: conservationRegressions, package: internal/console, reachable_from: TestConservationAndInvariantRegressions}
  <!-- product wiring open (fathom verify --strict reads it library-only, 2026-09-28): conservationRegressions has no production caller: no page renders the merged regressions list. -->
- [ ] One regressions list, newest first, rows plus certificates plus conservation plus invariants, each linking the breaking diff {id: WI-22, symbol: AllProofRegressions, package: internal/console, reachable_from: TestAllProofRegressionsMerged, needs: "WI-20,WI-21,progress-and-economics.md#PE-8"}
  <!-- product wiring open (fathom verify --strict reads it library-only, 2026-09-28): AllProofRegressions has no production caller: no page renders the merged regressions list. -->

## Wave 3: zones, U1, U2, U3, U5, U13

- [x] Zones: package or deepest symbol directory by default, `[zones]` globs override; a change splits evenly across its zones (§2) {id: WI-1, symbol: ZoneOf, package: internal/console, reachable_from: TestZoneOfDefaultsAndOverrides}
- [x] U1 per zone; the CI-minutes join from a CI summary file is library only (`ParseCISummary` is never read by the page) {id: WI-2, symbol: costPerZone, package: internal/console, reachable_from: TestCostPerZoneAndCIMinutes, needs: "WI-1,progress-and-economics.md#PE-15"}
- [x] CI minutes from a CI summary file the user names, bound to a commit, are read by `/work` {id: WI-2w, symbol: computeWork, package: cmd/fathom, reachable_from: TestComputeWorkReadsNamedCISummary, needs: "WI-2"}
- [x] U1 split between first-attempt proofs and proofs after claimed-unproven retries {id: WI-3, symbol: firstAttemptSplit, package: internal/console, reachable_from: TestFirstAttemptSplit, needs: "WI-2"}
- [x] Effort per proven change: tokens, turns and wall time from the session ledger; rework, reverted edits and claimed-unproven retries are library only (WI-4w) {id: WI-4, symbol: EffortPerProvenChange, package: internal/console, reachable_from: TestEffortPerProvenChange, needs: "progress-and-economics.md#PE-15"}
- [x] `/work` reads rework within 7 days, reverted edits and claimed-unproven retries per proven change {id: WI-4w, symbol: workCommitFacts, package: cmd/fathom, reachable_from: TestWorkReadsReworkAndReverts, needs: "WI-4"}
- [x] U2 the interest rate per zone: median effort over the fleet median, with n, IQR and model mix {id: WI-5, symbol: InterestRate, package: internal/console, reachable_from: TestInterestRateByZone, needs: "WI-1,WI-4"}
- [x] Friction signals per file, symbol and zone (§3 U3): blocked edits from the rule-outcome log are live; rediscovery, claimed-unproven, reverts and search loops are library only (WI-6w) {id: WI-6, symbol: FrictionSignals, package: internal/console, reachable_from: TestFrictionSignals, needs: "governance-learning.md#GL-10"}
- [x] `/work` reads rediscovery, claimed-unproven, revert and search-loop signals into the friction map {id: WI-6w, symbol: computeWork, package: cmd/fathom, reachable_from: TestComputeWorkReadsFrictionSignals, needs: "WI-6"}
- [x] Friction remedies computed from the pattern table, shown as proposals with evidence, never applied; the page passes no remedy inputs yet, so only the rule-outcome patterns can fire (WI-7w) {id: WI-7, symbol: frictionRemedies, package: internal/console, reachable_from: TestFrictionRemediesAreProposals, needs: "WI-6"}
- [x] `/work` passes real remedy inputs, so every pattern in the table can fire {id: WI-7w, symbol: computeWork, package: cmd/fathom, reachable_from: TestComputeWorkPassesRemedyInputs, needs: "WI-7,WI-6w"}
- [x] The friction map on the console {id: WI-8, symbol: renderFrictionMap, package: cmd/fathom, reachable_from: TestConsoleFrictionMap, needs: "WI-7"}
- [x] U5 the scorecard per (model, harness, effort): proven rows, honesty, cost per proven row, rework, n; cells below n = 10 greyed; confounders shown. The session ledger records neither model nor harness, so every row lands in one "unknown" cell (WI-9w) {id: WI-9, symbol: ModelScorecard, package: internal/console, reachable_from: TestModelScorecardDescriptive, needs: "WI-4"}
- [x] The session ledger records harness and model per session, so the scorecard splits into real cells {id: WI-9w, symbol: sessionEffortsOf, package: cmd/fathom, reachable_from: TestScorecardSplitsByRecordedModelAndHarness, needs: "WI-9"}
- [x] Causal sentences on the scorecard only when G2's admissibility test passes; otherwise the page says descriptive {id: WI-10, symbol: scorecardCausalGate, package: internal/console, reachable_from: TestScorecardCausalOnlyWhenAdmissible, needs: "WI-9"}
- [x] The scorecard, interest-rate and cost-per-zone pages on the console {id: WI-11, symbol: renderWorkIntelligence, package: cmd/fathom, reachable_from: TestConsoleWorkIntelligencePages, needs: "WI-5,WI-9"}
- [x] U13 hotspots: size (L1 range, named proxy), churn, centrality, uncovered share, distinct-author count as a bus-factor flag, over T1's stored factors; no product caller supplies coverage yet, so uncovered share reads "no coverage supplied" until TQ-14w wires coverage in {id: WI-12, symbol: Hotspots, package: internal/console, reachable_from: TestHotspotsOverStoredFactors, needs: "test-intelligence.md#TQ-4"}
- [x] The hotspots view {id: WI-13, symbol: renderHotspots, package: cmd/fathom, reachable_from: TestConsoleHotspots, needs: "WI-12"}
- [x] I58: every work-intelligence and progress view keys rows by agent identity only; a human name or email never appears as a row key {id: WI-14, symbol: agentIdentityKey, package: internal/console, reachable_from: TestWorkViewsKeyByAgentIdentityOnly}
- [ ] Nearest-neighbour index over the last 5,000 proven rows' feature vectors, rebuilt in the background {id: WI-15, symbol: rowFeatureIndex, package: internal/console, reachable_from: TestRowFeatureIndexBounded, needs: "WI-5"}
  <!-- library landed 3832dbaa; WI-16's preflight (cmd/fathom/planspreflight.go) now calls RowFeatureIndexFor, so verify --strict reads rowFeatureIndex wired (2026-09-29), but the index is built on read and memoized per watermark: nothing rebuilds it in the background (ADR-049 decision 2 holds background work until RT-M1), so the row stays open on that clause. -->

## Wave 4: task preflight (U6, the Jev site)

- [x] The difficulty profile of a row: blast radius, governed rules and levels, certificates to re-prove, zone interest, k = 5 similar rows' effort {id: WI-16, symbol: DifficultyProfile, package: internal/console, reachable_from: TestDifficultyProfileDeterministic, needs: "WI-15"}
- [x] The deterministic tier recommendation from a published rule table; advisory, printed by `plans context --row` and on the console; never dispatches {id: WI-17, symbol: recommendTier, package: internal/console, reachable_from: TestRecommendTierAdvisoryOnly, needs: "WI-16"}
- [x] The decision port's Score over the same features, shown with its calibration, only with `[decide]` configured (I42); it may refine an estimated row weight (`work-signals.md#WS-11`) only after it beats a fitted baseline (`work-signals.md#WS-M4`) {id: WI-18, symbol: preflightScore, package: internal/console, reachable_from: TestPreflightScoreOnlyWithProvider, needs: "WI-16"}
- [x] Outcomes recorded (model used, effort, proven or not) to calibrate the preflight {id: WI-19, symbol: recordPreflightOutcome, package: internal/console, reachable_from: TestRecordPreflightOutcome, needs: "WI-17"}

## Refused (recorded so they are not re-proposed)

- [-] A productivity score, leaderboard, or any view keyed by a person {id: WI-X1, disposition: refused, reason: "I58; surveillance, and it poisons the honesty signals"}
- [-] Predictive estimates without a calibrated reading {id: WI-X2, disposition: refused, reason: "ADR-051 decision 8"}
- [-] Preflight that starts, assigns or routes work {id: WI-X3, disposition: refused, reason: "the dispatch non-goal; the harness decides"}
- [-] Auto-applied friction remedies {id: WI-X4, disposition: refused, reason: "remedies are proposals with receipts (ADR-051 decision 8)"}

## Measurements (op)

- [ ] (op) WI-M1: U1 to U6 on the console, every number traced to an instrument with a truth state {id: WI-M1, oracle: lexical, path: docs/bench/readings/work/instrument-audit.md, pattern: "^WI-M1 verdict: PASS", needs: "WI-11,WI-8,WI-17"}
- [ ] (op) WI-M2: U2 on at least 3 repositories with at least 30 proven rows each; the top zone at least 1.2x the median, or the non-separation recorded {id: WI-M2, oracle: lexical, path: docs/bench/readings/work/interest-rate.md, pattern: "^WI-M2 verdict: (PASS|NO-SEPARATION)", needs: "WI-5"}
- [ ] (op) WI-M3: the friction map's top 10 hotspots on this repository with remedies, at least 7 judged useful {id: WI-M3, oracle: lexical, path: docs/bench/readings/work/friction.md, pattern: "^WI-M3 verdict: PASS", needs: "WI-7"}
- [ ] (op) WI-M4: U6's A/B at n at least 3 per arm over at least 30 rows, Score against the rule table; keep or drop {id: WI-M4, oracle: lexical, path: docs/bench/readings/work/preflight-ab.md, pattern: "^WI-M4 verdict: (KEEP|DROP)", needs: "WI-18,WI-19"}
- [ ] (op) WI-M5: U12 over 30 days matches every real state change and misses none of 10 seeded regressions {id: WI-M5, oracle: lexical, path: docs/bench/readings/work/regressions.md, pattern: "^WI-M5 verdict: PASS", needs: "WI-22"}
