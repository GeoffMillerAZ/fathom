package store

// sqlite_typededges.go — SQLiteStore's typed edges and their per-package
// fingerprints, beside sqlite_typededgestatus.go (the typed-edge pass
// outcome). Split out of sqlite.go (wave g1).

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

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
		rowCtx := context.WithoutCancel(ctx) // LB-F208: no driver goroutine per row (see PutObservations)
		for _, e := range edges {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("PutTypedEdges: %w", err)
			}
			if _, err := stmt.ExecContext(rowCtx, serviceID, pkgPath,
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
