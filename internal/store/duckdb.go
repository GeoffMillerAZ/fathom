package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/marcboeker/go-duckdb/v2"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// DuckStore is a DuckDB-backed implementation of Store.
// A single writer connection is used (SetMaxOpenConns(1)) so that concurrent
// writes are serialised without explicit mutexes.
type DuckStore struct {
	db *sql.DB
}

// OpenDuckDB opens (or creates) a DuckDB database at path. Pass "" for an
// in-memory database. The schema is created if it does not already exist.
//
// Prefer store.Open (or store.BackendForPath) for callers that should honor
// the ADR-020 extension dispatch; call OpenDuckDB directly only when the
// DuckDB backend specifically is required (e.g. migrate-store's source side,
// or this backend's own contract tests).
func OpenDuckDB(path string) (*DuckStore, error) {
	// Ensure the parent directory exists (e.g. the default workspace/db/);
	// DuckDB cannot create a file in a non-existent directory. "" = in-memory.
	if path != "" {
		if dir := filepath.Dir(path); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("store.OpenDuckDB mkdir %q: %w", dir, err)
			}
		}
	}
	db, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, fmt.Errorf("store.OpenDuckDB: %w", err)
	}
	// Single writer prevents concurrent-write conflicts in DuckDB. This is a
	// DuckDB-specific serialization requirement — SQLiteStore deliberately
	// does NOT carry it over (ADR-020): SQLite's own single-writer lock
	// already serialises writes, and capping its pool would throttle the
	// concurrent readers that are the whole point of the swap.
	db.SetMaxOpenConns(1)

	s := &DuckStore{db: db}
	if err := s.createSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store.OpenDuckDB schema: %w", err)
	}
	return s, nil
}

// openDuckDBReadOnly opens an EXISTING DuckDB database at path read-only
// (go-duckdb passes DSN query parameters straight to DuckDB's config, so
// access_mode=read_only engages the engine's own read-only mode) and runs
// NO schema creation or migration DDL. This is migrate-store's source-side
// open (F4a): OpenDuckDB would execute ALTER TABLE migrations against the
// very file the operator is trying to leave byte-identical. A store so old
// it predates a column the readers SELECT will fail loudly here rather
// than be silently altered — open it once read-write with fathom first to
// migrate its schema, then retry.
func openDuckDBReadOnly(path string) (*DuckStore, error) {
	db, err := sql.Open("duckdb", path+"?access_mode=read_only")
	if err != nil {
		return nil, fmt.Errorf("store.openDuckDBReadOnly: %w", err)
	}
	db.SetMaxOpenConns(1)
	// sql.Open is lazy; force the actual file open now so a missing file or
	// a held write lock surfaces here, not on the first table read.
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store.openDuckDBReadOnly %q: %w", path, err)
	}
	return &DuckStore{db: db}, nil
}

// createSchema idempotently creates the required tables.
func (s *DuckStore) createSchema() error {
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
    generated     BOOLEAN NOT NULL DEFAULT FALSE
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
    budget_stopped     BOOLEAN NOT NULL DEFAULT FALSE
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

-- Per-package content-hash cache: skip reloading a package via go/packages
-- when its file-set hash is unchanged since the last successful load.
CREATE TABLE IF NOT EXISTS typed_edge_fingerprints (
    service_id   TEXT NOT NULL,
    pkg_path     TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    PRIMARY KEY (service_id, pkg_path)
);
`
	if _, err := s.db.Exec(ddl); err != nil {
		return err
	}
	// Idempotent migrations: add columns to existing DBs that predate them.
	// DuckDB supports ADD COLUMN IF NOT EXISTS but NOT NULL + DEFAULT constraints
	// are not yet allowed in that form, so we add them with a plain DEFAULT.
	if _, err := s.db.Exec(`ALTER TABLE observations ADD COLUMN IF NOT EXISTS line INTEGER DEFAULT 0`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`ALTER TABLE observations ADD COLUMN IF NOT EXISTS package TEXT DEFAULT ''`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`ALTER TABLE compile_runs ADD COLUMN IF NOT EXISTS sections_failed INTEGER DEFAULT 0`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`ALTER TABLE compile_runs ADD COLUMN IF NOT EXISTS refused BOOLEAN DEFAULT FALSE`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`ALTER TABLE compile_runs ADD COLUMN IF NOT EXISTS refusal_reason TEXT DEFAULT ''`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`ALTER TABLE compile_runs ADD COLUMN IF NOT EXISTS budget_stopped BOOLEAN DEFAULT FALSE`); err != nil {
		return err
	}
	// ADR-021 §2 (critic F1): tool identity + generated marker on
	// observations — legacy stores predate both columns.
	if _, err := s.db.Exec(`ALTER TABLE observations ADD COLUMN IF NOT EXISTS tool_kind TEXT DEFAULT ''`); err != nil {
		return err
	}
	_, err := s.db.Exec(`ALTER TABLE observations ADD COLUMN IF NOT EXISTS generated BOOLEAN DEFAULT FALSE`)
	return err
}

// PutObservations inserts a batch of observations via a bare append — see
// the Store interface doc comment: this does NOT dedupe or replace by key.
// Callers that want replace semantics must call PurgeSource first (as
// internal/fusion does).
func (s *DuckStore) PutObservations(ctx context.Context, obs []core.Observation) error {
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
func (s *DuckStore) QueryObservations(ctx context.Context, serviceID, concernID string) ([]core.Observation, error) {
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

// Fingerprint returns the stored content hash for (serviceID, sourceRef).
func (s *DuckStore) Fingerprint(ctx context.Context, serviceID, sourceRef string) (string, bool, error) {
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
func (s *DuckStore) SetFingerprint(ctx context.Context, serviceID, sourceRef, hash string) error {
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
func (s *DuckStore) PurgeSource(ctx context.Context, serviceID, sourceRef string) error {
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
func (s *DuckStore) PurgeService(ctx context.Context, serviceID string) error {
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
func (s *DuckStore) CountAbsoluteSourceRefs(ctx context.Context, serviceID string) (int, error) {
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
func (s *DuckStore) DeleteFingerprint(ctx context.Context, serviceID, sourceRef string) error {
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
func (s *DuckStore) ListSourceRefs(ctx context.Context, serviceID string) ([]string, error) {
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
func (s *DuckStore) PutKnowledgeCards(ctx context.Context, cards []KnowledgeCard) error {
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
func (s *DuckStore) KnowledgeCards(ctx context.Context, serviceID string) ([]KnowledgeCard, error) {
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
func (s *DuckStore) PutDocSummaries(ctx context.Context, summaries []DocSummary) error {
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
func (s *DuckStore) DocSummaries(ctx context.Context, serviceID string) ([]DocSummary, error) {
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
func (s *DuckStore) ListServiceIDs(ctx context.Context) ([]string, error) {
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

// SetServiceMeta upserts the ingest metadata for svc.ServiceID.
func (s *DuckStore) SetServiceMeta(ctx context.Context, meta ServiceMeta) error {
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
func (s *DuckStore) GetServiceMeta(ctx context.Context, serviceID string) (ServiceMeta, bool, error) {
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
func (s *DuckStore) SetCommits(ctx context.Context, serviceID string, commits []Commit) error {
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
func (s *DuckStore) ListCommits(ctx context.Context, serviceID string) ([]Commit, error) {
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
func (s *DuckStore) PutClaims(ctx context.Context, claims []core.SemanticClaim, links []core.ClaimLink) error {
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
func (s *DuckStore) ClaimsForService(ctx context.Context, serviceID, layer string) ([]core.SemanticClaim, error) {
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

// ClaimLinks returns links for the given claim IDs within serviceID, ordered
// by (claim_id, rel, target_ref). Returns nil with no error for empty claimIDs.
func (s *DuckStore) ClaimLinks(ctx context.Context, serviceID string, claimIDs []string) ([]core.ClaimLink, error) {
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
func (s *DuckStore) SetClaimStatus(ctx context.Context, serviceID, claimID string, status core.ClaimStatus) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE semantic_claims SET status = ? WHERE service_id = ? AND claim_id = ?
`, string(status), serviceID, claimID)
	if err != nil {
		return fmt.Errorf("SetClaimStatus: %w", err)
	}
	return nil
}

// AppendClaimEvent inserts one verdict row into the append-only event log.
func (s *DuckStore) AppendClaimEvent(ctx context.Context, ev core.ClaimEvent) error {
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
func (s *DuckStore) ClaimEvents(ctx context.Context, claimID string) ([]core.ClaimEvent, error) {
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
func (s *DuckStore) RecordCompileRun(ctx context.Context, run core.CompileRun) error {
	_, err := s.db.ExecContext(
		ctx, `
INSERT INTO compile_runs
    (run_id, service_id, layer, corpus_watermark, model, prompt_version,
     dry_run, tokens_in, tokens_out, cost_usd, duration_ms,
     sections_total, sections_compiled, sections_failed, claims_emitted, claims_rejected, ran_at,
     refused, refusal_reason, budget_stopped)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		run.RunID, run.ServiceID, run.Layer, run.CorpusWatermark,
		run.Model, run.PromptVersion, run.DryRun,
		run.TokensIn, run.TokensOut, run.CostUSD, run.DurationMS,
		run.SectionsTotal, run.SectionsCompiled, run.SectionsFailed,
		run.ClaimsEmitted, run.ClaimsRejected,
		run.RanAt.UnixNano(),
		run.Refused, run.RefusalReason, run.BudgetStopped,
	)
	if err != nil {
		return fmt.Errorf("RecordCompileRun: %w", err)
	}
	return nil
}

// CompileRuns returns compile runs for serviceID, optionally filtered by layer
// (layer "" returns all), newest-first by RanAt then run_id.
func (s *DuckStore) CompileRuns(ctx context.Context, serviceID, layer string) ([]core.CompileRun, error) {
	const cols = `run_id, service_id, layer, corpus_watermark, model, prompt_version,
	dry_run, tokens_in, tokens_out, cost_usd, duration_ms,
	sections_total, sections_compiled, sections_failed, claims_emitted, claims_rejected, ran_at,
	refused, refusal_reason, budget_stopped`
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
			&r.Refused, &r.RefusalReason, &r.BudgetStopped,
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
func (s *DuckStore) PutTypedEdges(ctx context.Context, serviceID, pkgPath string, edges []core.Edge) error {
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
func (s *DuckStore) TypedEdges(ctx context.Context, serviceID string) ([]core.Edge, error) {
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
func (s *DuckStore) TypedEdgePackageFingerprint(ctx context.Context, serviceID, pkgPath string) (string, bool, error) {
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
func (s *DuckStore) SetTypedEdgePackageFingerprint(ctx context.Context, serviceID, pkgPath, hash string) error {
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
func (s *DuckStore) ListTypedEdgePackages(ctx context.Context, serviceID string) ([]string, error) {
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
func (s *DuckStore) DeleteTypedEdgePackage(ctx context.Context, serviceID, pkgPath string) error {
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
func (s *DuckStore) Close() error {
	return s.db.Close()
}
