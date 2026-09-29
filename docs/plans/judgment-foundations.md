# Judgment foundations: the plan (vision 40, ADR-056)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/judgment-foundations.md` proves it. Design:
`docs/design/judgment-foundations.md` (section numbers below are its). Goal G29 (with
`docs/plans/judged-telemetry.md`).

These are the corrections and guards the rest of the judgment program stands on (sequence step 0):
the rule-outcome instrument fix, per-site opt-in, route off by default, a deterministic
certify_suggest ladder, calibration readings, and the doc corrections. Nothing here needs a provider,
a table or a new tool. The rows sit beside `docs/plans/decision-port.md` (DP), `docs/plans/surface-reckoning.md`
(SR-61) and `docs/plans/governance-learning.md` (GL-10), which they amend.

## Wave 0: the instrument and the guards (sections 4 to 7)

- [ ] A1: the rule-outcome tracker emits `still_served` (was `violated`) and `top_changed` (was `complied`); `violated` is kept only for a rule with a checkable condition (section 5) {id: JF-1, symbol: trackRuleOutcomeAt, package: cmd/fathom, reachable_from: TestRuleOutcomeRepeatEditEmitsStillServedNeverViolated, needs: "surface-reckoning.md#SR-61"}
- [ ] A1: every reader of the old event names (the rule-outcome report, config propose, config audit, the config-change hook) reads the new names, and existing ledger rows are read under their new names, never dropped {id: JF-2, symbol: recordRuleOutcome, package: cmd/fathom, reachable_from: TestRuleOutcomeReadersMapOldEventNames, needs: "JF-1"}
- [x] A1: the GL-10 readings are re-labeled: the governance-learning design carries the correction that the old events measured scope persistence, not compliance {id: JF-3, oracle: lexical, path: docs/design/governance-learning.md, pattern: "Correction 2026-09-29"}
- [x] A1: the defect is filed with `fathom observe` (O120), so the mislabeled instrument is on the live log {id: JF-22, oracle: lexical, path: .fathom/observations.md, pattern: "^### O120 "}
- [ ] A2: `DefaultDecide.Sites` is frozen at today's list minus `route`, `decide.OptInSites` lists `triage_failure`, `ask`, `explore`, `compact_advice` and the `judge_*` family, and `SiteAllowed` refuses an opt-in site unless the repository's `[decide] sites` names it, even with a provider set (section 4) {id: JF-4, symbol: SiteAllowed, package: internal/decide, reachable_from: TestSiteAllowed_OptInSitesRefusedUnderDefault, weight: 3}
- [ ] A2: `TestSiteAllowed_Allowlist` asserts `AllSites` minus `OptInSites` are allowed by default, so the unconfigured behaviour of every existing site stays byte-identical (I42) {id: JF-5, symbol: SiteAllowed, package: internal/decide, reachable_from: TestSiteAllowed_Allowlist, needs: "JF-4"}
- [ ] A2: a missing key, a timeout, a 429 or 529, refuse mode and safe mode each yield exactly one disclosed line and the deterministic path, for every site {id: JF-6, symbol: Open, package: internal/decide, reachable_from: TestDecideDegradesToOneDisclosedLine, needs: "JF-4"}
- [ ] A6: `route` leaves `DefaultDecide.Sites` (the code stays; SR-44's deterministic precheck stays) (section 6) {id: JF-7, symbol: Load, package: internal/config, reachable_from: TestConfig_DecideDefaultSitesExcludeRoute, needs: "surface-reckoning.md#SR-44"}
- [-] A6 re-admission of `route`: at least 100 real prompts replayed through 4 to 6 decomposed nouls, weights fitted on half and tested on the other half, judged against R3 {id: JF-8, disposition: deferred, reason: "route failed calibration: 45.7 percent right at 0.9 or more, non-monotonic (ADR-042 amendment 2026-09-24)", trigger: "a replay of at least 100 real prompts clears R3's hint bar on a held-out half"}
- [ ] A7: `certify_suggest` computes which strength levels the evidence permits (assertion calls present, a golden file read, a property library imported, a mutation report present) and offers the model only those (section 7) {id: JF-9, symbol: buildCertifySuggestRequest, package: cmd/fathom, reachable_from: TestCertifySuggestNeverOffersALevelTheEvidenceRulesOut, needs: "test-certifications.md#TC-1"}

## Wave 1: calibration is read, not asserted (sections 2 and 8)

- [ ] R8: the resolved model is logged on every decision row and a model move marks the site "recalibration due" {id: JF-10, symbol: Recorder.Record, package: internal/decide, reachable_from: TestRecorderMarksRecalibrationDueOnModelMove}
- [ ] A8: a calibration reading per (site, template@version, provider, resolved model): n, accuracy by confidence band, Wilson bounds, Brier score, AUROC, the label provenance mix (human, frontier, computed) {id: JF-11, symbol: writeCalibrationReading, package: cmd/fathom, reachable_from: TestWriteCalibrationReadingCarriesWilsonBrierAndProvenance, needs: "JF-10"}
- [ ] `fathom decide calibrate --provider X` re-grades every site against its calibration set, so adopting a better model is a measurement (owner rule 2) {id: JF-12, symbol: cmdDecideCalibrate, package: cmd/fathom, reachable_from: TestCmdDecideCalibrateRegradesEverySite, needs: "JF-11"}
- [ ] Rule 3 of the bet: the confidence bands a site may use are read from its latest calibration reading, so re-grading a better provider widens them and a model move narrows them back, with no code change; a site with no passing reading has no band and answers on its deterministic path {id: JF-23, symbol: bandsFromReading, package: internal/decide, reachable_from: TestBandsWidenAndNarrowWithTheLatestReading, needs: "JF-11"}
- [-] Hexagonal hardening of the decision port (ADR-056 decision 11), implemented in agentic-kit `foundations/decision` and proven there by its `docs/plans/decision-port.md` section "Hexagonal hardening", rows DP-12 to DP-24: (1) a generic kind with the vendor term as a wire alias (DP-12); (2) vendor limits out of core validation, declared by adapters (DP-13); (3) adapters in subpackages, the core importing no vendor code (DP-14); (4) `Capabilities()` (DP-15); (5) a `decisiontest` conformance kit (DP-16); (6) cascade, fallback, cache and record and replay decorators with policy in configuration (DP-17 to DP-21); (7) a calibration mapping in the core and a readings library (DP-22, DP-23); plus the compat and migration record for the two breaking moves (DP-24) {id: JF-13, disposition: deferred, reason: "the kit owns the port; the rows are recorded there (2026-09-29) and land after the current fathom push", trigger: "agentic-kit docs/plans/decision-port.md rows DP-12 to DP-24 are proven"}
- [ ] After the kit lands, fathom bumps its vendored mirror (`task sync-kit`, then `task vendor-check`) and the port's `Capabilities()` is readable here {id: JF-14, oracle: lexical, path: third_party/agentic-kit/foundations/decision/capabilities.go, pattern: "func .*Capabilities", needs: "JF-13"}
- [ ] Fathom's own call sites follow the kit's hardening: they use the generic kind name and the `decision/llm` and `decision/typesafe` constructors, and existing config and `decisions.jsonl` rows that say `noul` still read {id: JF-24, symbol: Open, package: internal/decide, reachable_from: TestDecideOpenUsesSubpackageAdaptersAndStillReadsNoulRows, needs: "JF-14"}
- [-] A fitted recalibration map (Platt or temperature) and site auto-disable {id: JF-15, disposition: deferred, reason: "a fit at n near 100 is noise and cannot repair route's non-monotonic failure; auto-disable needs a minimum n per window or it flaps", trigger: "one (site, provider, model) holds at least 100 labels (F39)"}

## Invariants I60 to I62 (ADR-056)

- [ ] I62: a judgment prunes, gates or reorders a default list only when its calibration reading's Wilson 95 percent lower bound is at least 0.90, and every removed row is named {id: JF-19, symbol: judgmentMayPrune, package: internal/decide, reachable_from: TestJudgmentNeverPrunesBelowR3Bar, needs: "JF-11"}
- [ ] I62: a security property is never judged for serving; a template naming one is refused {id: JF-20, symbol: refuseSecurityTemplate, package: internal/decide, reachable_from: TestSecurityPropertiesNeverJudgedForServing, needs: "JF-4"}
- [ ] I61: a request plan carries only the spans, assembled context and question text a site declares, secret-shaped strings redacted and counted, `[decide] exclude` honoured, and prints exactly what would leave {id: JF-21, symbol: buildRequestPlan, package: internal/decide, reachable_from: TestRequestPlanCarriesOnlyDeclaredSpansRedacted, needs: "JF-4"}

## Corrections and records (sections 9 and 2)

- [x] A5: governance-learning section 11 no longer says Jev proposes a rewrite; Jev cannot generate text {id: JF-16, oracle: lexical, path: docs/design/governance-learning.md, pattern: "Jev cannot generate text"}
- [x] A5: knowledge-retrieval section 10 no longer says Jev reformulates a query {id: JF-17, oracle: lexical, path: docs/design/knowledge-retrieval.md, pattern: "Jev cannot generate text"}
- [x] A5: the conservation reach is stated exactly (an expected constant edited to match buggy output reads as `edited`) {id: JF-18, oracle: lexical, path: docs/design/refactor-conservation.md, pattern: "Reach, stated exactly"}

## Refused and deferred (recorded so they are not re-proposed)

- [-] Judged rule compliance, either feeding learning or served as a column now {id: JF-X1, disposition: refused, reason: "contradicts governance-learning section 11 and ADR-049 decision 7, and no labels exist"}
- [-] Jev for answer usefulness, config custodian proposals or a product digest {id: JF-X2, disposition: refused, reason: "the first two are joins over structured events; a digest is narrative (P14) and Jev cannot write it"}
- [-] A model deciding whether a question is computable {id: JF-X3, disposition: refused, reason: "route failed at exactly that; routing is structural (R1)"}

## Measurements (op)

- [ ] (op) JF-M1: a first calibration reading exists for `certify_suggest` (120 live rows on this machine) with n, Wilson bounds, Brier score and the label provenance mix {id: JF-M1, oracle: lexical, path: docs/bench/readings/decision-port/calibration-readings.md, pattern: "^JF-M1 reading:", needs: "JF-11"}
