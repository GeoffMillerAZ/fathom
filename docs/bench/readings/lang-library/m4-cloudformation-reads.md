LL-M4 verdict: PASS

CloudFormation resource reads against cfn-lint 1.52.1 cfnlint.decode.decode, on 30 preregistered public templates (198 resources, 1149 reader paths). Path precision 1.0000 (bar 0.99), path recall 1.0000 (bar 0.95), class agreement 1.0000 (bar 0.98).

## Reading

| Item | Value |
|---|---|
| Oracle | cfn-lint 1.52.1 cfnlint.decode.decode |
| Reader | `ReadCloudFormationFile` (`internal/iacfacts`) |
| Subjects | `calibration/oracles/lang-library/subjects/cloudformation.json`, committed before any oracle or reader ran on them (selection rule inside) |
| Templates selected, graded, rejected by the oracle | 30, 30, 0 |
| Resources in oracle, in reader | 198, 198 |
| Graded paths: reader, oracle, in both | 1149, 1149, 1149 |
| Graded class leaves, agreeing | 1172, 1172 |
| Reader sources (`internal/iacfacts`) read at | `c567da046`, unchanged in this change |
| sha256 prefix, raw oracle report and reader dump | `11d66cfed4c9fe40`, `b26add3d313d51bd` |
| Reader syntax errors reported | 0 |

## Against the bar

| Metric | Bar | Read | Count |
|---|---|---|---|
| Path precision | at least 0.99 | 1.0000 | 1149 of 1149 reader paths are in the oracle |
| Path recall | at least 0.95 | 1.0000 | 1149 of 1149 oracle paths are in the reader |
| Class agreement | at least 0.98 | 1.0000 | 1172 of 1172 leaves in both agree |

Agreement by reader class:

| Reader class | Graded | Agree |
|---|---|---|
| defaulted | 82 | 82 |
| expression | 142 | 142 |
| literal | 708 | 708 |
| reference | 240 | 240 |

## Subjects

| Repository | Pin | Templates | Paths (reader, oracle) | Leaves graded, agree |
|---|---|---|---|---|
| aws-cloudformation/aws-cloudformation-templates | `a0f43bc6d208` | 15 | 707, 707 | 717, 717 |
| aws/serverless-application-model | `69e2756413cf` | 15 | 442, 442 | 455, 455 |

## Not graded, counted

Nothing leaves the denominators on CloudFormation. Every reader and oracle path is graded.

## Disagreement classes

None.

## Rejected by the oracle

None.
## Notes

- The oracle is cfn-lint's own decode: its loader turns each short form (`!Ref`) into the long form and yields the tree. The classes (constant, references, empty) and the leaf walk are written in `cfn_reads_oracle.py` from the README mapping, not by cfn-lint, and share no code with the reader. The CloudFormation mapping has no written counterpart in the design, which fixes it for Terraform only, so this mapping is the analogue of that paragraph (README, committed before any oracle ran).
- Subjects are 30 templates from two repositories: 15 from the CloudFormation sample templates and 15 from the SAM repository, chosen by the sha256 rule in the subject file. 14 of the SAM subjects are translator fixtures under `tests/translator` (the rule did not weigh input against output; 2 are input and 12 are output, the SAM translator's already transformed CloudFormation), so only 4 of the 198 graded resources are `AWS::Serverless::*`. cfn-lint's decode does not transform them, so those are graded on their written `Properties`.
- A parameter whose Type is an SSM parameter type counts as having no default in the oracle (its default is a path, not the value). The reader makes the same call. 82 reader `defaulted` leaves agree.
- Reader defects: none found, no reader change, so there is no before and after.
- The reader reported no syntax error and named nothing in `Dropped` on any of the 30 templates, and the oracle found no resource the reader lacks or the reader found that the oracle lacks. Constructs that make the reader emit an `unknown` attribute (a merge key, `Fn::Transform` or `Fn::ForEach` inside Properties) are not in these subjects, so this reading does not test those branches.

## Rerun

```
select_subjects.py clone --subjects subjects/cloudformation.json --clones <clones>
<cfn-lint libexec>/bin/python cfn_reads_oracle.py --subjects subjects/cloudformation.json --clones <clones> --out <oracle.json>
FATHOM_READS_SUBJECTS=subjects/cloudformation.json FATHOM_READS_CLONES=<clones> FATHOM_READS_OUT=<reader.json> go test ./internal/iacfacts -run TestDumpReads
grade_reads.py --tool cloudformation --subjects subjects/cloudformation.json --clones <clones> --oracle <oracle.json> --reader <reader.json> --json <grade.json> --md <reading.md> --row LL-M4
```

The scripts are in `calibration/oracles/lang-library`.
