package main

// ingestowner.go — RT-109's liveness check (design §6, review finding 3).
// An ingest_state row left in_progress by another process is an
// interrupted ingest only when that process is gone: a live owner is a
// concurrent ingest (a CLI `fathom ingest` into the daemon's store), and
// re-ingesting beside it would double the memory the responsible-tenant
// program bounds. The pid-reuse guard is the same one `daemon stop` uses:
// the process holding the pid must have started no later than the row.

import (
	"sync"
	"time"
)

// ingestOwnerCacheTTL bounds how often a live concurrent ingest costs a ps
// (every read of its service asks).
const ingestOwnerCacheTTL = 10 * time.Second

// ingestOwnerProbe returns the check internal/mcp's ensure uses
// (mcp.SetIngestOwnerProbe): whether pid is still the process that began
// an ingest at started. A missing or zombie pid, or one whose process
// started after the row, is gone. The last answer is cached for
// ingestOwnerCacheTTL.
func ingestOwnerProbe(ops daemonOps) func(pid int, started time.Time) bool {
	var (
		mu        sync.Mutex
		lastPID   int
		lastStart int64
		lastAlive bool
		lastAt    time.Time
		have      bool
	)
	return func(pid int, started time.Time) bool {
		now := ops.Now()
		mu.Lock()
		defer mu.Unlock()
		if have && pid == lastPID && started.UnixNano() == lastStart && now.Sub(lastAt) < ingestOwnerCacheTTL {
			return lastAlive
		}
		info, ok := ops.Proc(pid)
		// ps's lstart has one-second resolution; the owner wrote its row
		// after it started, so its truncated start is never after the row.
		alive := ok && !info.Start.After(started)
		lastPID, lastStart, lastAlive, lastAt, have = pid, started.UnixNano(), alive, now, true
		return alive
	}
}
