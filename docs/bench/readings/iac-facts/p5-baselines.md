IF-P5 verdict: PASS

Question: on one seeded fixture set, what do `tofu validate`, `tflint` with its AWS ruleset, Checkov and the Terraform registry's MCP server each catch, and what is left for fathom? The row has no numeric bar, so the verdict reads the design's prediction (section 10): facts 1 to 4 are already covered by native tools, and facts 5 to 7 are not. The prediction held. Where the predictions were wrong, the misses are listed below.

## Plain-language summary

- Facts 1 to 4 (attribute exists, required, sensitive, deprecated): `tofu validate` caught 23 of 23 seeded cases and raised no false alarm on 8 clean controls. Once the provider is initialised, fathom adds no detection here. Its only edge is working before `init` (the provider is about 700 MB per AWS version in the plugin cache).
- `tflint` with the AWS ruleset caught 7 of 10 seeded value errors, of which `tofu validate` caught 4. Together they caught 9 of 10. This is where a linter does a fact well, so a value or enum check stops being a selling point for fathom.
- Checkov caught only the 3 seeded policy cases. It raised 2 to 10 findings on each of the 8 clean controls (44 in all), none of them validity errors.
- Fact 5, replacement: no baseline tool states it. `validate`, `tflint` and Checkov gave no signal on any of 10 edited pairs. The registry docs, served by the Terraform MCP server, state "Forces new resource" for 3 of the 7 attributes that do force replacement, and for none of the 3 that update in place.
- Fact 6, cascade: no baseline tool or doc offers it. It was not testable as a signal, only as an absence.
- Fact 7, upgrade impact: `tofu validate` catches all 4 seeded removed arguments, but only after initialising the new provider version. Under the old version it passes, with a deprecation warning for 3 of 4. Across the AWS schema, 79 arguments, nested ones included, disappeared between 5.100.0 and 6.67.0 in surviving resources, and 19 of them (24%) were marked deprecated in 5.100.0. The other 60 give no warning before the bump.
- What is left for fathom: facts 5 and 6 entirely, the 76% of removals that were never deprecated (fact 7), and any answer that must work before `init`.

## Method

Subject: AWS only. Azure and Google were skipped: each needs its own fixture set and a provider download of similar size, and the owner's priority is AWS and CloudFormation.

Pins: OpenTofu 1.13.0; `hashicorp/aws` 6.67.0 (the pin in `calibration/oracles/terraform-provider-schema/`) for every case, and 5.100.0 for the 4 upgrade cases; `tflint` 0.65.0 with `tflint-ruleset-aws` 0.50.0 and the bundled terraform ruleset 0.15.0 on its recommended preset; Checkov 3.4.3, terraform framework, `--skip-download`; `terraform-mcp-server` 1.4.1 built from its tag, driven over stdio, which served `hashicorp/aws` 6.72.0 docs (five patch releases newer than the fixtures). The OpenTofu registry's MCP server was not run: it is an npm package and this repository has no Node toolchain. It serves the same provider documentation sources, so the docs result is expected to carry over, but that is not measured.

Fixtures: 58 single-resource Terraform directories with fake credentials and no network use after `init`, plus 10 base and edit pairs. One defect per case, written before any tool ran, with predictions recorded first.

| Class | Cases | What is seeded |
|---|---|---|
| validity (V01 to V15) | 15 | unknown argument (5), unknown block, missing required argument or block (4), wrong type (3), unknown attribute in a reference, undeclared resource, undeclared variable |
| deprecated (D01 to D05) | 5 | `aws_s3_bucket` `acl`, `policy`, `request_payer`; `aws_iam_role` `managed_policy_arns`; `aws_cloudwatch_event_rule` `is_enabled` |
| sensitive (S01 to S03) | 3 | an output that exposes `aws_db_instance.password`, `aws_iam_access_key.secret`, a secret version's `secret_string` |
| value (E01 to E10) | 10 | invalid instance, database and cache type; previous-generation instance type; unsupported Lambda runtime; route with no target and with two; invalid canned ACL; malformed IAM JSON; unknown IAM action |
| policy (K01 to K03) | 3 | SSH open to the world, unencrypted database, public bucket ACL |
| upgrade (U01 to U04) | 4 | `aws_eip.vpc`, `aws_instance.cpu_core_count`, `aws_api_gateway_deployment.stage_name`, `aws_batch_compute_environment.compute_environment_name`, all valid on 5.100.0 |
| replacement (R01 to R10) | 10 pairs | 7 attributes that force replacement and 3 that update in place |
| clean controls (C01 to C08) | 8 | valid, representative configurations |

Replacement labels come from `ForceNew: true` in the provider source at tag v6.67.0 (`ec2_instance.go`, `vpc_subnet.go`, `s3/bucket.go`, `lambda/function.go`, `iam/role.go`, `rds/instance.go`, `dynamodb/table.go`), read by a regular expression over each attribute's schema block and spot-checked by eye. They are approximate labels for a baseline, not an oracle. IF-P3 and IF-M2 own replacement truth.

A tool "catches" a case when it emits a diagnostic that addresses the seeded defect: any diagnostic for `validate` and `tflint` (each case has one defect, and controls show no stray output), and the matching check id for Checkov (CKV_AWS_24, CKV_AWS_16, CKV_AWS_20).

## Results

| Class | Cases | `tofu validate` | `tflint` | Checkov | Any of the three |
|---|---|---|---|---|---|
| validity | 15 | 15 | 0 | 0 | 15 |
| deprecated | 5 | 5 (warnings) | 0 | 0 | 5 |
| sensitive | 3 | 3 | 0 | 0 | 3 |
| value | 10 | 4 | 7 | 0 | 9 |
| policy | 3 | 0 | 0 | 3 | 3 |
| upgrade, on 6.67.0 | 4 | 4 | 0 | 0 | 4 |
| upgrade, on 5.100.0 | 4 | 0 (3 deprecation warnings) | not run | not run | 0 |
| replacement | 10 | 0 | 0 | 0 | 0 |
| clean controls, false alarms | 8 | 0 | 0 | 8 controls, 44 findings | not applicable |

Details that matter:

- Value errors: `validate` caught the Lambda runtime, the route with no target, the invalid canned ACL and the malformed IAM JSON. `tflint` caught both instance types, the database and cache types, the Lambda runtime, and both route cases. Only E10 (an unknown IAM action inside valid JSON) escaped all three.
- `tflint` flags the previous-generation instance type (E03) as a warning, which is a style finding, not a validity error.
- Checkov findings on the 8 clean controls were 7, 6, 5, 9, 10, 2, 2 and 3. They are policy checks (encryption, logging, tagging). A fathom answer that cites Checkov's output would inherit this volume.
- Registry docs, 6.72.0, by the cases above:
  - Deprecated: the argument line is marked `Deprecated` for 5 of 5.
  - Required: marked for 5 of 6 attributes tested (`policy_arn` had no line in the expected format, so it is unread, not absent).
  - Sensitive: the docs for `aws_db_instance` and `aws_iam_access_key` never use the word, 0 of 2.
  - Unknown argument: the 3 tested names are absent, so an agent can infer the error, but it must fetch the page and compare.
  - Replacement: "Forces new resource" appears for `aws_s3_bucket.bucket`, `aws_iam_role.name` and `aws_dynamodb_table.hash_key`, and is missing for `aws_instance.availability_zone`, `aws_subnet.cidr_block`, `aws_lambda_function.function_name` and `aws_db_instance.storage_encrypted`, which do force replacement. Recall 3 of 7, no false positive on the 3 in-place attributes.
  - Upgrade: the version 6 upgrade guide names all 4 seeded arguments. Across the 79 removed arguments, a loose check (the resource name and the argument name both appear anywhere in the guide) finds at most 25.
- The MCP server refused calls after about a dozen in quick succession: `rate limit exceeded: too many requests from this session`. A docs lookup per resource in a review loop would hit it.
- Fact 7 over the whole schema, 5.100.0 to 6.67.0: 79 arguments removed from resources that remain, 19 of them marked deprecated in 5.100.0; 20 resources removed, all 20 marked deprecated in 5.100.0.
- Cost, warm plugin cache: `tofu init` 1.6 s and `validate` 2.3 s per directory, `tflint` 0.3 s. The AWS plugin cache for two versions is 1.4 GB.

## Where the predictions were wrong

Recorded before running: `validate` would not catch sensitive outputs (it did, 3 of 3), would catch none of the value cases except by chance (it caught 4 of 10), and `tflint` would catch some but not E05 (it caught E05 and E06). Checkov did what was predicted. These misses make the design's native-oracle column stronger, not weaker.

## Fixture defects, disclosed

Three fixtures (C08, K01, V09) used a security group name starting with `sg-`, which the AWS provider rejects. They were corrected and the three cases re-run before this table was read. No other fixture or prediction changed after a result was seen.

## Limits

One provider, 68 directories, one defect each, labels by hand and regular expression. Checkov was run on the terraform framework only (its separate secrets framework flagged one high-entropy string in S01 and was not graded). The previous design's lesson applies: these tools see one version of one configuration. They cannot say what a change does to a resource that already exists, which is the claim facts 5 to 7 make.

## What the next stage must know

- IF-M1 should treat `tofu validate` as the oracle for facts 1 to 4 and expect fathom to match, not beat, it. The value is working without `init`, which IF-P1 prices in bytes.
- Value and enum checks are `tflint`'s. Do not claim them.
- Replacement and upgrade impact are the only facts without a baseline. The registry docs state replacement for under half of the attributes tested, so a fact 5 that reads the provider source adds recall the docs lack. Whether it adds precision over the docs is IF-M2's question.
