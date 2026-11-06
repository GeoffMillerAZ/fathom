# LL-M6 preregistration: the unknown rate of resource reads

Committed 2026-11-05, before the scorer exists and before any subject has been read by the reader. Row LL-M6 in `docs/plans/lang-library.md`, design
`docs/design/lang-library.md` section 10. The reading is `docs/bench/readings/lang-library/m6-unknown-rate.md`. Nothing here may change after the
first scoring run without a new dated section that says what changed and why.

## What was and was not looked at

- The rules, the subject rule and the subjects were written by a model, not a human.
- Subjects were found by listing candidate repositories (below) and counting, with `grep` on the source text, the resource blocks whose type some
  rule names. No reader output, on any subject, informed a rule, a subject or a tag.
- Path spellings (`versioning_configuration[*].status`, `Tags[*].Key`, `rules[*].id`) follow the design's section 5 and the reader's own test
  fixtures, which use made-up templates and not the subjects.
- The CloudFormation and Pulumi YAML subject files are chosen by name. Whether a file reads as a template is decided by `iac.ReadFile` (a
  `*NoReaderError` drops the subject and the reading says so).

## The 20 rules

`rules.json` is the list. Each rule is an attribute check: a resource type and an attribute path, spelled once per tool (Terraform, CloudFormation,
Pulumi YAML). A rule matches a resource whose type is in the binding's `types`. Terraform matches managed resources only: data sources and module
calls never match. No rule inspects a class, an expression or a value: a rule names where to look, and only the scorer, afterwards, says what the
reader found there.

| Id | Check | Terraform type and path |
|---|---|---|
| R01 | S3 bucket versioning status | `aws_s3_bucket_versioning` `versioning_configuration[*].status` |
| R02 | S3 block public ACLs | `aws_s3_bucket_public_access_block` `block_public_acls` |
| R03 | S3 default encryption algorithm | `aws_s3_bucket_server_side_encryption_configuration` `rule[*].apply_server_side_encryption_by_default[*].sse_algorithm` |
| R04 | S3 bucket canned ACL | `aws_s3_bucket_acl` `acl` |
| R05 | Security group ingress CIDR | `aws_security_group` `ingress[*].cidr_blocks` |
| R06 | DynamoDB point-in-time recovery | `aws_dynamodb_table` `point_in_time_recovery[*].enabled` |
| R07 | Standalone security group rule CIDR | `aws_security_group_rule` `cidr_blocks` |
| R08 | RDS storage encrypted | `aws_db_instance` `storage_encrypted` |
| R09 | RDS publicly accessible | `aws_db_instance` `publicly_accessible` |
| R10 | RDS backup retention period | `aws_db_instance` `backup_retention_period` |
| R11 | EBS volume encrypted | `aws_ebs_volume` `encrypted` |
| R12 | Launch template IMDS token requirement | `aws_launch_template` `metadata_options[*].http_tokens` |
| R13 | EC2 instance type | `aws_instance` `instance_type` |
| R14 | KMS key rotation | `aws_kms_key` `enable_key_rotation` |
| R15 | IAM role trust policy | `aws_iam_role` `assume_role_policy` |
| R16 | IAM managed policy document | `aws_iam_policy` `policy` |
| R17 | CloudTrail multi-region | `aws_cloudtrail` `is_multi_region_trail` |
| R18 | SQS queue KMS key | `aws_sqs_queue` `kms_master_key_id` |
| R19 | SNS topic KMS key | `aws_sns_topic` `kms_master_key_id` |
| R20 | Lambda runtime | `aws_lambda_function` `runtime` |

The CloudFormation and Pulumi YAML spellings are in `rules.json`.

## The subject rule

A subject is a Terraform directory, a CloudFormation template file or a Pulumi YAML `Pulumi.yaml`, in a public repository, at a pinned commit.

1. **Pool.** Every directory or file of the candidate repositories. Terraform: all directories with `.tf` files, excluding any path with an
   `examples`, `example`, `wrappers` or `test*` segment or a `.terraform` directory. CloudFormation: `.yaml`, `.yml`, `.json` and `.template` files,
   excluding paths with `test*`, `example` or `examples` segments and a `.json` file that has a `.yaml` or `.yml` twin of the same name. Pulumi:
   every `Pulumi.yaml`.
2. **Count.** Terraform: the `resource "T" "N"` blocks in the directory whose `T` is a type of some rule. CloudFormation: the `Type:` lines of rule
   types in the file. Pulumi: the `type: aws:` lines in the file. By text, not by reader.
3. **Kind tag, from the path and the repository name only.** Terraform `reusable module`: a directory with a `modules` or `terraform-modules`
   segment, or the root of a repository named `terraform-aws-*`; every other directory is `deployed root`. CloudFormation `reusable module`: a
   template in `widdix/aws-cf-templates` or in an `aws-quickstart/*` or `aws-ia/*` repository (parameterised building blocks and Quick Start
   stacks that others deploy with their own values); every other template is `deployed root`. Pulumi: `pulumi/templates` is `reusable module` (a
   starting point copied and configured), `pulumi/examples` is `deployed root`.
4. **Pick.** Per tool and kind, rank the pool by count (descending), then by path in byte order. Take the top 5 (Pulumi: top 3) with at most 2 per
   repository, and for CloudFormation skip a file whose name without extension repeats one already taken.
5. **Pin.** The repository's default-branch commit on 2026-11-05, in `subjects.tsv`.

Candidate repositories. Terraform: `terraform-aws-modules/terraform-aws-{s3-bucket,security-group,kms,dynamodb-table,sqs,rds,ec2-instance,lambda}`,
`cloudposse/terraform-aws-cloudtrail`, `nozaq/terraform-aws-secure-baseline`, `bridgecrewio/terragoat`,
`hashicorp/learn-terraform-provision-eks-cluster`, `hashicorp/learn-terraform-lambda-api-gateway`, `brikis98/terraform-up-and-running-code`,
`futurice/terraform-examples`, `terraform-in-action/manning-code`, `ManagedKube/kubernetes-ops`,
`aws-samples/aws-security-reference-architecture-examples`. CloudFormation: `widdix/aws-cf-templates`,
`aws-cloudformation/aws-cloudformation-templates`, `aws-cloudformation/aws-cloudformation-samples`,
`aws-samples/aws-security-reference-architecture-examples`, `aws-samples/aws-modern-application-workshop`, `stelligent/cfn_nag` (every file is
under a `test*` path, so none was eligible), `aws-quickstart/quickstart-{aws-vpc,linux-bastion,atlassian-jira,amazon-aurora-postgresql,microsoft-rdgateway}`,
`aws-ia/cfn-ps-hashicorp-{consul,vault}`. Pulumi: `pulumi/templates`, `pulumi/examples`. The candidates are repositories one model could name, so
the pool is not a sample of all public infrastructure code, and the reading says so.

`calibration/iac/unknownrate/fetch.sh DEST` checks the subjects out at their pins (sparse, one directory each).

## Scoring

The scorer (`calibration/iac/unknownrate`) reads each subject with `pkg/lang/iac` and no options: Terraform with `ReadModule` on the directory
(one directory, not recursive), CloudFormation and Pulumi YAML with `ReadFile`. It never calls `WithValues`. A read that returns an error fails
that subject, which is reported and left out of the rates.

**Unit.** (rule, matching resource). A rule with no matching resource in a subject is n/a, and the reading prints the count of n/a pairs apart.

**Class of a pair.** Let P be the rule's path. Take every attribute of the resource that is

- at P, or below it (its path starts with `P[*]` or `P.`), or
- an ancestor of P (P starts with its path followed by `[*]` or `.`) or the root attribute with an empty path, with class `expression` or `unknown`.

The pair's class is the first that holds, in order: `unknown` if any such attribute is `expression` or `unknown`; `defaulted` if any is
`defaulted`; `reference` if any is `reference`; `literal` otherwise. A pair with no such attribute is `unset`: the reader found the attribute
absent, which a check answers by comparing with a contract. `unset` is not unknown. Multiplicity (`count`, `for_each`, a condition) does not
enter the class.

**Rates.** The unknown rate is `unknown / all pairs`, all pairs being the scored pairs of a tool, `unset` included. Reported apart, never merged:
the `defaulted` share, the `unset` share, the rate over pairs that are not `unset`, and the rate with `defaulted` counted as unknown. Every rate is
split by kind (reusable module, deployed root), by rule, and by subject, pooled over pairs.

**Verdict.** `LL-M6 verdict: PASS` when the Terraform unknown rate is under 30% and the CloudFormation unknown rate is under 30%, each pooled over
all of its subjects and rules. Otherwise `FAIL`, naming which tool. Either way the reading is committed. Per the design a miss does not remove the
door: the guidance then says source checks do not answer where the rate is high. Pulumi YAML is reported on the same terms with no verdict.

**Prediction, from design section 10, recorded before any reading.** Deployed roots read under the bar, CloudFormation reads under the bar, and
reusable modules whose values all come from variables read over it. The reading compares each.

## The 50-pair audit

The scorer prints every scored pair of Terraform and CloudFormation. The audit takes, per tool, the 25 pairs that sort first by the lowercase hex
SHA-256 of `ll-m6-audit-v1|RULE|SUBJECT|FILE|TYPE|NAME|STARTLINE` (the rule id, subject id, resource span file, resource type, name and start line
of its span), ascending; a tool with fewer than 25 pairs is audited whole. For each pair the auditor reads the source the pair points at, and any
variable default or local it names, and records `agree` when the scored class is the class the design's section 5 table gives what the source says
(`literal` a constant, `defaulted` only through a variable default or a local chain ending in one, `reference` another resource, data source,
module output or parameter, `expression` computed, `unknown` unreadable) and `unset` when the attribute is not in the source, otherwise `disagree`
with a note.

The auditor is a model reading the source. No human made this audit, and the reading says so. It checks the scorer's class against the source
text, not against any other tool's reading.
