package main

// wal_idle.go — the managed daemon's WAL truncation (docs/design/
// responsible-tenant.md §12.2, RT-128): after 5 idle minutes — no request
// served and nothing in flight — it runs wal_checkpoint(TRUNCATE), at most
// once an hour, so the -wal file an ingest grew does not hold its
// high-water mark on disk for the life of the process. A busy checkpoint
// (a reader holds the WAL) is logged and retried at the next idle window.
// VACUUM is never automatic.

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

// rpcActivity counts /rpc requests served by this process (the idle signal).
var rpcActivity atomic.Int64

// WAL truncation cadence (§12.2).
const (
	walIdleAfter  = 5 * time.Minute
	walEvery      = time.Hour
	walCheckEvery = time.Minute
)

// walIdleState is checkpointWhenIdle's memory between ticks.
type walIdleState struct {
	lastCount    int64     // the activity counter at the last tick
	lastActivity time.Time // when it last moved (or something was in flight)
	lastRun      time.Time // the last checkpoint attempt
}

// checkpointWhenIdle is one tick: activity is a monotonic request counter
// and busy whether anything is in flight. It runs checkpoint when the
// daemon has been idle walIdleAfter and no attempt ran within walEvery,
// and reports whether it ran.
func checkpointWhenIdle(st *walIdleState, now time.Time, activity int64, busy bool, checkpoint func() error, logf func(string)) bool {
	if st.lastActivity.IsZero() || activity != st.lastCount || busy {
		st.lastCount, st.lastActivity = activity, now
		return false
	}
	if now.Sub(st.lastActivity) < walIdleAfter {
		return false
	}
	if !st.lastRun.IsZero() && now.Sub(st.lastRun) < walEvery {
		return false
	}
	st.lastRun = now
	if err := checkpoint(); err != nil {
		logf(fmt.Sprintf("wal: idle checkpoint(TRUNCATE) did not complete: %v — retried at the next idle window after %s", err, walEvery))
		return true
	}
	logf(fmt.Sprintf("wal: idle %s — checkpoint(TRUNCATE) ran; next at most in %s", now.Sub(st.lastActivity).Round(time.Second), walEvery))
	return true
}

// runWALIdleCheckpoint ticks checkpointWhenIdle every walCheckEvery.
func runWALIdleCheckpoint(ctx context.Context, activity func() int64, busy func() bool, checkpoint func(context.Context) error, logf func(string)) {
	t := time.NewTicker(walCheckEvery)
	defer t.Stop()
	var st walIdleState
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			checkpointWhenIdle(&st, now, activity(), busy(), func() error {
				cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
				return checkpoint(cctx)
			}, logf)
		}
	}
}
