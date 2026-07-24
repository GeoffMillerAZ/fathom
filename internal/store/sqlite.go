package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// SQLiteStore is a pure-Go, SQLite-backed (modernc.org/sqlite, WAL mode)
// implementation of Store (ADR-020). Unlike DuckStore it does not cap the
// connection pool: SQLite's own single-writer lock serialises writes, and
// WAL mode lets any number of readers proceed concurrently with one writer —
// the property DuckDB's cross-process single-writer lacked and the reason
// for this backend's existence.
type SQLiteStore struct {
	db *sql.DB
}

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
// this backend (only DuckStore carries that, for stores that predate it).
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
	// SetMaxOpenConns(1) DuckStore uses to serialise writes is
	// DuckDB-specific and must NOT carry over; SQLite's own single-writer
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

	s := &SQLiteStore{db: db}
	if err := s.createSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store.NewSQLiteStore schema: %w", err)
	}
	return s, nil
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
    PRIMARY KEY (service_id, claim_id)
);

CREATE TABLE IF NOT EXISTS claim_links (
    service_id  TEXT NOT NULL DEFAULT '',
    claim_id    TEXT NOT NULL DEFAULT '',
    rel         TEXT NOT NULL DEFAULT '',
    target_ref  TEXT NOT NULL DEFAULT '',
    target_hash TEXT NOT NULL DEFAULT ''
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
    escape_note        TEXT    NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS claim_events (
    claim_id TEXT   NOT NULL DEFAULT '',
    verdict  TEXT   NOT NULL DEFAULT '',
    author   TEXT   NOT NULL DEFAULT '',
    note     TEXT   NOT NULL DEFAULT '',
    event_at BIGINT NOT NULL DEFAULT 0
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
		{"compile_runs", "escape_note", `ALTER TABLE compile_runs ADD COLUMN escape_note TEXT NOT NULL DEFAULT ''`},
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
     symbol, package, claim, content_hash, calls, complexity, line, ingested_at,
     tool_kind, generated)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
func (s *SQLiteStore) QueryObservations(ctx context.Context, serviceID, concernID string) ([]core.Observation, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if concernID == "" {
		rows, err = s.db.QueryContext(ctx, `
SELECT service_id, concern_id, source_ref, source_kind, evidence_tier,
       symbol, package, claim, content_hash, calls, complexity, line, ingested_at,
       tool_kind, generated
FROM observations
WHERE service_id = ?
ORDER BY ingested_at
`, serviceID)
	} else {
		rows, err = s.db.QueryContext(ctx, `
SELECT service_id, concern_id, source_ref, source_kind, evidence_tier,
       symbol, package, claim, content_hash, calls, complexity, line, ingested_at,
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

	return scanObservations(rows)
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
	} {
		if _, err := s.db.ExecContext(ctx, q, serviceID); err != nil {
			return fmt.Errorf("PurgeService: %w", err)
		}
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
func (s *SQLiteStore) ListSourceRefs(ctx context.Context, serviceID string) ([]string, error) {
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
func (s *SQLiteStore) KnowledgeCards(ctx context.Context, serviceID string) ([]KnowledgeCard, error) {
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
func (s *SQLiteStore) DocSummaries(ctx context.Context, serviceID string) ([]DocSummary, error) {
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
     compiled_at_watermark, compiled_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
    compiled_at           = excluded.compiled_at
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
			_, err = stmt.ExecContext(
				ctx,
				c.ServiceID, c.ClaimID,
				c.Layer, c.LayerVersion, c.Kind, c.Subject, c.Rule, string(scopeJSON),
				string(c.Tier), c.TierProfile,
				string(c.Status),
				c.Model, c.PromptVersion, c.ReproKey, c.InputDigest,
				c.CompiledAtWatermark, c.CompiledAt.UnixNano(),
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
INSERT INTO claim_links (service_id, claim_id, rel, target_ref, target_hash)
VALUES (?, ?, ?, ?, ?)
`)
		if err != nil {
			return fmt.Errorf("PutClaims prepare links: %w", err)
		}
		defer lstmt.Close()
		for _, l := range links {
			if _, err := lstmt.ExecContext(ctx, svcOf[l.ClaimID], l.ClaimID, l.Rel, l.TargetRef, l.TargetHash); err != nil {
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
	compiled_at_watermark, compiled_at`
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

// ClaimLinks returns links for the given claim IDs within serviceID, ordered
// by (claim_id, rel, target_ref). Returns nil with no error for empty claimIDs.
func (s *SQLiteStore) ClaimLinks(ctx context.Context, serviceID string, claimIDs []string) ([]core.ClaimLink, error) {
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
SELECT claim_id, rel, target_ref, target_hash
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
		if err := rows.Scan(&l.ClaimID, &l.Rel, &l.TargetRef, &l.TargetHash); err != nil {
			return nil, fmt.Errorf("ClaimLinks scan: %w", err)
		}
		result = append(result, l)
	}
	return result, rows.Err()
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

// AppendClaimEvent inserts one verdict row into the append-only event log.
func (s *SQLiteStore) AppendClaimEvent(ctx context.Context, ev core.ClaimEvent) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO claim_events (claim_id, verdict, author, note, event_at)
VALUES (?, ?, ?, ?, ?)
`, ev.ClaimID, ev.Verdict, ev.Author, ev.Note, ev.At.UnixNano())
	if err != nil {
		return fmt.Errorf("AppendClaimEvent: %w", err)
	}
	return nil
}

// ClaimEvents returns all events for claimID ordered by (event_at, verdict).
func (s *SQLiteStore) ClaimEvents(ctx context.Context, claimID string) ([]core.ClaimEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT claim_id, verdict, author, note, event_at
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
		if err := rows.Scan(&ev.ClaimID, &ev.Verdict, &ev.Author, &ev.Note, &at); err != nil {
			return nil, fmt.Errorf("ClaimEvents scan: %w", err)
		}
		ev.At = time.Unix(0, at).UTC()
		result = append(result, ev)
	}
	return result, rows.Err()
}

// RecordCompileRun appends a cost/audit ledger row for one compilation pass.
func (s *SQLiteStore) RecordCompileRun(ctx context.Context, run core.CompileRun) error {
	_, err := s.db.ExecContext(
		ctx, `
INSERT INTO compile_runs
    (run_id, service_id, layer, corpus_watermark, model, prompt_version,
     dry_run, tokens_in, tokens_out, cost_usd, duration_ms,
     sections_total, sections_compiled, sections_failed, claims_emitted, claims_rejected, ran_at,
     refused, refusal_reason, budget_stopped, escape_note)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		run.RunID, run.ServiceID, run.Layer, run.CorpusWatermark,
		run.Model, run.PromptVersion, run.DryRun,
		run.TokensIn, run.TokensOut, run.CostUSD, run.DurationMS,
		run.SectionsTotal, run.SectionsCompiled, run.SectionsFailed,
		run.ClaimsEmitted, run.ClaimsRejected,
		run.RanAt.UnixNano(),
		run.Refused, run.RefusalReason, run.BudgetStopped, run.Escape,
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
	refused, refusal_reason, budget_stopped, escape_note`
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
			r     core.CompileRun
			ranAt int64
		)
		if err := rows.Scan(
			&r.RunID, &r.ServiceID, &r.Layer, &r.CorpusWatermark,
			&r.Model, &r.PromptVersion, &r.DryRun,
			&r.TokensIn, &r.TokensOut, &r.CostUSD, &r.DurationMS,
			&r.SectionsTotal, &r.SectionsCompiled, &r.SectionsFailed,
			&r.ClaimsEmitted, &r.ClaimsRejected, &ranAt,
			&r.Refused, &r.RefusalReason, &r.BudgetStopped, &r.Escape,
		); err != nil {
			return nil, fmt.Errorf("CompileRuns scan: %w", err)
		}
		r.RanAt = time.Unix(0, ranAt).UTC()
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
func (s *SQLiteStore) TypedEdges(ctx context.Context, serviceID string) ([]core.Edge, error) {
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
	for rows.Next() {
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
	return s.db.Close()
}

// scanObservations scans a full observations row-set (the shared column
// order QueryObservations/ListSourceRefs' callers rely on). Backend-agnostic
// (operates on *sql.Rows only) — kept here as SQLiteStore's sole consumer
// since SS-8 deleted the DuckStore backend that used to share it.
func scanObservations(rows *sql.Rows) ([]core.Observation, error) {
	var result []core.Observation
	for rows.Next() {
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
			c          core.SemanticClaim
			scopeJSON  string
			compiledAt int64
		)
		if err := rows.Scan(
			&c.ServiceID, &c.ClaimID,
			&c.Layer, &c.LayerVersion, &c.Kind, &c.Subject, &c.Rule, &scopeJSON,
			(*string)(&c.Tier), &c.TierProfile,
			(*string)(&c.Status),
			&c.Model, &c.PromptVersion, &c.ReproKey, &c.InputDigest,
			&c.CompiledAtWatermark, &compiledAt,
		); err != nil {
			return nil, fmt.Errorf("scanClaims scan: %w", err)
		}
		if err := json.Unmarshal([]byte(scopeJSON), &c.Scope); err != nil {
			return nil, fmt.Errorf("scanClaims unmarshal scope: %w", err)
		}
		c.CompiledAt = time.Unix(0, compiledAt).UTC()
		result = append(result, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scanClaims rows: %w", err)
	}
	return result, nil
}
