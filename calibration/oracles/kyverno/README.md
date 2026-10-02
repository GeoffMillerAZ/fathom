# kyverno-oracle (tier C, cross-checked by `kyverno test`)

The language-breadth harness's oracle for the `kyverno` doc-engine dialect (LB-Q-kyverno, design
`docs/design/language-breadth.md` §1.3, §4 "Policy references", §10). A separate Go module (`go.mod`:
`gopkg.in/yaml.v3`), never imported by fathom, never a product dependency.

```
cd calibration/oracles/kyverno && go build -o <scratch>/kyverno-oracle . &&
  <scratch>/kyverno-oracle -subject <checkout> -kyverno <kyverno> > <scratch>/kyverno-<subject>.json
indexbench lang-policies -dialect kyverno -subject <checkout> -subject-name <name> \
    -tool-oracle <scratch>/kyverno-<subject>.json -out docs/bench/readings/lang/kyverno.json
```

## The instruments

- **The walk (tier C).** An independent `gopkg.in/yaml.v3` reading of every `.yaml`/`.yml` the subject
  holds (the directories fathom's discovery skips are skipped): a file whose every object document is a
  Kyverno policy object (`kyverno.io`, `policies.kyverno.io`) or a CLI `Test` (`cli.kyverno.io`) is a
  Kyverno file. Its definitions are the policies (`Kind/[ns/]name`, kind `policy`), the legacy rules (by
  name, kind `rule`), the PolicyExceptions (`policy`) and the Tests (`Test/name`, `test`); its references an
  exception's policyName and ruleNames (and the CEL policyRefs) and a Test's `results[].policy` and
  `results[].rule`; its cross_references a Test's `policies[]` and `exceptions[]` files (to the file's first
  entity) and an exception's policyName to the one `ClusterPolicy/<name>` or `Policy/<ns>/<name>` another
  file declares.
- **Policy targets** (`targets.go`, shared with `calibration/oracles/gator`): the match and exclude blocks
  (`any[]`, `all[]`, direct `resources`; a CEL policy's `resourceRules` and `excludeResourceRules`, the
  built-in plural resources mapped to kinds) joined to the subject's Kubernetes objects (every document with
  a kind and a name in a file with a top-level apiVersion and kind that is no policy file, Helm template,
  compose, kustomization or workflow file) by kind and group, namespace scope and label selector, nearest
  the policy first (its file, its directory's subtree, the repository), more than 256 refused (`fan-out`).
  Written as `policy_references` and `policy_unresolved` (lang-policies reads them; lang-oracle refuses
  them). The walk is written from the contract in the design, never from fathom's code.
- **Cross-check (tier A, never scored): `kyverno test`.** With `-kyverno`, each Test file is run
  (`kyverno test <dir> -f <file> -o json`, retried up to five times when no result table prints) and every
  row the engine evaluated whose Test expects `pass`, `fail` or `warn` (the rule applied to the resource) and
  whose result passed must be a pair of the walk's `policy_references`: the rule (or the CEL policy) in the
  Test's policy file to the object in its resource file. Rows of autogen (an `autogen-` rule, or a Pod
  controller row of a policy whose targets name Pod) are counted apart: Kyverno applies a Pod policy to
  controllers, the policy names Pod. Written as `policy_crosscheck`.

## Why tier C

§1.3's ladder: the kyverno CLI is the reference implementation, but offline it never lists a policy's rules
or targets: `kyverno apply` reports only the rules a given resource matched and `kyverno test` only the
results a Test declares. Neither enumerates definitions, so the walk is the oracle (good at best) and the
CLI cross-checks the part of it a run evaluates. kyverno 1.18.1 (Homebrew; the build the wave brief called
1.19.1 reports `Version: 1.18.1`), run against both subject pins.
