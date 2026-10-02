# aws-iam-policy — provenance and divergences

LB-Q-aws-iam-policy (wave pc), design `docs/design/language-breadth.md` §4, §13. A doc-engine
dialect (no `tags.scm`, no grammar, no incumbent: an IAM policy was generic JSON, read as bare
keys, or unclassified YAML). The decode is `gopkg.in/yaml.v3` through the doc engine for the
`.json` form too (YAML reads JSON). The format is AWS's IAM JSON policy grammar
(<https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_grammar.html>); nothing is
copied from a source tree, the selectors are written from that grammar.

## How a file is claimed: by content, never by name

The manifest claims no basename, glob or extension. `doc.json`'s `detect.content_sniff` is the
signal: a column-0 (or JSON) `Version` of `2012-10-17` or `2008-10-17` and a `Statement` key, in
either order. `fusion.cloudPolicyDispatch` (internal/fusion/cloudpolicydispatch.go) adds a decode
check (the root entity and at least one statement that is a map, so `{"Version": "2012-10-17",
"Statement": "x"}` stays generic JSON) and never claims a file an earlier signal claims
(package.json by basename, a CloudFormation template, an OpenAPI document, a Kubernetes manifest,
Envoy/Kong/Traefik, a GitHub Actions or GitLab CI file). An inline `PolicyDocument` in a
CloudFormation template or a Terraform `aws_iam_policy_document` is not read: LB-X-iam-inline.

## What it models

- **The document** (`IAMPolicy`, canonical `policy`): the root, named by the file, signed by its
  `Version`.
- **Statements** (`IAMStatement`, canonical `rule`): each `Statement` item that is a map (a lone
  object is its own one item), named by its `Sid`, else its position in the list from 0 (`$index`,
  an additive doc-engine value), signed by its `Effect`.
- **References** (`Calls` on a statement, L2, as written, never resolved): every `Action`,
  `NotAction`, `Resource`, `NotResource`, `Principal` and `NotPrincipal` value (a string or a list;
  a principal object's members, by principal type). `Calls` is a set: a `*` that is both a
  Principal and a Resource appears once.
- **`policy_ref`** (cross-file, emitted, resolved by item k8s's `internal/fusion/policyref.go`, not
  here): each `Resource` and `NotResource` value, edge `policy_ref`, strategy `aws-arn`, Raw and
  Target the value as written, From the statement. A `*` and a variable-bearing ARN
  (`${aws:username}`) are emitted too; whether one resolves, and why not, is the resolver's.

## What is not modelled, and why

- **Conditions** (`Condition` keys and values), `Id`, `Sid` uniqueness, **policy variables** inside
  ARNs, and the **meaning** of an action or ARN (what a managed policy grants): references are
  written names, not an access analysis.
- **YAML policies are fixtures only**: the oracle reads JSON, so no measured subject is YAML. The
  YAML form (a CloudFormation `PolicyDocument` pulled out, a standalone `.yaml`) is covered by the
  `policy.yaml` golden and the dispatch tests.
- **Inline IAM** in CloudFormation and Terraform (LB-X-iam-inline).

## Calibration oracle

`calibration/oracles/parliament/parliament_oracle.py` (bench-only), tier C: duo-labs/parliament
1.6.4 (BSD-3-Clause, pinned at 574ea45079fadfc5f4559ee8934b818d9066761c), an IAM linter, not the
format's reference parser; its reader is `jsoncfg`, and its `make_list` and `Statement` classes
list each statement's elements. Tier C caps every metric at **good**. Parliament's own test
policies are Python strings inside its tests, not files, so the subjects
(`calibration/corpus.d/aws-iam-policy.yaml`) are tenchi-security/camp (4,713 managed-policy
documents) and awslabs/terraform-iam-policy-validator's fixtures (6).
