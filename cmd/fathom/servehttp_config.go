package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// configure is the config-and-flags phase: the kill switch, the loggers,
// config.Load, the flag set, managed detection and the tenancy budget. It
// returns (code, true) when the process must exit.
func (r *serveRun) configure() (int, bool) {
	// RT-3 (design §9): the kill switch is the FIRST action — before the
	// loggers, config, store or bind — so a disabled managed daemon exits in
	// under 50 ms and launchd's relaunch costs nothing. Unmanaged daemons
	// ignore it (RT-167).
	if code, stop := checkKillSwitch(r.args, os.Getenv, time.Now(), r.stderr); stop {
		return code, true
	}

	// DG-4 (diagnostics sweep): one logger per stream, constructed FIRST so
	// every diagnostic below — including the ones that can fire before a
	// store or client exists — is timestamped and leveled instead of a bare
	// fmt.Fprintf. Two loggers, not one, because launchd pipes stdout and
	// stderr to SEPARATE files (daemon.go's plistBody, DG-6) — collapsing
	// them onto one writer would move today's stdout-only lines into
	// fathom-http.err and vice versa.
	r.flog = dlog.New(r.stderr, dlog.LevelFromEnv(os.Getenv), time.Now)
	r.outLog = dlog.New(r.stdout, dlog.LevelFromEnv(os.Getenv), time.Now)
	flog, outLog := r.flog, r.outLog
	// RT-20: the parent is the process that started this one; captured
	// before anything slow, so a parent that dies during startup is seen.
	r.startPPID = os.Getppid()
	r.startedAt = time.Now()

	loaded, cfgErr := config.Load(config.DefaultUserPath(), config.DefaultProjectPath())
	if cfgErr != nil {
		flog.Error(fmt.Sprintf("loading config: %v", cfgErr))
		return 1, true
	}
	r.loaded = loaded
	// SEC-2: a daemon started inside a repository reads its project layer;
	// every user-only field that layer set (store, addr, call_log,
	// auto_refresh, memory_limit, a profile's bin or endpoint) was dropped
	// at Load. Say so once, naming file and field, never the value.
	for _, w := range loaded.Warnings() {
		flog.Warn("config: " + w)
	}

	fs := newFlagSet("serve-http", r.stderr)
	storePath := fs.String("store", loaded.Config.Daemon.Store, "store path, .sqlite/.db (required; [daemon].store in config.toml, or --store)")
	addr := fs.String("addr", firstNonEmpty(loaded.Config.Daemon.Addr, "127.0.0.1:7575"), "listen address (host:port); loopback by default — surfaces are not a security boundary (amendment)")
	autoRefresh := fs.Bool("auto-refresh", daemonAutoRefreshDefault(loaded.Config.Daemon), "re-ingest a service when its git HEAD drifts from the indexed commit, before answering queries")
	callLogPath := fs.String("call-log", firstNonEmpty(loaded.Config.Daemon.CallLog, defaultCallLogPath()), "JSONL tool-call log path, resolved against the daemon working directory; empty to disable")
	fs.Var(&r.modulePaths, "module", "path to a conforming expert-module store (.sqlite/.db, docs/spec/expert-module-v0.md), mounted READ-ONLY so obligations unions in the rules of every module a service binds in .fathom/modules.cue; repeatable. A non-conforming or unreadable store aborts startup loudly (docs/plans/gke-module-host.md GKE-) rather than starting up silently degraded.")
	managedFlag := fs.Bool("managed", false, "this is the operator's managed (launchd/systemd-supervised) daemon (plist schema 2 passes it): it obeys ~/.fathom/daemon/disabled.json and paused.json ('fathom daemon stop|pause') and is the only daemon that watches its own binary for self-restart. A launchd job under the default label or an installed fathom agent's Label is detected as managed without it. Unmanaged daemons are bounded: --exit-with-parent, --idle-exit, --ttl, no auto-refresh by default, switch files ignored, and the managed daemon's store and call log refused")
	exitWithParent := fs.Bool("exit-with-parent", true, "unmanaged only: exit within 10 s of the parent process's death, SIGKILL included")
	idleExit := fs.Duration("idle-exit", tenancy.DefaultUnmanagedBounds().IdleExit, "unmanaged only: exit after this long with no request in flight; 0 disables")
	ttl := fs.Duration("ttl", tenancy.DefaultUnmanagedBounds().TTL, "unmanaged only: exit after this long regardless; 0 disables")
	allowOperatorState := fs.Bool("allow-operator-state", false, "unmanaged only: allow --store/--call-log to name the managed daemon's store or call log")
	lifecycleIDFlag := fs.String("lifecycle-id", "", "unmanaged only: keep the lifecycle ledger (and the crash-loop breaker) under ~/.fathom/daemon/<id>/ across relaunches — used by the stress harness; never a managed label. Default: a per-process addr-<port>-<pid> ledger")

	if err := fs.Parse(r.args); err != nil {
		return 1, true
	}
	r.storePath, r.addr, r.callLogPath, r.autoRefresh = *storePath, *addr, *callLogPath, *autoRefresh
	r.managedFlag, r.exitWithParent, r.allowOperatorState = *managedFlag, *exitWithParent, *allowOperatorState
	r.idleExit, r.ttl, r.lifecycleIDFlag = *idleExit, *ttl, *lifecycleIDFlag

	if r.storePath == "" {
		flog.Error("--store is required")
		fs.Usage()
		return 1, true
	}

	// §10.1: managed or unmanaged decides every tenancy default below.
	managed, managedLabel := detectManaged(r.managedFlag, os.Getenv, installedAgentLabels)
	r.managed, r.managedLabel = managed, managedLabel
	// §8.4 step 5 (O1363): a managed start records whether its supervisor
	// runs it; a hand start is named, not refused (other supervisors exist).
	if managed {
		sup := detectSupervised(runtime.GOOS, managedLabel, os.Getenv, r.startPPID)
		r.supervised = &sup
		if !sup {
			flog.Warn(unsupervisedStartLine(runtime.GOOS, managedLabel, r.startPPID))
		}
	}
	// RT-6: the managed daemon re-reads paused.json every 2 s and moves to
	// and from safe mode. XPC_SERVICE_NAME covers a schema-1 plist that
	// predates --managed.
	if managed {
		if dir, dirErr := daemonStateDir(); dirErr == nil {
			//fathom:tenancy-allow bare-go daemon lifetime goroutine started once at serve-http startup and stopped by the drain
			go watchPauseSwitch(dir, 2*time.Second, time.Now, flog, nil)
		}
	}
	// LB-F26: only the managed daemon keeps compiled grammar code on disk,
	// at user scope (~/.fathom/cache/wazero/<version>/, aged by `fathom
	// gc`); an unmanaged daemon compiles in memory and leaves nothing.
	if managed {
		if home, hErr := os.UserHomeDir(); hErr == nil {
			if cErr := tsengine.EnableCompilationCache(home); cErr != nil {
				flog.Warn("serve-http: wazero compilation cache off: " + cErr.Error())
			}
		}
	}
	explicit := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	if !managed {
		// A scratch daemon never re-ingests real repositories unless asked.
		if !explicit["auto-refresh"] {
			r.autoRefresh = false
		}
		if !r.allowOperatorState {
			// The managed daemon's paths from its record and its plist,
			// under HOME and the account's own home (a swapped HOME holds
			// neither); an unreadable managed record refuses (I38).
			homes := operatorStateHomes()
			addrHome := ""
			if len(homes) > 0 {
				addrHome = homes[len(homes)-1] // the account's home when HOME is swapped
			}
			warns, err := checkOperatorState(r.storePath, r.callLogPath, homes, managedDaemonAddrIn(addrHome, loaded.Config.Daemon.Addr))
			for _, w := range warns {
				flog.Warn(w)
			}
			if err != nil {
				flog.Error(err.Error())
				return 1, true
			}
		}
		outLog.Info(fmt.Sprintf("serve-http: unmanaged (idle-exit %s, ttl %s, exit-with-parent %v, auto-refresh %v) — pass --managed for a supervised daemon", r.idleExit, r.ttl, r.exitWithParent, r.autoRefresh))
	}

	// RT-29/RT-30/RT-177 (design §1, §2): the budget — user-scope
	// [tenancy] and env only; an unmanaged daemon takes its own
	// [tenancy.unmanaged] budget. Every mark is derived and named.
	tenancyCfg, tenancyNotes, tenancyErr := config.ResolveTenancyConfig(loaded, os.Getenv)
	if tenancyErr != nil {
		flog.Error(tenancyErr.Error())
		return 1, true
	}
	for _, n := range tenancyNotes {
		flog.Warn("tenancy: " + n)
	}
	budget, budgetNotes := tenancy.ResolveBudget(tenancyCfg, tenancy.HostFacts{RAM: hostRAM(), NumCPU: runtime.NumCPU(), Managed: managed})
	r.budget, r.budgetNotes = budget, budgetNotes
	outLog.Info(tenancyStartupLine(budget, budgetNotes))
	// The lifecycle modes (safe mode, warm-up, store distress) are
	// process-wide; an in-process caller (a test) never inherits them.
	r.deferred(tenancy.ResetLifecycleState)
	serveWarmupGate.mode = func() string { return budget.Admission }
	serveWarmupGate.logf = func(m string) { flog.Info(m) }
	return 0, false
}
