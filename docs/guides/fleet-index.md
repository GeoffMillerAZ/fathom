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
   edit-hook (`docs/design/coding-agent-excellence.md` §5.2).
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
  and you can force it via the `ingest` MCP tool.
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
stable `fathom-dev` codesigning identity when present, kickstarts the
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
