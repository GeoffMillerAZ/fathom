# The daemon's memory footprint — instrument, reproduce, fix (O108)

O108 (`.fathom/observations.md`): the always-on `fathom serve-http` daemon (21
typed-edge services, auto-refresh on git-HEAD drift) grew ~0.9 GB/hour, reaching
12 GB (11 GB compressed, 623 MB RSS — "the rest paged out") after 9.4h, twice in
one day, killing the pre-push gate's low-memory watchdog. `/metrics` carried no
memory or goroutine series and there was no `/debug/pprof`, so the growth could
not be attributed from outside. Grammar: `docs/plans/README.md`. Verify:
`fathom verify --service fathom --plan docs/plans/daemon-footprint.md`.

## A. Instrument (ship regardless of what the evidence shows)

- [x] `/metrics` gains `go_memstats_heap_inuse_bytes`, `go_memstats_heap_sys_bytes`, `go_memstats_sys_bytes`, `go_goroutines`, `process_uptime_seconds`, plus `fathom_graph_memo_entries`/`_hits_total`/`_misses_total` (the watermark-keyed call-graph memo), `fathom_open_stores`, `fathom_autorefresh_total`, `fathom_typed_pass_total` — every prior metric line is kept {id: DF-1, symbol: newHTTPHandlerDeps, package: cmd/fathom, reachable_from: TestServeHTTPMetrics}
- [x] `net/http/pprof`'s handlers (index, heap, goroutine, allocs, profile, cmdline, symbol, trace) are mounted under `/debug/pprof/` on the daemon's own mux — loopback-only, same unauthenticated trust boundary as every other route (ADR-014 amendment: surfaces are not a security boundary), documented in `docs/mcp-tools.md`'s HTTP surfaces list and `docs/guides/fleet-index.md`'s daemon ops {id: DF-2, symbol: newHTTPHandlerDeps, package: cmd/fathom, reachable_from: TestServeHTTPPprofMounted}
- [-] `fathom doctor` gains `daemon_footprint`: reads the daemon's own `/metrics`, reports heap in-use and uptime, WARNs (never FAILs) above a 2 GiB budget naming O108 and the restart remedy; SKIPs honestly when the daemon is unreachable or predates these fields {id: DF-3, disposition: refused, reason: "superseded by responsible-tenant RT-49 (ADR-047): doctor judges phys_footprint from the resource ring against the RAM-derived budget and never SKIPs an unreachable daemon into silence; the fixed 2 GiB budget and the restart remedy are gone"}

## B. Reproduce with evidence

Synthetic drivers against a scratch daemon (port 7690) over a read-only backup
of the shared store, `/debug/pprof/heap` sampled across each run, `go tool
pprof -base` diffed between the earliest and latest sample. Four HTTP-level
drivers (sequential auto-refresh churn on a tiny repo × 30 iterations;
LRU-eviction churn round-robining 19 real fleet services × 6 passes, well past
the memo's cap of 8; 3000 pure `obligations` calls with no ingest at all; 25
REAL typed-edge re-ingests of a 41-package synthetic service) and two
in-process stress tests at a scale no HTTP driver could reach in reasonable
time (300 sequential misses and a 50-round concurrent "thundering herd" of
12-way-simultaneous misses, both against 10k-node graphs — comparable to this
repo's own ~10.6k symbols) — none reproduced sustained, unbounded growth.
`go_memstats_heap_inuse_bytes` and the in-process `runtime.MemStats.HeapInuse`
consistently plateaued or oscillated within a bounded band once the graph
memo's cap-8 entries filled, across sequential churn, forced LRU eviction,
pure call volume, real re-ingest repetition, and concurrent thundering-herd
misses alike. `go_memstats_heap_sys_bytes` (the OS-reservation high-water
mark), by contrast, never decreased in any run — the one signature every
driver reproduced without exception, and the one O108's own numbers name
directly (12 GB MEM / 11 GB compressed vs. 623 MB RSS: memory that was
touched once and is no longer live, but that the runtime's own lazy background
scavenger had not yet returned to the OS, so macOS held it inactive and
compressed rather than free).
  - [x] The evidence and root-cause conclusion (every named candidate — the graph memo, the typed pass, store handles, the call logger, goroutines — empirically ruled out; the heap_sys high-water-mark signature reproduced in every run) are recorded in O108 {id: DF-4, oracle: lexical, path: .fathom/observations.md, pattern: "^### O108 —"}

## C. Fix what the evidence shows

- [x] The daemon returns freed heap memory to the OS on a fixed cadence (`debug.FreeOSMemory`, default 15 min, `FATHOM_SCAVENGE_INTERVAL` to tune, `FATHOM_SCAVENGE=off` to disable) instead of relying on Go's own lazy scavenger — bounds how long any one peak (a large graph build, a burst of concurrent cache misses) can inflate the daemon's OS-visible footprint across a 24/7 lifetime {id: DF-5, symbol: runMemoryScavenger, package: cmd/fathom, reachable_from: TestRunMemoryScavenger_CallsFreeOnEveryTick}
- [-] Regression: the call-graph memo never exceeds its configured capacity and `heap_inuse` stays within 2x of its post-fill level across 300 sequential large-graph (10k-node) misses spanning 15 services against a cap-8 cache {id: DF-6, symbol: CachedCallGraph.Graph, package: internal/analysis/callgraph, reachable_from: TestCachedCallGraph_SequentialChurnDoesNotLeak, disposition: refused, reason: "CachedCallGraph has had no production caller since RT-74 moved the graph memo into internal/memo (GraphFor, internal/memo/graphstore.go); blast_radius kind delete on CachedCallGraph.Graph at 9ea6da5f reads 0 typed referents and only internal/analysis/callgraph's own tests reach it, so this regression bounds a cache the daemon no longer uses"}
- [-] Regression: the same boundedness holds under concurrent load — 50 rounds of 12-way-simultaneous misses on the same (service, watermark) key (the daemon's own real operating condition serving many parallel agent sessions), which every sequential driver could not exercise; proves a "thundering herd" of redundant concurrent builds (`Graph()`'s `build()` runs outside the lock by design) does not become a permanent floor {id: DF-7, symbol: CachedCallGraph.Graph, package: internal/analysis/callgraph, reachable_from: TestCachedCallGraph_ConcurrentMissesDoNotLeak, disposition: refused, reason: "CachedCallGraph has had no production caller since RT-74 moved the graph memo into internal/memo (GraphFor, internal/memo/graphstore.go); blast_radius kind delete on CachedCallGraph.Graph at 9ea6da5f reads 0 typed referents and only internal/analysis/callgraph's own tests reach it, so this concurrent-miss regression bounds a cache the daemon no longer uses"}

## D. W-N (2026-09-23): bound the peaks — O108 reframed

Production evidence (O108's 2026-09-23 addendum) reframed root cause: not an
unattributed leak, but idle heap left behind by PEAKS — typed-edge passes
over large services (`app-kit` ≈390 packages; one session issued 19 `ingest`
calls on it in 20 minutes while the daemon grew 1.4 → 5.2 GB) — that darwin
reclaims lazily (compress, later swap) rather than immediately. W-L's
scavenger (§C, DF-5) already bounds how long a peak's OS-visible footprint
survives; this section bounds the peaks themselves — how large one can grow,
and how many can happen at once.

- [x] A process-wide semaphore serializes the typed-edge pass: only one runs at a time anywhere in the process; a second acquire that arrives while the first is held waits, the contended wait is counted (`fathom_typed_pass_waited_total`), and the inflight gauge (`fathom_typed_pass_inflight`, exposed on `/metrics`) returns to 0 once every holder has released {id: DF-8, symbol: acquireTypedPassSlot, package: internal/fusion, reachable_from: TestAcquireTypedPassSlot_SerializesAndCounts}
- [x] ctx cancellation while waiting for the slot returns `ctx.Err()` promptly instead of blocking for the holder to release, and the cancelled waiter never acquires (the gauge stays owned by the original holder throughout) {id: DF-9, symbol: acquireTypedPassSlot, package: internal/fusion, reachable_from: TestAcquireTypedPassSlot_CancelWhileWaiting}
- [x] Two real typed-edge passes over the small `testdata/typedfixture` module, run concurrently and each bracketed by the same acquire/release pair `Pipeline.enrichTypedEdges` uses in production, never overlap — the second's guarded window starts only after the first's ends {id: DF-10, symbol: EnrichTypedEdges, package: internal/fusion, reachable_from: TestTypedPassSemaphore_SerializesConcurrentFixturePasses}
- [x] The typed pass's own early-skip guard (`DisableTypedEdges`, no Go source, no `go.mod`) returns before ever touching the semaphore — a skipped service never inflates `fathom_typed_pass_inflight` {id: DF-11, symbol: Pipeline.enrichTypedEdges, package: internal/fusion, reachable_from: TestEnrichTypedEdges_ServiceSkipsSemaphoreGate}
- [x] `/metrics` gains `fathom_typed_pass_inflight` (0/1 gauge) and `fathom_typed_pass_waited_total` (counter) beside the existing `fathom_typed_pass_total`, plus `go_memory_limit_bytes` (below) {id: DF-12, symbol: newHTTPHandlerDeps, package: cmd/fathom, reachable_from: TestServeHTTPMetrics}
- [x] `[daemon].memory_limit` parses Go size syntax (`"3GiB"`, `"512MiB"`, `"1.5GiB"`, IEC units only) and the literal `"off"`, both shape-checked by `schema.cue`'s pattern and re-checked at load time {id: DF-13, symbol: ParseMemoryLimit, package: internal/config, reachable_from: TestParseMemoryLimit_Valid}
- [x] A malformed `memory_limit` value is refused outright — no silent fallback to the default for a typo, a decimal-SI unit, a negative, or a zero value {id: DF-14, symbol: ParseMemoryLimit, package: internal/config, reachable_from: TestParseMemoryLimit_Refused}
- [-] `FATHOM_MEMORY_LIMIT` (env) wins over `[daemon].memory_limit` (config), which wins over the `"3GiB"` built-in default — the same env-wins-over-config precedence `FATHOM_CALL_LOG` already follows {id: DF-15, disposition: refused, reason: "superseded by responsible-tenant RT-30 and RT-36 (ADR-047): [daemon].memory_limit and FATHOM_MEMORY_LIMIT migrate into the user-scope [tenancy] budget and the GC hint defaults to the exit mark; there is no fixed 3 GiB default"}
- [x] The resolved limit is actually applied to the runtime with `runtime/debug.SetMemoryLimit` at daemon startup, and reads back correctly via `debug.SetMemoryLimit(-1)`'s query-only negative-input contract; `"off"` never calls `SetMemoryLimit` at all {id: DF-16, symbol: applyMemoryLimit, package: cmd/fathom, reachable_from: TestApplyMemoryLimit_ActuallyAppliesToRuntime}
- [x] `/metrics` gains `go_memory_limit_bytes`, read live from the runtime (never a cached copy of what startup applied) {id: DF-17, symbol: memoryLimitMetricLine, package: cmd/fathom, reachable_from: TestMemoryLimitMetricLine}
- [-] `fathom doctor`'s `daemon_footprint` row names the configured memory limit (or `"off"`) in its reason, without forcing a SKIP on a daemon whose `/metrics` predates this wave (the limit segment is appended only when the metric is present) {id: DF-18, disposition: refused, reason: "superseded by responsible-tenant RT-49 (ADR-047): the doctor footprint row now reports the budget and ladder level, not a configured memory limit"}

## Status

Mitigated-and-bounded (was: not closed). The 2026-09-23 production reading
(O108's addendum) confirms the mechanism this round's synthetic drivers could
not pin down — peaks + darwin's lazy reclamation, not a live leak — and
confirms the scavenger (DF-5) closes the OS-visible gap it targets
(`top -stats mem,cmprs` 44 MB / nothing compressed after a pass that had left
`heap_sys` 753 MB and RSS 765 MB; `ps rss` is the wrong instrument to judge it
by — it counts `MADV_FREE`'d pages as resident until the kernel actually
reclaims them). §D adds two bounds ON the peaks themselves, not just their
cleanup: DF-8..11 cap how many typed-edge passes can run at once
(serialization — directly answering the 19-calls-in-20-minutes/1.4→5.2 GB
reading); DF-13..18 cap how large the heap is allowed to grow toward before
the runtime itself starts working harder to shrink it (`memory_limit`, soft,
default 3 GiB).

Not claimed fully closed: this observation's own prior closure bar — a
sustained +9h production reading (`top -o cmprs` / `/metrics`
`go_memstats_heap_sys_bytes` against the deployed daemon's own value at
start) — is still pending a sampler run with all three mitigations
(scavenger + semaphore + memory limit) live; the 2026-09-23 reading is a
single-pass snapshot, not that curve. Update O108's Status again once the
sampler confirms it.
