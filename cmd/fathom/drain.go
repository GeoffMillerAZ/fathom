package main

// drain.go — SIGTERM (design §8.5, RT-44). The exit{reason: drain} row is
// written FIRST, so a SIGKILL racing the drain still leaves a not-counted
// exit; heavy admission closes; in-flight tree/fleet work is cancelled and
// so is an in-flight ingest, which observes it at its batch boundary — the
// pipeline checks its context between fetch and write
// (internal/fusion/pipeline.go), so it stops before writing or finishes its
// write; that an aborted ingest leaves ingest_state in_progress and its
// watermark unmoved is RT-109's contract, not this file's; point and index
// calls finish; all within 12 s in total (under the plist's ExitTimeOut 20); the
// record is removed; exit 0 — an Activity Monitor "Quit" or `fathom daemon
// stop` stays down under schema 2, and `launchctl kickstart -k` is
// unaffected.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// drainBudget is the whole drain, under ExitTimeOut 20 (§8.1).
const drainBudget = 12 * time.Second

// drainer carries the drain's effects; each is injectable.
type drainer struct {
	ledger       string
	pid          int
	version      string
	startedAt    time.Time
	now          func() time.Time
	inflight     *tenancy.InFlight
	budget       time.Duration
	logf         func(string)
	shutdown     func(ctx context.Context) error // http.Server.Shutdown: in-flight requests finish
	removeRecord func()
	exit         func(code int)
	once         sync.Once
}

// drainAndExit runs the drain once, whatever signalled it.
func (d *drainer) drainAndExit(signal string) {
	d.once.Do(func() { d.drain(signal) })
}

func (d *drainer) drain(signal string) {
	start := d.now()
	budget := d.budget
	if budget <= 0 {
		budget = drainBudget
	}
	if d.ledger != "" {
		row := tenancy.NewLifecycleRow(tenancy.RowExit, start, d.pid, d.version)
		row.Code = tenancy.IntPtr(0)
		row.Reason = tenancy.ExitDrain
		row.Detail = signal + ": drained (not counted by the breaker)"
		if !d.startedAt.IsZero() {
			row.Uptime = start.Sub(d.startedAt).Round(time.Second).String()
		}
		if err := appendLifecycle(d.ledger, row); err != nil && d.logf != nil {
			d.logf(fmt.Sprintf("drain: exit row not written: %v", err))
		}
	}
	draining.Store(true)
	var cancelled []tenancy.InFlightEntry
	if d.inflight != nil {
		cancelled = d.inflight.Cancel(errDraining, tenancy.ClassTree, tenancy.ClassFleet, tenancy.ClassIngest)
	}
	if d.logf != nil {
		d.logf(fmt.Sprintf("drain: %s — heavy admission closed, %d tree/fleet/ingest call(s) cancelled (an ingest stops at its next cancellation check), point and index finishing within %s", signal, len(cancelled), budget))
	}
	ctx, cancel := context.WithDeadline(context.Background(), start.Add(budget))
	defer cancel()
	if d.shutdown != nil {
		if err := d.shutdown(ctx); err != nil && d.logf != nil {
			d.logf(fmt.Sprintf("drain: requests still in flight at the %s deadline (%v) — exiting anyway", budget, err))
		}
	}
	if d.removeRecord != nil {
		d.removeRecord()
	}
	if d.logf != nil {
		d.logf(fmt.Sprintf("drain: done in %s — exit 0", d.now().Sub(start).Round(time.Millisecond)))
	}
	d.exit(0)
}

// setCrashOutput makes runtime fatals durable: they are written to
// <ledger dir>/crash.log as well as stderr (§8.5).
func setCrashOutput(ledger string, logf func(string)) {
	if ledger == "" {
		return
	}
	p := filepath.Join(filepath.Dir(ledger), "crash.log")
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		logf(fmt.Sprintf("crash output: %s not opened: %v", p, err))
		return
	}
	if err := debug.SetCrashOutput(f, debug.CrashOptions{}); err != nil {
		logf(fmt.Sprintf("crash output: %v", err))
	}
	_ = f.Close() // SetCrashOutput duplicates the descriptor
}
