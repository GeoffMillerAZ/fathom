package main

// derived_backfill.go — RT-160 (docs/design/responsible-tenant.md §6): the
// serving daemon derives each registered service's service_summary and
// observation_calls from rows the store already holds, once, after
// warm-up — one service per transaction under the ingest slot, reading no
// file — so a store written before those tables needs no fleet re-ingest.
// A service already derived costs one state read. Paused at shed and above
// and in safe mode; a list_services read that finds a summary missing
// derives that one service itself (client.ServiceSummary).

import (
	"context"
	"fmt"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// derivedBackfillPoll is how often a paused backfill re-checks the ladder.
const derivedBackfillPoll = 5 * time.Second

func runDerivedBackfill(ctx context.Context, c *client.Client, logf func(string)) {
	wait := func() bool {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(derivedBackfillPoll):
			return true
		}
	}
	for tenancy.InWarmup(time.Now()) {
		if !wait() {
			return
		}
	}
	ids, err := c.Services(ctx)
	if err != nil {
		logf(fmt.Sprintf("derived backfill: services: %v", err))
		return
	}
	for _, svc := range ids {
		for {
			_, safe := tenancy.SafeModeNow()
			if tenancy.CurrentLevel() < tenancy.LevelShed && !safe {
				break
			}
			if !wait() {
				return
			}
		}
		st, err := c.DerivedState(ctx, svc)
		if err == nil && st.SummaryCurrent && st.CallsIndexed {
			continue
		}
		start := time.Now()
		rep, err := c.BackfillDerived(ctx, svc)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logf(fmt.Sprintf("derived backfill: %s: %v", svc, err))
			continue
		}
		if rep.Summary || rep.CallRows > 0 {
			logf(fmt.Sprintf("derived backfill: %s: summary=%v call rows=%d in %s", svc, rep.Summary, rep.CallRows, time.Since(start).Round(time.Millisecond)))
		}
	}
}
