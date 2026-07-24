# Store swap — pure-Go SQLite WAL backend (ADR-020)

Grounded by wf_d0588a59: 26 Store methods, 9 tables, JSON-as-TEXT
throughout, five ON CONFLICT upserts that port verbatim, zero other
dialect coupling. The only DDL divergence is DuckDB's `ADD COLUMN IF NOT
EXISTS` migration cruft — the SQLite schema is written fresh with the
final column set. modernc.org/sqlite (pure Go; throughline precedent).

Design constraints (normative):
- New `internal/store/sqlite.go` implementing the full Store interface;
  registered in the contract-test suite EXACTLY as DuckDB and memory are
  today — the suite is the migration's definition of done, extended with
  a cross-process concurrency test SQLite must pass and DuckDB never
  could: a reader process (or second connection in a separate process via
  a helper binary/test flag) reads while a writer holds a write
  transaction.
- Pragmas per connection: journal_mode=WAL, busy_timeout=5000,
  synchronous=NORMAL. Read pool UNCAPPED (do not carry over DuckDB's
  SetMaxOpenConns(1)); writes rely on SQLite's own single-writer lock.
- `store.BackendForPath(path)` — one shared helper dispatching
  `.duckdb` → DuckStore, `.sqlite`/`.db` → SQLiteStore; consumed by
  store.Open, PlatformStore.load (its `*.duckdb` glob widens to both),
  genmanifest, platformingest. Unknown extension → typed error naming
  both options.
- PutObservations: keep bare-INSERT + purge-then-insert semantics; FIX
  the store.go doc comment that implies DB-level dedup (it is false for
  both backends and the fusion contract depends on purge-first).
- `fathom migrate-store --from X.duckdb --to Y.sqlite`: ports ONLY the
  non-derivable tables — semantic_claims, claim_links, claim_events,
  compile_runs, service_meta, commits, knowledge_cards, doc_summaries
  (eight: cards and summaries are LLM-derived enrichment a re-ingest does
  NOT rebuild — enrichment is opt-in and unwired in production, and
  PurgeService deliberately retains cards for the same reason, so treating
  them as derivable would silently lose them) — then prints the re-ingest
  command for the derived remainder (observations, fingerprints). Opens
  the source read-only (no schema DDL against it). Refuses an existing
  non-empty target. Row-count + spot-hash verification printed per table,
  with each hash verified by re-reading the ported rows from the target.
- Defaults flip: CLI default store path, init scaffold, docs, daemon
  plist guidance in docs/configuration.md move to `fleet.sqlite`
  naming. DuckDB stays compiled-in this slice (migration needs it);
  its removal (and CGO's) is the follow-up row below, gated on the
  operator's fleet migration.
- Determinism: contract suite runs all three backends; sorted outputs
  unchanged; injected clocks unchanged.

## Tasks

- [x] SQLite backend implementing all 26 Store methods, fresh final-column DDL, WAL pragmas, uncapped readers {id: SS-1, symbol: NewSQLiteStore, package: internal/store}
- [x] Contract-suite registration for the SQLite backend — every existing contract subtest green on it unmodified {id: SS-2, symbol: NewSQLiteStore, package: internal/store, skip_test: true, skip: "the deliverable IS the test suite passing on the new backend; the contract tests exercise it via the suite runner, an indirect path the callgraph cannot attribute"}
- [x] Cross-process concurrency proof: reader succeeds while writer transaction is open (the property DuckDB lacks, the reason for the swap) {id: SS-3, symbol: TestSQLite_ReaderDuringWriter, package: internal/store, skip_test: true, skip: "the test IS the anchor; nothing reaches a Test* symbol"}
- [x] store.BackendForPath shared dispatch replacing all four .duckdb hardcodes; PlatformStore loads mixed-backend fleets {id: SS-4, symbol: BackendForPath, package: internal/store}
- [-] migrate-store command: non-derivable tables ported with per-table verification, derived tables re-ingest instruction, non-empty-target refusal {id: SS-5, disposition: refused, reason: "shipped, used to migrate the fleet successfully (workspace/logs/store-migration-stamp.txt), then deleted outright by SS-8/ADR-025 once the migration completed and the command's only purpose (reading DuckDB as a source) had no live consumer left — cmdMigrateStore no longer exists in cmd/fathom; a pre-SS-8 build is the historical reference"}
- [x] Doc-comment truth fix on PutObservations (purge-then-insert contract stated; DB-idempotency implication removed) {id: SS-6, symbol: PutObservations, package: internal/store, skip_test: true, skip: "interface-method doc fix; PutObservations is contract-suite-reached via the runner, the documented indirect-dispatch residue"}
- [x] Defaults + docs flip to sqlite (CLI default path, init scaffold, configuration.md incl. daemon guidance, distribution.md CGO note updated to 'gone after SS-8') {id: SS-7, symbol: cmdInit, package: cmd/fathom, skip_test: true, skip: "docs + default-string deliverable anchored to the verb that scaffolds it"}

- [x] DuckDB backend + CGO removal (deletes duckdb driver, bindings, GOTMPDIR/deployment-target workarounds; vendor probe shrinks ~540MB) {id: SS-8, symbol: BackendForPath, package: internal/store}
  - **Trigger fired 2026-07-23**: operator ran `migrate-store` on the fleet
    DB and the daemon ran a full day green on sqlite
    (`workspace/logs/store-migration-stamp.txt`: `2026-07-23T01:56:17Z`,
    fleet checkout). Executed same day (ADR-025): `internal/store/duckdb.go`
    + its test, `internal/store/migrate.go` + its test, and
    `cmd/fathom/migratestore.go` deleted; `go.mod`/`go.sum` cleaned by
    `go mod tidy` (DuckDB driver, bindings, and every Arrow-only indirect
    dependency gone); `BackendForPath` now refuses `.duckdb` with a typed
    error (`TestBackendForPath_DuckDBLoudlyRefused`) instead of opening
    `DuckStore`; `internal/store` builds and passes `go test -race` under
    `CGO_ENABLED=0` (proven, not asserted). `CGO_ENABLED=1`/
    `MACOSX_DEPLOYMENT_TARGET=14.0` remain required for the full binary
    solely because of the tree-sitter extractor bindings (ADR-013) — a
    separate, not-yet-triggered removal (XT-6b,
    `docs/plans/extraction-treesitter.md`), untouched by this row.
    `migrate-store` was removed rather than kept as a CGO island: the plan's
    own trigger text ties CGO's presence entirely to that command needing
    the DuckDB driver, and the fleet had already migrated, so keeping it
    would have defeated this row's own point for a tool with no remaining
    live purpose (a straggler store can still be migrated with a pre-SS-8
    build). `workspace/db/fleet.duckdb` (the on-disk rollback file) is left
    untouched — the operator's manual step to delete once satisfied it is
    no longer needed.

## Deferred

- [-] Litestream/backup story {id: SS-9, disposition: deferred, reason: "single-machine dogfood; the store is 26MB and mostly derivable", trigger: "first deployment where store loss is not recoverable by re-ingest + recompile"}
