package client

import (
	"context"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// IngestStateRow is one service's ingest_state row (RT-109, design §6):
// State is "in_progress" from an ingest's start until its final commit
// writes "complete"; PID and StartedAt name the process that began it (the
// liveness check reads both), Path the corpus root it read.
type IngestStateRow = store.IngestState

// IngestState returns service's ingest_state row. found is false when no
// row exists or the store has no ingest_state table. internal/mcp's ensure
// re-ingests a service another process left in_progress and is no longer
// running.
func (c *Client) IngestState(ctx context.Context, service string) (row IngestStateRow, found bool, err error) {
	iss, ok := c.st.(store.IngestStateStore)
	if !ok {
		return IngestStateRow{}, false, nil
	}
	return iss.GetIngestState(ctx, service)
}

// corpusPath is where IngestService reads service from: service_meta's
// recorded root, else — for a first ingest that was killed or halted
// before its final commit recorded any meta — the root its ingest_state
// row names (review finding 1 on RT-109). "" when neither is known.
func (c *Client) corpusPath(ctx context.Context, service string) string {
	if meta, found, err := c.st.GetServiceMeta(ctx, service); err == nil && found && meta.Path != "" {
		return meta.Path
	}
	if row, found, err := c.IngestState(ctx, service); err == nil && found && row.Path != "" {
		return row.Path
	}
	return ""
}
