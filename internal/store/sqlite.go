package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// openStoreCount is the process-lifetime count of SQLiteStore handles
// currently open (O108, cmd/fathom's /metrics fathom_open_stores):
// incremented on every successful NewSQLiteStore, decremented on every
// Close. A daemon that opens exactly one store at startup and never
// re-opens one should hold this at a constant 1 (+1 per mounted
// expert-module store) for its whole lifetime — a climbing value points
// directly at a per-request/per-refresh store leak (O108 candidate c)
// instead of leaving it to be inferred from RSS growth alone.
var openStoreCount int64 // atomic

// OpenStores returns the current count of open SQLiteStore handles in this
// process. See openStoreCount's doc for what a healthy value looks like.
func OpenStores() int64 {
	return atomic.LoadInt64(&openStoreCount)
}

// SQLiteStore is a pure-Go, SQLite-backed (modernc.org/sqlite, WAL mode)
// implementation of Store (ADR-020). Unlike the old DuckStore backend
// (deleted in SS-8/ADR-025) it does not cap the connection pool: SQLite's own single-writer lock serialises writes, and
// WAL mode lets any number of readers proceed concurrently with one writer —
// the property DuckDB's cross-process single-writer lacked and the reason
// for this backend's existence.
type SQLiteStore struct {
	db *sql.DB
	// path is the store file this handle was opened against ("" for an
	// in-memory database). Recorded because the physical half of a purge
	// (docs/design/scope-and-purge.md §3.2's last row) cannot be done
	// through the SQL handle alone: VACUUM needs a file to compact and the
	// raw-bytes probe needs one to read.
	path string
	// gate is the per-call suppression filter (scoped.go). Every
	// constructor initialises it via newScopeGate; the audit test
	// TestEveryStoreConstructorIsScoped fails any constructor that does
	// not.
	gate scopeGate
}

// storeFormatVersion is the format_version marker docs/spec/store-format-v0.md
// §2 requires every conforming store to carry, via SQLite's built-in PRAGMA
// user_version. 1 for this spec (v0's marker value). Bump only alongside a
// new spec version (v1+) that documents what changed — the marker exists so
// a future reader can identify which spec version wrote a store without
// guessing from column presence.
//
// Deliberately NOT 0: PRAGMA user_version defaults to 0 on every SQLite
// database ever created, including one this binary never touched — mapping
// spec v0 to marker 0 would make the marker vacuous (a reader could never
// distinguish a conforming store from an arbitrary SQLite file, and the
// stampFormatMarkers write-if-lower guard below would be structurally
// unreachable, since user_version is never negative in normal use). 0 is
// reserved to mean "pre-spec/unversioned" (see storeFormatVersionUnversioned)
// — a reader that sees 0 MUST fall back to schema introspection rather than
// trusting the marker.
const storeFormatVersion = 1

// storeFormatVersionUnversioned is SQLite's own default PRAGMA user_version.
// Documented as a named constant (rather than a bare 0) so its meaning —
// "no fathom writer has stamped this store yet" — is legible at every call
// site that compares against it.
const storeFormatVersionUnversioned = 0

// storeApplicationID is SQLite's PRAGMA application_id (docs/spec/
// store-format-v0.md §2): the file-TYPE identifier SQLite's own format
// reserves for exactly this purpose, distinct from user_version (which
// identifies WHICH VERSION of that type a given file is). Value: the ASCII
// bytes "FATH" read big-endian (0x46='F', 0x41='A', 0x54='T', 0x48='H') —
// a stable, human-verifiable mnemonic rather than an opaque hash. Fixed
// forever once published: changing it would break every existing reader's
// "is this a fathom store?" check.
const storeApplicationID = 0x46415448

// sqlitePragmaDSN appends the per-connection pragmas this backend requires
// (journal_mode=WAL, busy_timeout, synchronous=NORMAL) to path as DSN query
// parameters. modernc.org/sqlite applies "_pragma" query parameters to every
// new physical connection it opens (conn.go's newConn → applyQueryParams),
// so these apply uniformly across the whole (uncapped) pool, not just the
// first connection — the property a one-time "PRAGMA journal_mode=WAL"
// exec on a single *sql.DB handle would NOT guarantee once the pool grows.
func sqlitePragmaDSN(path string) string {
	q := url.Values{}
	// busy_timeout first by modernc convention (applied in that order
	// regardless; listed first here for readability).
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "synchronous(NORMAL)")
	return path + "?" + q.Encode()
}

// NewSQLiteStore opens (or creates) a SQLite database at path in WAL mode. Pass
// "" for an in-memory database (no WAL applies to memory databases; SQLite
// ignores the journal_mode pragma there). The schema is created fresh if it
// does not already exist — there is no legacy on-disk migration path for
// this backend (the old DuckStore backend, deleted in SS-8, carried that).
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	dsn := ":memory:"
	if path != "" {
		if dir := filepath.Dir(path); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("store.NewSQLiteStore mkdir %q: %w", dir, err)
			}
		}
		dsn = sqlitePragmaDSN(path)
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store.NewSQLiteStore: %w", err)
	}
	// File-backed stores: uncapped read pool (ADR-020) — the
	// SetMaxOpenConns(1) the old DuckStore backend used to serialise
	// writes was DuckDB-specific and must NOT carry over; SQLite's own single-writer
	// lock (busy_timeout-backed) already serialises writes, and capping the
	// pool would throttle concurrent readers for no reason.
	//
	// In-memory stores are the one exception: every physical modernc
	// connection to ":memory:" opens its OWN private database, so a second
	// pool connection sees no tables at all — an uncapped pool made 15 of
	// 16 concurrent readers fail with "no such table" (F1). One connection
	// keeps the single shared database; an ephemeral test/session store
	// needs no reader concurrency.
	if path == "" {
		db.SetMaxOpenConns(1)
	}

	s := &SQLiteStore{db: db, path: path, gate: newScopeGate()}
	if err := s.createSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store.NewSQLiteStore schema: %w", err)
	}
	if err := s.EnsureIndexes(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store.NewSQLiteStore indexes: %w", err)
	}
	atomic.AddInt64(&openStoreCount, 1)
	return s, nil
}

// serveIndexes are the secondary indexes SU-1 (docs/plans/substrate.md §A,
// ADR-041 decision 3) adds on top of the fixed schema above: one per column
// a serve path actually filters on — the obligations claim query
// (semantic_claims by service_id/layer/status), the find_callers/
// find_callees typed-edge queries (typed_edges by service_id/to_symbol and
// service_id/from_symbol), claim_links' own service/claim/rel lookup, the
// per-source observations lookup, and external_findings' cursor lookup.
// Every statement is CREATE INDEX IF NOT EXISTS — safe to run on every
// Open, on a fresh store or a years-old one, with no ALTER-TABLE-style
// migration bookkeeping required (unlike the tableColumns migrations
// above, an index either already exists or is created; there is no
// intermediate schema shape to detect).
var serveIndexes = []string{
	`CREATE INDEX IF NOT EXISTS idx_semantic_claims_serve
    ON semantic_claims (service_id, layer, status)`,
	`CREATE INDEX IF NOT EXISTS idx_claim_links_service_claim_rel
    ON claim_links (service_id, claim_id, rel)`,
	`CREATE INDEX IF NOT EXISTS idx_typed_edges_to_symbol
    ON typed_edges (service_id, to_symbol)`,
	`CREATE INDEX IF NOT EXISTS idx_typed_edges_from_symbol
    ON typed_edges (service_id, from_symbol)`,
	`CREATE INDEX IF NOT EXISTS idx_observations_service_source
    ON observations (service_id, source_ref)`,
	`CREATE INDEX IF NOT EXISTS idx_external_findings_service_claim
    ON external_findings (service_id, claim_id)`,
}

// EnsureIndexes creates (idempotently) the secondary indexes serveIndexes
// lists. Called once at construction (NewSQLiteStore) and safe to call
// again any time — every statement is CREATE INDEX IF NOT EXISTS, so a
// repeat call against an already-indexed store is a fast no-op check per
// index, never an error and never a duplicate index.
func (s *SQLiteStore) EnsureIndexes(ctx context.Context) error {
	for _, ddl := range serveIndexes {
		if _, err := s.db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("store.EnsureIndexes: %w", err)
		}
	}
	return nil
}

// createSchema creates the fixed, final-column schema. Unlike the old
// DuckStore backend (deleted in SS-8) there is no ALTER-TABLE migration
// cruft here: every SQLite store was created fresh from this slice onward —
// the now-removed `migrate-store` command ported data from an existing
// DuckDB store rather than upgrading an old SQLite one in place.
func (s *SQLiteStore) createSchema() error {
	const ddl = `
CREATE TABLE IF NOT EXISTS observations (
    service_id    TEXT    NOT NULL,
    concern_id    TEXT    NOT NULL DEFAULT '',
    source_ref    TEXT    NOT NULL,
    source_kind   TEXT    NOT NULL,
    evidence_tier TEXT    NOT NULL,
    symbol        TEXT    NOT NULL DEFAULT '',
    package       TEXT    NOT NULL DEFAULT '',
    claim         TEXT    NOT NULL DEFAULT '',
    content_hash  TEXT    NOT NULL DEFAULT '',
    calls         TEXT    NOT NULL DEFAULT '[]',
    complexity    INTEGER NOT NULL DEFAULT 0,
    line          INTEGER NOT NULL DEFAULT 0,
    end_line      INTEGER NOT NULL DEFAULT 0,
    ingested_at   BIGINT  NOT NULL,
    tool_kind     TEXT    NOT NULL DEFAULT '',
    generated     INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS fingerprints (
    service_id   TEXT NOT NULL,
    source_ref   TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    PRIMARY KEY (service_id, source_ref)
);

CREATE TABLE IF NOT EXISTS knowledge_cards (
    service_id TEXT NOT NULL,
    symbol     TEXT NOT NULL,
    file       TEXT NOT NULL DEFAULT '',
    summary    TEXT NOT NULL DEFAULT '',
    tags       TEXT NOT NULL DEFAULT '[]',
    PRIMARY KEY (service_id, symbol)
);

CREATE TABLE IF NOT EXISTS doc_summaries (
    service_id TEXT NOT NULL,
    source_ref TEXT NOT NULL,
    title      TEXT NOT NULL DEFAULT '',
    summary    TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (service_id, source_ref)
);

CREATE TABLE IF NOT EXISTS service_meta (
    service_id      TEXT   NOT NULL PRIMARY KEY,
    path            TEXT   NOT NULL DEFAULT '',
    indexed_commit  TEXT   NOT NULL DEFAULT '',
    indexed_at      BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS commits (
    service_id TEXT    NOT NULL,
    hash       TEXT    NOT NULL,
    author     TEXT    NOT NULL DEFAULT '',
    date       BIGINT  NOT NULL DEFAULT 0,
    message    TEXT    NOT NULL DEFAULT '',
    files      TEXT    NOT NULL DEFAULT '[]',
    spec_ids   TEXT    NOT NULL DEFAULT '[]',
    PRIMARY KEY (service_id, hash)
);

CREATE TABLE IF NOT EXISTS semantic_claims (
    service_id             TEXT   NOT NULL,
    claim_id               TEXT   NOT NULL,
    layer                  TEXT   NOT NULL DEFAULT '',
    layer_version          TEXT   NOT NULL DEFAULT '',
    kind                   TEXT   NOT NULL DEFAULT '',
    subject                TEXT   NOT NULL DEFAULT '',
    rule                   TEXT   NOT NULL DEFAULT '',
    scope                  TEXT   NOT NULL DEFAULT '[]',
    tier                   TEXT   NOT NULL DEFAULT '',
    tier_profile           TEXT   NOT NULL DEFAULT '',
    status                 TEXT   NOT NULL DEFAULT '',
    model                  TEXT   NOT NULL DEFAULT '',
    prompt_version         TEXT   NOT NULL DEFAULT '',
    repro_key              TEXT   NOT NULL DEFAULT '',
    input_digest           TEXT   NOT NULL DEFAULT '',
    compiled_at_watermark  TEXT   NOT NULL DEFAULT '',
    compiled_at            BIGINT NOT NULL DEFAULT 0,
    class                  TEXT   NOT NULL DEFAULT '',
    valid_from             TEXT   NOT NULL DEFAULT '',
    valid_to               TEXT   NOT NULL DEFAULT '',
    asserted_by            TEXT   NOT NULL DEFAULT '',
    meta                   TEXT   NOT NULL DEFAULT '',
    PRIMARY KEY (service_id, claim_id)
);

CREATE TABLE IF NOT EXISTS claim_links (
    service_id  TEXT NOT NULL DEFAULT '',
    claim_id    TEXT NOT NULL DEFAULT '',
    rel         TEXT NOT NULL DEFAULT '',
    target_ref  TEXT NOT NULL DEFAULT '',
    target_hash TEXT NOT NULL DEFAULT '',
    anchor_text TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS compile_runs (
    run_id             TEXT    NOT NULL DEFAULT '',
    service_id         TEXT    NOT NULL,
    layer              TEXT    NOT NULL DEFAULT '',
    corpus_watermark   TEXT    NOT NULL DEFAULT '',
    model              TEXT    NOT NULL DEFAULT '',
    prompt_version     TEXT    NOT NULL DEFAULT '',
    dry_run            BOOLEAN NOT NULL DEFAULT FALSE,
    tokens_in          INTEGER NOT NULL DEFAULT 0,
    tokens_out         INTEGER NOT NULL DEFAULT 0,
    cost_usd           DOUBLE  NOT NULL DEFAULT 0,
    duration_ms        BIGINT  NOT NULL DEFAULT 0,
    sections_total     INTEGER NOT NULL DEFAULT 0,
    sections_compiled  INTEGER NOT NULL DEFAULT 0,
    sections_failed    INTEGER NOT NULL DEFAULT 0,
    claims_emitted     INTEGER NOT NULL DEFAULT 0,
    claims_rejected    INTEGER NOT NULL DEFAULT 0,
    ran_at             BIGINT  NOT NULL DEFAULT 0,
    refused            BOOLEAN NOT NULL DEFAULT FALSE,
    refusal_reason     TEXT    NOT NULL DEFAULT '',
    budget_stopped     BOOLEAN NOT NULL DEFAULT FALSE,
    escape_note        TEXT    NOT NULL DEFAULT '',
    rejection_reasons_json TEXT NOT NULL DEFAULT '[]',
    resumed_from       TEXT    NOT NULL DEFAULT '',
    uncompiled_scopes_json TEXT NOT NULL DEFAULT '[]',
    sections_prefiltered INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS claim_events (
    claim_id          TEXT   NOT NULL DEFAULT '',
    verdict           TEXT   NOT NULL DEFAULT '',
    author            TEXT   NOT NULL DEFAULT '',
    note              TEXT   NOT NULL DEFAULT '',
    event_at          BIGINT NOT NULL DEFAULT 0,
    resolution_kind   TEXT   NOT NULL DEFAULT '',
    resolution_commit TEXT   NOT NULL DEFAULT '',
    asserted_by       TEXT   NOT NULL DEFAULT ''
);

-- Typed call edges (ADR-021 §3, internal/fusion.EnrichTypedEdges). Rows are
-- replaced at PACKAGE granularity (pkg_path), never per-file — the typed
-- pass's cache unit is a whole go/packages load, not a single source file.
CREATE TABLE IF NOT EXISTS typed_edges (
    service_id   TEXT NOT NULL,
    pkg_path     TEXT NOT NULL DEFAULT '',
    from_service TEXT NOT NULL DEFAULT '',
    from_file    TEXT NOT NULL DEFAULT '',
    from_symbol  TEXT NOT NULL DEFAULT '',
    to_symbol    TEXT NOT NULL DEFAULT '',
    kind         TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_typed_edges_service_id
    ON typed_edges (service_id);

-- Per-package content-hash cache: skip reloading a package via go/packages
-- when its file-set hash is unchanged since the last successful load.
CREATE TABLE IF NOT EXISTS typed_edge_fingerprints (
    service_id   TEXT NOT NULL,
    pkg_path     TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    PRIMARY KEY (service_id, pkg_path)
);

-- Every observations query filters on service_id (usually with concern_id
-- or a symbol filter on top); without an index each one is a full table
-- scan over the store's largest table. Cheap to create at birth, a
-- migration later — so it is part of the fresh DDL (F7).
CREATE INDEX IF NOT EXISTS idx_observations_service_id
    ON observations (service_id);
`
	if _, err := s.db.Exec(ddl); err != nil {
		return err
	}
	if _, err := s.db.Exec(externalFindingsDDL); err != nil {
		return err
	}

	// review_runs (FL-15, docs/plans/findings-loop.md) is a separate const,
	// executed right after the main schema — the same additive-table
	// convention a later wave's own table follows; the main ddl const above
	// is never edited for a new table.
	if err := s.createReviewRunsSchema(); err != nil {
		return err
	}

	// Test evidence + declared certifications (vision 33 / ADR-043 stage
	// 0, TC-2/TC-5) — same additive-table convention, own file
	// (sqlite_testevidence.go).
	if err := s.createTestEvidenceSchema(); err != nil {
		return err
	}

	// typed_edge_status (DG-10) follows the same additive-table convention
	// as review_runs above.
	if err := s.createTypedEdgeStatusSchema(); err != nil {
		return err
	}

	// ingest_state (RT-109) — same additive-table convention
	// (sqlite_ingeststate.go).
	if err := s.createIngestStateSchema(); err != nil {
		return err
	}

	// file_imports (RT-108) — same additive-table convention
	// (fileimports.go).
	if err := s.createFileImportsSchema(); err != nil {
		return err
	}

	// Conditional migrations for SQLite stores created before a column
	// existed. The "no ALTER cruft" stance above holds for the fleet store
	// (freshly migrated), but scratch/workspace SQLite stores created by
	// earlier slices already exist on disk — INCLUDING the live fleet store,
	// which predates escape_note — and SQLite has no ADD COLUMN IF NOT
	// EXISTS, so missing columns are detected via PRAGMA table_info and
	// added individually, per table (ADR-021 §2 / critic F1; A2-1/D3-1
	// bounce finding 1: adding escape_note only to the CREATE TABLE shape
	// was a no-op on every pre-existing store — CompileRuns and
	// RecordCompileRun both errored with "no such column").
	migrations := []struct{ table, column, ddl string }{
		{"observations", "tool_kind", `ALTER TABLE observations ADD COLUMN tool_kind TEXT NOT NULL DEFAULT ''`},
		{"observations", "generated", `ALTER TABLE observations ADD COLUMN generated INTEGER NOT NULL DEFAULT 0`},
		{"observations", "end_line", `ALTER TABLE observations ADD COLUMN end_line INTEGER NOT NULL DEFAULT 0`},
		{"compile_runs", "escape_note", `ALTER TABLE compile_runs ADD COLUMN escape_note TEXT NOT NULL DEFAULT ''`},
		// Q-3 (docs/plans/expert-suite-q.md, O19's fix): a pre-existing
		// store (including the live fleet store) predates rejection-reason
		// capture — RecordCompileRun/CompileRuns would otherwise error with
		// "no such column" exactly like escape_note's own migration bounce
		// finding above.
		{"compile_runs", "rejection_reasons_json", `ALTER TABLE compile_runs ADD COLUMN rejection_reasons_json TEXT NOT NULL DEFAULT '[]'`},
		// O54 (a second pilot reading found the obligations serve-time
		// coverage gap): a pre-existing store (including the live fleet
		// store) predates resumed_from/uncompiled_scopes_json —
		// RecordCompileRun/CompileRuns must not fail with "no such column"
		// on it, exactly like escape_note's own migration bounce finding
		// above; a legacy row simply reads back with ResumedFrom=="" and
		// UncompiledScopes==nil.
		{"compile_runs", "resumed_from", `ALTER TABLE compile_runs ADD COLUMN resumed_from TEXT NOT NULL DEFAULT ''`},
		{"compile_runs", "uncompiled_scopes_json", `ALTER TABLE compile_runs ADD COLUMN uncompiled_scopes_json TEXT NOT NULL DEFAULT '[]'`},
		// DP-6 (ADR-042, vision 32 §2 item 1): a pre-existing store (including
		// the live fleet store) predates the decision-port compile pre-filter —
		// RecordCompileRun/CompileRuns must not fail with "no such column" on
		// it, exactly like every other late compile_runs column above; a
		// legacy row simply reads back with SectionsPrefiltered==0 (honest:
		// no pre-filter ever ran on it).
		{"compile_runs", "sections_prefiltered", `ALTER TABLE compile_runs ADD COLUMN sections_prefiltered INTEGER NOT NULL DEFAULT 0`},
		// T5-1/O13: pre-existing claim_links rows (including the live fleet
		// store) predate AnchorText — a re-anchor read/write over such a
		// store must not fail with "no such column", it must simply see ''
		// (Sweep's existing "empty AnchorText → cannot heal, only stale"
		// leniency handles the legacy row honestly from there).
		{"claim_links", "anchor_text", `ALTER TABLE claim_links ADD COLUMN anchor_text TEXT NOT NULL DEFAULT ''`},
		// Findings-as-evidence (docs/design/findings-as-evidence.md §3.2,
		// §6 item 2): a pre-existing store (including the live fleet
		// store) predates the typed finding-class vocabulary —
		// PutClaims/ClaimsForService must not fail with "no such column"
		// on it; a legacy claim simply reads back with Class=="" (additive
		// migration, same "no such column" bounce class escape_note/
		// rejection_reasons_json/anchor_text already hit and fixed).
		{"semantic_claims", "class", `ALTER TABLE semantic_claims ADD COLUMN class TEXT NOT NULL DEFAULT ''`},
		// Findings-as-evidence resolution outcomes (§3.4, §6 item 4): a
		// pre-existing store predates ResolutionKind/ResolutionCommit —
		// AppendClaimEvent/ClaimEvents(Since) must not fail on it; a legacy
		// event simply reads back with both fields "".
		{"claim_events", "resolution_kind", `ALTER TABLE claim_events ADD COLUMN resolution_kind TEXT NOT NULL DEFAULT ''`},
		{"claim_events", "resolution_commit", `ALTER TABLE claim_events ADD COLUMN resolution_commit TEXT NOT NULL DEFAULT ''`},
		// SU-19 (ADR-041 decision 7, docs/vision/31-substrate.md §3 D6): a
		// pre-existing store (including the live fleet store) predates
		// bi-temporal validity — PutClaims/ClaimsForService(Serve) must not
		// fail with "no such column" on it; a legacy claim simply reads back
		// with ValidFrom/ValidTo both zero (core.SemanticClaim.ValidAt's own
		// doc comment: a zero bound is never constraining), exactly like
		// every earlier additive claim column's own migration bounce.
		{"semantic_claims", "valid_from", `ALTER TABLE semantic_claims ADD COLUMN valid_from TEXT NOT NULL DEFAULT ''`},
		{"semantic_claims", "valid_to", `ALTER TABLE semantic_claims ADD COLUMN valid_to TEXT NOT NULL DEFAULT ''`},
		// SU-22 (ADR-041 decision 7): a pre-existing store predates
		// AssertedBy — a legacy claim/event simply reads back with "".
		{"semantic_claims", "asserted_by", `ALTER TABLE semantic_claims ADD COLUMN asserted_by TEXT NOT NULL DEFAULT ''`},
		{"claim_events", "asserted_by", `ALTER TABLE claim_events ADD COLUMN asserted_by TEXT NOT NULL DEFAULT ''`},
		// vision 33 / ADR-043 stage 1 (TC-6): a pre-existing store predates
		// the opaque per-claim Meta provenance blob — PutClaims/
		// ClaimsForService(Serve) must not fail with "no such column" on
		// it; a legacy claim simply reads back with Meta==nil, same
		// additive-migration bounce every earlier claim column hit.
		{"semantic_claims", "meta", `ALTER TABLE semantic_claims ADD COLUMN meta TEXT NOT NULL DEFAULT ''`},
		// CV-2 (docs/plans/refactor-conservation.md, design doc §1.1,
		// O114, vision 34 / ADR-044 stage 1): a pre-existing store
		// (including the live fleet store) predates commit/dirty/race —
		// PutTestRuns/TestCaseRunsForSymbol must not fail with "no such
		// column" on it, same additive-migration bounce every earlier
		// test_runs-adjacent column hit; a legacy run simply reads back
		// unbound (commit_sha=='', dirty=false, race=false), which is
		// the honest answer for a run that never stated them.
		{"test_runs", "commit_sha", `ALTER TABLE test_runs ADD COLUMN commit_sha TEXT NOT NULL DEFAULT ''`},
		{"test_runs", "dirty", `ALTER TABLE test_runs ADD COLUMN dirty INTEGER NOT NULL DEFAULT 0`},
		{"test_runs", "race", `ALTER TABLE test_runs ADD COLUMN race INTEGER NOT NULL DEFAULT 0`},
	}
	existingByTable := map[string]map[string]bool{}
	for _, m := range migrations {
		existing, ok := existingByTable[m.table]
		if !ok {
			var err error
			existing, err = s.tableColumns(m.table)
			if err != nil {
				return err
			}
			existingByTable[m.table] = existing
		}
		if existing[m.column] {
			continue
		}
		if _, err := s.db.Exec(m.ddl); err != nil {
			return fmt.Errorf("migrate %s.%s: %w", m.table, m.column, err)
		}
	}
	return s.stampFormatMarkers()
}

// stampFormatMarkers writes the two SQLite pragmas docs/spec/
// store-format-v0.md §2 documents as a conforming store's self-identifying
// header: PRAGMA application_id (WHOSE file this is — storeApplicationID,
// fathom's stable magic number) and PRAGMA user_version (WHICH spec version
// wrote it — storeFormatVersion). Both writes are additive-safe:
//
//   - application_id is stamped only when it currently reads the SQLite
//     default (0, meaning "never set") — never overwriting a non-zero,
//     foreign application_id some other tool may already have stamped on
//     this file (BackendForPath dispatches on file extension alone, so a
//     ".sqlite" file that is not actually a fathom store is a real,
//     if pathological, possibility; refusing to relabel it is the safe
//     default).
//   - user_version is bumped only when it is strictly lower than
//     storeFormatVersion, never decreased — a store already stamped at or
//     above storeFormatVersion (e.g. by a future v1+ writer) is left
//     untouched, so an older binary opening a newer store cannot regress
//     the marker a newer reader depends on. A fresh or legacy store reads
//     storeFormatVersionUnversioned (0) here and is correctly bumped to 1.
//
// PRAGMA accepts no bind parameters; the interpolated values are the
// package-internal constants above, never caller input.
func (s *SQLiteStore) stampFormatMarkers() error {
	var appID int
	if err := s.db.QueryRow(`PRAGMA application_id`).Scan(&appID); err != nil {
		return fmt.Errorf("read application_id: %w", err)
	}
	if appID == 0 {
		if _, err := s.db.Exec(fmt.Sprintf(`PRAGMA application_id = %d`, storeApplicationID)); err != nil {
			return fmt.Errorf("set application_id: %w", err)
		}
	}

	var version int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}
	if version < storeFormatVersion {
		if _, err := s.db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, storeFormatVersion)); err != nil {
			return fmt.Errorf("set user_version: %w", err)
		}
	}
	return nil
}

// tableColumns returns the set of column names currently present on the
// named table, via PRAGMA table_info. table must be a trusted literal from
// this package (PRAGMA takes no bind parameters).
func (s *SQLiteStore) tableColumns(table string) (map[string]bool, error) {
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return nil, fmt.Errorf("pragma table_info(%s): %w", table, err)
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notNull int
			dflt    sql.NullString
			pk      int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			return nil, fmt.Errorf("pragma table_info scan: %w", err)
		}
		cols[name] = true
	}
	return cols, rows.Err()
}

// PutObservations inserts a batch of observations via a bare append — see the
// Store interface doc comment: this does NOT dedupe or replace by key.
// Callers that want replace semantics must call PurgeSource first (as
// internal/fusion does).
func (s *SQLiteStore) PutObservations(ctx context.Context, obs []core.Observation) error {
	if len(obs) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("PutObservations begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO observations
    (service_id, concern_id, source_ref, source_kind, evidence_tier,
     symbol, package, claim, content_hash, calls, complexity, line, end_line, ingested_at,
     tool_kind, generated)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`)
	if err != nil {
		return fmt.Errorf("PutObservations prepare: %w", err)
	}
	defer stmt.Close()

	for _, o := range obs {
		callsJSON, err := json.Marshal(o.Calls)
		if err != nil {
			return fmt.Errorf("PutObservations marshal calls: %w", err)
		}
		ingestedAt := o.IngestedAt
		if ingestedAt.IsZero() {
			ingestedAt = time.Now().UTC()
		}
		_, err = stmt.ExecContext(
			ctx,
			o.ServiceID,
			o.ConcernID,
			o.SourceRef,
			string(o.SourceKind),
			string(o.EvidenceTier),
			o.Symbol,
			o.Package,
			o.Claim,
			o.ContentHash,
			string(callsJSON),
			o.Complexity,
			o.Line,
			o.EndLine,
			ingestedAt.UnixNano(),
			o.ToolKind,
			o.Generated,
		)
		if err != nil {
			return fmt.Errorf("PutObservations exec: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("PutObservations commit: %w", err)
	}
	return nil
}

// QueryObservations returns observations for serviceID. When concernID is
// non-empty the result is filtered to rows whose concern_id matches; when
// concernID is "" all rows for the service are returned.
func (s *SQLiteStore) queryObservationsRaw(ctx context.Context, serviceID, concernID string) ([]core.Observation, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if concernID == "" {
		rows, err = s.db.QueryContext(ctx, `
SELECT service_id, concern_id, source_ref, source_kind, evidence_tier,
       symbol, package, claim, content_hash, calls, complexity, line, end_line, ingested_at,
       tool_kind, generated
FROM observations
WHERE service_id = ?
ORDER BY ingested_at
`, serviceID)
	} else {
		rows, err = s.db.QueryContext(ctx, `
SELECT service_id, concern_id, source_ref, source_kind, evidence_tier,
       symbol, package, claim, content_hash, calls, complexity, line, end_line, ingested_at,
       tool_kind, generated
FROM observations
WHERE service_id = ? AND concern_id = ?
ORDER BY ingested_at
`, serviceID, concernID)
	}
	if err != nil {
		return nil, fmt.Errorf("QueryObservations query: %w", err)
	}
	defer rows.Close()

	return scanObservations(ctx, rows)
}

// Fingerprint returns the stored content hash for (serviceID, sourceRef).
func (s *SQLiteStore) Fingerprint(ctx context.Context, serviceID, sourceRef string) (string, bool, error) {
	var hash string
	err := s.db.QueryRowContext(ctx, `
SELECT content_hash FROM fingerprints
WHERE service_id = ? AND source_ref = ?
`, serviceID, sourceRef).Scan(&hash)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("Fingerprint query: %w", err)
	}
	return hash, true, nil
}

// SetFingerprint records or replaces the content hash for (serviceID, sourceRef).
func (s *SQLiteStore) SetFingerprint(ctx context.Context, serviceID, sourceRef, hash string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO fingerprints (service_id, source_ref, content_hash)
VALUES (?, ?, ?)
ON CONFLICT (service_id, source_ref) DO UPDATE SET content_hash = excluded.content_hash
`, serviceID, sourceRef, hash)
	if err != nil {
		return fmt.Errorf("SetFingerprint: %w", err)
	}
	return nil
}

// PurgeSource removes all observations for the given (serviceID, sourceRef) pair.
func (s *SQLiteStore) PurgeSource(ctx context.Context, serviceID, sourceRef string) error {
	_, err := s.db.ExecContext(ctx, `
DELETE FROM observations WHERE service_id = ? AND source_ref = ?
`, serviceID, sourceRef)
	if err != nil {
		return fmt.Errorf("PurgeSource: %w", err)
	}
	return nil
}

// PurgeService removes all content rows (observations, fingerprints, doc
// summaries, typed edges, typed-edge fingerprints) for serviceID, keeping
// identity rows (service_meta, commits, knowledge cards). See the Store
// interface for the ADR-017 self-heal use.
func (s *SQLiteStore) PurgeService(ctx context.Context, serviceID string) error {
	for _, q := range []string{
		`DELETE FROM observations WHERE service_id = ?`,
		`DELETE FROM fingerprints WHERE service_id = ?`,
		`DELETE FROM doc_summaries WHERE service_id = ?`,
		`DELETE FROM typed_edges WHERE service_id = ?`,
		`DELETE FROM typed_edge_fingerprints WHERE service_id = ?`,
		`DELETE FROM file_imports WHERE service_id = ?`,
	} {
		if _, err := s.db.ExecContext(ctx, q, serviceID); err != nil {
			return fmt.Errorf("PurgeService: %w", err)
		}
	}
	return nil
}

// serviceIDTables is every table carrying a service_id column, in the order
// RemoveService deletes and RenameService updates them. claim_events is
// deliberately absent: it is claim_id-keyed (no service_id column — see
// ClaimEventsSince), so RemoveService reaches it through a semantic_claims
// subquery and RenameService leaves it alone entirely.
var serviceIDTables = []string{
	"observations",
	"fingerprints",
	"knowledge_cards",
	"doc_summaries",
	"service_meta",
	"commits",
	"semantic_claims",
	"claim_links",
	"compile_runs",
	"typed_edges",
	"typed_edge_fingerprints",
	"ingest_state",
	"file_imports",
}

// RemoveService deletes every row for serviceID in one transaction — see the
// Store interface doc comment for the registry-surgery motivation. The
// claim_events delete runs FIRST, while the service's semantic_claims rows
// still exist to resolve the claim_id membership subquery.
func (s *SQLiteStore) RemoveService(ctx context.Context, serviceID string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("RemoveService begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	removed := false
	res, err := tx.ExecContext(ctx, `
DELETE FROM claim_events
WHERE claim_id IN (SELECT claim_id FROM semantic_claims WHERE service_id = ?)
`, serviceID)
	if err != nil {
		return false, fmt.Errorf("RemoveService delete claim_events: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n > 0 {
		removed = true
	}

	for _, table := range serviceIDTables {
		res, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE service_id = ?`, serviceID)
		if err != nil {
			return false, fmt.Errorf("RemoveService delete %s: %w", table, err)
		}
		if n, err := res.RowsAffected(); err == nil && n > 0 {
			removed = true
		}
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("RemoveService commit: %w", err)
	}
	return removed, nil
}

// RenameService moves every service_id-scoped row from oldID to newID in one
// transaction — see the Store interface doc comment for the claim-ID
// stability contract. Both presence checks run inside the same transaction
// as the updates, so a concurrent writer cannot slip a newID row in between
// check and rename.
func (s *SQLiteStore) RenameService(ctx context.Context, oldID, newID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("RenameService begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const presenceQ = `
SELECT EXISTS(SELECT 1 FROM service_meta    WHERE service_id = ?)
    OR EXISTS(SELECT 1 FROM observations    WHERE service_id = ?)
    OR EXISTS(SELECT 1 FROM semantic_claims WHERE service_id = ?)
`
	var oldExists bool
	if err := tx.QueryRowContext(ctx, presenceQ, oldID, oldID, oldID).Scan(&oldExists); err != nil {
		return fmt.Errorf("RenameService check %q: %w", oldID, err)
	}
	if !oldExists {
		return &ServiceNotFoundError{ServiceID: oldID}
	}
	var newExists bool
	if err := tx.QueryRowContext(ctx, presenceQ, newID, newID, newID).Scan(&newExists); err != nil {
		return fmt.Errorf("RenameService check %q: %w", newID, err)
	}
	if newExists {
		return &ServiceExistsError{ServiceID: newID}
	}

	for _, table := range serviceIDTables {
		if _, err := tx.ExecContext(ctx, `UPDATE `+table+` SET service_id = ? WHERE service_id = ?`, newID, oldID); err != nil {
			return fmt.Errorf("RenameService update %s: %w", table, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("RenameService commit: %w", err)
	}
	return nil
}

// CountAbsoluteSourceRefs counts fingerprint rows whose source_ref is an
// absolute path (the pre-ADR-017 anchor style). POSIX-rooted refs only —
// the stores this migration concerns were written on macOS/Linux hosts.
func (s *SQLiteStore) CountAbsoluteSourceRefs(ctx context.Context, serviceID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
SELECT count(*) FROM fingerprints WHERE service_id = ? AND source_ref LIKE '/%'
`, serviceID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("CountAbsoluteSourceRefs: %w", err)
	}
	return n, nil
}

// ServiceContentDigest digests serviceID's fingerprints in source_ref order
// (see Store.ServiceContentDigest).
func (s *SQLiteStore) ServiceContentDigest(ctx context.Context, serviceID string) (string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT source_ref, content_hash FROM fingerprints WHERE service_id = ? ORDER BY source_ref
`, serviceID)
	if err != nil {
		return "", fmt.Errorf("ServiceContentDigest: %w", err)
	}
	defer rows.Close()
	h := sha256.New()
	for rows.Next() {
		var ref, hash string
		if err := rows.Scan(&ref, &hash); err != nil {
			return "", fmt.Errorf("ServiceContentDigest: %w", err)
		}
		writeDigestRow(h, ref, hash)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("ServiceContentDigest: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// writeDigestRow feeds one fingerprint into a content digest; both backends
// share it so they digest identically.
func writeDigestRow(h hash.Hash, ref, contentHash string) {
	h.Write([]byte(ref))
	h.Write([]byte{0})
	h.Write([]byte(contentHash))
	h.Write([]byte{'\n'})
}

// DeleteFingerprint removes the fingerprint row for (serviceID, sourceRef).
// Deleting a missing row is a no-op.
func (s *SQLiteStore) DeleteFingerprint(ctx context.Context, serviceID, sourceRef string) error {
	_, err := s.db.ExecContext(ctx, `
DELETE FROM fingerprints WHERE service_id = ? AND source_ref = ?
`, serviceID, sourceRef)
	if err != nil {
		return fmt.Errorf("DeleteFingerprint: %w", err)
	}
	return nil
}

// ListSourceRefs returns the sorted distinct source_refs for serviceID across
// the observations and fingerprints tables.
// sourcePackagesRaw is SourcePackages before the suppression filter.
func (s *SQLiteStore) sourcePackagesRaw(ctx context.Context, serviceID, sourceRef string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT DISTINCT package FROM observations
WHERE service_id = ? AND source_ref = ?
ORDER BY package
`, serviceID, sourceRef)
	if err != nil {
		return nil, fmt.Errorf("SourcePackages: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("SourcePackages scan: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("SourcePackages rows: %w", err)
	}
	return out, nil
}

// packageObservationsRaw is PackageObservations before the suppression
// filter.
func (s *SQLiteStore) packageObservationsRaw(ctx context.Context, serviceID string, pkgs []string) ([]core.Observation, error) {
	if len(pkgs) == 0 {
		return nil, nil
	}
	args := make([]any, 0, len(pkgs)+1)
	args = append(args, serviceID)
	marks := make([]string, len(pkgs))
	for i, p := range pkgs {
		marks[i] = "?"
		args = append(args, p)
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT service_id, concern_id, source_ref, source_kind, evidence_tier,
       symbol, package, claim, content_hash, calls, complexity, line, end_line, ingested_at,
       tool_kind, generated
FROM observations
WHERE service_id = ? AND package IN (`+strings.Join(marks, ",")+`)
ORDER BY source_ref, line, symbol
`, args...)
	if err != nil {
		return nil, fmt.Errorf("PackageObservations: %w", err)
	}
	defer rows.Close()
	return scanObservations(ctx, rows)
}

// sourceKindCountsRaw is SourceKindCounts before the suppression filter:
// one row per (source_ref, source_kind), never an observation row.
func (s *SQLiteStore) sourceKindCountsRaw(ctx context.Context, serviceID string) ([]SourceKindCount, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT source_ref, source_kind, count(*) FROM observations
WHERE service_id = ?
GROUP BY source_ref, source_kind
ORDER BY source_ref, source_kind
`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("SourceKindCounts: %w", err)
	}
	defer rows.Close()
	var out []SourceKindCount
	for rows.Next() {
		var r SourceKindCount
		if err := rows.Scan(&r.SourceRef, (*string)(&r.Kind), &r.Count); err != nil {
			return nil, fmt.Errorf("SourceKindCounts scan: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("SourceKindCounts rows: %w", err)
	}
	return out, nil
}

func (s *SQLiteStore) listSourceRefsRaw(ctx context.Context, serviceID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT source_ref FROM observations WHERE service_id = ?
UNION
SELECT source_ref FROM fingerprints WHERE service_id = ?
ORDER BY source_ref
`, serviceID, serviceID)
	if err != nil {
		return nil, fmt.Errorf("ListSourceRefs: %w", err)
	}
	defer rows.Close()
	var refs []string
	for rows.Next() {
		var ref string
		if err := rows.Scan(&ref); err != nil {
			return nil, fmt.Errorf("ListSourceRefs scan: %w", err)
		}
		refs = append(refs, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListSourceRefs rows: %w", err)
	}
	return refs, nil
}

// PutKnowledgeCards upserts a batch of knowledge-card rows. Existing rows for
// the same (service_id, symbol) are replaced.
func (s *SQLiteStore) PutKnowledgeCards(ctx context.Context, cards []KnowledgeCard) error {
	if len(cards) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("PutKnowledgeCards begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO knowledge_cards (service_id, symbol, file, summary, tags)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (service_id, symbol) DO UPDATE SET
    file    = excluded.file,
    summary = excluded.summary,
    tags    = excluded.tags
`)
	if err != nil {
		return fmt.Errorf("PutKnowledgeCards prepare: %w", err)
	}
	defer stmt.Close()

	for _, c := range cards {
		tagsJSON, err := json.Marshal(c.Tags)
		if err != nil {
			return fmt.Errorf("PutKnowledgeCards marshal tags: %w", err)
		}
		if _, err = stmt.ExecContext(ctx, c.ServiceID, c.Symbol, c.File, c.Summary, string(tagsJSON)); err != nil {
			return fmt.Errorf("PutKnowledgeCards exec: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("PutKnowledgeCards commit: %w", err)
	}
	return nil
}

// KnowledgeCards returns all knowledge cards for serviceID ordered by symbol.
func (s *SQLiteStore) knowledgeCardsRaw(ctx context.Context, serviceID string) ([]KnowledgeCard, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT service_id, symbol, file, summary, tags
FROM knowledge_cards
WHERE service_id = ?
ORDER BY symbol
`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("KnowledgeCards query: %w", err)
	}
	defer rows.Close()

	var result []KnowledgeCard
	for rows.Next() {
		var c KnowledgeCard
		var tagsJSON string
		if err := rows.Scan(&c.ServiceID, &c.Symbol, &c.File, &c.Summary, &tagsJSON); err != nil {
			return nil, fmt.Errorf("KnowledgeCards scan: %w", err)
		}
		if err := json.Unmarshal([]byte(tagsJSON), &c.Tags); err != nil {
			return nil, fmt.Errorf("KnowledgeCards unmarshal tags: %w", err)
		}
		result = append(result, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("KnowledgeCards rows: %w", err)
	}
	return result, nil
}

// PutDocSummaries upserts a batch of doc-summary rows. Existing rows for the
// same (service_id, source_ref) are replaced.
func (s *SQLiteStore) PutDocSummaries(ctx context.Context, summaries []DocSummary) error {
	if len(summaries) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("PutDocSummaries begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO doc_summaries (service_id, source_ref, title, summary)
VALUES (?, ?, ?, ?)
ON CONFLICT (service_id, source_ref) DO UPDATE SET
    title   = excluded.title,
    summary = excluded.summary
`)
	if err != nil {
		return fmt.Errorf("PutDocSummaries prepare: %w", err)
	}
	defer stmt.Close()

	for _, d := range summaries {
		if _, err = stmt.ExecContext(ctx, d.ServiceID, d.SourceRef, d.Title, d.Summary); err != nil {
			return fmt.Errorf("PutDocSummaries exec: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("PutDocSummaries commit: %w", err)
	}
	return nil
}

// DocSummaries returns all doc summaries for serviceID ordered by source_ref.
func (s *SQLiteStore) docSummariesRaw(ctx context.Context, serviceID string) ([]DocSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT service_id, source_ref, title, summary
FROM doc_summaries
WHERE service_id = ?
ORDER BY source_ref
`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("DocSummaries query: %w", err)
	}
	defer rows.Close()

	var result []DocSummary
	for rows.Next() {
		var d DocSummary
		if err := rows.Scan(&d.ServiceID, &d.SourceRef, &d.Title, &d.Summary); err != nil {
			return nil, fmt.Errorf("DocSummaries scan: %w", err)
		}
		result = append(result, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("DocSummaries rows: %w", err)
	}
	return result, nil
}

// ListServiceIDs returns the sorted distinct service IDs stored in the
// observations table.
func (s *SQLiteStore) ListServiceIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT DISTINCT service_id FROM observations ORDER BY service_id
`)
	if err != nil {
		return nil, fmt.Errorf("ListServiceIDs: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("ListServiceIDs scan: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListServiceIDs rows: %w", err)
	}
	return ids, nil
}

// ClaimServiceIDs returns the sorted distinct service IDs stored in the
// semantic_claims table — see the Store interface doc comment.
func (s *SQLiteStore) ClaimServiceIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT DISTINCT service_id FROM semantic_claims ORDER BY service_id
`)
	if err != nil {
		return nil, fmt.Errorf("ClaimServiceIDs: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("ClaimServiceIDs scan: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ClaimServiceIDs rows: %w", err)
	}
	return ids, nil
}

// SetServiceMeta upserts the ingest metadata for meta.ServiceID.
func (s *SQLiteStore) SetServiceMeta(ctx context.Context, meta ServiceMeta) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO service_meta (service_id, path, indexed_commit, indexed_at)
VALUES (?, ?, ?, ?)
ON CONFLICT (service_id) DO UPDATE SET
    path           = excluded.path,
    indexed_commit = excluded.indexed_commit,
    indexed_at     = excluded.indexed_at
`, meta.ServiceID, meta.Path, meta.IndexedCommit, meta.IndexedAt)
	if err != nil {
		return fmt.Errorf("SetServiceMeta: %w", err)
	}
	return nil
}

// GetServiceMeta returns the ingest metadata for serviceID.
// found is false when no row exists.
func (s *SQLiteStore) GetServiceMeta(ctx context.Context, serviceID string) (ServiceMeta, bool, error) {
	var m ServiceMeta
	err := s.db.QueryRowContext(ctx, `
SELECT service_id, path, indexed_commit, indexed_at
FROM service_meta
WHERE service_id = ?
`, serviceID).Scan(&m.ServiceID, &m.Path, &m.IndexedCommit, &m.IndexedAt)
	if err == sql.ErrNoRows {
		return ServiceMeta{}, false, nil
	}
	if err != nil {
		return ServiceMeta{}, false, fmt.Errorf("GetServiceMeta: %w", err)
	}
	return m, true, nil
}

// SetCommits replaces all commits for serviceID with the provided slice.
func (s *SQLiteStore) SetCommits(ctx context.Context, serviceID string, commits []Commit) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("SetCommits begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM commits WHERE service_id = ?`, serviceID); err != nil {
		return fmt.Errorf("SetCommits delete: %w", err)
	}
	if len(commits) > 0 {
		stmt, err := tx.PrepareContext(ctx, `
INSERT INTO commits (service_id, hash, author, date, message, files, spec_ids)
VALUES (?, ?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return fmt.Errorf("SetCommits prepare: %w", err)
		}
		defer stmt.Close()
		for _, c := range commits {
			filesJSON, _ := json.Marshal(c.Files)
			specJSON, _ := json.Marshal(c.SpecIDs)
			if _, err := stmt.ExecContext(ctx, c.ServiceID, c.Hash, c.Author, c.Date, c.Message,
				string(filesJSON), string(specJSON)); err != nil {
				return fmt.Errorf("SetCommits insert: %w", err)
			}
		}
	}
	return tx.Commit()
}

// ListCommits returns commits for serviceID newest-first (by date desc).
func (s *SQLiteStore) ListCommits(ctx context.Context, serviceID string) ([]Commit, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT service_id, hash, author, date, message, files, spec_ids
		 FROM commits WHERE service_id = ? ORDER BY date DESC`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("ListCommits query: %w", err)
	}
	defer rows.Close()
	var out []Commit
	for rows.Next() {
		var c Commit
		var filesJSON, specJSON string
		if err := rows.Scan(&c.ServiceID, &c.Hash, &c.Author, &c.Date, &c.Message,
			&filesJSON, &specJSON); err != nil {
			return nil, fmt.Errorf("ListCommits scan: %w", err)
		}
		_ = json.Unmarshal([]byte(filesJSON), &c.Files)
		_ = json.Unmarshal([]byte(specJSON), &c.SpecIDs)
		out = append(out, c)
	}
	return out, rows.Err()
}

// PutClaims upserts claims by (service_id, claim_id) and replaces the link
// set for every distinct claim_id that appears in links.
func (s *SQLiteStore) PutClaims(ctx context.Context, claims []core.SemanticClaim, links []core.ClaimLink) error {
	if len(claims) == 0 && len(links) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("PutClaims begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if len(claims) > 0 {
		stmt, err := tx.PrepareContext(ctx, `
INSERT INTO semantic_claims
    (service_id, claim_id, layer, layer_version, kind, subject, rule, scope,
     tier, tier_profile, status, model, prompt_version, repro_key, input_digest,
     compiled_at_watermark, compiled_at, class, valid_from, valid_to, asserted_by, meta)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (service_id, claim_id) DO UPDATE SET
    layer                 = excluded.layer,
    layer_version         = excluded.layer_version,
    kind                  = excluded.kind,
    subject               = excluded.subject,
    rule                  = excluded.rule,
    scope                 = excluded.scope,
    tier                  = excluded.tier,
    tier_profile          = excluded.tier_profile,
    -- Refutation durability (FT-4, docs/vision/03-findings-and-experts.md
    -- §3): a recompile that re-emits an unchanged rule under a refuted
    -- ClaimID must NOT resurrect it to the compiler's fresh status —
    -- suppression is refutation with a note, and it must survive every
    -- future upsert of the same claim_id, not just the first one.
    status                = CASE WHEN semantic_claims.status = 'refuted' THEN semantic_claims.status ELSE excluded.status END,
    model                 = excluded.model,
    prompt_version        = excluded.prompt_version,
    repro_key             = excluded.repro_key,
    input_digest          = excluded.input_digest,
    compiled_at_watermark = excluded.compiled_at_watermark,
    compiled_at           = excluded.compiled_at,
    class                 = excluded.class,
    -- SU-19 (ADR-041 decision 7): valid_from is stamped ONCE, at the
    -- claim_id's first-ever mint — an unchanged-content recompile that
    -- upserts the identical content-addressed row must never drift its
    -- own history forward, so a non-empty stored value always wins over
    -- whatever this call proposes.
    valid_from            = CASE WHEN semantic_claims.valid_from != '' THEN semantic_claims.valid_from ELSE excluded.valid_from END,
    -- valid_to mirrors status' own refutation-durability CASE above: a
    -- refuted claim's closed validity window survives every future
    -- upsert exactly like its status does. A NON-refuted row (including
    -- one this call is resurrecting from superseded back to fresh — see
    -- status' own doc comment) takes the incoming value, which reopens
    -- the window (an ordinary caller passes ValidTo's zero value/"" on a
    -- normal write) exactly when the status resurrection itself does.
    valid_to              = CASE WHEN semantic_claims.status = 'refuted' THEN semantic_claims.valid_to ELSE excluded.valid_to END,
    asserted_by           = excluded.asserted_by,
    meta                  = excluded.meta
`)
		if err != nil {
			return fmt.Errorf("PutClaims prepare claims: %w", err)
		}
		defer stmt.Close()

		for _, c := range claims {
			scopeJSON, err := json.Marshal(c.Scope)
			if err != nil {
				return fmt.Errorf("PutClaims marshal scope: %w", err)
			}
			metaText := ""
			if len(c.Meta) > 0 {
				metaText = string(c.Meta)
			}
			_, err = stmt.ExecContext(
				ctx,
				c.ServiceID, c.ClaimID,
				c.Layer, c.LayerVersion, c.Kind, c.Subject, c.Rule, string(scopeJSON),
				string(c.Tier), c.TierProfile,
				string(c.Status),
				c.Model, c.PromptVersion, c.ReproKey, c.InputDigest,
				c.CompiledAtWatermark, c.CompiledAt.UnixNano(), string(c.Class),
				formatClaimTime(c.ValidFrom), formatClaimTime(c.ValidTo), c.AssertedBy, metaText,
			)
			if err != nil {
				return fmt.Errorf("PutClaims exec claim: %w", err)
			}
		}
	}

	if len(links) > 0 {
		// Collect distinct claim_ids so we can replace their link sets.
		seen := make(map[string]struct{}, len(links))
		for _, l := range links {
			seen[l.ClaimID] = struct{}{}
		}
		for id := range seen {
			if _, err := tx.ExecContext(ctx, `DELETE FROM claim_links WHERE claim_id = ?`, id); err != nil {
				return fmt.Errorf("PutClaims delete links: %w", err)
			}
		}
		// Build service_id lookup from the claims in this call.
		svcOf := make(map[string]string, len(claims))
		for _, c := range claims {
			svcOf[c.ClaimID] = c.ServiceID
		}
		lstmt, err := tx.PrepareContext(ctx, `
INSERT INTO claim_links (service_id, claim_id, rel, target_ref, target_hash, anchor_text)
VALUES (?, ?, ?, ?, ?, ?)
`)
		if err != nil {
			return fmt.Errorf("PutClaims prepare links: %w", err)
		}
		defer lstmt.Close()
		for _, l := range links {
			if _, err := lstmt.ExecContext(ctx, svcOf[l.ClaimID], l.ClaimID, l.Rel, l.TargetRef, l.TargetHash, l.AnchorText); err != nil {
				return fmt.Errorf("PutClaims exec link: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("PutClaims commit: %w", err)
	}
	return nil
}

// ClaimsForService returns claims for serviceID ordered by (layer, subject,
// claim_id). When layer is "" all layers are returned.
func (s *SQLiteStore) ClaimsForService(ctx context.Context, serviceID, layer string) ([]core.SemanticClaim, error) {
	const cols = `service_id, claim_id, layer, layer_version, kind, subject, rule, scope,
	tier, tier_profile, status, model, prompt_version, repro_key, input_digest,
	compiled_at_watermark, compiled_at, class, valid_from, valid_to, asserted_by, meta`
	var (
		rows *sql.Rows
		err  error
	)
	if layer == "" {
		rows, err = s.db.QueryContext(ctx, `
SELECT `+cols+`
FROM semantic_claims
WHERE service_id = ?
ORDER BY layer, subject, claim_id
`, serviceID)
	} else {
		rows, err = s.db.QueryContext(ctx, `
SELECT `+cols+`
FROM semantic_claims
WHERE service_id = ? AND layer = ?
ORDER BY layer, subject, claim_id
`, serviceID, layer)
	}
	if err != nil {
		return nil, fmt.Errorf("ClaimsForService query: %w", err)
	}
	defer rows.Close()
	return scanClaims(rows)
}

// scopeReachAnywhereConditions are the SQL LIKE fragments matching a
// stored scope column (JSON array text, e.g. `["internal/store/**"]`)
// whose glob(s) can match ANY path regardless of scopePrefixes — see
// ClaimsForServe's doc comment on the Store interface. Checked against
// the column's literal JSON text, so the quoting mirrors how
// encoding/json renders a string element: `"**"`, `"**/…`, `"*"`.
var scopeReachAnywhereLike = []string{
	`%"**"%`, // exactly "**"
	`%"**/%`, // starts with "**/"
	`%"*"%`,  // exactly "*"
}

// ClaimsForServe is ClaimsForService's prefiltered sibling — see the Store
// interface doc comment for the exact (deliberately over-approximate)
// scope-prefix semantics.
func (s *SQLiteStore) ClaimsForServe(ctx context.Context, serviceID string, layers []string, statuses []core.ClaimStatus, scopePrefixes []string) ([]core.SemanticClaim, error) {
	const cols = `service_id, claim_id, layer, layer_version, kind, subject, rule, scope,
	tier, tier_profile, status, model, prompt_version, repro_key, input_digest,
	compiled_at_watermark, compiled_at, class, valid_from, valid_to, asserted_by, meta`

	where := []string{"service_id = ?"}
	args := []any{serviceID}

	if len(statuses) > 0 {
		placeholders := make([]string, len(statuses))
		for i, st := range statuses {
			placeholders[i] = "?"
			args = append(args, string(st))
		}
		where = append(where, "status IN ("+strings.Join(placeholders, ", ")+")")
	}
	if len(layers) > 0 {
		placeholders := make([]string, len(layers))
		for i, l := range layers {
			placeholders[i] = "?"
			args = append(args, l)
		}
		where = append(where, "layer IN ("+strings.Join(placeholders, ", ")+")")
	}
	if len(scopePrefixes) > 0 {
		var scopeOr []string
		for _, pat := range scopeReachAnywhereLike {
			scopeOr = append(scopeOr, "scope LIKE ?")
			args = append(args, pat)
		}
		// A bare literal scope entry (no "/" anywhere in the whole JSON
		// array) is gitignore-style "matches anywhere" — see globAnchor's
		// doc comment in internal/semantic/governingscope.go.
		scopeOr = append(scopeOr, "scope NOT LIKE ?")
		args = append(args, "%/%")
		for _, prefix := range scopePrefixes {
			scopeOr = append(scopeOr, "scope LIKE ?")
			args = append(args, "%"+prefix+"%")
		}
		where = append(where, "("+strings.Join(scopeOr, " OR ")+")")
	}

	q := `SELECT ` + cols + ` FROM semantic_claims WHERE ` + strings.Join(where, " AND ") + ` ORDER BY layer, subject, claim_id`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("ClaimsForServe query: %w", err)
	}
	defer rows.Close()
	return scanClaims(rows)
}

// ClaimLinks returns links for the given claim IDs within serviceID, ordered
// by (claim_id, rel, target_ref). Returns nil with no error for empty claimIDs.
func (s *SQLiteStore) claimLinksRaw(ctx context.Context, serviceID string, claimIDs []string) ([]core.ClaimLink, error) {
	if len(claimIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(claimIDs))
	args := make([]interface{}, 0, 1+len(claimIDs))
	args = append(args, serviceID)
	for i, id := range claimIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	q := `
SELECT claim_id, rel, target_ref, target_hash, anchor_text
FROM claim_links
WHERE service_id = ? AND claim_id IN (` + strings.Join(placeholders, ", ") + `)
ORDER BY claim_id, rel, target_ref
`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("ClaimLinks query: %w", err)
	}
	defer rows.Close()
	var result []core.ClaimLink
	for rows.Next() {
		var l core.ClaimLink
		if err := rows.Scan(&l.ClaimID, &l.Rel, &l.TargetRef, &l.TargetHash, &l.AnchorText); err != nil {
			return nil, fmt.Errorf("ClaimLinks scan: %w", err)
		}
		result = append(result, l)
	}
	return result, rows.Err()
}

// UpdateClaimLink re-anchors a single claim_links row in place — see the
// Store interface doc comment. AnchorText is left untouched: a re-anchor only
// fires on an UNAMBIGUOUS match of the already-stored anchor text at its new
// location, so the text itself never changes, only where it points.
func (s *SQLiteStore) UpdateClaimLink(ctx context.Context, serviceID, claimID, rel, oldTargetRef, newTargetRef, newTargetHash string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE claim_links
SET target_ref = ?, target_hash = ?
WHERE service_id = ? AND claim_id = ? AND rel = ? AND target_ref = ?
`, newTargetRef, newTargetHash, serviceID, claimID, rel, oldTargetRef)
	if err != nil {
		return fmt.Errorf("UpdateClaimLink: %w", err)
	}
	return nil
}

// AppendClaimLink inserts ONE link row without touching claim_id's existing
// link set (SU-20, ADR-041 decision 7) — see the Store interface doc
// comment for why this differs from PutClaims' wholesale replace.
func (s *SQLiteStore) AppendClaimLink(ctx context.Context, serviceID string, link core.ClaimLink) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO claim_links (service_id, claim_id, rel, target_ref, target_hash, anchor_text)
VALUES (?, ?, ?, ?, ?, ?)
`, serviceID, link.ClaimID, link.Rel, link.TargetRef, link.TargetHash, link.AnchorText)
	if err != nil {
		return fmt.Errorf("AppendClaimLink: %w", err)
	}
	return nil
}

// SetClaimStatus updates the status of a single claim. No error when absent.
func (s *SQLiteStore) SetClaimStatus(ctx context.Context, serviceID, claimID string, status core.ClaimStatus) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE semantic_claims SET status = ? WHERE service_id = ? AND claim_id = ?
`, string(status), serviceID, claimID)
	if err != nil {
		return fmt.Errorf("SetClaimStatus: %w", err)
	}
	return nil
}

// SetClaimValidTo closes a single claim's bi-temporal validity window at t
// (SU-19, ADR-041 decision 7) — see the Store interface doc comment.
func (s *SQLiteStore) SetClaimValidTo(ctx context.Context, serviceID, claimID string, t time.Time) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE semantic_claims SET valid_to = ? WHERE service_id = ? AND claim_id = ?
`, formatClaimTime(t), serviceID, claimID)
	if err != nil {
		return fmt.Errorf("SetClaimValidTo: %w", err)
	}
	return nil
}

// AppendClaimEvent inserts one verdict row into the append-only event log.
func (s *SQLiteStore) AppendClaimEvent(ctx context.Context, ev core.ClaimEvent) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO claim_events (claim_id, verdict, author, note, event_at, resolution_kind, resolution_commit, asserted_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`, ev.ClaimID, ev.Verdict, ev.Author, ev.Note, ev.At.UnixNano(), ev.ResolutionKind, ev.ResolutionCommit, ev.AssertedBy)
	if err != nil {
		return fmt.Errorf("AppendClaimEvent: %w", err)
	}
	return nil
}

// ClaimEvents returns all events for claimID ordered by (event_at, verdict).
func (s *SQLiteStore) ClaimEvents(ctx context.Context, claimID string) ([]core.ClaimEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT claim_id, verdict, author, note, event_at, resolution_kind, resolution_commit, asserted_by
FROM claim_events
WHERE claim_id = ?
ORDER BY event_at, verdict
`, claimID)
	if err != nil {
		return nil, fmt.Errorf("ClaimEvents query: %w", err)
	}
	defer rows.Close()
	var result []core.ClaimEvent
	for rows.Next() {
		var (
			ev core.ClaimEvent
			at int64
		)
		if err := rows.Scan(&ev.ClaimID, &ev.Verdict, &ev.Author, &ev.Note, &at, &ev.ResolutionKind, &ev.ResolutionCommit, &ev.AssertedBy); err != nil {
			return nil, fmt.Errorf("ClaimEvents scan: %w", err)
		}
		ev.At = time.Unix(0, at).UTC()
		result = append(result, ev)
	}
	return result, rows.Err()
}

// ClaimEventsSince (Q-6) returns every event in [since, until] across every
// claim, ordered by (event_at, verdict) — see the Store interface's own doc
// comment for the store-wide-not-service-scoped caveat (claim_events has no
// service_id column).
func (s *SQLiteStore) ClaimEventsSince(ctx context.Context, since, until time.Time) ([]core.ClaimEvent, error) {
	return queryClaimEventsSince(ctx, s.db, since, until)
}

// queryClaimEventsSince is SQLiteStore.ClaimEventsSince's query body,
// factored into its own free function — a name unique to this file (unlike
// the ClaimEventsSince method name, which SQLiteStore and MemoryStore both
// implement) so a symbol-based tool can anchor to a single, unambiguous
// definition.
func queryClaimEventsSince(ctx context.Context, db *sql.DB, since, until time.Time) ([]core.ClaimEvent, error) {
	rows, err := db.QueryContext(ctx, `
SELECT claim_id, verdict, author, note, event_at, resolution_kind, resolution_commit, asserted_by
FROM claim_events
WHERE event_at >= ? AND event_at <= ?
ORDER BY event_at, verdict
`, since.UnixNano(), until.UnixNano())
	if err != nil {
		return nil, fmt.Errorf("ClaimEventsSince query: %w", err)
	}
	defer rows.Close()
	var result []core.ClaimEvent
	for rows.Next() {
		var (
			ev core.ClaimEvent
			at int64
		)
		if err := rows.Scan(&ev.ClaimID, &ev.Verdict, &ev.Author, &ev.Note, &at, &ev.ResolutionKind, &ev.ResolutionCommit, &ev.AssertedBy); err != nil {
			return nil, fmt.Errorf("ClaimEventsSince scan: %w", err)
		}
		ev.At = time.Unix(0, at).UTC()
		result = append(result, ev)
	}
	return result, rows.Err()
}

// marshalRejectionReasons (Q-3, docs/plans/expert-suite-q.md, O19's fix) is
// RecordCompileRun's JSON-encode step, factored into its own free function —
// a name unique to this file, unlike the RecordCompileRun method name
// (both SQLiteStore and MemoryStore implement it), so a symbol-based tool
// can anchor to a single, unambiguous definition. RejectionReasons is nil
// on the overwhelming majority of runs (a clean compile with zero
// rejections, or a dry run) — marshal nil as "[]" explicitly rather than
// writing a NULL, so unmarshalRejectionReasons never has to special-case
// NULL vs "[]" vs a real array.
func marshalRejectionReasons(rr []core.RejectionBreakdown) (string, error) {
	if len(rr) == 0 {
		// json.Marshal(nil []T) encodes "null", not "[]" — normalize
		// explicitly so the column always holds a valid JSON array.
		return "[]", nil
	}
	data, err := json.Marshal(rr)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// unmarshalRejectionReasons is CompileRuns' JSON-decode step, the sibling
// of marshalRejectionReasons: a legacy row (empty string — the migration
// default, or a row written before this column existed at all) degrades to
// nil, never an error; a malformed value degrades the same lenient way
// every other JSON column in this file already does, rather than aborting
// the whole CompileRuns call.
func unmarshalRejectionReasons(s string) []core.RejectionBreakdown {
	if s == "" || s == "[]" {
		return nil
	}
	var rr []core.RejectionBreakdown
	_ = json.Unmarshal([]byte(s), &rr)
	return rr
}

// marshalStringSlice/unmarshalStringSlice (O54) are marshalRejectionReasons/
// unmarshalRejectionReasons' sibling for CompileRun.UncompiledScopes — same
// "nil marshals to the literal '[]', a legacy/malformed value degrades to
// nil rather than erroring" contract, so a pre-existing store's freshly
// migrated default ('[]') and a genuinely empty slice read back identically.
func marshalStringSlice(ss []string) (string, error) {
	if len(ss) == 0 {
		return "[]", nil
	}
	data, err := json.Marshal(ss)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func unmarshalStringSlice(s string) []string {
	if s == "" || s == "[]" {
		return nil
	}
	var ss []string
	_ = json.Unmarshal([]byte(s), &ss)
	return ss
}

// RecordCompileRun appends a cost/audit ledger row for one compilation pass.
func (s *SQLiteStore) RecordCompileRun(ctx context.Context, run core.CompileRun) error {
	rejectionJSON, err := marshalRejectionReasons(run.RejectionReasons)
	if err != nil {
		return fmt.Errorf("RecordCompileRun: marshal rejection_reasons_json: %w", err)
	}
	uncompiledJSON, err := marshalStringSlice(run.UncompiledScopes)
	if err != nil {
		return fmt.Errorf("RecordCompileRun: marshal uncompiled_scopes_json: %w", err)
	}
	_, err = s.db.ExecContext(
		ctx, `
INSERT INTO compile_runs
    (run_id, service_id, layer, corpus_watermark, model, prompt_version,
     dry_run, tokens_in, tokens_out, cost_usd, duration_ms,
     sections_total, sections_compiled, sections_failed, claims_emitted, claims_rejected, ran_at,
     refused, refusal_reason, budget_stopped, escape_note, rejection_reasons_json,
     resumed_from, uncompiled_scopes_json, sections_prefiltered)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		run.RunID, run.ServiceID, run.Layer, run.CorpusWatermark,
		run.Model, run.PromptVersion, run.DryRun,
		run.TokensIn, run.TokensOut, run.CostUSD, run.DurationMS,
		run.SectionsTotal, run.SectionsCompiled, run.SectionsFailed,
		run.ClaimsEmitted, run.ClaimsRejected,
		run.RanAt.UnixNano(),
		run.Refused, run.RefusalReason, run.BudgetStopped, run.Escape,
		rejectionJSON, run.ResumedFrom, uncompiledJSON, run.SectionsPrefiltered,
	)
	if err != nil {
		return fmt.Errorf("RecordCompileRun: %w", err)
	}
	return nil
}

// CompileRuns returns compile runs for serviceID, optionally filtered by layer
// (layer "" returns all), newest-first by RanAt then run_id.
func (s *SQLiteStore) CompileRuns(ctx context.Context, serviceID, layer string) ([]core.CompileRun, error) {
	const cols = `run_id, service_id, layer, corpus_watermark, model, prompt_version,
	dry_run, tokens_in, tokens_out, cost_usd, duration_ms,
	sections_total, sections_compiled, sections_failed, claims_emitted, claims_rejected, ran_at,
	refused, refusal_reason, budget_stopped, escape_note, rejection_reasons_json,
	resumed_from, uncompiled_scopes_json, sections_prefiltered`
	var (
		rows *sql.Rows
		err  error
	)
	if layer == "" {
		rows, err = s.db.QueryContext(ctx, `
SELECT `+cols+`
FROM compile_runs
WHERE service_id = ?
ORDER BY ran_at DESC, run_id
`, serviceID)
	} else {
		rows, err = s.db.QueryContext(ctx, `
SELECT `+cols+`
FROM compile_runs
WHERE service_id = ? AND layer = ?
ORDER BY ran_at DESC, run_id
`, serviceID, layer)
	}
	if err != nil {
		return nil, fmt.Errorf("CompileRuns query: %w", err)
	}
	defer rows.Close()
	var result []core.CompileRun
	for rows.Next() {
		var (
			r              core.CompileRun
			ranAt          int64
			rejectionJSON  string
			uncompiledJSON string
		)
		if err := rows.Scan(
			&r.RunID, &r.ServiceID, &r.Layer, &r.CorpusWatermark,
			&r.Model, &r.PromptVersion, &r.DryRun,
			&r.TokensIn, &r.TokensOut, &r.CostUSD, &r.DurationMS,
			&r.SectionsTotal, &r.SectionsCompiled, &r.SectionsFailed,
			&r.ClaimsEmitted, &r.ClaimsRejected, &ranAt,
			&r.Refused, &r.RefusalReason, &r.BudgetStopped, &r.Escape,
			&rejectionJSON, &r.ResumedFrom, &uncompiledJSON, &r.SectionsPrefiltered,
		); err != nil {
			return nil, fmt.Errorf("CompileRuns scan: %w", err)
		}
		r.UncompiledScopes = unmarshalStringSlice(uncompiledJSON)
		r.RanAt = time.Unix(0, ranAt).UTC()
		// Q-3: a legacy row (empty string — the pre-existing-store
		// migration default, or a row written before this column existed
		// at all) degrades to nil, never an error; a malformed value
		// (should not happen, but this is a lenient read like every other
		// JSON column in this file) degrades the same way rather than
		// aborting the whole CompileRuns call.
		r.RejectionReasons = unmarshalRejectionReasons(rejectionJSON)
		result = append(result, r)
	}
	return result, rows.Err()
}

// PutTypedEdges replaces all typed call edges for (serviceID, pkgPath) —
// delete-then-insert, keyed at package granularity (see the Store interface
// doc comment).
func (s *SQLiteStore) PutTypedEdges(ctx context.Context, serviceID, pkgPath string, edges []core.Edge) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("PutTypedEdges begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM typed_edges WHERE service_id = ? AND pkg_path = ?`,
		serviceID, pkgPath); err != nil {
		return fmt.Errorf("PutTypedEdges delete: %w", err)
	}

	if len(edges) > 0 {
		stmt, err := tx.PrepareContext(ctx, `
INSERT INTO typed_edges
    (service_id, pkg_path, from_service, from_file, from_symbol, to_symbol, kind)
VALUES (?, ?, ?, ?, ?, ?, ?)
`)
		if err != nil {
			return fmt.Errorf("PutTypedEdges prepare: %w", err)
		}
		defer stmt.Close()
		for _, e := range edges {
			if _, err := stmt.ExecContext(ctx, serviceID, pkgPath,
				e.FromService, e.FromFile, e.FromSymbol, e.ToSymbol, string(e.Kind)); err != nil {
				return fmt.Errorf("PutTypedEdges exec: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("PutTypedEdges commit: %w", err)
	}
	return nil
}

// TypedEdges returns every typed call edge stored for serviceID, ordered by
// (from_symbol, to_symbol, kind).
func (s *SQLiteStore) typedEdgesRaw(ctx context.Context, serviceID string) ([]core.Edge, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT from_service, from_file, from_symbol, to_symbol, kind
FROM typed_edges
WHERE service_id = ?
ORDER BY from_symbol, to_symbol, kind
`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("TypedEdges query: %w", err)
	}
	defer rows.Close()

	var result []core.Edge
	gov := tenancy.NewRowGovernor(ctx, tenancy.ClassIndex) // RT-153
	for rows.Next() {
		if err := gov.Row(); err != nil {
			return nil, err
		}
		var e core.Edge
		var kind string
		if err := rows.Scan(&e.FromService, &e.FromFile, &e.FromSymbol, &e.ToSymbol, &kind); err != nil {
			return nil, fmt.Errorf("TypedEdges scan: %w", err)
		}
		e.Kind = core.EdgeKind(kind)
		result = append(result, e)
	}
	return result, rows.Err()
}

// TypedEdgePackageFingerprint returns the cached content-hash for
// (serviceID, pkgPath). found is false when the package has never been
// loaded.
func (s *SQLiteStore) TypedEdgePackageFingerprint(ctx context.Context, serviceID, pkgPath string) (string, bool, error) {
	var hash string
	err := s.db.QueryRowContext(ctx, `
SELECT content_hash FROM typed_edge_fingerprints
WHERE service_id = ? AND pkg_path = ?
`, serviceID, pkgPath).Scan(&hash)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("TypedEdgePackageFingerprint query: %w", err)
	}
	return hash, true, nil
}

// SetTypedEdgePackageFingerprint records or replaces the content-hash cache
// entry for (serviceID, pkgPath).
func (s *SQLiteStore) SetTypedEdgePackageFingerprint(ctx context.Context, serviceID, pkgPath, hash string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO typed_edge_fingerprints (service_id, pkg_path, content_hash)
VALUES (?, ?, ?)
ON CONFLICT (service_id, pkg_path) DO UPDATE SET content_hash = excluded.content_hash
`, serviceID, pkgPath, hash)
	if err != nil {
		return fmt.Errorf("SetTypedEdgePackageFingerprint: %w", err)
	}
	return nil
}

// ListTypedEdgePackages returns the sorted distinct typed-edge package IDs
// for serviceID (union of typed_edges and typed_edge_fingerprints).
func (s *SQLiteStore) ListTypedEdgePackages(ctx context.Context, serviceID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT pkg_path FROM typed_edges WHERE service_id = ?
UNION
SELECT pkg_path FROM typed_edge_fingerprints WHERE service_id = ?
ORDER BY pkg_path
`, serviceID, serviceID)
	if err != nil {
		return nil, fmt.Errorf("ListTypedEdgePackages: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("ListTypedEdgePackages scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DeleteTypedEdgePackage removes all typed-edge rows and the fingerprint
// row for (serviceID, pkgPath). Missing rows are a no-op.
func (s *SQLiteStore) DeleteTypedEdgePackage(ctx context.Context, serviceID, pkgPath string) error {
	for _, q := range []string{
		`DELETE FROM typed_edges WHERE service_id = ? AND pkg_path = ?`,
		`DELETE FROM typed_edge_fingerprints WHERE service_id = ? AND pkg_path = ?`,
	} {
		if _, err := s.db.ExecContext(ctx, q, serviceID, pkgPath); err != nil {
			return fmt.Errorf("DeleteTypedEdgePackage: %w", err)
		}
	}
	return nil
}

// Close releases the underlying database connection pool.
func (s *SQLiteStore) Close() error {
	atomic.AddInt64(&openStoreCount, -1)
	return s.db.Close()
}

// scanObservations scans a full observations row-set (the shared column
// order QueryObservations/ListSourceRefs' callers rely on). Backend-agnostic
// (operates on *sql.Rows only) — kept here as SQLiteStore's sole consumer
// since SS-8 deleted the DuckStore backend that used to share it.
// scanObservations decodes observation rows, asking the producer governor
// every 1,000 rows (RT-153): a tree- or fleet-class scan halts at refuse,
// any scan at the exit mark.
func scanObservations(ctx context.Context, rows *sql.Rows) ([]core.Observation, error) {
	var result []core.Observation
	gov := tenancy.NewRowGovernor(ctx, tenancy.ClassIndex)
	for rows.Next() {
		if err := gov.Row(); err != nil {
			return nil, err
		}
		var (
			o          core.Observation
			callsJSON  string
			ingestedAt int64
		)
		if err := rows.Scan(
			&o.ServiceID,
			&o.ConcernID,
			&o.SourceRef,
			(*string)(&o.SourceKind),
			(*string)(&o.EvidenceTier),
			&o.Symbol,
			&o.Package,
			&o.Claim,
			&o.ContentHash,
			&callsJSON,
			&o.Complexity,
			&o.Line,
			&o.EndLine,
			&ingestedAt,
			&o.ToolKind,
			&o.Generated,
		); err != nil {
			return nil, fmt.Errorf("scanObservations scan: %w", err)
		}
		if err := json.Unmarshal([]byte(callsJSON), &o.Calls); err != nil {
			return nil, fmt.Errorf("scanObservations unmarshal calls: %w", err)
		}
		o.IngestedAt = time.Unix(0, ingestedAt).UTC()
		result = append(result, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scanObservations rows: %w", err)
	}
	return result, nil
}

// scanClaims scans a full semantic_claims row-set (the shared column order
// ClaimsForService's callers rely on). Backend-agnostic — see scanObservations.
func scanClaims(rows *sql.Rows) ([]core.SemanticClaim, error) {
	var result []core.SemanticClaim
	for rows.Next() {
		var (
			c             core.SemanticClaim
			scopeJSON     string
			compiledAt    int64
			validFromText string
			validToText   string
			metaText      string
		)
		if err := rows.Scan(
			&c.ServiceID, &c.ClaimID,
			&c.Layer, &c.LayerVersion, &c.Kind, &c.Subject, &c.Rule, &scopeJSON,
			(*string)(&c.Tier), &c.TierProfile,
			(*string)(&c.Status),
			&c.Model, &c.PromptVersion, &c.ReproKey, &c.InputDigest,
			&c.CompiledAtWatermark, &compiledAt, (*string)(&c.Class),
			&validFromText, &validToText, &c.AssertedBy, &metaText,
		); err != nil {
			return nil, fmt.Errorf("scanClaims scan: %w", err)
		}
		if err := json.Unmarshal([]byte(scopeJSON), &c.Scope); err != nil {
			return nil, fmt.Errorf("scanClaims unmarshal scope: %w", err)
		}
		c.CompiledAt = time.Unix(0, compiledAt).UTC()
		c.ValidFrom = parseClaimTime(validFromText)
		c.ValidTo = parseClaimTime(validToText)
		if metaText != "" {
			c.Meta = json.RawMessage(metaText)
		}
		result = append(result, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scanClaims rows: %w", err)
	}
	return result, nil
}

// formatClaimTime renders a claim's ValidFrom/ValidTo (SU-19) as the store's
// nullable-RFC3339-text column: the zero time.Time{} — "unset" — stores as
// "", exactly like every other additive claim column's empty-string
// default; a real time stores as RFC3339Nano (UnixNano-precision round
// trip, matching CompiledAt's own UnixNano storage).
func formatClaimTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// parseClaimTime is formatClaimTime's inverse. An empty or unparseable
// column value reads back as the zero time.Time{} — "unset", never an
// error: a legacy row predating valid_from/valid_to must read exactly like
// a row that was written with an explicitly empty value (core.SemanticClaim.
// ValidAt's own "zero means unconstrained" doc comment depends on this).
func parseClaimTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}
