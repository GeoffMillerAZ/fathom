package store

// filechurn.go — file_churn (TQ-1, docs/plans/test-intelligence.md; design
// doc docs/design/test-intelligence.md §3 T1). Churn is a git fact —
// commits touching a file in the 180 days before the ingest that computed
// it (internal/fusion.computeFileChurn) — over the service's OWN commits
// rows (SetCommits), never recomputed at request time (I48, RT-76's
// "whole-repository facts come from ingest" doctrine derived.go documents
// for file_stats/service_summary). It follows the "commits" table's own
// shape and replace contract exactly (a plain per-service wholesale
// replace, not the heavier file_stats/service_summary transaction: churn
// depends only on commits, which PurgeService already keeps as an identity
// row, so churn is left alone there too and simply recomputed whole on the
// next ingest) — same additive-table convention review_runs, ingest_state,
// file_imports and the rest of sqlite.go's createSchema already follow: a
// new table is its own const and its own createXSchema, the main ddl
// block is never edited for it.

import (
	"context"
	"fmt"
	"sort"
)

// FileChurn is one file_churn row: how many of the service's stored
// commits touched Path within the T1 churn window before the ingest that
// computed it.
type FileChurn struct {
	Path        string
	Commits180d int
}

const fileChurnDDL = `
CREATE TABLE IF NOT EXISTS file_churn (
    service_id    TEXT    NOT NULL,
    path          TEXT    NOT NULL,
    commits_180d  INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (service_id, path)
);
`

func (s *SQLiteStore) createFileChurnSchema() error {
	_, err := s.db.Exec(fileChurnDDL)
	return err
}

// SetFileChurn replaces all file_churn rows for serviceID with churn —
// the same delete-then-insert, wholesale-replace contract SetCommits
// gives the commits it is derived from.
func (s *SQLiteStore) SetFileChurn(ctx context.Context, serviceID string, churn []FileChurn) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("SetFileChurn begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM file_churn WHERE service_id = ?`, serviceID); err != nil {
		return fmt.Errorf("SetFileChurn delete: %w", err)
	}
	if len(churn) > 0 {
		stmt, err := tx.PrepareContext(ctx, `
INSERT INTO file_churn (service_id, path, commits_180d)
VALUES (?, ?, ?)`)
		if err != nil {
			return fmt.Errorf("SetFileChurn prepare: %w", err)
		}
		defer stmt.Close()
		for _, c := range churn {
			if _, err := stmt.ExecContext(ctx, serviceID, c.Path, c.Commits180d); err != nil {
				return fmt.Errorf("SetFileChurn insert: %w", err)
			}
		}
	}
	return tx.Commit()
}

// FileChurn returns serviceID's file_churn rows sorted by path. Returns a
// nil slice with no error when none are stored.
func (s *SQLiteStore) FileChurn(ctx context.Context, serviceID string) ([]FileChurn, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT path, commits_180d FROM file_churn WHERE service_id = ? ORDER BY path`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("FileChurn query: %w", err)
	}
	defer rows.Close()
	var out []FileChurn
	for rows.Next() {
		var c FileChurn
		if err := rows.Scan(&c.Path, &c.Commits180d); err != nil {
			return nil, fmt.Errorf("FileChurn scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetFileChurn replaces all file_churn rows for serviceID with a copy
// sorted by path, mirroring MemoryStore.SetCommits' replace contract.
func (m *MemoryStore) SetFileChurn(_ context.Context, serviceID string, churn []FileChurn) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]FileChurn, len(churn))
	copy(cp, churn)
	sort.Slice(cp, func(i, j int) bool { return cp[i].Path < cp[j].Path })
	m.fileChurn[serviceID] = cp
	return nil
}

// FileChurn returns serviceID's file_churn rows sorted by path.
func (m *MemoryStore) FileChurn(_ context.Context, serviceID string) ([]FileChurn, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	src := m.fileChurn[serviceID]
	if len(src) == 0 {
		return nil, nil
	}
	out := make([]FileChurn, len(src))
	copy(out, src)
	return out, nil
}
