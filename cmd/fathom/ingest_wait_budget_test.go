package main

import (
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// TestIngestWaitBudgetLeavesRoomUnderTheRPCWriteTimeout (O128, EW-18): the
// ingest tool's wait budget ends, and the queue-state answer is written,
// well inside the route's write deadline; an answer past it is the empty
// body the observation reported. A relation between two constants, not a
// timing.
func TestIngestWaitBudgetLeavesRoomUnderTheRPCWriteTimeout(t *testing.T) {
	const margin = 15 * time.Second
	if mcp.DefaultIngestWaitBudget+margin > daemonRPCWriteTimeout {
		t.Fatalf("ingest wait budget %v plus %v of margin exceeds the /rpc write deadline %v: a long ingest would again answer with an empty body",
			mcp.DefaultIngestWaitBudget, margin, daemonRPCWriteTimeout)
	}
}
