# The responsible tenant — engineering design

Status: accepted 2026-09-26 (ADR-047). Vision: `docs/vision/36-responsible-tenant.md`.
Plan: `docs/plans/responsible-tenant.md` (RT-1..). Incident: INC-23. Invariants I47–I53.
Grounded in the read-only audit of `4200482a` (82 hazards; the hazard ids below are that
audit's) and the heap profiles `heap-0518`, `heap-2118`, `heap-2315`.

The design has five parts that must all exist; each is useless alone:

| Part | Owns | Package(s) |
|---|---|---|
| A. Budget, ladder, breaker (the seatbelt) | resident memory, relaunches, host pressure, exit | `internal/tenancy` |
| B. Admission and cancellation | concurrency, abandoned work, overload signal | `internal/tenancy` (gate), `internal/mcp` (dispatch) |
| C. Request cost ∝ answer (the brakes) | whole-tree and whole-table reads, caches | `internal/corpus`, `internal/memo`, `internal/store`, `internal/lint/tenancy` |
| D. Ingest off the request path | refresh, streaming, typed pass, extractor leak | `internal/refresh`, `internal/fusion` |
| E. Hygiene and visibility | launchd, kill switch, scratch daemons, tests, hooks, disk, surfaces | `cmd/fathom`, `internal/hookguard`, `internal/testisolate` |

Contents: §1 config, §2 tenancy, §3 admission, §4 the request-path rule, §5 memo, §6 derived
facts, §7 refresh and streaming ingest, §8 launchd and lifecycle, §9 kill switch, §10
scratch daemons and tests, §11 hooks, §12 disk, §13 visibility, §14 console, §15 proof and
the stress harness, §16 rollout, §17 invariants and records.

---

## 1. Configuration: `[tenancy]` (user scope only)

Budgets are machine contracts (P13, law 13: user scope sets defaults, a cloned repository
must not raise a laptop's ceiling). They are read from `~/.fathom/config.toml` and env
only; a `[tenancy]` table in a repository's `.fathom/config.toml` is ignored and doctor
names it (`tenancy_scope`). CUE: `#Tenancy` in `schema/config/schema.cue` with valid and
invalid testdata pairs (`schema/CLAUDE.md`).

```toml
# ~/.fathom/config.toml
[tenancy]
budget          = "auto"      # phys_footprint; auto = clamp(RAM/8, 1GiB, 4GiB); Linux: min(auto, 0.5 × cgroup memory.max)
shed_at         = 0.75        # × budget
refuse_at       = 1.00        # × budget
exit_at         = 2.00        # × budget, capped at RAM/4; exit needs 2 consecutive samples
exit_enabled    = true        # managed daemons; false keeps shed/refuse only (disclosed)
gc_hint         = "budget"    # SetMemoryLimit target: "budget" | "exit" | "off" | an IEC size
admission       = "enforce"   # "observe" (decide, log, meter, never refuse) | "enforce" | "off"
gomaxprocs      = "auto"      # max(2, NumCPU/2)
child_parallelism = 2         # GOFLAGS=-p for go list / typed pass
notify          = true        # one local notification on a breaker stop or safe-mode entry, ≤ 1/hour

[tenancy.host]
shed_on_pressure   = "warn"      # macOS kern.memorystatus_vm_pressure_level 2; Linux PSI some avg10 ≥ 10
refuse_on_pressure = "critical"  # level 4; PSI ≥ 40
shed_swap_fraction   = 0.80
refuse_swap_fraction = 0.90
min_swap_volume_free = "5GiB"    # /System/Volumes/VM on macOS
min_store_volume_free = "2GiB"   # below it: store_distress

[tenancy.breaker]
safe_after = 3      # counted exits in safe_window → safe mode
safe_window = "30m"
stop_after = 5      # counted exits in stop_window → disabled.json + exit 0
stop_window = "60m"
warmup = "120s"

[tenancy.unmanaged]
idle_exit = "20m"
ttl = "8h"
budget = "512MiB"

[tenancy.caches]
fraction = 0.25     # of budget; one global byte budget across all memo groups
idle_drop = "30m"
```

Precedence: flags > env > user config > defaults (the `memoryLimitFromEnvOrConfig` order).
Env: `FATHOM_TENANCY_BUDGET`, `FATHOM_TENANCY_ADMISSION`. `FATHOM_TENANCY_ADMISSION=off`
is the escape hatch; it prints a loud startup line and doctor WARNs on every run.

**Migration from `[daemon].memory_limit` / `FATHOM_MEMORY_LIMIT`.** If only the old key is
set, it keeps its meaning (the GC hint) and doctor notes it is superseded by
`[tenancy].gc_hint`; the budget stays `auto`. If both are set, `[tenancy]` wins and doctor
names the conflict. The `memorylimit.go` comment ("catches a runaway before it fills
swap") and `docs/configuration.md`'s paragraph are corrected: the hint is not protection.

Other additive keys: `[connector] respect_gitignore = true`, `extra_ignore = []`,
`include = []` (§4.2); `[references] scan_bytes = "64MiB"`, `scan_wall = "5s"` (§4.5);
`[ingest] inflight_bytes = "64MiB"` (§7.2); `[console] conserve_max_files = 500` (§14).

---

## 2. `internal/tenancy`: budget, sampler, ladder, breaker

Pure Go, no store import, deterministic (injected clock and sampler; sorted outputs;
`internal/CLAUDE.md`). It holds the policy; `cmd/fathom` wires the OS readers in.

```go
type Budget struct {
    Budget, Shed, Refuse, Exit, GCHint, Caches int64 // bytes
    Slots  map[Class]int
    PerCall map[Class]int64 // byte ceiling per ticket
    Deadline map[Class]time.Duration
}
func ResolveBudget(cfg Config, host HostFacts) (Budget, []Note) // notes name every derivation

type Sample struct {
    At time.Time
    Footprint int64          // phys_footprint (darwin), RSS+swap (linux)
    GoTotalMinusReleased int64 // /memory/classes/total − /memory/classes/heap/released
    HeapLive int64           // /gc/heap/live:bytes
    GCCPUFraction float64    // Δ/cpu/classes/gc/total ÷ Δ/cpu/classes/total
    Pressure PressureLevel   // Normal | Warn | Critical | Unknown
    SwapUsed, SwapTotal int64
    VolumeFree map[string]int64 // store dir, TMPDIR, swap volume
    InFlight map[Class]int; Oldest InFlightEntry
}
type Sampler interface { Sample(ctx context.Context) (Sample, error) }

type Level uint8 // Normal, Shed, Refuse, Exit
type Ladder struct{ /* hysteresis state */ }
func (l *Ladder) Step(s Sample, b Budget) (Level, Transition) // pure
```

Host readers (`hostdarwin.go`, `hostlinux.go`, `hostother.go`): macOS
`unix.SysctlUint32("kern.memorystatus_vm_pressure_level")` (1/2/4), `vm.swapusage`
(`xsw_usage`), `hw.memsize`, `unix.Statfs`; the existing cgo `processPhysFootprintBytes`
(`rss_darwin.go`) for the footprint. Linux: `/proc/pressure/memory`, `/proc/self/status`
(`VmRSS` + `VmSwap`), `/proc/meminfo`, cgroup `memory.max`. Elsewhere: `Unknown`,
disclosed, never treated as healthy (I38). Sampling costs one syscall set and one
`metrics.Read` — no stop-the-world `ReadMemStats` anywhere in the sampler.

Cadence: every 2 s (the measured runaway was ~7.5 GB/min, so ~250 MB resolution).

### 2.1 The ladder

| Level | Enter when (any) | Actions (each logged, counted, shown in `/statusz`) |
|---|---|---|
| `normal` | below all marks | admission per slots |
| `shed` | footprint ≥ `shed_at`; pressure ≥ warn; swap ≥ 80% | `memo.Drop(Shed)` (every registered cache); pause the refresh queue, console jobs and feeders; auto-refresh answers stale with `refresh: deferred (shed)`; start the flight recorder; `debug.FreeOSMemory()` once on entry (then at most 1 per 30 s, after each heavy ticket release) |
| `refuse` | footprint ≥ `refuse_at`; pressure critical; swap ≥ 90%; swap-volume free < 5 GiB | also: `tree`, `fleet`, `ingest` return 503 for everyone; `index` wait drops to 0 for hooks and 1 s for agents; in-flight `tree`/`fleet` tickets past 50% of their deadline are cancelled (`outcome: abandoned-by-governor`); adoption suspended; decision port closed; one diag capture (§13.4) |
| `exit` | footprint ≥ exit mark for 2 consecutive samples; or pressure critical for 60 s with footprint > budget | diag capture; `exit` lifecycle row `{reason: over_budget, footprint, top in-flight, diag}`; drain 5 s; `os.Exit(70)` |

Step-down: 60 s below 0.6 × the current rung's threshold. Entry needs 2 consecutive
samples except `exit`'s own rule.

**Store distress** is a separate flag, not a rung: store-volume free < 2 GiB, or any
`SQLITE_FULL`/`SQLITE_IOERR` from the store. Ingest and writes are refused with the named
reason; a read that fails returns a named error, never zero. This fixes
`fleetAdapter.SymbolsFor` returning nil on any error and `list_services` then reporting
`symbol_count 0` (I38; hazard disk-writes-and-disk-full-invisible-rendered-as-zero).

### 2.2 GC hint and runtime

`applyMemoryLimit` takes `Budget.GCHint` (default the budget) instead of a fixed 3 GiB.
`GOMAXPROCS = max(2, NumCPU/2)` for managed daemons unless `GOMAXPROCS` is in the env.
The typed-pass sampler (`samplePeakHeapInuse`) moves to `runtime/metrics` and its log line
is relabelled `process heap during pass` with the concurrent in-flight tickets beside it;
the pass's own cost is attributed with pprof labels set by `refresh` (hazard
typed-pass-peak-misattributes-process-heap).

### 2.3 Lifecycle ledger and breaker

`~/.fathom/daemon/<id>/lifecycle.jsonl` (`<id>` = the launchd label when managed, else
`addr-<port>-<pid>`), append-only, rotated at 1 MiB keeping one generation, rows carry
`schema_version` and `fathom_version` (I32's pattern):

| Row | Fields |
|---|---|
| `start` | pid, binary_commit, managed, plist_schema, mode |
| `exit` | pid, code, reason (`over_budget`, `drain`, `adopt`, `stop`, `unclean`, `parent_exit`, `idle`, `ttl`, `unlinked`, `port_held`), uptime, last sample, top in-flight, diag path |
| `adopt` | old and new binary commit |
| `mode` | from, to, reason |
| `breaker` | trip reason |
| `stop` | by, reason, until |
| `store_wait` | errno, label (`tcc-wedge` / `volume-not-mounted`) |

At start, a `start` without a following `exit` means the predecessor died unclean
(SIGKILL, jetsam, force-quit); the new process writes a synthesized `exit{reason:
unclean, last_sample}` from the ring. Best effort, bounded to 20 files: it scans
`/Library/Logs/DiagnosticReports/JetsamEvent-*` and `fathom_*.diag` since that start for
its pid and adds `jetsam: largestProcess` or `resource: disk writes`.

`func EvaluateBreaker(rows []LifecycleRow, now time.Time, cfg BreakerConfig) BreakerVerdict`
— pure, evaluated at startup **after the kill-switch check and before config, store or
bind**. Counted exits: `over_budget`, `unclean`, `port_held` (foreign holder), panics.
Not counted: `adopt`, `stop`, `store_wait`, `drain`.

| Condition | Result |
|---|---|
| ≥ 3 counted exits in 30 min | start in **safe mode** |
| ≥ 5 counted exits in 60 min, or a safe-mode process exited over budget | write `disabled.json{reason: breaker, detail, since}`, notify once, exit 0 — launchd stays down |

Safe mode: auto-refresh off (stale served with disclosure); console feeders and jobs off;
`tree`/`fleet` refused with `fathom safe mode since 21:07: 3 over-budget exits in 30 min
— run fathom resources`; `point`/`index` served; decision port closed. After 30 min at
`normal` it steps to normal mode under the warm-up rules.

**Warm-up** for `warmup` (120 s) after any start: heavy classes get 1 slot each; the refresh
queue drains one service per 30 s in demand order. This absorbs the cold-cache stampede the
empty `seen` map causes today.

---

## 3. Admission, deadlines, cancellation

### 3.1 Cost classes

`toolDescriptor` (`internal/mcp/dispatch.go`) gains `cost CostClass` (and optionally
`costFor(params) CostClass` for argument-dependent tools). `mcp.Register`-style
registration panics at init without one; `TestEveryToolDeclaresCostClass` enumerates the
registry, and `docs/mcp-tools.md` gains a cost column held by `TestMcpToolsDocMatchesRegistry`
(I11).

| Class | Slots (4 GiB budget) | Per-call byte ceiling | Server deadline | Queue wait hook / console / agent / cli | Examples (initial; reassigned as §4 shrinks costs) |
|---|---|---|---|---|---|
| `point` | 16 × budget/GiB = 64 | 8 MiB | 10 s | 100 ms / 2 s / 5 s / 30 s | `capabilities`, `server_stats`, `list_services` (after §6), `obligations` (after §4.3), `find_symbol` (SQL point lookup), `staleness_report`, `read_source`, `memory_search`, `history` |
| `index` | `max(2, min(NumCPU−2, budget/512 MiB))` = 8 | 64 MiB | 30 s | 100 ms / 2 s / 5 s / 30 s | `symbol_context`, `find_callers`, `find_callees`, `query_symbols`, `pack_context`, `repo_map`, `verify_plan`, `test_impact`, `rename_impact`, `reachable`, `dead_code`, `community`, `localize`, `blast_radius` (after §6) |
| `tree` | `max(1, budget/2 GiB)` = 2 | budget/16 = 256 MiB | 60 s | refuse / 2 s / 5 s / 30 s | `search`, `secret_scan`, `cloc`, `spec_coverage`, `traceability`, `safe_delete_check`, `review_change`, `list_sources`, `discoveries`, `references` (one service); `obligations` **until §4.3 lands** |
| `fleet` | 1 | budget/8 = 512 MiB | 120 s | refuse / refuse / 20 s / 30 s | `references` with `services: ["*"]`, `fleet_drift`, `cross_repo_blast_radius`, `review_diff`, `dep_drift` |
| `ingest` | 1 machine-wide | streamed (§7) | detached | never inline: enqueue | `ingest`, `onboard_service`, auto-refresh, post-commit, typed and CUE passes, `compile_layer` |
| `egress` | 2 | 8 MiB | port timeout | refuse at refuse/safe | decision-port calls (`/route`, obligations re-rank when enabled) |

Envelope at 4 GiB: 64×8 + 8×64 + 2×256 + 1×512 MiB = 2 GiB = 0.5 × budget. On a 1 GiB
budget: 16×8 + 2×64 + 1×64 + 1×128 = 448 MiB. Slot counts and ceilings are derived by
`ResolveBudget`; they are not separately configurable (fewer knobs; RT-M2 retunes the
formula, not a table).

Argument-dependent classes: `references` without `services` is `tree` on the caller's own
service; with `["*"]` it is `fleet`. `obligations` is `tree` until its single-file path
lands — the near-term protection against the next incident: at most two whole-tree
obligations reads at once, and edit hooks get a fast refusal they render as silence.

### 3.2 The gate

```go
type Origin uint8 // Hook, Console, ConsoleInProcess, Agent, CLI, Test
type Request struct {
    Tool, Service string; Class CostClass; Origin Origin
    Deadline time.Time // min(X-Fathom-Deadline, class max)
    Caller CallerID     // pid, ppid, session hash, test flag
}
type Ticket interface {
    Context() context.Context // cancelled on deadline, client gone, or refuse-rung cancellation
    Charge(n int64) error     // ErrOverBudget at the per-call ceiling
    Release(Outcome)          // ok | partial | refused | abandoned | error
}
func (g *Gate) Admit(ctx context.Context, r Request) (Ticket, *Refusal)
type Refusal struct{ Reason, Class, Level string; RetryAfter time.Duration; InFlight []string }
```

Slots are per-class channels with per-origin wait queues; waiters `select` on the slot and
`ctx.Done()`. A request whose remaining client deadline is below the class's observed p50
is refused at once (`reason: cannot finish before your deadline`) — this alone would have
prevented the 19-minute `list_services` runs behind 2-second hooks. `mode=observe` runs
the whole decision, logs and counts every would-refuse, and admits.

`Charge` is called by the byte producers: `corpus` reads and scans, store row decoding
(column bytes), ripgrep output, log readers. On `ErrOverBudget` the producer stops and
returns what it has with `budget: {partial: true, elided: N, bytes_read, reason}`, rendered
with `readcoverage.RenderCount` as a lower bound.

### 3.3 Where admission happens

- `handleToolsCallWithCtx` admits before invoking the handler (a `tree`/`fleet` refusal
  costs nothing).
- `batch`: capped at 16 calls; the batch holds no ticket; each sub-call is admitted in its
  own class with the parent's deadline; the batch stops at `ctx.Done()`.
- In-process console calls (`inProcessServicesFn`, in-process `fetchServices`, the plans
  worker, conserve, the project index, the model rebuild) go through the same wrapper with
  `origin=console-inprocess`: excluded from agent-traffic counts (as `IsConsoleRead` does),
  included in every resource instrument (hazard in-process-registry-bypasses-instruments).
- Console-origin requests never trigger auto-refresh.

### 3.4 Refusal wire format

Before dispatch: HTTP 503, `Retry-After: <s>`, JSON-RPC error `{code: -32050, message:
"fathom busy", data: {reason, class, level, retry_after_ms, in_flight}}`. The reason is a
sentence: `tree-class slots full (2/2): search, obligations`. Store unavailable (§8.4):
503 `{reason: "store unavailable: operation not permitted", retry_after_ms: 30000}`. Test
caller at the managed daemon: 409 `{reason: "test binary reached the operator daemon"}`.
`-32050`, the 503 envelope and the 409 join the compatibility surface.

Consumers: hooks → silence + breaker (§11, I9); session and subagent primers → the existing
daemon-down line with the reason; agent MCP calls → the named error or a `budget.partial`
result, never an empty result that looks complete; `fathom verify` via the daemon → exit 2
(operational, fail-open per I8); console → the last good projection with truth state
`stale: fathom shedding`; explicit `ingest` → queued/refused with the level, never a second
parallel ingest.

### 3.5 Context and the HTTP server

- `/rpc` ctx derives from `r.Context()` with the class deadline (`servehttp.go`'s
  `context.Background()` for every RPC is deleted). Only `refresh` jobs and `memo` flights
  marked `Keep` run detached.
- `http.Server{ReadHeaderTimeout: 5s, IdleTimeout: 120s, MaxHeaderBytes: 64 KiB}` replaces
  bare `http.Serve`; per-route write deadlines via `http.ResponseController`: 150 s for
  `/rpc` (the fleet ceiling + margin), none for `/events` SSE.
- Every TCP route requires a loopback `Host` (`127.0.0.1:port`, `localhost:port`,
  `[::1]:port`) and a same-origin `Origin` when present — `requireLocalMutation`'s logic
  generalized — closing DNS rebinding to `/rpc` and the console.
- Every `<-done` wait (the console memos, the hotfix's `unsupportedCall`) honours ctx.
- Store queries use `QueryContext`; a cancelled request stops iterating rows.
- `exec.Command` in request and hook paths becomes `exec.CommandContext` with a timeout.

---

## 4. The request-path rule: cost follows the answer

**Rule (I48).** A request path reads only the files it was asked about, never loads a whole
table to answer a point question, and its cost at 10× fleet scale is at most 1.5× its cost
at 1×. Whole-repository facts come from ingest (§6); whole-tree scans exist only as
budgeted, early-stopping, lower-bound-reporting `corpus.Scan` calls in `tree`/`fleet`
tools.

### 4.1 `internal/corpus`

Sits beside `internal/connector` (I20 unchanged: it imports neither `query` nor `mcp`).

```go
type FileStat struct{ Path string; Size, MtimeNS int64; Ino uint64; Mode fs.FileMode }
type Excluded struct{ Path, Reason string; Size int64 } // oversize | binary | ignored | unreadable

type Reader interface {
    Paths(ctx context.Context) ([]FileStat, Coverage, error)       // from file_stats (§6), not a walk
    ReadFileCapped(ctx context.Context, path string, cap int64) ([]byte, *Excluded, error)
    Scan(ctx context.Context, paths []string, b Budget, fn func(path string, content []byte) (stop bool)) (Coverage, error)
}
type Budget struct{ Bytes int64; Files int; Wall time.Duration; Hits int }
type Coverage struct{ Scanned, Total int; Excluded []Excluded; Stopped string } // "", bytes, time, hits, ctx
func IngestScan(ctx context.Context, root string, fn func(FileStat, []byte) error) error // only internal/fusion, internal/refresh
```

- `ReadFileCapped` stats first; above the cap (default `fusion.MaxExtractSize` = 2 MiB) it
  returns `Excluded{oversize}` without reading. A binary sniff reads the first 8 KiB.
- `Scan` streams one file at a time, releases each buffer, charges the ticket, and stops
  early; callers render `Coverage` as a lower bound.
- Every byte read is charged to the request's meter → `fathom_bytes_read_total{tool}`.

### 4.2 Discovery

- **Git worktree:** `git ls-files -co --exclude-standard -z` through `internal/gitro`
  (read-only git), with `.fathomignore` and `~/.fathom/ignore` layered on top. App-kit's
  `.cache/` (19,659 fingerprinted files, ~2.3 GiB) and `compute/.tools`/`target/` drop out.
- **Non-git corpora (ADR-017):** the existing walk with `skipDir` plus a default list
  (`.cache`, `target`, `dist`, `build`, `.tools`, `__pycache__`, `.venv`, `.gradle`, `.next`,
  `.turbo`, `coverage`).
- `[connector] include` re-includes a gitignored directory an adopter genuinely indexes;
  `respect_gitignore = false` per service is the kill-criterion fallback.
- Discovery honours ctx every 256 entries; `LocalConnector.Fetch` stats before reading and
  honours ctx.
- Every exclusion is counted and named (`excluded: N gitignored, M oversize, K binary`),
  I25/I38. `fathom ingest --dry-run --stats` (stat only) measures the before/after delta.

### 4.3 Obligations (72% of request allocation)

- Layer coverage needs corpus **paths** plus globs: from `file_stats` via SQL, memoized per
  `(service, watermark, layerDigest)` in a `memo` group.
- `semantic.LoadLayerIn`'s `loadUnion` is memoized per `(root, digest of .fathom/layers +
  embedded pack version)` — no new CUE context twice per call.
- `RawFallbackObligations` reads only the edited file (`ReadFileCapped`, 2 MiB); an oversize
  file gets rules-only output and a disclosure; ranking features come from `file_stats` and
  `observations WHERE source_ref = ?` (index `idx_observations_service_source` exists).
- `ObligationsIndexStale` compares the stored stat with a live `os.Stat` and hashes only
  when they differ, under the cap.
- Class becomes `point`. Targets: ≤ 2 MiB allocation per call; p50 ≤ 60 ms (49 ms before
  2026-09-16, 577 ms after). I35's payload ceiling unchanged; an unreadable or oversize
  corpus file makes coverage a named lower bound (I38).

### 4.4 `list_services`, identity and "is X indexed"

- Served from `service_summary` + `service_meta` (§6): one select of ~21 rows.
- `IsIndexed(service)` (a `service_meta` row with `indexed_at > 0`) replaces
  `daemonServiceIndexed` (`verify.go`), the stop-hook and edit-hook claim checks,
  `plans.go`, and `console_plansdata.go`'s probe — yes/no questions that pay the full
  answer today.
- `GET /identity`: names, roots and the indexed flag, cached per store watermark, for
  hooks (§11.3).

### 4.5 Navigation, references, search

- Cross-service callers: one query over `observation_calls` (`WHERE callee_key IN (…)`);
  `crossServiceCallees` resolves each key once through the snapshot's name index.
- `FindSymbol`/`FindCallers` point lookups become `WHERE service_id = ? AND symbol = ?`
  (new index `observations(service_id, symbol)`); the `ORDER BY ingested_at` in
  `queryObservationsRaw` is dropped (sorted in Go on the small result) — no temp B-tree,
  no `SQLITE_FULL` on reads.
- `references`: default scope is the caller's own service (argument or identity header);
  `services: ["*"]` is `fleet` class (**Breaking**). Candidates by SQL name lookup; text
  occurrences via `corpus.Scan` with `[references] scan_bytes = 64 MiB, scan_wall = 5 s`,
  early stop at `max`, binaries skipped, `bufio.Scanner` lines (no `strings.Split` of whole
  files); typed edges per service as needed. Partial: `≥ N occurrences (scanned 812/2,236
  files: budget bytes)`.
- `search`: ripgrep via `CommandContext` with `--max-columns=512 --max-columns-preview
  --max-filesize=2M --max-count=<limit> -C min(context,5)`, stdout through a byte-capped
  reader (1 MiB) that kills rg at the cap; `root` must lie inside a registered service root;
  the Go fallback uses `corpus.Scan` (200 hits, not 20,000).
- `serviceref` (routes, HTTP client calls) is extracted at ingest (§6), never per call.
- The remaining whole-tree call sites (audit hazard other-tools-whole-tree-sources:
  `implementations.go`, `safe_delete.go`, `review_tool.go`, `extra_tools.go` spec_coverage
  / traceability / dep_drift / traverse, `discoveries_tool.go`, `source_tools.go`,
  `verify_plan_tool.go` `buildSpecMatrix`, console project index, `report_evidence.go`)
  move per site to `Paths`, `ReadFileCapped`, or `Scan`.
- Producers budget themselves; `budgetedResult` gets a size check before any copy and loses
  the marshal → unmarshal → per-key unmarshal cascade (hazard budget-after-build).

### 4.6 Type-level removal

`Sources` is deleted from `mcp.QueryInterface` and `autoRefreshBackend`;
`client.(*Client).Sources` is renamed `SourcesForIngest` (deprecated for external callers —
`pkg/client` is on the compatibility surface, CHANGELOG entry). Query code gets
`Corpus(service) corpus.Reader`. Code that tries to read a whole tree in a handler does not
compile.

### 4.7 `internal/lint/tenancy` — the rule enforced in the gate

A Go test (`TestTenancyRules`, run by `go test ./...` inside `task validate`; no vet tool,
no new gate step) that parses every non-test `.go` file under `cmd/`, `internal/`, `pkg/`
with `go/parser` and resolves imports per file. Outside an allowlist of packages
(`internal/fusion`, `internal/refresh`, `internal/corpus`, the `cmd/fathom` ingest verbs),
it forbids:

| Rule id | Pattern |
|---|---|
| `whole-read` | `os.ReadFile`, `io.ReadAll` on an `*os.File`, `filepath.WalkDir`/`os.ReadDir` on a service root |
| `sources` | `SourcesForIngest`, `ListAllSymbols`, unfiltered `QueryObservations(service)` in handler packages |
| `graph-build` | `callgraph.Build` outside the `graphstore` memo group |
| `exec-noctx` | `exec.Command` (require `CommandContext`) |
| `bg-ctx` | `context.Background()`/`TODO()` in `internal/mcp`, `cmd/fathom/console_*.go`, `servehttp.go` |
| `bare-go` | a `go` statement outside `console.Jobs`, `refresh`, `memo`, `tenancy` |
| `cache-map` | a package-level mutex-guarded `map` written from a handler (a cache outside `memo`) |
| `raw-serve` | `http.Serve`, `http.ListenAndServe` |
| `remarshal` | `json.Marshal` of a value followed by `json.Unmarshal` of the same bytes |
| `test-live-url` | `:7575` literal in a `_test.go` file |

Escape hatch: `//fathom:tenancy-allow <rule> <reason>` on the line. Every use is listed in
`.fathom/tenancy-allow.txt`; `TestTenancyAllowlistOnlyShrinks` fails when the count rises
above the committed number. Wave C seeds it with today's violations and later waves burn it
down; RT-49 closes it at zero or with each remaining entry justified.

### 4.8 Cost tests (`internal/mcp/costtest`)

Fixture: the `fauxtown` golden corpus generated at 1× and 10× (same shape, 10× files,
symbols and bytes), plus a 12 MiB file and a binary blob. For each registered tool, one cold
and one warm call at both scales, `GOMAXPROCS=1`, nothing else in flight. Measured:
`bytes_read` from the corpus meter; allocations as a `runtime/metrics
/gc/heap/allocs:bytes` delta.

- `point`: warm allocations ≤ 8 MiB, bytes read ≤ 2 MiB (one capped file).
- `index`: ≤ 64 MiB. `tree`: ≤ 256 MiB and a named partial beyond. `fleet`: ≤ 512 MiB.
- `point`/`index`: allocations at 10× ≤ 1.5 × allocations at 1× (I48 made executable).
- `TestEveryToolHasCostRow` fails when a tool lacks a row.

Stampede tests: 32 concurrent cold callers per memo group and per class → builds == 1,
bytes read == one walk; cancel all callers mid-build → the flight is cancelled and
`runtime.NumGoroutine` returns to baseline within 1 s. They generalize the hotfix's red
test and count bytes, not Discover calls (the DG-18 test's mistake).

---

## 5. `internal/memo`: one byte-budgeted, single-flighted registry

```go
type Group[K comparable, V any] struct{ /* … */ }
func NewGroup[K comparable, V any](name string, opts Options) *Group[K, V] // registers globally
type Options struct{ MaxEntries int; WatermarkScoped bool; Priority int }
func (g *Group[K, V]) Get(ctx context.Context, key K, build func(context.Context) (V, int64, error)) (V, error)
func (g *Group[K, V]) GetStale(ctx context.Context, key K, build …) (V, Freshness, error) // stale-while-revalidate
func Drop(reason DropReason)                   // Shed, Refuse, Idle, Watermark(service)
func Stats() []GroupStats                      // entries, bytes, hits, misses, flights, cancelled, evictions by reason
```

- Single flight; waiters `select` on `done` and `ctx.Done()`. The build runs under a flight
  context cancelled when the last waiter leaves, unless it is past half its expected cost
  and marked `Keep`.
- Size is the builder's estimate; one global LRU across groups under `caches.fraction ×
  budget` (1 GiB here). Entries for a service are evicted eagerly on
  `refresh.WatermarkBumped(service)`. Empty watermarks are never keys (I21 kept).
- Idle: after 30 min without tickets, `Drop(Idle)` then `debug.FreeOSMemory()`.
- Groups (migrations): `graphstore` keyed `(service, watermark, kind)` — replaces
  `callgraph.CachedCallGraph`, `projectIndexMemo`'s graph, the console plans graphs;
  `BuildBoth` becomes two keys — so no service's graph is held three times.
  `snapshot` — an immutable per-(service, watermark) symbol set with a name index and a
  callee index, lowercase keys stored once in the index; replaces `ListAllSymbols` loads for
  iteration, the console search's private copy, and `loadNavGraphs`' per-call reload.
  Also: `obsModel`, `operate`, `unsupported`, `layerCoverage`, `packUnion`, project
  activity / wiring / intent / explain / certify / layers.
- `internal/resultcache` (zero importers) is retired.
- If a single app-kit graph exceeds 40% of the cache budget, the graph representation is
  compacted (interned strings, `int32` node ids) before any budget raise.

---

## 6. Ingest-time derived facts

Additive tables (`CREATE … IF NOT EXISTS`), written in the ingest's final transaction:

| Table | Written by | Replaces request-time | Size today |
|---|---|---|---|
| `file_stats(service_id, path, size, mtime_ns, ino, content_hash, lang, excluded)` | pass 1 | `Sources`/`SourcePaths` for paths and languages, obligations corpus selection, the manifest watermark | ~45k rows after gitignore |
| `service_summary(service_id, watermark, symbol_count, languages_json, unsupported_langs_json, file_count, bytes, excluded_counts_json)` | end of ingest | `ServiceSummaries`' full symbol load, `unsupportedLanguagesCached` | 21 rows |
| `observation_calls(service_id, caller_rowid, callee_key)` + `INDEX(callee_key)` | ingest (normalizes `calls`) | `fleetCallers`, `crossServiceCallees` | ~669k rows, 30–40 MB |
| `service_routes`, `service_http_calls` | ingest (serviceref extractor moves here) | `serviceref.BuildFromStore` per call | small |
| `file_imports(service_id, path, content_hash, imports_json, resolved_json)` | ingest; unchanged hash reuses the row | `fileresolve` re-parsing every JS/TS/Python file twice | per JS/TS/Py file |
| `ingest_state(service_id, state, pid, started_at, target_commit)` | refresh | the watermark-before-extraction bug | 21 rows |

Indexes: `observations(service_id, symbol)`. Pragmas at open: `cache_size = -8192`,
`soft_heap_limit = 268435456`, `journal_size_limit = 67108864`, `temp_store` left at file
(sorts removed instead).

Migration: until a service's first post-upgrade ingest the new tables are empty for it;
the tool answers `summary unavailable until next index (refresh queued)` — a named absence
(I38), never a whole-tree fallback. The refresh queue backfills services one at a time in
demand order.

Watermark ordering: `ingest_state='in_progress'` is written first; `recordIngestMeta` moves
to the final commit. `ensure` treats `in_progress` with a dead or foreign pid as "re-ingest
needed" (`TestWatermarkNotAdvancedByKilledIngest`).

ADR-017 amendment: a service whose stored watermark is a git SHA never takes the content
manifest path — a git failure is `freshness: unknown`, disclosed. Genuine non-git corpora
use a stat manifest (paths, sizes, mtimes from `file_stats`), capped and cached with a TTL.

---

## 7. `internal/refresh` and streaming ingest

### 7.1 The queue

`refresh.Request(service, reason, wait bool) Status`, called by `ensure()`, the `ingest`
tool, the post-commit route (`POST /refresh-request`, 202) and `/refresh-file`'s
whole-service case.

- Per-service state: `idle → queued → running (→ dirty) → idle`; a request while running
  sets `dirty` → exactly one follow-up, never a parallel run. Explicit ingests update the
  auto-refresh debounce.
- One global heavy slot (`ingest` class). A second slot is not offered (app-kit measured
  5.6 GiB before streaming).
- Ordering: most recently queried first, then oldest drift; after any start, 60 s wait then
  one service at a time (warm-up). The ordering function is an interface (the decision-port
  option stays free, §vision 8).
- Paused at `shed`, refused at `refuse` and in safe mode; `store_distress` refuses.
- The ingest runs under the worker's daemon-lifetime ctx; `wait: true` callers wait under
  their own ctx and never cancel the shared job; drain on shutdown stops at a batch boundary.
- HEAD without a fork: `gitro.Head(root)` reads `.git/HEAD`, `refs/…` or `packed-refs`
  (worktree `gitdir:` handled), cached per root, invalidated by mtime; fallback
  `CommandContext` with a 2 s timeout; timeout = `freshness: unknown`.
- Reads answer at once with `staleness{index_stale: true, refreshing|queued: true,
  indexed_commit, head}` (I4).

### 7.2 Streaming, incremental ingest

- **Pass 1, stat gate:** compare `file_stats` with the live `(size, mtime_ns, ino)`; read
  only changed files; unchanged files keep `content_hash`. Full re-hash only with `ingest
  --rehash` or when stats are absent.
- **Pass 2, bounded:** 4 fetch workers feed `{path, content}` through an in-flight byte
  semaphore (`[ingest] inflight_bytes = 64 MiB`); each file is hashed, extracted and written
  in batches of 256 observations per transaction, then released. Files > 2 MiB are never
  read; binaries sniffed on 8 KiB. Peak content memory ≤ 64 MiB regardless of corpus.
- **Typed/CUE gate:** the typed pass runs only if a `.go`/`go.mod`/`go.sum`/`go.work` file is
  in the changed set since the last successful pass (`typed_pass_state`); it loads only the
  changed packages plus reverse dependencies from the stored import graph. The CUE pass
  likewise on `.cue`, and under the same heavy slot. Children: `GOFLAGS=-p=2`,
  `GOMAXPROCS=2`, `nice 10` via `SysProcAttr`, `taskpolicy -b` on darwin when present.
- **File-resolve pass:** reuses the pipeline's extractors (`p.exts`) instead of two new
  `TSExtractor`s per language per ingest; `TSExtractor.Close()` exists and is deferred;
  per-module `MemoryLimitPages` (1024 pages = 64 MiB); `file_imports` rows reused for
  unchanged hashes; `MaxExtractSize`, `IsBinary` and the known-fault fingerprint applied;
  faults counted and disclosed (the `_ =` drops at `fileresolve.go` removed). A regression
  test counts wazero modules before and after 3 ingests.
- `/refresh-file` stats before reading (2 MiB cap) and finds the file's packages with
  `WHERE source_ref = ?`, not `ListAllSymbols`.

---

## 8. launchd posture and lifecycle

### 8.1 Plist schema 2 (`plistBody`)

```xml
<!-- fathom-plist-schema: 2 -->
<key>ProgramArguments</key> … <string>serve-http</string><string>--managed</string> …
<key>RunAtLoad</key><true/>
<key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>
<key>ThrottleInterval</key><integer>60</integer>
<key>ProcessType</key><string>Background</string>
<key>LowPriorityIO</key><true/>
<key>LowPriorityBackgroundIO</key><true/>
<key>Nice</key><integer>5</integer>
<key>ExitTimeOut</key><integer>20</integer>
```

- Exit 0 = stay down (breaker stop, kill switch, "already served"). Non-zero and signal
  deaths relaunch; `Crashed` is not added (a SIGKILL is already not a successful exit).
- Throttle 60 s: gives hook breakers time to open; costs at most 60 s of fail-open hooks.
- `ProcessType Background`: the class Apple assigns to indexing; inherited by `git`/`go`
  children. On probation (RT-M6); fallback `Standard` + `Nice 5` + children under
  `taskpolicy -b`.
- `HardResourceLimits.ResidentSetSize` is not used (advisory on macOS).
- systemd mapping (documented for adopters): `MemoryHigh = budget`, `MemoryMax = exit
  mark`, `Restart=on-failure`, `RestartSec=60`, `StartLimitBurst=5`,
  `StartLimitIntervalSec=3600`, `Nice=5`, `IOSchedulingClass=idle`.

### 8.2 Self-restart

- Adoption exits **75** (was 0). `TestSelfRestartExitCodeMatchesPlistSchema` renders the
  plist and asserts the adoption code is non-zero exactly when `KeepAlive` is a dict.
- Poll stat `(size, mtime, ino)` every 20 s; SHA-256 only when it changes (today: ~4,320
  full hashes of an 89 MB binary per day).
- Cross-process cooldown: last adoption in `lifecycle.jsonl`; ≥ 5 min between adoptions;
  deploys during the cooldown coalesce. No adoption at `refuse` or above. The watcher runs
  only when `--managed`. Adoptions are not counted by the breaker.
- Drain before exit (§8.5).

### 8.3 Migrating installed plists: `fathom daemon install --reconcile`

- Doctor and `daemon status` row `daemon_posture`: find the installed plist (the label from
  `XPC_SERVICE_NAME` when under launchd, else `labelInstalled`), parse with
  `encoding/xml`, diff the tenancy keys (KeepAlive shape, ThrottleInterval, ProcessType,
  LowPriorityIO, Nice, ExitTimeOut, `--managed`, schema marker) against schema 2. WARN with
  the exact remedy: `fathom daemon install --reconcile --label com.geoffmilleraz.fathom-http`
  then the reload line.
- `--reconcile` rewrites only the tenancy keys and keeps the label, `ProgramArguments`
  paths, store, call log, `WorkingDirectory` and the environment dict; writes
  `<plist>.bak.<ts>` and a receipt (law 15). A plist fathom did not write needs `--adopt`
  (this machine's hand-written one).
- Order is enforced: `--reconcile` refuses unless the installed binary's `version --json`
  reports `tenancy_contract >= 1` (it exits 75 on adoption). Sequence: `task deploy`,
  confirm, reconcile, reload.
- Reload (`launchctl bootout` + `bootstrap`) is printed, not run, when the caller is an agent
  shell (`CLAUDECODE` or `CODEX_*` set): running it from an agent's Bash tool wedges.
- At start the daemon reads its own plist; schema < 2 is disclosed in `/statusz`,
  `resources` and (once a day) the primer: `posture: v1 (unconditional KeepAlive, no throttle)`.

### 8.4 Startup order, unavailable store, held port

1. Kill switch (`disabled.json`, `paused.json`) — before anything.
2. Breaker evaluation (§2.3).
3. Bind (was after the store open).
4. Store open. On failure, stay up in `waiting-for-store`: `/healthz` and `/statusz` answer
   with the errno; `tools/call` returns 503; reopen with backoff 5 s → 5 min; a
   `store_wait` row (not counted). EPERM on a path that `stat`s is labelled `tcc-wedge`
   (remedy names the tccutil line); ENOENT under `/Volumes/` is `volume-not-mounted`.
5. Port held: probe `/statusz`; a managed fathom of the same label → log "already served by
   pid N", exit 0; anything else → exit 75 (counted as `port_held`), `resources` and doctor
   name the holder's pid and command line, `daemon reap` handles a scratch holder.

### 8.5 Drain

`signal.Notify(SIGTERM)`: stop admitting heavy classes; let `point`/`index` finish for up to
5 s; let an in-flight ingest reach its next batch boundary for up to 15 s (`ingest_state`
stays `in_progress` if it cannot finish, so the next start resumes); write the exit row;
remove the daemon record. `debug.SetCrashOutput(~/.fathom/daemon/<id>/crash.log)` makes
runtime fatals durable.

---

## 9. The kill switch

| Command | Mechanism |
|---|---|
| `fathom daemon stop [--for 2h] [--reason …]` | write `~/.fathom/daemon/disabled.json {by, reason, since, until?}`; find the managed pid from the record, cross-checked by the census (executable path and start time must match — pid reuse guard); SIGTERM, 10 s, SIGKILL; poll until the port is closed and no process remains; print the proof (`pid 75018 gone after 3.1 s; :7575 closed; launchd will not relaunch: disabled.json`) |
| launchd relaunch after a stop | the new process's first action reads `disabled.json` and exits 0 in < 50 ms; launchd stays down |
| `--for <dur>` | while `until` is in the future a fresh process exits 75 (retried every 60 s at < 50 ms, disclosed); after it, normal start |
| `fathom daemon start` | remove `disabled.json`; write an `operator_reset` row (breaker window reset); `launchctl kickstart gui/$UID/<label>` — printed instead in an agent shell |
| `fathom daemon pause` / `resume` | `paused.json`: the daemon stays up in safe mode, picked up on the next 2 s tick |
| no binary available | `touch ~/.fathom/daemon/disabled.json` and kill the process; an empty file reads "disabled, reason unknown" |

Hooks `stat` both files before any RPC and return silently in < 5 ms when disabled.

---

## 10. Scratch daemons and tests

### 10.1 Managed vs unmanaged

Managed: started with `--managed` (plist schema 2), or `XPC_SERVICE_NAME` equals a fathom
label, or systemd `INVOCATION_ID` with `[daemon].managed = true`. Everything else is
unmanaged:

| Default | Behaviour |
|---|---|
| `--exit-with-parent` | kqueue `EVFILT_PROC NOTE_EXIT` on the startup ppid (x/sys/unix); fallback: poll `getppid() == 1` every 5 s. Survives SIGKILL of the parent, traps and `Setpgid` notwithstanding |
| `--idle-exit 20m` | no requests and no `/events` subscribers |
| `--ttl 8h` | hard stop |
| unlinked check | every 60 s, `fstat` of the executable and store; `nlink == 0` → exit (frees the deleted files it holds) |
| `--auto-refresh=false`, no self-restart watcher, no scavenger tick, feeders lazy | a scratch daemon never re-ingests real repos |
| operator state refused | `--store`/`--call-log` resolving to the managed daemon's paths (from its record) is refused without `--allow-operator-state` |
| `[tenancy.unmanaged] budget 512 MiB` | a leaked scratch daemon cannot become a second 5 GiB ingest engine |

Every exit logs its reason (`parent 4121 exited`, `idle 20m`, `ttl`, `store unlinked`).

### 10.2 Registry and census

`DaemonRecord` schema 2 adds `managed`, `ppid`, `exe` + inode, `cmdline_sha`, `idle_exit`,
`ttl`, `heartbeat_at` (every 60 s), `state_root`; removed on clean exit. The managed daemon
prunes records whose pid is dead or reused hourly (metadata, not evidence).

`fathom daemon ps`: records joined with a live census (`ps -axo pid,ppid,etime,rss,command`
filtered to `serve-http` of any fathom executable, including those with no record). Columns:
pid, addr, managed, age, footprint, idle, parent alive, unlinked files, record state.
Verdicts: `ok`, `orphaned`, `idle`, `unlinked`, `no-record`, `stale-record`.

`fathom daemon reap [--apply]`: dry-run by default; kills only unmanaged daemons whose
verdict is not `ok` (SIGTERM, 5 s, SIGKILL); never the managed label; prunes stale
records. Doctor `scratch_daemons` WARNs on any orphan or an unmanaged daemon idle > 1 h.

### 10.3 Scripts and bench

`scripts/context-bench.sh`, `scripts/stranger-check.sh` and `internal/scenariobench`
rely on the default parent-death watch (scenariobench keeps `Setpgid`). `stranger-check`
moves to `mktemp -d` and removes it in its trap (the shared-dir race; 227 MB left behind).
`TestUnmanagedServeHTTPExitsWhenParentKilled` starts a parent, runs serve-http, SIGKILLs
the parent, asserts exit within 10 s.

### 10.4 Tests never reach the operator

- `internal/testisolate.Main(m)`: sets `FATHOM_HTTP=http://127.0.0.1:1`, temp dirs for
  `FATHOM_STATE_DIR`, `XDG_STATE_HOME`, `FATHOM_CALL_LOG`, `HOME` and the hook caches;
  **exits 2** if it cannot (today a failed `MkdirTemp` silently keeps the real HOME).
- Backstop: `defaultFathomURL()` and `custodian.DefaultDaemonURL()` return
  `http://127.0.0.1:1` when `testing.Testing()` and `FATHOM_HTTP` is unset.
- Every client sends `X-Fathom-Caller: pid=…;ppid=…;origin=…;session=…;test=0|1`; the
  managed daemon answers `test=1` with 409 and counts `fathom_test_callers_rejected_total`;
  unmanaged daemons accept tests.
- `TestEveryPackageWithDaemonDefaultsIsIsolated` walks the import graph and fails for any
  package that reaches those resolvers without `testisolate.Main`.

---

## 11. Hooks

### 11.1 One deadline per invocation (`internal/hookguard`)

`deadline = start + descriptor.TimeoutSec − max(1 s, 20%)` from
`internal/harness/descriptors.go` (Claude Code 5 s hooks → 4 s; 10 s hooks → 8 s; Codex
SessionEnd 3 s → 2 s). Every RPC derives from it and sends `X-Fathom-Deadline` (ms
remaining). `fetchServices`' fresh 4 s retry client is replaced by a retry only on
`ECONNREFUSED`/`ECONNRESET` and only within the deadline — never after a timeout.
`TestNoHookRPCOutlivesItsDescriptorBudget`: every descriptor against a hanging server,
wall time ≤ deadline.

### 11.2 Shared breaker

`$HOOK_CACHE/daemon-health-<sha8(baseURL)>.json` `{state, until, failures, last_error}`,
written by rename.

- Opens on timeout, 5xx or refused: backoff 5 s, 10 s, 20 s … 300 s.
- On 503 with `Retry-After`: `until` = Retry-After (≤ 60 s); busy is not a failure.
- Half-open: one hook holds an `O_EXCL` lease (5 s TTL) and probes `GET /healthz` (300 ms);
  every other hook fails open at once.
- While open: serve the identity cache stale where possible; otherwise silent (I9, P18);
  beats record `breaker_open` and `busy`.
- `TestHookBreakerSingleProber`: 32 concurrent hook processes, exactly one probe.

### 11.3 Less fan-out

- Every identity lookup (`subagentPrimer`, `worktreeAdvice`/`indexedServiceFor`,
  `cwdChangedPrimer`, post-commit, `daemonServiceIndexed`, `doctor_languages`) goes through
  `resolveIndexedCandidateCached`, backed by `GET /identity`.
- `SubagentStart` reuses the parent session's SessionStart resolution from the session
  stamp (zero RPCs).
- `gitToplevel` memoized in-process and cached per cwd (keyed by cwd and `.git` mtime, 10
  min TTL).
- Observe-only paths (the Read|Grep|Glob observer; post-bash with no write targets) exit
  before any git or daemon work.
- The Stop hook reads the transcript from a per-session offset stamp, not up to 16 MB each
  turn.
- `custodian.walkAgentConfigFiles`: depth 6, 20,000 entries, a deadline 2 s under the hook
  timeout; a non-git root is refused and named; FileChanged audits debounced across sessions
  with a lease keyed on `sha(root, file, mtime)`.

Targets (from `hooks.jsonl`): no-op paths p50 ≤ 30 ms; breaker-open fail-open p50 ≤ 15 ms;
post-bash p99 ≤ 4.0 s (5,017 ms max today). Doctor `hook_budget`: p99 vs deadline per
channel, breaker-open minutes, harness kills (a `seen` beat with no `answered`/`error`).

---

## 12. Disk

### 12.1 `fathom gc` (dry-run default; `--apply`)

| Class | Policy | Managed daemon runs it daily at low priority? |
|---|---|---|
| call log | rotate at 64 MiB: rollup, gzip, archive (never deleted) up to 1 GiB, then report | rotate + gzip: yes; delete: never |
| verify / session / intent ledgers, `decisions.jsonl` | rotate + gzip at 32 MiB, keep all (I33 needs history) | rotate: yes |
| `session-stamps` | delete after 30 days | yes |
| `~/.fathom/daemons` with a dead pid | delete | yes |
| `$TMPDIR` `fathom-edit-set`, `fathom-*-test-*`, `fathom-stranger-check` | delete after 1 day | yes |
| `workspace/artifacts/scenariobench/worktrees` | keep last 5 runs or 14 days; `.pin` exempts | no; reported |
| `workspace/tmp/fathom-bin-*` | keep last 3 | no; reported |
| `~/.fathom/daemon/<id>/diag` | 10 captures / 200 MiB | yes |

Console tails read the rotated generation. Doctor `disk_retention`: reclaimable bytes by
class, the largest classes, internal-disk free. Targets: `workspace/` ≤ 3 GiB after `gc
--apply` (10 GB today); `~/.fathom` ≤ 500 MB.

### 12.2 Stdio logs and WAL

- Rotation moves off the scavenger tick onto its own 60 s ticker (`FATHOM_SCAVENGE=off` no
  longer disables it); the copy streams with `io.Copy`; above 256 MiB it truncates without
  copying and writes one line naming the discarded bytes.
- `journal_size_limit` 64 MiB; `wal_checkpoint(TRUNCATE)` after 5 idle minutes, at most
  hourly; `VACUUM` only via `fathom gc --apply --vacuum`.

### 12.3 Gates on a shared machine

- A machine-wide lease `flock ~/.fathom/locks/gate.lock` queues concurrent `task
  validate`/pre-push runs (`waiting for gate lease held by pid N (repo X)`).
- Gate runs use `taskpolicy -b` on macOS or `nice -n 10`, and `go test -p 4`.
- The post-commit relink runs in the background, niced; the post-commit ingest becomes
  `POST /refresh-request` (202, coalesced), refused at refuse and in safe mode.

---

## 13. Visibility

### 13.1 The resource ring

`~/.fathom/daemon/<id>/resources.ring`: fixed-size records (1 per minute × 24 h, ~160 B
each, ~230 KB), written in place and on every ladder transition: footprint,
total−released, live heap, GC CPU fraction, in-flight by class, oldest in-flight with its
tool, level and mode, pressure, swap, disk free, rejected and abandoned counts. Readers
(doctor, `resources`, the primer) need no daemon.

### 13.2 `fathom resources [--json] [--since 24h]`

Exit 0 within budget; 1 over budget, crash-looping, breaker-stopped or scratch daemons
leaked; 2 files unreadable.

```
fathom resources — com.geoffmilleraz.fathom-http (managed, plist schema 2)
state      normal · budget 4.0 GiB (auto: RAM 32 GiB / 8) · exit mark 8.0 GiB
footprint  1.2 GiB now · 24h max 3.4 GiB (09-26 21:14, shed) · ▁▁▂▅▃▁
lifecycle  up 3h12m · 2 starts/24h · 0 unclean · breaker closed
host       pressure normal · swap 3.1/22.0 GiB · internal disk 41 GiB free
in-flight  point 3 · index 1 · tree 0/2 · fleet 0/1 · ingest idle · refused 24h: 17 (tree 14)
hooks      post-edit p99 1.9s/4.0s · breaker open 0 min/24h
scratch    0 unmanaged daemons
disk       ~/.fathom 212 MB · workspace 2.8 GiB (reclaimable 0.4) · call log 23 MB
diag       last capture 09-26 21:14 refuse → ~/.fathom/daemon/…/diag/…
```

Every fact names its instrument (P22). Daemon unreachable: the report says so and shows
the ring's last sample with its age, never nothing (I38).

### 13.3 Surfaces

- **Doctor:** `daemon_footprint` rewritten to judge `phys_footprint` from the ring against
  the budget and never SKIP for an unreachable daemon; new rows `daemon_lifecycle`,
  `daemon_posture`, `scratch_daemons`, `host_pressure`, `hook_budget`, `disk_retention`,
  `tenancy_scope`. The remedy "restart the daemon" is gone; remedies name the level, class
  and top tool, and `fathom resources`.
- **Primer attention line** (≤ 160 B inside I12's 3,200 B cap, cut named): only when not
  normal — `fathom: safe mode since 21:07 (3 over-budget exits/30m; top: obligations) —
  fathom resources` or `fathom stopped by breaker 21:20 — operator: fathom daemon start`.
- **`/statusz`:** a `tenancy` block with the same projection (level and since, footprint vs
  budget, in-flight by class with tool and origin, refusals in 5 min, mode, last diag).
- **`/metrics`:** `fathom_tenancy_level`, `fathom_budget_bytes`, `fathom_host_pressure_level`,
  `fathom_swap_used_bytes`, `fathom_volume_free_bytes{vol}`, `fathom_inflight{class,origin}`,
  `fathom_oldest_inflight_seconds{class}`, `fathom_queue{class}`,
  `fathom_rejected_total{class,reason,origin}`, `fathom_abandoned_total{tool,cause}`,
  `fathom_bytes_read_total{tool}`, `fathom_bytes_written_total{subsystem}`,
  `fathom_cache_bytes{group}`, `fathom_lifecycle_starts_total{reason}`,
  `fathom_memory_released_bytes`, `fathom_gc_cpu_fraction`, `fathom_sqlite_memory_used_bytes`,
  `fathom_test_callers_rejected_total`. `HeapSys` stays only as a labelled virtual-size gauge.
- **Scavenger line:** logs total−released and phys_footprint before and after.
- **Call log:** per row `class`, `queued_ms`, `bytes_read`, `outcome ∈ {ok, partial,
  refused, abandoned}`, caller identity; `schema_version` bump (I32).

### 13.4 Automatic diagnostic capture

Triggers: first crossing of each power of two (1, 2, 4, 8 GiB) per process; entry to
`refuse`; `exit`; pressure critical. Written to `~/.fathom/daemon/<id>/diag/<ts>-<trigger>/`:
`heap.pb.gz`, `goroutine.txt` (labelled by `pprof.Do(tool, origin)` in the dispatch
wrapper), `inflight.json`, `trace.out` from `trace.FlightRecorder` (started at `shed`, 8 MiB
window), `metrics.txt`. Rate: ≤ 1 per 10 min; retention 10 captures / 200 MiB. Logged at
info with the path; the latest shows in doctor and `resources`.

### 13.5 Diagnostics socket

`~/.fathom/daemon/<id>/diag.sock` (0600) serves `/metrics`, `/tenancy`, `/debug/inflight`
and pprof with its own 4-goroutine budget, `seconds` capped at 30, one profile at a time,
not subject to admission. pprof leaves the TCP mux (404 with a pointer; **Breaking**).

---

## 14. Console

- Logs: incremental tail readers keep windowed aggregates (per-day and per-tool counts,
  latency sketches) over the maximum window any page shows (35 days), extending the
  `LedgerTail` pattern; the call-log tail keeps aggregates plus a 35-day ring (no row per
  line forever); nothing `os.ReadFile`s `hooks.jsonl`.
- Session pages: a per-session offset index (LRU of 64 sessions) into `hooks.jsonl` and its
  rotated generation; the legacy `/observatory/session` JSON route uses the same index.
- `cachedObservatoryModel` becomes a `memo.GetStale` group: serves the last model and
  rebuilds in one flight outside the global mutex.
- No loopback self-calls: `fetchCompileStates` calls `buildStatuszReport`; `surfaceSchemaTax`
  reads the in-process registry; `fleetFreshness` uses `gitro.Head` and one in-process
  statusz; the plans portfolio calls the verify-plan handler in process under a deadline
  and does not re-queue a key whose verification is running; `/learn/ask` goes through
  admission with `origin=console`.
- `console.Jobs`: 2 workers, a queue of 32, single flight per key, a superseded queued key
  is dropped; serves plans, conserve and the project index (replacing `go
  computeRefactorReport` and the wipe-at-64 map); git via `CommandContext` with 10 s;
  conserve refuses diffs above `conserve_max_files` with a named truncation.
- Feeders start on the first `/events` subscriber and stop at zero; the store poll reads only
  compile runs newer than a cursor.
- **Operate → Tenancy panel:** renders `fathom resources --json` (ADR-045: every panel renders
  a verb; JSON parity test), app-kit role utilities only, CSS via `task ui:css`
  (`cmd/fathom/CLAUDE.md`): a 24 h footprint sparkline with budget, refuse and exit marks;
  the ladder timeline; lifecycle events; in-flight by class; the scratch census; disk
  classes; per-tool per-hour p95.

---

## 15. Proof

### 15.1 In the gate (`task validate`, `-race`)

- `internal/tenancy`: ladder transitions and hysteresis (table, fake sampler and clock);
  breaker safe-mode and stop; budget resolution per RAM class (8/16/32/64 GiB, cgroup);
  gate priority (a hook never waits behind console work), zero-wait try-acquire < 1 ms,
  waiters released on ctx cancel, `observe` never refuses but logs.
- `TestEveryToolDeclaresCostClass`, `TestEveryToolHasCostRow`, the cost tests (§4.8), the
  stampede tests, `TestTenancyRules`, `TestTenancyAllowlistOnlyShrinks`.
- `TestClientDisconnectCancelsReadWork` (ctx done within 100 ms; abandoned counter +1);
  `TestIngestSurvivesDisconnectAndCoalesces` (two requests → one run + ≤ 1 follow-up);
  `TestWatermarkNotAdvancedByKilledIngest`; `TestDocOnlyChangeRunsNoTypedPass`;
  `TestIngestPeakContentWithinInflightBytes`; `TestWazeroModulesStableAcrossIngests`.
- `TestAdmissionReturnsNamed503WithRetryAfter`; `TestDiskFullIsDistressNeverZero`;
  `TestRefusalIsNamedNeverEmpty`.
- `TestPlistV2RoundTripAndReconcileKeepsOperatorKeys` (fixture shaped like this machine's
  hand-written plist); `TestSelfRestartExitCodeMatchesPlistSchema`;
  `TestDisabledFileCheckedBeforeStoreOpen` (store stub panics on open);
  `TestStopKillsDaemonIgnoringSIGTERM`; `TestStopProofRequiresPortClosed`;
  `TestTimedPauseExits75UntilExpiry`; `TestEmptyDisabledFileDisables`.
- `TestUnmanagedServeHTTPExitsWhenParentKilled`; `TestEveryPackageWithDaemonDefaultsIsIsolated`;
  `TestTestCallerRefusedByManagedDaemon`.
- `TestNoHookRPCOutlivesItsDescriptorBudget`; `TestHookBreakerSingleProber`.
- `TestResourcesRendersFromRingWhenDaemonDown`; `TestGCDryRunDeletesNothing`;
  `TestStdioRotationStreamsAndHardCaps`; `TestBudgetConfigUserScopeOnly`; CUE pairs.

### 15.2 The stress harness: `scripts/tenancy-stress.sh` (committed, not in the gate)

Runnable by anyone, heavy, run only on a quiet machine with the operator's go-ahead.

- Builds the binary under test into `workspace/tmp/`, copies the fleet store into a
  `mktemp -d`, starts an **unmanaged** daemon on a free port with `--parent-pid $$`,
  `[tenancy] budget` forced to 1 GiB (so the ladder is exercised without endangering the
  host), and an app-kit-shaped fixture corpus (generated: 40k files, 5 GiB including a
  gitignored cache dir and 400 files over 2 MiB).
- Load mix (the 2026-09-26 shape at 2× the 2026-09-24 peak rate, ~1,560 calls/min):
  32 concurrent cold `list_services`, 16 concurrent `obligations` on the fixture, 4
  `references` with `services: ["*"]`, one ingest, plus the recorded call-log mix.
- Then five SIGKILL/relaunch cycles (relaunch simulated by the harness with a 60 s
  throttle).
- Samples `/metrics` over the diag socket every 1 s; writes a reading to
  `docs/bench/readings/tenancy-stress-<date>.md` with the plain-language verdict first.
- **Pass (pre-fix binary, seatbelt only):** max footprint ≤ exit mark + one sample of
  growth; a diag capture exists; safe mode by the 3rd start; breaker stop by the 5th;
  `fathom resources` exits 1 and names `list_services`/`obligations`.
- **Pass (post-fix binary):** no exit rung; max footprint ≤ budget; hook-origin p99 ≤ 2 s;
  refusals only in `tree`/`fleet`; after a SIGKILL, no ingest in the first 60 s and one
  service at a time.

### 15.3 Field measurements

RT-M0..RT-M9 in vision 36 §6, read from the ring, lifecycle ledger, call log and
`hooks.jsonl` over 7 days on the operator's machine.

---

## 16. Rollout

Each step is a reviewable commit with a `Verified:` line, `task validate` green, then `task
deploy` (never a raw `cp`). The plan's waves:

| Wave | Content | Why first |
|---|---|---|
| **A. Tonight** | the hotfix (4200482a, RT-1); kill switch; hook deadline + breaker + disabled check; self-restart exits 75 with stat-gated hashing; plist schema 2 + `install --reconcile`; test-isolation backstop; unmanaged parent-death/idle/TTL exit + `daemon ps|reap`; obligations as `tree` with the stat cap and single-file fallback | the machine is safe while the rest lands; operator applies `reap --apply` and the reconcile after `task deploy` |
| **B. Seatbelt** | tenancy sampler, ladder, exit rung, lifecycle ledger, breaker, safe mode, warm-up; runtime/metrics instruments; ring + `fathom resources`; doctor rows + primer line; store-wait and bind-first; admission gate in `observe` (RT-M0 starts) with `r.Context()` and server timeouts | bounded regardless of cause |
| **C. Brakes** | `internal/corpus` + gitignore discovery + stat-first reads; `Sources` out of `QueryInterface`; `internal/memo` with graphstore/snapshot; derived tables + indexes + pragmas; obligations/list_services/nav/references/search rewrites; tenancy lint + ratchet; cost and stampede tests; admission to `enforce` after RT-M0 | removes the causes |
| **D. Ingest** | refresh queue; streaming stat-gated ingest; typed/CUE gating; extractor reuse + wazero Close; `ingest_state` watermark; `gitro.Head` | ingest ≤ 300 MiB and off reads |
| **E. Console and hygiene** | console tails, `console.Jobs`, in-process calls, lazy feeders, Tenancy panel; diag capture + socket + Host check; `fathom gc` + WAL + stdio rotation; gate lease; hook fan-out trims; custodian cap | the long tail |
| **F. Close** | ratchet at zero; stress-harness readings (RT-M3, RT-M5); field readings (RT-M1, M2, M6–M9); record corrections (O108, O112, CHANGELOG, fleet-index, configuration.md, memorylimit.go); domain.md platform facts | honesty debt paid |

---

## 17. Records

- ADR-047 (this design); ADR-017 amended (§6); ADR-020's premise refined (no pool cap).
- Invariants I47–I53 written VIOLATED, rising a rung as each test lands.
- INC-23 (2026-09-26), INC-24 (O108's misreading).
- P23 (a guest on the operator's machine).
- Scope G14.
- Compatibility surface additions: `-32050` + the 503 envelope, the 409, `[tenancy]`,
  `fathom resources --json`, `fathom daemon stop|start|pause|resume|ps|reap`, plist schema 2,
  the lifecycle ledger row shapes.
- `docs/intent/domain.md` platform facts (in wave F): `SetMemoryLimit` is soft and excludes
  mmap'd memory; `HeapSys` never falls; `ps` RSS ≠ `phys_footprint`; jetsam kills
  neighbours before the largest process; swap on the internal disk leads to `SQLITE_FULL`;
  `KeepAlive` dict semantics and the 10 s default throttle; `ResidentSetSize` is advisory;
  Background QoS is inherited by children; `.gitignore` is the authority for build and tool
  caches in git worktrees.
