# Go field-only ENGINE adoption audit

C-R11 authorizes the C-R2 lever, field symbols without reference edges, subject
to K17/K26 and every affected reading. This branch starts on Wave A main
2b6f341e, with PR #52's reviewed cost proof merged as a declared prerequisite.
The production port is the previously measured implementation; it preserves
call targets and bare plan anchors. Five-package race verification passed.

The first Wave-A-main cost gate **FAILS**: median wall 80.441 -> 95.389s (+18.583%,
limit +15%), store 205312000 -> 218238976 bytes (+6.296%, limit +20%). All six
launch loads were <=9, all complete-work/typed-edge controls matched, and
changed field work executed. The first baseline active-run load spike 14.48
is retained. CPU medians 85.170 -> 99.781s (+17.15%) support investigating
work and allocation costs rather than deleting an inconvenient wall trial.
See [the committed collector receipt](../bench/readings/go-members-cost-2026-10-26.json)
and [the preregistered protocol](../bench/go-members-adoption-protocol.md).

PR #52's earlier +11.62%/+6.30% critic-reproduced pass remains valid historical
evidence for its named base and candidate; it does not satisfy this run's
failed wall gate. This failed run claimed no new grade or adoption. Gin was still unspent at that failed run; its later frozen result is recorded below
and remains outside the canonical corpus. No issue #17 classification changes.

K29 scope correction: only lang/go.json actively consumes the affected Go
symbols. The Node route reading's fauxtown statement is a historical control,
not one of its 65 active JS/TS subjects. The Go/mixed fauxtown route and L0
fixture gates remain required separately. Full canonical Go, independent
scip-go cli/cobra and final frozen Gin readings follow a green cost gate.

## Optimized Wave B result

Measured: the field-only pass plus empty-reach allocation optimization passes
K17 against Wave B main de6c46e24: median wall 76.599 -> 87.653s (+14.431%),
store 205312000 -> 218238976 bytes (+6.296%). All six launches were <=9;
candidate trial 2's active-run load spike to 14.20 is retained. Complete work
and typed edges match. See the [actual collector receipt](../bench/readings/go-members-cost-2026-10-26-r2.json).

The full seven-package post-merge race suite passes. At 4,096 isolated fields,
the allocation control fell from
3,580,568 to 1,483,416 bytes/op and 8,254 to 4,158 allocations/op. Every one
of the 84,101 centrality rows is semantically identical across the old
candidate and all three optimized trials, including all 21,564 added zero-
caller field rows. No graph node or reference was removed to pass the gate.
The earlier failure and PR #52's distinct historical pass both remain intact.
This cost result does not itself establish a language grade.

## Canonical and independent SCIP measurements

Measured: scip-go 0.2.7, Go 1.27.3 and Universal Ctags 6.2.1 on the full pinned
cli/cli and cobra modules, tests included and no file/package filters, give L4
good: def_mapping .9345, typed_recall .9889. L1-L3 remain excellent. The
separate independent producer executions reproduce every count and numeric
metric. The current [canonical reading](../bench/readings/lang/go.json) and
[independent reading](../bench/readings/go-members-independent-2026-10-26.json)
retain computed grades and median-of-three extraction throughput.

A same-oracle Wave B main control gives def_mapping .5891 and typed_recall
.6714. Call precision/recall (.9254/.9241), file-edge precision/recall
(.9956/.7030) and import resolution (1) are identical before and after the
member pass. The L2 difference from the 2026-10-10 reading predates this
branch on main and is consistent with TypedEdgeEpoch 7 / O1237, not a
producer or toolchain change. The same-oracle main control establishes that
the member pass caused no L2 regression. The harness reports 692
unmapped definitions, including 689 field-category terms; this records its
counts without reclassifying issue #17. No interface methods or reference
edges were added. No L4-excellent claim follows from the cost pass.

The canonical, independent and cost measurements use full ingest, which
builds the graph from scratch. The incremental refresh correction in
`ReplacePackage` does not change their measured path, so the published
readings and cost receipt remain valid for the measured full-ingest code.

Landing-base K17 rerun (critic, base `6db920e72`, candidate `d0faef431`): wall +12.85% (86.637 -> 97.768s), store +6.30% (205312000 -> 218238976B), PASS with 2.15 wall points of margin; receipt [`core-t1-go-members-r1-cost-rerun-wave-c.json`](data/core-t1-go-members-r1-cost-rerun-wave-c.json).

## Frozen Gin validation

Measured once after evidence freeze 85ee60563031bba3fb1da2cc9f22bc0a8bcf72fa:
Gin v1.10.0 at 75ccf94d605a05fe24817fc2f166f6f2959d5cea reads L4 with
L1/L2/L4 excellent, def_mapping .9744 and typed_recall .9889; L3 is good,
file_edge_precision .9235 and recall .6255. Import resolution is 1, zero
faults, median-of-three throughput 15.58 MB/CPU-s. The [holdout reading](../bench/readings/lang/holdout/go-gin-fields.json)
and [counts/hashes receipt](data/2026-10-26-core-t1-go-members-gin.json) preserve
the result. All three phase launches were <=9. No source or oracle rule changed.

The preregistered criterion was excellent at every level; it **fails** at L3.
Gin is spent, stays outside the canonical shard, and is not a tuning corpus.
The canonical L4-good limit and Gin L3-good miss are recorded as measured
limits of this field-only wave. Broader member or file-edge work requires a
separate owner-authorized wave and fresh validation, without reclassifying
issue #17 here. C-R11's field-symbol-only implementation remains unchanged.

## C-R13 merged-tree reading

The merged implementation and issue #17 classification at
d28ff1d94ae56b86f271a35c70367cf03a96bd5c were regraded from the same
frozen canonical and independently produced SCIP indexes. Counts, metrics and
grades agree between the two readings: L4 good, def_mapping .9345 (9,877 of
10,569) and typed_recall .9889; L1-L3 remain excellent. Median-of-three
extraction rates are 20.56 and 20.31 MB/CPU-s. The [append-only regrade
receipt](data/2026-10-26-core-t1-go-members-merged-regrade.json) records the
merged binary, source and index identities, each phase's load and duration,
and the published reading hashes. All phases exited 0 below launch load 9.

Issue #17 class 2 remains in scope: cli/cli's tracked internal/build package
is pruned by RT-145's default cache-directory filter, leaving three SCIP
definitions unmeasured and counted. The merged reading has 689 unmapped
field-category definitions across cli/cli and cobra, which include interface
methods outside this field-only pass; the three class-2 definitions bring the
unmapped total to 692. This measured limit keeps L4 below excellent's .95
definition-mapping bar. The O1237 standard-library interface dispatch shift
predates this pass, and the same-oracle L2 metrics did not change.

The already spent Gin holdout was regraded under the merged harness from its
frozen SCIP index. L1/L2/L4 remain excellent and L3 good; file-edge precision
is .9235, recall .6255, and median throughput 15.68 MB/CPU-s. No producer
rerun, criterion change or tuning occurred. The field-only scope and its
remaining excellence limits are unchanged.

## Wave C regrade

The clean Wave C tree 6eff4e2ecad732f233859bbba44732e4d2b80666,
including 6db920e72, was regraded from the same frozen canonical,
independent and spent Gin SCIP indexes. The [separate Wave C
receipt](data/2026-10-26-core-t1-go-members-wave-c-regrade.json) preserves
the first merged-tree receipt and records the new binary hash, input hashes,
phase loads/times, reading hashes and median-of-three throughput.

Every canonical and independent count, metric and grade reproduces the prior
merged-tree result: L4 good, def_mapping .9345 and typed_recall .9889;
L1-L3 excellent. Their new medians are 20.43 and 20.83 MB/CPU-s. Gin's counts,
metrics and grades also reproduce: L3 good, L4 excellent, median 15.70
MB/CPU-s. All nine phases passed below launch load 9. No SCIP producer ran,
and no holdout tuning occurred. Issue #17 class 2 remains three counted
unmeasured internal/build definitions; O1237 remains the earlier L2 dispatch
attribution. The field-only implementation scope and excellence limits are
unchanged.

The final Wave C nine-package race suite passed (331.81s), including the
field refresh/build-equality controls. Only the controller-owned generated
tracker assertion was excluded. Wave C focused reading/SCIP controls, X-R2,
X-R3 and merge-debris checks passed. No new production change followed the regrade.

## R-D19 main-merged replay (2026-10-27)

Main 440a94365 and the field branch merged at 78d38fbaafd1af5dcb5a51bd147b2e0d8fdf60ca.
The combined harness regraded the canonical, independently produced and already
spent Gin readings from their five frozen SCIP indexes, then retimed extraction.
The [new receipt](data/2026-10-27-core-t1-go-members-rd19-regrade.json) records
the merged source, binary and input hashes, all nine successful phase loads and
durations, raw and published reading hashes, and the prior receipt hash. Launch
loads stayed below 9; no SCIP producer or K17 cost gate ran.

Canonical and independent counts, metrics and grades again agree: L4 good,
def_mapping .9345 (9,877 of 10,569), typed_recall .9889, and L1-L3 excellent.
Their median extraction rates are now 20.93 and 20.38 MB/CPU-s. D19's 20-reference
floor leaves Go's L3 evidence sufficient: file-edge precision has a 3,158-reference
denominator, recall 4,472 and import resolution 359. The field-only symbols account for the
L4 lift. The same-oracle main control found unchanged call and file-edge metrics
with and without fields. The earlier L2 dispatch shift belongs to TypedEdgeEpoch 7
/ O1237; historical file-edge differences between old main and this replay are
prior measurement context, not attributed to fields.

The landing-base K17 critic rerun remains historical: base 6db920e72 versus
candidate d0faef431 passed at +12.85% wall and +6.30% store. It was not rerun
against main 440a94365 or the R-D19 merged source.

The spent Gin reading remains L4 excellent and L3 good, with file-edge precision
.9235, recall .6255 and median extraction 16.29 MB/CPU-s. Its file-edge
denominators, 170 and 251, exceed D19's floor; its six in-repo imports do not.
L3 has two independent caps: file_edge_precision .9235 < .95, and
import_resolution has 6 in-repo imports, under D19's 20. Neither changes
Gin's earlier grade or the failed L3-excellent criterion.
No new holdout exposure, tuning, oracle change or producer run occurred.
The receipt's published hashes identify the original 276f2d233 reading bytes;
C-R15 restores the disposition and corrects the Gin cap note without changing
any measured value.


## Go field deletion coverage

The initial regression covered the stored name T.n, the service-less key
p/T.T.n and the canonical key; it did not cover a bare field identifier.
Expanded reader-backed controls reproduce and fix n, Inner, p.T.n, r.S.Name
and field/method collisions under n and Name. They also retain the stored
names, owner/key suffixes and canonical keys, while U.Name remains an ordinary
method query. The Name collision preserves its recorded ReadName caller.

Deletion checks and find_symbol now share exact, case-insensitive and
substring target resolution. The guard inspects the resolved node kind,
including candidates beyond the lookup's display cap, instead of recognizing
selected spellings. If any target is a GoField it preserves recorded referents,
reports field_references_indexed false and says field references are not indexed.
This query-only correction changes no full-ingest reading or cost path. The
pre-existing missing-target and Go variable/constant cases remain separately
filed as CT1-SD-1; that row also preserves the field-spelling controls.

## C-R14 consumer compatibility

C-R14 reopens the field pass after the broader consumer tests exposed ranking,
impact seeding and stale fixture counts. Matching Go fields now rank below
functions, methods and types, preserving text relevance among fields. The
symbol-group route already implements 50-row pages with offset-plus-limit
lookup; its old first-page assertion confused page length with total matches.
The replacement checks every JSON/HTML page, unique rows and next/previous links
against the advertised total, preserving access to every match.

For `blast_radius kind:"change"`, changed fields are depth-zero changed facts in
`changed_fields`, counted separately by `changed_field_count`, with the note
"field references not indexed". They neither seed nor bridge the traversal and
do not consume `max_symbols`; the graph-symbol and depth-bucket counts describe
only the traversable radius. Field-free responses retain their prior shape.
The unbounded verification/review analysis defaults remain unchanged.

C-R14 comments identify the two updated fixture expectations: payment-service
ingest has 12 symbols including five fields, and the oracle fixture maps three
definitions including its field. The generated `TestCmdLangTracker` and
`TestLanguagesTrackerIsGenerated` checks stay with the controller at landing;
both require the same unregenerated `docs/languages.md`. The earlier nine-package suite did not cover
all consumers; it is not the C-R14 verification gate. The focused five-package race passes, including the previously failing consumer
controls, with an independent read-only diff review finding no issues. Actual
browser inspection of the disposable synthetic handler showed callable-first
ranking and 91 matching symbols reachable as 50 plus 41 rows through Next. The
preview was stopped after inspection. Expanded race verification and the final
C-R14 result are recorded after execution below.

These are query and test corrections. They change no extraction, full-ingest
cost path, frozen SCIP index, language reading or epoch, so the recorded K17
measurements and Wave C regrades remain valid for their measured paths.

Expanded race coverage ran `bash scripts/fathom-go.sh test -race -count=1
-timeout=30m -skip "^TestCmdLangTracker$" ./cmd/fathom ./cmd/indexbench
./pkg/client ./internal/...` in 1,338.625s at launch load 5.986. It covered
144 packages: 141 passed, one had no tests, and two exposed expected fixture
work. `TestPayloadReplay` correctly detected the deliberate seven-symbol graph
radius plus four separate fields: 1,757 -> 1,852 bytes (+95, +5.4%). C-R14
authorizes that disclosure; its committed collector regenerates the golden.
`TestLanguagesTrackerIsGenerated` checks the same controller-owned language
board as `TestCmdLangTracker`; the board is not regenerated in this lane.
The expanded run exited 1, not a clean-suite pass. The two affected packages
are rerun after regeneration, with only the named board assertion excluded.

The committed `TestPayloadReplay -update` collector regenerated the measured
fields. Besides the deliberate blast-radius delta, it refreshed localize's
payload from 1,470 to 1,518 bytes (within 5%, still three candidates). No
authored arguments, budgets or count fields changed. The full affected-package
race rerun passes: `internal/indexbench` and `internal/langbench`, excluding
only `TestLanguagesTrackerIsGenerated`. Together with the broad run's 141
passing packages, this verifies all 143 test-bearing packages in the required
scope except the two controller-owned board assertions; one additional
package has no tests. No production source changed after the broad run.

The final release-note, merge-debris and org-name batch passes (21.441s at
launch load 7.392). The fragment was corrected to its required single heading
after the check rejected two headings. Added-line/new-file privacy scanning
found no subject identities. The independent read-only review
found no issues in the production delta. No generated board was written.

Passing replay and affected-package commands:

```sh
bash scripts/fathom-go.sh test -race -count=1 -run "^TestPayloadReplay$" ./internal/indexbench -update
bash scripts/fathom-go.sh test -race -count=1 -skip "^TestLanguagesTrackerIsGenerated$" ./internal/indexbench ./internal/langbench
bash scripts/fathom-go.sh run ./third_party/agentic-kit/cmd/release-notes -check
bash scripts/check-merge-debris.sh
bash scripts/fathom-go.sh test -count=1 -run TestNoOrgSpecificNames ./cmd/fathom
```
