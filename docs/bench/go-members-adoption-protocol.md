# Go field-only adoption (C-R11)

C-R11 adopts exactly the C-R2 lever measured under O673: field symbols without
reference edges. The prior gate is critic-reproduced PASS: +11.62% wall and
+6.30% store, below K17/K26 +15%/+20%. Issue #17 classification stays unchanged.

This ENGINE branch starts at main 2b6f341e and stacks on PR #52's reviewed cost
proof. Carry the production implementation from measured 4803e3d8b unchanged:
named top-level struct fields, owner-qualified symbols, embedded and unexported
fields included. No interface-method definitions, anonymous nested members,
field read/write/reference edges, or call-target pollution. Preserve bare plan
anchors. Adopt its fixture-derived test count corrections. The epoch fragment
is flagged for controller numbering; never edit generated epoch/board files.

Before measurements freeze a green implementation commit after the full
core/extractor/callgraph/fusion/langbench race suite. The first run used main
2b6f341e; after the controller's Wave B merge, build main de6c46e24
and the optimized candidate using the same Go 1.27.3 flags/caches, -trimpath
-buildvcs=false, GOWORK=off, GOFLAGS=-p=2, GOMAXPROCS=2. Repeat K17 through the
committed go-fields-cost/cost.py collector on the same frozen 5b84aa5b source
snapshot used by the reviewed experiment, outside ignored workspace ancestry.
Six fresh stores, baseline/candidate alternating, median of 3 each, nice 10,
wait one-minute load<=9 before every heavy launch. Preserve every raw measured
field, active-run load spikes, complete-work/typed/plan-anchor equivalence and
binary/source identities. A failed gate remains failed unless owner rules.

K29 impact inventory on this main: lang/go.json (cli/cli 45ffa3c6 and
cobra 40b5bc14). Correction before regrading: routes/node.json has 65 active
JS/TS subjects and no Go subject; its fauxtown note is a historical unchanged
control. Verify the mixed-Go fauxtown L0 goldens and Go route fixture tests
separately; do not manufacture a Node reading impact.
Historical lang/baseline/go.json is not a current claim; go-mod uses another
extractor. Regrade every affected current reading, not generated boards.

Then generate independent scip-go 0.2.7 indexes over the unchanged pinned
cli/cli and cobra modules, with tests and no package filters. Use the complete
committed Go corpus pins, ctags 6.2.1 and current harness, retain all failures and
all original denominators. Record all L1-L4 metrics and tenancy; median-of-three
throughput where required. No daemon or source edits. If any producer/read
route fails, record its exact limitation rather than asserting a new grade.

Only after those canonical readings freeze the candidate, oracle recipes and
configuration, then read the already frozen Gin v1.10.0 holdout once, at
75ccf94d605a05fe24817fc2f166f6f2959d5cea. It remains outside the canonical
shard. No tuning after that reading. Success criteria and limits stay as
preregistered in go-fields-holdout.yaml. No reclassification of issue #17.

A dated assessment, committed actual readings and a lane-unique change fragment
record the observable new field symbols. X-R2/X-R3 and the integration gates
precede READY. Landing is serialized after Azure #44 in wave B and before any
CSS ENGINE wave. Building these branches does not wait for landing slots.

## Current-main gate result before adoption

The first registered current-main six-ingest run FAILS: baseline wall trials
85.340/80.441/77.325s, candidate 96.479/85.353/95.389s. Medians 80.441 and
95.389s give +18.583% wall, over +15%; store 205312000 to 218238976 bytes,
+6.296%, below +20%. Equivalent complete work passed. All launches were <=9;
the first baseline reached active-run load 14.48 and remains included. CPU
medians 85.170 to 99.781s (+17.15%) are diagnostic, not a substitute wall gate.
Implementation c975fbbe3 remains frozen for this result. The earlier PR #52
critic-reproduced +11.62%/+6.30% pass is historical evidence on its named base;
it does not turn this failed ENGINE run into a pass. Keep the branch unready,
investigate the cost, and leave Gin unspent while the gate is unresolved.

The committed collector writes every raw measured field; the public receipt
adds only an annotations object for baseline/build identities and interpretation.
Reproduce after building both named source trees and setting the isolated Go
cache/environment described above, with fresh stores and logs:

```sh
python3 calibration/oracles/go-fields-cost/cost.py \
  --baseline "$BASELINE_BINARY" --candidate "$CANDIDATE_BINARY" \
  --subject "$FROZEN_SOURCE_ROOT" --stores "$FRESH_STORES" \
  --log-dir "$FRESH_LOGS" --out "$FRESH_RECEIPT" \
  --source-commit 5b84aa5b4d179460a2f9b7a96973a89a6cd8b322 \
  --candidate-commit c975fbbe38e9244bca839029cce58d483613c3ed
```

## Cost-directed empty-reach optimization preregistration

The per-run pass CPU summaries in the r1 receipt identify centrality: baseline/candidate median CPU
6.831/13.755s, +6.924s; extract +0.717s and typed_go -0.198s. On the first
complete store pair, all 62,537 existing centrality rows are identical; the
21,564 added rows have zero callers. Their inclusion must remain unchanged.

Optimize only allocation of empty SCC reach sets. Preserve every graph node,
edge, field symbol, caller count and normalized centrality row. Empty reach
sets use nil; unions accept empty inputs; a multi-node SCC still adds its own
component to forward reach. Independent BFS tests, source/sink/self/cycle
controls and an allocation benchmark precede a full race suite and freeze.
Then repeat the identical preregistered six-ingest gate with fresh stores,
current-main baseline binary and the new green candidate. Retain the failed
run, do not alter thresholds or exclude trials. Compare semantic centrality
rows against the failed-run candidate independently of watermark metadata.
No Gin reading while this cost gate is unresolved.

Wave B main de6c46e2456f5131b4b6d5287d75157c1d3930bc is the next baseline.
The post-merge seven-package race suite passes. The allocation control at 4,096
isolated fields reduced 3,580,568 to 1,483,416 bytes/op and 8,254 to 4,158
allocations/op; those microbenchmark timings are not the wall gate. The three
L0 payload fixtures include four newly visible ledger fields, with byte/count
expectations updated and all timing budgets and tolerances unchanged.

## Wave B optimized gate result

Frozen implementation 5b8d6a2a6124ec5cf84f3909c1ffa0231f8b1511 passes against
main de6c46e2456f5131b4b6d5287d75157c1d3930bc. Baseline wall trials
76.599/76.618/76.585s, candidate 87.653/89.662/86.632s; medians
76.599 -> 87.653s (+14.431%). Store 205312000 -> 218238976 bytes
(+6.296%). Complete work and typed-edge controls match; all launches <=9.
Candidate trial 2 reached active-run load 14.20 and remains included. CPU
medians 81.931 -> 91.709s are diagnostic. Every candidate trial's 84,101
normalized centrality rows hash to the failed candidate's unchanged semantic
hash ff28cb5b75e07b3d9d200dd595de7b8b938388198d74f2cc8947277e3fd0556f.
The first failed run remains recorded. The committed collector is unchanged.
The raw six-trial schema is preserved in go-members-cost-2026-10-26-r2.json;
only annotations add build identities, raw-receipt hashes, per-run centrality,
extract and typed_go CPU summaries, and semantic controls. The r1 receipt
carries the same pass summaries and its original raw-receipt hash.
Canonical and independent oracle grading precede the frozen Gin validation.

The canonical, independent, Gin and six-trial cost runs all used full ingest.
The subsequent `ReplacePackage` correction applies only to incremental
refresh, leaving those measurements on their original code path. The L2
difference from the 2026-10-10 historical Go reading predates this branch on
main; it is consistent with TypedEdgeEpoch 7 / O1237. The same-oracle main
control shows no L2 change from this member pass.

Binding review F1/F3-F7 follow-up: patch-time call indexes and incoming
edges exclude field targets, including a same-key method-to-field edit.
Real extractor fixtures and transition controls compare refresh with Build.
SCIP export now reports new field symbols as Field, retaining term descriptors.
The seven-package race suite plus memo and SCIP export passes; the generated
language tracker test is reserved for controller regeneration under this brief.
Both cost receipts retain their original collector data and now carry per-run
centrality, extract and typed-Go CPU summaries; r2 includes its raw receipt hash.
Per-trial UTC timestamps were not collected and cannot be reconstructed from
those CPU summaries. No new cost run or holdout tuning was performed.

C-R13 landing dependency: issue #17's classification branch lands first. This
branch must then merge main, never rebase or merge that topic branch directly,
and regrade the canonical and independent readings from the same frozen SCIP
indexes using the combined harness.

## C-R13 merged-tree regrade

The combined tree d28ff1d94ae56b86f271a35c70367cf03a96bd5c includes
issue #17's a1c183658 classification proof and the ca483d4e field-refresh
correction. A clean build of `cmd/indexbench` from that tree graded canonical
and independent cli/cli and cobra readings from their four frozen SCIP indexes,
then regraded the already spent Gin holdout from its frozen index. No SCIP
producer, subject pin, oracle rule or field scope changed. The
[merged-tree receipt](../assessments/data/2026-10-26-core-t1-go-members-merged-regrade.json)
records input and binary hashes, phase loads/times and the three median-of-three
throughput results. Every phase exited 0 with launch load below 9.

Canonical and independent counts, metrics and grades agree: L4 good,
def_mapping .9345 and typed_recall .9889, with L1-L3 excellent. Their median
extraction rates are 20.56 and 20.31 MB/CPU-s. Gin remains L4 excellent and L3
good, with file-edge precision .9235 and recall .6255; its median is 15.68
MB/CPU-s. This is a K29 regrade of a spent holdout, not a new validation or
tuning pass. The canonical denominator still includes three definitions from
cli/cli's tracked internal/build package, which RT-145's default directory
filter leaves unmeasured (issue #17 class 2). The O1237 L2 dispatch shift
predates the field pass; same-oracle metrics remain unchanged. No interface
methods or field-reference edges entered this field-only implementation.

## Wave C same-index regrade

After Wave C was merged, the clean tree
6eff4e2ecad732f233859bbba44732e4d2b80666 (including 6db920e72) was
graded again from the same five frozen SCIP indexes and subject pins. The
[Wave C receipt](../assessments/data/2026-10-26-core-t1-go-members-wave-c-regrade.json)
is separate from the earlier C-R13 receipt and records the new indexbench
binary hash, phase loads/times, raw and published reading hashes, and the
prior receipt hash. No producer ran. All nine phases exited 0 with launch load
below 9.

Canonical and independent counts, metrics and grades are identical to the
first merged-tree reading and to each other: L4 good, def_mapping .9345,
typed_recall .9889, with L1-L3 excellent. Their new median-of-three rates are
20.43 and 20.83 MB/CPU-s. The spent Gin holdout remains L3 good/L4 excellent,
with median 15.70 MB/CPU-s. Issue #17's three unmeasured internal/build
definitions, O1237's earlier L2 attribution and the field-only scope remain
unchanged. These timings replace the current reading's timings; the earlier
receipt remains a dated measurement of its named tree.

Wave C verification: the full nine-package race suite passed in 331.81s on
the merged implementation (core, extractor, callgraph, fusion, langbench,
routes, indexbench, memo and SCIP export). The generated tracker assertion
was excluded because the controller owns regeneration. The focused merged
Go/SCIP/refresh tests, release-note check, merge-debris check and ADR-011
org-name test also passed. All readings and cost receipts remain attributed
to their named measured sources; no new cost measurement is claimed.
