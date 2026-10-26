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
