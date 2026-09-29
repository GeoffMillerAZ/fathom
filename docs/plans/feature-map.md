# The feature map: the plan (vision 39, ADR-055)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/feature-map.md` proves it. Design:
`docs/design/feature-map.md` (section numbers below are its). Goal G26.

Wave 5 of the proof-layer roadmap (ADR-049 decision 2, the tenancy lane). This plan follows
`docs/plans/system-graph.md#SG-2` (the `contract_keys`/`system_links` storage the feature
map's flags, config and dependency columns join) and `docs/plans/work-intelligence.md`'s U1/U2
rows (`WI-2`, `WI-5`, the cost and debt-interest machinery the feature map re-zones by feature
instead of by package zone) — no row here starts before those land, and none of them is
re-implemented here.

Verdicts stay human. FM-10, FM-11 and FM-M2 (the model verdict proposal, its ditch routing and its
calibration reading) are deferred by the owner's decision of 2026-09-29 (ADR-055 amendment, future F51):
they return only after J0 passes and a calibration against the owner's 86 labeled verdicts reaches the bar.

## Wave 5: the feature map (§3 to §11)

- [ ] `docs/features.md` grammar: plan-grammar-style rows (id, name, keys, owner, cited)
  parsed and refused by name when malformed (§3) {id: FM-1, symbol: ParseFeatureMap, package: internal/featuremap, reachable_from: TestParseFeatureMapGrammar}
- [ ] The closure check: every ADR-044/ADR-050 inventory key maps to exactly one feature or is
  reported unmapped; every feature's keys resolve to a live inventory entry or the row is
  reported dangling (§4) {id: FM-2, symbol: CheckFeatureClosure, package: internal/featuremap, reachable_from: TestFeatureClosureReportsUnmappedAndDangling, needs: "FM-1,system-graph.md#SG-2"}
- [ ] Auto-proposed feature candidates from the surface inventory, shown as proposals only,
  never written into `docs/features.md` (§4) {id: FM-3, symbol: ProposeFeatureCandidates, package: internal/featuremap, reachable_from: TestProposeFeatureCandidatesNeverWritesFile, needs: "FM-2"}
- [ ] The computed-facts join: code reached, specs/plan rows/ADRs citing it, T1/T2 coverage,
  churn, owners, docs linked (KR), env:/config: flags, pkg: dependencies, incidents — each
  column names its source, none computed twice (§5) {id: FM-4, symbol: ComputeFeatureFacts, package: internal/featuremap, reachable_from: TestComputeFeatureFactsEveryColumnSourced, needs: "FM-2,system-graph.md#SG-9,test-intelligence.md#TQ-4,knowledge-retrieval.md#KR-6"}
- [ ] Usage per feature: fathom's call and hook logs joined to the feature's keys; for a
  service, an imported access-log or analytics file bound to a commit; else "unknown", never
  guessed (§5) {id: FM-5, symbol: FeatureUsage, package: internal/featuremap, reachable_from: TestFeatureUsageNamedSourceOrUnknown, needs: "FM-4"}
- [ ] Cost and debt interest per feature: U1/U2 re-zoned to the feature's keys instead of
  package zones (§5) {id: FM-6, symbol: FeatureCostAndInterest, package: internal/featuremap, reachable_from: TestFeatureCostAndInterestReZoned, needs: "work-intelligence.md#WI-2,work-intelligence.md#WI-5"}
- [ ] The health rule table (`.fathom/features.cue` local fallback, defaults in the binary): a
  band from the strongest matched factor, never a summed score, every reason cited with its
  evidence, the same shape as the risk band (§6, CX-2) {id: FM-7, symbol: FeatureHealthBand, package: internal/featuremap, reachable_from: TestFeatureHealthBandStrongestReasonNeverSummed, needs: "FM-4,FM-5,FM-6"}
- [ ] Value: usage-derived where a usage source exists; "not measured" otherwise, never
  inferred (§6) {id: FM-8, symbol: FeatureValue, package: internal/featuremap, reachable_from: TestFeatureValueNotMeasuredWithoutUsage, needs: "FM-5"}
- [ ] Verdict authoring: a human's cited claim (keep/improve/merge/ditch) read from
  `docs/features.md` and served, never computed (§7) {id: FM-9, symbol: AuthoredFeatureVerdict, package: internal/featuremap, reachable_from: TestAuthoredFeatureVerdictCitedClaim, needs: "FM-1"}
- [-] The decision-port verdict proposal: a Choice over the computed evidence row, calibrated against the owner's 86 labeled verdicts (2026-09-27); shown as a proposal only, dropped below the agreement bar (§7, §8) {id: FM-10, disposition: deferred, reason: "owner decision 2026-09-29: deferred, not refused; verdicts stay human (FM-9) and a model proposal is not built until it is earned; the critics estimate that 86 labels over four classes cannot support the 0.8 bar", trigger: "judged-predicates.md#JP-M1 reads PASS (J0) and a calibration against the owner's 86 labeled verdicts reaches agreement of at least 0.8 at n of at least 3 (F51)"}
- [-] A ditch proposal never auto-applies; it routes into the surface-reckoning pattern (callers proved, citing rows re-anchored or refused) (§7) {id: FM-11, disposition: deferred, reason: "rides FM-10: with no model proposal there is no ditch proposal to route; a human-authored ditch verdict already reaches the surface-reckoning pattern by hand", trigger: "FM-10's trigger fires (F51)"}
- [ ] `fathom features [--json]` verb (§10) {id: FM-12, symbol: cmdFeatures, package: cmd/fathom, reachable_from: TestCmdFeaturesJSON, needs: "FM-7,FM-8,FM-9"}
- [ ] The console Features page: value x implementation scatter with verdict shapes, a
  filterable table, an underused list, a ditch list (§10) {id: FM-13, symbol: renderFeatureMap, package: cmd/fathom, reachable_from: TestConsoleFeatureMapPage, needs: "FM-12"}
- [ ] Artifact-style export reproducing the Feature Scorecard artifact's first reading (§10) {id: FM-14, symbol: cmdFeaturesExport, package: cmd/fathom, reachable_from: TestCmdFeaturesExportMatchesScorecardShape, needs: "FM-12"}

## Refused and deferred (recorded so they are not re-proposed)

- [-] Feature-to-customer-ticket or CRM joins {id: FM-X1, disposition: refused, reason: "the connector non-goal (D2)"}
- [-] Feature-to-revenue {id: FM-X2, disposition: refused, reason: "no source exists (D2)"}
- [-] Feature-to-SLO/alerts/runbooks {id: FM-X3, disposition: refused, reason: "Review Kit's ops domain; fathom supplies the code side only (D2)"}
- [-] Fathom deciding or auto-applying a verdict {id: FM-X4, disposition: refused, reason: "ADR-049 decision 12; ADR-040's authority boundary"}

## Measurements (op)

- [ ] (op) FM-M1: fathom's own 86-feature scorecard re-derived; evidence columns match the
  hand-built one on at least 90% of cells {id: FM-M1, oracle: lexical, path: docs/bench/readings/feature-map/scorecard.md, pattern: "^FM-M1 verdict: PASS", needs: "FM-4,FM-5,FM-6"}
- [-] (op) FM-M2: verdict-proposal agreement at least 0.8 at n at least 3 against the 86 owner verdicts, or the proposal path is dropped {id: FM-M2, disposition: deferred, reason: "measures FM-10, which is deferred (owner decision 2026-09-29); the calibration against the 86 labeled verdicts is what would earn the proposal path", trigger: "FM-10's trigger fires (F51); the reading then lands at docs/bench/readings/feature-map/verdict-calibration.md with a line starting FM-M2 verdict: PASS or DROPPED"}
