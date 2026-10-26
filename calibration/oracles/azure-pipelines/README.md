# Azure Pipelines oracle

Bench-only Python with PyYAML 6.0.3 (MIT), installed under scratch; no product
dependency. `python azure_oracle.py -subject <checkout> -out <oracle.json>`
walks the independent parser's nodes and source marks. It reports the file,
literal stages/jobs/deployments and explicit same-file dependsOn name pairs.
It does not consume fathom output, manifest selectors or production code.
Malformed files are counted. A visited-node set bounds cyclic alias walks.
Tests: `python -m unittest discover -s calibration/oracles/azure-pipelines`.

Tier C is deliberate. Existing work checked before implementation:
[Microsoft's language server](https://github.com/microsoft/azure-pipelines-language-server)
at b36c23dba227da8077d125d33eda32d3fa64e7f2 (MIT, service 0.9.2) provides generic
YAML outlines and a URI-only template definition routine, not evaluation.
The checked native routes and concrete limitations are listed below. The PyYAML walk is not relabelled as Azure's own tool.

Measure complete sparse subject scopes in calibration/corpus.d/azure-pipelines.yaml
with `indexbench lang-oracle -dialect azure-pipelines -tool-oracle <oracle.json>`
without `-only`. Re-time with `indexbench lang-retime -runs 3` on every subject.
Literal-name syntax is measured; template execution and L3 are not.

## Named candidate audit (X-R1)

| Candidate | Version / identity | Result or concrete limitation |
|---|---|---|
| Microsoft Azure Pipelines language service | 0.9.2 at b36c23dba227da8077d125d33eda32d3fa64e7f2 | Source inspected, not installed. `yamlDefinition.ts` returns normalized template URIs without reading targets; document symbols are generic YAML outlines. Neither supplies evaluated pipeline entities or dependency bindings. |
| Microsoft agent `DistributedTask.Pipelines.Yaml.PipelineParser` | be18ae23fdcb8963453e1b961c0a5fecf62f1bba | Source inspected, not built: `ProcessConverter.ReadYaml` accepts legacy `phases`, `template`, `steps` and related process properties. Modern top-level `stages`, `jobs` and `extends` reach the default `Unexpected process property` exception. It is not the modern service compiler for this corpus. |
| Azure DevOps Pipelines Preview REST API | 7.1 | The documented native dry-run route returns `finalYaml` after template parsing. It requires an organization, project, pipeline ID and OAuth build scope. None is supplied for this program; no account/resource is created or request sent. This is an owner-supplied service-context ceiling, not an absent compiler. |
| Independent source positions and literal name pairs | PyYAML 6.0.3 | Run on all 91 files; parser-node walks remain C. Successful YAML parsing is not native Azure evaluation. |
| Product YAML locator on duplicate mapping keys | fathom 5b84aa5b4 | Earlier duplicate `extends` occurrence is overwritten (227/228 definitions); shared decoder work is held by T-R2. It is retained in truth. |

Primary source evidence:
[agent ProcessConverter at the inspected commit](https://github.com/microsoft/azure-pipelines-agent/blob/be18ae23fdcb8963453e1b961c0a5fecf62f1bba/src/Agent.Listener/DistributedTask.Pipelines/Yaml/TypeConverters/ProcessConverter.cs),
[Preview API 7.1](https://learn.microsoft.com/en-us/rest/api/azure/devops/pipelines/preview/preview?view=azure-devops-rest-7.1).
The preview route is a future native instrument if the owner supplies approved
service context and a bounded input protocol; it is not run under the current
no-paid-API brief. No candidate is described as a runtime failure when only
its source interface or required service inputs were checked.

## X-R4 source search, 2026-10-25

This bounded search records sources even when they produced no applicable
instrument. A negative search does not establish that no instrument exists.

| Required source | Search and observed result |
|---|---|
| Owner toolchain / introspection | The pinned Microsoft agent parser, service 0.9.2 and Preview REST 7.1 above were inspected. The documented native Preview route is usable with owner-supplied service context; the legacy agent parser does not accept the measured modern syntax. |
| SCIP and LSIF catalogs plus GitHub | Read [Sourcegraph supported indexers](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), the [SCIP indexer list](https://github.com/sourcegraph/scip#tools-using-scip), Sourcegraph's [LSIF indexer guide and linked catalog](https://sourcegraph.com/blog/writing-an-lsif-indexer), and [LSIF.dev](https://lsif.dev/). No Azure pipeline entity indexer was listed. GitHub repository search `scip-azure-pipelines in:name` returned zero; web GitHub searches for that literal found unrelated SCIP optimization feeds, not an indexer. TypeScript SCIP indexes implementation code, not pipeline YAML evaluation. |
| Language servers | Microsoft's pinned service above and its published npm server 0.9.3 (MIT) are the named owner candidates. The server packages the service rather than adding a pipeline execution engine. The LSP server catalog was also checked; it supplied no additional pipeline evaluator. No package was installed or runtime failure claimed. |
| Registry parsers / type checkers / AST dumpers | npm search `azure pipelines parser` and direct package metadata for `azure-pipelines-language-service` 0.9.2, `azure-pipelines-language-server` 0.9.3, `yaml-language-server` 2.0.0 (all MIT) checked. Search also surfaced task-lib 5.281.2, a task authoring library, not a pipeline compiler. Generic YAML ASTs do not supply owner evaluation; the measured independent AST instrument is PyYAML 6.0.3 (C). |
| Runtime call / require tracing | Read Microsoft's [pipeline diagnostic logs](https://learn.microsoft.com/en-us/azure/devops/pipelines/troubleshooting/review-logs?view=azure-devops): `system.debug`/agent diagnostics describe executed jobs and tasks after a service run. They require a configured pipeline and agent/service context, and are not a local template-binding dump. No workflow was executed. Preview remains the concrete native route to investigate when approved context is supplied. |

The ceiling is specifically the current offline syntax measurement and missing
service context, together with the retained duplicate-key decoder miss; it is
not a claim that Azure lacks native compilation or diagnostics.

## Dispatch review reproduction

`review_regrade.py` replays the complete canonical OpenAPI and CloudFormation
shards against two indexbench binaries using identical oracle output. It checks
pinned clean subjects and native tool versions. Install swagger-parser 13.1.0
and yaml 2.8.1 under a private npm prefix and cfn-lint 1.52.1 in a private Python
environment (Python 3.13 was used). With baseline and candidate binaries built
from the identities in `cross-item-regrade-r4.json`, run:

```sh
python3 calibration/oracles/azure-pipelines/review_regrade.py \
  -baseline workspace/tools/indexbench-main -baseline-commit c86095291 \
  -candidate workspace/tools/indexbench-candidate -candidate-commit 56529ac87 \
  -subjects workspace/calibration/azure-review -npm-prefix workspace/oracle/openapi \
  -cfn-python workspace/oracle/cfn/bin/python -out workspace/tmp/yaml-review-fresh
```

For every existing YAML fixture, build `dispatchsnapshot/main.go` once with the
baseline module and once with this branch's module. The source uses no new
product API. Feed both binaries the same candidate fixture tree:

```sh
go build -o workspace/tools/dispatch-candidate ./calibration/oracles/azure-pipelines/dispatchsnapshot
python3 calibration/oracles/azure-pipelines/compare_snapshots.py \
  -baseline workspace/tools/dispatch-baseline -candidate workspace/tools/dispatch-candidate \
  -root . -out workspace/tmp/yaml-dispatch-review/comparison.json
```

`cross-item-regrade-r4.json` and `yaml-dispatch-r4.json` retain aggregate counts,
subject identities and digests; raw native output and per-file inventories stay
in scratch. These checks establish regression equivalence, not new oracle
authority or a program L3 grade. The own reading is reproduced with
`regrade_azure.py -indexbench workspace/tools/indexbench-candidate -subjects
workspace/calibration/azure-pipelines -out workspace/tmp/azure-reading-fresh`.
