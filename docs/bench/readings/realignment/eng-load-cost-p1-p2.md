# W7-1: combined graph load cost, P1/P2

P1/P2 reduce the admitted cache-miss snapshot+syntax+BOTH median from **1.500602458 s
to 1.266565417 s** on the same pinned W5 ingest: **15.6% less time, 1.185x speedup**.
The one-second bar still fails. This is local helper evidence, not a deployed
ENG-1 proof. Architect-class **HOLD** pending separate critic and architect review.
No plan row is claimed. Issue 251 is not dispatched.

## Change and preserved laws

P1 compacts the graph's owned typed-pruning adjacency slice in place instead of
copying survivors into a capacity-zero buffer that was discarded when nothing was
removed. P2 adds `Graph.WithTypedEdges`, reusing the unchanged clone helpers in
`replace_package.go`, and routes `GraphFor(GraphBoth)` through it only when the
exact scope's completed syntax graph is already resident. An absent or in-flight
syntax graph keeps the existing full-build path. Combined-only loads retain no
additional syntax entry.

Every graph map and mutable adjacency slice is copied before typed augmentation.
Immutable node payloads retain their existing sharing contract. Optional nil maps
remain nil until populated. The existing combined memo admission, singleflight,
context cancellation, typed-read failure disclosure, watermark eviction and byte
accounting remain in force (I21, I38, I49, I51). No store, schema, configuration,
registry or handler change, new memo group, retained cache or dependency is added.
P3/P4, worker reload and RT-197 are outside this change.

## RED before implementation, GREEN after

The exclusive lease reproduced RED before production edits:

```text
go test -race ./internal/analysis/callgraph ./internal/graphmemo -run '^TestCloneBoth' -count=1
WithTypedEdges undefined
TestCloneBothProductionReusesCompletedSyntax: production GraphBoth re-resolved snapshot instead of cloning completed syntax
```

The production-entry marker graph has an edge absent from snapshot nodes. Its full
graph comparison therefore detects re-resolution, beyond helper-only correctness.
After P2, full equality passed and P1 remained RED against the frozen old pruning
algorithm: candidate 3 allocations, baseline 3. Only then did P1 change.

After both changes, under the exclusive lease:

```text
go test -race ./internal/analysis/callgraph ./internal/graphmemo -count=1
ok github.com/geoffmilleraz/fathom/internal/analysis/callgraph 12.055s
ok github.com/geoffmilleraz/fathom/internal/graphmemo 2.947s
```

The clone equality oracle independently resolves syntax a second time. It checks
all Graph fields with `reflect.DeepEqual`, plus served caller/callee/test-impact,
candidate and product-reach outputs. Inputs include nil/empty sets, generated
ambiguous/duplicate-key/method/field sets, and Go extraction of actual servedcalls
and fauxtown fixtures, each in four input orders and with nil/empty/populated typed
edges. Tests cover typed pruning, dispatch, instantiation, missing endpoints,
synthetic package init and mutation isolation in both directions. The frozen
pruning oracle also covers all 16 typed-edge keep/remove masks and idempotence.

Clone helpers trim spare adjacency capacity. Tests account for the exact
capacity-byte difference in `EstimatedBytes`, rather than demanding identical heap
cost for identical graph values. Production-entry tests check exact registry byte
accounting, concurrent singleflight identity, memo hits, watermark changes,
empty-watermark behavior, combined-only retention, cancellation, typed failures,
recovery and cold deadline admission. An initial admission test incorrectly reused
a scope whose syntax build had already measured its service cost; it was corrected
to assert the existing cold-admission rule on a distinct cold scope. Product
admission logic was not changed.

Two initial tests used a temporarily authorized shared package pool before its
self-check authorization was withdrawn. Both had finished before the correction.
All decisive RED/GREEN and all builds/timings here subsequently ran under the
exclusive lease. No further shared-pool job ran. Evidence logs preserve both
attempts. Harness preparation also hit an older Python tarfile API before any
build/trial; its compatibility fix and failure log are retained. No trial was
omitted, retried, selected or replaced.

## Controlled measurement

Ten fresh processes: five unchanged indexed-baseline and five P1/P2 indexed
trials, alternating which arm leads each pair. Each receives a separate
byte-identical copy of the original W5 scratch ingest. The pinned source is only
copied, never opened by the client or written. Every copy's before/after hash and
the pinned source's before/after hash match:

`e22f40d4dd467b0201badae263901741db23a81864c4f545c111b5c469ecd704`.

The unchanged W5 indexed serving-load harness is reused in both arms with identical
phase marks, full-field canonical graph serialization and additional common
boundary memory/memo sampling. The primary timer starts before actual enforced
admission and ends after snapshot, syntax and BOTH complete. Typed loading is
included in BOTH. Resolved builds occur afterward and are shown separately.
Full-load time includes resolved and common measurement overhead. Snapshot phase
also includes admission/scope setup. Memory samples at BOTH are outside its timer;
process peak RSS includes later resolved construction and output serialization.
This is a cold memo reading, not an OS page-cache flush.

Environment: Go 1.27.4, darwin/arm64, cgo enabled, 32 GiB RAM, 12-core Apple M2 Pro,
GOMAXPROCS 6, 4 GiB managed budget, 1 GiB memo budget, 8 GiB GC hint, normal level.
The harness uses actual `ModeEnforce` admission, with only the tree deadline
extended to ten minutes so a timing trial measures a complete load rather than a
short-deadline refusal. Budget's informational Admission field remains its default;
the constructed gate's mode is explicitly enforce. No paid API or benchmark service was
used. Builds and the complete trial sequence held the exclusive shared gate lease.

Every trial had one admitted successful ticket, one symbol read and one typed read.
At the primary boundary the snapshot and typed groups each had one miss, flight and
entry; graphstore had two misses, flights and entries. After resolved, graphstore
had three of each. All groups had zero declined, abandoned, refuse-cancelled and
uncached builds. No warm/cached substitute or fast refusal is included. Each arm
loaded 104183 snapshot nodes and 256033 typed edges; syntax had 103099 nodes/92411
edges, BOTH 103118/162606, and resolved 103118/120260. Constructor references: 4216.

## All trials

Primary and full-load timers below are exact nanoseconds. Phase and memory columns
are rounded only for display. Every raw result, phase nanosecond, counter and
memory value is retained in `workspace/tmp/w7/all-trials.json` and the ten trial
directories. The required primary boundary includes admission, snapshot, syntax
and BOTH; the original W5 scan trials remain independent historical evidence.

| Trial | Primary ns | Full-load ns | Snapshot s | Syntax s | BOTH s | Resolved extra s | Boundary allocated MiB | Boundary resident MiB | Process peak RSS MiB |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 01-baseline | 1457749750 | 1748081334 | 0.347254 | 0.300609 | 0.809887 | 0.290331 | 805.973 | 419.480 | 1782.516 |
| 02-p1p2 | 1289623250 | 1603740375 | 0.357020 | 0.325471 | 0.607132 | 0.314116 | 707.475 | 418.981 | 1446.250 |
| 03-p1p2 | 1266565417 | 1542403417 | 0.365570 | 0.309721 | 0.591274 | 0.275838 | 708.476 | 418.981 | 1342.234 |
| 04-baseline | 1500602458 | 1788748083 | 0.353245 | 0.330020 | 0.817337 | 0.288145 | 805.871 | 419.481 | 1533.516 |
| 05-baseline | 1527452958 | 1847544333 | 0.356559 | 0.322939 | 0.847955 | 0.320091 | 805.586 | 419.479 | 1716.422 |
| 06-p1p2 | 1284700666 | 1587694666 | 0.357571 | 0.328648 | 0.598482 | 0.302993 | 708.004 | 418.981 | 1524.656 |
| 07-p1p2 | 1235570875 | 1544945333 | 0.336146 | 0.303954 | 0.595471 | 0.309374 | 708.194 | 418.980 | 1627.406 |
| 08-baseline | 1486191375 | 1765184625 | 0.338095 | 0.315985 | 0.832111 | 0.278993 | 805.930 | 419.481 | 1678.062 |
| 09-baseline | 1566325625 | 1853985750 | 0.366572 | 0.333849 | 0.865905 | 0.287659 | 805.620 | 419.480 | 1618.516 |
| 10-p1p2 | 1250867958 | 1550767833 | 0.363169 | 0.312543 | 0.575156 | 0.299899 | 707.906 | 418.980 | 1549.609 |

## Medians and limits

| Measurement | Indexed baseline | P1/P2 | Reduction |
|---|---:|---:|---:|
| Primary boundary seconds | 1.500602 | 1.266565 | 15.60% |
| Full-load seconds | 1.788748 | 1.550768 | 13.30% |
| Boundary allocated MiB | 805.871155 | 708.003838 | 12.14% |
| Boundary live heap MiB | 495.294670 | 400.795525 | 19.08% |
| Boundary memo estimate MiB | 419.479890 | 418.980622 | 0.12% |
| Process peak RSS MiB | 1678.062500 | 1524.656250 | 9.14% |

These five-per-arm descriptive medians do not establish a confidence interval or
separate P1's effect from P2's. Peak RSS is the whole process maximum, not a primary
boundary peak. The memo estimate decreases only about 0.50 MiB because both graph
values are still retained; allocation and live-heap reductions are larger. The
one-second primary bar fails in every P1/P2 trial. Deployed behavior, RT-195's
original scan ratio and ENG-1 completion are not established by this reading.

## Identity and reproduction

Baseline/corpus commit: `d9eef4794afdbc200d1b8697ce121d9d688fedaf`. The candidate is that exact
archive with only `graph.go`, `cache.go`, and `graphstore.go` overlaid. Source archive
SHA256: `0d2836139e55958e8d7f4ab3ef9198034cbe70bc1e98df7c9b43f723be57d082`.

| Binary | SHA256 |
|---|---|
| baseline | `92a1585ea57c8c5fced0d366ca2895919ca2966db5cdaa110c5589e4bd7f5317` |
| p1p2 | `e29cf36ea32f85b602e650370a9f1e5b8593c28d5fa9074a4272e201451f55a7` |

The following output SHA256 values matched across all ten trials, including every
private Graph field and suffix-index value (not just node/edge counts):

| Output | SHA256 |
|---|---|
| both | `48860bbf28bb6948682035b351de339e5cd6175290ad54f551f2ac547f55d692` |
| resolved | `b087384be66589ee51a2f08db4d382b3b3dbf88000869ca6adb9792dbdbafa85` |
| snapshot | `106d44446d04dc54781a110e5e24f42a15bc9055e56346426e0121dbc3fab202` |
| syntax | `ce43dbb85e06842ab965604eba4f5b6ae527cfabeef793c37778e2e867fefced` |
| typed | `1c07d1627ad3ac3f96ba8ecad7db5b2e2b97223f40de0c785a0a9719bf6510bf` |

Retained evidence is under `workspace/tmp/w7`: `source.tar`, `identity.json`
(per-file baseline/candidate and identical harness hashes), source diffs, binaries
and Go build metadata, environment before/after, raw per-trial canonical outputs,
result JSON, time/RSS stderr, exit codes, input hashes, `all-trials.json`,
`summary.json`, RED/GREEN logs and `SHA256SUMS`. The source and candidate diffs
allow a critic to verify that no product change outside P1/P2 enters either arm.

Reproduction uses the retained ignored scripts and the original W5 indexed harness:
source the lane environment, set the shared `FATHOM_GATE_LOCK`, wait for
`GATE-RUNNING` to clear, then run these commands from the lane root with a fresh
`workspace/tmp/w7` output area (preserve the existing evidence first):

```sh
bash scripts/gate-lease.sh bash workspace/tmp/w7/run.sh
bash scripts/gate-lease.sh python3 workspace/tmp/w7/analyze.py
```

`run.sh` builds both source arms, preserves the original pinned input, copies it
per trial and runs all ten in their published order. Both use `-trimpath
-buildvcs=false`. The scripts preserve all outputs and reject changed input,
incomplete memo misses, refusals and graph/snapshot/typed output differences.
They run no ingest or deployment. Per-file source identities and commands are
retained so the same baseline can be reconstructed without reinvention.

Separate critic and architect reads are still required. No full gate, native plan
verification, push, deployment, restart, release or plan-row completion is claimed.
The integration owner owns any observable-performance changelog fragment because
this lane's path allowlist excludes the changelog. This diff must remain outside
normal land bundles until the architect reviews it.
