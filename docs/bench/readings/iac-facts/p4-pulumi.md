IF-P4 verdict: PARTIAL. Pulumi's own schema carries replacement facts for SDKv2-bridged resources (AWS, GCP) and for Azure Native, so those need no source scan. It carries none for AWS plugin-framework resources (0 of 487) and none for Kubernetes. The AWS token to Terraform type map is exact and one to one, and it lives in the bridge metadata, not in the schema. Design section 8's branch "if they carry it, Pulumi's own schema serves" holds for SDKv2 and Azure Native only.

## Summary

`pulumi package get-schema` was run for four packages with `PULUMI_BACKEND_URL=file://<scratch>`, `PULUMI_HOME` in scratch and no login. A replacement fact appears as `willReplaceOnChanges: true` on an input property. `replaceOnChanges` appears on 10 Azure Native properties and nowhere else.

| Package | Version | Resources | Top-level inputs | With `willReplaceOnChanges` | Resources with at least one | Nested type properties with it |
|---|---|---|---|---|---|---|
| `aws` (bridged) | 7.53.0 | 1,760 | 12,946 | 3,271 (25.3%) | 1,129 | 1,451 of 29,001 |
| `gcp` (bridged) | 10.2.0 | 1,495 | 11,651 | 6,124 (52.6%) | 1,488 | 4,681 of 29,744 |
| `azure-native` (generated) | 3.30.0 | 22,969 | 208,486 | 78,859 (37.8%) | 22,955 | 1,354 of 786,535 |
| `kubernetes` (generated) | 4.34.3 | 560 | 2,510 | 0 | 0 | 0 of 10,581 |

## Where a bridged token maps to its Terraform type

Not in the schema. Resource entries hold `description`, `properties`, `inputProperties`, `required` and `stateInputs`. The Terraform name appears only inside description prose. The map is `auto-aliasing.resources.<terraform type>.current` in each package's `bridge-metadata.json`, committed at `provider/cmd/pulumi-resource-<name>/bridge-metadata.json` in the Pulumi provider repository at the release tag (also embedded in the plugin binary). It is a source for the catalog builder, never for the product at query time.

- `aws` 7.53.0: 1,747 Terraform types, 1,747 distinct tokens, all 1,747 present in the schema, so no token is shared and the lookup is one to one. 13 schema tokens have no Terraform type: hand-written resources such as the S3 `...V2` aliases and `lambda/callbackFunction`. All 1,728 resources of Terraform `aws` 6.67.0 are in the metadata. The other 19 metadata entries are newer.
- `gcp` 10.2.0: 1,495 types, 1,495 tokens, all in the schema, 0 unmapped. 1,375 of the 1,381 resources in Terraform `google` 8.8.1 are in the metadata. 120 metadata entries are not in 8.8.1.
- Property names: camelCase of the Terraform name. No `name` override appeared in the `aws` metadata fields.

## Agreement with the Terraform source scan (IF-P3), `aws`, top-level configurable inputs

Pulumi `aws` 7.53.0 sits on a Terraform `aws` commit after the 6.71.0 prepare merge, four minor versions past the 6.67.0 scan, so drift is possible. Resources with no scan answer are left out.

| Terraform source | Pulumi true | Pulumi false |
|---|---|---|
| SDKv2, `ForceNew` unconditional | 3,066 | 4 |
| SDKv2, conditional (diff suppress or `CustomizeDiff`) | 70 | 73 |
| SDKv2, not `ForceNew` | 0 | 4,146 |
| Framework, `RequiresReplace` | 0 | 926 |
| Framework, not replacing | 0 | 1,026 |

- SDKv2: Pulumi marks 3,066 of 3,070 unconditional Terraform facts (99.9%) and marks nothing the scan calls plain non-replacing. It flattens conditional cases to unconditional, 70 of 143, so a Pulumi fact is an upper bound where the Terraform side has a diff suppress or `CustomizeDiff`.
- Framework: 0 of 487 framework resources have any `willReplaceOnChanges`. 926 attributes that Terraform replaces on are invisible in the Pulumi schema. Pulumi `aws` cannot serve fact 5 alone. Its tokens must map to the Terraform catalog, which the exact metadata map allows.

## Method, pins, limits

- Schemas: `pulumi package get-schema <name>` at the versions above (scratch `PULUMI_HOME`, so the plugin download stayed in scratch). Metadata: raw `bridge-metadata.json` at tags v7.53.0 (`aws`) and v10.2.0 (`gcp`). Terraform side: the IF-P3 scan output for `aws` 6.67.0.
- Counts are over each resource's top-level `inputProperties` and over every property in the `types` map. A shared type is counted once.
- Not measured: GCP and Azure Native correctness against `replace_paths`, which is IF-M2's job. No Pulumi preview or program ran. The Terraform source version behind `gcp` 10.2.0 was not resolved.
- Provider archives, schemas and metadata stay out of git.
- Kubernetes: the schema is generated from the OpenAPI document and states no immutability. IF-M4's server-side dry run stays the oracle for it.
