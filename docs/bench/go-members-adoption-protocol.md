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
core/extractor/callgraph/fusion/langbench race suite. Build both main 2b6f341e
and the candidate using the same Go 1.27.3 flags/caches, -trimpath
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
