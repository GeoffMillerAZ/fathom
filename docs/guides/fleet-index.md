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
MCP**. `/rpc/agent` is the curated 17-tool agent surface
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

## Working against the index

1. **Index first.** Resolve where / what / who-calls via fathom
   (`find_symbol`, `query_symbols`, `references`, `find_callers`,
   `symbol_context`, `outline`, `repo_map`, `pack_context`, `obligations`,
   `staleness_report`) for the questions only an index answers — where,
   who-calls, what breaks (RC-23, docs/vision/28-receipts.md §3.5): what
   governs this repository is `obligations` called with no path (the root
   conventions, not a file); is this still true / what was known is
   `staleness_report`; a literal string, a config value, a log line, or a
   document-versus-document contradiction is still grep's (vision 27 §8) —
   `references` replaces fleet-wide grep for a symbol,
   `symbol_context` gives def+callers+callees+tests in one call (plus an
   inline definition snippet), `read_source` reads an exact line range from
   the indexed corpus, `obligations` surfaces governing rules for a file
   before editing it. Read files to *edit* them, not to discover them. The
   index is fleet-wide, so cross-repo questions are one query. (These are
   the core of the 17-tool `agent` surface — see `docs/mcp-tools.md`;
   `memory_search`, the newest member, is **experimental**.)
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
