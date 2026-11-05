IF-C6 verdict: CEILING. CloudFormation syntax understanding reads L3 good, not excellent, and no oracle that exists today can lift it. The 25 missed Node handlers are fixed and re-measured (cross-file recall .9415 to 1.0, 427 of 427), but the family that carries the cross-file edges is graded against tier-C truth, which caps L3 at good. L1 and L2 stay excellent.

## Plain-language summary

- The O721 gap is closed. Wave lf3 (LB-F384, 0f4244a8) made the JavaScript dialect name `module.exports.x =`, `exports.x =` and call-wrapped exports as functions, so a Node handler binds. Nothing was left to fix in the dialect. What was stale was the measurement: the reading still said 402 of 427. Re-measured at c5e6b7a6d, all four subjects: 427 of 427, precision 1.
- The grade does not move. L3 is the cross-file level. Its evidence is 427 edges: 20 nested templates, 3 identity joins and 404 Lambda handler bindings. The truth for all of them is written by this repository's adapter (`calibration/oracles/cfn-lint/cfn_oracle.py`), not reported by a tool. A tier-C family caps at good (design 1.3). Recall of 1.0 against a tier-C truth proves agreement with our own reading of the rules, not correctness.
- Searched for a tier-A oracle for the cross-file edges (table below). The closest is the AWS SAM CLI: its own stack provider reproduces the nested edges (20 of 20) and its function provider reports every handler function with a CodeUri that holds the file the truth names (404 of 404). It does not resolve a handler to a file and symbol, and says nothing on the identity joins, so it confirms part of the truth and lifts nothing.
- JavaScript did not regress. Re-measured on its L4 holdout: L1 to L3 excellent, L4 good, as committed. def_mapping moved .9886 to 1 and typed_recall .9376 to .9481 from other work since 753ca697. The committed JavaScript reading is unchanged.

## Measurement (c5e6b7a6d, 2026-11-05)

Command: `calibration/clone.sh cloudformation`, then for each subject `<cfn-lint libexec>/bin/python calibration/oracles/cfn-lint/cfn_oracle.py -subject <dir> -out ... -xref-out ...` and `indexbench lang-oracle -dialect cloudformation -subject <dir> -subject-sha <pin> -tool-oracle <out> -tool-oracle <xref>`, subjects at the pins in `calibration/corpus.d/cloudformation.yaml`. `task lang:loop` cannot run this item: it has no per-subject sha for the two subpath subjects.

| Subject | xref truth | found | fathom edges in truth | before (c3) |
|---|---|---|---|---|
| aws-cloudformation/aws-cloudformation-templates | 20 | 20 | 20 of 20 | 20 of 20 |
| aws/serverless-application-model | 0 | 0 | 0 | 0 |
| aws-samples/serverless-patterns | 398 | 398 | 398 of 398 | 373 of 398 |
| aws-powertools/powertools-lambda-python | 9 | 9 | 9 of 9 | 9 of 9 |
| Pooled | 427 | 427 | 427 of 427 | 402 of 427 |

Definitions (8,857) and references (8,093) are unchanged and tier A. Throughput 7.42 MB/s per CPU, 0 faults, slowest file 7 ms.

## Search for a tier-A cross-file oracle (recipe 13.0.1), 2026-11-04

| Source | Query or command | Candidate and result |
|---|---|---|
| Language toolchain | `cfn-lint 1.52.1` (installed) | Its graph is within one template. Rule `NestedStackParameters` opens a local `TemplateURL` child to check parameters and writes no edge. Nothing on handlers or metric joins. Cannot grade: no edge output. |
| Language toolchain | `sam --version` 1.171.0 in a scratch venv, `calibration/oracles/cfn-lint/sam_probe.py` | Measured. `SamLocalStackProvider.get_stacks` resolves 20 of 20 nested templates. `SamFunctionProvider` reports 404 of 404 handler functions, each CodeUri holding the file the truth names. Not a handler-to-symbol resolver, no identity joins. Partial confirmation only. |
| Language toolchain | `aws cloudformation package help` | Rewrites local artifact paths and uploads them to S3. Needs credentials and a bucket. Untried: not offline. |
| SCIP and LSIF | `gh search repos` for `scip-cloudformation`, `lsif cloudformation`, `scip-yaml`, `scip cloudformation indexer`; the sourcegraph/scip README indexer list | Four empty results. The README lists no YAML or CloudFormation indexer. Unavailable. |
| Language servers | `cfn-lsp-extra 0.7.4` (pip), `@aws/cloudformation-languageserver` (npm, README read) | Both offer go-to-definition for Ref, GetAtt and parameters within a template. No nested template, CodeUri or Handler handling (grep of cfn-lsp-extra found none). Out of scope for cross-file edges. |
| Package registries | npm: `cfn-include` 1.5.0, `@aws-cdk/cloudformation-include` 1.204.0, `aws-cdk-lib` 2.275.0 (`CfnInclude`), `cfn-resolver-lib` 1.1.8; pip: `cfn-flip` 1.3.0, `cfn-tools` 0.1.6, `cloudformation-cli` 0.2.41 | `cfn-include` is a preprocessor (macro includes). `CfnInclude` loads a nested template only from a caller-written `nestedStacks` map, so the caller supplies the edge. `cfn-resolver-lib` resolves intrinsics. `cloudformation-cli` is for resource providers. None reads a handler. Untried beyond descriptions: none claims the edges. |
| Runtime tracing | `sam local invoke`, or importing each handler module under its runtime | Untried. Needs a container runtime, each pattern's dependencies installed and execution of public sample code. The first two are heavy for a bench host, and the third is not an oracle fathom should run unattended. |

## Ceiling and trigger

Ceiling: L3 good for CloudFormation. The 404 handler bindings and 3 identity joins have no tier-A oracle; the 20 nested edges have one (the SAM CLI) but a family grades at its weakest member.

Revisit when any of these holds: a maintained indexer or tool that reports a handler's file and symbol for a template (SCIP, LSIF or a tool's own resolution) exists; the SAM CLI or cfn-lint gains a resolved-edge output for functions; or a trace of handlers under their runtimes runs offline inside a cleared CPU and memory budget. None holds today. Splitting the family so nested edges grade at tier A would raise the level only for a template-nesting claim, not for the handler claim that most users ask about, so it is not taken.

## Not done

- The committed JavaScript reading is not rewritten: L4 stays good, and its notes cite a state that the numbers above supersede. A later pass can re-commit it.
- No change to `cfn_oracle.py`, the harness or the dialect. Truth independence and the tier logic are as measured in wave c3.
