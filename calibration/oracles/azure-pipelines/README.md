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

## Issue 211 offline SDK probe

The [assessment](../../../docs/assessments/2026-10-25-tools-azure-pipelines.md#issue-211-offline-evaluation-exists-native-l3-remains-unmeasured-2026-11-15)
and offline-probe-211.json retain a real, network-denied runtime feasibility
probe of runner.server's existing template evaluator. It grants no support tier
or canonical reading. The driver is authored; it does not execute jobs, start a
server or implement a resolver. Use the recipe's common heavy wrapper outside
these commands; do not nest a lease. Retain existing outputs and choose fresh
output directories for reruns. A retry in an explicitly owned output location
invalidates its prior success receipt before validation; aliased input/output
paths (including symlinks and hard links) are refused before any mutation.

Acquire the exact MIT source at release v3.14.0, commit
069646146c90d649c74dfd7a34569c9420195838 from
`https://codeload.github.com/ChristopherHX/runner.server/tar.gz/069646146c90d649c74dfd7a34569c9420195838`.
Acquire the SDK URL/version and SHA512 in offline-probe-211.json's acquisition
object from Microsoft's release metadata. Verify both archives against the
receipt before extraction, entirely under workspace/tools/azure-211. The source
archive SHA256 and public metadata URL are retained; no global install is needed.
This measured process used SDK 8.0.428 osx-arm64. Another platform requires its
own explicit SDK identity and a new diagnostic receipt, never bypassing a pin.

Before **any** dotnet command (including build), set the local environment and
disable certificate generation and persistent build/compiler servers:

```sh
export DOTNET_ROOT="$PWD/workspace/tools/azure-211/dotnet"
export DOTNET_CLI_HOME="$PWD/workspace/tools/azure-211/cli-home"
export NUGET_PACKAGES="$PWD/workspace/tools/azure-211/nuget/packages"
export NUGET_HTTP_CACHE_PATH="$PWD/workspace/tools/azure-211/nuget/http"
export DOTNET_GENERATE_ASPNET_CERTIFICATE=false DOTNET_ADD_GLOBAL_TOOLS_TO_PATH=false
export DOTNET_CLI_TELEMETRY_OPTOUT=1 DOTNET_NOLOGO=1 DOTNET_SKIP_FIRST_TIME_EXPERIENCE=1
export DOTNET_CLI_WORKLOAD_UPDATE_NOTIFY_DISABLE=1 DOTNET_CLI_USE_MSBUILD_SERVER=0
export MSBUILDDISABLENODEREUSE=1
```

Create workspace/tools/azure-211/probe/Probe.csproj with this exact project
reference (relative to that directory), and copy OfflineProbe211.cs alongside it
as Program.cs. All dependencies are the upstream SDK's own pinned references;
root go.mod and the production library are untouched.

```xml
<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup><OutputType>Exe</OutputType><TargetFramework>net8.0</TargetFramework></PropertyGroup>
  <ItemGroup><ProjectReference Include="../runner.server-069646146c90d649c74dfd7a34569c9420195838/src/Sdk/Sdk.csproj" /></ItemGroup>
</Project>
```

```sh
workspace/tools/azure-211/dotnet/dotnet build workspace/tools/azure-211/probe/Probe.csproj -c Release -m:2 -nodeReuse:false -p:UseSharedCompilation=false -p:RuntimeIdentifiers= -p:Version=3.14.0
python3 -B calibration/oracles/azure-pipelines/run_offline_211.py --dotnet workspace/tools/azure-211/dotnet/dotnet --assembly workspace/tools/azure-211/probe/bin/Release/net8.0/Probe.dll --out workspace/artifacts/lang-211/azure-runtime-replay
python3 -B -m unittest discover -s calibration/oracles/azure-pipelines -p test_offline_probe_211.py
bash scripts/fathom-go.sh test -race -count=1 ./pkg/lang -run '^TestAzurePipeline211PublicDispatch$'
bash scripts/fathom-go.sh test -race -count=1 ./internal/langbench -run '^(TestCommittedReadingsRoundTrip|TestCommittedReadingsAreCanonical|TestRequiredKindsThroughCanonical)$'
```

The runtime launcher uses `/usr/bin/sandbox-exec` with `(deny network*)`; failure
to apply the sandbox fails the process. A receipt is written only after the
entire authored population passes exact entity/dependency/refusal validation.
The 1 MiB stdout check is post-capture parsing, not an output-write cap; only
fixed authored inputs are accepted by this driver. No arbitrary subject-input
or adversarial producer-output bound is claimed.

Mutation: copy offline_probe_211.py and test_offline_probe_211.py into an ignored
scratch directory. Independently replace `len(rows)!=4` with `False`, the full
classification/support/attestation `if` condition with `if False:`, or
`if actual!=expected[name]:` with `if False:`; run
`python3 -B -m unittest discover -s <scratch> -p test_offline_probe_211.py` for
each mutant, restoring between them. The population, classification and pairing
controls must fail before the restored suite passes. The CLI cap+1 and malformed
JSON controls reject the whole receipt without creating an output. Test logs
and failed attempts are retained under workspace/artifacts/lang-211.

The first build unexpectedly emitted a development-certificate installation
notice; actual global state remains uninspected and controller-owned. The
current environment above is required before each invocation; it cannot prove
the historical build environment or absence of earlier side effects.
No server/trust command was run and no cleanup is performed before review.


### Correction 1: receipt boundary and current launch guards

Both Python entrypoints share one recursive duplicate-key decoder and the same
`ReceiptRefusal` validation. JSON scalar/array roots, malformed receipt/case field
types and conflicting duplicate keys refuse the whole unit with exit 1 and a
`receipt_refused:` diagnostic. No summary is published on refusal. Successful
summaries replace their destination atomically; retries invalidate the explicitly
owned prior summary. Input/output aliases are refused before invalidation or
capture writes. Preserve historical artifacts and use a fresh path for new work.

The current launcher overrides inherited T14 guards, including `DOTNET_NOLOGO=1`,
before its process invocation. CLI home and NuGet caches live under the requested
output directory's `runtime/`; `HOME` remains inherited. Build commands require
the explicit preflight above. The retained historical build helper lacks
`DOTNET_NOLOGO`; it remains unchanged evidence, not a current recommended helper.
Current guards cannot establish the old process environment.

Correction tests use Python-mocked producer launches only. They cover duplicate
keys at the root and nested objects, malformed type families, valid-to-invalid
retries, path/symlink/hard-link alias refusal, typed errors and inherited guard
overrides. Independent duplicate, stale-success, shape, guard and alias mutants
must fail at assertions. No SDK, native producer, population, consumer, holdout
or timing rerun is needed for these checks. The four authored controls remain
FEASIBILITY ONLY: two evaluated, two refused, zero canonical files measured;
the historical 91-file L2 C reading and absent L3 remain unchanged.
