package store

// sqlite_services.go — SQLiteStore's per-service lifecycle and identity:
// purge, remove and rename (serviceIDTables), the content digest (and
// writeDigestRow, which MemoryStore shares), service-ID listings, service
// meta and commits. Split out of sqlite.go (wave g1).

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
)

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
		// Derived from the content above (RT-76, RT-77): rebuilt by the
		// next ingest or backfill.
		`DELETE FROM file_stats WHERE service_id = ?`,
		`DELETE FROM service_summary WHERE service_id = ?`,
		`DELETE FROM observation_calls WHERE service_id = ?`,
		// RT-89: extracted routes and client calls; the next ingest
		// re-extracts every file.
		`DELETE FROM service_routes WHERE service_id = ?`,
		`DELETE FROM service_http_calls WHERE service_id = ?`,
		`DELETE FROM service_refs_state WHERE service_id = ?`,
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
	"file_stats",
	"service_summary",
	"observation_calls",
	"observation_calls_state",
	"service_routes",
	"service_http_calls",
	"service_refs_state",
	"file_imports",
	"file_churn",
	"symbol_centrality",
	"arch_violation_trend",
	// KR-1/KR-8: the doc index. docs_fts is an FTS5 virtual table with the
	// same service_id column; doc_links is keyed by (service_id, unit_id).
	"doc_units",
	"doc_links",
	"docs_fts",
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
// Commits sharing a date (git dates are whole seconds) keep the order
// SetCommits was given — git log's, a child before its parent: SetCommits
// inserts in that order after deleting the service's rows, so rowid breaks
// the tie. Without it the tie fell to the primary key's hash order.
func (s *SQLiteStore) ListCommits(ctx context.Context, serviceID string) ([]Commit, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT service_id, hash, author, date, message, files, spec_ids
		 FROM commits WHERE service_id = ? ORDER BY date DESC, rowid ASC`, serviceID)
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
