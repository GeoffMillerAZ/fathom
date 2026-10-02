# kyverno-oracle (tier A: definitions, references, cross-references; tier C: policy targets)

The language-breadth harness's oracle for the `kyverno` doc-engine dialect (LB-Q-kyverno, design
`docs/design/language-breadth.md` §1.3, §4 "Policy references", §10). A separate Go module (`go.mod`:
`gopkg.in/yaml.v3`; `github.com/kyverno/kyverno` v1.18.1, Apache-2.0, used offline as a library, with that
module's own `replace` of `k8s.io/pod-security-admission`), never imported by fathom, never a product
dependency.

```
cd calibration/oracles/kyverno && go build -o <scratch>/kyverno-oracle . &&
  <scratch>/kyverno-oracle -subject <checkout> -only <globs> -kyverno <kyverno> \
    -policies-out <scratch>/kyverno-<subject>-policies.json > <scratch>/kyverno-<subject>.json
indexbench lang-policies -dialect kyverno -subject <checkout> -subject-name <name> -only <globs> \
    -tool-oracle <scratch>/kyverno-<subject>.json -tool-oracle <scratch>/kyverno-<subject>-policies.json \
    -out docs/bench/readings/lang/kyverno.json
```

`-only` takes lang-policies' globs so the oracle reads the measured set fathom is measured on (the reading
names them). The kyverno CLI used for the cross-check is 1.18.1 (Homebrew, `kyverno version`: `Version:
1.18.1`); the library is the same release.

## The instruments

- **Definitions and references (tier A since wave ox, LB-Q-kyverno-rest).** `loader.go`: every claimed file
  (a file whose every object document is a Kyverno policy object or a CLI `Test`, the dialect's claim) is
  loaded by Kyverno's own CLI loading code: `cmd/cli/kubectl-kyverno/policy.Load` (kubectl-validate against
  Kyverno's CRD schemas and the built-in API, then the typed conversion `kyverno apply` and `kyverno test`
  run on) and `cmd/cli/kubectl-kyverno/test.LoadTest` (the strict decode `kyverno test` runs on). A policy of
  any kind (`Kind/[ns/]name`, `policy`), each legacy rule of `spec.rules` (`rule`), each PolicyException
  (`policy`) and each Test (`Test/name`, `test`) is an entity the loader returns, named off the typed
  object; references are an exception's `policyName` and `ruleNames` (a CEL exception's `policyRefs`) and a
  Test's `results[].policy` and `results[].rule`, read off the typed objects; cross_references a Test's
  `policies[]` and `exceptions[]` files (to the file's first loaded entity) and an exception's policyName to
  the one `ClusterPolicy/<name>` or `Policy/<name>` another file loads. `gopkg.in/yaml.v3` gives each
  entity's line, never an entity. A file the loader refuses, or a document it skips, yields nothing; stderr
  lists every refused file and counts the skipped documents.
- **Policy targets (tier C), `-policies-out`.** `targets.go` (shared with `calibration/oracles/gator`): the
  match and exclude blocks (`any[]`, `all[]`, direct `resources`; a CEL policy's `resourceRules` and
  `excludeResourceRules`, the built-in plural resources mapped to kinds) of the loaded policies and
  exceptions, joined to the subject's Kubernetes objects (every document with a kind and a name in a file
  with a top-level apiVersion and kind that is no policy file, Helm template, compose, kustomization or
  workflow file) by kind and group, namespace scope and label selector, nearest the policy first (its file,
  its directory's subtree, the repository), more than 256 refused (`fan-out`). Written from the contract in
  the design, never from fathom's code.
- **Cross-check (tier A, never scored): `kyverno test`.** With `-kyverno`, each Test file is run
  (`kyverno test <dir> -f <file> -o json`, retried up to five times when no result table prints) and every
  row the engine evaluated whose Test expects `pass`, `fail` or `warn` (the rule applied to the resource) and
  whose result passed must be a pair of the walk's `policy_references`: the rule (or the CEL policy) in the
  Test's policy file to the object in its resource file. Rows of autogen (an `autogen-` rule, or a Pod
  controller row of a policy whose targets name Pod) are counted apart (LB-X-kyverno-autogen). Written as
  `policy_crosscheck`.

## Why tier A for definitions and references, and C for the targets

§1.3's ladder puts "the format's reference parser" at A: Kyverno's CLI loading code is the reference
implementation's own reader, the code `kyverno apply` and `kyverno test` load policies and Tests with, and
every entity here is one it returns (the relabel from C cites that definition). It is stricter than a
parse: a document that fails Kyverno's CRD schema (an unknown field, an invalid value) or carries no `spec`
is refused or skipped, which a walk cannot tell. The YAML layer for lines (`gopkg.in/yaml.v3`, as the
dialect's doc engine) is a decoder detail, never an entity decision. What a policy matches is still a
rule-based walk: offline, `kyverno apply` and `kyverno test` report only the rules a given resource
matched, never a policy's targets, so the pairs stay tier C and `kyverno test` checks the part a run
evaluates.

## What the loader showed (wave ox)

On kyverno/policies main ec478cb5 and release-1.18 4422f711, every file the loader refuses or skips but one
is a chainsaw assertion fragment under a `.chainsaw-test*` directory (`policy-ready.yaml`,
`chainsaw-step-NN-assert-N.yaml`: a policy's `apiVersion`, `kind` and `metadata.name` with a `status` whose
JMESPath keys fail the schema, or no `spec`): 177 files on main, 595 on release-1.18. The dialect claims
them and fathom reads a policy definition in each (O431). The reading measures the claimed files outside
`.chainsaw-test*` plus the chainsaw files holding policies the loader loads, and discloses the full-scope
numbers in its notes. The one other refusal is `cost-optimization/schedule-based-quotas` on release-1.18
(an invalid value against the CRD schema), kept in scope: fathom's two definitions there are counted
against it.
