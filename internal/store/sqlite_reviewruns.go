package store

// sqlite_reviewruns.go — review_runs (FL-15, docs/plans/findings-loop.md;
// ADR-040 decision 6): the idempotency-cache table `fathom review
// --idempotency-key` reads/writes. Kept in its own file, added by its own
// exec right after the main schema const (sqlite.go's createSchema), per
// the additive-table convention documented there — the main ddl const is
// never edited for a new table.

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

const reviewRunsDDL = `
CREATE TABLE IF NOT EXISTS review_runs (
    service_id       TEXT   NOT NULL,
    idempotency_key  TEXT   NOT NULL,
    payload_digest   TEXT   NOT NULL DEFAULT '',
    response_json    TEXT   NOT NULL DEFAULT '',
    recorded_at      BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (service_id, idempotency_key)
);
`

// createReviewRunsSchema creates the review_runs table. Called once from
// createSchema, right after the main schema exec.
func (s *SQLiteStore) createReviewRunsSchema() error {
	_, err := s.db.Exec(reviewRunsDDL)
	return err
}

// RecordReviewRun upserts one review run by (service_id, idempotency_key) —
// see the Store interface doc comment for the replace-on-conflict contract.
func (s *SQLiteStore) RecordReviewRun(ctx context.Context, run core.ReviewRun) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO review_runs (service_id, idempotency_key, payload_digest, response_json, recorded_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (service_id, idempotency_key) DO UPDATE SET
    payload_digest = excluded.payload_digest,
    response_json  = excluded.response_json,
    recorded_at    = excluded.recorded_at
`, run.ServiceID, run.IdempotencyKey, run.PayloadDigest, run.ResponseJSON, run.RecordedAt.UnixNano())
	if err != nil {
		return fmt.Errorf("RecordReviewRun: %w", err)
	}
	return nil
}

// ReviewRun returns the recorded review run for (serviceID, idempotencyKey);
// found is false when no row exists.
func (s *SQLiteStore) ReviewRun(ctx context.Context, serviceID, idempotencyKey string) (core.ReviewRun, bool, error) {
	var (
		run        core.ReviewRun
		recordedAt int64
	)
	err := s.db.QueryRowContext(ctx, `
SELECT service_id, idempotency_key, payload_digest, response_json, recorded_at
FROM review_runs
WHERE service_id = ? AND idempotency_key = ?
`, serviceID, idempotencyKey).Scan(&run.ServiceID, &run.IdempotencyKey, &run.PayloadDigest, &run.ResponseJSON, &recordedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return core.ReviewRun{}, false, nil
		}
		return core.ReviewRun{}, false, fmt.Errorf("ReviewRun: %w", err)
	}
	run.RecordedAt = time.Unix(0, recordedAt).UTC()
	return run, true, nil
}
