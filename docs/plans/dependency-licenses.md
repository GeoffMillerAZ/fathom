# Dependency licenses: the plan

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/dependency-licenses.md` proves it. Design:
`docs/design/dependency-licenses.md` (section numbers below are its). Roadmap section 9.11. Invariants I55, I56, I62.

Built after RT-M1 PASS. Every reading is $0. Wave 1 derives on read and adds no table, column, cache or background job
(ADR-049 decision 2). No row fetches anything over the network (I56), and no row judges a license (ADR-060 decision 6).

## Wave 1: the inventory, derived on read

- [ ] Each dependency's on-disk directory is located through the dependency plane's cache locator for Go, npm, Cargo, Python, Maven and Ruby, and a dependency with nothing on disk reads `unknown (not on disk)`; nothing dials (§3) {id: DL-1, symbol: LocateLicenseFiles, package: internal/deplicense, reachable_from: TestLocateLicenseFilesNeverDials, needs: "SG-36"}
- [ ] A license is identified from the declared SPDX expression, else a `licensecheck` match above its coverage threshold, else `unknown`; a declared-versus-text disagreement is shown, not resolved, and every answer names its source (§3) {id: DL-2, symbol: IdentifyLicense, package: internal/deplicense, reachable_from: TestIdentifyLicenseDeclaredTextAndUnknown, needs: "DL-1"}
- [ ] The policy evaluates SPDX expressions (`OR`, `AND`, `WITH`) to `allow`, `review`, `deny` or `unknown`, applies scopes and dated exceptions, and an expired exception says so (§4) {id: DL-3, symbol: EvaluateLicensePolicy, package: internal/deplicense, reachable_from: TestEvaluateLicensePolicyExpressionsScopesExceptions, needs: "DL-2"}
- [ ] The `dependencies.licenses` config section, settable by a platform-module binding and narrowed by a repository under scoped-config precedence, with valid and invalid schema testdata (§4) {id: DL-4, symbol: LicensePolicy, package: internal/config, reachable_from: TestLicensePolicyConfigValidAndInvalid}
- [ ] `fathom deps licenses --service S` prints the inventory and verdicts as text or JSON, and `--check` exits 1 on any `deny` (§5) {id: DL-5, symbol: cmdDepsLicenses, package: cmd/fathom, reachable_from: TestCmdDepsLicensesCheckExitsOnDeny, needs: "DL-3,DL-4"}
- [ ] Each verdict carries its reach from import edges: shipped, test-only or never imported, and `scopes` decides whether test-only reach counts (§6) {id: DL-6, symbol: licenseReach, package: internal/deplicense, reachable_from: TestLicenseReachShippedTestOnlyUnused, needs: "DL-3"}
- [ ] (op) DL-M1: on a pinned seeded corpus (fathom's own Go modules plus one npm, one Cargo and one Python fixture tree), identified licenses agree with a reference SBOM tool's output on at least 95% of dependencies both identify, every disagreement listed, and the `unknown` share printed per ecosystem {id: DL-M1, oracle: lexical, path: docs/bench/readings/dependency-licenses/accuracy.md, pattern: "^DL-M1 verdict: (KEEP|CUT)", needs: "DL-2"}

## Wave 2: at the edit and in review

- [ ] The guard check `dep_policy` denies an edit to a recognized manifest that adds a dependency whose verdict is `deny`, naming the dependency, its license and the policy entry; `review` allows with a note; off by default and fails open (§5) {id: DL-7, symbol: checkDepPolicy, package: internal/guardspec, reachable_from: TestGuardDepPolicyDeniesAddedDeniedDependency, needs: "DL-3"}
- [ ] `review_diff` reports dependencies added, removed or re-versioned, each with its verdict (§5) {id: DL-8, symbol: reviewDependencyDelta, package: internal/review, reachable_from: TestReviewDiffReportsDependencyLicenseDelta, needs: "DL-3"}
- [ ] An operator-named SPDX 2.3 or CycloneDX 1.5 JSON file fills licenses for dependencies with nothing on disk, each labelled with that file as its source, and nothing dials (§3) {id: DL-9, symbol: LoadSBOMLicenses, package: internal/deplicense, reachable_from: TestLoadSBOMLicensesFillsUnknownNoNetwork, needs: "DL-2"}
- [ ] (op) DL-M2: the `dep_policy` check adds at most 50 ms p95 to a manifest edit on the seeded corpus with warm caches, measured by CPU time; a CUT gates a persisted license fact behind RT-M1 PASS {id: DL-M2, oracle: lexical, path: docs/bench/readings/dependency-licenses/guard-latency.md, pattern: "^DL-M2 verdict: (KEEP|CUT)", needs: "DL-7"}

## Wave 3: the fleet

- [ ] The fleet view: one policy's verdicts across every indexed service, with each service's `deny` and `unknown` counts (§2) {id: DL-10, symbol: fleetLicenseVerdicts, package: internal/console, reachable_from: TestFleetLicenseVerdictsAcrossServices, needs: "DL-5"}
- [-] License categories (permissive, weak copyleft, strong copyleft, network copyleft) as a pinned public table with its source named, so a policy can deny a family instead of listing identifiers {id: DL-11, disposition: deferred, reason: "build when an adopter's policy asks for families; a category is a published mapping fathom would import, never one it decides (ADR-060 decision 6)"}
