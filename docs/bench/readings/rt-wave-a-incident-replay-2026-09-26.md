# Responsible tenant, wave A: incident replay (2026-09-26)

Binaries: the wave-A integrated head `87cf1f6b` (rtA-integ, all six A/A2 lanes merged)
against the pre-hotfix baseline `3d58a0db`. Incident: INC-23. Design:
`docs/design/responsible-tenant.md` §4.3, §4.4, §16 (wave A row). Plan:
`docs/plans/responsible-tenant.md` §A.

## 1. Verdict first

**Wave A removes both causes of the incident.** The replay runs the incident's traffic: 32
concurrent `list_services` calls from a cold start, then 20 concurrent edit-hook
`obligations` calls on the three largest services.

- **Baseline:** it hit the 6 GiB kill line in both phases. The `list_services` phase took
  20 s to get there, with 0 of 32 calls answered. The `obligations` phase took 5 s, with
  13 of 20 answered.
- **Head:** it ran the same traffic and stayed under 110 MiB footprint throughout.
  `list_services` uses 1.2 MiB per call from cold and 0.26 MiB warm, down from several
  hundred MiB of file contents per call. `obligations` uses about 9 MiB per call, down from
  hundreds of MiB.
- **Heap profiles:** the baseline's heap at its peak is 95–99% `os.ReadFile` under
  `(*Client).Sources`, the whole-tree read. It is reached from
  `UnsupportedLanguagesFor` (`list_services`) and from `RawFallbackObligations`
  (`obligations`). The head's allocation profiles for the same phases contain no
  `(*Client).Sources` and no `ListAllSymbols`. The largest remaining source reads are
  `SourcePaths` (names only), `ServiceSymbolAggregates` (SQL), and 0.9 MiB of `os.ReadFile`
  across 128 `list_services` calls.

**Wave A does not make the daemon safe on its own, as the design said.**
`symbol_context` is still a whole-service reader. Its cost is the same in both binaries:
2.4–9.8 GB allocated per call, because each call runs `callgraph.Build` over
`ListAllSymbols` with no memo.

- 20 concurrent calls ran the head to 6.06 GiB RSS (5.88 GiB footprint) in 31 s, and I
  killed it.
- A repeat peaked at 5.78 GiB RSS (5.21 GiB footprint).
- The baseline peaked at 5.66 and 5.58 GiB RSS in two runs.
- The 20 calls take about 2 minutes (p50 75 s).

The design leaves this to wave B, where the `tree` class gets 2 slots and a ladder, and to
wave C (RT-84, RT-75, the `memo` graph store). This reading measures what that means: until
wave B ships, a burst of agent `symbol_context` calls can still put about 6 GiB on this
laptop. Hooks never call it, so the edit-hook storm that caused INC-23 cannot.

**Recommendation:** deploy wave A, because the known causes are gone. Treat wave B's
admission (the `tree` class for `symbol_context`) as the next deploy, not a later one.
Single-flighting `callgraph.Build` per (service, watermark) would be the cheapest
stop-gap if wave B slips.

## 2. Against the design's wave-A expectations

| Expectation (design §16 wave A, §4.3, §4.4) | Measured | Holds? |
|---|---|---|
| The edit path reads no whole tree's contents | head `obligations`: no `Sources` and no `ReadFile` of the tree in the allocation profile; baseline: 3.30 GB in use under `RawFallbackObligations` → `Sources` → `os.ReadFile` | **yes** |
| `list_services` loads no symbol row | head: `ServiceSymbolAggregates` (SQL) and `SourcePaths`, with no `ListAllSymbols`; baseline: 4.01 GB in use under `UnsupportedLanguagesFor` → `Sources` → `os.ReadFile` | **yes** |
| `obligations` allocates ≤ 2 MiB per call (§4.3) | sequential and warm: app-kit 1.7–2.5 MiB, platform-engineering 6.9–7.5 MiB, fathom 21.3–21.6 MiB; under 20-way concurrency 9.0–10.1 MiB | **no** for two of the three services (see §4) |
| `obligations` p50 ≤ 60 ms (§4.3) | sequential and warm: p50 12 ms, max 41 ms; 20-way concurrent: p50 80–113 ms, max 217 ms | **yes** sequentially; the target does not cover a 20-way burst |
| Inline ingests run one at a time (RT-149) | not exercised: `--auto-refresh=false`, as the replay rules require | not measured |

## 3. Numbers

Footprint is phys_footprint from `footprint -p`. RSS is from `ps`. Both were sampled every
second. Allocation is the change in `TotalAlloc` from `/debug/pprof/allocs?debug=1` across
the burst, divided by the number of calls. Each call class started on a fresh daemon and a
fresh copy of the store.

### Head `87cf1f6b`

| Phase | OK | Wall | p50 / p95 / max | Alloc/call | Peak RSS / footprint |
|---|---|---|---|---|---|
| list_services ×32, round 1 (cold) | 32/32 | 0.96 s | 950 / 958 / 960 ms | 1.25 MiB | 0.06 / 0.03 GiB |
| list_services ×32, rounds 2–4 | 96/96 | 0.17–0.19 s | 147–170 / ≤184 ms | 0.26 MiB | 0.06 / 0.03 GiB |
| obligations ×20, round 1 | 20/20 | 0.23 s | 113 / 217 / 217 ms | 10.1 MiB | 0.10 / 0.06 GiB |
| obligations ×20, rounds 2–5 | 80/80 | 0.15–0.17 s | 80–99 / ≤169 ms | 9.0–9.3 MiB | 0.11 / 0.09 GiB |
| obligations, sequential ×20 (warm) | 20/20 | — | 12 / 41 / 41 ms | median 7.3 MiB | — |
| symbol_context ×20, run 1 | 1/20 | killed at +30.7 s | — | — | **6.06 / 5.88 GiB (killed)** |
| symbol_context ×20, run 2 | 20/20 | 124 s | 76.7 s / 124 s | 4.59 GB | 5.78 / 5.21 GiB |
| symbol_context, one call each: app-kit / platform-engineering / fathom | 3/3 | 22.1 / 4.5 / 15.4 s | — | 5.28 / 2.45 / 9.78 GB | 1.16 / 1.11 GiB |

After the `list_services` and `obligations` phases, the steady state was 0.06 GiB RSS
(0.03 GiB footprint) and 0.10 GiB RSS (0.08 GiB footprint).

### Baseline `3d58a0db`

| Phase | OK | Time to the 6 GiB kill | Peak RSS / footprint | Heap at peak |
|---|---|---|---|---|
| list_services ×32, round 1 (cold) | 0/32 | **20.1 s** | 4.68 / **6.58 GiB** | 4.21 GB in use: 95% `os.readFileContents` under `UnsupportedLanguagesFor` → `Sources` |
| obligations ×20, round 1 | 13/20 (p50 1.46 s) | **5.0 s** | 4.12 / **7.27 GiB** | 3.36 GB in use: 98.5% `os.readFileContents` under `RawFallbackObligations` → `Sources` |
| symbol_context ×20, run 1 | 20/20 (p50 75.4 s) | not reached | 5.66 / 5.37 GiB | `callgraph.Build` (same as head) |
| symbol_context ×20, run 2 | 20/20 (p50 75.4 s) | not reached | 5.58 / 5.38 GiB | — |
| symbol_context, one call each | 3/3 | — | 1.08 / 0.87 GiB | 5.28 / 2.45 / 9.78 GB allocated |

The baseline never reached rounds 2–4 of `list_services` or rounds 2–5 of `obligations`.
Each phase ended in a kill during round 1. The footprint overshoots 6 GiB because the
runaway rate is above 1 GB per second: one-second samples cannot stop it at the line, and
`footprint` lags `ps`. This agrees with the design's measured ~7.5 GB/min, and the burst is
faster than that.

## 4. What remains, and where it goes

- **`symbol_context` (and anything else on `callgraph.Build`).** At the head's peak
  (4.08 GB in use), memory is split as follows. The shares are cumulative and overlap:
  - 78% `callgraph.Build`, including 42% in `addSuffixCandidate` (`buildSuffixIndex`) and
    35% in `buildIndexes`;
  - 19% `ListAllSymbols`;
  - 13% `QueryObservations`.

  Each concurrent call builds its own graph for the whole service. That work belongs to
  wave B (the `tree` class, 2 slots) and wave C (RT-84, RT-75, the `memo` graph store).
  The Go soft limit (3 GiB) holds the burst at about 5.2–5.9 GiB footprint by thrashing
  the GC: 20 calls take 2 minutes. It did not stop the head's first run from crossing
  6 GiB RSS. Both binaries behave the same here, so wave A neither caused this nor fixed
  it.
- **`obligations` allocation.** The profile (rounds 2–5, head) splits as follows. The
  shares are cumulative and overlap:
  - 49% `scanClaims`, including 29% through `ClaimsForService`, which loads the service's
    whole claims set per call;
  - 29% `LiveLayerCoverage`;
  - 31% `RawFallbackObligations`.

  All of it is bounded, and none of it reads a tree's contents. It misses the ≤ 2 MiB
  target on services with many claims: fathom allocates 21 MiB per call. RT-82's
  `file_stats` forms (wave C) address coverage, and a per-(service, watermark) claims memo
  would address the rest. That is a candidate for RT-174's lane, or a row of its own.
- **RSS is not footprint.** After the baseline's `symbol_context` burst, RSS stayed at
  5.62 GiB while the footprint fell to 0.16 GiB (freed pages not yet reclaimed). The
  design's choice of phys_footprint for the ladder (§2) is right. A monitor that reads
  RSS would report a daemon at rest as still overloaded.

## 5. Method

- **Binaries:** `go build ./cmd/fathom` at `87cf1f6b` (head) and at `3d58a0db` (baseline,
  a worktree).
- **Daemons:** each ran `serve-http --addr 127.0.0.1:7890|7891 --auto-refresh=false
  --store <copy> --call-log <scratch>`. Each had a scratch HOME and scratch
  `FATHOM_CACHE_ROOT`, and ran with no `FATHOM_*` or `XPC_*` variables, so each started
  unmanaged. Only one daemon ran at a time. The managed daemon on 7575 was never touched.
- **Store:** a `sqlite3 .backup` of the live fleet store: 18 indexed services and 322k
  observations. It was copied fresh for each daemon and deleted afterwards. The registered
  roots are the real repositories, read-only.
- **Requests:** the exact hook requests, sent to `POST /rpc`.
  - `list_services {}` carries `X-Fathom-Origin: session-hook`, as in
    `fetchServicesOnce`.
  - `obligations {service, file, include_snippets: false, layer: "all"}` carries
    `X-Fathom-Origin: edit-hook`, as in `fetchObligations`.
  - `symbol_context {service, symbol}` is sent with no origin, as an agent's call.
  - Every request carries an `X-Fathom-Caller`.
  - The client timeout was 300 s, so each call's full cost was measured, not a hook's
    abandonment.
- **Workload:** chosen by a fixed seed.
  - `obligations`: 100 distinct source files present on disk, each round split 7
    app-kit / 7 platform-engineering / 6 fathom. Vendored `.tools/`, `node_modules`,
    `target/`, `dist/` and `vendor/` paths were excluded.
  - `symbol_context`: 20 function or type symbols, split the same way.
- **Safety:** a sampler thread sent SIGKILL the moment RSS or phys_footprint exceeded
  6 GiB. It also took heap profiles (`/debug/pprof/heap`) at every 1 GiB crossing and after
  every burst.
- **Raw data:** outside the repository, in the session scratchpad (`rtA-proof/out/`: events,
  per-second samples, results JSON, heap profiles).
