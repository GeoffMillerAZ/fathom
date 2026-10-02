# EW-51: ingest cost, whole against summed (wave ic, item linear; O362)

**Verdict, in plain words.** A whole-repository ingest costs what its parts cost again. On the LB-M5
polyglot set the whole tree now retires 704.2 G instructions against 728.1 G for its 17 subjects ingested
one at a time (0.97x), where it retired 1,419.4 G against 784.5 G (1.81x) before. Nearly all of the
excess was one pass: the doc-link pass (KR-6) looked up each span name with a query that scanned every
symbol of the service, so its cost was names times symbols. It now reads the service's symbols once per
pass (EW-49), and its doc_links reconciliation reads the open links once per pass instead of once per
unit (EW-50). Every row the passes write is byte-identical. The ingest report now times each pass
(EW-48), which is how the next regression of this kind is found without a profiler. One pass is still
above its subjects' sum, centrality (1.5x CPU, 3% of the ingest; O410).

## Method

Measured on darwin/arm64 (10 cores), go 1.26.3, `GOWORK=off CGO_ENABLED=1 go build ./cmd/fathom`
(no `-ldflags`), three builds of branch `ic-linear`:

- **before** = c8a99868: base 0fd24ac2 plus EW-48's pass timing only (no change to what any pass does),
  so the before runs carry `pass_times`;
- **EW-49** = 44a1a626: the doc-link pass's one symbol read;
- **after** = e9dc2e9a: EW-49 plus EW-50, the branch's code.

**Input.** footprint.md's LB-M5 polyglot set (2026-10-01 section): LB-M1's 18 dialect subjects at the
SHAs `docs/bench/readings/lang/baseline/*.json` record (17 repositories; fmtlib/fmt serves CMake and
C++), each shallow-fetched by SHA and written with `git archive` into one directory per subject: 6,548
files; `ingest --dry-run` discovers 6,524 files and 75,971,918 bytes, as LB-M5 recorded. For the summed
reading each subject is ingested alone from a directory holding only it (nested one level, so a
subject's own `go.mod` is never a service root and the repo-relative paths equal the whole tree's).

**Runs.** `/usr/bin/time -l nice -n 10 fathom ingest --json --store <fresh sqlite> --service poly --path
<tree>`, each run with a fresh store, `FATHOM_HOME` and `FATHOM_CACHE_ROOT`; the whole tree twice per
build (the second valid run is the reading), then each subject once. `scripts/tcc-keeper.sh` ran
throughout. Load average (1 min) 9.7 to 18.0, sibling waves' gates and this item's own verify ingest;
the bar reads instructions retired, which load does not move; CPU and wall are reported and are noisier
(the after whole runs sat at load 17 to 18, the subjects at 12.5 to 18).

**Validity.** A run counts only when `read_failed` is 0, the doc pass neither failed a unit nor was
skipped, and the store's row counts (observations, typed_edges, file_imports, doc_links, doc_units,
symbol_centrality, service_routes, fingerprints) equal a complete reference run's: whole tree
`obs 85299, typed 30098, imports 4198, links 1880, units 3226, centrality 74907, routes 734, fps 6524`,
and per subject the before build's. The row-count check is new: the external volume's EPERM wedge hit
most whole-tree runs on 2026-10-02, and a run whose file-resolved pass silently skipped its re-reads
passed footprint.md's rule (`read_failed: 0`, no failed doc unit) at 443.1 G (O411). Of the whole-tree
attempts, 18 of 21 before, 6 of 8 at EW-49 and 2 of 4 after were discarded on these grounds; every
reading below passed all three checks (the three valid before runs read 1,417.1, 1,418.0 and 1,419.4 G).

## Whole against summed

| Build | whole instructions | summed instructions | whole / summed | whole CPU | summed CPU | whole wall | summed wall |
|---|--:|--:|--:|--:|--:|--:|--:|
| before c8a99868 | 1,419.4 G | 784.5 G | **1.81** | 93.88 s | 62.57 s | 109.21 s | 70.66 s |
| EW-49 44a1a626 | 715.7 G | 723.3 G | 0.99 | 45.02 s | 45.74 s | 49.02 s | 48.20 s |
| after e9dc2e9a | 704.2 G | 728.1 G | **0.97** | 66.79 s | 64.27 s | 94.84 s | 80.16 s |

The first valid whole runs agree: before 1,418.0 G (102.45 s CPU), after 703.5 G (59.22 s CPU). The
in-process profile runs below read 1,424.0 G (0fd24ac2) and 718.7 G (EW-49). The whole tree's cost fell
by 715.2 G (50.4%); the summed cost fell by 56.4 G, the same quadratic term inside each subject
(okhttp 121.3 to 101.6 G, Polly 86.4 to 75.4 G). The CPU rows of the EW-49 build (load 7.4 to 13.6) are
the quiet-machine reading: 0.98x.

## Per pass (EW-48's `pass_times`), CPU seconds, whole against summed

| Pass | before whole | before summed | x | after whole | after summed | x |
|---|--:|--:|--:|--:|--:|--:|
| discover | 0.68 | 0.83 | 0.82 | 0.86 | 0.92 | 0.94 |
| extract | 21.81 | 28.99 | 0.75 | 32.89 | 32.34 | 1.02 |
| &nbsp;&nbsp;store_writes | 5.76 | 6.04 | 0.95 | 9.14 | 6.72 | 1.36 |
| typed_go | 0.00 | 0.00 | | 0.00 | 0.00 | |
| cue | 0.00 | 0.00 | | 0.00 | 0.00 | |
| file_resolved | 19.37 | 24.29 | 0.80 | 28.32 | 26.45 | 1.07 |
| &nbsp;&nbsp;resolve | 0.43 | 0.59 | 0.73 | 0.67 | 0.64 | 1.05 |
| doc_xref | 0.33 | 0.41 | 0.81 | 0.49 | 0.48 | 1.04 |
| &nbsp;&nbsp;image_ref | 0.01 | 0.01 | | 0.01 | 0.01 | |
| route_ref | 0.26 | 0.32 | 0.79 | 0.34 | 0.36 | 0.93 |
| doc_index | 49.48 | 5.22 | 9.48 | 1.15 | 0.92 | 1.24 |
| &nbsp;&nbsp;doc_links | 48.88 | 4.71 | **10.39** | 0.41 | 0.35 | 1.16 |
| centrality | 1.60 | 1.30 | 1.23 | 2.35 | 1.55 | **1.52** |
| commit | 0.12 | 0.19 | 0.63 | 0.17 | 0.22 | 0.77 |

CPU is the process's (getrusage), read at each pass's ends. The after whole runs ran at load 17 to 18
beside this item's verify ingest, which inflates every after-whole CPU figure against the after-summed
ones run later at lower load; the EW-49 build's quieter runs read extract 21.35 / 22.96, file_resolved
18.97 / 18.58, store_writes 5.75 / 4.48 (1.28x), centrality 1.46 / 1.03 (1.42x) and doc_links 1.41 / 0.40
(3.50x), the term EW-50 then removed. Before EW-49, `doc_links` alone was 48.9 of the whole run's
93.9 s.

## Profiles (whole tree, in-process `cmdIngest` under `go test -cpuprofile`, scratch only)

- **0fd24ac2** (1,424.0 G, 89.76 s of samples): `store.observationsForSymbolRaw` 38.08 s cumulative
  (42.4%), reached only from `docindex.(*storeResolver).ResolveSymbol` -> `resolveSymbolNow`; in it
  SQLite's `_likeFunc` 11.88 s and `_patternCompare` 9.40 s, the `symbol GLOB '*.name'` test run on every
  row of `idx_observations_service_symbol` for every distinct span name. The rest: `runtime._ExternalCode`
  (the grammars' wasm and the CGO parsers) 21.01 s flat, syscalls 13.53 s, scheduler wakeups 7.72 s,
  the extract stream 10.99 s, `PutObservations` 6.54 s, the file-resolved pass 2.06 s, centrality 1.15 s
  (of which `TransitiveCallerCounts` 0.14 s).
- **44a1a626, EW-49** (718.7 G, 42.48 s): `runtime._ExternalCode` 17.77 s, syscalls 8.68 s, scheduler
  wakeups 7.32 s; the extract stream 7.92 s with `PutObservations` 4.98 s, the file-resolved pass 1.90 s,
  centrality 1.11 s, the doc pass 0.90 s of which `LinkUnits` 0.54 s and `SyncDocLinksForUnit` 0.42 s
  (the per-unit open-links scan EW-50 removes). No `ObservationsForSymbol` frame.
- **e9dc2e9a, after** (705.8 G, 52.27 s of samples, run beside this item's verify ingest and gate):
  `runtime._ExternalCode` 23.85 s, syscalls 11.40 s, scheduler wakeups 6.31 s; the extract stream
  10.01 s with `PutObservations` 6.19 s, the file-resolved pass 2.40 s, centrality 1.73 s (`BuildBoth`
  0.73 s, `TransitiveCallerCounts` 0.22 s), the doc pass 0.78 s, of which `LinkPass.Link` 0.22 s,
  `SymbolHashes` 0.04 s and `DocLinkSync.SyncUnit` 0.05 s.

## Output identity

Sorted rows of typed_edges, file_imports, doc_links (less its `valid_from`/`valid_to` clock columns) and
symbol_centrality, dumped from fresh stores and compared with `cmp`:

- the 0fd24ac2 build against 44a1a626 and against e9dc2e9a on libuv/libuv (5,266 typed_edges, 374
  file_imports, 6 doc_links, 5,334 symbol_centrality rows), square/okhttp (11,800, 689, 151, 15,696) and
  App-vNext/Polly (77, 802, 149, 6,440): **every diff empty**;
- the whole tree, c8a99868 against e9dc2e9a (30,098 typed_edges, 4,198 file_imports, 1,880 doc_links,
  74,907 symbol_centrality rows): **every diff empty**.

No `ExtractorEpoch` bump: extraction output is unchanged.

## What remains

- **centrality** (O410): 1.42x to 1.52x whole against summed; mostly `callgraph.BuildBoth` resolving
  each call name against more same-named symbols in a bigger service (a different graph, not a
  performance defect), plus the closure's dense bitset per SCC component, O(components^2/64), 0.10 to
  0.14 s at 74,907 components. 3% of the ingest; revisit trigger in O410.
- **store_writes** 1.28x to 1.36x: `PutObservations` into one larger set of tables and indexes, the
  B-tree's log factor and page-cache misses; linear in rows written.
- **O361** (item oneparse): the file-resolved pass's second parse of each tags-dialect file, a constant
  factor per file, not a whole-versus-summed term.
- **O411**: the later passes' re-reads skip an unreadable file silently; footprint.md's validity rule
  needs the row-count check this reading used.

EW-51 verdict: PASS (whole/summed = 0.97 instructions: 704.2 G against 728.1 G over 75,971,918 bytes; before 1.81, 1,419.4 G against 784.5 G)
