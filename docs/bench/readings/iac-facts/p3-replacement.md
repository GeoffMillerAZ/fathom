IF-P3 verdict: PARTIAL. For AWS the static scan resolves 98.1% of top-level attributes and blocks (97.8% of all 171k attribute paths) and, against the offline `replace_paths` oracle, produced 0 false alarms and 0 silent misses among the attributes it resolved. Fact 5 is feasible for AWS, and IF-13's resolution rate is 98%, not the 53% the earlier quick script reached. The offline oracle works for AWS and Google with fake credentials, seeded state and all egress denied, and for AzureRM only with a stub `az` command. The source scans of AzureRM and Google were skipped, so those two providers are unread.

## Summary

Two instruments, `terraform-provider-aws` v6.67.0 (the 6.67.0 pin in `calibration/oracles/terraform-provider-schema/`):

1. A `go/ast` scan of the tagged source (throwaway prototype, 1,248 lines of Go, not committed). It finds each resource from its `@SDKResource` or `@FrameworkResource` annotation, evaluates the schema expression, and joins the result to the OpenTofu export by attribute path.
2. `tofu plan -refresh=false` per resource against the same provider build, with fake credentials, a seeded state holding only an `id`, and every network call denied.

### Resolution rate (the number IF-13 rests on)

Resolved means the scan reached the attribute's schema element and read a definite answer: `ForceNew`/`RequiresReplace` true, conditional, or absent. Computed-only attributes and the wrapper-injected `id`, `tags`, `tags_all`, `region` and `timeouts` are left out of the base.

| Population | Attributes | Resolved |
|---|---|---|
| Top-level attributes and blocks | 10,000 | 9,807 (98.1%) |
| Top-level, SDKv2 (1,240 resources) | 7,800 | 7,707 (98.8%) |
| Top-level, framework (487 resources) | 2,200 | 2,100 (95.5%) |
| All paths, nested included | 170,964 | 167,132 (97.8%) |
| All paths, without the six mega resources | 28,328 | 24,496 (86.5%) |

The six mega resources are `aws_wafv2_web_acl`, `aws_wafv2_web_acl_rule`, `aws_wafv2_rule_group` and `aws_quicksight_analysis`, `_dashboard` and `_template`. They hold 142,636 of the 170,964 paths (83%) because the export expands recursion (WAF statements) and deep shared schemas (QuickSight). Path-weighted rates are mostly their rate, so the per-resource figures matter more: median resource 100%, 1,613 of 1,719 resources fully resolved, 1,637 at 90% or more, 27 under 50%.

Top-level status: 4,079 replace, 193 conditional, 5,535 do not replace, 193 unknown. 6,496 attribute paths replace in all. Conditional comes from `DiffSuppressFunc` on a `ForceNew` attribute (93 paths), `CustomizeDiff` or a framework `ModifyPlan` naming the attribute (83) and a framework `RequiresReplaceIf` modifier (44).

What stays unknown (all paths): framework schema built through a helper the scan does not follow (2,080 paths under an unresolved parent, 312 opaque maps), SDKv2 document helpers in `sdkv2` (`IAMPolicyDocumentSchema*`, `JSONDocumentSchema*`, a few dozen attributes), 46 framework attributes with a custom plan modifier, 16 framework resources whose `Schema` method assigns the schema in a form the scan did not read, and one resource that rewrites `ForceNew` after construction (below).

Cases the earlier quick script missed and this scan reads: helper-built schemas through `sync.OnceValue` variables and `tfsync.OnceValueCtx` (about 60k paths alone), `SchemaFunc` given as a function value, local-variable schemas in framework `Schema` methods, WAF's recursive `schemaCache`, QuickSight's `LoadOrStore` cache, `maps.Merge` and map index assignment, constants from the `names` package, and `ForceNew` that a `DiffSuppressFunc` can cancel.

### Counts the design asked for

- Resources with a `CustomizeDiff`: 145 of 1,240 SDKv2 resources. 54 of them name attributes or call `ForceNew` inside it (95 attribute names). The other 91 can still replace through paths the scan cannot see, so the answer for them carries a disclosure, as design section 3 says.
- Framework resources with a `ModifyPlan` method: 14 of 487. All 14 name their `RequiresReplace` paths with `path.Root("...")`.
- One resource rewrites `ForceNew` after building its schema: `aws_spot_instance_request` (`x.ForceNew = true` in a loop over a copied `aws_instance` schema). Its 43 top-level attributes are marked unknown. It is the only `.ForceNew =` assignment in the provider's non-test source.
- Raw lexical counts, not scans: `ForceNew: true` lines 4,738 (AWS), 4,619 (AzureRM), 8,057 (Google); `RequiresReplace` lines 1,930, 181 and 14; files mentioning `CustomizeDiff` 149, 224 and 769. Google and AzureRM are nearly all SDKv2, so the framework half of the scan matters mostly for AWS.

### Accuracy of the resolved answers against the oracle

204 AWS resources sampled with seed 20261104: 80 SDKv2 and 80 framework at random, 15 random with `CustomizeDiff`, 8 random with `ModifyPlan`, plus 22 widely used ones (`aws_instance`, `aws_s3_bucket`, `aws_vpc`, `aws_subnet`, `aws_security_group`, `aws_db_instance`, `aws_lambda_function`, `aws_iam_role`, `aws_lb`, `aws_kms_key`, `aws_dynamodb_table`, `aws_ecs_service`, `aws_rds_cluster`, `aws_sqs_queue`, `aws_sns_topic`, `aws_route53_record`, `aws_cloudfront_distribution`, `aws_elasticache_cluster`, `aws_autoscaling_group`, `aws_ebs_volume`, `aws_wafv2_web_acl`, `aws_quicksight_analysis`). 198 planned (115 of 117 SDKv2, 83 of 87 framework, 35 of 37 with `CustomizeDiff`, 6 of 8 with `ModifyPlan`). The 6 that did not: two plugin crashes on an all-unknown config (`aws_bedrockagentcore_gateway_target`, `aws_s3_directory_bucket`), two cross-field validation errors (`aws_sns_topic_subscription`, `aws_timestreaminfluxdb_db_cluster`), one value conversion error (`aws_quicksight_refresh_schedule`), and `aws_elasticsearch_domain`, which hung for over four minutes under denied egress and was killed (a plan that makes an API call retries until it times out).

Top-level attributes and blocks of the 198 planned resources, scan against `replace_paths`:

| | SDKv2 | Framework |
|---|---|---|
| Scan says replace, oracle agrees | 369 | 161 |
| Scan says replace, oracle disagrees (false alarm) | 0 | 0 |
| Scan says no replace, oracle replaces (silent miss) | 0 | 0 |
| Scan says no replace, oracle agrees | 1,057 | 244 |
| Scan says conditional, oracle replaces | 9 | 13 |
| Scan says conditional, oracle does not | 29 | 5 |
| Scan unknown, oracle replaces | 29 | 0 |
| Scan unknown, oracle does not | 18 | 5 |

Of 581 attributes the oracle replaces on, the scan names 530 as replace (91.2%), 22 as conditional (3.8%) and 29 as unknown (5.0%). 28 of the 29 are `aws_spot_instance_request`, the other is `aws_glacier_vault_lock.policy`. Precision of "replace" is 530 of 530. If conditional counts as a positive, precision is 552 of 586 (94.2%), the 34 extra being `DiffSuppressFunc` and `CustomizeDiff` cases that the all-unknown test input cannot trigger. Against the IF-M2 bars (precision 0.95, recall 0.90) this is a preview only: the sample is top-level, one provider, one input shape.

An earlier pass found two real gaps that changed the scan before these numbers: `aws_lb_target_group.protocol_version` is `ForceNew` but a `DiffSuppressFunc` cancels the diff (one false alarm until the scan marked it conditional), and `aws_glue_catalog.federated_catalog` replaces from `ModifyPlan` (one silent miss until the scan read `RequiresReplace` appends).

### Does `tofu plan -refresh=false` give `replace_paths` offline

Yes for AWS and Google, and for AzureRM with a stub.

- Method: per resource, a `main.tf` with the provider block (fake keys and the provider's `skip_*` switches for AWS, `access_token = "fake"` for Google), a `terraform_data` resource, and the resource under test with every configurable attribute set to `jsondecode(terraform_data.u.output)` (an unknown of any type, so validation and type checks pass) and every block as a `dynamic` block with an unknown `for_each` and its required children set the same way. A hand-written v4 state file holds the resource with only `id`. All unset attributes are null. `tofu plan -refresh=false -lock=false -state=<seed> -out=<plan>`, then `tofu show -json`. Every attribute differs from state, so `replace_paths` lists exactly the attributes that force replacement. One plan reads a whole resource.
- Egress: both commands ran under `sandbox-exec` with `(deny network-outbound (remote ip/tcp/udp))`, `(allow default)` otherwise, so the plugin's local socket still works. Control: `curl` to the registry fails inside the profile and returns 302 outside it.
- Providers came from a filesystem mirror of the pinned archives (checksums as in `pins.json`; the AWS archive is the OpenTofu build with sha256 `25afdd7e...`). No cloud credentials were used.
- AWS: 198 of 204 planned (above). 175 of the 198 planned resources replace on at least one attribute.
- Google 8.8.1: 30 of 30 random resources planned, 30 with `replace_paths` (97 paths in all).
- AzureRM 5.11.0: 0 of 30 with fake client credentials, because the provider requests a token from the Microsoft login endpoint at configure time and fails with egress denied. With `use_cli = true` and a stub `az` on `PATH` (answers `account show`, `account list` and `get-access-token` with fake JSON and an unsigned JWT) 29 of 30 planned and 28 had `replace_paths` (127 paths). The stub is a maintainer tool for the oracle, never part of the product. The one failure was a validation message on `azurerm_monitor_activity_log_alert`.

## Limits

- Top-level only for accuracy. Nested attributes and nested blocks are resolved by the scan (97.8% of all paths) but their answers were not checked against a plan. Block-level `replace_paths` entries were compared at the block name only.
- The oracle's all-unknown input cannot exercise conditional replacement (`RequiresReplaceIf`, `ForceNewIf` on a value, `DiffSuppressFunc` on a known value). It shows what replaces when everything changes, and conditional cases show as "cond" in the table.
- AWS 6.67.0 only. AzureRM and Google source was cloned (v5.11.0, v8.8.1) but not scanned: their registrations and typed resources use different shapes, and AWS is the owner's priority. Their oracle runs above are samples of 30, with no scan to compare.
- A resolved "no replace" is only as good as the resource's other signals: `aws_spot_instance_request` shows that a post-construction rewrite makes the schema literal wrong, which is why the scan marks a resource unknown when any `.ForceNew =` assignment is reachable from its constructor.
- The two plugin crashes and the hung plan mean about 3% of AWS resources need a different input shape or a timeout. 
- The scan is a prototype tuned on this provider after seeing its misses, so the 98% is a fit to `aws` 6.67.0. A new release can reintroduce unknowns. Re-measure per pin.
- Provider archives, exports, plans and state files stay under the maintainer's scratch directory and are not committed.

## What the next stage must know

- Ship the resolved-or-unknown shape: a fact is `replace`, `conditional` or `unknown`, never a plain "no", because 193 of 10,000 top-level answers are unknown and `CustomizeDiff` and `ModifyPlan` exist.
- The offline oracle is one plan per resource at about 5 seconds each (204 AWS resources in under 6 minutes with four workers), enough to run IF-M2's 200 pairs per provider. Set a per-plan timeout.
- AzureRM needs the stub `az` for any oracle run.
