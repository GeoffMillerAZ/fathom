package store

// sqlite_typededgestatus.go — typed_edge_status (DG-10, fathom diagnostics
// sweep): the last-known typed-edge pass outcome for a service, persisted
// across a daemon restart.
//
// internal/fusion.Pipeline.TypedEdgeStatus (typedenv.go) was, before this,
// a plain in-memory map with no persistence: since the daemon restarts
// roughly twice a day (O108 mitigation), most of the fleet's typed-edge
// health was unobservable through list_services/doctor at any given moment
// purely from restart timing, independent of whether anything was actually
// broken. Kept in internal/store, not internal/fusion, because
// internal/fusion already imports internal/store (a back-import would
// cycle) — Pipeline converts to/from its own TypedEdgeStatus at the two
// call sites that read/write this (internal/fusion/typedenv.go).
//
// Kept in its own file, added by its own exec right after the main schema
// const (sqlite.go's createSchema), per the additive-table convention
// documented there — the main ddl const is never edited for a new table.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

const typedEdgeStatusDDL = `
CREATE TABLE IF NOT EXISTS typed_edge_status (
    service_id                TEXT   NOT NULL PRIMARY KEY,
    packages_seen             INTEGER NOT NULL DEFAULT 0,
    packages                  INTEGER NOT NULL DEFAULT 0,
    edges_total               INTEGER NOT NULL DEFAULT 0,
    failed                    INTEGER NOT NULL DEFAULT 0,
    reason                    TEXT   NOT NULL DEFAULT '',
    fallback                  TEXT   NOT NULL DEFAULT '',
    discovery_gaps            INTEGER NOT NULL DEFAULT 0,
    discovery_gap_files       TEXT   NOT NULL DEFAULT '[]',
    edges_dropped_out_of_scope INTEGER NOT NULL DEFAULT 0,
    recorded_at               BIGINT NOT NULL DEFAULT 0
);
`

// createTypedEdgeStatusSchema creates the typed_edge_status table. Called
// once from createSchema, right after review_runs.
func (s *SQLiteStore) createTypedEdgeStatusSchema() error {
	_, err := s.db.Exec(typedEdgeStatusDDL)
	return err
}

// TypedEdgeStatusRecord is the store-persisted form of one service's last
// typed-edge pass outcome (DG-10, internal/fusion.TypedEdgeStatus).
// RecordedAt is the watermark a caller (doctor, in particular) uses to
// distinguish "never known" from "known as of <RecordedAt>, possibly
// pre-restart" — this record on its own carries no information about
// whether the process that wrote it is still the one running now.
type TypedEdgeStatusRecord struct {
	PackagesSeen           int
	Packages               int
	EdgesTotal             int
	Failed                 bool
	Reason                 string
	Fallback               string
	DiscoveryGaps          int
	DiscoveryGapFiles      []string
	EdgesDroppedOutOfScope int
	RecordedAt             time.Time
}

// SetTypedEdgeStatus records or replaces the persisted typed-edge status
// for serviceID.
func (s *SQLiteStore) SetTypedEdgeStatus(ctx context.Context, serviceID string, rec TypedEdgeStatusRecord) error {
	filesJSON, err := json.Marshal(rec.DiscoveryGapFiles)
	if err != nil {
		return fmt.Errorf("SetTypedEdgeStatus: marshal discovery_gap_files: %w", err)
	}
	recordedAt := rec.RecordedAt
	if recordedAt.IsZero() {
		recordedAt = time.Now().UTC()
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO typed_edge_status (
    service_id, packages_seen, packages, edges_total, failed, reason,
    fallback, discovery_gaps, discovery_gap_files, edges_dropped_out_of_scope,
    recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (service_id) DO UPDATE SET
    packages_seen              = excluded.packages_seen,
    packages                   = excluded.packages,
    edges_total                = excluded.edges_total,
    failed                     = excluded.failed,
    reason                     = excluded.reason,
    fallback                   = excluded.fallback,
    discovery_gaps             = excluded.discovery_gaps,
    discovery_gap_files        = excluded.discovery_gap_files,
    edges_dropped_out_of_scope = excluded.edges_dropped_out_of_scope,
    recorded_at                = excluded.recorded_at
`, serviceID, rec.PackagesSeen, rec.Packages, rec.EdgesTotal, rec.Failed, rec.Reason,
		rec.Fallback, rec.DiscoveryGaps, string(filesJSON), rec.EdgesDroppedOutOfScope,
		recordedAt.UnixNano())
	if err != nil {
		return fmt.Errorf("SetTypedEdgeStatus: %w", err)
	}
	return nil
}

// TypedEdgeStatus returns the last persisted typed-edge status for
// serviceID. found is false when none has ever been recorded.
func (s *SQLiteStore) TypedEdgeStatus(ctx context.Context, serviceID string) (TypedEdgeStatusRecord, bool, error) {
	var (
		rec        TypedEdgeStatusRecord
		filesJSON  string
		recordedAt int64
	)
	err := s.db.QueryRowContext(ctx, `
SELECT packages_seen, packages, edges_total, failed, reason, fallback,
       discovery_gaps, discovery_gap_files, edges_dropped_out_of_scope, recorded_at
FROM typed_edge_status
WHERE service_id = ?
`, serviceID).Scan(&rec.PackagesSeen, &rec.Packages, &rec.EdgesTotal, &rec.Failed, &rec.Reason,
		&rec.Fallback, &rec.DiscoveryGaps, &filesJSON, &rec.EdgesDroppedOutOfScope, &recordedAt)
	if err == sql.ErrNoRows {
		return TypedEdgeStatusRecord{}, false, nil
	}
	if err != nil {
		return TypedEdgeStatusRecord{}, false, fmt.Errorf("TypedEdgeStatus: %w", err)
	}
	_ = json.Unmarshal([]byte(filesJSON), &rec.DiscoveryGapFiles)
	rec.RecordedAt = time.Unix(0, recordedAt).UTC()
	return rec, true, nil
}
