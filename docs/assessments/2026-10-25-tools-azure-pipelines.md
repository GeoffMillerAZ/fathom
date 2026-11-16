# Azure Pipelines: L2 syntax, native-evaluation ceiling

T-R4 permits this L1/L2 slice. D19 has since landed; the evaluated L3 evidence below remains missing. The queue's L3 target
is preserved; no plan checkbox is claimed. The new independent tier-C oracle
and recipe reading cover all 91 files ingest claims in the pinned sparse
subjects, without `-only`: 15 in microsoft/azure-pipelines-yaml and 76 in
runtime's eng/pipelines. No calibration source was copied into fixtures.

From no reading to L2 good: 227/228 definitions (recall .9956, precision 1),
16/16 explicit dependency pairs (precision/recall 1), zero faults, median
3.15 MB/CPU-s from 3.15, 3.20, 2.53 after dispatch ordering was corrected. The single definition miss is the earlier
Build stage under duplicate `extends` keys in
eng/pipelines/coreclr/libraries-gcstress0x3-gcstress0xc.yml: the document decoder
overwrites the earlier map value while the independent parser sees both syntax
occurrences. This is disclosed, not removed from truth or fixed in the shared
engine (T-R2).

The implementation uses the existing document engine and item-specific dispatch.
Actual chart-owned Helm templates, Helm basenames, existing named CI formats,
Kubernetes objects and workflow paths retain ownership. A broad Helm manifest
glob alone is not a chart claim; the existing chart-directory classifier makes
that distinction. Regression tests cover named and content-discovered Azure
files, script text, earlier claims, ingest and single-file refresh.

Existing-work evidence and the tier ceiling live in the oracle README and
dialect provenance. Microsoft's inspected editor library computes template URIs
and generic YAML outlines, not the semantic evaluation the native tier requires.
PyYAML is independent of production's Go YAML decoder; it is deliberately C.
Literal names are syntax: neither runtime bindings, parameter/variable/step
entities, implicit execution order nor template expansion is proven. L3 remains
unmeasured and held. The dispatch ordering correction is an ENGINE change; no generated board was changed.

The extractor epoch gets its own tools-azure-pipelines bump file. Integration
must regenerate the frozen language/support surfaces and run its gate; this lane
does not write generated artifacts. No model decision applies to syntax and
identity checks.

The X-R1 candidate audit names the editor library, the official agent's legacy
parser at be18ae23, hosted Preview API 7.1, PyYAML 6.0.3 and the product locator.
The legacy parser's exhaustive source switch rejects modern stages/jobs/extends.
The native hosted compiler exists, but needs owner-supplied organization,
project, pipeline and authentication context; it was not called. Those concrete
limits replace any inference from an unsuccessful search for an offline tool.

The binding review found that the first dispatch position could claim content-discovered
OpenAPI and CloudFormation files. Azure now runs only in the unclassified branch,
after every existing claimant. Ingest and refresh both enforce that condition.
Full extractor and fusion race suites pass. The previous 6.96 MB/CPU-s reading
is replaced by the identity-checked late-dispatch reading above; it still clears
the 1 MB/CPU-s minimum.

`cross-item-regrade-r4.json` records baseline/candidate canonical replay summaries
for all four OpenAPI and four CloudFormation subjects with identical truth.
OpenAPI: 3372/3372 definitions, 3805/3805 references, 25/25 cross-file pairs;
CloudFormation: 8857/8857, 8093/8093, 427/427. Every metric, tier, grade and
excluded-kind count agrees. OpenAPI's independent swagger-parser 13.1.0 is C
under P-T1 in both runs, rather than carrying forward its historical A label.
The CloudFormation historical 402/427 JavaScript gap is already absent in the
fresh baseline; this branch claims no improvement from it. These are binding
regression replays, not a program L3 regrade or replacement of other items'
canonical readings while D19 is held.

`yaml-dispatch-r4.json` covers all 145 existing YAML fixture files across 30
fixture dialect directories: source, classification, skip/error outcome and
normalized extracted-node/call digests are identical. No prior claim changes
and no new Azure claims occur. This equivalence uses fathom on both sides and
provides regression evidence only, with no oracle-tier claim. The committed
snapshot program and comparison driver reproduce it; source bodies and the
full per-file inventories stay in scratch. The two aggregate receipts retain
counts, subject pins and hashes only.

The own reading includes 91 file entities selected by a shared file rule;
independent stage/job/deployment declarations account for 136/137 matches.
All 16 literal dependency pairs come from Microsoft's subject; runtime has
none. PyYAML rejects one mustache starter. Shared selection, single-subject
reference diversity and the syntax-only evidence limit the support claim.


## Issue 211: offline evaluation exists; native L3 remains unmeasured (2026-11-15)

A real offline template evaluator was found and executed. The earlier implication
that service context is the only available route is withdrawn. The
[probe receipt](../../calibration/oracles/azure-pipelines/offline-probe-211.json)
binds runner.server v3.14.0, source 069646146c90d649c74dfd7a34569c9420195838
(MIT), local .NET SDK 8.0.428 and the authored driver. Its existing
`AzureDevops.ReadTemplate` and `Pipeline.Parse` evaluate a parameterized stage
template; the driver adds no resolver or syntax-walk oracle. HTTP/HTTPS server,
Runner.Client, agents, jobs and Docker entrypoints were never invoked.

The short-lived SDK API process ran under macOS sandbox-exec `deny network*`,
with a 30-second timeout and 768 MiB GC heap limit. The limits are declared,
not cap+1 measurements. All four authored controls are retained: two evaluated
and two explicitly refused, zero unexpected failures. A stage parameter expands
to Build and then Changed; Publish's dependency changes correspondingly, while
Compile/Release jobs remain equal. Malformed YAML and a missing local template
both produce TemplateValidationException refusals. Other exceptions fail the
whole process. All file contents come from an in-memory provider without a
network/cache fallback. No subject scripts execute. Raw stdout/stderr and hashes
are retained locally. The control receipt rejects incomplete or duplicate case
populations; failed controls cannot become an empty successful measurement.

This is **feasibility only**, not support evidence or an L3 row claim (T13).
Zero canonical files were measured. P-T1 keeps ordinary third-party analysis at
C; the source audit found no format-owner attestation of complete entity and
dependency truth in this producer. Its pinned `ValidateYamlFormat` tests generally
assert success/exception and do not compare finalYaml/entity-edge truth against
Azure service output. Authored expectations establish these controls only, not
owner equivalence. No native metric, grade, threshold, required kind, canonical
reading or corpus membership changes. The existing 91-file L2 C-good reading
remains: 227/228 definitions, 16/16 explicit syntax pairs, L3 absent. Its historical
HEAD-named subject refs are retained, not reused as a new release measurement.
Recipe §13.3 requires preregistered release cohorts for a real new reading; there
is no HEAD exception or new support claim from the authored population.

### Refreshed five-source audit

All searches use public unauthenticated sources; no live organization or PAT
request was sent. Exact URLs/statuses/digests are in the receipt and retained
source captures. A search miss or access gap is not proof of absence.

| Source group | Current evidence and disposition |
|---|---|
| Owner toolchain | [Preview 7.1](https://learn.microsoft.com/en-us/rest/api/azure/devops/pipelines/preview/preview?view=azure-devops-rest-7.1) returns finalYaml but requires live organization/project/pipeline context: W-R13 REFUSED, never called. [Agent v4.264.2](https://github.com/microsoft/azure-pipelines-agent/tree/cf615ecf440fb7250650f2742e382bdad1651a8b), MIT, ProcessConverter still accepts legacy phases/steps rather than modern stages/jobs/extends: source-audited boundary, not runtime failure. |
| SCIP/LSIF | Sourcegraph SCIP README and lsif.dev were retrieved; GitHub `scip-azure-pipelines in:name` returned zero, incomplete_results false. The old sourcegraph/lsif-indexer-action/master README returned 404: disclosed access gap. No applicable producer was established by these bounded searches. |
| Language servers/forks | Microsoft server/service npm 0.9.3, MIT, pinned prior definition source plus published yamlDefinition.js reviewed: returns template URIs rather than evaluated target entities. Fork search and coolhome fork d426b9b3e872823f80e3ced215f5fccaa6359c88 README inspected; no runtime or owner attestation established. |
| Registry/evaluators | npm broad parser query and exact package records, NuGet Runner.Client versions, and runner.server v3.14.0 examined. The SDK API is actually evaluated as above; the normal client starts services and is refused. Its source validation tests are not owner-equivalence attestations. |
| Runtime tracing | Microsoft diagnostic-log docs describe executed jobs/tasks, not complete written template/entity dependencies; service routes remain refused. Local runner and template-navigation candidates below were checked without executing pipelines. |

Named additional candidates (versions/primary source pins in the receipt):

- [qSharpy/azure-templates-navigator](https://github.com/qSharpy/azure-templates-navigator/tree/2e4ec367588e25663f67d8def570646532d4d462), MIT: existing local template graphs and resolution, README and engine/parity tests inspected. Parity test compares editor settings, not Azure evaluation. Runtime untried; default shared cache/credential fetch is outside scope. No absence or runtime-failure claim.
- [amalabey/azp-local-runner](https://github.com/amalabey/azp-local-runner/tree/a8411989eb3810e46f700d8721cd8cdb2c9a8d2d), MIT: live organization/PAT/Docker route REFUSED; not attempted.
- [fredrikn/AzurePipelineRunner](https://github.com/fredrikn/AzurePipelineRunner/tree/4e0b1ac133ad75e830600e5aba1cbd3fdf433be6), Apache-2.0: README describes running tasks; source interface/complete dependency attestation not established, runtime untried.
- [rodmtl/ado-pipelines-local-runner](https://github.com/rodmtl/ado-pipelines-local-runner/tree/d3b76d0f3b955a20090a194f660bcb663d5cc7b8), MIT: README advertises local template expansion/validation; source attestation and runtime untried. This is a remaining candidate, not evidence of no evaluator.

### Limits, proof and retry

The concrete missing evidence is a format-owner native output or independently
attested complete template/entity/dependency producer on the preregistered release
population, including expressions, namespaces, implicit edges, failed inputs and
all required kinds. D19 and the existence of an offline evaluator are already
satisfied; neither is retained as an unmet trigger. Do not substitute generic YAML
parsing, a selected subset, or these four authored controls for that measurement.

Six receipt tests cover exact controls, unknown fields, no support inference,
missing/duplicate/failed cases, changed fact/entity/dependency outputs, malformed
JSON and 1 MiB+1 post-capture receipt parse refusal without output. Runtime control
failures and untried/source-audited/refused candidates remain separate. Reproduction
and scoped verification commands are in the [oracle README](../../calibration/oracles/azure-pipelines/README.md#issue-211-offline-sdk-probe).
No full gate, daemon or generated shared surfaces were used; no boxes flipped.

The isolated build's first SDK run unexpectedly reported installing an ASP.NET
HTTPS development certificate despite a lane-local CLI home. Actual global
certificate state was not inspected. No trust command was run. Subsequent commands
explicitly disable certificate generation; controller review/remediation is a
separate helper-owned incident request. Do not infer a clean global state from
successful offline execution. Upstream build warnings are retained with the log;
no dependency was updated to suppress them. All downloaded tools/source and raw
receipts remain ignored for controller-owned fresh review and cleanup afterward.

Mutation: offline_probe_211.py's population, classification and exact output
pairing mutants fail the six-test suite with respectively one, one and three
failures; restored six-test run passes. Caps: test_cli_cap_plus_one_and_malformed_json_refuse_without_output
supplies 1,048,577 bytes against the 1,048,576-byte receipt parsing limit and
malformed JSON, each rejecting the whole receipt with no summary file. The
actual public-dispatch race test and strict top-level Reading round-trip passed;
no nested Reading was touched. Current canonical syntax metrics are historical
and explicitly not claimed remeasured by this runtime feasibility probe.


### Correction 1: receipt refusal and containment, 2026-11-16

The receipt boundary previously let JSON's last duplicate key win, assumed
object/string shapes before validating them, and left an old summary on a failed
retry. Both entrypoints now use one recursive duplicate-key decoder and typed
whole-unit `ReceiptRefusal`. Invalid JSON or field types produce exit 1 and a
stable diagnostic, without a traceback or success summary. An explicitly owned
summary is invalidated before parsing/capture, after refusing any input/output
alias; successful serialization replaces it atomically. Tests cover path,
symlink and hard-link input preservation and valid-to-invalid retries.

The runtime environment now overrides inherited `DOTNET_NOLOGO=0` with `1`
alongside the other T14 guards before each invocation. CLI home and NuGet caches
are confined under the requested output's `runtime/`, with `HOME` unchanged.
The README's current build preflight includes `DOTNET_NOLOGO=1`. The retained
historical build helper omits this guard and is deliberately immutable. Its old
process environment was not captured; new guards cannot prove that environment
or disprove prior side effects. The certificate remains LEAVE UNCHANGED; only
previously recorded metadata was consulted, with no new certificate, key or store
query, trust change or cleanup.

The correction's final 18-test Python suite fails against exact predecessor
464c0a3f2 at 154 assertions (zero test errors), then passes with the correction.
Five separate duplicate-key, stale-success, shape, SDK-guard and alias mutants
fail at respectively 4, 8, 13, 1 and 6 assertions; restored tests pass. Every SDK
launch in these checks is mocked. The earlier six-test mutation/proof account
above describes the retained original run, not these correction controls.

Scoped race verification selects the already-landed
`TestCommittedReadingsAreCanonical`, `TestRequiredKindsThroughCanonical` and
`TestCommittedReadingsRoundTrip` under recipe 13.5.1, plus
`TestAzurePipeline211PublicDispatch`; all four passed with `-race -count=1`.
Release-note and merge-debris checks also passed. Receipts remain in the correction's ignored
workspace. These tests check committed serialization/kinds and public dispatch;
they do not rerun or promote an oracle population. No original native proof,
collector, capture, raw JSON, canonical fixture or plan row is changed. The four
AUTHORED controls remain FEASIBILITY ONLY (two evaluated, two refused), with
zero canonical files measured; the historical 91-file L2 C reading and absent
L3 remain exactly as recorded. Independent fresh critic review is parent-owned.
