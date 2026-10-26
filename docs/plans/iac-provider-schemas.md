# IaC provider schemas: review gate before implementation

Request (2026-10-26): "let's add the major terraform providers and pulumi providers
and crossplane providers if applicable ... the providers for aws, azure and gcp, no?"

Spec: [iac-provider-schemas](../design/iac-provider-schemas.md), issue #55,
I-R0/I-R1/I-R4. All rows stay unchecked. These are proposed work contracts, not
permission to implement. Controller review and an explicit free language-lane slot
are prerequisites for all source, reading, queue and existing-plan changes.

## Design and calibration contracts

- [ ] Document the local schema contract, representation ceiling and native-oracle requirement in the proposal {id: IAC-1, oracle: lexical, path: docs/design/iac-provider-schemas.md, pattern: "^## Resolution and honest persistence$", eq: 1}
- [ ] Record a reproducible three-provider calibration assessment with pinned archives/exports, per-subject counts and an explicit native binding-oracle disposition; no L4 claim from schema loading {id: IAC-2, oracle: lexical, path: docs/assessments/iac-provider-schema-calibration.md, pattern: "^## Native binding oracle disposition$", eq: 1, needs: IAC-1}
- [ ] Expand Kustomize cross-reference evidence so both precision and recall denominators reach 20, preserving the declared subjects and exclusions {id: IAC-3, oracle: lexical, path: docs/bench/readings/lang/kustomize.json, pattern: "(?s)\"counts\": \{[^}]*\"xref_fathom\": ([2-9][0-9]|[1-9][0-9]{2,})[,\n][^}]*\"xref_truth\": ([2-9][0-9]|[1-9][0-9]{2,})[,\n]", needs: IAC-1}
- [ ] Record a Pulumi SDK resolution assessment that separates native compiler bindings from imported external Fathom edges for all three clouds {id: IAC-4, oracle: lexical, path: docs/assessments/iac-pulumi-sdk-probe.md, pattern: "^## Imported external edges$", eq: 1, needs: IAC-1}

Lexical contracts above prove document/reading facts only. Acceptance additionally
requires the design's commands, raw oracle truth and per-subject receipts; a
heading is not runtime proof. IAC-17 is a desired measured outcome, not permission
to tune a corpus or override a ceiling. Leave it open if the expanded reading
fails. Local I-R0 probes do not close these durable-delivery contracts.

## Controller acceptance and expanded evidence

- [ ] Record controller acceptance of the reviewed design and release of the I-R0 implementation slot {id: IAC-15, oracle: lexical, path: docs/assessments/iac-provider-schema-calibration.md, pattern: "^Controller disposition: accepted for implementation; I-R0 slot released\.$", eq: 1, needs: IAC-1}
- [ ] Record the accepted grading-policy amendment admitting Terraform native provider-schema evidence at L4 {id: IAC-16, oracle: lexical, path: ADR.md, pattern: "Terraform native provider-schema typed evidence is admitted at L4", eq: 1, needs: IAC-15}
- [ ] Grade expanded Kustomize evidence excellent at L3 only after the two denominator floors are met {id: IAC-17, oracle: lexical, path: docs/bench/readings/lang/kustomize.json, pattern: "(?s)\"L3\": \"excellent\".*\"counts\": \{[^}]*\"xref_fathom\": ([2-9][0-9]|[1-9][0-9]{2,})[,\n][^}]*\"xref_truth\": ([2-9][0-9]|[1-9][0-9]{2,})[,\n]", needs: "IAC-3, IAC-18"}
- [ ] Record per-subject overlay/component/generator/patch coverage and native build-oracle limitations, including unevidenced components {id: IAC-18, oracle: lexical, path: docs/assessments/iac-kustomize-expanded-evidence.md, pattern: "^## Per-subject native evidence and exclusions$", eq: 1, needs: IAC-3}

`needs` controls work eligibility, not verification. Controller review is the
sole authority gate for I-R0 and policy acceptance; lexical checks only record
the resulting evidence. IAC-17 therefore repeats its denominator condition in
the grade contract itself. IAC-10 stays deferred until that policy is accepted.

Only the controller records the acceptance and slot-release line after that
ruling. A lexical acceptance contract records authority; it does not grant it.
The exact amendment wording is a proposed proof marker for owner review, not an
ADR change this lane is authorized to make. IAC-3 checks both denominators within
one counts object; any subject reaching the floor implies the pooled floor,
and the aggregate object covers a pool whose individual subjects are thinner.

## Implementation contracts, held by I-R0 and design review

The symbols below exist on the design base and are product-path anchors. Extend
and test them rather than introducing test-only wrappers to satisfy a box. If a
review chooses a different integration point, revise the open contract first.

- [ ] Preserve unresolved module-output intent at extraction and resolve it through the unique caller-directory module source and child output; prove sibling-file, ambiguity, remote and escaping cases {id: IAC-5, symbol: ResolveDocXrefs, certified_by: TestTerraformModuleOutputAcrossSiblingFiles, strength: conformance, package: internal/fusion, needs: IAC-15, ctx: internal/extractor/hclextract.go internal/fusion/terraform_xref_test.go}
- [ ] Enforce static repository-contained module sources as the only two-hop candidates, retaining unresolved registry and dynamic sources {id: IAC-6, symbol: localModuleDir, certified_by: TestTerraformModuleSourceBounds, strength: conformance, package: internal/extractor, needs: IAC-5, ctx: internal/extractor/hclextract.go}
- [ ] Integrate admitted schema-source replacement and schema-backed relations into the existing ingest path only after the representation feasibility test passes, with no new table/column/memo/job {id: IAC-7, symbol: ingest, certified_by: TestIngestProviderSchemaRelations, strength: conformance, package: internal/fusion, needs: "IAC-2, IAC-15", ctx: internal/fusion/ingest.go internal/core/model.go}
- [ ] Exercise product ingest with no schema, mismatched bindings, malformed exports, path escapes, cancellation, cap and cap+1; preserve syntax behavior and count every refused typed target {id: IAC-8, symbol: Ingest, certified_by: TestIngestProviderSchemaWholeInputRefusal, strength: conformance, package: internal/fusion, needs: IAC-7, ctx: internal/fusion/ingest.go internal/AGENTS.md}
- [ ] Regrade Terraform L3 against the pinned native oracle after the two-hop fix, without losing any reference population {id: IAC-9, oracle: lexical, path: docs/bench/readings/lang/terraform.json, pattern: "\"L3\": \"excellent\"", needs: "IAC-5, IAC-6"}
- [-] Publish a provider-native typed reading {id: IAC-10, disposition: deferred, reason: "ADR-048 bars tooling L4 and a native binding oracle has not been established", trigger: "controller accepts the Terraform L4 policy amendment and native binding oracle then rewrites this open work contract"}
- [ ] Record ENGINE impact over every committed doc-xref reading, nested readings included, and reconcile all moved readings before controller board regeneration {id: IAC-11, oracle: lexical, path: docs/assessments/iac-docxref-impact.md, pattern: "^## Regraded reading deltas$", eq: 1, needs: "IAC-5, IAC-8, IAC-9"}

IAC-7/IAC-8 must be refined with the accepted schema-pass function and its
certifying regression tests before a completion claim. Existing ingest reach
alone does not prove the new behavior. The four named certifying tests are
planned, not implemented: their certificates must specifically prove these rows
before claiming them. Their absence deliberately blocks a baseline-only pass. If the representation or oracle fails,
record a named-candidate ceiling, revise the still-open outcome rows with the
controller, and keep IAC-10 deferred. No table expansion is authorized here.

## Deferred scope

- [-] Persist typed targets from out-of-root schema exports {id: IAC-12, disposition: deferred, reason: "root-relative anchors and lane table freeze require a separately reviewed dependency representation", trigger: "controller accepts an existing-table representation or the dependency plane lands"}
- [-] Evaluate Crossplane composition functions {id: IAC-13, disposition: deferred, reason: "function runtimes require containers outside this lane's allowed probes", trigger: "owner authorizes a separate runtime-capable calibration environment"}
- [-] Add a Pulumi-specific provider-schema pass {id: IAC-14, disposition: deferred, reason: "first distinguish ordinary installed-SDK resolution from the generic external-target gap", trigger: "IAC-4 demonstrates a provider-specific gap that ordinary language support cannot cover"}

## Proof and handoff

Before the controller claims a row, refresh a local index and run:

```sh
fathom verify --service fathom --plan docs/plans/iac-provider-schemas.md
```

Never measure through the shared daemon. This checkout is not enrolled as a
service, so no verification success is claimed here. A later local scratch-store
verification must target this exact commit. The controller owns box flips, full
merged-head gates, board regeneration and all pushes. I-R0 permits this new plan
and design only. No existing plan, reading, queue or engine file changes with it.
