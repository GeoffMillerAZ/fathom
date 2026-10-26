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
cobra 40b5bc14) and routes/node.json (fauxtown: six Go routes in two files).
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
