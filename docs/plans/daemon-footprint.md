# The daemon's memory footprint — instrument, reproduce, fix (O108)

O108 (`.fathom/observations.md`): the always-on `fathom serve-http` daemon (21
typed-edge services, auto-refresh on git-HEAD drift) grew ~0.9 GB/hour, reaching
12 GB (11 GB compressed, 623 MB RSS — "the rest paged out") after 9.4h, twice in
one day, killing the pre-push gate's low-memory watchdog. `/metrics` carried no
memory or goroutine series and there was no `/debug/pprof`, so the growth could
not be attributed from outside. Grammar: `docs/plans/README.md`. Verify:
`fathom verify --service fathom --plan docs/plans/daemon-footprint.md`.

## A. Instrument (ship regardless of what the evidence shows)

- [x] `/metrics` gains `go_memstats_heap_inuse_bytes`, `go_memstats_heap_sys_bytes`, `go_memstats_sys_bytes`, `go_goroutines`, `process_uptime_seconds`, plus `fathom_graph_memo_entries`/`_hits_total`/`_misses_total` (the watermark-keyed call-graph memo), `fathom_open_stores`, `fathom_autorefresh_total`, `fathom_typed_pass_total` — every prior metric line is kept {id: DF-1, symbol: newHTTPHandler, package: cmd/fathom, reachable_from: TestServeHTTPMetrics}
- [x] `net/http/pprof`'s handlers (index, heap, goroutine, allocs, profile, cmdline, symbol, trace) are mounted under `/debug/pprof/` on the daemon's own mux — loopback-only, same unauthenticated trust boundary as every other route (ADR-014 amendment: surfaces are not a security boundary), documented in `docs/mcp-tools.md`'s HTTP surfaces list and `docs/guides/fleet-index.md`'s daemon ops {id: DF-2, symbol: newHTTPHandler, package: cmd/fathom, reachable_from: TestServeHTTPPprofMounted}
- [x] `fathom doctor` gains `daemon_footprint`: reads the daemon's own `/metrics`, reports heap in-use and uptime, WARNs (never FAILs) above a 2 GiB budget naming O108 and the restart remedy; SKIPs honestly when the daemon is unreachable or predates these fields {id: DF-3, symbol: checkDaemonFootprint, package: cmd/fathom, reachable_from: TestCheckDaemonFootprint_AboveBudget_Warns}

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
- [ ] Regression: the call-graph memo never exceeds its configured capacity and `heap_inuse` stays within 2x of its post-fill level across 300 sequential large-graph (10k-node) misses spanning 15 services against a cap-8 cache {id: DF-6, symbol: TestCachedCallGraph_SequentialChurnDoesNotLeak, package: internal/analysis/callgraph, skip_test: true, skip: "the row's symbol IS the test"}
- [ ] Regression: the same boundedness holds under concurrent load — 50 rounds of 12-way-simultaneous misses on the same (service, watermark) key (the daemon's own real operating condition serving many parallel agent sessions), which every sequential driver could not exercise; proves a "thundering herd" of redundant concurrent builds (`Graph()`'s `build()` runs outside the lock by design) does not become a permanent floor {id: DF-7, symbol: TestCachedCallGraph_ConcurrentMissesDoNotLeak, package: internal/analysis/callgraph, skip_test: true, skip: "the row's symbol IS the test"}

## Status

Not closed. The synthetic curves this round produced are flat (Task 2/3's own
closure bar), but that is a NEGATIVE result against the named candidates, not
a positive identification of the allocation site that reached 12 GB in
production — see O108 for the full honesty accounting. The scavenger (DF-5)
is a defensible, evidence-directed mitigation (it targets the ONE signature
every driver reproduced: `heap_sys` never decreasing) rather than a fix for a
located leak. Re-measure `top -o cmprs` / `/metrics` `go_memstats_heap_sys_bytes`
against the deployed daemon at +9h and update O108's Status before this plan's
rows are treated as the end of the story.
