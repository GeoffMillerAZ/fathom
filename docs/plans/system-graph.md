# The system graph and the dependency plane: the plan (vision 38, ADR-050)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/system-graph.md` proves it.
Design: `docs/design/system-graph.md` (section numbers below are its). Goals G17 (SG-M1..M3),
G18 (SG-M4, SG-M5) and G25's catalog (SG-M6). Invariants I56 and I57 rise from `stated` as
SG-36 and SG-16 land.

Already built and not re-claimed here: the route extractor (`routes.Extract`), the Go HTTP
client extractor and the heuristic `service_ref` join (`internal/analysis/serviceref`,
VB-30), and the inventory keys `route:`, `rpc:`, `env:`, `cli:` (`internal/analysis/inventory`,
ADR-044). Rows below extend them; none re-implements them.

Lane rule (ADR-049 decision 2): no row that adds a table, a cache or a background job starts
before `responsible-tenant.md#RT-M1` reads PASS.

## Wave 2: the core (§3 to §7)

- [x] The contract key grammar: `ParseContractKey` accepts the eleven kinds, normalizes each body (§3), refuses anything else by name, and round-trips ADR-044's four kinds byte-identically {id: SG-1, symbol: ParseContractKey, package: internal/syslink, reachable_from: TestParseContractKeyKindsAndNormalization}
- [ ] Store tables `contract_keys` and `system_links`, additive to store format v0 with their indexes (§6); `TestStoreFormatV0_RequiredSchema` still passes {id: SG-2, symbol: PutContractKeys, package: internal/store, reachable_from: TestContractKeysRoundTrip, needs: "responsible-tenant.md#RT-M1"}
- [x] The `ContractSource` port and registry by dialect id (§5); a source records extractor id and epoch on every row {id: SG-3, symbol: RegisterContractSource, package: internal/syslink, reachable_from: TestContractSourceRegistry}
- [x] Route providers: `routes.Extract` results become `route:` provides rows with node ids and tiers {id: SG-4, symbol: routeProvides, package: internal/syslink, reachable_from: TestRouteProvidesFromExtractor, needs: "SG-1,SG-3"}
- [x] Go HTTP client consumers: `ExtractClientCalls` results become `route:` consumes rows at `literal` or `template` tier {id: SG-5, symbol: goClientConsumes, package: internal/syslink, reachable_from: TestGoClientConsumesTiers, needs: "SG-1,SG-3"}
- [x] HTTP client consumers for JS/TS (fetch, axios, ky) and Python (requests, httpx, aiohttp) as dialect queries (§5) {id: SG-6, symbol: scriptClientConsumes, package: internal/syslink, reachable_from: TestScriptClientConsumes, needs: "SG-3"}
- [x] OpenAPI 3.x documents provide `route:` keys at `spec` tier, bound by `operationId` (§5) {id: SG-7, symbol: openAPIProvides, package: internal/syslink, reachable_from: TestOpenAPIProvidesSpecTier, needs: "SG-3"}
- [x] Proto service definitions provide `rpc:` keys at `spec` tier; Go gRPC stub calls consume them at `typed` tier {id: SG-8, symbol: protoRPCKeys, package: internal/syslink, reachable_from: TestProtoProvidesAndStubConsumes, needs: "SG-3"}
- [x] The link join: provides/consumes matched by key, tier = the weaker end, evidence at both ends (§4) {id: SG-9, symbol: MatchLinks, package: internal/syslink, reachable_from: TestMatchLinksTierIsWeakerEnd, needs: "SG-2"}
- [ ] Incremental link refresh after an ingest, only for keys that ingest changed; a deleted provider closes its links {id: SG-10, symbol: RefreshLinksForKeys, package: internal/syslink, reachable_from: TestRefreshLinksOnlyChangedKeys, needs: "SG-9"}
- [ ] `fathom links` verb with `--json` and the `links` MCP tool in the `system` group (§7) {id: SG-11, symbol: cmdLinks, package: cmd/fathom, reachable_from: TestCmdLinksJSON, needs: "SG-9"}
- [ ] `fathom system-impact` verb and the `system_impact` MCP tool: local walk, links, consumer callers, grouped by kind and tier (§7) {id: SG-12, symbol: SystemImpact, package: internal/syslink, reachable_from: TestSystemImpactCrossesServices, needs: "SG-9"}
- [ ] `cross_repo_blast_radius` retired once `system_impact` lands (law 4: tool count flat) {id: SG-13, gone: pattern "Name:\s+.cross_repo_blast_radius.", paths: "internal/mcp/", needs: "SG-12"}
- [x] Fauxtown gains the fixture fleet (an OpenAPI-first service, a gRPC service with Go and TS clients, seeded negatives) and `links.answer.json` (§12) {id: SG-14, oracle: lexical, path: internal/connector/testdata/fauxtown/links.answer.json, pattern: "kind.: .route"}
- [ ] `indexbench link-oracle`: ingest the fleet, compare `system_links` to the answer key, write precision and recall per kind and tier {id: SG-15, symbol: cmdLinkOracle, package: cmd/indexbench, reachable_from: TestCmdLinkOracle, needs: "SG-14"}
- [x] I57: a `literal` or `template` link never passes or fails a verify row, a conservation plane or a review verdict {id: SG-16, symbol: linkGates, package: internal/syslink, reachable_from: TestHeuristicLinksNeverGate, needs: "SG-9"}

## Wave 3: more kinds, drift, co-change, dependencies, the catalog

- [ ] `gql:` keys from GraphQL SDL and resolver maps (provides) and operation documents (consumes) {id: SG-17, symbol: graphQLKeys, package: internal/syslink, reachable_from: TestGraphQLKeys, needs: "SG-3"}
- [ ] `topic:` keys from AsyncAPI, Avro subjects and producer/consumer calls with literal names; broker kind as an attribute {id: SG-18, symbol: topicKeys, package: internal/syslink, reachable_from: TestTopicKeys, needs: "SG-3"}
- [ ] `table:` keys from SQL migrations and ORM models (provides) and SQL literals and ORM queries (consumes), with `access: read|write` {id: SG-19, symbol: tableKeys, package: internal/syslink, reachable_from: TestTableKeysReadWrite, needs: "SG-3"}
- [ ] `env:` and `config:` keys from k8s, compose, Helm values, `.env` examples and flag files (provides) and getenv/config/flag lookups (consumes) {id: SG-20, symbol: configKeys, package: internal/syslink, reachable_from: TestConfigKeysProvideConsume, needs: "SG-3"}
- [ ] `image:` and `deploy:` keys from Dockerfiles, Deployments, Services, Ingress hosts and compose services {id: SG-21, symbol: deployKeys, package: internal/syslink, reachable_from: TestDeployKeys, needs: "SG-3"}
- [ ] Route extractors for Spring, ASP.NET, Rails, Django, FastAPI, Express, Fastify and Nest, and their HTTP client consumers, as dialect data where the framework allows {id: SG-22, symbol: frameworkRouteKeys, package: internal/syslink, reachable_from: TestFrameworkRouteKeys, needs: "SG-4"}
- [ ] `contract_drift`: a producer's contract against consumers' usage, per kind, with the closed breaking-change list (§7) {id: SG-23, symbol: ContractDrift, package: internal/syslink, reachable_from: TestContractDriftBreakingChanges, needs: "SG-9"}
- [ ] `fathom contract-drift --base --head` verb and the `contract_drift` MCP tool; SARIF through the shared emitter {id: SG-24, symbol: cmdContractDrift, package: cmd/fathom, reachable_from: TestCmdContractDriftSARIF, needs: "SG-23"}
- [ ] `dep_drift` retired once `contract_drift` lands (law 4) {id: SG-25, gone: pattern "Name:\s+.dep_drift.", paths: "internal/mcp/", needs: "SG-24"}
- [ ] The co-change miner: bounded (2,000 commits per service, streamed through `gitro`), measurement-only, never an input to the verbs (§9) {id: SG-26, symbol: MineCoChange, package: internal/indexbench, reachable_from: TestMineCoChangeBounded}
- [ ] `blast_radius` and `system_impact` graded against co-change on this repository and the fixture fleet {id: SG-27, oracle: lexical, path: docs/bench/readings/system-graph/cochange.md, pattern: "^Co-change agreement", needs: "SG-26,SG-12"}
- [ ] The `DepGraph` port (§8.1): `DepGraph(ecosystem, root) -> nodes, edges, lockDigests` {id: SG-30, symbol: RegisterDepGraph, package: internal/depplane, reachable_from: TestDepGraphPort}
- [ ] In-tree lockfile adapter, Go: `go.mod` and `go.sum` {id: SG-31, symbol: goLockGraph, package: internal/depplane/lockfile, reachable_from: TestGoLockGraph, needs: "SG-30"}
- [ ] In-tree lockfile adapter, npm: `package-lock.json`, `pnpm-lock.yaml`, `yarn.lock` {id: SG-32, symbol: npmLockGraph, package: internal/depplane/lockfile, reachable_from: TestNPMLockGraph, needs: "SG-30"}
- [ ] In-tree lockfile adapter, PyPI: `uv.lock`, `poetry.lock`, pinned requirements {id: SG-33, symbol: pypiLockGraph, package: internal/depplane/lockfile, reachable_from: TestPyPILockGraph, needs: "SG-30"}
- [ ] In-tree lockfile adapter, Maven and Gradle (declared-only tier without a lock) {id: SG-34, symbol: mavenLockGraph, package: internal/depplane/lockfile, reachable_from: TestMavenLockGraph, needs: "SG-30"}
- [ ] In-tree lockfile adapter, Cargo: `Cargo.lock` {id: SG-35, symbol: cargoLockGraph, package: internal/depplane/lockfile, reachable_from: TestCargoLockGraph, needs: "SG-30"}
- [ ] The cache locator: resolve each locked package to a toolchain cache directory read-only; absent packages are `refused` with the reason; no network dial anywhere in the plane (I56) {id: SG-36, symbol: LocateCached, package: internal/depplane, reachable_from: TestDepPlaneNeverDials, needs: "SG-30"}
- [ ] The dependency API indexer: public surface only, content-addressed by package digest, once per machine, 8 MiB source cap, background ingest class, `pending` answers on a request (§8.3) {id: SG-37, symbol: IndexPackageAPI, package: internal/depplane, reachable_from: TestIndexPackageAPIOncePerDigest, needs: "SG-36,SG-2"}
- [ ] Usage map: our imports and typed references become `pkg:` consumes rows; `links` answers per package {id: SG-38, symbol: pkgConsumes, package: internal/syslink, reachable_from: TestPkgConsumesFromImports, needs: "SG-37"}
- [ ] Upgrade impact: `contract_drift {pkg, from, to}` diffs two indexed versions and joins removed or re-signatured symbols to our consumes {id: SG-39, symbol: UpgradeImpact, package: internal/syslink, reachable_from: TestUpgradeImpactSeededCorpus, needs: "SG-38,SG-23"}
- [ ] Dependency index eviction by `fathom gc` after 30 days unused, and the 256 MiB on-disk ceiling reported {id: SG-40, symbol: evictDepIndexes, package: internal/depplane, reachable_from: TestEvictDepIndexes, needs: "SG-37"}
- [ ] The service catalog view: owners, APIs, dependencies, data, config, certificates and docs per service, each field's source named (§11, U10) {id: SG-43, symbol: BuildServiceCatalog, package: internal/console, reachable_from: TestServiceCatalogFieldsAndSources, needs: "SG-9"}
- [ ] The catalog on the console's project workspace {id: SG-44, symbol: renderServiceCatalog, package: cmd/fathom, reachable_from: TestConsoleServiceCatalogPage, needs: "SG-43"}
- [ ] `fathom catalog export --service S --out FILE` writes a catalog-info shape only when asked {id: SG-45, symbol: cmdCatalogExport, package: cmd/fathom, reachable_from: TestCmdCatalogExportOnRequest, needs: "SG-43"}

## Wave 4: live updates and vuln reach

- [ ] A file watcher refreshes keys and links for changed files under the budget (§10) {id: SG-28, symbol: watchContractKeys, package: internal/syslink, reachable_from: TestWatchRefreshesChangedFiles, needs: "SG-10"}
- [ ] The incremental-equals-full check: after N incremental updates a full re-derivation is byte-identical, else the watcher disables itself with a named reason {id: SG-29, symbol: checkIncrementalEqualsFull, package: internal/syslink, reachable_from: TestIncrementalEqualsFull, needs: "SG-28"}
- [ ] OSV reader for a user-supplied file only; advisories mapped to `pkg:` keys {id: SG-41, symbol: LoadOSVFile, package: internal/depplane, reachable_from: TestLoadOSVFileNoNetwork, needs: "SG-38"}
- [ ] Vuln reach: `system_impact` walked backwards from an advisory's symbol to our entrypoints {id: SG-42, symbol: VulnReach, package: internal/syslink, reachable_from: TestVulnReachToEntrypoints, needs: "SG-41,SG-12"}

## Refused and deferred (recorded so they are not re-proposed)

- [-] A tool per edge kind (`routes`, `topics`, `tables` …) {id: SG-X1, disposition: refused, reason: "law 4; one link model and three verbs (ADR-050 decision 3)"}
- [-] The decision port in link matching {id: SG-X2, disposition: deferred, reason: "the route site failed calibration 45.7% at confidence at least 0.9 (2026-09-24)", trigger: "an adjudicated sample of at least 100 template candidates reads nine in ten at the shown confidence"}
- [-] Downloading packages, advisories or docs to fill a gap {id: SG-X3, disposition: refused, reason: "I56; egress is an admission cost the operator never granted"}
- [-] Building the depgraph engine in fathom {id: SG-X4, disposition: refused, reason: "vision 15 §3: ci-kit owns the engine; fathom consumes it through the DepGraph port"}
- [-] NuGet API indexing {id: SG-X5, disposition: deferred, reason: "reference assemblies need a metadata reader", trigger: "a fleet service pins NuGet packages and SG-M4 passes for the five G18 ecosystems"}

## Measurements (op)

- [ ] (op) SG-M1: the fixture fleet read by `indexbench link-oracle`: route, rpc, gql, topic, table and env links at precision at least 0.95 and recall at least 0.85 for the spec and typed tiers, per kind {id: SG-M1, oracle: lexical, path: docs/bench/readings/system-graph/links.md, pattern: "^SG-M1 verdict: PASS", needs: "SG-15,SG-17,SG-18,SG-19,SG-20"}
- [ ] (op) SG-M2: `contract_drift` on 40 seeded breaking changes catches at least 90%, with at most 1 false breaking per 20 non-breaking diffs {id: SG-M2, oracle: lexical, path: docs/bench/readings/system-graph/drift.md, pattern: "^SG-M2 verdict: PASS", needs: "SG-23"}
- [ ] (op) SG-M3: `system_impact` on the home fleet with every link's tier named, compared with co-change {id: SG-M3, oracle: lexical, path: docs/bench/readings/system-graph/home-fleet.md, pattern: "^SG-M3 verdict: PASS", needs: "SG-12,SG-27"}
- [ ] (op) SG-M4: upgrade impact at precision at least 0.9 on the seeded-upgrade corpus for Go, npm, PyPI, Maven and Cargo {id: SG-M4, oracle: lexical, path: docs/bench/readings/system-graph/upgrade.md, pattern: "^SG-M4 verdict: PASS", needs: "SG-39"}
- [ ] (op) SG-M5: one dependency index per package version fleet-wide, link refresh under 10% of ingest wall time, `phys_footprint` within budget after a fleet ingest, zero network dials {id: SG-M5, oracle: lexical, path: docs/bench/readings/system-graph/tenancy.md, pattern: "^SG-M5 verdict: PASS", needs: "SG-37,SG-10"}
- [ ] (op) SG-M6: the service catalog for every fleet service, each field's source named {id: SG-M6, oracle: lexical, path: docs/bench/readings/system-graph/catalog.md, pattern: "^SG-M6 verdict: PASS", needs: "SG-44"}
