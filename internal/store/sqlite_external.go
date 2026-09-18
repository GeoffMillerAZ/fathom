package store

// sqlite_external.go — the findings-loop's per-claim cursor table (plan
// FL-7, vision 30 §3.3 item 4, ADR-040 decision 2). Additive: a new table,
// created by its own DDL const executed right after the main createSchema
// DDL — the main const stays untouched, matching every prior additive-table
// slice in this package.

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// externalFindingsDDL creates the per-claim cursor row the external import
// path (internal/findings/external) upserts on every applied event:
// enforces ordering/idempotency on the NEXT import for the same
// (service_id, claim_id), and carries the source-linked provenance vision
// 30 §4 item 4 requires be served alongside the claim.
const externalFindingsDDL = `
CREATE TABLE IF NOT EXISTS external_findings (
    service_id       TEXT    NOT NULL,
    claim_id         TEXT    NOT NULL,
    producer         TEXT    NOT NULL DEFAULT '',
    producer_version TEXT    NOT NULL DEFAULT '',
    source_id        TEXT    NOT NULL DEFAULT '',
    source_version   BIGINT  NOT NULL DEFAULT 0,
    payload_digest   TEXT    NOT NULL DEFAULT '',
    source_revision  TEXT    NOT NULL DEFAULT '',
    rule_id          TEXT    NOT NULL DEFAULT '',
    rule_version     TEXT    NOT NULL DEFAULT '',
    remedy           TEXT    NOT NULL DEFAULT '',
    anchor_path      TEXT    NOT NULL DEFAULT '',
    anchor_start     INTEGER NOT NULL DEFAULT 0,
    anchor_end       INTEGER NOT NULL DEFAULT 0,
    anchor_symbol    TEXT    NOT NULL DEFAULT '',
    anchor_digest    TEXT    NOT NULL DEFAULT '',
    observed_at      BIGINT  NOT NULL DEFAULT 0,
    imported_at      BIGINT  NOT NULL DEFAULT 0,
    unanchored       BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (service_id, claim_id)
);
`

const externalFindingCols = `service_id, claim_id, producer, producer_version, source_id, source_version,
    payload_digest, source_revision, rule_id, rule_version, remedy,
    anchor_path, anchor_start, anchor_end, anchor_symbol, anchor_digest,
    observed_at, imported_at, unanchored`

// PutExternalFinding upserts the cursor row for (service_id, claim_id).
func (s *SQLiteStore) PutExternalFinding(ctx context.Context, f core.ExternalFinding) error {
	_, err := s.db.ExecContext(
		ctx, `
INSERT INTO external_findings (`+externalFindingCols+`)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (service_id, claim_id) DO UPDATE SET
    producer         = excluded.producer,
    producer_version = excluded.producer_version,
    source_id        = excluded.source_id,
    source_version   = excluded.source_version,
    payload_digest   = excluded.payload_digest,
    source_revision  = excluded.source_revision,
    rule_id          = excluded.rule_id,
    rule_version     = excluded.rule_version,
    remedy           = excluded.remedy,
    anchor_path      = excluded.anchor_path,
    anchor_start     = excluded.anchor_start,
    anchor_end       = excluded.anchor_end,
    anchor_symbol    = excluded.anchor_symbol,
    anchor_digest    = excluded.anchor_digest,
    observed_at      = excluded.observed_at,
    imported_at      = excluded.imported_at,
    unanchored       = excluded.unanchored
`,
		f.ServiceID, f.ClaimID, f.Producer, f.ProducerVersion, f.SourceID, int64(f.SourceVersion),
		f.PayloadDigest, f.SourceRevision, f.RuleID, f.RuleVersion, f.Remedy,
		f.AnchorPath, f.AnchorStart, f.AnchorEnd, f.AnchorSymbol, f.AnchorDigest,
		f.ObservedAt.UnixNano(), f.ImportedAt.UnixNano(), f.Unanchored,
	)
	if err != nil {
		return fmt.Errorf("PutExternalFinding: %w", err)
	}
	return nil
}

// ExternalFinding returns the cursor row for (serviceID, claimID). found is
// false when no such row exists.
func (s *SQLiteStore) ExternalFinding(ctx context.Context, serviceID, claimID string) (core.ExternalFinding, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT `+externalFindingCols+`
FROM external_findings
WHERE service_id = ? AND claim_id = ?
`, serviceID, claimID)
	f, err := scanExternalFindingRow(row)
	if err == sql.ErrNoRows {
		return core.ExternalFinding{}, false, nil
	}
	if err != nil {
		return core.ExternalFinding{}, false, fmt.Errorf("ExternalFinding: %w", err)
	}
	return f, true, nil
}

// ExternalFindingsForService returns every cursor row for serviceID, sorted
// by claim_id.
func (s *SQLiteStore) ExternalFindingsForService(ctx context.Context, serviceID string) ([]core.ExternalFinding, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT `+externalFindingCols+`
FROM external_findings
WHERE service_id = ?
ORDER BY claim_id
`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("ExternalFindingsForService query: %w", err)
	}
	defer rows.Close()

	var out []core.ExternalFinding
	for rows.Next() {
		f, err := scanExternalFindingRow(rows)
		if err != nil {
			return nil, fmt.Errorf("ExternalFindingsForService scan: %w", err)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ExternalFindingsForService rows: %w", err)
	}
	return out, nil
}

// externalFindingScanner abstracts over *sql.Row and *sql.Rows, both of
// which implement Scan with an identical signature.
type externalFindingScanner interface {
	Scan(dest ...any) error
}

func scanExternalFindingRow(row externalFindingScanner) (core.ExternalFinding, error) {
	var (
		f             core.ExternalFinding
		sourceVersion int64
		observedAt    int64
		importedAt    int64
	)
	if err := row.Scan(
		&f.ServiceID, &f.ClaimID, &f.Producer, &f.ProducerVersion, &f.SourceID, &sourceVersion,
		&f.PayloadDigest, &f.SourceRevision, &f.RuleID, &f.RuleVersion, &f.Remedy,
		&f.AnchorPath, &f.AnchorStart, &f.AnchorEnd, &f.AnchorSymbol, &f.AnchorDigest,
		&observedAt, &importedAt, &f.Unanchored,
	); err != nil {
		return core.ExternalFinding{}, err
	}
	f.SourceVersion = uint64(sourceVersion)
	f.ObservedAt = time.Unix(0, observedAt).UTC()
	f.ImportedAt = time.Unix(0, importedAt).UTC()
	return f, nil
}
