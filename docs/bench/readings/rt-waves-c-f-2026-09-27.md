# Responsible tenant, waves C–F: the integrated head, live (2026-09-27)

Binary: the waves C–F integrated head `a01249d2` (rtC-integ: storage, tools, ingest,
console and hygiene lanes, the tenancy lint and the cost harness). Incidents: INC-23,
INC-24. Design: `docs/design/responsible-tenant.md` §4.8, §7, §15.2. Plan:
`docs/plans/responsible-tenant.md`. Earlier readings: `rt-wave-a-incident-replay-2026-09-26.md`,
`rt-wave-b-seatbelt-2026-09-27.md`.

## 1. Verdict first

**Keep waves C–F. The daemon is now a small tenant under the load that made it
30 GB.** In every run on the real fleet (256 MiB store, 21 services), the daemon's own
footprint stayed under 0.8 GiB. With its `go list` and type-checker children counted, the
highest reading was 0.96 GiB. The governor's ladder never left `normal`. No call was
refused or abandoned for memory, and no run came near the 6 GiB kill line.

- **The incident's `symbol_context` ×20 burst is fixed.** It now answers 20 of 20 in
  **1.7 s** at **61 MiB per call**, with a **0.72 GiB** peak. Repeating the burst warm costs
  **23 MiB per call** at a p50 of 159 ms. On the wave-A head the same burst took 53 s at
  3.8 GiB per call. On wave B it peaked at 1.00 GiB, and 6 of the 20 calls were abandoned
  at the 60 s deadline. What stays resident is the three services' call graphs. The memo
  registry accounts for 319 MiB of the 372 MiB heap in use, so this is budgeted cache,
  not a leak.
- **An auto-refresh storm drains one ingest at a time.** Seven drifted services queued
  at once and were refreshed one after another in 77 s. At most one ingest was in
  progress in each of 150 half-second samples. Reads answered in 2–10 ms throughout and
  said "queued" or "refreshing". The most file content any of those ingests held at once
  was 13 MiB, inside the 64 MiB window.
- **Costs do not grow with the fleet.** In the committed cost harness, 57 of the 59 tools
  cost the same at 1× and 10× fleet size. The two that grow are fleet-wide by
  definition: `cross_repo_blast_radius` and `fleet_drift`. They grow linearly, and at 10×
  they cost at most 17 MiB against a 512 MiB fleet ceiling.
- **The stress harness passes RT-M3's post-wave-D test.** Under a 1 GiB budget and 120 s
  of the harness's mixed load, the daemon peaked at 114 MiB with no over-budget exit.
  After five SIGKILLs, the lifecycle breaker put it in safe mode at start 4 and stopped
  it at start 6. The harness's older seatbelt verdict still reads FAIL, because this
  binary never goes over budget, so nothing exercises the seatbelt (§2.4).

**The live runs also found two ingest bugs and five harness bugs (§6).** The harness
bugs are fixed in `01c7e260`. The ingest bugs are open:

- **F1.** A read on a service that is still being onboarded makes that ingest fail for
  every file.
- **F2.** That failed ingest still records the service as indexed at HEAD, so
  auto-refresh never retries it.

Neither bug costs memory. Both are correctness and honesty problems for a wave-D
follow-up.

## 2. The stress harness (`scripts/tenancy-stress.sh`)

### 2.1 Setup

- **Daemon:** one unmanaged `serve-http` at a time on `127.0.0.1:18523` (never 7575). It ran with
  `--lifecycle-id tenancy-stress-<run>`, a scratch HOME and a `sqlite3 .backup` copy of
  `workspace/db/fleet.sqlite`, at the harness's default budget:
  `FATHOM_TENANCY_BUDGET=1GiB` (shed 768 MiB, refuse 1 GiB, exit 2 GiB). Admission was
  `observe`, the default.
- **Fixture:** 20,000 Go files, 100 files over 2 MiB and a gitignored 2 GiB cache
  directory, under `/Volumes/m2ext/tmp`. The default is 40,000 files and 5 GiB. It was
  scaled down because the external volume had 10 GiB free.
- **Load:** the harness's own mix for 120 s: 32 concurrent `list_services`, 16
  `obligations`, 4 `references` over `["*"]` and one `ingest` of the fixture. Then 5
  SIGKILL/relaunch cycles with a 20 s throttle.
- **Sampling:** a separate watchdog sampled `footprint -p` for the daemon and every
  descendant every 0.5 s, and would SIGKILL above 6 GiB. It never fired.

### 2.2 The first run found the harness broken

The first run (harness as committed) exited 1 without writing its reading. Its data:

- **The first 692 calls were all 503.** The harness's readiness probe is `curl -s`
  on `/healthz`, and without `-f` it counts a 503 as ready. `/healthz` answers 503 while
  the daemon opens its store (`storewait.go`), so the harness started its load about 2 s
  early. Every call in that window was refused daemon-wide by the store-wait handler.
  That handler behaved correctly.
- **The run's one `ingest` was among those 692 refusals,** so the run measured no ingest.
- **`resources` exited 1 and killed the harness.** `resources --id` reports exit 1
  when the ledger shows a crash loop, which the SIGKILL cycles produce. Under `pipefail`,
  that ended the harness before it wrote the reading.

§6 lists the fixes.

### 2.3 The second run (harness with the fixes)

| Measure | Value | Instrument |
|---|---|---|
| Max own footprint | **114 MiB** (119,603,992 B) | `/metrics fathom_footprint_bytes`, 1 s |
| Max own + children | **127 MiB**, 0 children | watchdog, `footprint -p`, 0.5 s |
| Ladder | `normal` for the whole run | watchdog, `fathom_tenancy_level` |
| Calls | 11,405 × 200; 16 × 503 and 16 unanswered, all in the relaunch cycles after the load | curl codes |
| Refusals during the 120 s load | **0** | curl codes, call log |
| `/metrics` reachability | 117 of 118 samples while the daemon was up (the miss was before the first start); 118/217 overall, because the cycles hold the daemon down for 5 × 20 s | 1 s curl |
| Lifecycle | 5 unclean exits: safe mode at start 4, breaker STOP at start 6 | `lifecycle.jsonl` |
| Over-budget exits and diagnostic captures | 0 and none (the ladder never reached refuse) | ledger, `diag/` |
| Tenant verdict (RT-M3 post-wave-D) | **PASS**: no over-budget exit, max own ≤ budget | harness |
| Seatbelt verdict (RT-M3 wave B) | FAIL: no diagnostic capture, and safe mode after start 3 | harness |

The seatbelt verdict expects the load to push the daemon over its budget. This binary
never gets close, so nothing exercises the seatbelt. SIGKILLs alone reach safe mode on
the start after the third counted exit, which is start 4.

Latency by class under the harness's 52-way concurrency, from the call log:

| Tool (class) | Calls | p50 | p95 | p99 | max | Queued p50 / p95 |
|---|---|---|---|---|---|---|
| `obligations` (tree) | 3,504 | 3 ms | 8 ms | 44 ms | 71 ms | 2 / 7 ms |
| `list_services` (fleet) | 7,024 | 161 ms | 284 ms | 315 ms | 1,172 ms | 151 / 275 ms |
| `references` over `["*"]` (fleet) | 876 | 403 ms | 475 ms | 551 ms | 1,438 ms | 355 / 426 ms |

Fleet calls spend most of their time queued, because an unmanaged 1 GiB daemon has one
fleet slot. Their work is short. The one `ingest` returned in 361 ms with an error
(§6, F1).

A second probe on the same store, with no concurrent reads, ran the fixture ingest after
warm-up:

- It finished in 2.1 s with 20,101 fingerprints.
- It skipped all 100 files over 2 MiB, which were never read.
- It ignored the `.cache` directory by default.
- The daemon and its children peaked at 127 MiB.

## 3. Cost per call, and whether it grows with the fleet

### 3.1 The committed cost harness (`internal/mcp/costtest`, generated fleet, 1× and 10×)

`go test -run 'TestEveryToolHasCostRow|TestPointIndexToolsScaleSublinearly|TestColdStampedeOneBuildPerKey'`
passed: every row was within its class ceiling at both scales, cold and warm.

- **Cold allocations at 10×:** point at most 0.32 MiB, index at most 1.43 MiB, tree at
  most 1.41 MiB, ingest-class tools at most 1.16 MiB.
- **Flat in fleet size:** 57 of the 59 tools stay within 1.5× between 1× and 10×, with a
  0.25 MiB floor.
- **The two that grow are fleet-wide:**
  - `cross_repo_blast_radius`: 1.78 → 16.96 MiB cold, 0.15 → 0.98 MiB warm.
  - `fleet_drift`: 1.08 → 10.64 MiB.

  Both grow linearly with the number of services, which is their job, and stay far
  under the 512 MiB fleet ceiling.
- `list_services` goes from 0.02 to 0.15 MiB, which is under the floor.
- **Stampede:** 32 cold callers cause one build per memo key, and cancelling them all
  returns the goroutines to baseline.

### 3.2 The real fleet (4 GiB budget, sequential, after warm-up)

The fleet is the store copy, and the budget is the managed default on this 32 GB
machine. Each tool was called once cold, then 5 times warm, one call at a time.
Allocation is the change in `TotalAlloc` from `/debug/pprof/allocs`.

| Class | Tool (scope) | Cold: latency / allocations | Warm p50: latency / allocations |
|---|---|---|---|
| point | `capabilities` | 1 ms / 0.5 MiB | 0 ms / 0.3 MiB |
| point | `staleness_report` fathom | 118 ms / 92.2 MiB | 9 ms / **11.7 MiB** |
| index | `find_symbol` fathom | 4 ms / 0.3 MiB | 4 ms / 0.2 MiB |
| index | `find_callers` fathom | 1,329 ms / **228.6 MiB** (builds fathom's graph) | 12 ms / 1.8 MiB |
| index | `outline` fathom | 2 ms / 4.7 MiB | 2 ms / 4.7 MiB |
| tree | `symbol_context` fathom (graph already built) | 7 ms / 7.3 MiB | 7 ms / 7.3 MiB |
| tree | `symbol_context` app-kit | 708 ms / 228.2 MiB (builds app-kit's graph) | 3 ms / 4.4 MiB |
| tree | `obligations` fathom | 65 ms / 30.3 MiB | 32 ms / 26.1 MiB |
| tree | `references` [fathom] | 576 ms / 66.3 MiB | 77 ms / 66.4 MiB |
| tree | `search` fathom | 223 ms / 17.4 MiB | 54 ms / 17.4 MiB |
| fleet | `list_services` | 30 ms / 2.0 MiB | 10 ms / 2.0 MiB |
| fleet | `references` ["*"] | 1,028 ms / 231.1 MiB | 165 ms / 168.2 MiB |
| fleet | `cross_repo_blast_radius` | 310 ms / 288.3 MiB | 30 ms / 29.0 MiB |

The daemon and its children peaked at 557 MiB during this probe.

Two real-fleet readings are above the class ceilings that the generated fleet passes.
Both belong to RT-M3's real-fleet row:

- **`staleness_report`, warm: 11.7 MiB against the 8 MiB point ceiling.**
- **The cold index call that builds a service's call graph: 229 MiB against the 64 MiB
  index ceiling.** The build is single-flighted and memoized, so the same call warm
  costs 1.8 MiB. The cost is paid once per service per index version, not per call.

## 4. The incident replay (INC-23 traffic, `rtA-proof/replay.py`)

- **Store:** a fresh `.backup` copy of `fleet.sqlite`.
- **Daemon:** one scratch daemon per phase, with `--lifecycle-id rtc-replay-*`,
  `FATHOM_TENANCY_BUDGET=4GiB` (the managed auto budget on this machine, as in the
  wave-B default-budget runs) and admission `observe`.
- **Timing:** each burst was sent after the 120 s warm-up.

| Phase | Result | p50 / max | Allocations per call |
|---|---|---|---|
| `list_services` ×32 ×4 (session hook) | 128/128 | 122–152 / 161 ms | 0.4 MiB |
| `obligations` ×20 ×5 (edit hook) | 100/100 | 86–185 / 204 ms | 9.1–10.1 MiB |
| **`symbol_context` ×20, concurrent, cold** | **20/20, wall 1.71 s** | **1,409 / 1,708 ms** | **61.1 MiB** |

The daemons peaked at 0.21, 0.06 and **0.72 GiB** (`phys_footprint`). The highest reading
with children was 0.78 GiB, and there were no children. The level stayed `normal`.

The same service sequence as wave A (`MODE=symctx`), cold then a warm repeat:

| Call | Wave-A head | Waves C–F |
|---|---|---|
| `symbol_context` app-kit, cold | 22.1 s, 5,281 MiB | **0.67 s, 245 MiB** |
| `symbol_context` platform-engineering, cold | 4.5 s, 2,454 MiB | **0.82 s, 260 MiB** |
| `symbol_context` fathom, cold | 15.4 s, 9,782 MiB | **1.28 s, 331 MiB** |
| ×20 repeat burst | p50 76.7 s, 4,595 MiB per call, 5.21 GiB peak | **20/20, p50 159 ms, 22.7 MiB per call, 0.78 GiB peak** |

Against wave B, for the same ×20 burst at the same budget:

| Binary | Result | Peak footprint |
|---|---|---|
| wave-A head | 20/20 in 52.8 s | 2.96 GiB (a second run reached 5.2–5.9 GiB) |
| wave B, observe | 14 ok; 6 abandoned at the 60 s tree deadline | 1.00 GiB |
| wave B, enforce | 13 ok; 7 × 503 for full slots | 0.45 GiB |
| **waves C–F, observe** | **20/20 in 1.7 s** | **0.72 GiB** |

**Is it flat?** Yes, per call. Twenty concurrent cold calls cost 61 MiB each, because
each service's graph is built once. Twenty warm calls cost 23 MiB each. None failed or
were abandoned.

The peak is **28% under wave B's 1.00 GiB, not an order of magnitude**. What remains is
resident cache: three services' call graphs.

A separate probe built the three graphs one at a time. `fathom_cache_bytes` then read:

| Memo group | Bytes |
|---|---|
| `graphstore` | 259 MiB |
| `snapshot` | 42 MiB |
| `typed_edges` | 19 MiB |
| **Total** | **319 MiB** |

That total is 86% of the 372 MiB heap in use. The heap profile after the burst agrees:
`callgraph.Build` 155 MB and `symbolnav.buildForwardIndex` 55 MB. This memory is
accounted for, and it is trimmed at shed (I51). The wave-B enforce figure (0.45 GiB) came
from refusing work, not from answering it cheaply.

## 5. Auto-refresh storm: one ingest at a time, inside the window

**Setup:**

- **Store:** a store copy in which the `indexed_commit` of seven registered services was
  set to zeros so they read as drifted: car-assistant, geoffmiller-cloud, iam_horcrux,
  planning, pulumi_eks_component, study and agentic-kit.
- **Daemon:** `--auto-refresh=true`, `--lifecycle-id rtc-storm`, a 4 GiB budget.
- **Trigger:** after warm-up, 21 concurrent `find_symbol` reads, 3 per service, then
  continued reads every 2 s.
- **Sampling:** `ingest_state` was read from the store copy, read-only, every 0.5 s.

**Results:**

- **The requests:** all seven drifts were requested in the same second (`auto-refresh: <svc> drifted 0000000→<head>, refresh
  requested`). Each read answered at once from the index as it stood: 20 said `queued (index_stale)`
  and one said `refreshing`.
- **The queue ran them one after another:** study (about 25 s), agentic-kit (5 s),
  pulumi_eks_component (0.5 s), planning (1.5 s), iam_horcrux (21 s), geoffmiller-cloud
  (2.5 s) and car-assistant (about 19 s). Durations are to the 0.5 s sampling interval. All seven were refreshed in 77 s.
- **Serial:** 150 samples, and at most **1** ingest in progress at any time.
- **Reads during the drain:** 266 calls, p50 2 ms, p95 6 ms, max 10 ms. None waited for
  an ingest.
- **Footprint:** own at most 498 MiB, children (`go list` and the typed pass) at most
  817 MiB, both together at most **962 MiB**. The ladder stayed at `normal`. That is
  inside the 4 GiB budget but above design §15's "ingest ≤ 300 MiB" target for the
  daemon with children. Most of it is the children of the typed pass.

**The 64 MiB window.** The daemon does not publish an ingest's `PeakInflightBytes`, so
the same seven ingests were run through the same pipeline with `fathom ingest --json`
into a store copy. The most file content any of them held at once:

| Service | Files discovered | Peak content in flight | CLI peak `phys_footprint` |
|---|---|---|---|
| car-assistant | 104 | 0.42 MiB | 138 MiB |
| geoffmiller-cloud | 300 | 0.94 MiB | 161 MiB |
| iam_horcrux | 383 | 0.10 MiB | 169 MiB |
| planning | 588 | 2.11 MiB | 183 MiB |
| pulumi_eks_component | 116 | 1.34 MiB | 26 MiB |
| study | 782 | **12.9 MiB** | 211 MiB |
| agentic-kit | 1,213 | 0.22 MiB | 369 MiB |

All seven are inside the 64 MiB `inflight_bytes` window. The largest is 20% of it.

## 6. What the live runs found

| # | Finding | Evidence | Weight | State |
|---|---|---|---|---|
| F1 | **A read on a service that is being onboarded breaks the onboarding ingest.** An `ingest` with `path` for a new service fails for every file, with `stat <service>/<path>: no such file or directory`, whenever a concurrent read names the same service. The path is joined to the service name, not the root. Likely mechanism: `LocalConnector.roots` is process-wide and keyed by service, and a read's discovery of the not-yet-registered service records the service name as its root. The ingest's `Stat` then resolves against it. | Reproduced 3 ways on a 300-file repo: ingest alone ok; one read first then ingest ok; 4 concurrent `obligations` reads during the ingest → 300 of 300 files unreadable. Harness run 2: 20,101 of 20,101. Probe run: 11,728 and 12,395 of 20,101. | High: onboarding under hook traffic fails. | open, wave-D follow-up |
| F2 | **An ingest in which every file failed is recorded as complete at HEAD.** After F1, `service_meta.indexed_commit` = HEAD, `ingest_state` = `complete` and 20,101 `file_stats` rows, but **0 fingerprints**. Auto-refresh compares HEAD to the watermark, sees no drift and never retries, so the service serves an empty index as fresh until an explicit ingest. | harness run-2 store | High (I4/I38) | open |
| H1 | Harness readiness `curl -s /healthz` counted the store-wait 503 as ready, so the first 692 calls and the run's ingest were refused. | run 1 codes, `storewait.go` | High for the harness | fixed `01c7e260` (`curl -sf`) |
| H2 | Harness `resources --id` without `--url` fell back to the managed daemon's address and read the operator's daemon. The exit mark in the reading could be the managed daemon's. | reproduced by hand (`url: http://localhost:7575`) | High (safety) | fixed: the run's `--url` |
| H3 | `resources` exits 1 on a crash-looping ledger, and under `pipefail` the harness died before writing its reading. | run 1 exit 1, no reading | High for the harness | fixed: `\|\| true` |
| H4 | The ledger attribution read one start early, because a breaker row precedes the start it decides. The stop pattern matched the first breaker row (to safe): run 2 read "safe 3, stop 3". | run-2 ledger | Medium | fixed: safe 4 and stop 6 from the same ledger, with a self-check case |
| H5 | `${x:+a}${x:-b}` printed the value twice ("start 33"). | run-2 reading | Cosmetic | fixed |
| O1 | On the real fleet, `staleness_report` warm (11.7 MiB) is over the point ceiling (8 MiB), and a cold graph-building index call (229 MiB) is over the index ceiling (64 MiB). The generated fleet passes both. | §3.2 | Medium | RT-M3 real-fleet row |
| O2 | During the auto-refresh storm, the typed pass's children reached 817 MiB. Together with the daemon that is 962 MiB, against design §15's "ingest ≤ 300 MiB". | §5 | Medium | RT-M10 per-RAM-class row |

## 7. Safety and cleanup

- One scratch daemon ran at a time, on ports 18523 and 18531. The live daemon on 7575
  was never started, stopped or signalled.
- `~/.fathom`, `~/Library/LaunchAgents` and the operator's call log were not touched.
- No `fathom ingest` was run against the shared store. The CLI ingests in §5 wrote only
  to a store copy.
- Before H2 was understood, one hand-run `fathom resources` with a malformed `--id` made a
  read-only `/statusz` request to the live daemon. Nothing was changed there.
- The highest reading in any run was 0.96 GiB with children. The 6 GiB watchdog never
  fired.
- The store copies, scratch HOMEs, fixtures, run directories and binary were deleted
  after the reading.
