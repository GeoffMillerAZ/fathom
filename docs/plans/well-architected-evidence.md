# Well-architected evidence: concern facts, joins and fact receipts (WAE-)

Design: `docs/vision/44-evidence-before-the-incident.md`. Decision: ADR-069. Goal: G37. Assessments:
`docs/assessments/2026-10-11-*.md`. Fathom stores concern facts and computes joins with coverage; Review Kit judges,
pe's `pdk/risk` tiers, policy-kit owns resource and security rules, and an external agent acts. Fathom never grades,
admits, holds the findings record, or executes, proposes or records a mitigation.

Gates: wave 0 is $0 readings, their bars fixed here before any run (K19: never moved after the data). Wave 1 rides
war1 (paused: `docs/plans/resume-2026-10.md` R1). Wave 2 rides language wave lf8 (ops formats, Kubernetes facts,
observed-by; vision 38 section 9.4 as corrected) and starts only after Review Kit commits the reliability bundle's
requirements. Wave 3 needs WAE-M2 KEEP. Wave 4 needs a WAE-M3 join KEEP. A fact family over the cost gate (K26: +20%
store or +15% wall, cold and warm, summed per wave and per language) ships only with an owner ruling beside its gain.
Kill (G37): on 2027-01-31, a WAE-M4 that is unread or CUT drops waves 3 to 4, not deferred. Capacity: about 3, 9, 9
and 6 worker-days for waves 1 to 4; about 23 owner hours. Every reading is $0 in model spend. Symbols are
provisional until grounded with `find_symbol` at row start.

## Wave 0: readings

- [ ] (op) WAE-M0: the owner commits a six-concern draft bundle (alert quality, probe semantics, resources and headroom, retries and timeouts per hop, PDB and spread, rollout safety) and stratified hand labels before any fathom run: 40 alerts and 30 workloads sampled by a committed rule from opentelemetry-demo, microservices-demo, kube-prometheus and istio samples at pinned SHAs, plus platform-engineering's substrate manifests for workload facts; who labels (the owner, or agents with a 20% owner audit) is the owner's U3 {id: WAE-M0, oracle: lexical, path: docs/bench/readings/well-architected/labels.md, pattern: "^WAE-M0 labels: [0-9]+ alerts, [0-9]+ workloads"}
- [ ] (op) WAE-M1: on the WAE-M0 corpus, the share of hand-labelled client construction sites that carry literal retry or timeout options or a named retry library; BUILD at 30% or more {id: WAE-M1, oracle: lexical, path: docs/bench/readings/well-architected/code-clients.md, pattern: "^WAE-M1 verdict: (BUILD|HOLD)", needs: "WAE-M0"}

## Wave 1: correctness and keys (rides war1)

- [x] The findings importer hashes cited lines at the event's `SourceRevision` through git, or refuses the cites link naming the skew when that revision is unreadable, never the working tree (internal/findings/external/importer.go `AnchorLinks`; the disk read at pkg/client/findings.go) {id: WAE-1, symbol: AnchorLinks, package: internal/findings/external, reachable_from: TestImportAnchorsHashAtSourceRevision}
- [x] `fathom version --json` adds `extractor_epoch`, `typed_edge_epoch`, `format_version` (`PRAGMA user_version`) and the fact-kind versions, the fathom-side components of an index snapshot's cache key {id: WAE-2, symbol: cmdVersion, package: cmd/fathom, reachable_from: TestVersionJSONCarriesCacheKeyComponents}
- [x] `read_source` takes an optional `ref` and serves the blob at that commit through git (`git show <ref>:<path>`), never the working tree, naming a filtered file; the default is unchanged; docs/mcp-tools.md updated in the same change {id: WAE-3, symbol: readSourceAtRef, package: internal/mcp, reachable_from: TestReadSourceRefServesBlobBytes}

## Wave 2: facts (rides lf8)

- [ ] Dialect facts are stored as observations of the entity's own file (the ADR-064 precedent), purged with the file, no table {id: WAE-4, symbol: storeEntityFacts, package: internal/fusion, reachable_from: TestEntityFactsStoredAsObservationsOfOwnFile}
- [ ] `symbol_context` on an entity serves its stored facts with anchors and coverage {id: WAE-5, symbol: entityFactsSection, package: internal/mcp, reachable_from: TestSymbolContextServesWorkloadFactsWithCoverage, needs: "WAE-4"}
- [ ] Prometheus `for`, `keep_firing_for` and annotations, Alertmanager `group_wait` and `repeat_interval` are facts {id: WAE-6, oracle: lexical, path: internal/extractor/dialects/prometheus/doc.json, pattern: "\"key\": \"keep_firing_for\"", needs: "WAE-4"}
- [ ] Kubernetes container facts widened under the ADR-067 decision 12 amendment: preStop, terminationGracePeriodSeconds, strategy, minReadySeconds, progressDeadlineSeconds, affinity, topology spread, env names and `valueFrom` references, arg flag names; env and arg values withheld except the two allow-lists (non-secret retry env keys; heap flags) {id: WAE-7, symbol: k8sContainerFacts, package: internal/extractor, reachable_from: TestK8sContainerFactsShutdownStrategyEnvNamesArgFlags, needs: "language-breadth.md#LB-F451"}
- [ ] A sentinel planted in a fixture workload's env value and arg value appears in no row of any store table and in no receipt (ADR-065 decision 2) {id: WAE-13, symbol: k8sContainerFacts, package: internal/extractor, reachable_from: TestK8sEnvArgSentinelInNoStoreRow, needs: "WAE-7"}
- [ ] (op) WAE-M2: fact precision at least 0.95 against a structural-YAML oracle on the calibration manifests, store growth at most 5% (K26), and WAE-13's sentinel in no row; CUT at under 0.90 or over 10% reverts that family {id: WAE-M2, oracle: lexical, path: docs/bench/readings/well-architected/fact-precision.md, pattern: "^WAE-M2 verdict: (KEEP|CUT)", needs: "WAE-6,WAE-7,WAE-13"}

## Wave 3: joins and transport

- [ ] J1: a probe's path resolves to its handler and names any database or network client the handler reaches, computed at request time {id: WAE-8, symbol: probeReach, package: internal/mcp, reachable_from: TestProbeReachNamesNetworkCallee, needs: "WAE-M2"}
- [ ] J2: a workload names its monitors, their alerts and each alert's receiver, computed at request time {id: WAE-9, symbol: alertChain, package: internal/mcp, reachable_from: TestWorkloadAlertChainNamesReceiver, needs: "telemetry-links.md#TL-15"}
- [ ] Security joins S1 to S3 and S6: IAM grant to service account to workload, public route to handler, workload to selecting NetworkPolicy (none named as an absence with its read set), workload to selecting policies, computed at request time {id: WAE-14, symbol: securityJoins, package: internal/mcp, reachable_from: TestSecurityJoinsNameGrantRouteAndPolicyCoverage, needs: "WAE-M2"}
- [ ] `fathom export receipt --kind fact|join` writes anchored receipts with coverage; both re-derive at `fathom@<version>`; an absence fact carries its read set {id: WAE-10, symbol: exportFactReceipt, package: cmd/fathom, reachable_from: cmdExport, needs: "portable-receipts.md#EVX-9,WAE-5"}
- [ ] (op) WAE-M3: per reliability join on the WAE-M0 corpus, at least one true instance per two repositories at precision at least 0.90 against the committed labels; a join under either bar is removed, not re-tuned on the same holdout {id: WAE-M3, oracle: lexical, path: docs/bench/readings/well-architected/joins.md, pattern: "^WAE-M3 verdict: ", needs: "WAE-8,WAE-9"}
- [ ] (op) WAE-M6: the WAE-M3 bar applied to each security join {id: WAE-M6, oracle: lexical, path: docs/bench/readings/well-architected/security-joins.md, pattern: "^WAE-M6 verdict: ", needs: "WAE-14"}

## Wave 4: proxies and the consumer

- [ ] Envoy retry_policy, timeouts, circuit_breakers, retry_budget, outlier_detection; Istio DestinationRule trafficPolicy and VirtualService retries and timeout are facts {id: WAE-11, oracle: lexical, path: internal/extractor/dialects/envoy/doc.json, pattern: "\"key\": \"retry_policy\"", needs: "WAE-M2"}
- [ ] J3: per-hop retries and timeouts along a route chain, with the attempt product and any violated timeout inequality named {id: WAE-12, symbol: retryChain, package: internal/mcp, reachable_from: TestRetryChainMultipliesAttemptsAcrossHops, needs: "WAE-11"}
- [ ] (op) WAE-M4: Review Kit's `fathom-facts` adapter closes at least one finding on pass with complete coverage, and owner-judged precision is at least 80% on 30 sampled findings over three repositories; the share of evaluated requirements depending on a fathom lane is recorded (F32) {id: WAE-M4, oracle: lexical, path: docs/bench/readings/well-architected/consumer.md, pattern: "^WAE-M4 verdict: (KEEP|CUT)", needs: "WAE-10"}
- [ ] (op) WAE-M5: at least 20 public postmortems, selected by a rule committed before the bundle is authored (each names a culprit commit in a public repository and a cause in a covered concern family), replayed at each culprit's parent; recall reported with its Wilson 95% lower bound; KEEP (a "prevention" claim allowed) only at a lower bound of at least 0.20 (8 of 20) {id: WAE-M5, oracle: lexical, path: docs/bench/readings/well-architected/replay.md, pattern: "^WAE-M5 verdict: (KEEP|CUT)", needs: "WAE-M4"}
- [ ] (op) WAE-M7: the latency Review Kit's admission adds per deploy with the fathom checks (median and p95 over 30 deploys), and the share of gate runs that fail open, each with its recorded reason; KEEP at most 2% over 30 days; CUT above 10%, after which the prod side leaves the gate's required inputs and is shown as advisory until a reading falls back under 2% {id: WAE-M7, oracle: lexical, path: docs/bench/readings/well-architected/gate-latency.md, pattern: "^WAE-M7 verdict: (KEEP|CUT)"}

## Deferred and refused

- [-] Code client-construction facts {id: WAE-D1, disposition: deferred, reason: "F71", trigger: "WAE-M1 reads BUILD"}
- [-] A rollback dimension of review --since over previous..deployed, extending incident maps 6.3 {id: WAE-D2, disposition: deferred, reason: "mitigation input decided in shape (ADR-069 decision 8)", trigger: "incident-maps.md#IM-M0 reads BUILD"}
- [-] Judged sites for hand-rolled retry loops, health-pool separation, alert investigative value and route auth middleware {id: WAE-D3, disposition: deferred, reason: "ADR-056 opt-in; the spend hold", trigger: "the spend hold lifts and WAE-M4 reads KEEP"}
- [-] Cross-repository pool arithmetic and retry amplification {id: WAE-D4, disposition: deferred, reason: "F72", trigger: "responsible-tenant.md#RT-M1 reads PASS and system-graph.md#SG-2 lands"}
- [-] Kubernetes resource or security rules in fathom or a tech pack {id: WAE-X1, disposition: refused, reason: "ADR-063 O11: policy-kit or Kyverno"}
- [-] A fathom deploy-gate, readiness verb or change-risk score {id: WAE-X2, disposition: refused, reason: "a second verifier (ADR-063 O2); Review Kit admit and pe pdk/risk decide"}
- [-] Fathom as the findings record {id: WAE-X3, disposition: refused, reason: "ADR-040; Review Kit ADR-0098"}
- [-] Fathom executes, proposes or records a mitigation {id: WAE-X4, disposition: refused, reason: "IM-X3 and IM-X9"}
- [-] A literal env or arg value in any fact or receipt {id: WAE-X5, disposition: refused, reason: "ADR-065 decision 2"}
