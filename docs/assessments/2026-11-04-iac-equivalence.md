# IaC equivalence: how far one tool's resource relates to another's

Measured 2026-11-04. Pins: `hashicorp/aws` 6.72.0, `hashicorp/awscc` 1.109.0, OpenTofu 1.13.0, `terraform-provider-aws` source at v6.72.0, CloudFormation registry
schemas for us-east-1, `pulumi-aws` default branch. Scripts and results: `data/2026-11-04-iac-equivalence/`.

## Verdict

Relating two tools' resources is worth having. Converting between them is not fathom's job. The question to ask is whether a fact proven for one tool can answer for another,
and how we would know.

- `awscc` and CloudFormation are the same schema in all but name: 99.2% of names match mechanically and the median attribute match is 1.00.
- Bridged Pulumi and Terraform are the same provider: 1,747 resources map one to one.
- Terraform `aws` and CloudFormation are not close: 47.6% of names match, about 65% with an alias table, and about 35% needs a split or has no counterpart. A flat table cannot
  express that, so any crosswalk entry must be able to say one-to-many and none.
- Replacement facts agree often enough to be a cross-check: 69 of 75 resolved attributes on a small sample. Fathom can use that as a second opinion with no crosswalk at all.

No maintained, property-level `aws_*` to `AWS::*` crosswalk was found. Nor was a measured accuracy for any resource-to-resource translation, by people or by models.

## Measurements

### Counts

| Source | Resources |
|---|---|
| CloudFormation types | 1,876 (zip 3.31 MB, about 18 MB unzipped) |
| Terraform `aws` | 1,758 |
| Terraform `awscc` | 1,722 |

### Names, by a mechanical rule

Rule: lowercase, strip `_`, `::`, `aws_` and `awscc_`, then compare.

| Pair | Match |
|---|---|
| `awscc_*` to `AWS::*` | 1,708 of 1,722 (99.2%) |
| `aws_*` to `AWS::*`, exact | 836 of 1,758 (47.6%) |
| Plus dropping the service prefix, unique matches only | 888 (50.5%) |
| Plus an estimated alias table | about 1,147 (65%, roughly 58 to 72%) |

50 random misses of 922 (seed 20261104), hand-labeled, so these are judgments: 22 no counterpart, 14 fixable by alias, 10 inline in a CloudFormation parent, 3 prefix drops and 1
left unlabeled. Extrapolated: about 400 with no counterpart, about 260 fixable and about 200 inline.

### Attributes, top level only

Share of CloudFormation writable properties with a same-named Terraform attribute, over 835 name-matched pairs.

| Provider | Median | Q1 | Q3 |
|---|---|---|---|
| `aws` | 0.80 | 0.625 | 1.00 |
| `awscc` | 1.00 | 1.00 | 1.00 |

Of 4,378 matched `aws` properties, 1,152 show a coarse type mismatch, nearly all shape: 636 are object against a single-item block and 474 are a `Tags` array against a map.

### Replacement

30 sampled name-matched resources. A script resolved 78 of 147 matched attributes and left 69 unknown. 3 raw disagreements were parser artifacts.

| | Terraform ForceNew | Terraform in-place |
|---|---|---|
| CloudFormation createOnly | 27 | 6 |
| CloudFormation updatable | 0 | 42 |

All 6 real disagreements are createOnly against in-place. Five are `comprehend_entity_recognizer`, whose Terraform update path creates a new model version. The sixth is
`datazone_environment.domain_identifier`. The disagreements look informative rather than random, on too small a sample to say more.

### Splits

53 of 158 prefixed Terraform companions have a same-named inline CloudFormation property, across 30 parents, and S3 alone is 20. The whole-provider total is unknown, because
the check covered 30 of the 98 parents with prefixed companions.

### What the CloudFormation schema carries

| Key | Coverage |
|---|---|
| `createOnlyProperties` | 1,643 of 1,876 types, 4,627 paths |
| `readOnlyProperties` | 1,676 types |
| `writeOnlyProperties` | 518 types |
| `conditionalCreateOnlyProperties` | 49 types |
| `deprecatedProperties` | 49 types |
| `replacementStrategy` | 106 types |
| `default` | 180 of 17,663 top-level properties, so not a reliable source |

So CloudFormation states its replacement fact in the contract, which Terraform's export does not (`2026-11-02-iac-facts-probe.md`).

### Bridged Pulumi

From `pulumi-aws`'s `bridge-metadata.json` and `schema.json`: 1,747 Terraform resources, 0 shared tokens and all 1,747 tokens in the schema. 30 of 38 sampled list fields convert from
snake_case to camelCase, and the other 8 look like pluralization, which was not confirmed. A `default` field appears on 15 of 12,946 inputs. One worker's run, not rechecked.

## Prior art

Mostly from web research, so "none found" means not found in this pass.

| Tool | Direction | Note |
|---|---|---|
| [cf2tf](https://github.com/DontShaveTheYak/cf2tf) | CloudFormation to Terraform | Maintained, authored per resource, says it is not fully accurate |
| [cdk migrate](https://docs.aws.amazon.com/cdk/v2/guide/migrate.html) | CloudFormation to CDK | Preview, L1 constructs |
| [pulumi-converter-terraform](https://github.com/pulumi/pulumi-converter-terraform) | Terraform to Pulumi | Language level |
| [pulumi-cdk](https://github.com/pulumi/pulumi-cdk) | CloudFormation to `aws-native` | Only 7 hand-written overrides to `aws` |
| [AWS IaC generator](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/generate-IaC.html) | Live account to CloudFormation | Cloud Control types only |
| [Trivy](https://github.com/aquasecurity/trivy/tree/main/pkg/iac) | One state model read by checks | Adapters for Terraform, CloudFormation, ARM and Ansible, none for Pulumi |
| [Pulumi policy on Terraform](https://pulumi.com/tutorials/eval-compliance-terraform) | Policy packs over Terraform stacks | Through the bridge mapping, not for native providers |
| [terraform-provider-awscc](https://github.com/hashicorp/terraform-provider-awscc) | Generated from CloudFormation schemas | Its own README says so |
| [Multi-IaC-Eval](https://arxiv.org/abs/2509.05303) | Model generation across CloudFormation, Terraform and CDK | Tests generation and mutation, not translation |

## Limits

- One run on one provider version, from one worker. Hand labels are judgments.
- Attributes are top level only. Nested properties are unmeasured.
- The replacement script left 47% of attributes unknown, so 92% says nothing about those.
- `aws-native`'s lineage comes from Pulumi's announcement, and its repository page does not state it.

## What it drove

- F97's evidence is amended with these numbers, and its identity table stays unbuilt (`docs/intent/futures/f92-f97.md`).
- ADR-091's amendment brings CloudFormation contracts forward, because the schema already states what Terraform's export lacks.
- Row IF-CM3 reads Terraform and CloudFormation replacement facts against each other as a measurement, with no product surface.
