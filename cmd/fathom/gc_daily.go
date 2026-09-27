package main

// gc_daily.go — the managed daemon's daily gc (docs/design/
// responsible-tenant.md §12.1, RT-180). Once a day, never in its first
// 10 minutes and never while work is in flight or the ladder is above
// normal, the managed daemon applies gc's daily classes: runGC over the
// home and $TMPDIR only (session stamps, dead daemons' records, the hooks'
// $TMPDIR caches, diag captures; ledgers are only reported). The
// workspace classes are a repository's and stay with `fathom gc`; the
// call log's archive pass runs on its writer.

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

const (
	gcDailyEvery  = 24 * time.Hour
	gcDailyWarmup = 10 * time.Minute
	gcDailyCheck  = 10 * time.Minute
)

// gcDailyState is gcDailyTick's memory between ticks.
type gcDailyState struct {
	started time.Time // the daemon's start
	last    time.Time // the last daily run (zero: none yet)
}

// gcDailyTick is one tick: it runs gc when the daemon is past its warmup,
// a day has passed since the last run and it is not busy, and reports
// whether it ran.
func gcDailyTick(st *gcDailyState, now time.Time, busy bool, run func(time.Time) gcReport, logf func(string)) bool {
	if now.Sub(st.started) < gcDailyWarmup || busy {
		return false
	}
	if !st.last.IsZero() && now.Sub(st.last) < gcDailyEvery {
		return false
	}
	st.last = now
	rep := run(now)
	var freed int64
	done := 0
	for _, it := range rep.Items {
		if it.Done {
			freed += it.Bytes
			done++
		}
	}
	msg := fmt.Sprintf("gc (daily): %d item(s) reclaimed, %s; next in %s", done, tenancy.FormatBytes(freed), gcDailyEvery)
	if len(rep.Errors) > 0 {
		msg += fmt.Sprintf(" · %d error(s): %v", len(rep.Errors), rep.Errors)
	}
	logf(msg)
	return true
}

// runDailyGC ticks gcDailyTick every gcDailyCheck until ctx ends.
func runDailyGC(ctx context.Context, started time.Time, busy func() bool, logf func(string)) {
	home, _ := os.UserHomeDir()
	st := gcDailyState{started: started}
	t := time.NewTicker(gcDailyCheck)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			gcDailyTick(&st, now, busy(), func(at time.Time) gcReport {
				return runGC(gcOptions{Home: home, TmpDir: os.TempDir(), Apply: true, Now: at})
			}, logf)
		}
	}
}
