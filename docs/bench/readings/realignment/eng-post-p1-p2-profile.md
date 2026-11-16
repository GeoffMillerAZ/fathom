# W12-1a: CPU reading after accepted P1/P2

**Verdict: INCONCLUSIVE for another permitted change that would meet the one-second bar.**
Five native CPU captures authenticate the accepted production path and locate
remaining sampled cost. They do not establish an additional speedup. The narrow
remaining changes identified below have small or unmeasured shares. This evidence
does not justify promising a sub-second load inside the current freeze.

The prior unprofiled primary median remains **1.496902958 s → 1.250227709 s**;
the separate full-load median remains **1.775724458 s → 1.555657042 s**. Every
accepted-source trial exceeded one second. Those are the retained W7 FIX2
measurements, not new measurements in this lane. ENG-1 deployed acceptance and
plan rows remain open.

## Authority and scope

W7-1 admits P1/P2, freezes P3's storage index, and does not pursue P4's decoder
optimization. W12-1a permits further load-cost investigation without adding stored
or resident structure. W15-1 accepts production `a56ab63a6` and requires the
Graph-field clone guard. Capture source is guard commit
`ddcd305ed22fbd59bdf2542d956083fcfd1ab8b2`, tree
`8ed84d51efa617e316e25c361f3d9fab1b00f6f4`. Its three P1/P2 production blobs are
byte-identical to accepted `a56ab63a6` and the retained W7 FIX2 candidate.

Applicable documented constraints are deterministic graph results (I1),
watermark-scoped memoization (I21), and singleflight plus byte accounting (I51).
ADR-095 remains proposed and conditional on OQ5 and the tenant-clause reading.
This profile supplies no evidence that lifts that freeze or fires its acceptance
conditions. No stored/resident structure, worker reload, RT-197 repair, fifth
patch scheme, decoder change, deployment or new contract is proposed here.
All cost-removal observations below are analysis only.

Only this new reading is tracked. All instrumentation, copied source, binaries,
profiles and stores are ignored scratch under `workspace/tmp/w12-profile`.
Source, old reports, archived evidence and the pinned input were read-only.
No full gate, ingest, paid work, daemon operation or subagent ran.

## Method and boundaries

The W6 indexed admitted-load profiling method is reused over W7 FIX2's W5 serving
harness and full-field serializer. Original W5/W6 artifacts were read from
`../out/evidence-bundle4/measurement-251/workspace`; W7 harness/raw references
were read from `../engine-load-cost/workspace/tmp/w7-fix2`.

A `git archive` of the exact capture commit supplies product source. Only scratch
`resolved_nav.go` receives the existing four phase marks plus a phase label.
The other three harness/export/entrypoint files are scratch additions. A
file-by-file comparison against every tracked file before building finds only
that instrumented navigation file different. `profile-only.diff`, `identity.json`
and the source archive retain the exact changes and bytes. No product algorithm
is replaced or bypassed.

Each of five predetermined trials is a fresh process and receives its own
byte-identical copy of the same W5 store. There is no warmup, discarded capture,
retry of a capture, old-cohort rerun or OS page-cache flush. Native
`runtime/pprof` samples at 10 ms. Profiling starts before admission and scope
resolution, labels snapshot/syntax/BOTH, and labels the real backend symbol and
typed reads. Memo builder goroutines inherit those labels. As in W6, CPU
profiling stops at the completed BOTH boundary, before resolved loading and
canonical output serialization.

The primary wall marker is taken immediately after BOTH returns. W7's common
boundary memory/memo sampler then runs before `StopCPUProfile`, so this capture
window also includes that small harness-accounting interval. No
`runtime.ReadMemStats` frame was sampled; zero samples is not proof of zero
accounting overhead. Native profile duration also covers profiler shutdown.
Unlabelled background runtime activity stays unassigned to a product phase.
No resolved-builder or output-serializer frame occurs in the native samples.

Primary means admission + scope + snapshot + syntax + BOTH, including typed
loading. Full-load wall additionally includes common memory sampling, profiler
stop/flush, and resolved loading. It excludes later output serialization.
Process time/RSS in `stderr.txt` also includes serialization. The profile is
therefore **primary-scoped CPU**, not a profile of the resolved extra or a new
full-load latency experiment.

All archive extraction, source/harness preparation, Go build, input copying,
profiling, pprof processing and evidence hashing held the shared **EX** gate
lease. No SH package pool was used. `GATE-RUNNING` was clear at checked operation
boundaries. The scripts stop with HOLD before subsequent heavy work if it appears.
`lease-holder.txt`, environment and per-trial ambient receipts are retained.
The lease has since been released.

Environment: Go 1.27.4, darwin/arm64, cgo enabled, 32 GiB RAM, 12 logical CPUs,
GOMAXPROCS 6, managed 4 GiB budget, 1 GiB memo budget, 8 GiB GC hint, normal
level, enforce-mode admission. The existing harness extends only the tree
request deadline to ten minutes so a complete load, rather than a timeout,
is measured. This is a local helper reading, not a deployed outcome reading.

## Every capture and profiling overhead

Times below are instrumented observations, not new unprofiled timing claims.
Coverage is sampled CPU ns divided by the native profile's exact duration ns,
read from the retained protobuf, not from pprof's rounded display. It is not
percentage of functions covered, disk wait time, or a confidence interval.

| Capture | Primary ns | Full-load ns | Stop/flush ns | Native duration ns | Samples × 10 ms | Coverage |
|---|---:|---:|---:|---:|---:|---:|
| 01 | 1273472916 | 1738399916 | 157447875 | 1429061458 | 135 | 94.47% |
| 02 | 1259068125 | 1729035750 | 174071541 | 1431975750 | 133 | 92.88% |
| 03 | 1287860417 | 1748274709 | 135285250 | 1422394125 | 136 | 95.61% |
| 04 | 1287264333 | 1729329708 | 140373083 | 1426779125 | 139 | 97.42% |
| 05 | 1324963042 | 1725536708 | 100978166 | 1425097084 | 140 | 98.24% |

The profiled primary median is 1.287264333 s, 2.96% above the historical
unprofiled FIX2 median. Those cohorts were captured at different times, so that
is only an observed gap, **not an isolated profiler-overhead estimate**. No
unprofiled control was rerun. The directly measured stop/flush interval is
100.978166–174.071541 ms, median 140.373083 ms. It is excluded from primary
and included in full-load. Labeling, sampling, background work and startup
cost have not been isolated. Subtracting flush alone would not reconstruct an
unprofiled experiment.

Total: **683 samples = 6.83 sampled CPU seconds**, over **7.135307542 s** summed
native profile duration (95.72%). Per-capture primary values all remain above
one second. These numbers do not replace the earlier acceptance medians.

## Admission, misses and output identity

All five captures exit 0 with `valid=true`, successful admission, no context
error or stopped ticket, zero starting memo residency, exactly one symbol read
and one typed read. `ticket_bytes_read=0` is retained as emitted; the admission
and memo receipts plus actual backend call counts establish the path, not an
invented nonzero byte counter.

At primary completion, `(misses, flights, entries, hits)` is snapshot
`(1,1,1,0)`, graphstore `(2,2,2,0)`, typed edges `(1,1,1,1)`. At full completion
it is snapshot `(1,1,1,0)`, graphstore `(3,3,3,0)`, typed edges `(1,1,1,3)`.
Every group has zero declined, abandoned, refuse-cancelled, uncached builds and
evictions. Primary graphstore builds are syntax and BOTH; resolved is the
third build outside profiling. Typed hits are reuse of the one admitted typed
load, not a warm graph substitute.

Each trial has 104183 snapshot observations, 256033 typed edges, syntax
103099 nodes / 92411 edges, BOTH 103118 / 162606, and resolved 103118 / 120260.
All **25 complete canonical populations** are byte-identical to retained W7
FIX2 trial `02-p1p2`. This includes every Graph field and patch provenance in
the shared serializer, not just counts or selected edges.

| Population, same in all five captures | SHA256 |
|---|---|
| both | `e7a4a4a7764051b2c2462dcb938be9ad4928a765caf6a6ce08d604ef1f3088ac` |
| resolved | `cafaba53c5b445595ace544567269223e0734b475998057dab8e4072676b21d7` |
| snapshot | `106d44446d04dc54781a110e5e24f42a15bc9055e56346426e0121dbc3fab202` |
| syntax | `cf89006dad2a148dc3b384167850a2966f2938aa317ce97eb13a27474678afca` |
| typed | `1c07d1627ad3ac3f96ba8ecad7db5b2e2b97223f40de0c785a0a9719bf6510bf` |

## CPU attribution without overlapping shares

The following labels partition every native sample exactly once. Read labels
are removed from their containing phase before that phase's remainder is
reported. Thus typed loading is not counted again as combined work.
All shares use the same 6.83 s denominator.

| Exclusive owner | Sampled CPU s | Share |
|---|---:|---:|
| Symbol store read, including decoding | 1.09 | 15.96% |
| Snapshot work outside symbol read | 0.56 | 8.20% |
| Syntax phase | 1.33 | 19.47% |
| Typed store read | 1.60 | 23.43% |
| Combined phase outside typed read | 1.18 | 17.28% |
| Unattributed / unlabelled | 1.07 | 15.67% |
| Total | 6.83 | 100.00% |

The inclusive BOTH phase is 2.78 s / 40.70%, comprising typed read 1.60 s
and combined remainder 1.18 s. Snapshot including its read is 1.65 s / 24.16%.
The two store-read labels total 2.69 s / 39.39%. No scope/admission sample was
captured. These are sample-stack/label shares, not wall-time fractions or
measurements of storage latency. Reads may wait without consuming sampled CPU.

Native flat leaders are `syscall.rawsyscalln` 2.02 s / 29.58%,
`runtime.madvise` 1.82 s / 26.65%, `runtime.pthread_cond_wait` and
`runtime.tryDeferToSpanScan` each 0.23 s / 3.37%, and `runtime.(*mspan).init`
0.21 s / 3.07%. Flat means the sampled instruction's innermost function;
inclusive means a function occurs anywhere on that sample's stack, including
inlined frames. They must not be added together.

| Inclusive stack family / function | CPU s | Share | Relationship |
|---|---:|---:|---|
| `Rows.Next` | 2.14 | 31.33% | Inside the store reads |
| `Pread` | 1.99 | 29.14% | Nested under `Rows.Next`; 0.68 symbol + 1.31 typed |
| JSON decoding, `Rows.Scan` | 0.00 | 0.00% | No sampled frames, not proof of no work |
| `runtime.madvise` | 1.82 | 26.65% | Crosses all labelled phases; overlaps phase owners |
| `runtime.gcBgMarkWorker` | 0.49 | 7.17% | Subset of the unlabelled 1.07 s |
| Syntax `Build` | 0.67 | 9.81% | Subset of syntax's 1.33 s |
| `buildIndexes` | 0.23 | 3.37% | Nested in `Build` |
| `buildSuffixIndex` | 0.10 | 1.46% | Nested in `Build` |
| `buildCtorScopeIndex` | 0.05 | 0.73% | Nested in `Build` |
| `newSnapshot` | 0.14 | 2.05% | Subset of snapshot remainder |
| `WithTypedEdges` | 0.72 | 10.54% | Subset of combined remainder |
| `AddTypedEdges` | 0.63 | 9.22% | Nested in `WithTypedEdges` |
| Clone helpers, union of their stacks | 0.09 | 1.32% | Nested in `WithTypedEdges` |
| `CanReuseSyntax` | 0.10 | 1.46% | Combined eligibility check before clone |
| `dropTypedFileGuesses` | 0.09 | 1.32% | Nested in `AddTypedEdges` |
| `containsStr` | 0.09 | 1.32% | Nested in `AddTypedEdges` |
| `removeOne` | 0.02 | 0.29% | Nested in pruning; inverse adjacency removal |
| Graph `EstimatedBytes` | 0.01 | 0.15% | Existing accounting, not an extra graph build |
| Fallback `BuildBoth` | 0.00 | 0.00% | No sampled fallback resolution; clone route sampled |

For example, syntax includes 0.63 s of `madvise` samples whose system-stack
frames do not retain the application builder above them. `Build`'s 0.67 s
inclusive stack alone therefore undercounts the phase's allocation cost.
`madvise` is also 0.28 s under symbol read, 0.39 s under snapshot remainder,
0.20 s under typed read and 0.32 s under combined remainder. This is the same
1.82 s, not another 1.82 s to add. Unlabelled GC and scheduler samples cannot
be assigned causally to a specific builder. Native stack samples cannot
separate CPU execution from all system-call effects or establish a disk-wait
breakdown. Short captures leave decoding and other small operations unresolved.

## Smallest source-grounded removal observations

Each row identifies the nearest existing operation that could be removed or
reduced. It does not prescribe implementation or attribute its enclosing
inclusive share as a recoverable wall-time saving.

| Ref | Cost and source | Smallest removal or reduction to examine | Stored/resident structure and disposition |
|---|---|---|---|
| C1 | Store reads, 39.39% labels; `internal/store/sqlite_symbolnodes.go:16`, `sqlite_typededges.go:62` | Avoiding required row/page visits would remove the sampled `Pread` work. No valid full-load omission is identified. A storage-index route is P3. Bounded reservation of the existing typed result slice could reduce its growth, but would not remove the page reads. | P3 changes stored structure and is frozen. A bounded capacity hint uses the existing returned slice, no new structure, but has no isolated saving here. |
| C2 | JSON decoding, zero samples; `sqlite_symbolnodes.go:91` | Replacing per-row JSON decoding is the earlier P4 direction. | P4 is not pursued. This capture provides no positive cost evidence for it. |
| C3 | Snapshot remainder 8.20%, `graphmemo/graphstore.go:201` | Fewer transient `seen`/intern-map growth allocations could reduce snapshot allocation work. Removing required lowercase/name/callee population would change the existing snapshot contract. | Sizing existing temporary maps adds no stored/resident structure. No measured saving. Retaining extra precomputed state is outside the freeze. |
| C4 | Syntax phase 19.47%; `callgraph/graph.go:143,191` | Capacity hints for the existing unhinted build-index maps could reduce rehash/allocation work. Constructor indexing is already present and contributes only 0.73% inclusive here. | Same existing maps, no new structure. Retained capacity/byte budget would still need proof. No new index or performance claim. |
| C5 | Combined clone 1.32%; `callgraph/cache.go:216`, `replace_package.go:272–343` | Eliminating the clone would remove copying, but sharing mutable adjacency violates the established ownership contract. The existing clone helpers already reserve capacity. | No safe local removal identified. Adding shared/resident representation or changing ownership is outside this reading. P2 has already removed the second syntax resolution. |
| C6 | Eligibility 1.46%; `callgraph/cache.go:248–276` | Native source attribution places these samples in `n.Key()` construction. Omitting the reverse duplicate-aware equality check removes required coherence protection. Pre-retained node keys would add resident state. | The guard is required by accepted FIX2/W15. No permissible whole-cost removal identified; no validator bypass or stored key proposal. |
| C7 | Typed augmentation 9.22%, including pruning 1.32%; `graph.go:702,794` | Existing touched maps/result slices could be sized to reduce growth. The remaining inverse-edge removal calls `removeOne`, whose capacity-clamped append copies a slice (`replace_package.go:349–355`). Removal within an already owned adjacency slice could avoid that particular copy. | No additional stored/resident structure is needed for that narrow copy reduction. Its sampled enclosing removal share is only 0.29%, not a predicted gain. P1 already compacts the forward adjacency in place. Generic helper callers/ownership would need independent review before any change. |
| C8 | Accounting 0.15%; `callgraph/estimate.go:25` | Skipping `EstimatedBytes` removes required budget accounting; retaining a precomputed total would add graph state. | I51 stands. No permissible removal identified. |
| C9 | `madvise` 26.65% and unlabelled 15.67% | Reducing existing transient allocation/reallocation could reduce allocator/GC demand. No profile-backed single source edit removes this cross-cutting total. | No new cache, pool, resident structure, GC-policy change or background job proposed. Unlabelled work stays unattributed. |

The eligible observations are allocation reductions in existing temporary work,
not another eliminated graph build or a measured way to remove the dominant
required reads. The small directly named checks/copies are not evidence for the
roughly 0.25 s still missing from the prior primary median. Aggregate CPU shares
cannot be converted to a latency prediction. **The remaining approved space is
unpromising on this evidence; another sub-second claim is not supported.**

## Exact identities and raw evidence

The original pinned input is `../out/w5-2-pinned-scratch.sqlite`, SHA256
`e22f40d4dd467b0201badae263901741db23a81864c4f545c111b5c469ecd704`.
Every scratch copy has that hash before and after its capture; pinned before/after
receipts match. The original was never opened by the harness or ingested.

| Artifact, common to all captures | SHA256 |
|---|---|
| Exact ddcd source archive | `26ea37b9a5d920e67c10b685bcb7ccae1a08bedce1988dfb967757cb46ea8524` |
| Profile binary | `e4774dde87b527950c7213e6aeda85c4d676fdb195f386147b7136c7340c3be5` |
| `internal/analysis/callgraph/graph.go` | `f51a1db0efe9e8ddd89db57eb5720077257d9ed7035805a62ea882fed6bb1a08` |
| `internal/analysis/callgraph/cache.go` | `5b18e2f61c5a5b327ba4b87abf2319bb55ea1780dd1233198929a988b6e0e7d6` |
| `internal/graphmemo/graphstore.go` | `d1150a807c9f922cbcbc7d7a5e4c89ed8656050512b7c1bff0c953303c9a0dd8` |
| Scratch `internal/mcp/w5_reading.go` | `2ede5133646263bb02cf1885cd14502b2e2b4ac7215805bebb5093ec7aaf2d61` |
| Scratch `internal/mcp/resolved_nav.go` | `9eefcc4b5642372456f5eb0ae9da01fd1d2acfc7395ac8898609c030f3dc1f9e` |
| Scratch `internal/analysis/callgraph/w5_export.go` | `ede21a48bd73368bb7351931a02be726b3a5dd908a061eb6b9ddc05d15e54ef4` |
| Scratch `cmd/w5read/main.go` | `ff253640acc90a3072e32bd2d9b6542446eaa71ccd3072e72343859619c83b5e` |

Every capture below uses that source, harness, binary and input identity.

| Capture | Raw native profile, relative to scratch root | SHA256 |
|---|---|---|
| 01 | `results/01-current-profile/cpu.pprof` | `9ad7f25c25b039064a9f75895fd6dd0c42e9524ec15e703bf761a32d720d5d01` |
| 02 | `results/02-current-profile/cpu.pprof` | `fbaf6f1705ec5b05123458a0d01ee10057e10710242ffdca1a210b0b5fe6f091` |
| 03 | `results/03-current-profile/cpu.pprof` | `894cc78f97e0b072ecf6e9d410ad64295e8fe716126972a66e115d08b08951a5` |
| 04 | `results/04-current-profile/cpu.pprof` | `8e9c4f88bf396d7c30c014b478540f32ffd3a987920b6a814790b0c334de8c8d` |
| 05 | `results/05-current-profile/cpu.pprof` | `437d91bc3d7128f1f6e58150358ddb04f0911486fef11dc3b2835855db42bde1` |

The scratch root retains `source.tar`, extracted source, `identity.json`,
`profile-only.diff`, exact binary and build metadata, every result/counter,
input hash, full canonical output, native profile, raw/top pprof report,
process time/RSS and ambient receipt. `merged.pprof`, merged raw/tags/flat/
inclusive reports, native source annotations, `captures.json`, exact
`coverage.json`, `sample-analysis.json` and `SHA256SUMS` retain the analysis.
Use `coverage.json` for exact duration: `captures.json` also retains the earlier
parser's rounded native text duration and its approximate coverage fields.
The report's coverage table uses the exact protobuf values instead.

An initial preparation attempt failed before build/capture because the local
Python tar API lacked the `filter` keyword. Its script/log remain. One analysis
assertion initially expected two final typed hits; the real path and retained
W7 receipt have three (the resolved wrapper and builder both consult the typed
memo). The failed assertion/log and corrected script remain. All five original
captures are kept; none was rerun. Initial source-list reports could not resolve
trimmed import paths; preserved reports show that error. A scratch source-map
symlink supplies the exact hashed source for successful `source-*.txt` native
annotations. These are evidence-tool corrections, not product changes.

## Reproduction and proof limits

From the lane root, source the established lane environment and set
`FATHOM_GATE_LOCK` to the existing shared heavy lock. Wait for `GATE-RUNNING`
to clear. Preserve this scratch root first: scripts intentionally refuse to
replace an existing populated source or existing reading. For a fresh evidence
area with the same scripts and pinned inputs:

```sh
bash scripts/gate-lease.sh bash workspace/tmp/w12-profile/run.sh
bash scripts/gate-lease.sh python3 workspace/tmp/w12-profile/analyze.py
bash scripts/gate-lease.sh bash workspace/tmp/w12-profile/inspect.sh
bash scripts/gate-lease.sh bash workspace/tmp/w12-profile/inspect-source.sh
```

Build command inside the exact archived scratch source:

```sh
go build -trimpath -buildvcs=false -o ../bin/current-profile ./cmd/w5read
```

Native inspection uses the retained binary and profile, for example:

```sh
go tool pprof -top -cum workspace/tmp/w12-profile/bin/current-profile workspace/tmp/w12-profile/merged.pprof
```

Run such inspection under the same EX lease. `run-attempt2.log` records all five
successful capture exits. `analysis-attempt2.log` proves:

```text
PASS: 5/5 complete admitted cache-miss loads; exact boundary/final counters; no refusals, evictions or uncached builds
PASS: 25/25 canonical populations byte-identical to retained W7 FIX2 output
PASS: pinned input and 5 scratch copies retain expected SHA256 before/after
PASS: 683 native samples x 10ms = 6.83s; no resolved/output frames; disjoint partitions sum exactly
```

No test suite or full gate was run for this reading-only commit. Product tests,
normal integration gate/main status, deployed timing, a causal overhead estimate,
per-change speedups, and one-second acceptance are not established here.
Independent critic review is still required before any future bundle. This
reading does not approve itself and is not added to frozen bundle 6 or 7.
