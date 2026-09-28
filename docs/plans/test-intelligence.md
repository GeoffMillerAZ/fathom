# Test intelligence: the plan (vision 38 §4, ADR-051)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/test-intelligence.md` proves it.
Design: `docs/design/test-intelligence.md` (section numbers below are its). Goal G23
(TQ-M1..M3).

Not re-claimed here: TC-1 to TC-24 (certificates, liveness, `certify` verbs, the review
dimension) and RC-37 onward (Go coverage by layer) are shipped; TC-M2, TC-M3 and TC-M4 stay
open in `docs/plans/test-certifications.md`. T7's rows are TC rows; only its measurement
(TQ-M4) is here. T8 is `certified_by:` (`docs/plans/spec-interop.md` SI-14, SI-15).

## Wave 2: risk-weighted coverage (T1) and the test portfolio (T2)

- [x] Churn per file (commits in 180 days) computed at ingest through `gitro` and stored; never at request time; the retired churn analyzer stays retired {id: TQ-1, symbol: computeFileChurn, package: internal/fusion, reachable_from: TestFileChurnAtIngest}
- [x] Centrality per symbol (transitive caller count, normalized) memoized at ingest {id: TQ-2, symbol: computeCentrality, package: internal/fusion, reachable_from: TestCentralityAtIngest}
- [x] Governance severity per file (enforced guard 1.0, fresh obligation claim 0.5, else 0.1; design §1) and incident count per file from incident records and findings {id: TQ-3, symbol: riskInputs, package: internal/certify, reachable_from: TestRiskInputsSeverityAndIncidents}
- [x] The risk rank: product of normalized factors, each shown with its citation, sorted with a `LIMIT` (§3 T1) {id: TQ-4, symbol: RiskWeightedCoverage, package: internal/certify, reachable_from: TestRiskWeightedCoverageExplainable, needs: "TQ-1,TQ-2,TQ-3"}
- [x] `test_impact` with `rank: risk` serves the list (no new tool) {id: TQ-5, symbol: testImpactRiskRank, package: internal/mcp, reachable_from: TestTestImpactRiskRank, needs: "TQ-4"}
- [x] The test portfolio classes: certifying, reaching governed code, orphan, redundant (exact set equality), critical target without a certificate (§3 T2) {id: TQ-6, symbol: ClassifyTests, package: internal/certify, reachable_from: TestClassifyTestsPortfolio}
- [x] A purpose line per test: what it certifies at which strength, or "reaches X, certifies nothing" {id: TQ-7, symbol: testPurpose, package: internal/certify, reachable_from: TestTestPurposeLine, needs: "TQ-6"}
- [x] The portfolio and risk views on the console {id: TQ-8, symbol: renderTestPortfolio, package: cmd/fathom, reachable_from: TestConsoleTestPortfolio, needs: "TQ-4,TQ-6"}

## Wave 3: coverage for every language (T3) and certifying-first selection (T4)

- [ ] One block model for foreign coverage; `fathom coverage ingest <file>` bound to a commit or `unbound` (never gating), streamed, 256 MiB cap {id: TQ-9, symbol: cmdCoverageIngest, package: cmd/fathom, reachable_from: TestCmdCoverageIngestBindsCommit}
- [ ] lcov (`.info`) parser {id: TQ-10, symbol: ParseLcov, package: internal/analysis/coverage, reachable_from: TestParseLcov, needs: "TQ-9"}
- [ ] Cobertura XML and JaCoCo XML parsers {id: TQ-11, symbol: ParseCoberturaJaCoCo, package: internal/analysis/coverage, reachable_from: TestParseCoberturaJaCoCo, needs: "TQ-9"}
- [ ] coverage.py JSON and Istanbul `coverage-final.json` parsers {id: TQ-12, symbol: ParseCoveragePyIstanbul, package: internal/analysis/coverage, reachable_from: TestParseCoveragePyIstanbul, needs: "TQ-9"}
- [ ] llvm-cov JSON export parser {id: TQ-13, symbol: ParseLLVMCov, package: internal/analysis/coverage, reachable_from: TestParseLLVMCov, needs: "TQ-9"}
- [ ] Foreign blocks join to symbols by line range where the language has L1 ranges (LB-F3), else at file level, named {id: TQ-14, symbol: joinForeignBlocks, package: internal/analysis/coverage, reachable_from: TestJoinForeignBlocksByRange, needs: "TQ-10,language-breadth.md#LB-F3"}
- [ ] Certifying-first selection: the minimal set re-proving every touched certificate (greedy set cover, deterministic tie-break), `--safe` adds every reaching test (§3 T4) {id: TQ-15, symbol: SelectCertifyingFirst, package: internal/certify, reachable_from: TestSelectCertifyingFirstCoversTouched}
- [ ] The selection's CI time saved from attested run durations, recorded for economics {id: TQ-16, symbol: selectionTimeSaved, package: internal/certify, reachable_from: TestSelectionTimeSaved, needs: "TQ-15"}

## Wave 4: contract certificates on links (T5) and upgrade test reach (T6)

- [ ] A link is certified when both ends carry live certificates (provider test, consumer contract test); uncertified `spec`/`typed` links listed for the risk receipt (takes TC-25 with its trigger) {id: TQ-17, symbol: CertifyLinks, package: internal/certify, reachable_from: TestCertifyLinksBothEnds, needs: "test-certifications.md#TC-M2,system-graph.md#SG-9"}
- [ ] Uncertified links feed `review_diff`'s receipt {id: TQ-18, symbol: uncertifiedLinksForReceipt, package: internal/certify, reachable_from: TestUncertifiedLinksInReceipt, needs: "TQ-17,change-and-compliance.md#CX-1"}
- [ ] Upgrade test reach: tests that statically reach our consumes of the `pkg:` symbols a bump changed or removed {id: TQ-19, symbol: UpgradeTestReach, package: internal/certify, reachable_from: TestUpgradeTestReach, needs: "system-graph.md#SG-39"}
- [ ] `contract_drift {pkg}` output lists the tests to run before merging the bump {id: TQ-20, symbol: driftTestsToRun, package: internal/syslink, reachable_from: TestContractDriftListsTests, needs: "TQ-19"}

## Refused (recorded so they are not re-proposed)

- [-] Reviving the churn or cognitive analyzers {id: TQ-X1, disposition: refused, reason: "retired (docs/plans/retire-unused-analyzers.md); churn is an ingest-time git fact and size a named proxy (ADR-051 decision 1)"}
- [-] Selection that marks a certificate live {id: TQ-X2, disposition: refused, reason: "I43: only an attested passing run makes a certificate live"}
- [-] Near-duplicate test detection by similarity {id: TQ-X3, disposition: refused, reason: "redundancy is exact set equality; similarity is a guess"}

## Measurements (op)

- [ ] (op) TQ-M1: risk-weighted coverage ranked on the home fleet; a human check of the top 20 per service with at least 14 judged dangerous and under-tested {id: TQ-M1, oracle: lexical, path: docs/bench/readings/tests/risk-coverage.md, pattern: "^TQ-M1 verdict: PASS", needs: "TQ-4"}
- [ ] (op) TQ-M2: a replayed month of diffs: selection re-proves every touched certificate at no more than 40% of the suite and misses no failure a full run caught {id: TQ-M2, oracle: lexical, path: docs/bench/readings/tests/selection-replay.md, pattern: "^TQ-M2 verdict: PASS", needs: "TQ-15"}
- [ ] (op) TQ-M3: coverage ingested for at least 5 languages, blocks joined to symbols {id: TQ-M3, oracle: lexical, path: docs/bench/readings/tests/coverage-languages.md, pattern: "^TQ-M3 verdict: PASS", needs: "TQ-14"}
- [ ] (op) TQ-M4: T7 measured: flaky certificates counted over 30 days, every live certificate's run bound to a commit {id: TQ-M4, oracle: lexical, path: docs/bench/readings/tests/flakiness.md, pattern: "^TQ-M4 verdict: PASS"}
- [ ] (op) TQ-M5: the portfolio on this repository: counts per class, a human check of 10 orphans and 10 redundant pairs {id: TQ-M5, oracle: lexical, path: docs/bench/readings/tests/portfolio.md, pattern: "^TQ-M5 verdict: PASS", needs: "TQ-6"}
