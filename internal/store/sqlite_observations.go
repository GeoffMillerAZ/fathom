package store

// sqlite_observations.go — SQLiteStore's observation rows and what hangs off
// them: the batch append, the raw per-service read, the shared row scanner,
// fingerprints, and the source-ref / package / source-kind queries. Split out
// of sqlite.go (wave g1).

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

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

	// LB-F208: the driver starts a goroutine, a channel and a wake-up for every statement run
	// under a context that can be cancelled (interruptOnDone). One row is a bounded statement,
	// so it runs under the uncancellable context and the loop checks ctx between rows; the
	// transaction itself still ends with ctx.
	rowCtx := context.WithoutCancel(ctx)
	for _, o := range obs {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("PutObservations: %w", err)
		}
		callsJSON, err := json.Marshal(o.Calls)
		if err != nil {
			return fmt.Errorf("PutObservations marshal calls: %w", err)
		}
		ingestedAt := o.IngestedAt
		if ingestedAt.IsZero() {
			ingestedAt = time.Now().UTC()
		}
		_, err = stmt.ExecContext(
			rowCtx,
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
		rows, err = s.db.QueryContext(ctx, queryObservationsSQL, serviceID)
	} else {
		rows, err = s.db.QueryContext(ctx, queryObservationsConcernSQL, serviceID, concernID)
	}
	if err != nil {
		return nil, fmt.Errorf("QueryObservations query: %w", err)
	}
	defer rows.Close()

	obs, err := scanObservations(ctx, rows)
	if err != nil {
		return nil, err
	}
	// RT-78: ingested_at order, sorted here on the service's rows rather
	// than by SQLite in a temp B-tree. Rows arrive in rowid (insertion)
	// order from the service_id index, so the stable sort keeps insertion
	// order among equal timestamps.
	sort.SliceStable(obs, func(i, j int) bool { return obs[i].IngestedAt.Before(obs[j].IngestedAt) })
	return obs, nil
}

// queryObservationsSQL and queryObservationsConcernSQL read a service's
// observations with no ORDER BY (RT-78: no temp B-tree); the caller sorts.
const (
	queryObservationsSQL = `
SELECT service_id, concern_id, source_ref, source_kind, evidence_tier,
       symbol, package, claim, content_hash, calls, complexity, line, end_line, ingested_at,
       tool_kind, generated
FROM observations
WHERE service_id = ?`
	queryObservationsConcernSQL = `
SELECT service_id, concern_id, source_ref, source_kind, evidence_tier,
       symbol, package, claim, content_hash, calls, complexity, line, end_line, ingested_at,
       tool_kind, generated
FROM observations
WHERE service_id = ? AND concern_id = ?`
)

// Fingerprint returns the stored content hash for (serviceID, sourceRef).
func (s *SQLiteStore) Fingerprint(ctx context.Context, serviceID, sourceRef string) (string, bool, error) {
	var hash string
	if err := ctx.Err(); err != nil {
		return "", false, fmt.Errorf("Fingerprint query: %w", err)
	}
	err := s.db.QueryRowContext(context.WithoutCancel(ctx), `
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
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("SetFingerprint: %w", err)
	}
	_, err := s.db.ExecContext(context.WithoutCancel(ctx), `
INSERT INTO fingerprints (service_id, source_ref, content_hash)
VALUES (?, ?, ?)
ON CONFLICT (service_id, source_ref) DO UPDATE SET content_hash = excluded.content_hash
`, serviceID, sourceRef, hash)
	if err != nil {
		return fmt.Errorf("SetFingerprint: %w", err)
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
