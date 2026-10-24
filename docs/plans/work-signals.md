# Work signals: the plan (vision 40, ADR-056)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/work-signals.md` proves it. Design:
`docs/design/work-signals.md` (section numbers below are its). Goal G32.

Stall detection ("spinning wheels"), estimated weights and cost per proven weight point, and the U6
preflight model site against a fitted baseline. Detection and estimation are deterministic and need no
provider; the nudge and any model refinement are experiments with kill criteria. Beside
`docs/plans/work-intelligence.md` (WI-16 to WI-19 are still open, so E4 waits for them) and
`docs/plans/progress-and-economics.md` (PE-15, PE-16).

This plan is where the owner's stall-detection note (decision E3) and the preflight baseline (E4) land;
`work-intelligence.md` and `progress-and-economics.md` carry pointers and the page row PE-22, never copies. Every
ledger read here goes through the registered-roots filter (PE-24): 13,514 of the 19,639 verify-ledger rows on
2026-09-28 sat under scratch paths, so a fold or a fit over the raw file would learn from scratch runs.

## Wave 1: detection and reports (section 1)

- [-] The deterministic stall detectors: the same test failing N times, an identical tool call repeated, edit-then-revert cycles, verify failing repeatedly on one row, a grep loop right after a fathom answer, tokens burned since the last proof progress above a threshold, frequent compactions {id: WS-1, symbol: DetectStalls, package: internal/stall, reachable_from: TestDetectStallsEachPattern, needs: "test-run-hook.md#TR-7", weight: 3, disposition: deferred, reason: "EW-D3 (efficiency-wave.md): the detectors are a daemon fold, a background job ADR-049 decision 2 holds back", trigger: "responsible-tenant.md RT-M1 reads PASS"}
- [-] Detection runs as an asynchronous daemon fold over the hook and call logs, never inside a hook {id: WS-2, symbol: foldStallEpisodes, package: internal/stall, reachable_from: TestStallFoldRunsOutsideHooks, needs: "WS-1", disposition: deferred, reason: "EW-D3 (efficiency-wave.md): the detectors are a daemon fold, a background job ADR-049 decision 2 holds back", trigger: "responsible-tenant.md RT-M1 reads PASS"}
- [-] Before any line is injected, candidate episodes are checked against abandoned sessions and later green runs, so the TDD red phase is not flagged {id: WS-3, symbol: checkEpisodeAgainstOutcomes, package: internal/stall, reachable_from: TestTDDRedPhaseIsNotAStall, needs: "WS-2", disposition: deferred, reason: "EW-D3 (efficiency-wave.md): the detectors are a daemon fold, a background job ADR-049 decision 2 holds back", trigger: "responsible-tenant.md RT-M1 reads PASS"}
- [ ] (op) WS-M1: candidate episodes over 14 days are checked against abandoned sessions and later green runs, and the false-stall rate on TDD red phases is named {id: WS-M1, oracle: lexical, path: docs/bench/readings/work-signals/stall-episodes.md, pattern: "^WS-M1 reading:", needs: "WS-3"}
- [-] Reports: stall episodes and stall cost per session, model, harness and zone, keyed by agent identity and never a person (I58) {id: WS-4, symbol: StallReport, package: internal/console, reachable_from: TestStallReportKeysByAgentIdentityOnly, needs: "WS-2,work-intelligence.md#WI-14", disposition: deferred, reason: "EW-D3 (efficiency-wave.md): the detectors are a daemon fold, a background job ADR-049 decision 2 holds back", trigger: "responsible-tenant.md RT-M1 reads PASS"}
- [-] The cost-effectiveness view names when a cheaper model cost more per proven weight point than a stronger one (U5's model scorecard) {id: WS-5, symbol: modelCostPerWeightPoint, package: internal/console, reachable_from: TestModelScorecardShowsCheaperCostingMore, needs: "WS-4,WS-10", disposition: deferred, reason: "EW-D3 (efficiency-wave.md): the detectors are a daemon fold, a background job ADR-049 decision 2 holds back", trigger: "responsible-tenant.md RT-M1 reads PASS"}

## Wave 2: the nudge, an experiment (section 1)

- [-] A stall signal file an orchestrator may read; fathom never switches models, respawns or dispatches {id: WS-6, symbol: writeStallSignal, package: internal/stall, reachable_from: TestStallSignalNeverDispatches, needs: "WS-3", disposition: deferred, reason: "EW-D3 (efficiency-wave.md): the detectors are a daemon fold, a background job ADR-049 decision 2 holds back", trigger: "responsible-tenant.md RT-M1 reads PASS"}
- [ ] The nudge: at most one per episode, opt-in; the next hook's additionalContext names the pattern and offers proof tools (`localize`, `symbol_context`, `blast_radius`, the failing test's reach) {id: WS-7, symbol: stallNudgeLine, package: cmd/fathom, reachable_from: TestStallNudgeOncePerEpisodeOptIn, needs: "WS-3,WS-M1"}
- [ ] (op) WS-M2: nudges A/B by session hash: stall episodes shorten and cost per proven weight point drops, or the nudge is dropped and detection and reports stay {id: WS-M2, oracle: lexical, path: docs/bench/readings/work-signals/nudge-ab.md, pattern: "^WS-M2 verdict: (KEEP|DROP)", needs: "WS-7", weight: 5}

## Wave 3: estimated weights and cost per weight point (section 2)

- [x] `EstimateRowWeight`: a deterministic estimate from the anchor type, planned files and the blast radius of the anchor's package, dependent rows, and the effort of similar proven rows from the U6 feature index; shown as "estimated weight" beside any authored weight and never overwriting it {id: WS-8, symbol: EstimateRowWeight, package: internal/console, reachable_from: TestEstimateRowWeightNeverOverwritesAuthored, needs: "work-intelligence.md#WI-16", weight: 3}
- [x] One command accepts an estimate into the plan as the author's own weight {id: WS-9, symbol: cmdPlansWeightAccept, package: cmd/fathom, reachable_from: TestCmdPlansWeightAcceptWritesAuthoredWeight, needs: "WS-8"}
- [ ] Cost per proven WEIGHT POINT, not only per row, on the Progress and Economics pages, per model, harness and zone {id: WS-10, symbol: CostPerWeightPoint, package: internal/console, reachable_from: TestCostPerWeightPointPerModelAndZone, needs: "progress-and-economics.md#PE-15,progress-and-economics.md#PE-16,progress-and-economics.md#PE-24,WS-9"}
- [ ] With provider none the estimate is the deterministic one; a model may refine it only after passing calibration {id: WS-11, symbol: EstimateRowWeight, package: internal/console, reachable_from: TestEstimatedWeightProviderNoneIsDeterministic, needs: "WS-8"}
- [ ] (op) WS-M3: the estimated-weight error against actual effort (tokens, turns, wall time per proven row) is published {id: WS-M3, oracle: lexical, path: docs/bench/readings/work-signals/weight-error.md, pattern: "^WS-M3 reading:", needs: "WS-10"}

## Wave 4: the preflight site against a fitted baseline, E4 (section 3)

- [ ] A 5-fold held-out logistic regression on the U6 features, settled by effort to proof (the session ledger joined to the verify ledger) {id: WS-12, symbol: fitPreflightBaseline, package: internal/console, reachable_from: TestFitPreflightBaselineHeldOutFiveFold, needs: "work-intelligence.md#WI-16,work-intelligence.md#WI-19,progress-and-economics.md#PE-24"}
- [ ] (op) WS-M4: the model's score beats the fitted baseline on held-out AUROC by at least 0.05 with the CI excluding 0, or the U6 model site is dropped and the fitted model serves {id: WS-M4, oracle: lexical, path: docs/bench/readings/work-signals/preflight-baseline.md, pattern: "^WS-M4 verdict: (PASS|DROP)", needs: "WS-12"}

## Deferred (recorded so they are not re-proposed)

- [-] A typed model classifies ambiguous stall episodes as productive iteration versus stuck: sampled, calibrated against the owner's labels through the one-key verdict, an opt-in site (it joins `decide.OptInSites` when built, R2), never required for detection {id: WS-D1, disposition: deferred, reason: "detection is deterministic first (WS-X2); the classifier needs its own calibration (R3, R4) and episodes the deterministic checks cannot settle", trigger: "WS-M1 shows ambiguous episodes exist and a provider passes calibration on this site (R3)"}

## Refused (recorded so they are not re-proposed)

- [-] Fathom switching models, respawning or dispatching in response to a stall {id: WS-X1, disposition: refused, reason: "the dispatch non-goal: fathom serves facts about the work and may write a signal an orchestrator reads"}
- [-] A model required for stall detection {id: WS-X2, disposition: refused, reason: "detection is deterministic first; a model only classifies ambiguous episodes, sampled and calibrated (R3, R4)"}
- [-] Ranking or scoring people by stall cost or weight error {id: WS-X3, disposition: refused, reason: "I58: every per-author figure is an agent identity"}
