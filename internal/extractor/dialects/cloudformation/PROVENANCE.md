# cloudformation — provenance and divergences

LB-Q-cloudformation, design `docs/design/language-breadth.md` §4, §13. A doc-engine dialect (no
`tags.scm`, no upstream grammar, no incumbent: a YAML template was unclassified YAML, a JSON one
read by the generic JSON extractor as bare keys). The decode is `gopkg.in/yaml.v3` through the
doc engine, for `.json` templates too (YAML reads JSON), with `yaml_tags: "cloudformation"`: a
short-form intrinsic reads as its long form, cfn-lint's rule (`!Ref X` is `{Ref: X}`,
`!Condition X` is `{Condition: X}`, any other `!Name v` is `{Fn::Name: v}`, and a GetAtt written
`A.B`, short form or long, is the list `[A, B]`).

## How a file is claimed: by content, never by name

The manifest claims no basename, glob or extension: a template is a `.yaml`, `.yml` or `.json`
whose content says so, and claiming every YAML or JSON file would take them from the Kubernetes,
Compose, GitHub Actions, GitLab CI and generic JSON extractors. `doc.json`'s
`detect.content_sniff` is the signal, a regular expression over the whole file:
`AWSTemplateFormatVersion` as a top-level YAML key or a JSON key, a `Transform` naming an `AWS::`
transform (SAM's `AWS::Serverless-2016-10-31`), or a top-level `Resources` map (a column-0 YAML
key, or a JSON key) followed by a `Type` that is a `Namespace::Service` name. fusion routes a
YAML file to the dialect through classifyYAML's last signal (only a file no other tool's signal
claimed: every earlier verdict is unchanged) and a JSON file through resolveExtractor before the
generic JSON extractor, when the sniff matches and the file decodes with a top-level `Resources`
(so a JSON schema or specification that only names a template key keeps the JSON extractor); a
basename claim (`package.json`) still wins. `detect.require_keys: ["Resources"]` holds per
document. `fusion.Pipeline.ContentDialect` is the same decision, which the harness measures on.

## What it models

- **The template** (`CFNTemplate`, canonical `package`): one entity per file, named by the file
  (`$file`), at its first key, signed by its Transform, else its AWSTemplateFormatVersion. It is
  what a nested stack's reference resolves to.
- **Parameters** (`CFNParameter`, `variable_decl`), **Mappings** (`CFNMapping`, `table`),
  **Conditions** (`CFNCondition`, `rule`), **Resources** (`CFNResource`, `resource`) and
  **Outputs** (`CFNOutput`, `output`): every entry of the section whose value is a mapping, named
  by its logical ID, at its key's line; a parameter and a resource are signed by their `Type`, an
  output by its `Description`.
- **References** (`Calls` on a resource or an output, L2, intra-file, each only when it names an
  entity the template declares: `declared_only`): a `Ref` at any depth (a parameter or a
  resource; a pseudo parameter, `AWS::Region`, is not the template's), the resource a
  `Fn::GetAtt` names at any depth, a resource's `DependsOn` (a string or a list), a `Condition`
  attribute, and the condition a `Fn::If` names first.
- **Nested templates** (L3, `fusion.ResolveDocXrefs`, file-tier edges from the resource to the
  child template's entity): an `AWS::CloudFormation::Stack`'s `TemplateURL` and an
  `AWS::Serverless::Application`'s `Location` written as a relative local path (what `aws
  cloudformation package` and `sam package` upload), resolved by `repo-path` from the template's
  directory. An S3 or HTTPS URL, one built with `!Sub`, a SAR application (a `Location` mapping)
  and a placeholder resolve to nothing.

## What is not modelled, and why

- **`Fn::Sub`'s `${}` names**: reading them needs a string scan, not a selector (design §4: no
  expression language). cfn-lint's graph draws them (110 of its 3,173 edges on the subjects); the
  truth leaves them out and the reading says so.
- **Fn::FindInMap's map, Fn::ImportValue and Export names** (cross-stack, not cross-file),
  **references from a Conditions entry**, SAM **Globals**, the **Rules** section, **Fn::ForEach**
  resources (a list, not a mapping, under Resources), and a `.template` file (no extractor reads
  the extension).
- **L3 is not measured**: neither pinned subject nests a template by a relative local path
  (LB-Q-cloudformation-rest).

## Calibration oracle

`calibration/oracles/cfn-lint/cfn_oracle.py` (bench-only), run with cfn-lint 1.52.1's own
interpreter: its decode (tier A, the format's reference parser: definitions and references) and
its dependency graph as the cross-check; nested local templates resolved by the adapter (tier C,
since cfn-lint does not resolve them). Subjects: `calibration/corpus.d/cloudformation.yaml`.

## Telemetry identities (TL-5, ADR-062)

A literal `Properties.FunctionName` on an `AWS::Lambda::Function` or an
`AWS::Serverless::Function` is the deployed function's name, the identity an alert or a
log group carries: a `TelemetryIdentity` entity (canonical `key`, the contract kind
several dialects share) at the resource, named by the literal and signed
`aws.lambda.function_name=<name>`. `!Ref`, `!Sub`, `!Join` (read as their long-form maps)
and an absent name yield nothing: CloudFormation names those at deploy time, and fathom
never guesses one. Fixture: `testdata/telemetry.yaml`.
