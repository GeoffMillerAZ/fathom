# COV-1 dependency completeness census

Status: independently reproduced replacement baseline under ESCALATE-19.
The clean rerun matched the development and allowed holdout aggregate
bytes and the sealed closure receipt. Every reported count and
denominator was confirmed, and all three pooled B3 checks passed.
COV-D11 stops the TypeScript and JavaScript levers. The holdout found
259 TypeScript external claims across 222 complete files and 14
JavaScript inconclusive bindings across 14 complete files. A controlled
diagnostic found a TypeScript product defect and a JavaScript oracle
root-containment defect. Neither grader nor product changed for this reading.

The preregistration in `docs/design/dependency-edges.md` fixes COV-D1 through
COV-D11. The original DEP-M1 and DEP-M2 populations remain development inputs.
The COV-0 pins fix the sealed holdout. No product reader, oracle semantics,
admission bar or public dependency model changes in this lane.

## Accounting protocol

B1 and B2 use `depbench.Score`. Zero complete files and complete files without
unique native truth are inconclusive, separately from positive missing or wrong
edges. The independent repository inventory also refutes complete external claims
that name a repository package or module, including files with native truth.
Native capture precedes construction of the product reader.

Every unknown contributes one occurrence. Distinct-file counts deduplicate a
bucket within each file. A file's blocking set is a conjunction: closing just one
of two buckets does not unlock it. Oracle coverage and gaps are reported alongside
reader buckets. Source forms without exact evidence remain `unknown-shape`.
Identity-only means every unknown has a corresponding external edge with a known
identity and no version. No other unknown code qualifies by inference.

The greedy table measures each marginal unlock after all previous buckets close.
A prerequisite is projected only when a remaining blocking set within the
five-bucket cap reaches the 2% development-file bar. At most five buckets
are projected. This mechanical shortlist does not authorize an oracle extension or a reader change.

Closure starts at each selected file and traverses all local reader edges,
including configuration references. Targets beyond the selection are read with
the unchanged reader against the same tracked repository view. Directory targets
expand to regular tracked files directly in that directory, not descendants.
Cycles are visited once. Configuration and other files without a measured reader
block closure. No config-file exemption or trust opt-in is used. Report closures
blocked solely by unmeasured files separately from measured and mixed blockers.
The denominator remains selected files, never all reached targets.

Holdout per-file observations and truth stay in memory. Compiler traces and file
lists exist only in a private transient directory and are removed on exit. The
successful holdout write exposes only the COV-D2 allowlist and retains COV-D3
aggregate counts separately for the final measurement. The replacement reached
both writes; the earlier aborted campaign did not.
The repeat and relocation are part of this baseline campaign. The final holdout
reading is not performed here.

## Reproduction

Source the lane environment before each command. Prepare fresh detached clean
clones from the pinned local caches using
`python3.13 -I -B -S calibration/oracles/dep-coverage/cov_run.py prepare`.
The thin runner's `--help` lists the required destination and source arguments.
No subject dependency installation or network fetch is needed.

## Baseline provenance

Harness commit `bd043bd69cf5be4758641d05b9fec0a060d5e7a7`. Reader source remains COV-0 commit `884c77754d17a8f09129d3fd5b0955bef0db9a81`. No reader edits or oracle extensions.

Aggregate artifact: `cov-census-replacement/reading/aggregate.json`, SHA-256 `d8d68af4735a00a6711b92247fb8bac7f3fb8b6e244567c95d0f61960523b58a`. Reproduction commands are in the lane handback's `workspace/tmp/REPRO.md`. The aggregate includes every development file's blocking set. The three `development-*-observations.json` companions retain development evidence only. They are byte-identical to the earlier campaign's development captures. The holdout closure baseline is retained separately as a private aggregate in `sealed-closure.json`, outside reader-lane records.

Controls: leased `go test -race ./pkg/lang/deps ./pkg/lang/deps/internal/depbench -count=1 -v`, with the pinned TSJS control enabled, passed. Three journal/path controls, two cohort controls and three pinned Python oracle controls passed. The isolation fixture failed before implementation; its green rerun checked accepted output, full denominators, native coverage, rejection-set B3 and rejected-target closure. The single leased baseline command was `go test -race ./pkg/lang/deps -run '^TestCOVBaseline$' -count=1 -timeout=30m -v` with the pinned corpus/compiler/interpreter environment. The replacement campaign completed in 398.43 seconds, with B3 passing in every pooled table. The command passing means the instrument ran; the table verdicts still govern soundness. It is the first sealed baseline authorized by ESCALATE-19, not the final holdout reading.

The opt-in test requires `FATHOM_COV_BASELINE=1`, `FATHOM_COV_ROOT` naming the prepared campaign root, `FATHOM_COV_PYTHON` naming the pinned interpreter, and `FATHOM_COV_COMPILER` naming the pinned compiler JavaScript. The cohort manifest is verified against the unchanged original pins and `cov_holdout.json`. A fresh independent campaign needs a new prepared destination. The campaign refuses to overwrite an existing `reading/` directory.

### Campaign receipts

The earlier campaign used harness `a1faba52af6f6815e57ce269ae1b9a1e852cd341` and aborted after 381.51 seconds on an inadmissible selected path. Its untouched receipt SHA-256 is `860decc58cbbe3b508bb2c885d30d6028ac1b5c736e68cb7ff7c2615cc4f86b1`. The earlier recovery draft belongs only to that aborted campaign. Its development-only aggregate SHA-256 was `34be19e859dce447d47429c156dc9dd0d901b8f533a8bc9b0c07fc374e95abef`; it was not a sealed baseline.

The replacement journal is `cov-census-replacement/campaign.json`, with status `completed`, reader revision, harness revision, identical subject-manifest digest and both receipt hashes. The replacement receipt SHA-256 is `48ace736e28d5ffa2051fba5d729e3ccabe1d4b49bf0c588546a4f2953b7fe8a`. Original and replacement artifact roots are both retained. No rejected identity was persisted, and no holdout file or line result was opened or printed.

## Development grading

| Dialect | Files | Oracle | Complete | Complete + truth | Complete no truth | B1 missing | COV-D6 | B2 wrong | B2 inconclusive | B3 |
|---|---|---|---|---|---|---|---|---|---|---|
| javascript | 1508 | 1508 | 153 | 153 | 0 | 0 | 0 | 0 | 0 | True |
| python | 1314 | 156 | 121 | 121 | 0 | 0 | 0 | 0 | 0 | True |
| typescript | 1224 | 990 | 250 | 250 | 0 | 0 | 0 | 0 | 0 | True |

B1 missing and COV-D6 count different evidence: native missing target occurrences and independently inventoried external-as-local claims. They can refer to the same underlying defect and are not deduplicated into one invented miss count. B1/B2 require a nonempty complete slice with unique truth for every complete file.

| Dialect | Complete share | B4 extra rate | Complete expected targets | Complete local edges | Complete extras | All edges | Unknowns | Evidence |
|---|---|---|---|---|---|---|---|---|
| javascript | 153/1508 (10.15%) | 753/8351 (9.02%) | 459 | 459 | 0 | 8473 | 8137 | 16610 |
| python | 121/1314 (9.21%) | 0/131 (0.00%) | 24 | 24 | 0 | 6334 | 4571 | 10905 |
| typescript | 250/1224 (20.42%) | 990/5990 (16.53%) | 1296 | 1546 | 250 | 7219 | 3624 | 10843 |

B4 comparable-edge denominator counts reported local edges in native-covered files, including incomplete files. Unknown shares use all edges plus all unknown findings as the evidence denominator. Neither count is a file share.

### Subject counts

| Subject | Dialect | Files | Oracle | Complete | Missing | COV-D6 | Wrong | B4 extra/comparable |
|---|---|---|---|---|---|---|---|---|
| fastapi | python | 1175 | 128 | 114 | 0 | 0 | 0 | 0/95 |
| flask | python | 83 | 9 | 3 | 0 | 0 | 0 | 0/12 |
| attrs | python | 56 | 19 | 4 | 0 | 0 | 0 | 0/24 |
| fp-ts | typescript | 204 | 204 | 0 | 0 | 0 | 0 | 204/2213 |
| rxjs | typescript | 586 | 585 | 250 | 0 | 0 | 0 | 585/2590 |
| react-bootstrap | typescript | 233 | 0 | 0 | 0 | 0 | 0 | 0/0 |
| eslint | javascript | 389 | 389 | 59 | 0 | 0 | 0 | 0/1055 |
| openlayers | javascript | 373 | 373 | 94 | 0 | 0 | 0 | 7/2527 |
| webpack | javascript | 746 | 746 | 0 | 0 | 0 | 0 | 746/4769 |
| headlessui | typescript | 201 | 201 | 0 | 0 | 0 | 0 | 201/1187 |

The aggregate records every development subject's pinned commit, selected-path digest and frozen tracked-input digest. Both checkout arms were detached, clean and canonically identified before and after native capture and product traversal.

### Closure and identity-only

| Dialect | Closure complete/selected | Only unmeasured | Only measured incomplete | Mixed blockers | Rejected-target overlay | Identity-only/all selected |
|---|---|---|---|---|---|---|
| javascript | 0/1508 (0.00%) | 127 | 0 | 1381 | 0 | 10/1508 (0.66%) |
| python | 116/1314 (8.83%) | 0 | 1198 | 0 | 0 | 0/1314 (0.00%) |
| typescript | 0/1224 (0.00%) | 34 | 0 | 1190 | 0 | 0/1224 (0.00%) |

The four closure categories partition the selected denominator. Rejected-target is an overlapping diagnostic, not a fifth category. A refused target in an admitted dialect is a measured-reader refusal and blocks closure as measured incomplete. Other targets without a measured reader remain unmeasured. Config references are included with no special exemption. Identity-only requires a matching versionless external edge for every unknown, not just a suggestive unknown code.

## Development census and projected unlocks

Each syntactic key is an observed bucket, not permission to treat its code as one resolver mechanism. The ranks do not admit levers for mixed `unknown-shape` buckets. `unknown-shape` explicitly means that the instrument cannot establish a construct shape from its located source. Those buckets require investigation before any lever is proposed. Python gap codes come from the unchanged native adapter. The compiler adapter supplies covered rows but no per-file gap reasons, so absent compiler rows are named `native-no-truth`; their individual causes remain unknown.

### javascript

| Unknown code | Occurrences | Distinct files | Occurrences/all evidence |
|---|---|---|---|
| dynamic-import | 11 | 9 | 11/16610 (0.07%) |
| external-unpinned | 115 | 59 | 115/16610 (0.69%) |
| unresolved-alias | 207 | 139 | 207/16610 (1.25%) |
| unsupported-construct | 7804 | 1343 | 7804/16610 (46.98%) |

| Bucket key | Occurrences | Files | Native-covered files | Native-covered occurrences | Sole blocker files | decidable_by | Native gaps by files |
|---|---|---|---|---|---|---|---|
| dynamic-import/unknown-shape | 11 | 9 | 9 | 11 | 0 | observation, toolchain | none |
| external-unpinned/unknown-shape | 115 | 59 | 59 | 115 | 11 | config-file, lockfile | none |
| unresolved-alias/unknown-shape | 207 | 139 | 139 | 207 | 0 | config-file, toolchain | none |
| unsupported-construct/jsdoc-import | 6754 | 1206 | 1206 | 6754 | 456 | config-file, toolchain | none |
| unsupported-construct/unknown-shape | 1050 | 841 | 841 | 1050 | 127 | config-file, observation, toolchain | none |

Native gaps over all selected files: none.

| Step | Bucket | Marginal unlock | Cumulative newly unlocked | Prerequisite for |
|---|---|---|---|---|
| 1 | unsupported-construct/jsdoc-import | 456/1508 (30.24%) | 456 | no prerequisite |
| 2 | unsupported-construct/unknown-shape | 701/1508 (46.49%) | 1157 | no prerequisite |
| 3 | unresolved-alias/unknown-shape | 133/1508 (8.82%) | 1290 | no prerequisite |
| 4 | external-unpinned/unknown-shape | 56/1508 (3.71%) | 1346 | no prerequisite |

| Bucket marginal after prior steps | Initial | 1 | 2 | 3 | 4 |
|---|---|---|---|---|---|
| dynamic-import/unknown-shape | 0 | 0 | 5 | 6 | 9 |
| external-unpinned/unknown-shape | 11 | 51 | 52 | 56 | closed |
| unresolved-alias/unknown-shape | 0 | 1 | 133 | closed | closed |
| unsupported-construct/jsdoc-import | 456 | closed | closed | closed | closed |
| unsupported-construct/unknown-shape | 127 | 701 | closed | closed | closed |

The remaining rows and their `decidable_by` identify observed blockers; mixed unknown shapes do not establish a mechanism-level ceiling. A bucket below 2% after prior closures is not projected unless it is a prerequisite demonstrated by an explicit blocking set within the cap. No more than five buckets are projected. Hypothetical reader unlocks do not create native truth; the oracle-covered and gap counts remain binding.

### python

| Unknown code | Occurrences | Distinct files | Occurrences/all evidence |
|---|---|---|---|
| dynamic-import | 1490 | 972 | 1490/10905 (13.66%) |
| missing-config | 3081 | 1020 | 3081/10905 (28.25%) |

| Bucket key | Occurrences | Files | Native-covered files | Native-covered occurrences | Sole blocker files | decidable_by | Native gaps by files |
|---|---|---|---|---|---|---|---|
| dynamic-import/python-from-import | 1101 | 891 | 7 | 7 | 173 | declaration, observation | namespace:12, package-fromlist:809, relative-context:7, runtime:54, unresolved-direct:2 |
| dynamic-import/python-import-statement | 85 | 85 | 0 | 0 | 0 | declaration, observation | runtime:85 |
| dynamic-import/unknown-shape | 304 | 128 | 0 | 0 | 0 | declaration, observation | runtime:128 |
| missing-config/python-from-import | 2396 | 976 | 29 | 103 | 135 | config-file, declaration, toolchain | namespace:92, package-fromlist:636, relative-context:7, runtime:111, unresolved-direct:101 |
| missing-config/python-import-statement | 685 | 318 | 14 | 22 | 9 | config-file, declaration, toolchain | namespace:22, package-fromlist:109, relative-context:1, runtime:121, unresolved-direct:51 |

Native gaps over all selected files: `namespace`=92, `package-fromlist`=820, `relative-context`=7, `runtime`=135, `unresolved-direct`=104.

| Step | Bucket | Marginal unlock | Cumulative newly unlocked | Prerequisite for |
|---|---|---|---|---|
| 1 | dynamic-import/python-from-import | 173/1314 (13.17%) | 173 | no prerequisite |
| 2 | missing-config/python-from-import | 688/1314 (52.36%) | 861 | no prerequisite |
| 3 | missing-config/python-import-statement | 200/1314 (15.22%) | 1061 | no prerequisite |
| 4 | dynamic-import/unknown-shape | 47/1314 (3.58%) | 1108 | no prerequisite |
| 5 | dynamic-import/python-import-statement | 85/1314 (6.47%) | 1193 | no prerequisite |

| Bucket marginal after prior steps | Initial | 1 | 2 | 3 | 4 | 5 |
|---|---|---|---|---|---|---|
| dynamic-import/python-from-import | 173 | closed | closed | closed | closed | closed |
| dynamic-import/python-import-statement | 0 | 0 | 0 | 4 | 85 | closed |
| dynamic-import/unknown-shape | 0 | 0 | 14 | 47 | closed | closed |
| missing-config/python-from-import | 135 | 688 | closed | closed | closed | closed |
| missing-config/python-import-statement | 9 | 20 | 200 | closed | closed | closed |

The remaining rows and their `decidable_by` identify observed blockers; mixed unknown shapes do not establish a mechanism-level ceiling. A bucket below 2% after prior closures is not projected unless it is a prerequisite demonstrated by an explicit blocking set within the cap. No more than five buckets are projected. Hypothetical reader unlocks do not create native truth; the oracle-covered and gap counts remain binding.

### typescript

| Unknown code | Occurrences | Distinct files | Occurrences/all evidence |
|---|---|---|---|
| parse-error | 725 | 83 | 725/10843 (6.69%) |
| unresolved-alias | 1829 | 697 | 1829/10843 (16.87%) |
| unsupported-construct | 1070 | 829 | 1070/10843 (9.87%) |

| Bucket key | Occurrences | Files | Native-covered files | Native-covered occurrences | Sole blocker files | decidable_by | Native gaps by files |
|---|---|---|---|---|---|---|---|
| parse-error/js-module-statement | 94 | 45 | 45 | 94 | 0 | toolchain | none |
| parse-error/unknown-shape | 631 | 43 | 42 | 628 | 0 | toolchain | native-no-truth:1 |
| unresolved-alias/js-module-statement | 1787 | 690 | 461 | 1126 | 142 | config-file, toolchain | native-no-truth:229 |
| unresolved-alias/unknown-shape | 42 | 40 | 23 | 25 | 1 | config-file, toolchain | native-no-truth:17 |
| unsupported-construct/ts-reference-directive | 3 | 2 | 2 | 3 | 2 | config-file, toolchain | none |
| unsupported-construct/unknown-shape | 1067 | 827 | 594 | 633 | 197 | config-file, toolchain | native-no-truth:233 |

Native gaps over all selected files: `native-no-truth`=234.

| Step | Bucket | Marginal unlock | Cumulative newly unlocked | Prerequisite for |
|---|---|---|---|---|
| 1 | unsupported-construct/unknown-shape | 197/1224 (16.09%) | 197 | no prerequisite |
| 2 | unresolved-alias/js-module-statement | 652/1224 (53.27%) | 849 | no prerequisite |
| 3 | parse-error/js-module-statement | 40/1224 (3.27%) | 889 | no prerequisite |
| 4 | parse-error/unknown-shape | 43/1224 (3.51%) | 932 | no prerequisite |
| 5 | unresolved-alias/unknown-shape | 40/1224 (3.27%) | 972 | no prerequisite |

| Bucket marginal after prior steps | Initial | 1 | 2 | 3 | 4 | 5 |
|---|---|---|---|---|---|---|
| parse-error/js-module-statement | 0 | 40 | 40 | closed | closed | closed |
| parse-error/unknown-shape | 0 | 34 | 38 | 43 | closed | closed |
| unresolved-alias/js-module-statement | 142 | 652 | closed | closed | closed | closed |
| unresolved-alias/unknown-shape | 1 | 7 | 40 | 40 | 40 | closed |
| unsupported-construct/ts-reference-directive | 2 | 2 | 2 | 2 | 2 | 2 |
| unsupported-construct/unknown-shape | 197 | closed | closed | closed | closed | closed |

The remaining rows and their `decidable_by` identify observed blockers; mixed unknown shapes do not establish a mechanism-level ceiling. A bucket below 2% after prior closures is not projected unless it is a prerequisite demonstrated by an explicit blocking set within the cap. No more than five buckets are projected. Hypothetical reader unlocks do not create native truth; the oracle-covered and gap counts remain binding.

## Sealed holdout baseline (COV-D2)

One pooled table per dialect follows. Harness rejected-input counts come from the authorized aggregate sidecar beside the unchanged seal. Rejected files remain incomplete in the full file denominator; native truth still contributes to oracle coverage. They add no invented product edge or unknown code. B3 compares both accepted product output and the exact in-memory rejected sets across repeat and relocation.

### javascript

| Metric | Count or verdict |
|---|---|
| Files | 317 |
| Oracle covered | 317 |
| Complete | 112 |
| Complete with oracle | 112 |
| Complete without oracle | 0 |
| B1 | true |
| Native missing | 0 |
| COV-D6 external as local | 0 |
| B2 | false |
| Wrong | 0 |
| B2 inconclusive | 14 |
| B3 | true |
| Harness rejected inputs (sidecar) | 0 |
| Unknown: dynamic-import | 10 |
| Unknown: external-unpinned | 293 |
| Unknown: unresolved-alias | 44 |
| Unknown: unsupported-construct | 17 |

### python

| Metric | Count or verdict |
|---|---|
| Files | 2820 |
| Oracle covered | 896 |
| Complete | 795 |
| Complete with oracle | 795 |
| Complete without oracle | 0 |
| B1 | true |
| Native missing | 0 |
| COV-D6 external as local | 0 |
| B2 | true |
| Wrong | 0 |
| B2 inconclusive | 0 |
| B3 | true |
| Harness rejected inputs (sidecar) | 0 |
| Unknown: dynamic-import | 5274 |
| Unknown: missing-config | 4606 |

### typescript

| Metric | Count or verdict |
|---|---|
| Files | 2077 |
| Oracle covered | 1481 |
| Complete | 723 |
| Complete with oracle | 723 |
| Complete without oracle | 0 |
| B1 | false |
| Native missing | 0 |
| COV-D6 external as local | 259 |
| B2 | true |
| Wrong | 0 |
| B2 inconclusive | 0 |
| B3 | true |
| Harness rejected inputs (sidecar) | 3 |
| Unknown: dynamic-import | 5 |
| Unknown: external-unpinned | 135 |
| Unknown: parse-error | 26 |
| Unknown: unresolved-alias | 1831 |
| Unknown: unsupported-construct | 1250 |

A false B1/B2 with zero complete files or complete files without unique oracle truth is inconclusive. An inconclusive operand binding also leaves B2 unproven, even when the file has native truth. Positive missing and wrong counts remain separate soundness evidence. Unknown findings count product evidence only; harness refusal counts are reported separately.

## Input metadata and COV-5 follow-up (ESCALATE-19)

The same preflight audited all pinned development and holdout selections. Development rejections by path metadata were zero for Python 1,314, JavaScript 1,508 and TypeScript 1,224 files. Holdout Python 2,820 and JavaScript 317 files had none. The 2,077-file TypeScript holdout had three rejected paths: parentheses affected two files and square brackets affected two, with overlap. No path was dropped or renamed. The path-metadata receipt is retained with the replacement artifacts.

COV-5 finding: one path outside the input alphabet refuses the whole product batch. A future per-file incomplete record changes public behavior and remains reserved for the architect after the readings. This program changes the test harness only. Go admission pins are not present in this lane; their metadata check remains with Foreman and the Go admission lane. The same metadata helper accepts an alternate pinned manifest.

## Routing and proof limits

COV-D7: not fired. No dialect reaches 10% identity-only development files.

COV-D11 fired for TypeScript: 259 independently inventoried external-as-local claims in complete files, with zero native missing or wrong targets. Foreman must stop TypeScript levers and route conservative repair. JavaScript B2 is false because 14 bindings are inconclusive, despite full file-level native coverage and zero wrong targets. Its soundness remains unproven and requires Foreman disposition before levers proceed.

This is a provisional measurement, not independent admission proof. No final holdout reading, reader correction, oracle extension, full gate or strict proof was run in this lane. COV-1 remains unchecked until the required fresh independent rerun and Foreman's strict proof. Any inability to reproduce native truth is a blocker, never zero-filled coverage.
