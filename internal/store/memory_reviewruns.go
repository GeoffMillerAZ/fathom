package store

// memory_reviewruns.go — the MemoryStore mirror of sqlite_reviewruns.go
// (FL-15, docs/plans/findings-loop.md): review_runs's idempotency-cache
// contract, backed by a plain map for tests and session-mode operation.

import (
	"context"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// reviewRunKey scopes a recorded review run to (serviceID, idempotencyKey) —
// review_runs' own primary key.
type reviewRunKey struct {
	serviceID      string
	idempotencyKey string
}

// RecordReviewRun upserts run by (ServiceID, IdempotencyKey) — see the Store
// interface doc comment for the replace-on-conflict contract.
func (m *MemoryStore) RecordReviewRun(_ context.Context, run core.ReviewRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reviewRuns[reviewRunKey{run.ServiceID, run.IdempotencyKey}] = run
	return nil
}

// ReviewRun returns the recorded review run for (serviceID,
// idempotencyKey); found is false when no run was ever recorded under that
// key.
func (m *MemoryStore) ReviewRun(_ context.Context, serviceID, idempotencyKey string) (core.ReviewRun, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	run, ok := m.reviewRuns[reviewRunKey{serviceID, idempotencyKey}]
	return run, ok, nil
}
