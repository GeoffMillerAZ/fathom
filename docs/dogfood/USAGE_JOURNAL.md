# Sextant Usage Journal

> **Historical (June to July 2026).** The maintainer's session journal from
> before the project was renamed fathom (it was called sextant until
> 2026-07-07). Kept for the asks it records, which early CHANGELOG entries
> cite as "USAGE_JOURNAL.md ask #N". Moved here from the repository root.

Running, dated feedback from Claude Code sessions that actually use sextant — what worked, what broke,
and concrete suggestions — so sextant can improve from real usage rather than guesses. Newest entries
on top. Each entry: date · session context · what worked · friction/bugs · suggestions · carried-forward.

---

## 2026-06-16 (app-kit · component pipeline + Assistant build) — consumer: Opus 4.8 / 1M, session in `app-kit/`

**Context:** Built `internal/twplus` (TailwindPlus dump → cgo-free SQLite corpus) + `cmd/appkit-mcp`
(the kit's MCP Assistant) — ~2,800 LOC, all **uncommitted** at ingest time. First app-kit session to
use sextant after the `serve-http` daemon switch + an explicit reconnect.

### Worked well
- **Working-tree ingest picks up UNCOMMITTED code — accurately.** After `ingest ../app-kit`,
  `find_symbol` resolved brand-new, never-committed symbols with exact location + signature:
  `PreviewBuilder` → `internal/twplus/index/previews.go:36` (GoType); `Index.SearchComponents` →
  `query.go:96` with full `func SearchComponents(query string, f Filters, limit int) (...)`, tier
  `verified`. The core promise delivers: I could navigate code that exists only in my working tree.
- **HTTP daemon connected cleanly for yet another concurrent session** — further corroborates the
  `serve-http` multi-session fix.

### Friction / bugs (corroborates the net-new `ingest` naming bug, with app-kit specifics)
1. **`ingest` can't refresh the registered service in place — it forks a duplicate.**
   `ingest({"service":"app-kit"})` → `-32603 ... discover "app-kit": lstat app-kit: no such file or
   directory` (resolves `service` as a path from the daemon CWD). Workaround
   `ingest({"service":"../app-kit"})` succeeded but created a SECOND service `../app-kit` (632 symbols)
   distinct from the registered `app-kit` (954). **The consequence that bites:**
   `find_symbol(service="app-kit", name="PreviewBuilder")` → `[]` — every tool/session querying the
   canonical name `app-kit` is blind to the refresh; the new code lives only under `../app-kit`. So an
   **MCP-only client literally cannot refresh-in-place the service it queries.** Same root as the prior
   entry's #1/#2 (identity == raw path string).
2. **The higher symbol count is the buggy one.** Freshly single-path-ingested `../app-kit` = **632**
   vs the older double-spelled `app-kit` = **954** — consistent with `app-kit` being double-counted.
   Dedupe will *lower* counts, not raise them; don't treat the bigger number as healthier.

### Suggestion (reinforces prior asks — this is the #1 daily-loop blocker)
- Decouple logical `service` from `path` in the MCP `ingest` tool (mirror the CLI
  `--service NAME --path DIR`); key identity on canonical absolute realpath; add
  `rename_service` / `remove_service` so the `app-kit`↔`../app-kit` (and `geoffmiller-cloud`↔
  `../geoffmiller-cloud`) splits can be healed over MCP without CLI/daemon-lock gymnastics. Until
  then, **MCP-only sessions cannot cleanly re-index an existing service**, which undercuts the
  "ingest at session start / after each commit" guidance — the refresh silently lands in a fork.

### Carried forward
- Collapse `app-kit` + `../app-kit` into one canonical `app-kit` (once name/path is decoupled, or via
  CLI under `launchctl unload`), then confirm `find_symbol(service="app-kit", …)` sees
  `internal/twplus` + `cmd/appkit-mcp`.

---

## 2026-06-16 (later) — `geoffmiller-cloud` fleet bring-up (consumer: Opus 4.8 / 1M, session in `geoff/`)

**Context:** A *different* concurrent session from the `planning/` entry below — standing up a new
Go/CUE monorepo `geoffmiller-cloud` (public résumé site + private career studio) on
`app-kit` / `agentic-kit` / `sextant`. Caught the `serve-mcp` → `serve-http` daemon switch mid-session.

### Worked well
- **The HTTP daemon connected cleanly for a *second* concurrent session** — corroborates that
  `serve-http` fixes the multi-session lock pain documented below. `claude mcp list` →
  `:7575 ✔ Connected`; `list_services` worked over HTTP.
- **`repo_map agentic-kit` (budget 1200): high signal** — ranked the orienting symbols
  (`Harness.Run`, `Server.Serve`, endpoint-guard PoW/budget/injection tests, llmclient
  `consumeSSE`); replaced a large Explore sweep.
- **`.sextantignore` honored on first ingest** of geoffmiller-cloud (only its 22 real source files;
  `design-reference/`, `bin/`, `workspace/` excluded). `staleness_report` listed exact sources.

### Friction / bugs (net-new, plus corroboration)
1. **Adding a NEW repo over MCP mis-names the service (net-new).**
   `ingest({"service":"geoffmiller-cloud"})` →
   `-32603: ... discover "geoffmiller-cloud": lstat geoffmiller-cloud: no such file or directory`
   — the tool resolves `service` as a path relative to the **daemon CWD**, not a logical name.
   Workaround `ingest({"service":"../geoffmiller-cloud"})` worked but **named the service
   `../geoffmiller-cloud`** (name == path argument). There's no MCP analog of the CLI's
   `--service NAME --path DIR`, so the shared store now holds a mis-named service that can't be
   renamed without CLI access (daemon lock). Same family as the duplicate-path bug below (#2):
   service identity is derived from the raw path string. **Ask:** decouple `service` (logical name)
   from `path` in the `ingest` tool; add `add_service` / `rename_service` / `remove_service`;
   canonicalize paths to absolute realpath and key identity on that.
2. **Corroborates the duplicate-ingest data bug below (it's not only `sextant`).** `repo_map
   agentic-kit` returned **every symbol exactly 2×**, consistent with agentic-kit being ingested
   under two path spellings. Canonical-realpath keying would fix the whole class.
3. **Language mis-attribution (net-new).** geoffmiller-cloud (only Markdown + CUE, zero JS) was
   reported as `languages:["javascript"]`, 300 symbols. CUE/Markdown content is being tagged
   JavaScript — worth auditing the extractor's language attribution for non-Go inputs.
4. **CLI help (minor):** `sextant --help` → `error: unknown subcommand "--help"`; only
   `sextant help` works. Map `--help` / `-h` to help.

### Carried forward
- On the next fresh `serve-http` re-ingest (per the entry below), **register `geoffmiller-cloud`
  with a clean logical name + canonical absolute path** to replace the `../geoffmiller-cloud`
  entry, and confirm the 2× agentic-kit duplicates collapse.

---

## 2026-06-16 — multi-session setup saga + first real queries

**Context:** Opus 4.8 (1M) session running in `planning/` (building the `platform-kit` monorepo + a
GovCloud/defense engagement). This stretch: get sextant working across the user's *many* concurrent
Claude Code sessions, then document it. Store: `sextant/workspace/db/fleet.duckdb`.

### Worked well
- **`query_symbols` (concern-scoped) is the standout.** Asked "serve-http MCP server command" with
  keyword hints and got relevant code (`Handler` type in `internal/mcp/server.go`, `postRPC` in
  `servehttp_test.go`) ranked **alongside the governing doc chunk** (`docs/usage.md` → "serve-http:
  listening on :7575"). Code + docs in one ranked list, exactly the pitch — and found without knowing
  an exact symbol name.
- **Staleness honesty.** Every query carried `reingest_recommended:true, fresh_sources:0`, last
  ingest 2026-06-13. The index was 3 days stale and *said so* — no silent wrong answers. Real trust win.
- **`list_services` / `server_stats`:** instant and accurate. `list_services` corrected my assumption —
  the fleet is **5 services** (agentic-kit 11,360 · throughline 10,824 · sextant 7,093 · app-kit 954 ·
  geoffmiller-cloud 300), not the 3 I'd guessed. Ground truth in one call.
- **Deferred tool schemas:** ~80 tools, near-zero context until used; loaded 4 on demand. Confirmed.

### Friction / bugs (the useful part)
1. **Multi-session lock contention is invisible from the surface — cost a long misdiagnosis.** The
   user runs many concurrent Claude Code sessions. Each registers stdio `serve-mcp`; DuckDB is
   single-writer, so only the first session gets the lock and every other session shows
   `claude mcp list` → `✘ Failed to connect` **with no reason**. I misdiagnosed it twice (assumed a
   schema-stale store, then a stale orphan process) and nearly told the user to `kill` a PID that
   turned out to belong to another *live* session. Only `sextant inspect` surfaced the truth: a DuckDB
   lock error naming the holding PID.
   - **Docs fix shipped this session:** added the shared `serve-http` + HTTP-registration + launchd
     path to `docs/claude-code-integration.md` (new "Multiple concurrent sessions" section, a §3
     callout, and a pitfalls item) and strengthened `docs/use-cases/getting-started.md`. Previously the
     integration guide described *only* per-session stdio, which silently breaks for multi-session users.
   - **Suggested product fixes:** (a) `serve-mcp` should **fail fast with an actionable message** when
     it can't get the lock — e.g. "store locked by sextant PID N; another session is likely running —
     use `serve-http` for multiple clients" — instead of an opaque MCP connect failure. (b) a
     `sextant doctor` that reports lock-holder, registration transport, server reachability, and
     recommends `serve-http` when it sees >1 stdio client.
2. **Service triple-ingested under 3 path spellings (data bug).** Every result and every staleness
   source appeared **3×** — as `../sextant/...`, absolute `/Volumes/.../sextant/...`, and bare
   `cmd/...`. The `sextant` service was ingested three separate times with different `--path` values,
   so the store holds 3 duplicate copies → triplicated query results and inflated symbol counts.
   **Suggest:** canonicalize `--path` to an absolute realpath on ingest and key service identity by
   that canonical root, so re-ingests from different working dirs collapse instead of duplicating.
3. **`find_symbol` is exact-match only.** `find_symbol(service=sextant, name="serveHTTP")` → `[]`
   because that isn't the exact symbol spelling. **Suggest:** case-insensitive / substring / fuzzy
   fallback, or auto-degrade to a `query_symbols` ranking with "did you mean" — the tool currently
   punishes not knowing the exact identifier, which is exactly when you reach for it.
4. **Staleness payload is very verbose.** An 8-node `query_symbols` returned ~117 lines of per-source
   staleness metadata (39 sources × the 3 path dupes) that dwarfed the actual answer. **Suggest:** a
   compact summary by default (`{total:39, fresh:0, oldest:"3d", reingest_recommended:true}`) with full
   per-source detail behind a flag or `staleness_report`. (Fixing #2 also cuts this ~3×.)

### Carried forward
- **Re-ingest the fleet fresh** on the new `serve-http` server — clears the 3-day staleness AND the
  triple-path duplicates — then add **`platform-kit`** as a service once it's created. Must go through
  the `ingest` MCP tool, or `launchctl unload` → CLI `ingest` → `launchctl load`, because the daemon
  holds the single-writer lock.
- Re-run the `serve-http` query benchmark after a fresh ingest to confirm dedupe + that this session's
  doc edits are now indexed (they were absent here — index predated them).

---
