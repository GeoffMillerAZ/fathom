# Index snapshots: export, import, cache-from ingest and environment daemons (IXS-)

Design: `docs/design/index-snapshots.md` (accepted as a design, parked, 2026-10-11); deployment options and costs:
`docs/design/index-snapshots-deployment.md`. Decision: ADR-069. Roadmap: `docs/vision/38-proof-layer-roadmap.md`
section 9.4. Futures: F27, F70, F72, F73, F74, F3 as amended. Readings live under
`docs/bench/readings/index-snapshots/`; every reading is $0 in model spend, and its bar is fixed here before any
run (K19: never moved after the data). Named consumer: helper-machine cold start (ADR-078 D3); phase 1 starts after RT-M1 reads PASS.

Parked, not scheduled. The owner prioritizes this work when ready, and no earlier than war1 landing and RT-M1
reading (its first amended window closes 2026-10-19 00:00Z). Only phase 0 rides war1 (IXS-M0, IXS-0); war1 is
paused, and its queue is `docs/plans/resume-2026-10.md` R1. Each later phase starts on its own trigger (the design,
section 12), and every phase carries a kill that can fire after it starts. The worktree warm index (seam H5) has its
own start, behind RT-M1 PASS. Symbols are provisional until grounded with `find_symbol` at row start. The
environment-daemon rows (phase 5) need the owner's U1 answer.

## Phase 0: the timing reading (rides war1)

- [x] (op) IXS-M0: in CI from a clean checkout, over 20 recent fathom commits, one ingest at a time under the ingest lock: wall time of cold, warm and cache-from (a copy of the previous commit's store plus `fathom remap`, no new code) ingests; the medians of cache-from over warm and over cold recorded; KEEP at a cache-from median of at most 0.6x cold; above it, F27 stays deferred and phases 1 and 2 do not start {id: IXS-M0, oracle: lexical, path: docs/bench/readings/index-snapshots/cache-from-timing.md, pattern: "^IXS-M0 reading: "}
- [x] An index snapshot carries no machine-local field: the store spec lists `ingest_state.path` (internal/store/ingeststate.go), `file_stats.mtime_ns` and `file_stats.ino` (internal/store/derived.go) beside `service_meta.path` as machine-local, and an export clears them; no store-format break {id: IXS-0, symbol: clearMachineLocal, package: internal/store, reachable_from: TestIndexSnapshotCarriesNoMachineLocalField}

## Phase 1: export and cache-from ingest, parity first

Start: the owner prioritizes it; war1 landed and RT-M1 read; IXS-M0 KEEP. Kill: a parity mismatch unfixed after two
waves deletes `--cache-from` and `export snapshot` under a `gone:` row.

- [ ] `fathom ingest --cache-from <file>` copies a snapshot the operator names in that invocation (I56), remaps it to the root (`fathom remap`) and ingests; files whose epoch-salted fingerprints match skip extraction; a snapshot from another fathom release or config is named with the differing cache-key components and costs only time; a snapshot of another repository is refused; on fixtures a cache-from ingest equals a cold one {id: IXS-1, symbol: cacheFromSnapshot, package: cmd/fathom, reachable_from: TestCacheFromIngestEqualsColdIngest}
- [ ] `fathom export snapshot` copies an allow-listed table set with `VACUUM INTO`, clears machine-local fields on the copy, verifies `application_id` and `user_version`, records its cache key and components in an additive `export_meta` table, and prints them; `--no-docs` drops `docs_fts`, `doc_units` and `doc_summaries`; it never uploads or signs {id: IXS-9, symbol: exportSnapshot, package: cmd/fathom, reachable_from: cmdExport, needs: "IXS-0"}
- [ ] (op) IXS-M0b: across 10 commits, a digest over the portable derived tables (symbols, edges, observations, typed edges, `docs_fts`; machine-local and wall-clock columns excluded) of a `--cache-from` ingest equals a cold ingest at the same commit; 10 of 10 equal is KEEP; one mismatch blocks every export until fixed {id: IXS-M0b, oracle: lexical, path: docs/bench/readings/index-snapshots/cache-from-parity.md, pattern: "^IXS-M0b verdict: (KEEP|CUT)", needs: "IXS-0,IXS-1,IXS-9"}

## Phase 2: the publish recipe and its readings

Start: phase 1's exit; the owner names the first producing repositories (design question 5). Kill: `--cache-from`
above 0.6x cold, or a hit rate under 0.2: F27 stays unpromoted, and the publish job stops.

- [ ] The publish recipe: a plain CI job runs a clean-checkout ingest with `--cache-from` the previous snapshot, `fathom export snapshot`, signs with the operator's signer, and `env.lock` names the fathom release; fathom never uploads, signs or fetches {id: IXS-4, oracle: lexical, path: docs/guides/index-snapshots.md, pattern: "^## Publish an index snapshot from CI", needs: "IXS-M0b"}
- [ ] (op) IXS-M1: `--cache-from` wall time over warm, the median of 20 merges in the same CI environment, on fathom and one platform-engineering repository; KEEP at most 1.3x warm; CUT above 0.6x cold {id: IXS-M1, oracle: lexical, path: docs/bench/readings/index-snapshots/cache-from-cost.md, pattern: "^IXS-M1 verdict: (KEEP|HOLD|CUT)", needs: "IXS-1,IXS-4"}
- [ ] (op) IXS-M2: the same-release cache hit rate over 30 days with the release locked in `env.lock`; KEEP at 0.5 or more; CUT under 0.2 (sharing stops); between 0.2 and 0.5, F73 is considered {id: IXS-M2, oracle: lexical, path: docs/bench/readings/index-snapshots/cache-hits.md, pattern: "^IXS-M2 verdict: (KEEP|CUT)", needs: "IXS-4"}
- [ ] (op) IXS-M5: on 20 PRs, the share of tool answers for changed symbols that differ between the base and head index, and a cache-from PR ingest over cold; KEEP at 10% or more and over 20%; under either bar the overlay store stays deferred, and F70 drops on two CUTs six months apart {id: IXS-M5, oracle: lexical, path: docs/bench/readings/index-snapshots/branch-divergence.md, pattern: "^IXS-M5 verdict: (KEEP|CUT)", needs: "IXS-1"}
- [ ] (op) IXS-M7: compressed size and cross-commit dedup over 20 consecutive snapshots, export wall time, CI runner peak RSS, environment daemon RSS with 1, 5 and 20 snapshots under a fixed query mix, and compose wall time per lineup change; KEEP when the p95 compose is at most half the median interval between lineup changes in the owner's first environment; over it, the in-place lineup change (design question 8) is designed; RSS or disk over one reader's budget fires F3 trigger (a) {id: IXS-M7, oracle: lexical, path: docs/bench/readings/index-snapshots/cost.md, pattern: "^IXS-M7 reading: ", needs: "IXS-4"}

## Phase 3: read-only serving, the answer basis and a read-only sidecar

Start: phase 1's exit; a named consumer for a read-only sidecar (a review runner or an incident agent). Loopback
only, so no U1 answer is needed. Kill: no session recorded with a law 6 stamp file within 60 days deletes
`--read-only` and the recipe under a `gone:` row (the SR-17 precedent).

- [ ] Every tool answer carries an answer basis: each `service@commit` it read, with repository identity (root-commit id and subpath) and cache key, the lineup and digest when one is served, and one state, `exact`, `differs`, `unchecked` or `uncommitted` (the weakest across the services read); a caller names an expectation by the `X-Fathom-Expect-Lineup` or `X-Fathom-Expect-Commit` header or the CLI's `--expect-commit`, never `_meta` or a tool parameter; `behind N` is computed on the caller's side {id: IXS-2, symbol: answerBasis, package: internal/mcp, reachable_from: TestEveryToolAnswerCarriesBasis}
- [ ] `serve-http --read-only` opens snapshots with `store.MountReadOnly`, turns auto-refresh off with no refresh queue, refuses the `ingest` tool, refresh-file, compile and claim writes, turns the observatory's wiring preview, apply and undo endpoints off, holds the decision port at provider `none`, and hard-gates or disables bare `/rpc` {id: IXS-3, symbol: readOnlyGuard, package: cmd/fathom, reachable_from: runServeHTTP}
- [ ] `serve-http --read-only --lineup <file>` validates the lineup against `schema/lineup` (valid and invalid testdata pairs), refuses an entry whose sha256 or recorded cache key differs, and serves a one-entry lineup from a read-only mount; it reads the lineup once, at start {id: IXS-10, symbol: loadLineup, package: cmd/fathom, reachable_from: runServeHTTP, needs: "IXS-3"}
- [ ] (op) IXS-M3: a hostile sweep of every MCP tool, bare `/rpc`, every named surface, the auto-refresh path, the observatory routes, and `/reload`, `/route` and the wiring apply and undo routes by name, against `--read-only`, leaves the snapshot bytes unchanged and calls no provider; and one consumer's read-only sidecar session is recorded with a law 6 stamp file; a changed byte or a provider call is a defect that blocks {id: IXS-M3, oracle: lexical, path: docs/bench/readings/index-snapshots/read-only.md, pattern: "^IXS-M3 verdict: (KEEP|CUT)", needs: "IXS-3,IXS-12,IXS-13"}

## Defects the deployment research found (fix with phase 3, or earlier)

- [x] A store opened read-only from read-only media opens: `sqliteReadOnlyDSN` sets only `mode=ro` (internal/store/conformance.go:182-188), and SQLite opens a WAL-mode file on read-only media only when its `-shm` and `-wal` files exist, its directory is writable, or the URI says `immutable=1` (https://www.sqlite.org/wal.html), while the store spec describes a WAL-mode store; so `export snapshot` writes `journal_mode=DELETE` into the snapshot, or a read-only mount of an immutable file opens with `immutable=1`; the test mounts a WAL-mode store from a read-only directory, red first {id: IXS-12, symbol: sqliteReadOnlyDSN, package: internal/store, reachable_from: TestMountReadOnlyOpensStoreFromReadOnlyDirectory}
- [ ] `POST /reload`, a stub that answers 200 and does nothing (cmd/fathom/servehttp.go:653-665) on the serving address, is refused under `--read-only`, so a proxied request can never reach it; the docs that describe it as a reload (docs/usage/serve-http.md, docs/claude-code-integration.md, docs/architecture.md) are corrected in the same change; deleting the route instead is a compatibility-surface change that needs a `### Breaking` line {id: IXS-13, symbol: newHTTPHandlerDeps, package: cmd/fathom, reachable_from: TestReadOnlyRefusesReloadStub, needs: "IXS-3"}

## Phase 4: import snapshot

Start: IXS-M1 KEEP and a named consumer of a cross-repo read. Kill: no session recorded with a law 6 stamp file
reading an imported service within 60 days of IXS-7 landing deletes `import snapshot` under a `gone:` row.

- [ ] `fathom import snapshot <file> [--as <service>]` merges an operator-named snapshot's service into a writable store under the lineup's service name, so every code tool answers it unchanged; refused under `--read-only` {id: IXS-7, symbol: importSnapshot, package: cmd/fathom, reachable_from: cmdImportSnapshot, needs: "IXS-M1"}

## Phase 5: the environment daemon (needs U1)

Start: U1 answered yes; IXS-M3 KEEP; phase 4 built; one named service with an on-call owner (F74's promote); the
owner's ruling that a lineup change is not attach (the design, section 2). Incident answers are code-only until WAE
wave 2's facts land. Kill: IXS-M8 fails, IXS-M9 reads CUT, or no second team within 90 days of exit: F74 drops.

- [ ] `serve-http --read-only --lineup <file>` with several entries composes its own store at start and reports ready only when every entry loads; a failed entry keeps it unready and names the entry and the reason {id: IXS-11, symbol: composeLineup, package: cmd/fathom, reachable_from: runServeHTTP, needs: "IXS-7,IXS-10"}
- [ ] Suppressions, call logs and ledgers are keyed per lineup, never shared across lineups on one machine {id: IXS-5, symbol: lineupScopedPath, package: internal/store, reachable_from: TestScopedStateKeyedPerLineup, needs: "IXS-M3"}
- [ ] The observatory shows a per-service freshness column (each entry's age and `resolved_from`) from GKE-MH-6's staleness library {id: IXS-8, symbol: freshnessColumn, package: cmd/fathom, reachable_from: runServeHTTP, needs: "IXS-M3"}
- [ ] The environment-daemon recipe: the operator's authenticating proxy in the same pod over loopback (rewrites `Host`, drops `Origin`, passes the `X-Fathom-Expect-*` headers, authenticates every request), an init container that delivers and verifies the lineup's files, `serve-http --read-only --lineup`, a lineup change as a rolling update, and the freshness column {id: IXS-6, oracle: lexical, path: docs/guides/index-snapshots.md, pattern: "^## Run an environment daemon", needs: "IXS-5,IXS-8,IXS-11"}
- [ ] (op) IXS-M8: every unauthenticated request through every route of the proxy-plus-daemon pair is refused, with `/reload`, `/route` and the wiring apply and undo routes named (law 9 as amended); any accepted means no launch {id: IXS-M8, oracle: lexical, path: docs/bench/readings/index-snapshots/proxy-hostile.md, pattern: "^IXS-M8 verdict: (KEEP|CUT)", needs: "IXS-6"}
- [ ] (op) IXS-M9: an environment daemon serves a gate or incident session recorded with a law 6 stamp file within 60 days of its first deploy; with none, F74 drops and the lineup mode and the recipe are deleted under a `gone:` row {id: IXS-M9, oracle: lexical, path: docs/bench/readings/index-snapshots/environment-daemon-use.md, pattern: "^IXS-M9 verdict: (KEEP|CUT)", needs: "IXS-11,IXS-M8"}
- [ ] (op) IXS-M4: a second team uses an environment daemon for four weeks; CUT if no second team is named within 90 days of phase 5's exit {id: IXS-M4, oracle: lexical, path: docs/bench/readings/index-snapshots/second-team.md, pattern: "^IXS-M4 verdict: (KEEP|CUT)", needs: "IXS-M8"}

## Worktree warm index (seam H5, readouts design §20)

Start: RT-M1 reads PASS and the tenancy budget fix lands (O1073, O1014), since a warm service adds a second service's
rows to daemon memory; the owner's ask of 2026-10-16 (every seam rated Medium or higher working, goal G41) schedules
these rows ahead of this plan's parked phases. IXS-M0's CUT measured commit-to-commit cache-from and bars phases 1
and 2; IXS-M10 measures the zero-diff case it found cheap (7 of 20 commits, 16 to 24 s) under its own bar and moves
no IXS-M0 bar. Kill: IXS-M10 CUT dispositions IXS-14 deferred with that reading as its reason and re-rates H5; IXS-M11
FAIL turns the warm index off by default in the same change.

- [ ] (op) IXS-M10, the worktree case IXS-M0 did not bar: on the owner's machine, 20 worktrees created at the parent's HEAD and 20 at HEAD plus a one-file diff, each seeded from the parent service's rows by IXS-M0's copy-and-remap path and then ingested as its own service, against a cold ingest of the same tree, one ingest at a time under the ingest lock; KEEP at a median of at most 0.3x cold in both sets, CUT otherwise {id: IXS-M10, oracle: lexical, path: docs/bench/readings/index-snapshots/worktree-warm.md, pattern: "^IXS-M10 verdict: (KEEP|CUT)"}
- [ ] WorktreeCreate stays unhooked: it is a creation hook, so a registered hook must create the worktree and print its path, and one that prints nothing fails every worktree-isolated spawn (O34, HP-29 refused, `CapWorktreeAdvice` in `internal/harness/descriptors.go`). The warm starts instead from SubagentStart when the subagent's cwd is a worktree (where `worktreeAdvice` already rides) and from the first daemon call whose root is a worktree with no service of its own; either asks the loopback daemon, within the hook's ceiling and never waiting for the ingest, to warm the worktree as its own service, named for the worktree and never a re-root of the parent (I30), seeded from the parent service's rows and ingested through the daemon's existing refresh queue; the worktree advice (`worktreeAdvice`) names the warm service once it is ready and says `warming` before, never presenting the parent's index as the worktree's; fathom removes a warm service it created once the worktree path is gone (I63), and doctor lists each one; off by default behind `[hooks] worktree_warm`, added in this row {id: IXS-14, symbol: warmWorktreeService, package: cmd/fathom, reachable_from: TestWorktreeWarmIsItsOwnServiceNeverAReroot, needs: "IXS-M10"}
- [ ] (op) IXS-M11, H5's seam reading: over at least 7 days of real sessions from the first warm row's `at` (INCOMPLETE under it) and at least 10 warm worktrees (INSUFFICIENT under it), a false answer is a tool answer served from a warm service that names a location the worktree's tree does not hold at that call's HEAD, rechecked from the session transcripts, and one is FAIL; recorded: ready time p50 and p95, daemon RSS added per warm service, the share of worktree sessions whose first fathom call found it ready, and the same mismatch share for answers from the parent's index in the 7 days before {id: IXS-M11, oracle: lexical, path: docs/bench/readings/index-snapshots/worktree-warm-week.md, pattern: "^IXS-M11 verdict: (PASS|FAIL|INSUFFICIENT|INCOMPLETE)", needs: "IXS-14"}

## Outside fathom

- [ ] (op) IXS-M6: reverts per 100 PRs traced to concurrently open PRs with intersecting blast radius, once the owner names a repository with concurrent open PRs; KEEP at 3 or more opens a merge-ahead bot outside fathom; under 3, no bot {id: IXS-M6, oracle: lexical, path: docs/bench/readings/index-snapshots/merge-ahead.md, pattern: "^IXS-M6 verdict: (KEEP|CUT)"}

## Phases 6 to 8, and what is deferred or refused

- [-] Versioned cross-repository links across index snapshots (phase 6) {id: IXS-D3, disposition: deferred, reason: "F72", trigger: "responsible-tenant.md#RT-M1 reads PASS, system-graph.md#SG-2 lands, phase 4 is built and a lineup names at least two services"}
- [-] An overlay store keyed by (service, ref, merge-base) (phase 7) {id: IXS-D1, disposition: deferred, reason: "F70", trigger: "IXS-M5 reads KEEP"}
- [-] Shared compiled layers inside index snapshots (phase 8) {id: IXS-D5, disposition: deferred, reason: "model spend; the spend hold of 2026-09-29", trigger: "the spend hold lifts"}
- [-] Per-extractor epoch salts {id: IXS-D2, disposition: deferred, reason: "F73", trigger: "IXS-M2 reads under 0.5 with the release locked"}
- [-] Retaining a snapshot for every deployed commit without a cap {id: IXS-D4, disposition: deferred, reason: "uncapped growth (about 155 GB a week at 50 services); the owner's cap D13 is 90 days then one per service per month", trigger: "IXS-M7 reads a dedup ratio"}
- [-] An in-place lineup change (the design's question 8 option b) {id: IXS-D7, disposition: deferred, reason: "a rolling update is the default; an in-place change needs a verb that is not reload and a channel the deploy job reaches without the serving address", trigger: "IXS-M7's compose bar fails"}
- [-] Query-time RPC between live daemons {id: IXS-D6, disposition: deferred, reason: "ADR-069 decision 9: daemons connect by shipping snapshots", trigger: "one of F3's amended triggers fires"}
- [-] A deploy ledger, or CD pushing deploy events into fathom {id: IXS-X1, disposition: refused, reason: "IM-X2 attach; deploy state is a declaration (ADR-069 decision 7)"}
- [-] Fathom fetching, pulling or syncing a snapshot, or listing a registry {id: IXS-X2, disposition: refused, reason: "I56; F27 do-not-build"}
- [-] Authentication inside fathom, or a non-loopback bind {id: IXS-X3, disposition: refused, reason: "the operator's proxy owns auth under law 9 as amended (ADR-069 decision 10); SR-17 deleted the bearer path"}
- [-] A multi-tenant or writable daemon {id: IXS-X4, disposition: refused, reason: "the scope non-goal stands even after U1"}
- [-] Federating code tools over mounted stores {id: IXS-X5, disposition: refused, reason: "re-adds the read path SR-17 deleted; IXS-7 imports instead"}
- [-] A watch on the lineup file, a reload endpoint, a control socket or a reload signal {id: IXS-X6, disposition: refused, reason: "the dispatch non-goal: fathom never watches, schedules or retries; a lineup change is a new process"}
