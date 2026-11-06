# Resource reads: oracles and grading (LL-M3, LL-M4, LL-M5)

Plan rows LL-M3, LL-M4 and LL-M5 in `docs/plans/lang-library.md`, design `docs/design/lang-library.md` section 5 (Oracles) and
section 10. The readers are `ReadTerraformFile` and `ReadTerraformModule`, `ReadCloudFormationFile` and `ReadPulumiYAMLFile` in
`internal/iacfacts`. Readings land in `docs/bench/readings/lang-library/m3-terraform-reads.md`, `m4-cloudformation-reads.md` and
`m5-pulumi-yaml-reads.md`.

## Preregistration

`subjects/<tool>.json` is committed before any oracle runs against any subject. It holds the public repositories, the pinned commits,
the written selection rule and the roots or templates the rule selected. `select_subjects.py` applies the rule and reads only file
text and paths, never reader output. Clones live outside the repository. Terraform subjects use only the providers of the oracle pin
list (`calibration/oracles/terraform/pins.json`): aws, cloudinit, helm, http, local, null, time, tls.

A subject the oracle itself cannot process (init fails offline, cfn-lint cannot decode, pulumi-yaml refuses the program) is rejected
by the oracle, and the reading counts every such rejection. The subject list is never edited after reading.

## Oracles (share no code with the readers)

| Tool | Oracle | Script |
|---|---|---|
| Terraform | `tofu show -json -config` after `tofu init -backend=false` against the pinned filesystem provider mirror, plus `tofu providers schema -json` for the schema | `tf_oracle.py` |
| CloudFormation | `cfnlint.decode.decode`, cfn-lint's own YAML and JSON loader | `cfn_reads_oracle.py` |
| Pulumi YAML | pulumi-yaml v1.36.0 template parse, the `-props-out` report of `calibration/oracles/pulumi-yaml-go` | `props.go` |

`reads_dump_test.go` in `internal/iacfacts` writes the reader side as JSON (set `FATHOM_READS_SUBJECTS` and `FATHOM_READS_OUT`).
`grade_reads.py` joins the two sides and writes the numbers.

## What is compared

One record per attribute leaf: resource key, path, class. A resource is keyed by its logical identity: Terraform `mode.type.name`,
CloudFormation logical id, Pulumi resource name.

**Paths.** The reader's spelling is the design's: nested blocks and list elements take `[*]`, mapping keys join with `.` (CloudFormation
and Pulumi), a Terraform attribute is one leaf however its value is built. A mapping or list that is empty is one leaf at its own path.
A Properties or properties value that is not a mapping is one leaf with the empty path.

**Oracle classes**, three on every tool, as the design's Terraform paragraph defines them:

| Oracle class | Terraform | CloudFormation | Pulumi YAML |
|---|---|---|---|
| constant | `constant_value` | a scalar, and a `Fn::Sub` with nothing left to substitute (no `${name}`, or only `${!x}` escapes and names its map binds to constants) | a null, boolean, number or string with no interpolation |
| references | `references` | any intrinsic (`Ref`, `Fn::*`) whose argument names a Ref, GetAtt, ImportValue or Sub variable | a symbol or interpolation, or a builtin whose arguments hold one |
| empty | `{}` (a function with no static answer) | an intrinsic that names none | a builtin that holds no symbol, and `${pulumi.*}` |

Pulumi follows a symbol rooted in a declared variable to the class of the variable's value, as the reader's documentation says it
does, and `fn::secret` and `fn::rawSecret` take the class of their argument. A symbol rooted in nothing declared is `unresolved`.
A CloudFormation pseudo parameter (`AWS::Region`) is a reference to the oracle. A path several blocks or list elements write has one oracle
leaf per occurrence, and a reader leaf agrees when some occurrence of the same path agrees with its class (one reader leaf per distinct class).
The oracle scripts were written after the first commit of this directory (the subjects) and before any oracle ran on a subject; this
paragraph and the `Fn::Sub` row above were settled in that gap.

**Mapping of reader class to oracle class, exactly the design's:**

| Reader class | Agrees with oracle class |
|---|---|
| literal | constant |
| reference | references |
| defaulted | references, and every name the leaf references is a Terraform variable (`var.x`), CloudFormation parameter or Pulumi config key that has a default (graded against `root_module.variables`, `Parameters` and `config`) |
| expression | references or empty. A conditional tofu folds also agrees with constant, and so do `!true`, `1 + 2`, a `for` over constants and the negation of anything but a number literal (Terraform only, decided on the written expression) |
| unknown | nothing, except Pulumi `unresolved` |

A reader defaulted leaf whose oracle references include a Terraform `local.*` is not graded (locals are ungraded), and is counted and
disclosed. Defaulted on CloudFormation and Pulumi is graded against the parameter and config tables the same way.

**What leaves the denominators and is counted.** Terraform only: a reader path inside a `dynamic` block (tofu drops dynamic blocks),
a path that starts with `lifecycle`, and a path outside the provider schema of the resource type (`depends_on`, and any attribute or
block the schema does not hold). Module calls are not graded and are counted. A reader resource missing from the oracle, or the other way
round, is reported as a resource miss, and its paths count in the path metrics.

**Metrics**, micro-averaged over every graded leaf of every subject:

- path precision: reader paths the oracle also has, over graded reader paths
- path recall: oracle paths the reader also has, over graded oracle paths
- class agreement: agreeing leaves over leaves both sides have

Bar: precision at least 0.99, recall at least 0.95, class agreement at least 0.98. Every disagreement class is listed with its count
and one example.

## Reader defect after a miss

A miss from a reader defect may be fixed in `read_*.go` with a test that fails without the fix. The same preregistered subjects are read
again and the before and after numbers are both reported. The subjects, the mapping and the bar are never changed after reading.
