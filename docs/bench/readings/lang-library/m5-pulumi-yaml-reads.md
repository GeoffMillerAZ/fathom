LL-M5 verdict: PASS

Pulumi YAML resource reads against pulumi-yaml v1.36.0 pulumiyaml.LoadYAMLBytes, on 26 preregistered public programs (117 resources, 415 reader paths). Path precision 1.0000 (bar 0.99), path recall 1.0000 (bar 0.95), class agreement 0.9952 (bar 0.98).

## Reading

| Item | Value |
|---|---|
| Oracle | pulumi-yaml v1.36.0 pulumiyaml.LoadYAMLBytes |
| Reader | `ReadPulumiYAMLFile` (`internal/iacfacts`) |
| Subjects | `calibration/oracles/lang-library/subjects/pulumi-yaml.json`, committed before any oracle or reader ran on them (selection rule inside) |
| Programs selected, graded, rejected by the oracle | 27, 26, 1 |
| Resources in oracle, in reader | 117, 117 |
| Graded paths: reader, oracle, in both | 415, 415, 415 |
| Graded class leaves, agreeing | 417, 415 |
| Reader sources (`internal/iacfacts`) read at | `c567da046`, unchanged in this change |
| sha256 prefix, raw oracle report and reader dump | `70a91c8b343097d8`, `7f65861a7a730f44` |
| Reader syntax errors reported | 2 |

## Against the bar

| Metric | Bar | Read | Count |
|---|---|---|---|
| Path precision | at least 0.99 | 1.0000 | 415 of 415 reader paths are in the oracle |
| Path recall | at least 0.95 | 1.0000 | 415 of 415 oracle paths are in the reader |
| Class agreement | at least 0.98 | 0.9952 | 415 of 417 leaves in both agree |

Agreement by reader class:

| Reader class | Graded | Agree |
|---|---|---|
| defaulted | 18 | 18 |
| expression | 60 | 60 |
| literal | 225 | 225 |
| reference | 112 | 112 |
| unknown | 2 | 0 |

## Subjects

| Repository | Pin | Programs | Paths (reader, oracle) | Leaves graded, agree |
|---|---|---|---|---|
| pulumi/examples | `f89873d4c8ab` | 12 | 351, 351 | 353, 353 |
| pulumi/pulumi-yaml | `a71c3da0ab88` | 14 | 64, 64 | 64, 62 |

## Not graded, counted

Nothing leaves the denominators on Pulumi YAML. Every reader and oracle path is graded.

## Disagreement classes

| Class | Count | One example (subject, resource, path) |
|---|---|---|
| class disagreement: reader unknown, oracle references | 2 | `pulumi/pulumi-yaml:pkg/tests/testdata/resource-properties-config/Pulumi.yaml randomPassword (whole properties)` |

## Rejected by the oracle

| Program | Reason |
|---|---|
| `pulumi/pulumi-yaml:pkg/tests/testdata/invalid-resource-object/Pulumi.yaml` | pulumi/pulumi-yaml:pkg/tests/testdata/invalid-resource-object/Pulumi.yaml:5,24-24: resources.badResource must be an object; |
## Notes

- The oracle is pulumi-yaml's own template parse (`LoadYAMLBytes`, the parse the YAML language host runs), reported by `calibration/oracles/pulumi-yaml-go -props-out`. Its classes are written in `props.go` from the README mapping: a symbol rooted in a declared variable takes the class of the variable's value, `fn::secret` takes the class of its argument, and a configuration key with a scalar default read whole is `defaultable`.
- The two class disagreements are the same shape in two programs: `properties: ${props}`, where the whole properties value is one interpolation. The reader cannot walk it and emits one `unknown` attribute at the root with an empty path, as the design requires ("absence is never silent"), while the oracle sees a reference. The mapping gives `unknown` no agreement with a reference, so both count.
- Subjects: 12 programs from the Pulumi examples repository (351 paths) and 14 graded from the pulumi-yaml repository (64 paths), of which 9 sit under `pkg/tests/testdata` as test programs. The denominator is small (415 paths, 117 resources). One selected program is a deliberately invalid fixture and the oracle's loader refuses it, so it is rejected and counted.
- Reader defects: none found, no reader change, so there is no before and after.

## Rerun

```
select_subjects.py clone --subjects subjects/pulumi-yaml.json --clones <clones>
pulumi-yaml-oracle -props-list <key-tab-file lines> -props-out <oracle.json>      # built from calibration/oracles/pulumi-yaml-go
FATHOM_READS_SUBJECTS=subjects/pulumi-yaml.json FATHOM_READS_CLONES=<clones> FATHOM_READS_OUT=<reader.json> go test ./internal/iacfacts -run TestDumpReads
grade_reads.py --tool pulumi-yaml --subjects subjects/pulumi-yaml.json --clones <clones> --oracle <oracle.json> --reader <reader.json> --json <grade.json> --md <reading.md> --row LL-M5
```

The scripts are in `calibration/oracles/lang-library`.
