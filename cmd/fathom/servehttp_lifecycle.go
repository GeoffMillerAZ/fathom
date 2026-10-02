package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/modulehost"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// startLedger is the lifecycle phase's first step: the ledger and the
// crash-loop breaker, then the warm-up window and the exit-row writer.
func (r *serveRun) startLedger() (int, bool) {
	flog := r.flog
	managed, managedLabel, budget := r.managed, r.managedLabel, r.budget
	// RT-37/RT-38/RT-158 (design §2.3, §8.4 step 2): the lifecycle ledger
	// and the crash-loop breaker — after the kill switch and the
	// [tenancy] resolution (the breaker's thresholds and the ledger id come
	// from them), before the store and the bind. An unmanaged daemon
	// without --lifecycle-id gets its per-process ledger after the bind
	// (its id names the bound port).
	lcID, lcErr := lifecycleID(r.lifecycleIDFlag, managed, managedLabel, installedAgentLabels())
	if lcErr != nil {
		flog.Error(lcErr.Error())
		return 1, true
	}
	r.lcID = lcID
	r.pid = os.Getpid()
	r.commit = statuszBuildInfo().BinaryCommit
	r.lcIn = lifecycleStart{
		managed: managed, addr: r.addr, pid: r.pid, commit: r.commit, version: Version,
		breaker:  budget.Breaker,
		alive:    func(p int) bool { return !pidGone(p) },
		evidence: func(since time.Time) []tenancy.EvidenceFile { return scanUncleanEvidence(evidenceDirs(), since) },
		logf:     func(m string) { flog.Warn(m) },
	}
	if managed {
		if dir, dirErr := daemonStateDir(); dirErr == nil {
			r.lcIn.switchDir = dir
		}
		r.lcIn.relaunch = func(rec tenancy.SwitchRecord) []string { return relaunchClause(realDaemonOps{}, managedLabel, rec) }
		if _, _, post, ok := installedPosture(managedLabel); ok {
			r.lcIn.plistSchema = post.Schema
		}
	}
	// r.lcOut is the breaker's verdict for this start: only its safe mode
	// (never paused.json's) steps down after 30 min at normal.
	if lcID != "" {
		if out, ok := r.begin(lcID); ok {
			r.lcOut = out
			if out.stop {
				return out.code, true
			}
			if out.safe != nil {
				applySafeMode(out.safe, func(m string) { flog.Warn(m) })
			}
		}
	}
	// Warm-up after any start (§2.3, RT-40): heavy classes get one slot
	// each and auto-refresh runs no inline ingest.
	tenancy.SetWarmup(r.startedAt, budget.Breaker.Warmup)
	if ws, wd := tenancy.Warmup(); !ws.IsZero() {
		flog.Info(fmt.Sprintf("tenancy: warm-up %s — one slot per heavy class and no inline auto-refresh ingest until %s", wd, ws.Add(wd).Format(time.TimeOnly)))
	}
	return 0, false
}

// begin opens the lifecycle ledger under id and runs the breaker.
func (r *serveRun) begin(id string) (lifecycleOutcome, bool) {
	home, hErr := os.UserHomeDir()
	if hErr != nil || home == "" || id == "" {
		return lifecycleOutcome{}, false
	}
	p, pErr := ledgerPathFor(home, id)
	if pErr != nil {
		r.flog.Warn(fmt.Sprintf("lifecycle: no ledger (%v)", pErr))
		return lifecycleOutcome{}, false
	}
	r.lcIn.ledger, r.lcIn.id, r.lcIn.now = p, id, time.Now()
	return beginLifecycle(r.lcIn), true
}

// exitRow appends the exit row to the lifecycle ledger, when there is one.
func (r *serveRun) exitRow(reason, detail string, code int) {
	if r.lcIn.ledger == "" {
		return
	}
	row := tenancy.NewLifecycleRow(tenancy.RowExit, time.Now(), r.pid, Version)
	row.Code, row.Reason, row.Detail = tenancy.IntPtr(code), reason, detail
	row.Uptime = time.Since(r.startedAt).Round(time.Second).String()
	if err := appendLifecycle(r.lcIn.ledger, row); err != nil {
		r.flog.Warn(fmt.Sprintf("lifecycle: exit row not written: %v", err))
	}
}

// mountModules mounts every --module store read-only.
func (r *serveRun) mountModules() (int, bool) {
	flog := r.flog
	// GKE-MH-1: mount every --module store read-only, conformance-checked
	// against LevelL0 (the module host's floor — sufficient for
	// internal/semantic.ObligationsFor and the staleness-report shape; see
	// store.ConformanceLevel's doc for why Server conformance itself has no
	// separate schema footprint to check). A refusal aborts startup loudly
	// — never a partial mount, never a silently-degraded module. It is
	// configuration, checked before the bind.
	if len(r.modulePaths) > 0 {
		r.moduleHost = modulehost.NewHost(store.LevelL0)
		for _, p := range r.modulePaths {
			if _, mErr := r.moduleHost.Mount(p, ""); mErr != nil {
				msg := fmt.Sprintf("mounting module %s: %v", p, mErr)
				flog.Error(msg)
				r.exitRow(tenancy.ExitStartFailed, msg, 1)
				return 1, true
			}
			flog.Info(fmt.Sprintf("serve-http: mounted module %s (%s)", p, modulehost.ModuleNameFromPath(p)))
		}
	}
	return 0, false
}

// limitMemory applies the soft memory limit and GOMAXPROCS.
func (r *serveRun) limitMemory() {
	// Soft memory limit (memorylimit.go, W-N, docs/plans/daemon-footprint.md):
	// bounds how hard the runtime lets the heap grow toward a ceiling in
	// the first place, complementing the scavenger below (which bounds how
	// long a peak's OS-visible footprint survives AFTER the fact).
	// RT-36: the GC hint is the budget's (the exit mark by default) and a
	// managed daemon's GOMAXPROCS is max(2, NumCPU/2).
	// Restored when serve-http returns (review finding 11): the process
	// exits then in production, but an in-process caller (a test) must
	// never inherit the daemon's GC hint or GOMAXPROCS.
	prevLimit, prevProcs := debug.SetMemoryLimit(-1), runtime.GOMAXPROCS(0)
	r.deferred(func() {
		debug.SetMemoryLimit(prevLimit)
		runtime.GOMAXPROCS(prevProcs)
	})
	applyMemoryLimit(r.budget, r.stdout, debug.SetMemoryLimit, runtime.GOMAXPROCS, os.Getenv)
}

// bind listens (bind BEFORE the store), names the port holder on failure,
// opens the unmanaged per-process ledger and records the daemon.
func (r *serveRun) bind() (int, bool) {
	flog, outLog := r.flog, r.outLog
	managed, managedLabel, addr := r.managed, r.managedLabel, r.addr
	// RT-42 (design §8.4 step 3): bind BEFORE the store. DG-19 (owner
	// amendment c): a self-restart or `task deploy` exits the OLD daemon
	// so launchd starts the new one — if the old process is still tearing
	// down when the new one races up, the bind can lose to EADDRINUSE even
	// though nothing is permanently wrong. Retry the BIND ONLY, with
	// backoff, for up to bindRetryMaxWait; then RT-43: a managed fathom of
	// the same label already serves the port (exit 0), any other holder
	// is named (exit 75).
	listener, listenErr := listenTCPWithRetry(addr, flog)
	if listenErr != nil {
		flog.Error(fmt.Sprintf("http serve: %v", listenErr))
		if isAddrInUse(listenErr) {
			code, reason, detail := resolvePortHolder(managed, managedLabel, gatherPortHolder(addr, realDaemonOps{}))
			flog.Error("serve-http: " + detail)
			r.exitRow(reason, detail, code)
			return code, true
		}
		r.exitRow(tenancy.ExitStartFailed, "bind "+addr+": "+listenErr.Error(), 1)
		return 1, true
	}
	r.listener = listener
	outLog.Info(fmt.Sprintf("serve-http: listening on %s", addr))
	if r.lcIn.ledger == "" && !managed {
		// The unmanaged per-process ledger: addr-<port>-<pid>, beside the
		// record; removed with it on a clean exit.
		if out, ok := r.begin(daemonID(daemonMeta{}, listener.Addr().String(), r.pid)); ok {
			r.lcOut = out
			if out.stop {
				_ = listener.Close()
				return out.code, true
			}
		}
	}
	setCrashOutput(r.lcIn.ledger, func(m string) { flog.Warn(m) })

	// OC-2: record this daemon (its absolute --call-log, store, pid,
	// commit) only now that the listener is bound, so a verb run from any
	// directory reads the log this daemon writes and a serve-http that
	// failed to start never leaves a record behind — and before the store,
	// so `fathom daemon stop` finds a daemon waiting for its store.
	r.prevRecord, r.hadPrev = readDaemonRecordFor("http://" + addr)
	meta := daemonMeta{Managed: managed, Label: managedLabel, PPID: r.startPPID, LifecycleID: r.lcID}
	if !managed {
		meta.IdleExit, meta.TTL = r.idleExit, r.ttl
	}
	rec, recordPath, recErr := recordServingDaemonAs(meta, listener.Addr().String(), r.callLogPath, r.storePath, r.commit, time.Now())
	r.recordPath = recordPath
	if recErr != nil {
		flog.Warn(fmt.Sprintf("serve-http: daemon record not written (%v) — verbs fall back to config and the legacy call-log path", recErr))
	} else {
		outLog.Info(fmt.Sprintf("serve-http: daemon record: %s (call log %s)", recordPath, rec.CallLog))
	}
	return 0, false
}

// serveWaiting starts serving: the waiting handler until the store opens,
// then the full one, swapped in place on one listener and one server.
func (r *serveRun) serveWaiting() {
	r.swap = newSwapHandler(waitingHandler(&r.waitState))
	// Activity (the unmanaged idle bound's input) is counted from the
	// bind, the store wait included.
	r.act = newActivity(time.Now)
	// RT-59 (§3.5): header, idle and per-route write timeouts and the
	// loopback-Host guard; the managed daemon answers a test caller 409
	// (RT-63).
	r.httpSrv = newDaemonServer(r.act.middleware(r.swap), daemonServerOpts{managed: r.managed})
	r.serveErr = make(chan error, 1)
	httpSrv, serveErr, listener := r.httpSrv, r.serveErr, r.listener
	//fathom:tenancy-allow bare-go daemon lifetime goroutine started once at serve-http startup and stopped by the drain
	go func() { serveErr <- httpSrv.Serve(listener) }()
}

// stopServing shuts the HTTP server down with a 2 s grace.
func (r *serveRun) stopServing() {
	//fathom:tenancy-allow bg-ctx daemon lifetime context in serve-http startup, cancelled by the drain; not a request
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = r.httpSrv.Shutdown(ctx)
}

// removeRecord removes the daemon record, through the lifetime once it exists.
func (r *serveRun) removeRecord() {
	if lt := r.lifetimeP.Load(); lt != nil {
		lt.closeRecord()
		return
	}
	removeDaemonRecord(r.recordPath)
}

// handleSignals installs the signal handler and the drainer.
func (r *serveRun) handleSignals() {
	flog, outLog := r.flog, r.outLog
	managed := r.managed
	// Signals are handled from the bind on (RT-42 moved the bind and the
	// record ahead of the store, so a SIGTERM during a store wait must not
	// take the default action and leave the record behind). Until the
	// lifetime exists the handler removes the record itself.
	r.perProcessLedger = !managed && r.lcID == "" && r.lcIn.ledger != ""
	// RT-97 (§7.1): the refresh queue's worker lives until the drain (or
	// return) stops it; it is started once the backend exists below.
	//fathom:tenancy-allow bg-ctx daemon lifetime context in serve-http startup, cancelled by the drain; not a request
	r.refreshCtx, r.stopRefresh = context.WithCancel(context.Background())
	r.deferred(r.stopRefresh)
	// RT-44 (§8.5): a managed daemon's SIGTERM drains and exits 0.
	dr := &drainer{
		stopRefresh: r.stopRefresh,
		ledger:      r.lcIn.ledger, pid: r.pid, version: Version, startedAt: r.startedAt, now: time.Now,
		inflight: tenancy.DefaultInFlight, budget: drainBudget, logf: func(m string) { flog.Warn(m) },
		shutdown:     r.httpSrv.Shutdown,
		removeRecord: r.removeRecord,
		exit:         os.Exit,
	}
	sigs := make(chan os.Signal, 1)
	if managed {
		signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	} else {
		signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	}
	r.deferred(func() {
		signal.Stop(sigs)
		close(sigs) // the handler below returns (an in-process caller)
	})
	//fathom:tenancy-allow bare-go daemon lifetime goroutine started once at serve-http startup and stopped by the drain
	go func() {
		sig, ok := <-sigs
		if !ok {
			return
		}
		if managed {
			dr.drainAndExit(sig.String())
			return
		}
		code := 1
		if s, isSys := sig.(syscall.Signal); isSys {
			code = 128 + int(s)
		}
		reason := "signal " + sig.String()
		if lt := r.lifetimeP.Load(); lt != nil {
			lt.shutdownWith(reason, code)
			return
		}
		outLog.Info(fmt.Sprintf("serve-http: unmanaged daemon exiting before its store opened: %s", reason))
		r.exitRow(tenancy.ExitSignal, reason, code)
		if r.perProcessLedger {
			removePerProcessLedger(r.lcIn.ledger)
		}
		r.removeRecord()
		os.Exit(code)
	}()
}

// startLifetime starts the lifetime tick and the parent-death watch.
func (r *serveRun) startLifetime() {
	outLog := r.outLog
	// RT-20/RT-21: the lifetime tick (heartbeat for every daemon; exit
	// bounds for an unmanaged one) and the parent-death watch, from the
	// bind — BEFORE the store wait, so an unmanaged daemon waiting for a
	// store that never comes (a mistyped --store) still keeps its --ttl,
	// --idle-exit and --exit-with-parent (I47, I48). Every lifetime exit
	// writes its exit row (not counted by the breaker).
	lifetime := &daemonLifetime{
		managed:    r.managed,
		bounds:     tenancy.UnmanagedBounds{IdleExit: r.idleExit, TTL: r.ttl},
		startedAt:  r.startedAt,
		act:        r.act,
		store:      identify(absStorePath(r.storePath)),
		recordPath: r.recordPath,
		now:        time.Now,
		logf:       func(m string) { outLog.Info(m) },
		exit:       os.Exit,
		stopped:    make(chan struct{}),
		onExit: func(reason string, code int) {
			r.exitRow(lifetimeExitReason(reason), reason, code)
			if r.perProcessLedger {
				removePerProcessLedger(r.lcIn.ledger)
			}
		},
	}
	r.lifetime = lifetime
	if exe, exeErr := os.Executable(); exeErr == nil {
		lifetime.exe = identify(exe)
	}
	r.lifetimeP.Store(lifetime)
	r.deferred(lifetime.stop)
	//fathom:tenancy-allow bare-go daemon lifetime goroutine started once at serve-http startup and stopped by the drain
	go lifetime.run(daemonLifetimeTick)
	if !r.managed && r.exitWithParent {
		if !watchParentExit(r.startPPID, parentExitPoll, os.Getppid, lifetime.shutdown) {
			outLog.Info(fmt.Sprintf("serve-http: parent-exit watch off (started by pid %d)", r.startPPID))
		}
	}
}
