# Claude Code integration guide

> **Superseded for wiring (2026-07-16):** the canonical, current guide is
> [use-cases/coding-agents.md](use-cases/coding-agents.md) — `fathom init`,
> the three hooks, the `/rpc/agent` surface, gates, and the cost story.
> This document predates the obligations layer, the verify gate, and the
> hook stack; its measured assessment content remains historically accurate.

How to wire fathom into Claude Code so coding sessions (and every subagent
they spawn) query a pre-built index instead of re-reading the repo. This is
the playbook proven on a four-repo fleet on 2026-06-12; measured results in
[assessment-claude-code-integration.md](assessment-claude-code-integration.md)
— headline: discovery agents eliminated entirely, wave wall-clock roughly
halved, comparable workers dropped from ~148 tool calls to ~32.

## 1. Build and index

```sh
cd fathom
CGO_ENABLED=1 go build -o bin/fathom ./cmd/fathom   # macOS: MACOSX_DEPLOYMENT_TARGET=14.0 silences linker warnings

# One store for the whole fleet — one service per repo:
for repo in my-app my-lib; do
  bin/fathom ingest --service $repo --path ../$repo --store workspace/db/fleet.duckdb
done
```

Full ingest is seconds per repo; re-ingest is hash-gated (~0.2 s when clean).

## 2. `.fathomignore` — do this BEFORE first ingest

Gitignore-style file at each repo root. Two purposes:

1. **Noise**: exclude mirrored third-party source, scratch dirs, build
   output, generated code (index the sources, not the codegen — e.g.
   `*_templ.go` when `.templ` files are indexed).
2. **Privacy — non-negotiable**: exclude any directory holding user data,
   case files, secrets, or PII. The index is readable by every connected
   MCP client and sits OUTSIDE any application egress/DLP gate.

```gitignore
ref/
tmp/
workspace/
bin/
data/          # user/case data — never index
*_templ.go
```

Skipping this cost us 15k junk symbols on day one (own repo: 2,427 files
indexed → 222 after).

## 3. Register the MCP server

> **Run more than one Claude Code session at a time?** Skip the per-session stdio registration below
> and use the shared HTTP server instead (see **Multiple concurrent sessions** further down). Each
> stdio `serve-mcp` holds the single-writer DuckDB lock, so with concurrent sessions only the first
> connects and the rest report `✘ Failed to connect`.

User scope makes it available in every repo and to every subagent:

```sh
claude mcp add --scope user fathom -- \
  /abs/path/fathom/bin/fathom serve-mcp --store /abs/path/fathom/workspace/db/fleet.duckdb
claude mcp list   # expect: fathom … ✔ Connected
```

Beware shell line-wrapping: a wrapped command registers `--store` without
its value and the server silently fails to start. Verify with `claude mcp
list` every time. Sessions pick the server up at start — a rebuild of the
binary or store needs a session restart to take effect.

## 4. The operating contract (CLAUDE.md) — where the value actually comes from

The index pays off only if agents are *told* to use it. Add a section like
this to each repo's CLAUDE.md (adapt the service name):

```markdown
## Fathom fleet index (code intelligence — use it first)

1. **Index before exploration.** Resolve "where/what/who-calls" questions
   via fathom tools (`find_symbol`, `query_symbols`, `find_callers`,
   `outline`, `repo_map`, `pack_context`) before any Grep/Read sweep.
   Read files to edit them, not to discover them.
   **Never re-discover what the architect already discovered:** worker
   prompts carry a pre-packed context section (exact signatures, file
   paths, verbatim contract types). Workers trust it and verify only what
   they change; if it conflicts with the code, STOP and report — never
   silently drift.
2. **Refresh:** call the MCP `ingest` tool ({"service":"<repo>"}) at
   session start and after each commit — hash-gated, ~0.2 s when clean.
3. **Honesty:** after heavy uncommitted edits, run `staleness_report`
   before trusting results.
4. **Privacy:** keep sensitive dirs in `.fathomignore`.
```

Name the ~6 tools that matter. Don't list all ~80 — modern Claude Code
defers MCP tool schemas (near-zero context cost when unused); the curation
that matters is in the prompt, not the server.

## 5. Refresh discipline (single-writer constraint)

DuckDB allows ONE writing process. While any Claude Code session is open,
its MCP server holds the store lock and **CLI ingest fails** — measured,
not theoretical. Therefore:

- **Primary**: refresh through the live server's `ingest` MCP tool
  (session start + after each commit). Encode it in CLAUDE.md so the agent
  does it unprompted.
- **Belt-and-braces**: a best-effort post-commit hook for commits made with
  no session running (lefthook example):

```yaml
post-commit:
  commands:
    fathom-refresh:
      run: |
        [ -x ../fathom/bin/fathom ] || exit 0
        ../fathom/bin/fathom ingest --service my-app --path . \
          --store ../fathom/workspace/db/fleet.duckdb >/dev/null 2>&1 || true
```

No daemon/watcher needed: hash-gating makes on-demand refresh effectively
free, and `staleness_report` makes drift visible instead of silent.

## Multiple concurrent sessions: the shared HTTP server

The stdio `serve-mcp` registration in §3 starts **one server per Claude Code session**, and DuckDB is
**single-writer**. Run two sessions at once and only the first to open the store gets the lock; every
other session — and every `claude mcp list` health check — reports **`✘ Failed to connect`**. Killing
the lock-holder doesn't help: it's a *live* server owned by one of your other sessions, so you'd just
break that one. Restarts don't help either — the next session loses the same race. The per-session
model simply doesn't fit a multi-session workflow.

The fix is built in: run **one** `serve-http` daemon (the documented shared fleet server) and register
it over **HTTP**. Every session becomes an HTTP client of that single lock-holder — unlimited
concurrency, one shared index, no contention.

**1 — Run one always-on server.** A launchd agent keeps it up across logins/reboots and, via
`KeepAlive`, grabs the store lock the instant it's free:

```xml
<!-- ~/Library/LaunchAgents/<reverse-dns>.fathom-http.plist  — adapt the absolute paths -->
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>org.example.fathom-http</string>
  <key>ProgramArguments</key><array>
    <string>/abs/path/fathom/bin/fathom</string>
    <string>serve-http</string>
    <string>--store</string><string>/abs/path/fathom/workspace/db/fleet.duckdb</string>
    <string>--addr</string><string>127.0.0.1:7575</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <!-- anchors the relative --call-log default (workspace/logs/mcp-calls.jsonl);
       without it launchd starts the daemon at / and call logging is disabled -->
  <key>WorkingDirectory</key><string>/abs/path/fathom</string>
  <key>StandardOutPath</key><string>/tmp/fathom-http.log</string>
  <key>StandardErrorPath</key><string>/tmp/fathom-http.err</string>
</dict></plist>
```

```sh
cp <reverse-dns>.fathom-http.plist ~/Library/LaunchAgents/
launchctl load ~/Library/LaunchAgents/<reverse-dns>.fathom-http.plist
tail -f /tmp/fathom-http.log     # expect: serve-http: listening on 127.0.0.1:7575
```

No launchd (Linux/CI)? `nohup fathom serve-http --store … --addr :7575 &` works, but it dies with the
shell — use a systemd unit or supervisor for persistence.

**2 — Register over HTTP at user scope.** Remove the stdio entry first, or `add` fails with
"already exists":

```sh
claude mcp remove fathom -s user
claude mcp add --transport http --scope user fathom http://localhost:7575/rpc
claude mcp get fathom            # expect:  Type: http  ·  Status: ✔ Connected
```

If `http` won't connect, register the SSE endpoint instead: `--transport sse … http://localhost:7575/events`.

**3 — Free the lock once.** `serve-http` can't open the store while any stdio `serve-mcp` still holds
it. Quit your running sessions once so their stdio servers exit; `KeepAlive` then hands the lock to
`serve-http` for good. Reopen sessions — each picks up the HTTP registration at start and shares the
one server.

**Refreshing changes.** The daemon is the single writer, so a **CLI `ingest` will fail** with a lock
error. Refresh through the **`ingest` MCP tool** (`{"service":"<repo>"}`) from any session, or
`POST /reload`; because the store is shared, one refresh updates the index for **every** session. To
add a new repo as a service while the daemon runs, ingest via the MCP tool, or `launchctl unload` →
CLI `ingest` → `launchctl load`.

The **session operating contract (§4) is unchanged** — index-first, the same handful of tools, refresh
at session start and after commits. Only the transport (HTTP, not stdio) and the refresh path (MCP
tool / `POST /reload`, never a CLI `ingest` while the daemon runs) differ.

### Path-scoped surfaces: slim endpoints for schema-loading clients

The daemon serves the same store on several endpoints, each an immutable **surface** of the tool
registry (see [mcp-tools.md](mcp-tools.md)):

- `POST /rpc` — the full surface, every tool. **Register this for Claude Code**: it defers MCP tool
  schemas (only names are injected until a tool is used), so the full registry costs ~600 tokens per
  session and every tool stays one on-demand load away.
- `POST /rpc/companion` — the curated 15-tool coding-agent hot loop. Register this for MCP clients
  that load **all** tool schemas into context up front (Cursor, generic MCP hosts), or anywhere you
  want a hard-bounded surface: it cuts the schema payload ~80% and off-surface calls are rejected,
  not silently misrouted.
- `POST /rpc/{coding|investigation|shared|platform}` — group surfaces for job-specific sessions
  (change review, incident investigation, fleet health).

For a schema-loading client, point its MCP config at `http://localhost:7575/rpc/companion`
instead of `/rpc`. If you want the hard-bounded surface in Claude Code too, register it
**under its own name, alongside** the step-2 full-surface registration (re-using the name
`fathom` would fail with "already exists" — or silently replace the recommended `/rpc`
registration):

```sh
# optional, additive — a slim hard-gated surface next to the full one:
claude mcp add --transport http --scope user fathom-companion http://localhost:7575/rpc/companion
```

`set_profile` is rejected over HTTP (the daemon is shared; the profile is process-global) — the
error points here. Tool selection per surface is measurable from the daemon's call log
(`--call-log`, default `workspace/logs/mcp-calls.jsonl` resolved against the daemon working
directory — which is why the plist above sets `WorkingDirectory`).

## 6. The pre-packed brief pattern (biggest tokenomics lever)

The architect does recon ONCE via the index, then ships it verbatim inside
each worker prompt:

- exact function/type signatures workers will call or implement,
- file paths and line-anchored snippets for everything load-bearing,
- the inter-team contract as literal Go (or your language) declarations,
- the instruction: *trust this section; verify only what you change; if
  the code disagrees, stop and report the conflict — never adapt silently.*

Re-derivation is the most expensive failure mode (it's the whole pre-index
baseline). The stop-and-report clause matters: a worker that quietly
deviates from a pinned contract produces code that compiles and then
no-ops at runtime — we hit exactly this once before adding the rule.

## 7. Pitfalls checklist

- [ ] `.fathomignore` written before first ingest (privacy + noise).
- [ ] `claude mcp list` shows ✔ Connected after registration.
- [ ] Running more than one session at once? You're on the shared HTTP server (see **Multiple
      concurrent sessions**), not per-session stdio — otherwise all but one session fails on the
      single-writer lock and `claude mcp list` shows `✘ Failed to connect`.
- [ ] CLAUDE.md operating contract added to EVERY repo in the fleet.
- [ ] Session restart after rebuilding the fathom binary or recreating
      the store (the running server keeps its old file handle).
- [ ] Schema/extractor changes need a from-scratch re-ingest — the hash
      gate skips unchanged files, so new extractors never see them
      otherwise (`rm` the store and re-ingest; it's seconds).
- [ ] Don't trust mid-wave query results for files you just edited:
      refresh first or read the file.

## 8. What to expect (measured on a 4-repo fleet, ~12k symbols)

- Concern query answering "where is X and what governs it": one ~600-token
  response replacing a 30–80k-token Explore-agent sweep — including design
  docs and schemas ranked alongside the code they govern.
- Discovery-tier agents: eliminated (2 per wave → 0).
- Comparable implementation workers: ~148 tool calls → ~32 at similar
  token size (spend moves from searching to building).
- Wave wall-clock: roughly halved.
- Worker token totals per wave: only modestly lower — read-to-edit and
  code-writing are irreducible; set expectations accordingly.

Full numbers, method, and caveats:
[assessment-claude-code-integration.md](assessment-claude-code-integration.md).
