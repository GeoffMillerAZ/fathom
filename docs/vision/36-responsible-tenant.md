# 36 — The responsible tenant

Status: accepted 2026-09-26 (ADR-047). Owner directive of 2026-09-26: "put measures into
place to keep fathom from being a problematic tenant again. put a lot of design effort
and implement completely to make it much more responsible and safe." Design:
`docs/design/responsible-tenant.md`. Plan: `docs/plans/responsible-tenant.md`. Incident:
INC-23 (`docs/intent/incidents.md`). Principle P23; invariants I47–I53. Revised the same day
after a design review (§10; ADR-047's amendment).

This vision synthesizes three design lenses written over one audit (82 hazards, read-only,
at `4200482a`): a budget lens (admission and a ladder), an architecture lens (no request
proportional to repository size, enforced by the type system and the gate), and an
operator lens (launchd posture, a kill switch, scratch-daemon hygiene, hooks). Where they
disagreed, §9 says which way it went and why.

## 1. What went wrong

On 2026-09-26 fathom stopped being a tool on the operator's machine and became the
machine's problem.

- **The footprint.** The launchd daemon reached about 29.1 GB resident on a 32 GB Mac
  mini (jetsam reports 13:58 and 14:02 PDT, fathom the largest process), filled 20.9 of
  22.5 GB of swap, and macOS killed a dozen neighbours (contactsd, CoreLocationAgent,
  photolibraryd …) instead of fathom. With swap on a 91%-full internal disk, SQLite reads
  failed with `database or disk is full` (127 times for app-kit alone), and
  `list_services` then reported those services as `symbol_count 0`.
- **The relaunch loop.** The operator force-quit it five times in 17 minutes (20:49:33Z to
  21:06:39Z). `KeepAlive true` relaunched it each time, into cold caches and an empty
  auto-refresh map, and it grew again: `heap_sys` 19.7 GB 15 minutes after the last start.
- **The cause was a class, not a bug.** In one 62-minute profile, request paths allocated
  730.7 GB. Every hot spot did work proportional to a repository or the whole fleet:

  | Hot spot | Allocated in 62 min |
  |---|---|
  | obligations — two whole-tree reads per edit, from the PostToolUse hook (17,589 calls) | 468.6 GB |
  | `list_services` → `ServiceSummaries` — every symbol row of every service (271,612 rows) | 133.8 GB |
  | `symbol_context` → `fleetCallers` — every service loaded twice | 31.1 GB |
  | `callgraph.Build` per call | 5.5 GB |

  The 4200482a hotfix removed the file-content read under `list_services`. Twenty minutes
  after it, a profile still held 4.95 GB in use under obligations' whole-tree read.
- **Three multipliers turned cost into an outage.** Nothing deduplicated concurrent
  identical work (49 concurrent cold `list_services` at 05:24:08Z). Nothing cancelled work
  whose caller had gone (every RPC ran under `context.Background()`; calls ran 19 minutes
  after their 2-second hook clients gave up, and the session hook retried). Nothing
  refused work that could not fit (no admission control, no overload signal).
- **Ingest ran inside reads.** A read that saw drift re-ingested the whole service inline,
  holding every file's content: app-kit's discovered corpus is 5.3 GiB because discovery
  ignores `.gitignore` (a Go build cache, a cargo registry) and reads 3.5 GiB of files over
  2 MiB only to skip them. The typed pass ran on doc-only commits. Duplicate ingests of
  one service ran seconds apart.
- **The only bound was a hint.** `SetMemoryLimit(3 GiB)` is soft by the Go runtime's own
  documentation; the incident exceeded it tenfold. The daemon ran at Standard priority,
  read no host pressure, and its one footprint check (`doctor`) read the wrong gauge and
  SKIPped when the daemon was too busy to answer.
- **It was the second time.** O108 (2026-09-22: swap killed the pre-push gate twice) was
  closed "mitigated-and-bounded" on a reading of `HeapSys`, which by definition never falls.
  No invariant, test or decision came out of it.
- **The same day, around the daemon:** two scratch `serve-http` daemons from proof agents
  had run for 22–23 h holding 0.5 GB of deleted files; `cmd/fathom` tests appended rows to
  the operator's call log; hook processes timed out, retried and doubled their load on the
  struggling daemon.

## 2. The principle: a guest on the operator's machine

Fathom runs on the operator's laptop next to their editor, browser, builds and a dozen
agent sessions. It is an indexer. The house rules for a guest are simple, and P23 states
them:

1. **Declare what you take, in the unit the host measures.** A budget in `phys_footprint`,
   the number Activity Monitor and jetsam use — not `HeapSys`, not a GC target.
2. **Measure yourself and stop yourself.** No one else will: macOS does not enforce an RSS
   limit on a launchd agent. Shed, then refuse, then leave, and say why each time.
3. **Cost follows the answer, not the repository.** A request pays for what it returns.
   Only ingest reads the corpus, and ingest streams, serializes and yields (P19).
4. **When the host is struggling, do less.** Host pressure, swap and disk are inputs.
5. **Leave when asked, and when your parent leaves.** A kill switch that works with the
   daemon wedged; scratch daemons that die with their parent; exit 0 means "stay down".
6. **Be visible where the operator already looks,** even when the daemon cannot answer.

The guest's manners are deterministic thresholds, not judgement. That is also why the
decision port has no role in them (§8).

## 3. The resource contract

Declared per machine at user scope (`[tenancy]` in `~/.fathom/config.toml`; a repository
cannot raise a laptop's ceiling). Defaults and the reasoning:

| Resource | Default | On the 32 GB reference machine | Why |
|---|---|---|---|
| Budget (`phys_footprint`) | `clamp(RAM/8, 1 GiB, 4 GiB)` | 4 GiB | A background indexer takes at most an eighth of the host. The legitimate standalone costs measured fit: typed passes at 0.8–1.0 GiB with nothing else in flight. The larger "peaks" in the logs were process-wide readings during concurrent request storms. |
| Shed mark | 0.75 × budget (self + children), or host pressure warn | 3 GiB | Room to act before refusing. |
| Refuse mark | 1.0 × budget (self + children), or pressure critical, or swap used grew > 2 GiB in 5 min while fathom is past the shed mark, or swap-volume free < 5 GiB | 4 GiB | Past the budget, heavy work waits. No swap-used *fraction* trigger: macOS grows swap files on demand, so 80% used is normal (this machine at rest: 4.9 of 6.1 GB, pressure level 1). Unknown readings are disclosed, never triggers. |
| Exit mark | `min(2 × budget, RAM/4)`, own footprint, for 2 samples (a > 3× late sample counts as the second) | 8 GiB | Leaves ~24 GB for the operator. An in-flight ingest is halted and its service quarantined *before* the exit fires, so a legitimate ingest never causes a crash loop; RT-M10 checks the typed pass fits each RAM class. Reviewed down to 1.5× once RT-M4 reads. |
| GC hint (`SetMemoryLimit`) | = exit mark | 8 GiB | At the budget it would make the refuse-to-exit band a GC-thrash band; the ladder does the bounding. RT-M7 reads it, refuse windows included. |
| Admitted work envelope | ≈ 0.5 × budget (slots × per-call ceilings) — **from wave C** | 2 GiB | point 64 × 8 MiB, index 8 × 64 MiB, tree 2 × 256 MiB, fleet 1 × 512 MiB. Classes follow each tool's *current* cost, so until the rewrites land `list_services` is `fleet` and `symbol_context` is `tree`; in waves A–B the cause fixes, the ladder and the producer governor bound memory, not the envelope. |
| Cache registry | 0.25 × budget | 1 GiB | Fits the fleet symbol snapshot (~200 MB) and a few large graphs; compaction before any raise. |
| SQLite | `cache_size` 8 MiB per connection, `soft_heap_limit` 256 MiB, `hard_heap_limit` 512 MiB, `journal_size_limit` 64 MiB | — | SQLite's heap is outside Go accounting; `phys_footprint` sees it, these pragmas bound it (the soft limit alone is advisory). |
| Children | `go list`, git and rg footprints sampled and counted toward shed and refuse; killed at exit | — | P23: declare what you take; `go list -export` compiles outside the daemon's footprint. |
| CPU | `GOMAXPROCS = max(2, NumCPU/2)`; Background QoS; `Nice 5`; toolchain children `GOFLAGS=-p=2`, background priority | 5 of 10 cores | The GC limiter takes up to half of GOMAXPROCS; `go list -export` compiles at `-p=NumCPU` today. |
| Ingest | one per service, one machine-wide; 64 MiB content in flight; files > 2 MiB never read | — | Peak ingest memory bounded by a window, not a corpus (5.3 GiB → ≤ 300 MiB target). |
| Hooks | one deadline per hook = `TimeoutSec − max(1 s, 20%)`; no retry after a timeout | post-edit 4 s | The harness kills hooks at 5–10 s; today `2 s + 4 s` retries exceed 5 s. |
| Processes | one managed daemon; unmanaged ones die with their parent, idle 20 min, TTL 8 h | — | The 22-hour leaks. |
| Disk | call log and ledgers rotate + gzip (archived, never deleted); caches and stamps age out; `workspace/` ≤ 3 GiB after `gc` | — | 10 GB of `workspace/` today; 91% full internal disk. |
| Disk writes | ≤ 2 GiB/day by the managed daemon outside explicit ingest, disclosed in `resources` and doctor | — | macOS's resource report: 8.6 GB dirtied in 1,395 s against a ~99 KB/s limit. |

## 4. The mechanisms

Five groups, each owning a class of hazard. The design doc has the APIs.

### 4.1 The seatbelt: budget, ladder, breaker

`internal/tenancy` samples every 2 s and moves a ladder: **normal → shed → refuse → exit**,
with hysteresis. Shed trims the caches to half their budget, pauses background work
(inline auto-refresh, later the refresh queue, console workers), and serves stale with
disclosure. Refuse drops every cache, answers heavy classes with a named 503 (`fathom over
budget: 4.3/4.0 GiB (obligations×6 in flight)`), cancels every in-flight heavy request and
captures diagnostics. Because sampling alone cannot catch an I/O-bound stampede, every byte
producer (file reads, `Sources`, row decoding) checks the ladder's level itself and halts
within one file or 1,000 rows. At the exit mark an in-flight ingest is halted first and its
service quarantined; if the footprint is still over, the daemon writes a lifecycle row
with the reason, the footprint and the top in-flight work and exits 70 at once. A lifecycle ledger and a crash-loop breaker make repeated exits converge: 3
counted exits in 30 minutes start in **safe mode** (index served stale, no heavy classes,
no auto-refresh); 5 in 60 minutes write `disabled.json` and exit 0, and launchd leaves it
down. On 2026-09-26 the third start would have come up in safe mode and stopped the regrowth.

### 4.2 The brakes: no request costs more than its answer

- **The rule is structural.** Query code cannot name a whole-tree read: `Sources` leaves
  `mcp.QueryInterface`; request code reads through `internal/corpus` — stat-capped single
  files and budgeted, early-stopping scans that report a lower bound (I38).
- **Whole-repository facts are computed at ingest,** per watermark: file stats, a service
  summary (counts, languages), a normalized reverse call index, routes and HTTP calls,
  per-file imports. `list_services` becomes one 21-row select; obligations reads one file;
  cross-service callers become one indexed query.
- **One cache registry** (`internal/memo`): single-flight with context-aware waits,
  byte-accounted under one budget, evicted on a new watermark. One graph store serves MCP
  tools and the console, replacing three caches that could each hold the same graph.
- **Enforced before merge.** A tenancy lint of purely syntactic rules forbids the patterns
  (whole-file reads and walks in handler packages, `ListAllSymbols` in handlers,
  `exec.Command` without a context, `context.Background()` in handlers, bare `go`
  statements, package-level maps in handler packages without an allowlist entry, test
  commands that drop the environment), with an allowlist that may only shrink. Per-tool cost tests assert bytes read and allocations at 1× and 10× fleet
  scale — a point or service tool may grow at most 1.5× when the fleet grows 10×. Stampede
  tests assert 32 cold callers cause one build, counting bytes, not calls (the DG-18 test
  counted calls sequentially and missed the incident).

### 4.3 Admission and cancellation

Every tool declares a cost class — `point`, `index`, `tree`, `fleet`, `ingest` — and every
call, in-process console call, batch sub-call and console job takes a ticket. A hook never
waits for tree or fleet work: it gets a fast 503 and stays silent (I9). Read work runs
under the request's context and the client's deadline, and dies when the client leaves;
only ingest runs detached, deduplicated per service. `batch` is capped at 16 sub-calls,
each admitted separately. The console calls tool handlers in process through the same
wrapper instead of looping back over HTTP.

### 4.4 Ingest off the request path

A refresh queue coalesces triggers per service (one in flight, one dirty follow-up), runs
one heavy job machine-wide, orders by demand, and staggers after any start. Reads answer
from the current index with `index_stale: true, refresh: queued`. HEAD is read from
`.git/HEAD` and ref files, not a fork per read. Ingest streams (64 MiB in flight), reads
only files whose stat changed, skips the typed and CUE passes when no Go or CUE input
changed, reuses its extractors (closing the wazero module leak), and advances the
watermark only at commit.

### 4.5 Hygiene around the daemon

- **launchd posture:** plist schema 2 (`KeepAlive {SuccessfulExit: false}`, throttle 60 s,
  Background, low-priority I/O, `--managed`), and `fathom daemon install --reconcile` to
  migrate existing installs, including this machine's hand-written one; doctor names drift.
- **A kill switch** that needs nothing working: `fathom daemon stop` (writes
  `disabled.json`, kills, proves the port is closed), `start`, `pause`, `resume`; with no
  binary, `touch ~/.fathom/daemon/disabled.json` works. It governs only the managed daemon.
  Under a not-yet-reconciled schema-1 plist it says truthfully that launchd will relaunch
  every 10 s (each start exits in < 50 ms) and prints the `launchctl bootout` line.
- **Scratch daemons die with their parent,** go idle-exit, and cannot touch the operator's
  store or call log; `fathom daemon ps|reap` is the census.
- **Tests never reach the operator** (shared `TestMain` setting `FATHOM_TEST=1`, which child
  processes inherit; a `testing.Testing()` backstop; a 409 at the daemon).
- **Hooks** share one deadline and a breaker file with a single prober, reuse the parent
  session's identity resolution, and exit observe-only paths before any git or daemon work.
- **Disk:** `fathom gc`, WAL caps, gate runs serialized machine-wide and niced.

### 4.6 Visibility

One verb, `fathom resources`, reads a persisted 24-hour ring and the lifecycle ledger, so it
answers when the daemon is down. Doctor rows judge `phys_footprint` from the ring and never
SKIP into silence; the session primer carries one line only when not normal; Operate gets a
Tenancy panel that renders the verb; `/metrics` gains in-flight, rejected, abandoned,
pressure and released-memory series; threshold crossings capture a heap profile, a labelled
goroutine dump, the in-flight table and a flight-recorder trace; pprof stays behind the
loopback Host check with its duration capped and one profile at a time (a private socket is
deferred until the stress harness shows `/metrics` unreachable under load).

## 5. What it refuses to do

- **Trust the soft limit** or any OS limit macOS documents as advisory.
- **Read `HeapSys` as a footprint,** or attribute a process-wide heap reading to one pass.
- **Serve a zero it did not earn.** A refused, truncated or disk-full answer names itself;
  `symbol_count 0` from a failed read is a bug (I38).
- **Silently cap** ADR-020's read pool; concurrency is admitted upstream instead.
- **Idle-exit the managed daemon.** A cold start is the stampede; it idles by shrinking.
- **Delete evidence** in retention. Call logs and ledgers are archived, never deleted.
- **Put a model in the protection path** (§8).
- **Pre-build the future.** No identifier-occurrence index, no learned admission estimator,
  no socket activation, no diagnostics socket, no ingest worker process, no Linux PSI or
  cgroup readers — each has a named trigger in the plan.

## 6. Measurements and kill criteria

Pre-registered. Instruments are the ring, the lifecycle ledger, the call log,
`hooks.jsonl`, and the committed stress harness `scripts/tenancy-stress.sh` (against a
scratch unmanaged daemon on a copy of the store — never the live one).

| Id | Claim | Pass | Kill or adjust |
|---|---|---|---|
| RT-M0 | Admission observed before it refuses | 3 days in `admission = "observe"`, on the post-wave-C binary (so slots are tuned to post-rewrite costs), report would-refuse volume by class and origin | > 5% of hook calls would be refused in ordinary hours: retune slots before enforcing |
| RT-M1 | Bounded tenant | 7 days of operator traffic: `phys_footprint` ≤ exit mark at every sample and ≤ budget outside refuse windows; refuse windows ≤ 15 min/day in total; zero exit rungs, zero unclean exits | An exit on legitimate work more than once a month after waves C–D: raise `auto` to RAM/6 or retract the exit rung |
| RT-M2 | Not over-throttled | refusals < 1% of calls outside shed/refuse windows | > 5%: widen slots; never disable silently |
| RT-M3 | The storm is survivable | stress harness (`--lifecycle-id`, fixture on the external volume, operator go-ahead per run), wave-B binary (seatbelt without brakes): max footprint ≤ exit mark + 1 GiB, measured; a diag capture; safe mode by the 3rd start, stopped by the 5th. Post-wave-D binary: no exit, footprint ≤ budget | Any unbounded relaunch in the ledger reopens ADR-047 |
| RT-M4 | Request cost no longer scales with repositories | 60 min of real traffic: request-path allocation per call ≤ 1/20 of the heap-0518 window's per-call mean (normalized, since traffic varies); no `Sources`/`ListAllSymbols` frame under a handler; obligations p50 ≤ 60 ms, p99 ≤ 300 ms; app-kit ingest ≤ 300 MiB content+extract; doc-only commit runs 0 typed passes | Point/Service cost tests flaking > 1% of gate runs: keep the bytes-read assertion in the gate, move allocations to the harness |
| RT-M5 | Relaunch is safe | harness variant: SIGKILL mid-load, relaunch: no ingest in the first 60 s, one service at a time, footprint ≤ budget | Otherwise warm-up and stagger are redesigned before any idle-exit is considered |
| RT-M6 | Background QoS does not hurt hooks | after waves C–D, `ProcessType` alternates Background/Standard by day for 14 days; Background-day post-edit p99 within +25% of Standard days (relative only) | Revert to `Standard` + `Nice 5`, children under `taskpolicy -b` |
| RT-M7 | The GC hint does not thrash | GC CPU fraction ≤ 25% sustained below the refuse mark and ≤ 50% inside refuse windows | Move the hint to `off` |
| RT-M8 | Hygiene holds | 7 days: 0 orphaned unmanaged daemons older than 1 h, 0 test rows in the operator call log, post-* hook p99 ≤ 4.0 s, managed idle CPU ≤ 5 s/h, `workspace/` ≤ 3 GiB after `gc` | The specific mechanism is revisited, not the contract |
| RT-M9 | The tenancy layer is cheap | its own idle overhead ≤ 1% CPU and ≤ 20 MB | Profile and cut it back |
| RT-M10 | The exit mark is feasible per RAM class | fathom's own full ingest with typed pass (self + children) peaks under the exit mark at forced budgets of 1, 2 and 4 GiB | Raise the budget floor for a failing class before its exit rung is relied on |
| RT-M11 | `.gitignore` discovery removes only ignored files | dry-run stats over every service: no service loses > 1% of observations outside gitignored dirs | The default does not flip for that service |
| RT-M12 | Disk writes are bounded | 7 days: ≤ 2 GiB/day outside explicit ingest | Find the writer and bound it |
| RT-M13 | The host triggers are quiet on a healthy Mac | 7 days: ≤ 2% of time in shed or above; no host-trigger entry at normal pressure | Retune the host triggers |

## 7. What changes in the records

INC-23 records the incident with the invariants as its outcome; INC-24 records O108's
misreading. Invariants I47–I53 are written VIOLATED today and move up a rung as their tests
land. Principle P23 joins P19. Scope gains G14 (the plan closes and RT-M1/RT-M3 read PASS).
ADR-017 is amended (no manifest fallback for git services) and I21 by I51 (bytes, not
entries). O108, O112, the CHANGELOG line "bounded at the peaks", `fleet-index.md`'s
`heap_sys` guidance, `memorylimit.go`'s comment and `docs/configuration.md` are corrected
in the build waves.

## 8. The decision port

**Where it must not go:** admission, the ladder, the breaker, cache eviction, refresh
gating, cost classification, the lint, hook guards and the kill switch. These must be
deterministic, local, sub-millisecond and work with no network, key or spend (P2, P11, P18);
I14 forbids the port in hook processes. A model call made because the host is under memory
pressure adds allocation, latency and egress exactly when they hurt. Cost of putting it
there: a new failure mode inside the protection path. Not paid.

**Where it is governed, not used:** the port's own calls (`/route` per prompt, the
obligations re-rank when enabled) are admitted as their own class with two slots and are
closed at refuse and in safe mode; I42's byte-identical default means skipping them never
changes served content. Measurement: `/route` p99 against prompt-hook budget breaches from
the decision log and hook beats; if route latency drives breaches, cache route decisions
per prompt hash.

**Where it could help, off and unbuilt:** ordering the refresh backlog when more than three
services are queued (a typed `choice` over demand signals), and classifying a recorded
incident window (stampede, disk-full, leak, crash loop) to order doctor's remedies. Cost:
one batched call per backlog event or explicit invocation. Measurement: stale reads served
per hour against deterministic most-recently-queried order, pre-registered at ≥ 20% fewer;
correct-shape rate over ≥ 5 labelled episodes, pre-registered at ≥ 15 points. With two
incidents and a serialized single-digit backlog, neither is provable today, so neither is
built; the refresh ordering function is an interface, which keeps the option free.

## 9. How the three lenses were reconciled

| Question | Budget lens | Architecture lens | Operator lens | Settled |
|---|---|---|---|---|
| Exit mark | 1.5 × for 30 s | none (envelope design) | 2 × budget, ≤ RAM/4 | 2 ×, ≤ RAM/4; an in-flight ingest is halted and quarantined before it fires (review); reviewed to 1.5 × after RT-M4 |
| GC soft limit | 0.75 × budget | unchanged | at the exit mark | at the exit mark (review: at the budget it thrashes between refuse and exit); RT-M7 moves it to `off` if GC still thrashes |
| Admission weights | EWMA byte reservations | slots + per-call ceilings | slots | slots × per-call ceilings (deterministic); reservations rejected unless RT-M3 shows the envelope exceeded |
| `KeepAlive` | `SuccessfulExit:false` | + `Crashed:true` | `SuccessfulExit:false` only | `SuccessfulExit:false` — a SIGKILL is already not a successful exit |
| Throttle | 30 s | — | 60 s | 60 s — gives hook breakers time to open |
| Exit codes | 75 over-budget, 76 adoption | 75 adoption | 70 over-budget, 75 adoption | 70 over-budget (counted), 75 adoption (not counted) |
| Breaker | safe 3/30 m, stop 6/60 m | — | safe 3/30 m, stop 5/60 m | 3/30 m and 5/60 m, or a safe-mode over-budget exit |
| `ProcessType` | nice + QoS children | measure before choosing | Background on probation | Background on probation, RT-M6 reverts; toolchain children always niced |
| `references` default | fleet class | own service (breaking) | — | own service; `services: ["*"]` opts in |
| Enforcement of the request rule | cost-class tests | type removal + lint + 10× tests | — | all three; the lint is a Go test, not a vet tool |
| Refusal before enforce | observe 3 days first | — | enforce | seatbelt (ladder, governor, breaker, exit) enforces from wave B; admission observes (the default) until RT-M0 reads on the post-wave-C binary, then enforces |
| Config | `[budget]` | `[daemon.memory]` | `[tenancy]` | `[tenancy]`, user scope only |
| Where state lives | `~/.fathom/state/` | — | `~/.fathom/daemon/<id>/` | per-daemon dir (records move in too); `disabled.json`/`paused.json` global, governing only the managed daemon |

## 10. What the design review changed (2026-09-26)

A review of this vision, the design and the plan found that the first draft would not have
stopped the next incident on its first deploy. What changed:

- **Wave A removes the causes, not only adds switches.** The obligations fixes (single-file
  fallback, layer coverage from names memoized per watermark, the CUE union memo), a
  `list_services` that loads no symbol row (SQL aggregates, no new table), the meta-only
  "is X indexed", the connector's stat cap and default cache ignores, two process-wide
  `Sources` slots that hooks never wait for, and a detached, serialized inline ingest all
  ship first. Posture reconcile and the daemon census may follow as A2.
- **Classes follow today's costs.** The admission envelope is real only after wave C; until
  then the ladder and the new producer governor bound memory.
- **The ladder is harder to outrun and quieter on a healthy Mac:** producers check the level
  themselves, refuse cancels every heavy request, a late sample escalates, the exit skips the
  drain; the swap-used fraction is no longer a trigger; unknown readings never are.
- **No crash loop from a legitimate ingest:** the exit rung halts an ingest and quarantines
  its service first, and the watermark only moves at commit (moved to wave B).
- **Honest lifecycle:** SIGTERM writes its row first and exits 0 within 12 s; deploys are
  not counted by the breaker; under an old plist the kill switch says launchd will relaunch.
- **Smaller surface:** the diagnostics socket, Linux PSI/cgroup readers and an ingest worker
  process are deferred with triggers; the lint keeps only rules a parser can decide.

The decision port's place (§8) did not change: it stays out of every amended mechanism. The
review's one open trade-off is the ingest worker process (RT-D6), weighed in ADR-047's
amendment.
