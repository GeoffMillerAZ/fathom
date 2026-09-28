package store

// centrality.go — symbol_centrality (TQ-2, docs/plans/test-intelligence.md;
// design doc docs/design/test-intelligence.md §1 owner decision and §3
// T1): per symbol, its transitive caller count and that count normalized
// to [0, 1] within the service, computed at ingest by
// internal/fusion.computeCentrality over the SCC condensation of the call
// graph and memoized per watermark (the ingest skips the recompute when
// the stored watermark matches and nothing changed). Never computed at
// request time (I48): a T1 risk read is a sort over these stored rows.
//
// Same additive-table convention as file_churn (filechurn.go): its own
// DDL const and createXSchema, a wholesale per-service replace. The read
// carries paths, so it is FILTERED through the suppression gate on File
// (internal/scope surface_test.go's disposition), unlike file_churn.

import (
	"context"
	"fmt"
	"sort"
)

// SymbolCentrality is one symbol_centrality row.
type SymbolCentrality struct {
	Symbol     string  // canonical symbol key
	File       string  // repo-relative file the symbol is declared in
	Callers    int     // distinct transitive callers
	Centrality float64 // Callers / the service's maximum Callers, in [0, 1]
}

const symbolCentralityDDL = `
CREATE TABLE IF NOT EXISTS symbol_centrality (
    service_id  TEXT    NOT NULL,
    symbol      TEXT    NOT NULL,
    file        TEXT    NOT NULL DEFAULT '',
    callers     INTEGER NOT NULL DEFAULT 0,
    centrality  REAL    NOT NULL DEFAULT 0,
    watermark   TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (service_id, symbol)
);
`

func (s *SQLiteStore) createSymbolCentralitySchema() error {
	_, err := s.db.Exec(symbolCentralityDDL)
	return err
}

// SetSymbolCentrality replaces all symbol_centrality rows for serviceID,
// stamping each with watermark (the memo key the next ingest compares).
func (s *SQLiteStore) SetSymbolCentrality(ctx context.Context, serviceID, watermark string, rows []SymbolCentrality) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("SetSymbolCentrality begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM symbol_centrality WHERE service_id = ?`, serviceID); err != nil {
		return fmt.Errorf("SetSymbolCentrality delete: %w", err)
	}
	if len(rows) > 0 {
		stmt, err := tx.PrepareContext(ctx, `
INSERT INTO symbol_centrality (service_id, symbol, file, callers, centrality, watermark)
VALUES (?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return fmt.Errorf("SetSymbolCentrality prepare: %w", err)
		}
		defer stmt.Close()
		for _, r := range rows {
			if _, err := stmt.ExecContext(ctx, serviceID, r.Symbol, r.File, r.Callers, r.Centrality, watermark); err != nil {
				return fmt.Errorf("SetSymbolCentrality insert: %w", err)
			}
		}
	}
	return tx.Commit()
}

// SymbolCentrality returns serviceID's rows sorted by symbol and the
// watermark they were computed at ("" when none are stored). Rows in a
// suppressed file are dropped.
func (s *SQLiteStore) SymbolCentrality(ctx context.Context, serviceID string) (string, []SymbolCentrality, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT symbol, file, callers, centrality, watermark FROM symbol_centrality WHERE service_id = ? ORDER BY symbol`, serviceID)
	if err != nil {
		return "", nil, fmt.Errorf("SymbolCentrality query: %w", err)
	}
	defer rows.Close()
	var out []SymbolCentrality
	watermark := ""
	for rows.Next() {
		var r SymbolCentrality
		if err := rows.Scan(&r.Symbol, &r.File, &r.Callers, &r.Centrality, &watermark); err != nil {
			return "", nil, fmt.Errorf("SymbolCentrality scan: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return "", nil, err
	}
	out, err = filterScoped(ctx, s.gate, serviceID, out, nil, func(r SymbolCentrality) string { return r.File })
	return watermark, out, err
}

type centralityEntry struct {
	watermark string
	rows      []SymbolCentrality
}

// SetSymbolCentrality replaces serviceID's rows with a copy sorted by
// symbol.
func (m *MemoryStore) SetSymbolCentrality(_ context.Context, serviceID, watermark string, rows []SymbolCentrality) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]SymbolCentrality, len(rows))
	copy(cp, rows)
	sort.Slice(cp, func(i, j int) bool { return cp[i].Symbol < cp[j].Symbol })
	if len(cp) == 0 {
		delete(m.centrality, serviceID)
		return nil
	}
	m.centrality[serviceID] = centralityEntry{watermark: watermark, rows: cp}
	return nil
}

// SymbolCentrality returns serviceID's rows sorted by symbol and their
// watermark; rows in a suppressed file are dropped.
func (m *MemoryStore) SymbolCentrality(ctx context.Context, serviceID string) (string, []SymbolCentrality, error) {
	m.mu.RLock()
	e, ok := m.centrality[serviceID]
	var out []SymbolCentrality
	if ok {
		out = make([]SymbolCentrality, len(e.rows))
		copy(out, e.rows)
	}
	m.mu.RUnlock()
	if !ok {
		return "", nil, nil
	}
	out, err := filterScoped(ctx, m.gate, serviceID, out, nil, func(r SymbolCentrality) string { return r.File })
	return e.watermark, out, err
}
