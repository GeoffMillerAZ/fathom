### `serve-http`

Serve the MCP JSON-RPC protocol over HTTP. Suited for multi-client, platform, and CI use.

```sh
fathom serve-http --store <path> [--addr <host:port>] [--managed] [--idle-exit 20m] [--ttl 8h] [--exit-with-parent=true] [--allow-operator-state] [--module <store>]...
```

| Flag | Default | Description |
|---|---|---|
| `--store` | (required, or `[daemon].store` in config.toml) | store path (.sqlite/.db) |
| `--addr` | `127.0.0.1:7575` | Listen address — loopback by default; surfaces are not a security boundary (ADR-014 amendment) |
| `--auto-refresh` | `true` managed; `false` unmanaged | Queue a re-ingest of a service on git-HEAD drift (HEAD read from the ref files, no `git` fork); an explicit flag wins either way. The read never waits: it answers from the index as it stands with `refresh: queued (index_stale) …` in the envelope. Every whole-service ingest the daemon runs — auto-refresh, the `ingest` tool, `POST /refresh-request` (post-commit) — shares one refresh queue: one ingest per service (a request mid-run becomes one follow-up), one at a time machine-wide, the most recently queried service first, background work held 60 s after a start and staggered one per 30 s during warm-up, paused at shed and refused at refuse; `/refresh-file` leaves a file to a queued or running service refresh. One that fails before recording the new commit is retried after 30 s, doubling to at most 30 min; a read inside that wait says `index_stale`, the failure and the retry time in its `refresh` note |
| `--managed` | `false` | This is the supervised daemon (ADR-047 §10.1; plist schema 2 passes it). A launchd job whose label is the default `dev.fathom.serve-http` or the `Label` of an installed fathom agent (`XPC_SERVICE_NAME`) is detected as managed without it; a systemd unit passes it. Only the managed daemon obeys `~/.fathom/daemon/disabled.json`/`paused.json` and watches its own binary: it stats it every 20 s, hashes only on a (size, mtime, inode) change, adopts a verified new binary by exiting **75**, and keeps a 5-minute cooldown between adoptions across restarts in `~/.fathom/daemon/<label>/lifecycle.jsonl` (`FATHOM_SELF_RESTART=off` still pins the binary). Every other serve-http is **unmanaged** (a scratch daemon) and bounded by the four flags below; it also runs no self-restart watcher or scavenger tick and starts the console feeders only when `/events` is first requested |
| `--exit-with-parent` | `true` | Unmanaged: exit within 10 s of the parent process's death, SIGKILL included (kqueue `NOTE_EXIT` on macOS, a `getppid` poll everywhere); a daemon started by pid 1 has no parent to watch |
| `--idle-exit` | `20m` | Unmanaged: exit after this long with no request in flight (an open `/events` stream counts as one); `0` disables |
| `--ttl` | `8h` | Unmanaged: exit after this long regardless; `0` disables. Every bounded exit — also an unlinked (deleted or replaced) executable or store, checked every 60 s — logs its reason (`parent 4121 exited`, `idle 20m0s`, `ttl 8h0m0s`, `store unlinked`) and removes the daemon's record |
| `--lifecycle-id` | unset | Unmanaged: keep the lifecycle ledger (and so the crash-loop breaker) under `~/.fathom/daemon/<id>/lifecycle.jsonl` across relaunches — the stress harness's (RT-158). Never a managed label, an `addr-*` id or a path (refused, exit 1). Without it an unmanaged daemon keeps a per-process `addr-<port>-<pid>` ledger, removed on a clean exit. A breaker stop under it exits 0 but never writes `disabled.json` |
| `--allow-operator-state` | `false` | Unmanaged: without it, a `--store` or `--call-log` resolving to the managed daemon's (from its record) is refused and serve-http exits 1 before opening anything |
| `--call-log` | `<repo>/workspace/logs/mcp-calls.jsonl` (else `~/.fathom/logs/mcp-calls.jsonl`) | JSONL tool-call log, resolved from the daemon working directory's repository (§CLI subcommands); empty disables |
| `--module` | — | Path to a conforming expert-module store, mounted READ-ONLY so `obligations` unions in bound modules' rules; repeatable; a non-conforming store aborts startup |

**The memory budget (ADR-047, RT-29..RT-36).** Every serve-http samples its
own `phys_footprint` and its child processes every 2 s and steps a ladder
against a budget from `~/.fathom/config.toml` `[tenancy]` (auto: RAM/8,
1–4 GiB; an unmanaged daemon takes `[tenancy.unmanaged].budget`, 512 MiB):
**shed** at 0.75× drops the console and fleet memos and pauses background
work; **refuse** at 1× drops every cache, cancels in-flight tree and fleet
calls and stops whole-tree reads; the **exit mark** (2×, capped at RAM/4)
halts an in-flight ingest and quarantines its service, then — if still over
— exits **70** with an `over_budget` row in `lifecycle.jsonl` and a heap
profile under `~/.fathom/daemon/<id>/diag/`. The startup line names the
budget and its derivation; `/metrics` carries `fathom_tenancy_level` and the
footprints; `/statusz` lists quarantined services, which only a completed
explicit call of the daemon's `ingest` tool lifts (never a hook's refresh
or a shell `fathom ingest`). A scratch daemon that needs more sets
`FATHOM_TENANCY_BUDGET`. See [configuration.md](../configuration.md#the-budget-tenancy-user-scope-only).

**The lifecycle ledger and the breaker (ADR-047 §2.3, §8.4, §8.5; RT-37..RT-44,
RT-156, RT-158, RT-165).** Every start and exit is a row in
`~/.fathom/daemon/<id>/lifecycle.jsonl` (rotated at 1 MiB, one generation
kept). A start with no following exit is named at the next start —
`exit{reason: unclean}` with the jetsam or `fathom_*.diag` resource report the
system kept for that pid (`jetsam: largestProcess`, `resource: disk writes`).
Before the store and the bind, the breaker counts `over_budget`, `unclean`,
foreign-holder `port_held` and non-zero `start_failed` exits (a store that
opens and is not a database, a module that does not mount, a bind error that
is not a held port) since the last `operator_reset`: 3 in 30 min
start the daemon in **safe mode** (tree and fleet refused with `fathom safe
mode since 21:07: … — run fathom resources`; point and index served;
auto-refresh, the console feeders and the decision port off; after 30 min at
normal it steps back); 5 in 60 min, or a safe-mode process exiting over
budget while still in safe mode, **stop** it — the managed daemon writes `disabled.json {reason:
breaker}` and exits 0 (`fathom daemon start` clears it). Adoption, `stop`,
a drained SIGTERM, a store wait, the unmanaged lifetime exits and a
`task deploy` kickstart (its `deploy` intent row, `fathom daemon
deploy-intent`, is written before the kickstart) are not counted. For
`[tenancy.breaker] warmup` (120 s) after any start each heavy class holds one
slot — a second concurrent call is refused 503 under the default `admission =
"observe"` as under `"enforce"` (only `"off"` skips it) — and auto-refresh
ingests nothing inline. The daemon binds
**before** it opens its store: an unavailable store is waited for, never
exited on — `/healthz` and `/statusz` answer `waiting-for-store` with the
errno and label (`tcc-wedge` for EPERM on a path that stats, with the daemon
binary's path, signing identity and Full Disk Access / Removable Volumes
grant as the remedy; `volume-not-mounted`), tools/call is 503 `store
unavailable: …`, reopened with backoff 5 s → 5 min, one `store_wait` row per
errno (recovered by opening the path itself when SQLite's `unable to open
database file` drops it). An unmanaged daemon keeps its `--ttl`,
`--idle-exit` and `--exit-with-parent` while it waits. A port a managed fathom of the same label already serves exits 0
(`already served by pid N`); any other holder exits 75 and is named. SQLITE_FULL or
SQLITE_IOERR from a read or a write (an ingest, `/refresh-file`, an
auto-refresh ingest), or a store volume under `min_store_volume_free`, is
**store distress**: ingest and `/refresh-file` are refused with the reason and a
failed read is a named error (`list_services` `symbol_count_error`), never a
zero. The managed daemon's SIGTERM writes `exit{reason: drain}` first, closes
tree, fleet and ingest, lets point and index finish within 12 s, and exits 0;
runtime fatals go to `~/.fathom/daemon/<id>/crash.log`. `/metrics` carries
`fathom_safe_mode`, `fathom_warmup` and `fathom_store_distress`; `/statusz`'s
`tenancy` block the same, named.

**Admission (ADR-047, RT-53..RT-65).** Every tool call is admitted in its
cost class (`docs/mcp-tools.md`, "Cost classes") before its handler runs:
per-class slots derived from the budget, per-origin queue waits (a hook never
waits for a `tree` or `fleet` slot), a refusal when the remaining
`X-Fathom-Deadline` is below the tool's recent median, and the class deadline on
the call's context — which is now the request's, so a client that disconnects
cancels its read and a caller already gone is never run (an ingest runs
detached). `[tenancy].admission` defaults to
`observe` (every would-refuse logged and counted, nothing refused); `enforce`
answers HTTP 503 + `Retry-After` + JSON-RPC `-32050`; `off` is the escape
hatch. Every route requires a loopback `Host` (and a same-origin `Origin`);
the managed daemon answers a
test binary (`X-Fathom-Caller … test=1`) 409. See
[configuration.md](../configuration.md#the-budget-tenancy-user-scope-only).

The daemon also reads the `budgets` block of `.fathom/capabilities.cue` /
`~/.fathom/capabilities.cue` (project → user → built-in) for class-budgeted answers
(`list` = `query_symbols`/`references`, `composite` = `pack_context`, `map` = `repo_map`;
BA-9, **experimental**) — see [configuration.md](../configuration.md). Stdio `serve-mcp`
still uses the built-in defaults (documented gap).

HTTP endpoints:

| Method | Path | Description |
|---|---|---|
| `POST` | `/rpc` | Single JSON-RPC 2.0 request/response (4 MB body limit) — full tool surface |
| `POST` | `/rpc/{surface}` | Same protocol scoped to a named surface (`agent`, `extended`, `companion`): filters `tools/list`; an off-surface `tools/call` still **executes** and is logged `off_surface:true` (execute-and-flag, ADR-014 amendment) — the hard gate exists only behind `Server.strictSurfaces` |
| `GET` | `/healthz` | Liveness probe — returns `ok` |
| `GET` | `/events` | Live freshness (OC-13): SSE with an `id:` per event (`<epoch>-<n>`: the epoch is this daemon process's boot ID, named in the opening `hello` event), a `: keepalive` every 25 s, replay of the last 64 events after `Last-Event-ID` (or `?last_event_id=`) — an id from another epoch (a reconnect across a daemon restart) replays the whole ring, the restart event first — at most 16 subscribers (the 17th gets 503 + `Retry-After`). Kinds: `ingest` (a service's indexed content moved; a hash-gated no-op re-ingest is not one), `compile` (one per new run), `verify` (the verify ledger, polled every 2 s), `session` (the row's repo label mapped to the service whose root it names; a label that is not a name fires only the unscoped `sessions` source), `daemon`. Events carry names and IDs only, never content |
| `GET` | `/identity?path=DIR[&path=TOPLEVEL]` | Which indexed service a directory is, from the daemon's registry (RT-133; `docs/mcp-tools.md`) — the hooks' identity lookup |
| `GET` | `/debug/pprof/*`, `/debug/inflight` | pprof (loopback `Host` and loopback peer only, `seconds` ≤ 30, one profile at a time — 429 otherwise) and the in-flight table as JSON (RT-123) |
| `GET` | `/metrics` | Text metrics: `fathom_uptime_seconds`, `fathom_requests_total`, memory and memo gauges, the ladder (`fathom_tenancy_level`, `fathom_budget_bytes`, `fathom_footprint_bytes`, `fathom_children_footprint_bytes`, `fathom_host_pressure_level`, `fathom_swap_used_bytes`, `fathom_gc_cpu_fraction`, `fathom_quarantined_services`), admission (`fathom_inflight{class,origin}`, `fathom_queue{class}`, `fathom_slots{class}`, `fathom_rejected_total{class,reason,origin}`, `fathom_would_refuse_total{class,reason,origin}`, `fathom_calls_total{class,outcome}`, `fathom_admission_mode`, `fathom_test_callers_rejected_total`), `fathom_console_page_views_total{route=…}` (OC-15: HTML views per console route pattern since start; the durable 30-day record is `~/.fathom/console-views.jsonl`, `FATHOM_CONSOLE_VIEWS` overrides) and `fathom_console_event_subscribers` |
| `GET` | `/statusz` | Daemon health, per-service index freshness, channel liveness, last compile per layer, (schema 2) the daemon's own build stamp `binary_commit` / `binary_dirty` / `built_at`, and (additive) the ladder's `tenancy` block — level, marks, the last sample and every unknown reading (`footprint unknown (ladder off)`), plus `quarantined` — JSON (schema 3, unchanged) via `Accept: application/json`/`?format=json`; HTML on the console shell otherwise, where every service names its typed-edge state — run by this daemon, recorded by an earlier daemon process (replayed from the store), failed with its reason, **does not apply** (no `go.mod` at the service root), or **unknown** (a Go module with no pass recorded anywhere), never blank (OC-6; `docs/design/observability-surfaces.md` §8 item 6). Gated inside the `/rpc/modules` bearer-token allowlist when `--modules-token-env` is configured; open on the default loopback-only daemon |
| `POST` | `/reload` | Reload the store (PlatformStore re-scan in platform mode) |
| `GET` | `/`, `/fleet`, `/projects/{svc}`, `/layers`, `/layers/{layer}`, `/claims`, `/claims/{id}`, `/packs`, `/explain`, `/impact`, `/sessions`, `/sessions/{id}`, `/brief`, `/wiring`, `/statusz`, `/search`, `/operate`, `/decide`, `/learn`, `/learn/ask`, `/learn/guides`, `/learn/guides/{slug}`, `/plans`, `/plans/{svc}/{plan}`, `/plans/{svc}/{plan}/rows/{id}`, `/certifications`, `/refactors`, `/rules` | The Observatory console (see below): every page on one shell; `?format=json` returns the page's envelope (`fathom.console.page/v1`) |
| `GET` | `/observatory`, `/observatory/service`, `/observatory/impact`, `/observatory/story`, `/observatory/brief`, `/observatory/session`, `/observatory/wiring/` | Legacy routes (OC-7): an HTML request is a **308** to the page's new home; `?format=json` still returns the old JSON unchanged (`observatoryModel` v1 and its per-page slices) |
| `POST` | `/observatory/wiring/preview` · `apply` · `undo` | Law-15 wiring through `internal/wiring`: `preview` returns the plan + `plan_digest`; `apply` requires it (409 if the repo changed since preview) and returns a `receipt_digest`; `undo` requires that digest. All three refuse non-loopback `Host`, cross-origin `Origin`, unknown service ids, and paths that are not registered service roots (`requireLocalMutation`); a refusal, and a `GET` of any of the three, renders on the console shell with the reason and a way back to `/wiring` |

```sh
fathom serve-http --store workspace/db/fathom.sqlite --addr 127.0.0.1:7575
# serve-http: listening on 127.0.0.1:7575

# In another terminal:
curl -s http://localhost:7575/healthz
# ok

curl -s -X POST http://localhost:7575/rpc \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
```

---
