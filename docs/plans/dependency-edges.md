# Dependency edges

Grammar v2 (`docs/plans/README.md`). Prefixes DEP-, BND- and PF-. Decisions ADR-114 and ADR-115.
[Design](../design/dependency-edges.md) · [Contract](../contracts/dependency-edges.md).

## Records and public door

- [x] (op) Record the accepted dependency-edges boundary, contract, preregistered admission bars and register {id: DEP-0, oracle: lexical, path: ADR.md, pattern: "^## ADR-114: Standalone dependency edges disclose unknowns per file"}
- [x] Public door, closed sets, completeness, deterministic output, CUE schema and API snapshot {id: DEP-1, symbol: ReadFiles, package: pkg/lang/deps, needs: DEP-0}
- [x] Go control reader and independent-oracle grading harness {id: DEP-2, symbol: readGo, package: pkg/lang/deps, reachable_from: ReadFiles, needs: DEP-1}
- [x] JSON CLI with golden and the declared-edge export {id: DEP-3, symbol: declaredPairs, package: cmd/fathom-lang, reachable_from: cmdDeps, needs: "DEP-1,DEP-2"}

## Dialect readings and extension

- [x] TypeScript and JavaScript reader and resolution with DEP-M1 {id: DEP-4, symbol: readTSJSDependencies, package: pkg/lang/deps, reachable_from: ReadFiles, needs: DEP-3}
- [x] Python reader and resolution with DEP-M2 {id: DEP-5, symbol: readPythonDependencies, package: pkg/lang/deps, reachable_from: ReadFiles, needs: DEP-3}
- [x] Dockerfile reader and resolution with DEP-M3 {id: DEP-6, symbol: readDockerfileDependencies, package: pkg/lang/deps, reachable_from: ReadFiles, needs: DEP-3}
- [ ] Resolver registration, pack captures where needed, declared edges, trust, conformance helper and runnable example {id: DEP-7, needs: "DEP-4,DEP-5,DEP-6"}

## Import boundaries (ADJ-5)

Start after DEP-1 through DEP-3 land.

- [ ] Evaluate declared direct-edge rules with violated, clean and unverified verdicts; hold the wire model to schema and fixture checks {id: BND-1, needs: DEP-3}
- [ ] Run the rules from `fathom-lang boundaries --rules` with stable JSON, findings and strict exit status {id: BND-2, needs: BND-1}
- [ ] Evaluate the rule file in the family gate and match the Go import tests on all seeded forbidden edges {id: BND-3, needs: BND-2}
- [-] Read gate evaluations, violations caught before merge and outside uses (BND-M1). Keep if a violation was caught more clearly than by the older tests or an outside use is observed {id: BND-M1, disposition: deferred, reason: "observe use after the boundary gate lands", trigger: "30 days after BND-3 lands", needs: BND-3}

## Engine move and computed path filters (ADJ-6)

- [-] Move the generic closure engine and conformance suite into `pkg/depgraph`. Enforce family import separation and obtain HOLD-18 acceptance {id: DEP-P1a, disposition: deferred, reason: "DEP-M2 alone qualified because DEP-M1 had mixed admission and DEP-M3 was inconclusive", trigger: "two of DEP-M1 through DEP-M3 pass B1 and B2", needs: DEP-7}
- [-] Read push and pull request path filters with platform glob conformance and disclose unevaluated expressions {id: PF-1, disposition: deferred, reason: "the required family closure engine is held", trigger: "DEP-P1a lands", needs: DEP-P1a}
- [-] Join the declared entry map, direct edges, closure and filters in `fathom-lang pathfilters` and prove the three verdicts and unused patterns {id: PF-2, disposition: deferred, reason: "the required family closure engine is held", trigger: "DEP-P1a lands and PF-1 is verified", needs: PF-1}
- [-] Wire the first use in a fixture repository and detect every seeded miss with no false misses on the control. The first real consumer is the CI kit after DEP-P1b {id: PF-3, disposition: deferred, reason: "the required family closure engine is held", trigger: "DEP-P1a lands and PF-2 is verified", needs: PF-2}
- [-] Read the first real workflow wiring and decide whether to keep or demote the check {id: PF-M1, disposition: deferred, reason: "fixture wiring alone does not start the real-use window", trigger: "30 days after the first wiring outside fixtures", needs: "PF-3,DEP-P1b"}

## Deferred

- [-] Audit under-keying by comparing declared inputs, static edges and observed reads (formerly TQ-38) {id: DEP-8, disposition: deferred, reason: "the consumer is named and needs measured static edges", trigger: "DEP-M1 through DEP-M3 have been read and one consumer wires dependency edges"}
- [-] Complete the owner-approved staged migration of the one closure engine {id: DEP-P1, disposition: deferred, reason: "owner-approved 2026-11-18, split into the gated move DEP-P1a and the consumer switch DEP-P1b by ADR-115", trigger: "two of DEP-M1 through DEP-M3 pass B1 and B2", needs: "DEP-P1a,DEP-P1b"}
- [-] Switch the existing CI kit consumer to the family closure engine and reconcile both boundary records {id: DEP-P1b, disposition: deferred, reason: "the consuming repository owns the switch in a separate program", trigger: "DEP-P1a lands and the consumer switch program starts", needs: DEP-P1a}
- [-] Add remaining dialects, cheapest class first {id: DEP-9, disposition: deferred, reason: "first prove the readers and the declared exchange", trigger: "DEP-P1 lands or a second consumer is named"}
