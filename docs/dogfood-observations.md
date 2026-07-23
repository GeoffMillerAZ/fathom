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

### O12 — context-bench B1-T09: the fathom arm lost 6.6× to grep on its own home question (cross-repo callers) — three compounding product gaps

- **Symptom:** in the first context-bench run (docs/plans/falsifiability.md
  B1-3, run-20260723T045809Z), task B1-T09 — "which agentic-kit heartbeat
  functions does cmd/fathom/instrument.go call?" — cost the fathom arm
  504,785 tokens / 20 turns / 56.5s where the grep arm took 76,294 tokens /
  2 turns / 11.1s. The fathom arm eventually PASSED, but only after burning
  turns on three distinct product gaps. This was the bench's worst data
  point and a driver of the ×10 retraction.
- **Evidence (live reproductions, daemon @ fleet store, 2026-07-23):**
  - **(a) Cross-repo call edges missing.** `symbol_context(fathom,
    cmdInstrument)` returns `callees: [fathom::main.beatLogPath,
    fathom::main.newFlagSet, fathom::main.reportInstrument]` — the
    `heartbeat.Load` / `heartbeat.CheckLiveness` / `heartbeat.Rollup`
    calls into agentic-kit (cmd/fathom/instrument.go:37,71,75) are absent.
    The reverse direction agrees: `find_callers(agentic-kit,
    CheckLiveness)` returns only the kit's own
    `heartbeat_test.TestCheckLiveness` — never
    `fathom::main.cmdInstrument`. Only `references`' text tier can save
    the query; the graph tiers are blind to fleet-crossing edges.
  - **(b) Parameter-name inconsistency across the hot quartet.** The
    B1-T09 transcript burned ~5 error turns on `-32602` invalid-params
    guess-rounds (three rounds observed in the critic's replay): the four
    most-adjacent tools each name their primary argument differently —
    `find_symbol` wants `name` (internal/mcp/tools.go:145), `references`
    wants `query` (internal/mcp/references_tool.go:19), `query_symbols`
    wants `concern` (internal/mcp/tools.go:22), `symbol_context` wants
    `symbol` (internal/mcp/nav_tools.go:147). An agent that has just used
    one tool reliably guesses the next one's parameter wrong.
  - **(c) Case-insensitive tier steers to the wrong package.**
    `find_symbol(agentic-kit, "heartbeat")` → `matched_via:
    case_insensitive` → the `Heartbeat` GoType in
    `foundations/coordination/worker-heartbeat/types.go` — not the
    `foundations/telemetry/heartbeat` package the caller (and
    instrument.go) actually uses. The honest `matched_via` marker is
    present, but the steering still sent the bench agent down the wrong
    package before it recovered.
- **Root cause:** (a) call-edge extraction resolves callees within a
  service; cross-service edges exist only in platformlink/text tiers, and
  `symbol_context`/`find_callers` don't consult them. (b) tool schemas
  were named per-tool, never normalized as a set. (c) the case-insensitive
  tier ranks bare-name equality without package-affinity context.
- **Candidate fixes (in leverage order):** normalize the primary-argument
  name (accept `symbol` as an alias on all four; cheap, additive);
  cross-repo callee/caller stitching via the platform graph for fleet
  stores; package-affinity ranking (or a multi-match list) for the
  case-insensitive tier.
- **Status:** open — logged from the bench run + live reproductions above;
  no fix landed with this entry.
- **Impact:** the exact query class the fleet index exists to win
  ("who calls this, anywhere?") is currently its worst measured case:
  6.6× more tokens than grep. Fixing (a) alone likely flips B1-T09's sign;
  (b) is a tax on every discovery session.

### O10 — `fathom claim` triage is store-direct: fleet-store triage requires stopping the daemon

- **Symptom (by design, not a bug):** `fathom claim refute|corroborate` (D3 findings
  tracer, FT-3) opens the DuckDB store directly (`--store` required). Against the
  always-on fleet daemon's `workspace/db/fleet.duckdb` that write collides with the
  single-writer lock — triaging a fleet claim means stopping the daemon, running the
  verb, and restarting.
- **Why it shipped this way:** a triage verdict is a rare, deliberate, single write
  best made against a store the operator can inspect; v0 mirrors `verify --store` /
  `remap --store` rather than growing the read-serving daemon a write verb.
- **Trade-off cost:** on the dogfood fleet the daemon *is* the store's owner, so the
  ergonomics are stop/start-shaped for what should be a one-liner.
- **Candidate fix (when triage sees real use):** a `claim` MCP tool or daemon verb so
  the daemon (the lock holder) performs the write — same shape as the `ingest` tool
  solving the same lock problem for refreshes.
- **Status:** open; documented v0 trade-off.

### O10 — relative corpus paths at the ingest boundary: typed edges silently absent + a daemon cross-refresh hazard — FIXED

- **Symptom:** rehearsing the ADR-020 store migration (2026-07-22), a CLI
  `ingest --service fathom --path . --store <fresh>.sqlite` produced 4723
  symbols and **0 typed edges — no error, no counter, no stderr** — while
  `TestTypedEdges_PerformanceGate` over the same tree produced 9236.
- **Root cause 1 (typed edges):** the connector-scope filter rel-maps
  go/packages' absolute file positions against `repoPath`; `filepath.Rel`
  errors on a relative base, so `repoRelSlash` fell back to the absolute
  path, which never matches the repo-relative allowed set → every package
  skipped. The CLI never printed the typed counters, so `PackagesTotal=215,
  processed=0` was invisible (probe: the perf test passes an absolute
  fixture root — the only difference).
- **Root cause 2 (worse, latent):** `ServiceMeta.Path` was recorded verbatim,
  so `--path .` (the CLI default AND both repos' post-commit hooks) records
  `"."` — which the daemon resolves from **its own** working directory on
  auto-refresh. The kit's hook would refresh the fathom corpus under the
  `agentic-kit` service ID. Latent only because DuckDB's single-writer lock
  made those hook ingests always fail while the daemon ran; ADR-020's WAL
  store would have activated it on migration day.
- **Fix (commit with this entry):** `EnrichTypedEdges` normalizes `repoPath`
  to absolute at entry (`TestEnrichTypedEdges_RelativeRepoPath`); the
  pipeline records `ServiceMeta.Path` absolute
  (`TestIngest_ServiceMetaPathAbsolute`); a never-silent stderr guard fires
  when go/packages sees packages but the scope filter excludes all of them;
  `cmd/fathom ingest` prints the typed-edge counters; both repos' hooks pass
  `--path "$PWD"`.
- **Lesson:** a "best-effort, never blocks" pass still owes a *visible*
  zero — the counters existed in `fusion.Report` and died unprinted at the
  CLI boundary. And rehearse operator runbooks against real data: this was
  found only because the migration was rehearsed on a scratch copy first.

### O9 — test_reaches recall residue after H1: aliased external-test calls and re-ambiguated methods

- **Symptom:** first HTTP-mode run of the re-armed gate on the restarted daemon
  (new binary 5983251, agentic-kit re-ingested at 149a570): evidence-spine.md
  verifies 13/15 — `{symbol: PreconditionSatisfied}` and `{symbol: Run,
  package: foundations/evidence/evidence-ladder}` fail `test_reaches`, while
  H1's store-mode proof (kit at the pre-H2 snapshot) had 15/15.
- **Evidence (verbatim probes):** both symbols ARE tested —
  `evidencesignature_test.go:190` calls `sig.PreconditionSatisfied(...)` from
  an external `_test` package via import alias; `evidenceladder_test.go:97`
  calls `l.Run(context.Background(), ...)`. Known-good neighbor on the same
  run: `{symbol: Build, package: foundations/evidence/case-file}` passes.
- **Root cause (CONFIRMED — the two initial hypotheses were wrong):** a fresh
  full-kit scratch ingest with the new binary verifies BOTH claims (2/2), so
  the H1 recall fixes (byDirName aliased-selector resolution, method edges)
  work. The fleet store was serving **stale-extractor observations**: the
  pass-3 hash-gate keyed fingerprints on file CONTENT only, so re-ingesting
  after the binary upgrade skipped every unchanged file and kept the
  pre-H1 `Calls` forms (`sig.PreconditionSatisfied` un-canonicalized, no
  ctor-scoped method forms). Extractor improvements could never reach
  unchanged files — the observation-level twin of the documented
  binary-staleness class.
- **Fix (this commit):** `fusion.ExtractorEpoch` salts every fingerprint
  (`fusion.FingerprintHash`, also adopted by `ObligationsIndexStale` so the
  formulas stay identical — the client test caught the divergence
  immediately). Bump the epoch in any commit that changes extraction
  semantics; the salt change invalidates all stored fingerprints and the next
  ingest re-extracts everything. Regression:
  `TestIngest_ExtractorEpochInvalidatesOldFingerprints`.
- **Also learned:** a `--path`-rooted scratch ingest of a single package dir
  flattens `FilePath` (no parent dirs), which disables `byDirName` — probe
  stores must be rooted at the repo, or dir-segment resolution silently
  vanishes. (Cost me one wrong intermediate conclusion.)
- **Status:** fixed in code; operational step = re-ingest the fleet on the
  new binary (every service re-extracts once — the epoch salt forces it).

### O8 — obligations v2 serves code-scoped and descriptive rules on documentation edits (fix landed; fleet recompile pending)

- **Symptom:** every `Write` to `docs/vision/*.md` (2026-07-14) injected the same six
  obligations, several of which cannot bind a docs edit.
- **Evidence (verbatim, from the PostToolUse payload):** editing `docs/vision/README.md`
  injected — among others — "Always verify code with the race detector using go test
  -race. [CLAUDE.md#L50-L52]" (a code-only rule), "Assert extractor and query outputs
  using golden files. [ADR.md#L174-L175]" (code-only), and "Bucket pending results by
  language and extract. [ADR.md#L68-L68]" — which is a *description of how the lazy
  pipeline works* (ADR-005 context), not an obligation on any editor. The identical
  6-rule set appeared for eight different markdown files, suggesting `**`-wide scopes
  rather than file-class scoping.
- **Root cause (probable):** the v2 prompt prefers narrow scopes but the compiler has
  no notion of *file-class* (code vs docs vs config), so rules whose source text doesn't
  name a path compile with repo-wide scopes; and the "binding rules only" instruction
  still passes descriptive-imperative sentences ("bucket pending results…") that read
  like commands.
- **Addendum (same session):** the hook also injected the full fathom rule set for an
  edit to `~/.claude/projects/.../memory/*.md` — a file *outside every indexed repo*.
  The edit-hook resolves the service from session cwd candidates and apparently still
  scope-matches a file that doesn't relativize into the service root; it should exit
  silent when the edited file is outside the resolved candidate's root.
- **Fix direction:** serve-time precision pass (docs/vision/04 Gap 3): sample injected
  payloads across the fleet, add file-class awareness to scoping (a code-only rule must
  not match `**/*.md`), and tighten the prompt's rejection of descriptive imperatives.
  Success metric: injected-rule precision ≥0.95 sampled at serve time.
- **Status:** fix landed in code (docs/plans/o8-precision-and-speclines.md
  OP-1..OP-3): prompt v3 (`internal/semantic/prompts.go`) instructs file-class
  scoping and forbids bare `**`; a compile-time over-broad-scope guard
  (`verifyDraft`, `internal/semantic/compile.go`) rejects any scope matching
  more than half the indexed corpus, exactly like a failed citation; the
  edit-hook (`cmd/fathom/edithook.go`) now exits silently for a file that
  relativizes under NO service-candidate root instead of falling back to the
  raw out-of-repo path, closing the addendum. Not yet observably fixed at
  serve time: every already-compiled claim in the fleet index still carries
  its old (pre-v3) scope until the flag-day recompile (OP-6, deferred —
  operator-side, needs a keyed shell) re-runs `compile` on a binary containing
  this slice. **Impact:** unchanged until OP-6 lands; the compile-time and
  edit-hook halves of the fix are what OP-6 will activate fleet-wide.

### O7 — optional-capability interfaces silently stripped by the auto-refresh wrapper (fixed)

- **Symptom:** first post-migration smoke test: `fathom compile --service fathom` over
  HTTP → "RPC error: backend does not support layer compilation", while the same call
  worked in every unit test; the edit-hook smoke test looked like an expected
  zero-obligations silence but was actually failing the same way.
- **Evidence:** `compile_layer`/`obligations` assert optional interfaces on the served
  backend; the daemon serves `mcp.NewAutoRefresh(client)` — a struct embedding the
  `QueryInterface` *interface*, which forwards only that interface's method set. Every
  semantic method fell off the wrapper. Direct-backend tests (no wrapper) stayed green.
- **Root cause:** Go interface embedding + type-assertion capability detection don't
  compose across wrappers; and the edit-hook's (correct) silent-on-error contract
  masked the failure in manual smoke testing.
- **Fix (same session):** explicit forwards on `autoRefreshBackend` (Obligations /
  ObligationsIndexStale / CompileObligations / ClaimsSummary), with `ensure()` freshness
  on the read paths — plus `TestSemanticTools_ThroughAutoRefreshWrapper`, which
  exercises the tools exactly as the daemon serves them.
- **Status:** resolved. **Impact/lesson:** any future optional backend capability MUST
  be forwarded on every wrapper and covered by a through-the-wrapper test; smoke tests
  of silent-by-design channels must use the loud path (the raw tool) first.

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
