# IaC provider schemas: review gate before implementation

Request (2026-10-26): "let's add the major terraform providers and pulumi providers
and crossplane providers if applicable ... the providers for aws, azure and gcp, no?"

Spec: [iac-provider-schemas](../design/iac-provider-schemas.md), issue #55,
I-R0/I-R1/I-R4 and GAP1-32 through GAP1-37. Terraform is T3, target L3
under amended I-R1. All rows stay unchecked. These are proposed work contracts, not
permission to implement. Controller review and an explicit free language-lane slot
are prerequisites for all source, reading, queue and existing-plan changes.

## Design and calibration contracts

- [ ] Document the local schema contract, representation ceiling and native-oracle requirement in the proposal {id: IAC-1, oracle: lexical, path: docs/design/iac-provider-schemas.md, pattern: "^## Resolution and honest persistence$", eq: 1}
- [ ] Record a reproducible three-provider calibration assessment with pinned archives/exports, per-subject counts and store bytes per provider; grade native show references and validate diagnostics separately in the binding-oracle disposition {id: IAC-2, oracle: lexical, path: docs/assessments/iac-provider-schema-calibration.md, pattern: "^## Native binding oracle disposition$", eq: 1, needs: IAC-1}
- [ ] Expand Kustomize cross-reference evidence so both precision and recall denominators reach 20, preserving the declared subjects and exclusions {id: IAC-3, oracle: lexical, path: docs/bench/readings/lang/kustomize.json, pattern: "(?s)\"counts\": \{[^}]*\"xref_fathom\": ([2-9][0-9]|[1-9][0-9]{2,})[,\n][^}]*\"xref_truth\": ([2-9][0-9]|[1-9][0-9]{2,})[,\n]", needs: IAC-1}
- [ ] Record a Pulumi SDK resolution assessment that separates native compiler bindings from imported external Fathom edges for all three clouds {id: IAC-4, oracle: lexical, path: docs/assessments/2026-10-26-pulumi-probe.md, pattern: "(?s)^## Verdict$.*^## Imported external edges$", eq: 1, needs: IAC-1}

Lexical contracts above prove document/reading facts only. Acceptance additionally
requires the design's commands, raw oracle truth and per-subject receipts; a
heading is not runtime proof. IAC-17 is a desired measured outcome, not permission
to tune a corpus or override a ceiling. Leave it open if the expanded reading
fails. Local I-R0 probes do not close these durable-delivery contracts.

## Controller acceptance and expanded evidence

- [ ] Record controller acceptance of the reviewed design and release of the I-R0 implementation slot {id: IAC-15, oracle: lexical, path: docs/design/iac-provider-schemas.md, pattern: "^Accepted: controller approved implementation with I-R0 slot released\.$", eq: 1, needs: IAC-1}
- [ ] Record the accepted grading-policy amendment admitting Terraform native provider-schema evidence at L4 {id: IAC-16, oracle: lexical, path: ADR.md, pattern: "Terraform native provider-schema typed evidence is admitted at L4", eq: 1, needs: IAC-15}
- [ ] Grade expanded Kustomize evidence excellent at L3 only after the two denominator floors are met {id: IAC-17, oracle: lexical, path: docs/bench/readings/lang/kustomize.json, pattern: "(?s)\"L3\": \"excellent\".*\"counts\": \{[^}]*\"xref_fathom\": ([2-9][0-9]|[1-9][0-9]{2,})[,\n][^}]*\"xref_truth\": ([2-9][0-9]|[1-9][0-9]{2,})[,\n]", needs: "IAC-3, IAC-18"}
- [ ] Record per-subject overlay/component/generator/patch coverage and native build-oracle limitations, including unevidenced components {id: IAC-18, oracle: lexical, path: docs/assessments/2026-10-26-kustomize-d19-evidence.md, pattern: "(?s)^## Verdict$.*^## Per-subject native evidence and exclusions$", eq: 1, needs: IAC-3}

`needs` controls work eligibility, not verification. Controller review is the
sole authority gate for I-R0 and policy acceptance; lexical checks only record
the resulting evidence. IAC-17 therefore repeats its denominator condition in
the grade contract itself. IAC-10 stays deferred until that policy is accepted.

Only the controller records the Accepted line in the design after that
ruling. A lexical acceptance contract records authority; it does not grant it.
The exact amendment wording is a proposed proof marker for owner review, not an
ADR change this lane is authorized to make. IAC-3 checks both denominators within
one counts object; any subject reaching the floor implies the pooled floor,
and the aggregate object covers a pool whose individual subjects are thinner.

## Implementation contracts, held by I-R0 and design review

The four implementation symbols below are intentionally new, prescribed by this
design, and absent on the review base. Each must be on the product path with its
named asserting regression test. Absence fails symbol_present until implemented.
Do not bind the row to an older caller or introduce a test-only wrapper.

- [ ] Add the two-hop module-output resolver and prove sibling-file resolution, ambiguity, registry-source and escape behavior {id: IAC-5, symbol: resolveTerraformModuleOutputs, certified_by: TestTerraformModuleOutputAcrossSiblingFiles, strength: asserts, package: internal/fusion, needs: "IAC-6, IAC-15", ctx: internal/fusion/docxref_terraform.go internal/fusion/terraform_xref_test.go}
- [ ] Emit unresolved module-output intents and carry the local child directory while keeping the existing module-source bounds under the two-hop path {id: IAC-6, symbol: terraformModuleOutputIntents, certified_by: TestTerraformModuleSourceBounds, strength: asserts, package: internal/extractor, needs: IAC-15, ctx: internal/extractor/hclextract.go}
- [ ] Add the bounded schema-export reader and validate the explicit binding without persisting declarations or running a provider {id: IAC-7, symbol: ReadTerraformSchemaExport, certified_by: TestTerraformSchemaExportReaderRefusesWholeInput, strength: asserts, package: internal/depplane, needs: "IAC-2, IAC-15, IAC-16", ctx: docs/design/iac-provider-schemas.md}
- [ ] Resolve configuration references to declarations already admitted by IAC-12; own the configuration-reference edges and counted refusals, not schema declaration persistence {id: IAC-8, symbol: EnrichTerraformSchemaRefs, certified_by: TestTerraformSchemaPassRefusesWholeInput, strength: asserts, package: internal/fusion, needs: "IAC-7, IAC-12, IAC-16, system-graph.md#SG-37", ctx: docs/design/iac-provider-schemas.md internal/fusion/ingest.go}
- [ ] Regrade Terraform L3 excellent on the same pinned subjects with native xref_truth at least 866 for EKS, 2662 for VPC and 3528 in the top-level aggregate {id: IAC-9, oracle: lexical, path: docs/bench/readings/lang/terraform.json, pattern: "(?s)\"L3\": \"excellent\".*\"name\": \"terraform-aws-modules/terraform-aws-eks\"[^{}]*\"sha\": \"323c633629b8e4306d94eb1392d7400a80143621\"[^{}]*\"counts\": \{[^}]*\"xref_truth\": (86[6-9]|8[7-9][0-9]|9[0-9]{2}|[1-9][0-9]{3,})[,\n].*\"name\": \"terraform-aws-modules/terraform-aws-vpc\"[^{}]*\"sha\": \"b3abd6df2ecf052451a361ed55b8f06f8742a795\"[^{}]*\"counts\": \{[^}]*\"xref_truth\": (266[2-9]|26[7-9][0-9]|2[7-9][0-9]{2}|[3-9][0-9]{3}|[1-9][0-9]{4,})[,\n].*\n  \"counts\": \{[^}]*\"xref_truth\": (352[89]|35[3-9][0-9]|3[6-9][0-9]{2}|[4-9][0-9]{3}|[1-9][0-9]{4,})[,\n]", needs: "IAC-5, IAC-6"}
- [-] Publish a provider-native typed reading {id: IAC-10, disposition: deferred, reason: "ADR-048 bars tooling L4; native show references and validate diagnostics are candidates not yet graded for field binding", trigger: "controller accepts the Terraform L4 policy and the per-candidate native binding grades then rewrites this work contract"}
- [ ] Record ENGINE and EPOCH impact over every committed doc-xref reading, nested readings included, and add the extractor epoch entry and reconcile all moved readings before controller board regeneration {id: IAC-11, oracle: lexical, path: docs/assessments/iac-docxref-impact.md, pattern: "^## Regraded reading deltas$", eq: 1, needs: "IAC-5, IAC-6, IAC-9"}

The four named functions and tests are planned, not implemented. Each ordinary
regression certificate declares asserts strength, not port conformance. Existing
source bounds alone do not close IAC-6: the new intent carrier must use them on
the two-hop path. Controller policy acceptance is a manual authority gate in
addition to eligibility. No schema declaration persists before that acceptance
and SG-37. IAC-12 alone owns declaration admission/persistence and its anchor
contract; IAC-8 owns references from configuration into those admitted targets.
It stays ineligible while IAC-12 is deferred. IAC-11 covers the independent
two-hop ENGINE/EPOCH regrade, without waiting for the schema pass. The schema
implementation must later supply its own impact inventory before integration.
IAC-9 also fixes both native subject SHAs and each denominator floor.
If the representation or oracle fails,
record a named-candidate ceiling, revise the still-open outcome rows with the
controller, and keep IAC-10 deferred. No table expansion is authorized here.

## Remaining issue #55 deliverables

- [ ] Calibrate Crossplane with pinned AWS/Azure/GCP provider CRDs, validate composed resources and directional fromFieldPath evidence, and record the remaining ceiling (GAP1-34) {id: IAC-19, oracle: lexical, path: docs/assessments/2026-10-26-crossplane-provider-crds.md, pattern: "(?s)^## Verdict$.*^## Remaining ceiling$", eq: 1, needs: IAC-1}
- [ ] Triage Terragrunt, HCL tool configs and AWS CDK using their concrete native candidates, recording tried, refused or needs-owner-approval for each and a Verdict per format (GAP1-37) {id: IAC-20, oracle: lexical, path: docs/assessments/2026-10-26-iac-oracle-first-triage.md, pattern: "(?s)^## Terragrunt$.*^## Verdict: Terragrunt$.*^## HCL tool configs$.*^## Verdict: HCL tool configs$.*^## AWS CDK$.*^## Verdict: AWS CDK$", eq: 1, needs: IAC-1}

IAC-19's assessment records source/version/digests and native evidence without
committing provider CRDs. The controller's GAP1-34 row separately proves that
the crossplane reading names those provider inputs. IAC-20 must test or dispose
of terragrunt stack generate/native block dumps, offline cdk synth with installed
dependencies, and Packer's required plugins; restating old ceilings is not triage.

## Deferred scope

- [-] Admit and persist the referenced schema declaration closure through the dependency adapter for both in-root and out-of-root exports {id: IAC-12, disposition: deferred, reason: "SG-37 and a Terraform adapter need accepted L4 policy plus anchor and per-ecosystem byte/row budgets; no per-service schema copy is authorized", trigger: "controller accepts L4 policy and the dependency-plane adapter with usable anchors and budgets; an interim duplicate representation requires a separate explicit cost acceptance"}
- [-] Evaluate Crossplane composition functions {id: IAC-13, disposition: deferred, reason: "function runtimes require containers outside this lane's allowed probes", trigger: "owner authorizes a separate runtime-capable calibration environment"}
- [-] Add a Pulumi-specific provider-schema pass {id: IAC-14, disposition: deferred, reason: "first distinguish ordinary installed-SDK resolution from the generic external-target gap", trigger: "IAC-4 proves a provider-specific gap ordinary language support cannot cover or SG-37 lands and the gap persists"}

## Proof and handoff

X-R5: committed receipts, readings, assessments and data contain no subject
person identities or contributor metadata. Retain counts and hashes. Before
READY inspect the two new files for at-signs, Author and name/email shapes, then
run X-R2 and X-R3. No private probe result becomes durable proof before its
reproduction and sanitized assessment land through the corresponding row.


Before the controller claims a row, refresh a local index and run:

```sh
fathom verify --service fathom --plan docs/plans/iac-provider-schemas.md
```

Never measure through the shared daemon. This checkout is not enrolled as a
service, so no verification success is claimed here. A later local scratch-store
verification must target this exact commit. The controller owns box flips, full
merged-head gates, board regeneration and all pushes. I-R0 permits this new plan
and design only. No existing plan, reading, queue or engine file changes with it.
