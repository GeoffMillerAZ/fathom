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
The queue's az pipeline validation names a hosted service and has no installed
offline reference evaluator. No SCIP producer or native semantic dump was
identified. The PyYAML walk is not relabelled as Azure's own tool.

Measure complete sparse subject scopes in calibration/corpus.d/azure-pipelines.yaml
with `indexbench lang-oracle -dialect azure-pipelines -tool-oracle <oracle.json>`
without `-only`. Re-time with `indexbench lang-retime -runs 3` on every subject.
Literal-name syntax is measured; template execution and L3 are not.
