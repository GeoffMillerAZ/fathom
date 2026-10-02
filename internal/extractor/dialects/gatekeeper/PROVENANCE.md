# gatekeeper — provenance and divergences

LB-Q-gatekeeper, design `docs/design/language-breadth.md` §4 ("Policy references") and §13; wave pc. A
doc-engine dialect (no `tags.scm`, no upstream grammar). Before it a Gatekeeper file was a Kubernetes
manifest and k8sextract.go stored each template and constraint as a `K8sWorkload`. The decode is
`gopkg.in/yaml.v3`. Fixtures are written for this dialect from the shapes in the Gatekeeper documentation and
open-policy-agent/gatekeeper-library (Apache-2.0), with reserved example names (ADR-011).

## How a file is claimed

By content: `fusion.PolicyDialect` (LB-F134) gives a `.yaml`/`.yml` to this dialect when every object
document has the API group `templates.gatekeeper.sh` or `constraints.gatekeeper.sh`, or is gator's `Suite`
(`test.gatekeeper.sh`); decided before the Kubernetes verdict in ingest and `ReingestFile`. A file mixing
them with other objects stays Kubernetes. `indexbench lang-policies` owns the measured files by the same
predicate.

## What it models

- **ConstraintTemplates** (`GatekeeperConstraintTemplate`, `policy`), named by the kind they define,
  `spec.crd.spec.names.kind` (what a constraint names), signed by `metadata.name`.
- **Targets** (`GatekeeperTarget`, `rule`): `spec.targets[*].target` (`admission.k8s.gatekeeper.sh`).
- **Constraints** (`GatekeeperConstraint`, `resource`: an instance of its template's kind), every kind of
  `constraints.gatekeeper.sh` v1alpha1, v1beta1 and v1, named `Kind/name`, signed by `enforcementAction`.
- **Suites** (`GatekeeperSuite`, `test`): gator's `Suite`, named `Suite/name`.
- **References** (`Calls`, L2): a constraint to its template's kind.
- **Cross-file references** (L3): a constraint to the template that defines its kind (`service-name`, edge
  `constraint_template`, resolved by `fusion.ResolveDocXrefs`: the one entity of that name in another file); a
  suite's `tests[].template` and `tests[].constraint` files (`repo-path`).
- **Policy targets** (`policy_ref`, LB-F135, added in Go by `fusion.policyDocExtractor`): a constraint's
  `spec.match.kinds[]` as `apiGroups × kinds` (a `""` group is the core group, a `*` or absent group any
  group; Target `Kind` or `group/Kind`), scoped by `spec.match.namespaces`, narrowed by
  `labelSelector.matchLabels`; a selector alone is a `label-selector` reference; a constraint with neither
  matches every object (one `*` reference, counted as a wildcard).

## Divergences

- **Embedded Rego is not extracted** (`spec.targets[].rego`, `libs`): it needs the native Rego extractor
  (LB-X-gatekeeper-rego, deferred until LB-Q-rego is `[x]` on main). CEL `code` blocks likewise.
- **A template declared twice resolves no constraint.** The `service-name` join takes the one entity of
  that name in another file; gatekeeper-library's `artifacthub/` tree republishes every template per
  version, so the reading is taken on its `library/` tree, and a repository that vendors a template twice
  draws no `constraint_template` edge (the edge is never guessed).
- **`excludedNamespaces`, `namespaceSelector`, `name`, `scope` and `matchExpressions` are not read**: the
  join is wider than the engine's.

## Calibration

`calibration/corpus.d/gatekeeper.yaml`: open-policy-agent/gatekeeper-library at d2b86f87, subpath
`library`. The oracle is `calibration/oracles/gator`, tier A: `gator verify` (v3.23.1, the darwin-arm64
release binary) runs every suite, and a passing test attests its template, its constraint and the pair
between them; names and lines come from `gopkg.in/yaml.v3`. The policy targets are a tier-C walk of the same
files, cross-checked by gator: a passing case that asserts violations proves its constraint matched the
object.
