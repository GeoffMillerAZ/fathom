package store

// ingeststate.go — ingest_state (RT-109, docs/design/
// responsible-tenant.md §6): one row per service naming whether its last
// ingest finished. An ingest writes `in_progress` (with its pid, start and
// target commit) before its first write and `complete` at its final commit,
// beside the watermark; an ingest that was killed or halted leaves
// `in_progress`, so the next process knows the index is not the one the
// watermark describes and re-ingests it (unless the service is
// quarantined). Before this, the watermark was written before extraction,
// and a killed ingest looked fresh.
//
// Kept in its own file and its own exec after the main schema const, per
// the additive-table convention sqlite.go's createSchema documents. The
// table is optional to readers (docs/spec: a conforming reader tolerates
// unknown tables), so callers reach it through IngestStateStore.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const ingestStateDDL = `
CREATE TABLE IF NOT EXISTS ingest_state (
    service_id    TEXT    NOT NULL PRIMARY KEY,
    state         TEXT    NOT NULL,
    pid           INTEGER NOT NULL DEFAULT 0,
    started_at    BIGINT  NOT NULL DEFAULT 0,
    target_commit TEXT    NOT NULL DEFAULT ''
);
`

// Ingest states.
const (
	IngestInProgress = "in_progress"
	IngestComplete   = "complete"
)

// IngestState is one service's ingest progress row.
type IngestState struct {
	ServiceID    string
	State        string // IngestInProgress | IngestComplete
	PID          int    // the process that wrote the row
	StartedAt    int64  // Unix nano
	TargetCommit string // the watermark the ingest will record when it completes
}

// IngestStateStore is the optional store capability behind ingest_state.
// Both SQLiteStore and MemoryStore implement it.
type IngestStateStore interface {
	SetIngestState(ctx context.Context, st IngestState) error
	GetIngestState(ctx context.Context, serviceID string) (st IngestState, found bool, err error)
	DeleteIngestState(ctx context.Context, serviceID string) error
}

func (s *SQLiteStore) createIngestStateSchema() error {
	_, err := s.db.Exec(ingestStateDDL)
	return err
}

// SetIngestState upserts serviceID's row.
func (s *SQLiteStore) SetIngestState(ctx context.Context, st IngestState) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO ingest_state (service_id, state, pid, started_at, target_commit)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (service_id) DO UPDATE SET
    state = excluded.state, pid = excluded.pid,
    started_at = excluded.started_at, target_commit = excluded.target_commit
`, st.ServiceID, st.State, st.PID, st.StartedAt, st.TargetCommit)
	if err != nil {
		return fmt.Errorf("SetIngestState: %w", err)
	}
	return nil
}

// GetIngestState returns serviceID's row; found is false when none exists.
func (s *SQLiteStore) GetIngestState(ctx context.Context, serviceID string) (IngestState, bool, error) {
	st := IngestState{ServiceID: serviceID}
	err := s.db.QueryRowContext(ctx, `
SELECT state, pid, started_at, target_commit FROM ingest_state WHERE service_id = ?
`, serviceID).Scan(&st.State, &st.PID, &st.StartedAt, &st.TargetCommit)
	if errors.Is(err, sql.ErrNoRows) {
		return IngestState{}, false, nil
	}
	if err != nil {
		return IngestState{}, false, fmt.Errorf("GetIngestState: %w", err)
	}
	return st, true, nil
}

// DeleteIngestState removes serviceID's row; a missing row is a no-op.
func (s *SQLiteStore) DeleteIngestState(ctx context.Context, serviceID string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM ingest_state WHERE service_id = ?`, serviceID); err != nil {
		return fmt.Errorf("DeleteIngestState: %w", err)
	}
	return nil
}

// SetIngestState upserts serviceID's row.
func (m *MemoryStore) SetIngestState(_ context.Context, st IngestState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ingestState[st.ServiceID] = st
	return nil
}

// GetIngestState returns serviceID's row; found is false when none exists.
func (m *MemoryStore) GetIngestState(_ context.Context, serviceID string) (IngestState, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	st, ok := m.ingestState[serviceID]
	return st, ok, nil
}

// DeleteIngestState removes serviceID's row; a missing row is a no-op.
func (m *MemoryStore) DeleteIngestState(_ context.Context, serviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.ingestState, serviceID)
	return nil
}
