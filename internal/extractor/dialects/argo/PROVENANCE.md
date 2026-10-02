# argo — provenance and divergences

LB-Q-argo, design `docs/design/language-breadth.md` §4, §13; the iac wave (EXCELLENCE T3). A doc-engine
dialect (no `tags.scm`, no upstream grammar, no incumbent). The decode is `gopkg.in/yaml.v3` through the
doc engine. Fixtures are written for this dialect; the shapes follow the Argo Workflows examples and the
Argo CD documentation (Apache-2.0).

## How a file is claimed: every document, never some

`detect.content_sniff` names `apiVersion: argoproj.io/*` AND a `kind` of Application, ApplicationSet,
AppProject, Workflow, WorkflowTemplate, ClusterWorkflowTemplate or CronWorkflow (either order); a Rollout,
a Sensor or any other argoproj.io kind stays Kubernetes. `fusion.crDispatch` (LB-F149) claims a file only
when every non-empty document passes the sniff and the classifier would call it a plain Kubernetes
manifest; a file with an Argo document beside others is MIXED and stays Kubernetes (`fusion.CRMixed`).

## What it models

- **The object** (`ArgoResource`, canonical `resource`): `Kind/<namespace>/<name>`, the Kubernetes
  dialect's naming; an object with only `generateName` (every Argo example) is named by it, where the
  Kubernetes dialect gives it no entity.
- **Templates** (`ArgoTemplate`, `task`): each `templates[]` entry (and a CronWorkflow's
  `workflowSpec.templates[]`), **steps** (`ArgoStep`, `step`) of a steps template, **dag tasks**
  (`ArgoTask`, `job`), and **container images** (`ArgoContainer`, `data_source`, from `container`,
  `script`, containerSet containers, sidecars, initContainers, an inline template's and an executor
  plugin's), found at any depth by `..`.
- **References** (`Calls`, L2, declared only): a step's or task's `template`, a task's `dependencies`,
  an object's `entrypoint` and `onExit`, each only when the document declares that template or task.
- **Cross-file references** (L3): `templateRef` (a step or task) and `workflowTemplateRef` (a Workflow,
  a CronWorkflow's `workflowSpec`) to the WorkflowTemplate (or, with `clusterScope: true`, the
  ClusterWorkflowTemplate) of that name, `service-name` over `WorkflowTemplate/<name>`; an Application's
  and an ApplicationSet template's `source.path`/`sources[].path` (`repo-root`), `source.chart`
  (`module-source`), and every image (`image-ref`, resolved only when a Dockerfile of the repository
  builds it).

## Divergences, each disclosed (and where it is a ceiling, a -rest row)

- **A namespaced template is not found.** A reference sits deep in the object and cannot read the
  object's namespace, so the target is named without one: a WorkflowTemplate that writes
  `metadata.namespace` is named `WorkflowTemplate/<ns>/<name>` and no entry reaches it. The examples and
  most repositories write none. Both forms are tried for `clusterScope` (a ClusterWorkflowTemplate has no
  namespace).
- **`depends:` and `{{tasks.x.outputs}}` are not read.** A dependency written as an expression
  (`"A && B.Succeeded"`) needs string-interpolation capture, which the engine does not have;
  `dependencies:` lists are read (LB-Q-argo-rest).
- **Application `source.path` is a path in the Application's repository.** `repoURL` may name another
  repository, and the engine cannot compare it with this one: the edge resolves when the path names a
  directory of this repository holding exactly one Argo file, which no measured subject does (the
  Applications of argo-cd's documentation name directories of other repositories). `source.chart`
  names a chart of a Helm repository and resolves to a root entity of that name, which none is.
- **`argo lint --offline` is the acceptance, not the entities.** The tool (v4.1.4, tier A) accepts 200 of
  the 212 workflow files of argo-workflows' examples and rejects 12 (counted on the oracle's stderr);
  it prints no entity and no line. An Application, ApplicationSet or AppProject has no offline tool.
  Entities, references and edges are tier C: every metric caps at good.

## Calibration

Subjects in `calibration/corpus.d/argo.yaml`; the oracle is `calibration/oracles/argo` (an independent
`gopkg.in/yaml.v3` walk, with `-argo` for the lint acceptance). argoproj/argocd-example-apps ships no
LICENSE file; the argoproj organisation's licence (Apache-2.0) is the declaration, as the kustomize shard
records it (a licence doubt, flagged in the report).
