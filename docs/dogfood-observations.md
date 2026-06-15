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

_(none — see Resolved below; new findings go here.)_

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
