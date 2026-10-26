# Azure Pipelines: syntax only through L2

Authored fixtures; no calibration source is copied into the repository.
The document engine reads a pipeline file and literal stage, job and deployment
names, plus explicit literal dependsOn references to declared names in the
same file. Dynamic expressions are text, never evaluated. The named-stage/job
content claim is checked against parsed entities so text inside a script does
not claim a file. Earlier named/glob-owned formats, workflow paths and Kubernetes
objects retain ownership. Ingest, content discovery and single-file refresh
use the same item-specific dispatch.

An independent PyYAML 6.0.3 walk is the oracle (tier C). No native Azure compiler
runs offline here. Microsoft's azure-pipelines-language-service 0.9.2 was
inspected at b36c23dba227da8077d125d33eda32d3fa64e7f2 (MIT): its yamlDefinition.ts
normalizes template paths but does not read the target or evaluate a pipeline.
Its generic document symbols are not semantic Azure entity truth. The queue's
az validation requires the hosted service, so neither source earns tier A.

The target remains L3. Template expansion, template file joins, implicit
execution order, runtime dependencies, parameters, variable/step/resource
entities and namespace qualification of repeated job names across stages
remain gaps. No L3 edge or grade is claimed before D19. Literal references
are syntax pairs at the existing file/name granularity, not proof of runtime
execution or successful stage/job binding in Azure.
