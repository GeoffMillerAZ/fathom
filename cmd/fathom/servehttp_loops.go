package main

import (
	"context"
	"os"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// startLoops starts the daemon's background loops: stdio rotation, the
// memory scavenger, the governor, the derived backfill, the WAL and GC
// loops, and the console event feeders.
func (r *serveRun) startLoops() {
	flog, outLog, c, srv, fleet := r.flog, r.outLog, r.c, r.srv, r.fleet
	managed, budget, consoleDeps := r.managed, r.budget, r.consoleDeps
	// Periodic OS-memory return (memscavenge.go, O108): bounds how long any
	// one peak (a large graph build, a burst of concurrent cache misses)
	// can inflate the daemon's OS-visible footprint, instead of leaving it
	// to Go's own lazy background scavenger for a process that runs 24/7.
	// The launchd-piped stdout/stderr bound (DG-6) moved off this ticker
	// onto its own (RT-127): turning scavenging off no longer lets the
	// logs grow unbounded.
	// outFile/errFile are the real *os.File only when this IS a genuine
	// os.Stdout/os.Stderr (production); a test's non-*os.File stdout/stderr
	// simply makes rotateDaemonStdio a no-op for those two arguments.
	outFile, _ := r.stdout.(*os.File)
	errFile, _ := r.stderr.(*os.File)
	// RT-127 (§12.2): the stdio logs rotate on their own 60 s ticker —
	// FATHOM_SCAVENGE=off no longer pauses them.
	//fathom:tenancy-allow bg-ctx daemon lifetime context in serve-http startup, cancelled by the drain; not a request
	stdioCtx, stopStdio := context.WithCancel(context.Background())
	r.deferred(stopStdio)
	//fathom:tenancy-allow bare-go daemon lifetime goroutine started once at serve-http startup and stopped by the drain
	go runStdioRotation(stdioCtx, stdioRotateEvery, outFile, errFile, outLog, flog)
	if !managed {
		outLog.Info("scavenge: off (unmanaged daemon, §10.1)")
	} else if scavengeDisabled() {
		outLog.Info("scavenge: OFF (FATHOM_SCAVENGE=off) — the daemon's OS-visible footprint may not shrink after a peak (O108); fathom-http.log/.err rotation runs on its own ticker")
	} else {
		//fathom:tenancy-allow bg-ctx daemon lifetime context in serve-http startup, cancelled by the drain; not a request
		//fathom:tenancy-allow bare-go daemon lifetime goroutine started once at serve-http startup and stopped by the drain
		go runMemoryScavenger(context.Background(), scavengeIntervalFromEnv(), outLog, func() {
			scavengeTick(outLog, flog, time.Now)
		}, nil)
	}

	// RT-33..RT-36, RT-155, RT-161, RT-172, RT-173: the governor samples
	// every 2 s and bounds memory whatever the cause. Each sample also
	// feeds store distress (the store volume's free space, RT-41) and the
	// breaker's safe mode, which steps down after 30 min at normal.
	sm := newSafeModeController(r.lcOut, r.lcIn.ledger, r.pid, Version, budget.Breaker.Warmup, func(m string) { flog.Warn(m) })
	minStoreFree := budget.Host.MinStoreVolumeFree
	_, stopGovernor := startGovernor(budget, r.budgetNotes, absStorePath(r.storePath), r.lcIn.ledger, servingLadderActions(srv, c, fleet), flog, func(s *tenancy.Sample, l tenancy.Level) {
		tenancy.NoteStoreVolumeFree(s.StoreFree, minStoreFree, s.At)
		sm.step(s.At, l)
	})
	r.deferred(stopGovernor)
	// RT-160: derive service_summary and observation_calls for services a
	// pre-RT-76 store holds, one at a time under the ingest slot.
	//fathom:tenancy-allow bg-ctx daemon lifetime context in serve-http startup, cancelled by the drain; not a request
	backfillCtx, stopBackfill := context.WithCancel(context.Background())
	r.deferred(stopBackfill)
	//fathom:tenancy-allow bare-go daemon lifetime goroutine started once at serve-http startup and stopped by the drain
	go runDerivedBackfill(backfillCtx, c, func(m string) { flog.Info(m) })
	// RT-128 (§12.2): the managed daemon truncates the WAL after 5 idle
	// minutes, at most hourly.
	if cp, ok := c.Store().(interface{ Checkpoint(context.Context) error }); ok && managed {
		//fathom:tenancy-allow bg-ctx daemon lifetime context in serve-http startup, cancelled by the drain; not a request
		walCtx, stopWAL := context.WithCancel(context.Background())
		r.deferred(stopWAL)
		//fathom:tenancy-allow bare-go daemon lifetime goroutine started once at serve-http startup and stopped by the drain
		go runWALIdleCheckpoint(walCtx, rpcActivity.Load, func() bool { return tenancy.DefaultInFlight.Len() > 0 }, cp.Checkpoint, func(m string) { outLog.Info(m) })
	}
	// RT-181 (§12.1): the managed daemon applies gc's daily classes once a
	// day, idle and at the ladder's normal level only.
	if managed {
		//fathom:tenancy-allow bg-ctx daemon lifetime context in serve-http startup, cancelled by the drain; not a request
		gcCtx, stopGC := context.WithCancel(context.Background())
		r.deferred(stopGC)
		//fathom:tenancy-allow bare-go daemon lifetime goroutine started once at serve-http startup and stopped by the drain
		go runDailyGC(gcCtx, time.Now(), func() bool {
			return tenancy.DefaultInFlight.Len() > 0 || tenancy.CurrentLevel() != tenancy.LevelNormal
		}, func(m string) { outLog.Info(m) })
	}
	publishDaemonEvent(consoleDeps.bus, r.prevRecord, r.hadPrev, r.commit)
	eventSources := newConsoleEventSources(consoleDeps.bus, c, verifyLedgerPath(), sessionLedgerPath())
	eventSources.warn = func(msg string) { flog.Warn(msg) }
	// RT-119 (§14, §10.1): the feeders run only while GET /events has a
	// subscriber — managed or not, an unwatched console polls nothing.
	feeders := newFeederSwitch(eventSources)
	consoleDeps.feeders = feeders
	consoleDeps.bus.OnSubscribers(feeders.subscribers)
	bus := consoleDeps.bus
	r.deferred(func() { bus.OnSubscribers(nil) })
}
