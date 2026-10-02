# gator-oracle (tier A; policy targets tier C)

The language-breadth harness's oracle for the `gatekeeper` doc-engine dialect (LB-Q-gatekeeper, design
`docs/design/language-breadth.md` §1.3, §4 "Policy references", §10). A separate Go module (`go.mod`:
`gopkg.in/yaml.v3`), never imported by fathom, never a product dependency.

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
- **Policy targets (tier C), `-policies-out`.** `targets.go` (shared with `calibration/oracles/kyverno`)
  joins each attested constraint's `spec.match` (kinds × apiGroups, namespaces, `labelSelector.matchLabels`)
  to the subject's Kubernetes objects, nearest the constraint first, more than 256 refused. Gator
  cross-checks it (`policy_crosscheck`, never scored): a passing case that asserts violations (`yes`, or a
  count above 0) proves the constraint matched its object.

## Why tier A, and why the targets are C

§1.3's ladder: gator is the Gatekeeper project's reference tool, run offline on the subject's own suites,
and every definition, reference and cross-reference here is one a passing gator test depends on. What a
constraint matches is evaluated by gator only through violations: an object a constraint matches without
violating it leaves no trace, so the target walk is tier C and gator checks the part it attests.
