package client

import (
	"context"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// IngestState returns service's ingest_state row (RT-109, design §6):
// "in_progress" with the pid of the process that began the ingest, until
// its final commit writes "complete". found is false when no row exists
// or the store has no ingest_state table. internal/mcp's ensure re-ingests
// a service another process left in_progress.
func (c *Client) IngestState(ctx context.Context, service string) (state string, pid int, found bool, err error) {
	iss, ok := c.st.(store.IngestStateStore)
	if !ok {
		return "", 0, false, nil
	}
	st, found, err := iss.GetIngestState(ctx, service)
	if err != nil || !found {
		return "", 0, false, err
	}
	return st.State, st.PID, true, nil
}
