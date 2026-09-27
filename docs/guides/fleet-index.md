# The fleet index — daemon, refresh discipline, deploys, dogfood

How agents working in fathom (or any fleet repo) consume fathom itself. The
root `CLAUDE.md` carries the one-line rules; this guide is the full
instruction set. (Moved out of the root CLAUDE.md 2026-08-12 — this is the
canonical home now.)

## The always-on daemon

Fathom indexes the fleet (agentic-kit, app-kit, fathom; platform-kit once
it's added) into `workspace/db/fleet.sqlite`, served by a **single always-on
HTTP daemon** — a launchd agent (`fathom daemon status` names yours) running `serve-http`
at `http://localhost:7575/rpc/agent`, registered user-scope as an **HTTP
MCP**. `/rpc/agent` is the curated 25-tool agent surface
(`internal/mcp/surface.go`; `/rpc/companion` is a compatible alias; bare
`/rpc` is the full, unscoped registry — see `docs/mcp-tools.md`). Every
session shares one index and one
store, so there is no per-session server, no single-writer lock contention,
and no "Failed to connect."

**Restart caveat:** the registered URL is read once at session start, so a
session started before either (a) the daemon first switched to `/rpc/agent`
or (b) your own client's registration was pointed at it, never sees the
surface change — check `~/.claude.json` for the registered URL, and if a
session doesn't see the fathom tools (`find_symbol`, `repo_map`, …), restart
it once. `server_stats`' `surface_traffic` block (A1-1,
`docs/plans/activation.md`) answers "is anything flowing on `/rpc/agent`?"
directly from the daemon, without tailing the call log.

**The daemon's own Go environment is load-bearing.** launchd hands an agent
almost nothing, so `fathom daemon install` writes `PATH`, `GOPATH`, `GOCACHE`
and `GOMODCACHE` (absolute) into the plist. Without them the daemon's
typed-edge pass — the go/types load that produces resolved call edges —
fails on every ingest of a Go service and falls back to syntax-only edges,
which look identical to a healthy index from every query. That is exactly
what happened on 2026-09-12: `GOPATH` reached the daemon unset, go resolved
its default to the literal `"$HOME/go"`, and the fleet was served a weaker
graph than the tests measured for an unknown number of days.

Three things now make that visible, and any one of them is enough:

```sh
fathom doctor                 # the typed_edges row: FAIL with the reason and the remedy
fathom daemon status          # names any Go variable MISSING from the installed plist
curl -s localhost:7575/statusz -H 'Accept: application/json' | jq '.services[].typed_edges'
```

`staleness_report` carries the same `{known, packages, failed, reason}` block,
so a session can ask "is this graph typed?" in the same call that asks "is it
fresh?". `known: false` means this daemon process has not ingested the service
since it started — it is not a pass. A daemon installed by an older binary is
repaired by `fathom daemon install --force` followed by a `launchctl kickstart`.

**Ingest itself now says so, not just a follow-up call (O82,
.fathom/observations.md).** The 2026-09-16 incident above had a second half:
even with the environment fix live, the `ingest` tool's own response stayed a
bare `{"ok":true}` when a Go tree's typed-edge pass saw packages but processed
none (the connector-scope filter excluded every one — a go.work naming a
sibling checkout that existed only beside the operator's own checkout, not
beside the push clone being ingested) — `test_impact`/`verify test_reaches`
silently read "no test reaches it" and a push was refused, with nothing in
the ingest that ran it to explain why. `ingest`'s result, `fathom ingest`
(text and `--json`), and `list_services`' per-service row now all carry
`typed_edges:{packages_seen, packages_processed, edges, failed, error?,
fallback?}` — the SAME fact this section's three checks already read — and a
`failed:true` flips the envelope's own `ok`/exit code, never a silent pass
over a syntax-only graph. `fallback:"GOWORK=off"` names when an ambient
workspace was discoverable and overridden (TypedEdgeEnv forces GOWORK=off
unconditionally, so this is disclosure, not a live risk).

**A hook may never re-point a service (O82).** `worktree.CheckReroot`
(O22/PC-33) only refuses re-rooting a service onto a LINKED git worktree of
its own recorded root; a plain `git clone` elsewhere sails through it. The
daemon-side `ingest` handler now refuses ANY `path` that differs from a
service's recorded root unless `remap:true` is also passed (the deliberate-
move escape hatch, parallel to the store-direct `fathom remap` verb), and the
lefthook `fathom-refresh` post-commit step (`post-commit-hook` verb) checks
the registered root via `list_services` BEFORE calling `ingest` and skips
with a plain message on a mismatch — never stumbling into that refusal and
reporting a confusing "daemon refresh failed" line for what is really "this
hook is running inside the wrong checkout".

## Daemon logging (diagnostics sweep, 2026-09-23)

Every daemon diagnostic line now goes through `internal/dlog` — a leveled,
timestamped `log/slog` wrapper with per-key dedupe — instead of a bare,
unattributable `fmt.Fprintf(stderr/stdout, ...)`. Before this sweep the
daemon's error log carried no timestamps at all, and one condition (a stale
`list_services` re-walk against a vanished service root) alone was measured
at 97%+ of the entire log's volume with no dedup.

- **`FATHOM_LOG_LEVEL`** (env, unset default `info`): `debug` / `info` /
  `warn` / `error`, case-insensitive; an unset or unrecognized value falls
  back to `info` rather than refusing to start — an operational tuning
  knob, the same convention `FATHOM_SCAVENGE_INTERVAL` follows.
- **Format**: RFC3339-timestamped, leveled text lines
  (`slog.NewTextHandler`). A repeating condition (the same dedupe key) logs
  once immediately, then folds every further occurrence into one
  `"<msg> (repeated N times since <first-seen>)"` line flushed on the same
  cadence as the memory scavenger (`FATHOM_SCAVENGE_INTERVAL`, default 15
  min) and once at graceful shutdown.
- **Rotation**: the launchd-piped `~/.fathom/logs/fathom-http.log`/`.err`
  are truncated in place (not renamed — launchd holds the fd open for the
  daemon's entire lifetime) once either exceeds 64MB, keeping exactly one
  prior generation (`fathom-http.log.1`); piggybacked on the same ticker as
  the memory scavenger, so `FATHOM_SCAVENGE=off` pauses log rotation too.
- **`X-Fathom-Request-Id`**: every HTTP response now carries this header
  (also logged on the per-request access line — method, path, status,
  duration) so a request an operator sees fail can be traced back to its
  server-side log line without guessing from method/path/timing alone.
- **Startup bind retry**: if the configured address is already in use (a
  previous instance from a self-restart or `task deploy` still exiting),
  the daemon retries the bind with backoff for up to 10s, logging each
  attempt at `warn`, before failing with the original bind error — absorbs
  the launchd restart race without masking a genuinely stuck port forever.

## Working against the index

1. **A verb first, a lookup second (ADR-035 decision 1, docs/vision/
   29-verbs-not-lookups.md §3 wave A).** Name the verb whose answer grep
   cannot give before reaching for a lookup: `blast_radius` (who
   transitively breaks), `review_diff` (one cited verdict over a diff),
   `test_impact` (which tests reach a symbol), `rename_impact` (every call
   site a rename must touch) and its read-only sibling
   `safe_delete_check` (can this symbol go, or a proven-zero receipt) lead
   — never chase another lookup-cost claim, since the shapes grep already
   answers stay grep's job. Resolve where / what / who-calls via fathom
   (`find_symbol`, `query_symbols`, `references`, `find_callers`,
   `symbol_context`, `outline`, `repo_map`, `pack_context`, `obligations`,
   `staleness_report`) for the questions only an index answers — where,
   who-calls, what breaks (RC-23, docs/vision/28-receipts.md §3.5): what
   governs this repository is `obligations` called with no `file` (the root
   conventions, not a file; `path` is accepted as an alias for `file` — O82,
   `.fathom/observations.md` — so reaching for the more common name never
   silently falls back to root-scope instead of the file you meant); is
   this still true / what was known is
   `staleness_report`; a literal string, a config value, or a log line
   routes to `search` (RC-24: grep with a receipt — the hit plus its
   enclosing symbol from the index) when fathom is wired, plain grep
   otherwise, and a document-versus-document contradiction is still
   grep's either way (vision 27 §8) — `references` replaces fleet-wide
   grep for a symbol, `symbol_context` gives def+callers+callees+tests in
   one call (plus an inline definition snippet), `read_source` reads an
   exact line range from the indexed corpus, `obligations` surfaces
   governing rules for a file before editing it. Read files to *edit*
   them, not to discover them. The index is fleet-wide, so cross-repo
   questions are one query. (These are the core of the 25-tool `agent`
   surface — see `docs/mcp-tools.md`; `memory_search` is **experimental**;
   `search` (RC-24) and `discoveries` (RC-27, its own kill criterion — see
   "Discoveries and routes" below) are the newest members.)
   Every Edit/Write also gets governing rules injected via the PostToolUse
   edit-hook (`docs/design/coding-agent-excellence.md` §5.2) — treat them as
   binding, not advisory. **ADR-038:** a `.fathom/guard.cue` rule marked
   `enforce: true` goes further and DENIES a violating PreToolUse
   Edit/Write/MultiEdit or Bash call outright (a one-line reason in place of
   the tool call), rather than only injecting advisory text after it lands;
   `FATHOM_ENFORCE=off` / `[guards] enforce = false` is the escape hatch.
2. **Refresh through the server, never the CLI.** SQLite (ADR-020) allows
   many concurrent readers, but writes still serialize — a shell
   `fathom ingest` racing the daemon's own writes can hit its busy_timeout
   and fail with a lock error. Call the `ingest` MCP tool
   (`{"service":"<this-repo>"}`) at session start and after each commit
   (hash-gated, ~0.2s when clean). Because the store is shared, your refresh
   updates the index for *every* session. The tool is honest about a
   wedged reader (IT-14/IT-15): files it could not read are named with
   their errno and the call returns a JSON-RPC **error** (cause + hint),
   never `ok:true`; a run that could read *nothing* while the store already
   holds the service **refuses to write** (`ingest refused`), and on
   "operation not permitted" the hint is the external-volume TCC reset
   (`tccutil reset SystemPolicyRemovableVolumes <terminal bundle id>`).
   Every ingest that wrote anything busts the service-identity cache.
3. **Honesty.** After heavy *uncommitted* edits, call `staleness_report`
   before trusting results, or just read the file you're about to change.
4. **Scope.** Only fleet repos are indexed. A repo that isn't a fathom
   service yet must be ingested as a new service before its symbols appear.

Repo-specific index excludes live in `.fathomignore` (`ref/`, `tmp/`,
`workspace/`, `bin/`).

## Discoveries and routes

Receipts on the hand-off (RC-25..RC-30, `docs/vision/28-receipts.md` §4): a
**discovery** is a pointer a tool answer established — a symbol id
(`internal/mcp.registerReferences`) or a repo-relative file path — never
rendered text. Every agent-surface call gets its answer's top pointers
extracted mechanically (no model distills anything) and logged; an agent
may also file one explicitly (`fathom thread point`). Nothing is ever
served from what a pointer meant when it was recorded — every render
resolves it FRESH against the live index and marks what no longer holds.

- **`fathom discoveries [--since T] [--budget N] [--json]`** (and the
  `discoveries` MCP tool, `{service, since?, budget?}`) renders this
  repo's recent pointers, deduped and ranked by recency then frequency,
  each resolved fresh — `symbol <ptr> → file:line`, `read <path>`,
  `<ptr> — stale (unresolved)` when the index is reachable and genuinely
  no longer knows the pointer, or `<ptr> — unresolved (index unreachable:
  <err>)` when the index itself could not be consulted at all (an
  operational fact, never the positive "stale" claim). Read it before
  re-exploring what a prior call, or a prior session, already found.
  Resolution is lazy and budget-bounded: pointers are resolved one at a
  time inside the render loop and stop the moment the byte budget fills,
  so a large or slow pointer set can never cost more round trips than the
  render actually needed.
- **Workers.** A subagent's `SubagentStart` primer appends the parent
  session's discoveries since its own recorded start, within 1,200 bytes
  total (the plain ≤600-byte primer stays the cap when there is nothing
  to add). The scenario driver gives a wired arm's planner-workers cell
  the same brief (`workerDiscoveryBrief`), so a bench reading measures the
  feature, not the prompt. The brief is capped to the 12 most relevant
  pointers (recency×frequency) before resolution even starts, and the
  whole resolution pass carries its own ~2s deadline independent of the
  hook's own timeout: past it, the discoveries brief is dropped and the
  constant identity+opening-moves primer ships alone, on time, rather than
  a slow or unreachable index costing the primer that sits ahead of it.
- **The next session.** A session-end note carries that session's top
  pointers; the next `SessionStart` thread brief renders them resolved
  fresh, inside its existing budget, as "last session established: …".
- **Routes** (long-lived discovery paths). `fathom routes [package|concern]`
  derives, per package prefix, the pointers established in **two or more
  distinct sessions** (attributed via `workspace/logs/sessions.jsonl`'s own
  recorded windows) over the last 30 days, resolved fresh and dropped the
  moment the index CONFIRMS them gone. An unreachable index is reported as
  its own operational line (`routes unknown — index unreachable: <err>`,
  or `index_unreachable` in `--json`) — never the empty-list "no routes
  yet", which is a positive claim the index never made when it was simply
  never consulted. The `SessionStart` primer names routes in ONE line only
  when at least one exists (`routes known for N package(s) — fathom
  routes`).
- **`fathom thread promote --route <package> --plan <file> --row <id>
  [--service S] [--url URL | --store DB]`** writes that route's pointers —
  the SAME derivation `fathom routes` uses, resolved fresh and refused
  outright if the index is unreachable (never write an unconfirmed route)
  — onto the named plan row's `ctx:` annotation, UNIONED with whatever
  pointers the row already carries (a promotion extends a row's discovery
  pack, it never replaces what a human or a prior promotion already
  grounded there). This is the OTHER `thread promote` mode: `--route`
  never scaffolds a new plan, it annotates an existing row.
- **Scope by identity.** A thread deposit is now tagged by the repo's
  identity (the normalized `origin` URL, else the absolute path) rather
  than a directory basename — two worktrees of unrelated repos that happen
  to share a basename no longer merge each other's notes (F5); an older
  item with no identity tag still matches on the legacy basename rule, so
  nothing existing is hidden.
- **Every discovery action counts (RC-56).** reuse-pilot-2 found the wired
  planners reaching for grep and Read, never a fathom tool, so the call log
  held no pointers and every worker brief was empty. The counting-only
  PreToolUse observer that already sees every Grep/Glob/Read/shell-search
  call (`cmd/fathom/discovery.go`, instrument B) also records what it saw —
  a Read's file, a Grep/Glob's `path` argument (never its pattern: a
  pattern is not a location), a recognizable shell read/search's literal
  path — to `workspace/logs/observed-discoveries.jsonl`, zero daemon calls,
  zero index lookups. `fathom discoveries` and `fathom routes` merge these
  rows into the same ranked pointer set the call log feeds; a pointer that
  came only from observation renders `read <path>` or `grepped <path>`
  instead of a wrong "stale (unresolved)", so a planner that only grepped
  still hands its workers a brief.
- **Kill criteria, pre-registered (vision 28 §4).** The `discoveries` tool
  is the one deliberate exception to the 19-tool agent-surface diet
  (`surface_diet_test.go`, TD-2): M5 at K=3 with the worker brief —
  `facts_reused >= 3` per worker and tokens-to-first-edit below the
  control on two of three tasks, else the worker brief is removed and
  `discoveries` stays pull-only (and the diet cap drops back to 17).
  Routes: pulled in five of the next twenty sessions on this repo, or
  removed.

## Staleness — two kinds

- **Data staleness** is handled automatically: `serve-http` re-ingests a
  service on git-HEAD drift before answering (`--auto-refresh`, default on),
  and you can force it via the `ingest` MCP tool. The inline re-ingest runs
  detached from its requester, inline auto-refresh ingests run one at a time
  (the `ingest` tool does not share that ticket yet), other reads of the
  service meanwhile answer from the index as it stands, and one that fails
  before recording the new commit is retried after a backoff (30 s doubling
  to 30 min). When the daemon is down, the post-commit hook
  ingests nothing and says so; re-run `fathom post-commit-hook --dir <repo>`
  (or the `ingest` MCP tool) once it is back.
- **Binary staleness** is the one to watch — the daemon runs an **installed
  copy** at `~/.local/libexec/fathom/fathom` (never `bin/fathom` directly,
  so a post-commit rebuild can't swap the running daemon's binary out from
  under launchd). A committed Go fix reaches the daemon only after a
  deliberate deploy — see below. The `post-commit` hook rebuilds
  `bin/fathom` on Go changes; if a tool misbehaves, compare `fathom version`
  to `git rev-parse HEAD` — a mismatch means stale, not buggy.

## Deploying the daemon: `task deploy`, never a raw `cp`

`task deploy` is the only sanctioned path from "committed a daemon fix" to
"agents actually get it". It builds, installs to
`~/.local/libexec/fathom/fathom` via write-to-temp + rename, signs with the
stable `fathom-dev` codesigning identity — and **fails** (`fathom-dev signing
identity missing`, the running daemon untouched) when it is absent, instead
of installing an ad-hoc binary (RT-147) — kickstarts the
launchd job, then **asserts** the running process is the freshly built
binary and that `fathom version` matches `git rev-parse HEAD`. Run it from a
real terminal (launchd + TCC interactions don't survive sandboxed shells).

Never hand-roll it with `cp`:

- `cp` over a running executable truncates in place — on macOS that can
  fault the live process's text pages.
- An unsigned (ad-hoc) binary has a new cdhash every build, which silently
  voids the daemon's TCC/FDA grant: launchd can never present the consent
  prompt, so the daemon hangs or crash-loops
  (`.fathom/observations.md` O10, closed structurally 2026-08-08 — the
  grant now follows the `fathom-dev` identity, not the hash).

## Stopping the daemon: `fathom daemon stop`

`fathom daemon stop` is the off switch that works when nothing else does: it
writes `~/.fathom/daemon/disabled.json`, kills the managed pid (SIGTERM,
10 s, SIGKILL) and proves the port closed. Every launchd relaunch then exits
0 in under 50 ms before touching config, store or port. `fathom daemon
start` undoes it (`status`, doctor and the session primer name it as the
remedy while the switch is set; `task deploy` installs but does not start). With no binary at hand, `touch
~/.fathom/daemon/disabled.json` and kill the process. Under a schema-1 plist
launchd keeps relaunching every 10 s (harmlessly); `fathom daemon install
--reconcile` (after `task deploy`) and a reload from your terminal make
exit 0 stay down. `docs/usage.md` §daemon has the full contract.

## Daemon memory footprint (O108)

The always-on daemon grew ~0.9 GB/hour in one incident (`.fathom/observations.md`
O108, `docs/plans/daemon-footprint.md`), reaching 12 GB (11 GB compressed) over
9.4h and killing the pre-push gate's low-memory watchdog twice. Watch it before
it's felt, not after:

```sh
curl -s localhost:7575/metrics | grep -E 'heap_(inuse|sys)_bytes|goroutines|graph_memo|open_stores|typed_pass|memory_limit|resident_memory|phys_footprint'
fathom doctor   # daemon_footprint row: WARN above 2 GiB heap in-use, names the memory limit too
```

**Footprint vs heap — read the right gauge (DG-16, diagnostics sweep).**
`/metrics` carries two DIFFERENT kinds of memory number; conflating them
misreads the daemon's health:

- `go_memstats_heap_sys_bytes`/`go_memstats_heap_inuse_bytes` are the Go
  RUNTIME's own view — `heap_sys` is a high-water mark that never
  decreases for the process's life (it is the OS memory the Go allocator
  has ever reserved, not what is live now); the scavenger (below) is what
  brings the OS-visible number back down after a peak.
- `process_resident_memory_bytes` (darwin: `proc_pid_rusage`'s
  `ri_resident_size`; linux: `/proc/self/status`'s `VmRSS`) is the OS's own
  LIVE reading — the same figure `top`'s RES column shows right now.
  Darwin also exports `process_phys_footprint_bytes` (`ri_phys_footprint`)
  — the figure Activity Monitor's own "Memory" column reads, which
  additionally accounts for compressed pages `ri_resident_size` does not.
  Neither line is emitted, ever, on a platform/build without a proven live
  reading (a fabricated zero would be worse than an absent line).
- `process_max_resident_memory_bytes` (darwin and linux, `getrusage`'s
  `ru_maxrss`, normalized to bytes) is an **honestly-named peak** —
  it is `heap_sys`'s OS-level sibling: also a high-water mark that never
  decreases, useful ALONGSIDE the two live gauges above to see the same
  peak-vs-live divergence O108 was about, without cross-referencing the Go
  runtime's own accounting. **Never read this line as "current usage" —
  that was the exact naming mistake O108's own instrumentation had to
  correct (getrusage's Maxrss is a peak, not a current value).**

Read the daemon's OS-visible footprint with `top -pid <pid> -stats mem,cmprs`
(or `/metrics`' `go_memstats_heap_sys_bytes`), never `ps`'s RSS column — `ps`
counts `MADV_FREE`'d pages as resident until the kernel actually reclaims
them, so it can read hundreds of MB high for minutes after a scavenge pass
has already returned that memory to the OS (2026-09-23 production reading,
O108).

A live heap snapshot when a number looks wrong:

```sh
curl -s localhost:7575/debug/pprof/heap -o heap.pb.gz && go tool pprof -top -sample_index=inuse_space heap.pb.gz
```

Two snapshots minutes apart, `-base`-diffed, show what's actually growing
rather than what's merely large:

```sh
curl -s localhost:7575/debug/pprof/heap -o heap-a.pb.gz   # wait, then:
curl -s localhost:7575/debug/pprof/heap -o heap-b.pb.gz
go tool pprof -top -sample_index=inuse_space -base heap-a.pb.gz heap-b.pb.gz
```

Three independent bounds now target O108's growth pattern — a 2026-09-23
production reading reframed it as idle heap left behind by PEAKS (typed-edge
passes over large services), reclaimed lazily by darwin, not a live leak.
The daemon returns freed heap to the OS every 15 minutes by default
(`debug.FreeOSMemory`, `FATHOM_SCAVENGE_INTERVAL` to tune, `FATHOM_SCAVENGE=off`
to disable) rather than waiting on Go's own lazy background scavenger — bounds
how long any one peak (a large graph build, a burst of concurrent cache
misses across many parallel agent sessions) can inflate `go_memstats_heap_sys_bytes`.
A process-wide semaphore serializes the typed-edge pass so two large services
(an `app-kit`-sized one included) can never type-check concurrently — bounds
how many peaks can stack at once. `[daemon] memory_limit` (default `"3GiB"`,
`docs/configuration.md`) applies a soft `runtime.SetMemoryLimit` ceiling —
bounds how large any one peak's heap is allowed to grow in the first place.
If footprint is still climbing after a restart with all three live, re-read
O108 before assuming the mitigations cover it.

## Fleet wiring: `fleet report`, `reconcile`, receipts, `undo` (law 15)

`fathom fleet report` writes the wiring/freshness report across every
indexed repo (`--json` to stdout, else `~/.fathom/fleet`). `fathom fleet
reconcile [--repo R]` is a dry-run — hooks vs the current template, stale
binary paths → the shim, stanza drift — and `--apply` writes the safe
subset. Since SH-7/SH-8 every write goes through `internal/wiring`: one
plan → apply → one receipt per repo (`.fathom/receipts.jsonl`, gitignored);
`fathom fleet receipts` lists them and `fathom fleet undo [--last |
<receipt-id>]` reverses one. A tracked stanza region (`<!-- fathom:begin
v3 sha256:… -->`) a human edited is **blocked** and named — the file stays
byte-identical, the other files still apply — while a v1 region (`<!--
fathom:begin -->`, no digest) is adopted and upgraded on the first write
(dry-run reports it as `up to date (will be re-marked with a digest on
apply; no prose changes)`, not stale). The observatory's wiring page
(`/observatory/wiring/`) drives the same plan (`preview` → `apply` with the
returned `plan_digest`, 409 if the repo changed → `undo` with the
`receipt_digest`); its mutating endpoints accept loopback, same-origin
requests for registered service roots only.

**A service whose recorded root vanished (DG-18, diagnostics sweep).**
`list_services` used to re-walk every registered service's filesystem on
EVERY call (`fleetAdapter.UnsupportedLanguagesFor` → `client.Sources` →
`connector.Discover`) — measured live as the fleet's single most expensive
MCP call (~31k calls, p95 4.6s, max 182s), almost entirely spent re-lstating
a corpus root that no longer existed (a cleaned-up scratch clone, most
often). The result is now memoized per (service, store watermark) — computed
once per ingest, never re-walked to answer a read — and a vanished root is
reported structurally instead of as a live-re-walk error: `list_services`
returns `"root_missing": true` plus a `"remedy"` field on that service's
entry, naming the cleanup command:

```sh
fathom fleet   # or the remove_service MCP tool directly
```

This never runs automatically — deregistering a service record is an
owner decision (a stale `demo` service and similar scratch-rooted entries
are a known, deliberately-deferred cleanup item; see
`docs/plans/diagnostics-sweep.md`'s Owner-decision section) — `list_services`
only makes the absence cheap to detect and visible, never acted on.

## Rebuilding the fleet index

If the index is stale despite auto-refresh, has duplicate/path-like
services, or is bloated (originally ingested without `.fathomignore`),
rebuild it cleanly with `scripts/reingest-fleet.sh` (run from a real
terminal — it stops/starts the daemon). It re-ingests each fleet repo with a
clean `--service`/`--path`, recording the `ServiceMeta` path+commit that
auto-refresh, `staleness_report`, and the `ingest` tool all depend on.

## Dogfood discipline

We are fathom's first consumer — keep it sharp for the rest of the fleet.
**Log what you observe**: recall gaps, contract mismatches, latency,
pleasant surprises → `docs/dogfood-observations.md`, with a reproduction
(probe a known-good case beside the failing one and root-cause the
difference). When you fix fathom, record it in `CHANGELOG.md` and close the
observation.
