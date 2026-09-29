# Judged telemetry: the plan (vision 40, ADR-056)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/judged-telemetry.md` proves it. Design:
`docs/design/judged-telemetry.md` (section numbers below are its). Goal G29 (with
`docs/plans/judgment-foundations.md`).

The flywheel reframed as a label economy: judgments that are forecasts the index settles grade
themselves, and the scarce input, human labels, goes to one template at a time. The instrument fix and
the calibration readings are in `docs/plans/judgment-foundations.md` (JF-1 to JF-12). What is here is the
labeled template `failure_kind` and the machinery that waits on RT-M1. Beside
`docs/plans/governance-learning.md` (GL-10 to GL-13) and `docs/plans/decision-port.md` (DP).

## Wave 1: the one labeled template (sections 2 and 3)

- [ ] `settleFailureKind` settles a `failure_kind` forecast from where the fix lands: test files only means test_expectation_outdated; the next run passes with no change to reached symbols means timing_or_flaky; a change to reached non-test code before green means code_bug {id: JT-1, symbol: settleFailureKind, package: internal/testrun, reachable_from: TestSettleFailureKindFromWhereTheFixLands, needs: "test-run-hook.md#TR-4,test-run-hook.md#TR-5"}
- [ ] `fathom adjudicate`: a batch of 10 items, one key each, every label recorded with provenance `human`, tied to a decision the owner already makes; labels go to one template until it passes or is dropped (R4) {id: JT-2, symbol: cmdAdjudicate, package: cmd/fathom, reachable_from: TestAdjudicateBatchOfTenRecordsHumanProvenance, needs: "judgment-foundations.md#JF-11,judged-predicates.md#JP-M1", weight: 3}
- [ ] The `failure_kind` calibration set grows from settled and adjudicated rows, its provenance mix (human, frontier, computed) shown in the reading {id: JT-3, symbol: writeCalibrationReading, package: cmd/fathom, reachable_from: TestFailureKindCalibrationSetGrowsWithProvenance, needs: "JT-1,JT-2"}
- [ ] (op) JT-M1: `failure_kind` judged versus settled or human agreement at least 0.8 on at least 100 adjudicated items across at least 3 sittings, or the template is dropped after one revision {id: JT-M1, oracle: lexical, path: docs/bench/readings/judged-telemetry/failure-kind.md, pattern: "^JT-M1 verdict: (PASS|DROPPED)", needs: "JT-3", weight: 3}
- [ ] (op) JT-M2: a settled template's Brier score beats its base-rate forecast on its first settled batch {id: JT-M2, oracle: lexical, path: docs/bench/readings/judged-telemetry/failure-kind.md, pattern: "^JT-M2 reading:", needs: "JT-3"}

## Deferred behind RT-M1 and a data trigger (section 4)

- [-] The sampler, queue, background judge and judgments table: deterministic seeded stratified sampling, a per-day cap on count and USD, a daemon worker admitted by cost class and closed at refuse and in safe mode, spend labeled and excluded from A/B units (I46), a settlement job {id: JT-D1, disposition: deferred, reason: "R5 and ADR-049 decision 2: no new table, queue or background job before RT-M1 reads PASS", trigger: "responsible-tenant.md#RT-M1 reads PASS, and tier 2 or ask needs more labels than settlement supplies (F40)"}
- [-] The owner's call before JT-D1: unfreeze the assess lanes (ADR-046) as the judged-telemetry host, or supersede them; a parallel lane is refused {id: JT-D2, disposition: deferred, reason: "judged telemetry would otherwise rebuild assess's consent capability and spend isolation", trigger: "JT-D1's trigger fires"}
- [-] Fitted recalibration and site auto-disable (a site whose rolling Brier score falls below the base rate for 14 days disables itself) {id: JT-D3, disposition: deferred, reason: "a fit at n near 100 is noise; auto-disable needs a minimum n per window or it flaps", trigger: "one (site, provider, model) holds at least 100 labels (F39)"}
- [-] Guard proposals from rule outcomes: a rule with a checkable condition repeatedly not met produces a deterministic proposal a human accepts; never sets `enforce: true` on its own; model-judged hunks are never written into tests {id: JT-D4, disposition: deferred, reason: "the signal is mislabeled until JF-1 ships", trigger: "JF-1 has shipped and 30 days of relabeled outcomes show such a rule (F41)"}
- [-] U3 friction episode classification and product feedback {id: JT-D5, disposition: deferred, reason: "no consumer yet (P20); feedback needs labels worth sharing and goes through the research bundle's closed allowlist, run by a human", trigger: "U3 exists, or labels worth sharing exist (F49)"}

## Refused (recorded so they are not re-proposed)

- [-] Judged rule compliance, either feeding learning or served as a column now {id: JT-X1, disposition: refused, reason: "contradicts governance-learning section 11 and ADR-049 decision 7; no labels exist; revisit through an ADR-049 decision 7 amendment backed by a template that passes R3 on owner-adjudicated hunks"}
- [-] Jev for answer usefulness, config custodian proposals and a weekly digest {id: JT-X2, disposition: refused, reason: "the first two are joins over structured events; a digest is narrative (P14) and Jev cannot write it"}
- [-] A judged-telemetry lane parallel to assess {id: JT-X3, disposition: refused, reason: "ADR-046 already owns consent, spend isolation and the judge process; a second lane forks them"}
