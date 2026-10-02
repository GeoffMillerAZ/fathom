# gator-oracle (tier A, policy targets included)

The language-breadth harness's oracle for the `gatekeeper` doc-engine dialect (LB-Q-gatekeeper, design
`docs/design/language-breadth.md` §1.3, §4 "Policy references", §10). A separate Go module (`go.mod`:
`gopkg.in/yaml.v3`, `github.com/open-policy-agent/gatekeeper/v3` v3.23.1, Apache-2.0, used offline as a
library), never imported by fathom, never a product dependency.

```
gh release download v3.23.1 -R open-policy-agent/gatekeeper -p 'gator-v3.23.1-darwin-arm64.tar.gz'   # sha256 dc96df4a…ed99b
cd calibration/oracles/gator && go build -o <scratch>/gator-oracle . &&
  <scratch>/gator-oracle -subject <checkout>/library -gator <gator> \
    -files-out <scratch>/gator-files.txt -policies-out <scratch>/gator-policies.json > <scratch>/gator.json
indexbench lang-policies -dialect gatekeeper -subject <checkout>/library -subject-name <name> \
    -tool-oracle <scratch>/gator.json -tool-oracle <scratch>/gator-policies.json \
    -out docs/bench/readings/lang/gatekeeper.json
```

## The instruments

- **Acceptance and the template-constraint pair: gator itself (tier A).** `gator verify <suite> -v`
  (Gatekeeper's own CLI, v3.23.1 release binary for darwin-arm64, OPA v1.17.1) runs each suite's tests: a
  test passes only when its ConstraintTemplate compiles and its constraint is an instance of the kind the
  template defines (gator refuses a constraint whose kind no loaded template defines). A passing test
  attests its template file, its constraint file and the pair; a failing one contributes nothing.
- **Names and lines: `gopkg.in/yaml.v3`.** From the attested files: the template, named by
  `spec.crd.spec.names.kind` (`policy`), its `spec.targets[].target` (`rule`), each constraint `Kind/name`
  (`resource`), each suite with a passing test `Suite/name` (`test`); references a constraint to its kind;
  cross_references a constraint to its template and a suite to the template and constraint files a passing
  test names (each file's first entity). `-files-out` lists the attested files and their cases' objects.
- **Policy targets (tier A since wave ox, LB-Q-gatekeeper-rest), `-policies-out`.** `engine.go`: which
  subject objects an attested constraint matches is decided by Gatekeeper's own match code, linked as a Go
  library at the release gator v3.23.1 is built from: `pkg/target`'s `K8sValidationTarget.ToMatcher(constraint)`,
  then the matcher's `Match` over a review of every object (`HandleReview`), with the subject's Namespace
  objects in the target's own cache (`ProcessData`, `GetCache().Add`). That is the evaluation audit and gator
  run: kinds × apiGroups (`""` the core group only, never "any group"), scope, namespaces, excludedNamespaces,
  labelSelector (matchExpressions included), namespaceSelector, name and source. Nothing here reads
  `spec.match` to decide a match. Objects are read as written, as gator reads them: an object that declares
  no namespace is not disqualified by a `namespaces` list (`match.namespacesMatch`); the oracle counts the
  verdicts that would change if it were placed in `default` (stderr and the cross-check note).
- **What stays the dialect's, mirrored and said.** The object set (`targets.go` `objectsOf`, the kubernetes
  dialect's object documents); the target split (each kinds entry's kind × apiGroup is one target, as the
  dialect emits one `policy_ref` per cell; each cell is handed to Gatekeeper as a one-entry kinds list, and
  the union of the cells is exactly the constraint's kinds match); the refusals (no kinds and no matchLabels,
  or a `*` kind without matchLabels: `wildcard`; a `{{ }}` or `${}` value: `variable`; no match:
  `undeclared`); and the join (design §4: the nearest matched objects, the constraint's file, else its
  directory's subtree, else the repository, more than 256 refused). These are the dialect's definition of
  which engine-matched pairs it reports, not a match decision, as envoy's identity rules are (envoy README).
- **Cross-check (never scored).** gator verify: a passing case that asserts violations (`yes`, or a count
  above 0) proves the constraint matched its object. The note splits the disagreements: pairs Gatekeeper's
  `Match` matches that the nearest-first join drops for a nearer object, and constraints with no
  `spec.match` (Gatekeeper applies them to every object; the dialect refuses the wildcard).
- `targets.go` is shared with `calibration/oracles/kyverno` (byte-identical); the gator oracle no longer
  calls its `resolveTargets` walk.

## Why tier A

§1.3's ladder: gator is the Gatekeeper project's reference tool, run offline on the subject's own suites,
and every definition, reference and cross-reference here is one a passing gator test depends on. The policy
pairs meet §1.3's "the tool's own evaluation" (the definition the relabel from C cites): the match verdict is
Gatekeeper's `pkg/target` code, the same `ToMatcher`/`Match` the audit controller and gator call, linked
from the gatekeeper/v3 module at v3.23.1 (gator's release), not a reading of `spec.match`. What a
constraint matches without violating left no trace in gator's output, which is why the walk was tier C
until wave ox; the library evaluates every object, violating or not. The YAML layer (`gopkg.in/yaml.v3`,
names and lines; objects decoded to `unstructured` through `encoding/json`) is a decoder detail, never a
match decision. Measured on gatekeeper-library d2b86f87 `library/`: 608 pairs, 3 of which fathom does not
report (`NoUpdateServiceAccount/no-update-kube-system-service-account`, `namespaces: [kube-system]`, to
three StatefulSets that declare no namespace: Gatekeeper cannot disqualify them, fathom's object node is
placed in `default`; O430).
