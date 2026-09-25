# Refactor conservation — the plan (vision 34, ADR-044)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/refactor-conservation.md`
proves it. Op rows carry a `skip` naming the measurement; they are never
silently skipped. Design: `docs/design/refactor-conservation.md`.

## A. Stage 1 — proof integrity

- [x] A test-kit envelope's `env.commit`, `env.dirty` and `env.race` (or an in-toto `gitCommit` subject) are read into the run {id: CV-1, symbol: runCommitFromEnvelope, package: internal/testevidence, reachable_from: TestLoadEnvelopes_CommitDirtyRace}
- [x] Stored test runs keep the commit, tree cleanliness and race mode they were made with {id: CV-2, symbol: SQLiteStore.PutTestRuns, package: internal/store, reachable_from: TestTestRuns_CommitRoundTrip}
- [x] A certificate is live only for a passing run bound to a commit after which nothing it reaches changed; unbound runs leave it unproven, a changed reach stales it (O114) {id: CV-3, symbol: CertificateStateAt, package: internal/certify, reachable_from: TestCertificateStateAt_EveryRule}
- [x] The reach check sees committed and uncommitted changes to the test file and every reached symbol's file {id: CV-4, symbol: reachUnchangedSince, package: internal/certify, reachable_from: TestReachUnchangedSince_CommittedAndWorktreeEdits}
- [x] The sweep stales a certificate whose reached symbol changed since its attested run, the half of TC-8 that O114 found missing {id: CV-5, symbol: SweepCertifications, package: internal/certify, reachable_from: TestSweepCertifications_ReachChangedSinceRunGoesStale}
- [x] `task tests:attest` re-attests this repository's runs with a commit and ingests them {id: CV-6, oracle: lexical, path: Taskfile.yml, pattern: "tests:attest"}
- [x] Guarded tests are found with their reason: certifies marker, manifest entry, invariant enforcement cell, compatibility-surface evidence, or reach of a changed symbol {id: CV-7, symbol: GuardedTests, package: internal/analysis/integrity, reachable_from: TestGuardedTests_EveryReason}
- [x] A function body's normalized hash is unchanged by a pure rename {id: CV-8, symbol: NormalizedBodyHash, package: internal/analysis/integrity, reachable_from: TestNormalizedBodyHash_RenameOnlyEqual}
- [x] Test integrity over base..head reports deleted, renamed, weakened, skipped, golden_changed, marker_moved and edited guarded tests {id: CV-9, symbol: Check, package: internal/analysis/integrity, reachable_from: TestCheck_EveryKind}
- [x] Declarations added inside the range (skip, disposition, retires, surface_change, raised max) and rows deleted in it are self-declared (I45) {id: CV-10, symbol: AnnotateSelfDeclared, package: internal/verify, reachable_from: TestAnnotateSelfDeclared_RangeWide}
- [x] A hand edit to a generated file whose generator inputs did not change is a finding {id: CV-11, symbol: GeneratedHandEdits, package: internal/analysis/integrity, reachable_from: TestGeneratedHandEdits_InputsUnchanged}
- [x] `fathom review --since` and `review_diff` carry an `integrity` dimension; without a base it says so {id: CV-12, symbol: DimIntegrity, package: internal/analysis/review, reachable_from: TestReview_IntegrityDimension}
- [x] `review_diff` accepts an optional `since` base ref {id: CV-13, symbol: reviewDiffWithBase, package: internal/mcp, reachable_from: TestReviewDiff_SinceArgument}
- [x] `fathom verify --since <ref>` renders self-declared rows and fails a self-declared claimed skip {id: CV-14, symbol: cmdVerify, package: cmd/fathom, reachable_from: TestCmdVerify_SinceMarksSelfDeclared}
- [ ] (op) CV-M1: the integrity and self-declaration checks flag both guards-pilot-8 i28 cells and the i36 self-checks {id: CV-M1, skip: "measurement: replay of the recorded cells"}
- [ ] (op) CV-M2: at most 10% of the last 50 commits that touched guarded tests flag after rename normalization, else the check stays advisory {id: CV-M2, skip: "measurement: 50-commit replay"}
- [ ] (op) CV-M3: kill the integrity check if it catches nothing in 30 days of real sessions and the i28 cells {id: CV-M3, skip: "operator measurement: 30-day window"}

## B. Stage 2 — the conservation ledger

- [x] The capability inventory extracts api, json, route, env, rpc, cli and configured-pattern keys from file contents {id: CV-15, symbol: Extract, package: internal/analysis/inventory, reachable_from: TestExtract_EveryKeyKind}
- [x] The inventory delta lists removed, changed and added keys {id: CV-16, symbol: Diff, package: internal/analysis/inventory, reachable_from: TestDiff_RemovedChangedAdded}
- [x] `.fathom/config.toml` `[[surface.pattern]]` blocks declare string surfaces {id: CV-17, symbol: SurfacePatterns, package: internal/config, reachable_from: TestConfig_SurfacePatternBlock}
- [x] The conservation run reports every plane's verdict and the inputs it was missing; a plane with a missing input is never conserved (I44) {id: CV-18, symbol: Run, package: internal/conserve, reachable_from: TestRun_PlanesVerdictsAndMissingInputs}
- [x] `fathom conserve --since <ref> [--json] [--strict]` emits `fathom.conserve/v1` {id: CV-19, symbol: cmdConserve, package: cmd/fathom, reachable_from: TestCmdConserve_JSONSchemaV1}
- [x] A removed or changed route that another service calls names the caller {id: CV-20, symbol: ConsumerLosses, package: internal/conserve, reachable_from: TestConsumerLosses_RemovedRouteNamesCaller}
- [x] A certifies declaration at base with no declaration at head and no retirement is lost {id: CV-21, symbol: LostDeclarations, package: internal/certify, reachable_from: TestLostDeclarations_RemovedNotRetired}
- [x] A `behaviour` certificate resolves to an inventory key, and an unknown key contradicts it {id: CV-22, symbol: resolveBehaviour, package: internal/certify, reachable_from: TestResolveTarget_BehaviourInventoryKey}
- [x] A claim whose scope matched files at base and none at head is orphaned {id: CV-23, symbol: GovernanceOrphans, package: internal/conserve, reachable_from: TestGovernanceOrphans_ScopeMatchedAtBaseNoneAtHead}
- [x] `fathom review --since` and `review_diff` carry a `conservation` dimension {id: CV-24, symbol: DimConservation, package: internal/analysis/review, reachable_from: TestReview_ConservationDimension}
- [x] The plan schema accepts `retires`, `surface_change`, `gone`, `baseline` and `tests` {id: CV-25, oracle: lexical, path: schema/plan/schema.cue, pattern: "surface_change"}
- [x] This repository declares its tool and verb surfaces {id: CV-26, oracle: lexical, path: .fathom/config.toml, pattern: "surface.pattern"}
- [ ] (op) CV-M4: the Go surface delta agrees with apidiff at precision and recall of at least 0.95 over 50 release pairs of the calibration repositories {id: CV-M4, skip: "measurement: apidiff agreement run"}

## C. Stage 3 — absence and anti-Blindness

- [x] `gone:` rows prove import, module, pattern and kept-but-unused-symbol absence {id: CV-27, symbol: EvaluateGone, package: internal/verify, reachable_from: TestEvaluateGone_EveryForm}
- [x] A `gone:` row is Unknown, never proven, when the index is not at HEAD, typed packages failed in scope, or build-tag-excluded files reference the target {id: CV-28, symbol: EvaluateGone, package: internal/verify, reachable_from: TestEvaluateGone_UnknownOnStaleOrFailedIndex}
- [x] The lexical oracle evaluates `max`, `min` and `eq` on plan rows {id: CV-29, symbol: lexicalBoundsVerdict, package: internal/verify, reachable_from: TestLexicalOracle_MaxMinEq}
- [x] A call site to a `Deprecated:` symbol that is new in the range is a finding {id: CV-30, symbol: DeprecatedCallers, package: internal/conserve, reachable_from: TestDeprecatedCallers_NewCallSiteInRange}
- [x] A function body hash is insensitive to identifier renames {id: CV-31, symbol: FuncBodyHash, package: internal/analysis/clone, reachable_from: TestFuncBodyHash_IdentifierRenameInsensitive}
- [x] A body deleted in the range that reappears under another name is a resurrection finding {id: CV-32, symbol: Resurrected, package: internal/conserve, reachable_from: TestResurrected_DeletedBodyReappears}
- [ ] (op) CV-M5: seeded copies in replays of fathom's own migrations (ADR-025, ADR-041) are all caught, with no finding on the genuine commits {id: CV-M5, skip: "measurement: Blindness replay"}

## D. Stage 4 — refactors as programs

- [x] `fathom refactor plan <target>` writes a gate-ready plan with a baseline, ratcheted `gone:` rows per package, certified_by rows and a conserve row {id: CV-33, symbol: cmdRefactorPlan, package: cmd/fathom, reachable_from: TestCmdRefactorPlan_GateReadyPlan}
- [x] `fathom refactor status` reports each ratchet against its max and the baseline, and drift that landed on main {id: CV-34, symbol: cmdRefactorStatus, package: cmd/fathom, reachable_from: TestCmdRefactorStatus_RatchetAndDrift}
- [x] Raising a ratchet's `max` inside the range is self-declared {id: CV-35, symbol: AnnotateSelfDeclared, package: internal/verify, reachable_from: TestAnnotateSelfDeclared_RaisedMaxIsSelfDeclared}
- [ ] (op) CV-M6: retiring this repository's unused analysis packages runs as a refactor program and finishes with every row proven and a clean conserve report {id: CV-M6, skip: "measurement: the dogfood program"}

## E. Stage 5 — the typed floor beyond Go

- [x] The language tracker records the shipped file-resolved tier {id: CV-37, oracle: lexical, path: docs/languages.md, pattern: "file-resolved tier shipped"}
- [x] Every conservation report names the edge tier per language touched {id: CV-38, symbol: TiersByLanguage, package: internal/conserve, reachable_from: TestTiersByLanguage}
- [x] `fathom report evidence` gives per-service evidence density and ranks no languages {id: CV-39, symbol: cmdReportEvidence, package: cmd/fathom, reachable_from: TestCmdReportEvidence_NoRanking}
- [ ] (op) CV-M7: precision and recall per language at the scip and file tiers, from the free L0 oracle on calibration clones, published beside Go's {id: CV-M7, skip: "measurement: L0 per-language run"}

## F. Layers and packs

- [x] A citation whose lines moved re-anchors by its anchor text and the claim stays fresh {id: CV-40, symbol: ReanchorCitation, package: internal/semantic, reachable_from: TestReanchorCitation_MovedLinesStayFresh}
- [x] The default compile set includes adr-rules and spec-bindings when their corpus exists {id: CV-41, symbol: DefaultLayersFor, package: internal/semantic, reachable_from: TestDefaultLayersFor_CorpusPresence}
- [ ] (op) CV-M8: the fleet recompile ran for services with no or fully stale layers, dry-run priced, spend recorded {id: CV-M8, skip: "operator run: fleet recompile"}
- [ ] (op) CV-M9: obligations recall is at least 0.75 under the production model on its labeled fixtures {id: CV-M9, skip: "measurement: obligations eval"}
- [ ] (op) CV-M10: the findings layer's eval scores {id: CV-M10, skip: "measurement: findings eval"}

## G. The decision port

- [x] `[decide] sites` gates which sites may send questions off-host {id: CV-42, symbol: SiteAllowed, package: internal/decide, reachable_from: TestSiteAllowed_Allowlist}
- [ ] (op) CV-M11: the calibration check (0.9 right nine times in ten) over logged route decisions and reviewed certificate suggestions, with the proceed band raised for any site that fails {id: CV-M11, skip: "measurement: calibration reading"}

## H. Review Kit, test-kit and the gate

- [ ] (op) CV-43: test-kit's envelope records `env.commit`, `env.dirty`, `env.race`, and its go-test-json adapter fills them (platform-engineering) {id: CV-43, skip: "cross-repo: platform-engineering test-kit"}
- [ ] (op) CV-44: Review Kit's `refactoring-conservation` pack and `fathom-conserve` adapter, armed at warn on this repository (platform-engineering) {id: CV-44, skip: "cross-repo: platform-engineering review-kit"}
- [ ] ADR-043's kill rule counts warn, not deny {id: CV-45, oracle: lexical, path: ADR.md, pattern: "never fires at warn"}
- [ ] The stanza routes refactors to `rename_impact` and `safe_delete_check` first {id: CV-46, oracle: lexical, path: CLAUDE.md, pattern: "refactor/delete: rename_impact"}
- [x] The pre-push hook runs `fathom conserve --since` as a warning {id: CV-47, oracle: lexical, path: lefthook.yml, pattern: "fathom conserve"}

## Deferred (trigger on file)

- [-] A cross-language port ledger: a port is complete when the source inventory is closed in the target and behaviour certificates are live against both {id: CV-50, disposition: deferred, reason: "no named port consumer", trigger: "a fleet service ports to another language (F22)"}
- [-] Decision-port ordering of a very large refactor frontier after the deterministic type-check split {id: CV-51, disposition: deferred, reason: "needs a real frontier and its own A/B", trigger: "a refactor frontier over 100 sites and a corpus to A/B it (F23)"}
- [-] A same-concept question for renames in languages without typed edges {id: CV-52, disposition: deferred, reason: "must follow measured file-tier precision", trigger: "CV-M7 published and a JavaScript or Python rename corpus exists (F23)"}
- [-] Architectural decisions compiled into dependency rules the call graph refutes {id: CV-53, disposition: deferred, reason: "no layer without a consumer (TX-4)", trigger: "adr-rules compiled in two or more repositories and one asks for enforcement (F24)"}
- [-] Data, schema and infrastructure refactors: store formats, SQL DDL, Terraform moved blocks, Kubernetes apiVersion moves {id: CV-54, disposition: deferred, reason: "different conservation law", trigger: "a named consumer's migration (F25)"}
- [-] Security, performance and concurrency conservation: authorization chains on routes, bench certificates, race-attested runs {id: CV-55, disposition: deferred, reason: "facts recorded (env.race) but no consumer", trigger: "a regression of that kind is observed (F26)"}
