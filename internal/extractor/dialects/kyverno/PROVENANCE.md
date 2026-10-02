# kyverno — provenance and divergences

LB-Q-kyverno, design `docs/design/language-breadth.md` §4 ("Policy references") and §13; wave pc. A
doc-engine dialect (no `tags.scm`, no upstream grammar). Before it a Kyverno file was a Kubernetes manifest
and k8sextract.go stored each policy as a `K8sWorkload` (`ClusterPolicy/<name>`); the dialect keeps that
`Kind/<namespace>/<name>` naming. The decode is `gopkg.in/yaml.v3`. Fixtures are written for this dialect
from the shapes in Kyverno's documentation and the kyverno/policies samples (Apache-2.0), with reserved
example names (ADR-011).

## How a file is claimed

By content: `fusion.PolicyDialect` (LB-F134) gives a `.yaml`/`.yml` to this dialect when every document that
is an object (a top-level `apiVersion` or `kind`) has the API group `kyverno.io` or `policies.kyverno.io`, or
is the Kyverno CLI's `Test` (`cli.kyverno.io`). The decision is taken before the Kubernetes verdict, in
ingest (`resolveExtractor`) and in `ReingestFile`. A file mixing a policy with other objects (the Deployments it
governs, a chainsaw test, a CLI `Values` file) stays Kubernetes, as it was: claiming it would drop the other
documents' symbols. The manifest names no extension, basename or glob (a `.yaml` is Kubernetes's), so
`indexbench lang-policies` owns the measured files by the same predicate.

## What it models

- **Policies** (`KyvernoPolicy`, `policy`): `ClusterPolicy`, `Policy`, the cleanup policies and the CEL kinds
  of `policies.kyverno.io` (`ValidatingPolicy`, `MutatingPolicy`, `GeneratingPolicy`, `DeletingPolicy`,
  `ImageValidatingPolicy` and their `Namespaced*` forms), named `Kind/[ns/]name`, signed by the apiVersion.
- **Rules** (`KyvernoRule`, `rule`): a `ClusterPolicy`'s or `Policy`'s `spec.rules[*]`, by `name`.
- **Exceptions** (`KyvernoPolicyException`, `policy`): `PolicyException` of either group.
- **CLI tests** (`KyvernoTest`, `test`): `cli.kyverno.io` `Test`, named `Test/<name>`.
- **References** (`Calls`, L2): an exception's `exceptions[].policyName` and `ruleNames[]` (and the CEL
  form's `policyRefs[].name`); a Test's `results[].policy` and `results[].rule`, as written.
- **Cross-file references** (L3, not the target): a Test's `policies[]` and `exceptions[]` files
  (`repo-path`); an exception's policyName to `ClusterPolicy/<name>` or `Policy/<ns>/<name>` (`service-name`,
  edge `exception_of`).
- **Policy targets** (`policy_ref`, LB-F135): added in Go by the dispatched extractor
  (`fusion.policyDocExtractor`), since the doc spec cannot normalize a kind or map a plural. A legacy rule's
  `match` and `exclude` (`any[]`, `all[]` and the direct `resources` form) give one `resource-kind` reference
  per `resources.kinds[]` entry from the rule — Raw as written, Target `Kind` or `group/Kind`
  (`apps/v1/Deployment` is `apps/Deployment`, `Pod/exec` is `Pod`, `*` stays `*`), Scope the block's
  `namespaces` (a namespaced kind's own namespace), Labels the block's `selector.matchLabels` narrowing it; a
  selector with no kind (or with `*`) is a `label-selector` reference. A cleanup policy's and an exception's
  own `match` give the same from the policy entity. A CEL policy's `matchConstraints.resourceRules` and
  `excludeResourceRules` give `apiGroups × resources`, each plural mapped to its kind through the built-in
  Kubernetes resources (a custom resource's plural is Target "", counted `unmapped`), narrowed by
  `objectSelector.matchLabels`.

## Divergences

- **Rules are named alone.** A rule's name is unique in its policy, not in its file; the doc engine reads
  downward from an entity and cannot qualify a rule by its policy. Two policies of one file with a rule of
  the same name share a symbol key.
- **Autogen is not modelled.** Kyverno applies a Pod rule to Pod controllers (an `autogen-` rule, or a CEL
  policy's controller rows); the reference is the kind the policy names, so a Pod policy joins Pods only.
  The `kyverno test` cross-check counts those rows apart.
- **Match filters beyond kinds, namespaces and matchLabels are not read** (`names`, `annotations`,
  `operations`, `matchExpressions`, `namespaceSelector`, `subjects`, `roles`): the join is wider than the
  engine's, never narrower than the kinds named.
- **Preconditions, context and variables are CEL or JMESPath**; a value with `{{ }}` is a variable.
- **JSON policies** are not claimed (`.yaml`/`.yml` only).

## Calibration

`calibration/corpus.d/kyverno.yaml`: kyverno/policies at the brief's pin ec478cb5 (main, CEL policies only:
upstream removed the legacy samples in 36340047) and at release-1.18's head 4422f711 (the legacy
`ClusterPolicy`, `Policy`, `PolicyException` and `CleanupPolicy` samples). The oracle is
`calibration/oracles/kyverno`: a tier-C `gopkg.in/yaml.v3` walk (the kyverno CLI lists no policy's rules
offline: `apply` reports only matched rules, `test` only declared results), cross-checked by
`kyverno test` (tier A, kyverno 1.18.1: the brew build the brief named 1.19.1 is 1.18.1), never scored.
