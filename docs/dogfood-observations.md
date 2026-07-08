# Dogfood observations — performance & recall

A living log of how sextant's MCP tools behave **in real fleet sessions** (throughline,
agentic-kit, app-kit, sextant). Kept current by whoever is dogfooding the index: every
session that hits a recall gap, a latency cliff, a contract mismatch, or a pleasant
surprise records it here while the evidence is fresh. This is the feeder queue for
sextant's own roadmap — observations graduate into `tmp/TRACKING.md` / `ADR.md` /
`CHANGELOG.md` as fixes land.

**Discipline (truth-seeking, same as the product):** never log a "tool is broken" claim
without a reproduction. Probe a known-good case alongside the failing one, root-cause the
difference, and write down both. The first instinct ("find_symbol can't find methods")
was wrong this round — the probe proved the running server was stale, not the code. An
unverified complaint is noise; a root-caused observation is a fix.

Entry shape: **symptom → evidence (verbatim probe) → root cause → fix → status → impact.**

---

## Open

### O6 — test_reaches misses tests that exercise a symbol only through method calls

- **Symptom:** `fathom verify` failed `test_reaches` on two genuinely-tested symbols
  while passing a third, on the same store, same run (2026-07-07, first gated plan).
- **Evidence (verbatim):** plan contracts `{symbol: ParseMarkdownPlan}` → ✓
  `test_reaches`; `{symbol: Discover}` → ✗ "no Test* statically reaches it";
  `{symbol: manifestWatermark}` → ✗ — yet `TestLocalConnector_Discover_PaymentService`
  calls `lc.Discover(...)` and `TestLocalConnector_Watermark_ManifestStableAcrossMove`
  reaches `manifestWatermark` via `lc.Watermark(...)`.
- **Root cause of the difference:** `ParseMarkdownPlan` is reached by a *plain function
  call* from its test; the failing two are reached only through *method* calls
  (`lc.Discover`), whose call edges the Go extractor doesn't attribute to the method
  symbol, so `callgraph.TestImpact` finds no Test* path.
- **Workaround:** `skip_test: true` on method-reached contracts (symbol presence still
  verified). **Fix direction:** method-call edge attribution in `goextract` (receiver
  type resolution), then remove the workaround from `docs/plans/`.
- **Status:** open. **Impact:** verify gates under-prove any contract whose symbol is
  only test-reached through methods — a false-negative class that pushes plan authors
  toward `skip_test`, weakening the gate.

---

## Resolved

### O1 — `outline` ignored its "repo-relative" contract; needed an absolute path — FIXED
- **Tool:** `outline`. **Severity:** high (core discovery tool silently returned nothing).
- **Symptom:** `outline{service:"throughline", file:"app/participants/participant.go"}`
  → `{"symbols":[]}`, as if the file weren't indexed.
- **Evidence:** `find_symbol{name:"Participant"}` returned the symbol with
  `file_path:"/Volumes/.../app/participants/participant.go", tier:"verified"` (so the file
  **was** indexed); `outline` with the **absolute** path returned the full symbol set.
- **Root cause:** `symbolnav.Outline` matched `n.FilePath == file` exactly. The store keys
  files by absolute path; any repo-relative `file` arg missed — contradicting the tool's
  documented "Repo-relative file path".
- **Fix:** `fileMatches(stored, query)` resolves a relative query by path-segment suffix
  (leading `/` so `participant.go` can't match `myparticipant.go`); absolute queries still
  match exactly. Regression test `TestOutline_relativePathMatchesAbsoluteStore`. Verified
  over HTTP: relative `outline` returns 10 symbols.
- **Status:** fixed (this commit). **Impact:** index-first `outline` works on the natural
  (relative) call agents actually make.

### O2 — `find_symbol` appeared blind to methods — NOT A CODE BUG (stale server) → O3
- **Symptom:** `find_symbol{name:"ListAssertions"}` / `{name:"HasAlias"}` →
  `{"symbols":[]}`, while `find_symbol{name:"Participant.HasAlias"}` (qualified) returned
  the method. Looked like bare-name method lookup was broken.
- **Evidence that it is NOT the code:** current `symbolMatches` (engine.go) already has
  `strings.HasSuffix(o.Symbol, "."+name)`; a freshly-built binary serving the same data
  over HTTP returned `find_symbol{name:"ListAssertions"}` →
  `OutputAssertionReader.ListAssertions`, `Projector.ListAssertions`, … correctly. The
  fix commit `7ac8011` (2026-06-12 14:45) predates even the stale binary.
- **Root cause:** the **running MCP server was stale** — see O3. Bare-name method lookup
  works in current code; the deployed process was serving older logic/data.
- **Status:** resolved by O3 (rebuild + restart). No code change to `symbolMatches`.

### O3 — running serve-mcp outlives commits; the binary was never rebuilt — FIXED (mechanism)
- **Tool:** the whole `serve-mcp` deployment. **Severity:** high — this is the systemic
  one. It made a correct code fix invisible to consumers for days (and produced O2's
  false alarm).
- **Symptom / evidence:**
  - `bin/sextant` mtime **Jun 12 18:18**; HEAD **Jun 15** with later commits it predates
    (incl. `923977b "Persist SymbolNode.Package through the store"`).
  - **Two** `serve-mcp` processes (started Jun 13, 2PM and 7PM) from the same stale
    binary, both holding `--store .../fleet.duckdb` — and DuckDB is **single-writer**, so
    duplicate writers contend for one lock.
  - The `post-commit` hook re-ingested **data** with `bin/sextant` but never **rebuilt the
    binary** — so query-engine fixes landed in source and `bin/sextant` stayed stale.
- **Fix (this commit):**
  1. `post-commit` now rebuilds `bin/sextant` whenever the commit touched `*.go`, before
     re-ingesting — the on-disk binary tracks HEAD.
  2. `serve-mcp` logs its build identity at startup (`BuildRevision()` via embedded
     `vcs.revision`, no ldflags); `sextant version` reports the same. Compare against
     `git rev-parse HEAD` to detect a stale server in one line.
- **Operational residue (manual, by design):** a long-running `serve-mcp` process cannot
  hot-swap its own binary — it must be **restarted** to load a rebuilt one (Claude Code
  restarts MCP servers on session/MCP-config reload). And **duplicate** server processes
  should be reaped; only one can hold the write lock.
- **Status:** mechanism fixed; binary rebuilt from HEAD. **Impact:** future fixes reach
  consumers; staleness is now self-evident instead of silent.
- **Update 2026-06-16 — residual SUPERSEDED by 0.2.0.** The deployment moved to a single
  always-on HTTP daemon (launchd `serve-http` at `localhost:7575/rpc`, one shared store),
  so there are no per-session servers and no lock contention. The two manual residuals above
  are now closed by the tool: `serve-mcp --auto-refresh` (default on) re-ingests a service on
  git-HEAD drift before answering — killing the *data*-staleness loop — and a second
  `serve-mcp` on the same store now refuses with actionable guidance instead of silently
  contending for the lock (no more "reap duplicates" chore). **Only *binary* staleness
  remains a manual step**: a code change still needs the daemon's binary rebuilt and the
  daemon restarted (it can't hot-swap itself); `sextant version` vs `git rev-parse HEAD` is
  still the one-line check. See CLAUDE.md "Dogfood: the fleet index" for the daemon model.

### O4 — the shared daemon's index was "frozen": every refresh clobbered the service path — FIXED
- **Tool:** `ingest` / `staleness_report` / auto-refresh on the `serve-http` daemon.
  **Severity:** high — the always-on index silently stopped tracking new commits.
- **Symptom:** after a clean fleet re-ingest, `staleness_report{service:"sextant"}` returned
  **empty** `indexed_commit`/`current_commit`, and `find_symbol` missed newly-added symbols
  (e.g. `ValidServiceID`) — yet `query_symbols`/sources still worked. The data was there; the
  *freshness metadata* was gone.
- **Evidence (verbatim probe):** `scripts/reingest-fleet.sh` set `indexed_commit=79c0cad`;
  one `find_symbol` query later (which fires auto-refresh on HEAD-drift) it was **empty
  again**, while symbols still resolved. So a *refresh*, not a query, was wiping it.
- **Root cause:** `client.IngestService(service)` re-ingested with
  `ServiceRef{ID: service, Path: service}` — walking the bare service ID as a directory
  (e.g. `./sextant`, nonexistent), ingesting nothing, and `SetServiceMeta` then recorded an
  empty path + commit. Auto-refresh, `staleness_report`, and the `ingest` tool all key off
  `ServiceMeta.path`, so every refresh froze the index. (Sibling `Sources` already resolved
  the path from `service_meta`; `IngestService` just never did.)
- **Fix (`4bbaf56`):** `IngestService` now uses the stored `meta.Path` (mirrors `Sources`),
  falling back to the ID only for path-less connectors. Added `serve-http --auto-refresh`
  (the daemon runs serve-http, which never had it) and `scripts/reingest-fleet.sh` to rebuild
  cleanly (path metadata + no malformed/bloated services — sextant was 7093 symbols from
  indexing `ref/`/`tmp/`; correctly ~2923 with `.sextantignore`). Regression:
  `TestIngestService_UsesAndPreservesStoredPath`.
- **Status:** fixed; the freshness loop the 0.2.0 daemon promised now actually holds.

### O5 — one Nix/devbox symlink aborted a whole-repo ingest — FIXED
- **Tool:** `ingest` (LocalConnector). **Severity:** high — a single stray symlink made
  an entire repo un-indexable, so the repo silently never joined the fleet.
- **Symptom:** ingesting `resume_as_code` failed hard:
  `error: ingest failed: fusion: fetch: read .../.devbox/nix/profile/default-9-link: is a directory`.
  No symbols stored; the repo could not be added as a service at all.
- **Evidence (verbatim probe):** the failing path is a **symlink to a directory** (a Nix
  profile link devbox creates). `WalkDir` does not follow symlinks, so it surfaced as a
  non-dir entry and was emitted as a `SourceRef`; `Fetch`'s `os.ReadFile` on it returned
  "is a directory". The fusion fetch pass treats the **first** fetch error as fatal
  (deterministic first-error abort), so one link killed the ingest of 394 real files.
- **Root cause:** `LocalConnector.Discover` emitted every non-dir `WalkDir` entry —
  including symlinks/devices/sockets/pipes — none of which `os.ReadFile` can read as a
  source file.
- **Fix (this commit):** `Discover` now gates on `d.Type().IsRegular()`, skipping all
  non-regular entries. `resume_as_code` went from a hard abort to **1,160 symbols, 0
  skipped**, with the Nix store behind the link correctly excluded (not indexed as noise).
  Regression: `TestLocalConnector_Discover_SkipsSymlinkToDir`. `resume_as_code` added to
  `scripts/reingest-fleet.sh`.
- **Status:** fixed. **Impact:** any real-world repo with Nix/devbox, vendored symlinks,
  or build-tool profile links now ingests instead of aborting.

---

## Confirmed-good (keep regression coverage)

- **`ingest` hash-gated refresh** — `ingest{service:"throughline"}` at session start
  returned `{"ok":true}` fast with no rework when clean, as documented. Single-writer lock
  behaves.
- **`find_symbol` on types/consts/free funcs and qualified methods** — exact-name lookup
  returns multi-hit results with `file_path` + `tier` (`Participant` → 2 hits;
  `Participant.HasAlias` → the method). Bare-name method lookup works in current code (O2).

---

## How to add an entry

1. Reproduce. Capture the verbatim probe + result for **both** the failing and a
   known-good case. If a tool looks broken, **also test a freshly-built binary** before
   blaming the code — O2 was a stale server, not a bug.
2. Root-cause the difference (don't stop at the symptom).
3. File under **Open** with a concrete fix. Move to **Resolved** (link the commit) when
   fixed. Promote anything systemic into `CHANGELOG.md` / `tmp/TRACKING.md`.
