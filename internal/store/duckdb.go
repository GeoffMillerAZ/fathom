package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/marcboeker/go-duckdb/v2"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// DuckStore is a DuckDB-backed implementation of Store.
// A single writer connection is used (SetMaxOpenConns(1)) so that concurrent
// writes are serialised without explicit mutexes.
type DuckStore struct {
	db *sql.DB
}

// Open opens (or creates) a DuckDB database at path. Pass "" for an in-memory
// database. The schema is created if it does not already exist.
func Open(path string) (*DuckStore, error) {
	// Ensure the parent directory exists (e.g. the default workspace/db/);
	// DuckDB cannot create a file in a non-existent directory. "" = in-memory.
	if path != "" {
		if dir := filepath.Dir(path); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("store.Open mkdir %q: %w", dir, err)
			}
		}
	}
	db, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, fmt.Errorf("store.Open: %w", err)
	}
	// Single writer prevents concurrent-write conflicts in DuckDB.
	db.SetMaxOpenConns(1)

	s := &DuckStore{db: db}
	if err := s.createSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store.Open schema: %w", err)
	}
	return s, nil
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
    ingested_at   BIGINT  NOT NULL
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
	_, err := s.db.Exec(`ALTER TABLE observations ADD COLUMN IF NOT EXISTS package TEXT DEFAULT ''`)
	return err
}

// PutObservations inserts a batch of observations. Existing rows for the same
// (service_id, source_ref, symbol) are replaced.
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
     symbol, package, claim, content_hash, calls, complexity, line, ingested_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		_, err = stmt.ExecContext(ctx,
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
       symbol, package, claim, content_hash, calls, complexity, line, ingested_at
FROM observations
WHERE service_id = ?
ORDER BY ingested_at
`, serviceID)
	} else {
		rows, err = s.db.QueryContext(ctx, `
SELECT service_id, concern_id, source_ref, source_kind, evidence_tier,
       symbol, package, claim, content_hash, calls, complexity, line, ingested_at
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

// Close releases the underlying database connection pool.
func (s *DuckStore) Close() error {
	return s.db.Close()
}
