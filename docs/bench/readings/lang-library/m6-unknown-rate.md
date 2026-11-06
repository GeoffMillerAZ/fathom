LL-M6 verdict: PASS

The unknown rate of resource reads is 10.0% on Terraform (9 of 90 pairs) and 6.1% on CloudFormation (8 of 132 pairs), both under the 30% bar. The 95% Wilson intervals are about 5% to 18% and 3% to 12%, so the bar holds at the upper end too. The design predicted deployed roots and CloudFormation under the bar, which held, and reusable modules over it, which did not: reusable Terraform modules read 10.3% unknown. Pulumi YAML reads 16.7% (3 of 18), reported apart with no verdict.

## What this is, and is not

- A preregistered reading (`calibration/iac/unknownrate/PREREGISTRATION.md`, committed before the scorer existed and before the reader had read any subject): 20 rules, a subject rule, 26 pinned subjects (5 reusable and 5 deployed per tool for Terraform and CloudFormation, 3 and 3 for Pulumi YAML), the scoring definitions, the bars and the audit draw. Nothing was changed after the first scoring run.
- The rules, subjects and tags were chosen by a model, by text counts on the source, from a candidate list one model could name. The pool is not a sample of public infrastructure code. Every subject is a public repository at a pinned commit.
- The unit is (rule, matching resource). Terraform data sources and module calls never match. A (rule, subject) with no matching resource is n/a: 156 of 200 for Terraform, 147 of 200 for CloudFormation. Three Terraform rules (R06, R12, R17) and four CloudFormation rules (R11, R13, R16, R17) had no matching resource anywhere, so the rate rests on 17 and 16 rules.
- Reads use `pkg/lang/iac` with no options: `ReadModule` for a Terraform directory, `ReadFile` for a template. No read failed, and every subject read with 0 syntax errors and 0 dropped entries.
- The scorer is `calibration/iac/unknownrate` (`go run ./calibration/iac/unknownrate -dest DIR -mode md|json|audit`, subjects from `fetch.sh DIR`). Toolchain go1.27.3, darwin/arm64.

## Result

Unknown is `expression` or `unknown`. `unset` means the reader found the attribute absent, which a check answers against a contract, so it is not unknown. The headline rate is unknown over all pairs, `unset` included. The last two columns are the preregistered sensitivities, never merged into the verdict.

| | pairs | literal | reference | defaulted | unknown | unset | unknown rate | rate excl. unset | rate incl. defaulted |
|---|---|---|---|---|---|---|---|---|---|
| Terraform | 90 | 43 | 23 | 3 | 9 | 12 | 10.0% | 11.5% | 13.3% |
| CloudFormation | 132 | 67 | 3 | 9 | 8 | 45 | 6.1% | 9.2% | 12.9% |
| Pulumi YAML | 18 | 8 | 0 | 3 | 3 | 4 | 16.7% | 21.4% | 33.3% |

Counting `defaulted` as unknown too moves Terraform to 13.3% and CloudFormation to 12.9%: still under the bar.

### Reusable module against deployed root

| | pairs | literal | reference | defaulted | unknown | unset | unknown rate | rate excl. unset | rate incl. defaulted |
|---|---|---|---|---|---|---|---|---|---|
| Terraform reusable | 29 | 18 | 1 | 2 | 3 | 5 | 10.3% | 12.5% | 17.2% |
| Terraform deployed | 61 | 25 | 22 | 1 | 6 | 7 | 9.8% | 11.1% | 11.5% |
| CloudFormation reusable | 66 | 27 | 3 | 6 | 2 | 28 | 3.0% | 5.3% | 12.1% |
| CloudFormation deployed | 66 | 40 | 0 | 3 | 6 | 17 | 9.1% | 12.2% | 13.6% |
| Pulumi YAML reusable | 5 | 3 | 0 | 1 | 1 | 0 | 20.0% | 20.0% | 40.0% |
| Pulumi YAML deployed | 13 | 5 | 0 | 2 | 2 | 4 | 15.4% | 22.2% | 30.8% |

### By subject (unknown pairs of all pairs)

| Subject | Kind | Unknown / pairs | Subject | Kind | Unknown / pairs |
|---|---|---|---|---|---|
| tf-r1 | reusable | 2 / 7 | cfn-r1 | reusable | 0 / 19 |
| tf-r2 | reusable | 0 / 7 | cfn-r2 | reusable | 0 / 19 |
| tf-r3 | reusable | 0 / 5 | cfn-r3 | reusable | 0 / 10 |
| tf-r4 | reusable | 0 / 5 | cfn-r4 | reusable | 1 / 11 |
| tf-r5 | reusable | 1 / 5 | cfn-r5 | reusable | 1 / 7 |
| tf-d1 | deployed | 3 / 15 | cfn-d1 | deployed | 0 / 31 |
| tf-d2 | deployed | 3 / 15 | cfn-d2 | deployed | 0 / 11 |
| tf-d3 | deployed | 0 / 15 | cfn-d3 | deployed | 0 / 10 |
| tf-d4 | deployed | 0 / 8 | cfn-d4 | deployed | 3 / 7 |
| tf-d5 | deployed | 0 / 8 | cfn-d5 | deployed | 3 / 7 |

The Pulumi YAML subjects and the per-rule tables are in the scorer's `-mode md` output.

## Where the unknowns are

All 17 unknown Terraform and CloudFormation pairs, by what the source does:

| Cause | Pairs | Where |
|---|---|---|
| A KMS alias ARN built by interpolating data sources or pseudo-parameters (R19) | 8 | tf-d1 (2), tf-d2 (2), cfn-d4 (2), cfn-d5 (2) |
| `!If` choosing the key on a condition (R19) | 1 | cfn-r4 |
| An IAM document with a computed part (R15, R16) | 7 | `jsonencode(...)`: tf-d1, tf-d2. `templatefile(...)` and a `var`-indexed `policy_json`: tf-r1 (2). `AWS::AccountId` in a trust condition: cfn-d4, cfn-d5. `!Sub` with `AWS::URLSuffix`: cfn-r5 |
| `each.value.encrypted` from a `for_each` map (R11) | 1 | tf-r5 |

A document-valued rule counts a pair unknown when any attribute under its path is, so one pseudo-parameter in a condition makes the whole trust policy unknown. That is the preregistered rule, and it inflates R15 and R16 against scalar rules.

Three things limit how far the pooled rate carries:

1. **Clusters.** tf-d1 and tf-d2 are two near-identical solutions of one repository, and cfn-d4 and cfn-d5 likewise. They hold 6 of 9 Terraform unknowns and 6 of 8 CloudFormation unknowns. Left out (not preregistered, so a sensitivity and not the result), Terraform reads 3 of 60 (5.0%) and CloudFormation 2 of 118 (1.7%). The effect runs toward passing harder, not away.
2. **Few pairs.** 90 and 132 pairs over 17 and 16 live rules. Two rules (R15, R16) are 32 of the 90 Terraform pairs.
3. **`unset` is large on CloudFormation** (45 of 132). A rule that asks about an optional property mostly finds it absent, which is a correct, non-unknown answer, and it lowers the headline rate. The rate over pairs that are not `unset` is 9.2%, still under the bar.

## Prediction against the reading

| Prediction (design section 10) | Read | |
|---|---|---|
| Deployed roots under the bar | Terraform deployed 9.8% | held |
| CloudFormation under the bar | 6.1% overall, 3.0% reusable, 9.1% deployed | held |
| Reusable modules whose values all come from variables over the bar | Terraform reusable 10.3% (17.2% with `defaulted` counted) | did not hold in this pool |

The miss is about the pool, and this reading does not test why. The five reusable Terraform modules the density rule selected mostly write constants or locals inside the module (tf-r2, tf-r3 and tf-r4 read 0 unknown). The one that wires values through variables, tf-r5, reads 1 unknown and 2 defaulted of 5 pairs, which is 60% once `defaulted` is counted. A pool of reusable modules that pass every value in from variables (the `terraform-aws-modules` pattern, across more modules) would be the test of the prediction, and this one was too small to be it.

## The 50-pair audit

The audit was made by a model reading the source of each pair. No human made it. It checks each scored class against the source text under the design's class table, not against any other tool. The draw is the preregistered one: per tool, the 25 pairs with the smallest SHA-256 key of `ll-m6-audit-v1|rule|subject|file|type|name|startline`.

**Result: 50 agree, 0 disagree.** Scored classes in the 50: 28 literal, 6 reference, 5 unknown, 1 defaulted, 10 unset. With one `defaulted` and five `unknown` pairs, the audit has little power on the classes that decide the verdict. It confirms that the `literal`, `reference` and `unset` calls are right on what it drew, and that the five unknowns are computed in the source. A model auditing a scorer's output and finding nothing wrong is weaker evidence than a human doing it.

Judgement calls an auditor could read the other way, all marked agree:

- A01, A05: the value is `local.all_ips`, a local whose body is the constant `["0.0.0.0/0"]`. The reader says `literal`. The design's `defaulted` covers a local chain ending in a variable default and says nothing of one ending in a constant, so `literal` is the nearest class.
- A03: `each.value.encrypted` reads `expression`, not `reference`: it names an iteration value, none of the things `reference` lists.
- A35, A43: `!Ref 'AWS::AccountId'` inside a trust-policy condition reads `expression`, so the whole document pair is unknown.

| # | Rule | Subject | Resource | Scored | Audit | Note |
|---|---|---|---|---|---|---|
| A01 | R07 | tf-r4 | `aws_security_group_rule.allow_server_http_inbound` | literal | agree | local `all_ips` is a constant list; the reader resolves it to literal |
| A02 | R16 | tf-d2 | `aws_iam_policy.security_hub_org_policy_sns` | reference | agree | |
| A03 | R11 | tf-r5 | `aws_ebs_volume.this` | unknown | agree | `each.value`, no static answer |
| A04 | R01 | tf-r2 | `aws_s3_bucket_versioning.content` | literal | agree | |
| A05 | R07 | tf-r4 | `aws_security_group_rule.allow_all_outbound` | literal | agree | local `all_ips` is a constant list; the reader resolves it to literal |
| A06 | R07 | tf-d3 | `aws_security_group_rule.egress` | literal | agree | |
| A07 | R07 | tf-d4 | `aws_security_group_rule.incoming_dns_tcp` | literal | agree | |
| A08 | R05 | tf-d5 | `aws_security_group.db` | unset | agree | the only `cidr_blocks` is in an egress block |
| A09 | R05 | tf-d5 | `aws_security_group.alb` | literal | agree | |
| A10 | R10 | tf-d3 | `aws_db_instance.default` | literal | agree | |
| A11 | R08 | tf-d3 | `aws_db_instance.default` | literal | agree | |
| A12 | R15 | tf-d1 | `aws_iam_role.inspector_org_lambda_role` | reference | agree | |
| A13 | R13 | tf-d3 | `aws_instance.web_host` | literal | agree | |
| A14 | R19 | tf-d1 | `aws_sns_topic.inspector_org_dlq_alarm_topic` | unknown | agree | interpolation of data sources |
| A15 | R07 | tf-d4 | `aws_security_group_rule.outgoing_any` | literal | agree | |
| A16 | R05 | tf-d5 | `aws_security_group.wordpress` | unset | agree | the only `cidr_blocks` is in an egress block |
| A17 | R16 | tf-d1 | `aws_iam_policy.cloudformation_policy` | reference | agree | |
| A18 | R18 | tf-d2 | `aws_sqs_queue.securityhub_org_dlq` | literal | agree | |
| A19 | R16 | tf-d2 | `aws_iam_policy.security_hub_org_policy_securityhub` | reference | agree | |
| A20 | R05 | tf-r4 | `aws_security_group.instance` | unset | agree | |
| A21 | R16 | tf-d2 | `aws_iam_policy.security_hub_org_policy_logs` | reference | agree | |
| A22 | R19 | tf-d2 | `aws_sns_topic.securityhub_org_dlq_alarm_topic` | unknown | agree | interpolation of data sources |
| A23 | R11 | tf-d3 | `aws_ebs_volume.web_host_storage` | unset | agree | attribute is commented out |
| A24 | R15 | tf-d5 | `aws_iam_role.task_execution_role` | literal | agree | |
| A25 | R07 | tf-d4 | `aws_security_group_rule.incoming_dns_udp` | literal | agree | |
| A26 | R03 | cfn-d1 | `AWS::S3::Bucket.SiteContentLogBucket` | literal | agree | |
| A27 | R07 | cfn-r1 | `AWS::EC2::SecurityGroupIngress.ALBSecurityGroupInHttpWorld` | literal | agree | |
| A28 | R07 | cfn-d2 | `AWS::EC2::SecurityGroupIngress.EcsSecurityGroupIngressFromSelf` | unset | agree | |
| A29 | R05 | cfn-r3 | `AWS::EC2::SecurityGroup.VaultSecGroup` | reference | agree | parameter has no default |
| A30 | R15 | cfn-r2 | `AWS::IAM::Role.BackupRole` | literal | agree | |
| A31 | R04 | cfn-d1 | `AWS::S3::Bucket.SiteCloudFrontLogsLogBucket` | unset | agree | |
| A32 | R15 | cfn-d1 | `AWS::IAM::Role.JwtResourceHandlerRole` | literal | agree | |
| A33 | R07 | cfn-r5 | `AWS::EC2::SecurityGroupIngress.BastionSecurityGroupPrefixListSshIngress` | unset | agree | |
| A34 | R15 | cfn-r1 | `AWS::IAM::Role.Role` | literal | agree | |
| A35 | R15 | cfn-d4 | `AWS::IAM::Role.rScheduledComplianceScheduleRole` | unknown | agree | `AWS::AccountId` pseudo-parameter inside the document |
| A36 | R04 | cfn-d1 | `AWS::S3::Bucket.SiteCloudFrontLogsReplicaBucket` | unset | agree | |
| A37 | R15 | cfn-d2 | `AWS::IAM::Role.ECSRole` | literal | agree | |
| A38 | R05 | cfn-r2 | `AWS::EC2::SecurityGroup.AgentSG` | unset | agree | |
| A39 | R20 | cfn-d1 | `AWS::Lambda::Function.JwtResourceHandler` | literal | agree | |
| A40 | R15 | cfn-d2 | `AWS::IAM::Role.AutoscalingRole` | literal | agree | |
| A41 | R07 | cfn-d2 | `AWS::EC2::SecurityGroupIngress.PrivateLoadBalancerIngressFromECS` | unset | agree | |
| A42 | R03 | cfn-d3 | `AWS::S3::Bucket.VpcFlowLogsBucket` | literal | agree | |
| A43 | R15 | cfn-d5 | `AWS::IAM::Role.rScheduledComplianceScheduleRole` | unknown | agree | `AWS::AccountId` pseudo-parameter inside the document |
| A44 | R20 | cfn-d4 | `AWS::Lambda::Function.rAccountAlternateContactsLambdaFunction` | literal | agree | |
| A45 | R12 | cfn-r5 | `AWS::EC2::LaunchTemplate.BastionLaunchTemplate` | defaulted | agree | parameter default is `required` |
| A46 | R15 | cfn-r3 | `AWS::IAM::Role.LeaderElectionLambdaExecutionRole` | literal | agree | |
| A47 | R01 | cfn-d1 | `AWS::S3::Bucket.SiteContentLogBucket` | literal | agree | |
| A48 | R01 | cfn-d1 | `AWS::S3::Bucket.SiteContentBucket` | literal | agree | |
| A49 | R15 | cfn-d1 | `AWS::IAM::Role.TestResourceHandlerRole` | literal | agree | |
| A50 | R15 | cfn-r3 | `AWS::IAM::Role.ClusterBootstrapLambdaExecutionRole` | literal | agree | |

## Method and reproduction

```
calibration/iac/unknownrate/fetch.sh DIR
go run ./calibration/iac/unknownrate -dest DIR -mode md      # the tables above
go run ./calibration/iac/unknownrate -dest DIR -mode audit   # the 50 pairs with source
```

Pins are in `calibration/iac/unknownrate/subjects.tsv`, rules in `rules.json`, definitions in `PREREGISTRATION.md`. Nothing derived from provider or CloudFormation registry schemas is used: the rules name types and paths, and the reader's own output is the only reading scored.
