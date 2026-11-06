LL-M3 verdict: PASS

Terraform resource reads against OpenTofu v1.13.0, tofu show -json -config, on 36 preregistered public roots (368 resources, 2151 reader paths). Path precision 1.0000 (bar 0.99), path recall 1.0000 (bar 0.95), class agreement 0.9986 (bar 0.98).

## Reading

| Item | Value |
|---|---|
| Oracle | OpenTofu v1.13.0, tofu show -json -config |
| Reader | `ReadTerraformModule` (`internal/iacfacts`) |
| Subjects | `calibration/oracles/lang-library/subjects/terraform.json`, committed before any oracle or reader ran on them (selection rule inside) |
| Roots selected, graded, rejected by the oracle | 36, 36, 0 |
| Resources in oracle, in reader | 368, 368 |
| Graded paths: reader, oracle, in both | 2151, 2151, 2151 |
| Graded class leaves, agreeing | 2172, 2169 |
| Reader sources (`internal/iacfacts`) read at | `c567da046`, unchanged in this change |
| sha256 prefix, raw oracle report and reader dump | `17ec9390b49c5c98`, `47870161a178a9de` |
| Reader syntax errors reported | 0 |

## Against the bar

| Metric | Bar | Read | Count |
|---|---|---|---|
| Path precision | at least 0.99 | 1.0000 | 2151 of 2151 reader paths are in the oracle |
| Path recall | at least 0.95 | 1.0000 | 2151 of 2151 oracle paths are in the reader |
| Class agreement | at least 0.98 | 0.9986 | 2169 of 2172 leaves in both agree |

Agreement by reader class:

| Reader class | Graded | Agree |
|---|---|---|
| defaulted | 804 | 804 |
| expression | 921 | 921 |
| literal | 296 | 293 |
| reference | 151 | 151 |

## Subjects

| Repository | Pin | Roots | Paths (reader, oracle) | Leaves graded, agree |
|---|---|---|---|---|
| terraform-aws-modules/terraform-aws-alb | `6c6e48c10d45` | 2 | 147, 147 | 147, 147 |
| terraform-aws-modules/terraform-aws-autoscaling | `2a249e91c798` | 1 | 120, 120 | 120, 120 |
| terraform-aws-modules/terraform-aws-cloudfront | `5b9a0a480220` | 2 | 81, 81 | 81, 81 |
| terraform-aws-modules/terraform-aws-dynamodb-table | `b6cc51576046` | 1 | 134, 134 | 134, 134 |
| terraform-aws-modules/terraform-aws-ec2-instance | `4dfedcdd1f26` | 1 | 192, 192 | 192, 192 |
| terraform-aws-modules/terraform-aws-iam | `da8e6a867393` | 6 | 152, 152 | 168, 168 |
| terraform-aws-modules/terraform-aws-kms | `d25e459bd43d` | 2 | 59, 59 | 59, 59 |
| terraform-aws-modules/terraform-aws-lambda | `e73238631a03` | 2 | 95, 95 | 95, 95 |
| terraform-aws-modules/terraform-aws-rds | `175da043429c` | 5 | 30, 30 | 30, 30 |
| terraform-aws-modules/terraform-aws-s3-bucket | `9afa4fc8c8f1` | 6 | 369, 369 | 374, 374 |
| terraform-aws-modules/terraform-aws-security-group | `b3c1b2e8beff` | 2 | 51, 51 | 51, 51 |
| terraform-aws-modules/terraform-aws-sns | `4538b7e208f5` | 1 | 51, 51 | 51, 51 |
| terraform-aws-modules/terraform-aws-sqs | `a68b4d515f9c` | 1 | 55, 55 | 55, 55 |
| terraform-aws-modules/terraform-aws-vpc | `b3abd6df2ecf` | 4 | 615, 615 | 615, 612 |

## Not graded, counted

| Item | Count |
|---|---|
| Reader attributes inside a `dynamic` block (tofu drops dynamic blocks) | 1990 |
| Reader attributes under `lifecycle` | 38 |
| Reader attributes outside the provider schema (`depends_on`) | 13 |
| Module calls (reader, oracle) | 24, 24 |
| Defaulted leaves through a local (ungraded) | 0 |
| Graded leaves whose oracle references include a local (in the agreement figure) | 158, of which 3 disagree |
| Expression leaves that agree only because tofu folded them to a constant | 2 |

## Disagreement classes

| Class | Count | One example (subject, resource, path) |
|---|---|---|
| class disagreement: reader literal, oracle references | 3 | `terraform-aws-modules/terraform-aws-vpc:examples/ipam managed.aws_vpc_ipam.this operating_regions[*].region_name` |

## Rejected by the oracle

None.
## Notes

- Prediction (design section 10): "LL-M3 reads high on paths, and class agreement misses first on expressions the oracle folds or empties." Paths read exactly 1.0000. The only disagreements are three literals that resolve through a local (`local.region = "..."`), not folds or empties. Two expression leaves agree only because tofu folded them to a constant, and the rule that decides them is a text match on the written expression (`grade_reads.py`, `folded_form`).
- Locals. The design leaves locals ungraded and disclosed. The README of the oracle directory fixed that as: a reader `defaulted` leaf whose oracle references include a `local.` is not graded (0 occurred). The three literal-through-local leaves therefore stay in the figure and count as disagreements. If every leaf whose oracle references include a local were ungraded (158), agreement would read 1.0000 (2014 of 2014). The verdict is PASS either way.
- Dynamic blocks. 1990 of the 4192 reader attributes of managed and data resources (47%) sit inside a `dynamic` block, which tofu drops, so this reading says nothing about them. The design excludes them from the denominators and this reading counts them. A policy check that reads an attribute written inside `dynamic` is outside what this oracle can grade.
- Subjects are 36 roots in 14 repositories from one publisher (`terraform-aws-modules`), mostly reusable modules and some examples, all aws provider. The selection rule requires local module sources and the pinned provider list, so roots that call remote modules are not represented. Module calls are counted (24) and not graded. Override files, `.tofu` files and `.tf.json` files are outside the rule, so those branches of `ReadTerraformModule` are not read here.
- A path is graded once per resource however many blocks write it, and a reader leaf agrees when some occurrence in the oracle agrees.
- Reader defects: none found, no reader change, so there is no before and after.

## Rerun

```
select_subjects.py clone --subjects subjects/terraform.json --clones <clones>
tf_oracle.py --subjects subjects/terraform.json --clones <clones> --work <scratch> --out <oracle.json>   # TF_CLI_CONFIG_FILE names the pinned filesystem mirror
FATHOM_READS_SUBJECTS=subjects/terraform.json FATHOM_READS_CLONES=<clones> FATHOM_READS_OUT=<reader.json> go test ./internal/iacfacts -run TestDumpReads
grade_reads.py --tool terraform --subjects subjects/terraform.json --clones <clones> --oracle <oracle.json> --schema <oracle.json>.schema.json --reader <reader.json> --json <grade.json> --md <reading.md> --row LL-M3
```

The scripts are in `calibration/oracles/lang-library`.
