# IaC facts: provider contracts joined to the repository at its locked version

Grammar v2 (`docs/plans/README.md`). Prefix: IF- (IF-n work, IF-Pn phase 0 probes, IF-Mn readings). Design:
`docs/design/iac-facts.md` (section numbers are its). Decision: ADR-091. Goal: G49. Evidence:
`docs/assessments/2026-11-02-iac-facts-probe.md`.

No store table, column, cache, memo file or background job. The product never runs a provider, `init`, a plan or a registry
client (I56). Readings live under `docs/bench/readings/iac-facts/`, one file per row, each opening with a verdict line. A
row flips only through `fathom verify --service fathom --plan docs/plans/iac-facts.md`. Raw exports, provider archives and
plans stay under `workspace/`.

## Phase 0: what the design assumes ($0)

- [x] (op) Catalog size and lookup cost: AWS, AzureRM, Google, Kubernetes and Helm at their pins, raw, pruned and compressed; a blob-deduplicated catalog over 12 consecutive releases of AWS; one-fact lookup p95 and peak memory from a prototype reader {id: IF-P1, oracle: lexical, path: docs/bench/readings/iac-facts/p1-size.md, pattern: "^IF-P1 verdict:"}
- [x] (op) Registry equality: the OpenTofu and HashiCorp builds of `aws` at one version export equal schemas, or the differences are counted {id: IF-P2, oracle: lexical, path: docs/bench/readings/iac-facts/p2-registries.md, pattern: "^IF-P2 verdict:"}
- [ ] (op) Replacement facts: a `go/ast` scan of each provider's tagged source for SDKv2 `ForceNew` and framework `RequiresReplace`, counting resources with `CustomizeDiff`; whether `tofu plan -refresh=false` with fake credentials and seeded state gives `replace_paths` for AWS, AzureRM and Google with all egress denied {id: IF-P3, oracle: lexical, path: docs/bench/readings/iac-facts/p3-replacement.md, pattern: "^IF-P3 verdict:"}
- [ ] (op) Pulumi schemas under a local backend: which of AWS, Azure Native, GCP and Kubernetes carry replacement facts, and where a bridged token maps to its Terraform type {id: IF-P4, oracle: lexical, path: docs/bench/readings/iac-facts/p4-pulumi.md, pattern: "^IF-P4 verdict:"}
- [ ] (op) Baselines on one seeded fixture set: what `tofu validate`, `tflint` with its cloud rulesets, Checkov and the registries' MCP servers each catch, and what is left for fathom {id: IF-P5, oracle: lexical, path: docs/bench/readings/iac-facts/p5-baselines.md, pattern: "^IF-P5 verdict:"}
- [ ] (op) Lockfile coverage: on a public sample of at least 200 repositories with a committed `.terraform.lock.hcl`, chosen by a rule written before reading, the share of the first five providers' selections a catalog of N versions binds, for N of 12, 50 and every release, against IF-P1's size per N {id: IF-P6, oracle: lexical, path: docs/bench/readings/iac-facts/p6-coverage.md, pattern: "^IF-P6 verdict:"}
- [ ] (op) CloudFormation registry schemas: catalog size, a digest as the version key, regional differences between two regions, and the licence on the schema files, naming whether a catalog may be published {id: IF-P7, oracle: lexical, path: docs/bench/readings/iac-facts/p7-cloudformation.md, pattern: "^IF-P7 verdict:"}
- [ ] (op) ctl: go or no-go on the catalog format, version coverage and facts 5 to 7, from IF-P1 to IF-P6, amending bounds in the design {id: IF-M0, oracle: lexical, path: docs/bench/readings/iac-facts/p0-exit.md, pattern: "^IF-M0 verdict:", needs: "IF-P1,IF-P2,IF-P3,IF-P4,IF-P5,IF-P6"}

## Phase 1: catalogs and facts 1 to 4

- [ ] `ReadIaCCatalog` reads one version index and one blob from a catalog under the design's bounds, refusing the whole input at cap + 1 {id: IF-1, symbol: ReadIaCCatalog, package: internal/iacfacts, needs: IF-M0}
- [ ] `fathom iac add|import|list|remove`; `add` takes a local path only, and `import` builds a catalog from an operator's export {id: IF-2, symbol: cmdIaC, package: cmd/fathom, needs: IF-1}
- [ ] I56 holds in code: no product path of this package dials, proven by a test that fails on a dial {id: IF-3, symbol: TestIaCFactsNeverDials, package: internal/iacfacts, skip_test: true, skip: "the test is the anchor: it fails on any dial from the package", needs: IF-1}
- [ ] `cmd/iaccatalog` builds a deterministic catalog from a pin list and receipts it: two builds are byte-identical {id: IF-4, symbol: buildCatalog, package: cmd/iaccatalog, needs: IF-M0}
- [ ] `calibration/iac/pins.yaml` pins the first five providers by version and archive sha256 {id: IF-5, oracle: lexical, path: calibration/iac/pins.yaml, pattern: "hashicorp/aws", needs: IF-M0}
- [ ] A CI workflow publishes catalogs as release assets with attestations, no secrets beyond the release token {id: IF-6, oracle: lexical, path: .github/workflows/iac-catalogs.yml, pattern: "attest", needs: IF-4}
- [ ] `ResolveIaCBinding` answers locked, constraint-matched, ambiguous or unbound for a block, through the lockfile, `required_providers`, aliases and module mappings, with the reusable-module-plus-examples fixture {id: IF-7, symbol: ResolveIaCBinding, package: internal/iacfacts, needs: IF-1}
- [ ] `CheckAttributeUse` walks a block's attributes and static traversals against a schema: unknown, unset required, deprecated, sensitive {id: IF-8, symbol: CheckAttributeUse, package: internal/iacfacts, needs: IF-7}
- [ ] `symbol_context` on a Terraform resource carries the `iac` block {id: IF-9, symbol: symbolContextIaCFacts, package: internal/mcp, needs: IF-8}
- [ ] `review_diff` carries the attribute findings under its byte cap {id: IF-10, symbol: reviewIaCFindings, package: internal/mcp, needs: IF-8}
- [ ] Licence audit of each provider at its pin, naming what a catalog may carry {id: IF-11, oracle: lexical, path: docs/assessments/2026-11-02-iac-catalog-licences.md, pattern: "^## Verdict", needs: IF-5}
- [ ] Docs: the use-case doc, `docs/configuration.md` and `docs/usage.md` name `fathom iac`, and a change note ships {id: IF-12, oracle: lexical, path: docs/use-cases/iac-facts.md, pattern: "^## Maintained", needs: IF-2}
- [ ] (op) Facts 1 to 4 against `tofu validate -json` on public subjects chosen before reading, with seeded errors and a truth population independent of the extractor {id: IF-M1, oracle: lexical, path: docs/bench/readings/iac-facts/m1-validity.md, pattern: "^IF-M1 verdict:", needs: "IF-9,IF-10"}

## Phase 2: replacement and upgrades

- [ ] `extractReplacementFacts` scans a provider's tagged source and records each fact's provenance and any `CustomizeDiff` {id: IF-13, symbol: extractReplacementFacts, package: cmd/iaccatalog, needs: IF-M0}
- [ ] `review_diff` flags a changed attribute that forces replacement, with the static-floor disclosure {id: IF-14, symbol: reviewIaCReplacement, package: internal/mcp, needs: "IF-10,IF-13"}
- [ ] `ReplacementCascade` joins fact 5 to the repository's reference edges and re-parses the bounded referring set {id: IF-15, symbol: ReplacementCascade, package: internal/iacfacts, needs: "IF-8,IF-13"}
- [ ] `blast_radius` on an IaC change answers the cascade, tagged as prediction {id: IF-16, symbol: blastRadiusIaCCascade, package: internal/mcp, needs: IF-15}
- [ ] `IaCUpgradeImpact` diffs two catalog versions and joins the difference to usage; `review_diff` carries it when `.terraform.lock.hcl` changes {id: IF-17, symbol: IaCUpgradeImpact, package: internal/iacfacts, needs: IF-8}
- [ ] (op) Facts 5 and 6 against `replace_paths` and plan `resource_changes` on sampled pairs per provider {id: IF-M2, oracle: lexical, path: docs/bench/readings/iac-facts/m2-replacement.md, pattern: "^IF-M2 verdict:", needs: "IF-14,IF-16"}
- [ ] (op) Fact 7 on mined public provider-bump commits and `tofu validate` before and after {id: IF-M3, oracle: lexical, path: docs/bench/readings/iac-facts/m3-upgrade.md, pattern: "^IF-M3 verdict:", needs: IF-17}

## Phase 3: Kubernetes

- [ ] `buildKubernetesCatalog` reads upstream `api/openapi-spec` per minor version, with API lifecycle from the difference between minors {id: IF-18, symbol: buildKubernetesCatalog, package: cmd/iaccatalog, needs: IF-M0}
- [ ] `fathom iac import` takes an operator's CRD files {id: IF-19, symbol: importCRDs, package: internal/iacfacts, needs: IF-2}
- [ ] `CheckManifestAgainstAPI` checks a manifest's fields and API lifecycle, with the version from `kubeVersion` or the `[iac] kubernetes` key {id: IF-20, symbol: CheckManifestAgainstAPI, package: internal/iacfacts, needs: "IF-18,IF-8"}
- [ ] (op) Kubernetes facts against `kubectl apply --dry-run=server --validate=strict` on `kind`, with kubeconform as a cross-check {id: IF-M4, oracle: lexical, path: docs/bench/readings/iac-facts/m4-kubernetes.md, pattern: "^IF-M4 verdict:", needs: IF-20}

## Phase 4: Pulumi

- [ ] `buildPulumiCatalog` reads `pulumi package get-schema` under a local backend, keyed by token {id: IF-21, symbol: buildPulumiCatalog, package: cmd/iaccatalog, needs: IF-M0}
- [ ] `PulumiResourceUses` reads Pulumi YAML resources, then TypeScript and Python constructor arguments from an SDK import, into the same attribute check {id: IF-22, symbol: PulumiResourceUses, package: internal/iacfacts, needs: "IF-21,IF-8"}
- [ ] (op) Pulumi arguments against the SDK's own compiler {id: IF-M5, oracle: lexical, path: docs/bench/readings/iac-facts/m5-pulumi.md, pattern: "^IF-M5 verdict:", needs: IF-22}

## Phase 2C: CloudFormation (ADR-091 amendment)

- [ ] `buildCloudFormationCatalog` reads AWS's public registry schema file into a catalog keyed by source, region and digest {id: IF-C1, symbol: buildCloudFormationCatalog, package: cmd/iaccatalog, needs: "IF-P7,IF-4"}
- [ ] `CheckCloudFormationUse` walks a resource's `Properties` against a schema: unknown, missing required, read-only set and deprecated, with binding `unversioned` and transform types `unbound` {id: IF-C2, symbol: CheckCloudFormationUse, package: internal/iacfacts, needs: "IF-1,lang-library.md#LL-32"}
- [ ] `symbol_context` on a CloudFormation resource carries the `iac` block {id: IF-C3, symbol: symbolContextCFNFacts, package: internal/mcp, needs: "IF-C2,IF-9"}
- [ ] `review_diff` flags a changed create-only property as forcing replacement, a conditional one as may replace, and carries attribute findings {id: IF-C4, symbol: reviewCFNFindings, package: internal/mcp, needs: "IF-C2,IF-10"}
- [ ] `iac.OpenCatalog` and `Contract` lookups are the public door to catalogs, reached through `fathom-lang contract --catalog PATH` {id: IF-C5, symbol: OpenCatalog, package: pkg/lang/iac, needs: "IF-1,lang-library.md#LL-34"}
- [ ] (op) CloudFormation syntax understanding at L3 excellent or a written ceiling: fix the 25 missed Node handlers (O721) in the JavaScript dialect, re-measure, and either a tier-A cross-file oracle lifts it or the reading records why none exists {id: IF-C6, oracle: lexical, path: docs/bench/readings/iac-facts/c6-cloudformation-l3.md, pattern: "^IF-C6 verdict:"}
- [-] SAM and other transform types (`AWS::Serverless::*`) carry contract facts {id: IF-C7, disposition: deferred, reason: "transform types are not in the registry schemas, and SAM's own schema is a second source to pin and grade", trigger: "IF-CM1 reads PASS and a measured share of public templates uses a transform"}
- [ ] (op) CloudFormation attribute findings against cfn-lint on public templates with seeded errors: precision at least 0.99, recall at least 0.95 {id: IF-CM1, oracle: lexical, path: docs/bench/readings/iac-facts/cm1-cloudformation-validity.md, pattern: "^IF-CM1 verdict:", needs: "IF-C3,IF-C4"}
- [ ] (op) CloudFormation replacement facts against the separately published resource specification's `UpdateType`, a consistency check of the reader that grades nothing, since a probe read 99.93% agreement across 1,872 types and the two share a source {id: IF-CM2, oracle: lexical, path: docs/bench/readings/iac-facts/cm2-cloudformation-replacement.md, pattern: "^IF-CM2 verdict:", needs: IF-C1}
- [ ] (op) Terraform and CloudFormation replacement facts against each other on name-matched pairs, the only independent read of fact 5 for CloudFormation offline: agreement reported per side and disagreements named, with a preregistered floor of 0.85 agreement before fact 5 reads as cross-checked in the answer, and no crosswalk surface {id: IF-CM3, oracle: lexical, path: docs/bench/readings/iac-facts/cm3-cross-check.md, pattern: "^IF-CM3 verdict:", needs: "IF-13,IF-C1"}

## Phase 5: does it help an agent

- [ ] ctl: prereg for IF-M6: IaC edit tasks from public repositories, hidden `validate` and plan graders, arms, the primary (failed validates and unintended replacements), MEI and kill {id: IF-23, oracle: lexical, path: docs/bench/prereg/iac-facts.md, pattern: "^## Kill", needs: "IF-M1,IF-M2"}
- [-] (op) ctl: IF-M6, the agent run {id: IF-M6, disposition: deferred, reason: "spends agent budget and needs the bench harness past its dry runs", trigger: "BX-M1 reads PASS, IF-23 is signed and the owner approves the envelope"}
