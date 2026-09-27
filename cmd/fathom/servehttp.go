package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/capability"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/modulehost"
	"github.com/geoffmilleraz/fathom/internal/refresh"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// stringSliceFlag is a flag.Value collecting one or more occurrences of a
// repeatable flag (e.g. --module path1 --module path2) into a slice, in
// the order given.
type stringSliceFlag []string

func (f *stringSliceFlag) String() string { return strings.Join(*f, ",") }
func (f *stringSliceFlag) Set(v string) error {
	*f = append(*f, v)
	return nil
}

// runServeHTTP is the testable entry point for the "serve-http" subcommand.
//
// CF-7 (docs/plans/config-experience.md): the same user/project
// config.toml layers compile/providers/smoke read are loaded here too, BEFORE
// flag parsing, and feed flag DEFAULTS ([daemon] table: store, addr,
// call_log, auto_refresh) — a flag on the actual invocation still wins
// (flags > env > project > user > defaults). This is what makes
// compile_layer work on the always-on daemon with zero plist env: a launchd
// plist carries no env at all today, which is exactly the "SILENTLY
// DISABLED" gap the file's grounding names. Restart-to-apply: this load
// happens once, at process start.
func runServeHTTP(args []string, stdout, stderr io.Writer) int {
	// RT-3 (design §9): the kill switch is the FIRST action — before the
	// loggers, config, store or bind — so a disabled managed daemon exits in
	// under 50 ms and launchd's relaunch costs nothing. Unmanaged daemons
	// ignore it (RT-167).
	if code, stop := checkKillSwitch(args, os.Getenv, time.Now(), stderr); stop {
		return code
	}

	// DG-4 (diagnostics sweep): one logger per stream, constructed FIRST so
	// every diagnostic below — including the ones that can fire before a
	// store or client exists — is timestamped and leveled instead of a bare
	// fmt.Fprintf. Two loggers, not one, because launchd pipes stdout and
	// stderr to SEPARATE files (daemon.go's plistBody, DG-6) — collapsing
	// them onto one writer would move today's stdout-only lines into
	// fathom-http.err and vice versa.
	flog := dlog.New(stderr, dlog.LevelFromEnv(os.Getenv), time.Now)
	outLog := dlog.New(stdout, dlog.LevelFromEnv(os.Getenv), time.Now)
	// RT-20: the parent is the process that started this one; captured
	// before anything slow, so a parent that dies during startup is seen.
	startPPID := os.Getppid()
	startedAt := time.Now()

	loaded, cfgErr := config.Load(config.DefaultUserPath(), config.DefaultProjectPath())
	if cfgErr != nil {
		flog.Error(fmt.Sprintf("loading config: %v", cfgErr))
		return 1
	}

	fs := newFlagSet("serve-http", stderr)
	storePath := fs.String("store", loaded.Config.Daemon.Store, "store path, .sqlite/.db (required; [daemon].store in config.toml, or --store)")
	addr := fs.String("addr", firstNonEmpty(loaded.Config.Daemon.Addr, "127.0.0.1:7575"), "listen address (host:port); loopback by default — surfaces are not a security boundary (ADR-014 amendment)")
	autoRefresh := fs.Bool("auto-refresh", daemonAutoRefreshDefault(loaded.Config.Daemon), "re-ingest a service when its git HEAD drifts from the indexed commit, before answering queries")
	callLogPath := fs.String("call-log", firstNonEmpty(loaded.Config.Daemon.CallLog, defaultCallLogPath()), "JSONL tool-call log path, resolved against the daemon working directory; empty to disable")
	var modulePaths stringSliceFlag
	fs.Var(&modulePaths, "module", "path to a conforming expert-module store (.sqlite/.db, docs/spec/expert-module-v0.md), mounted READ-ONLY on POST /rpc/modules; repeatable. A non-conforming or unreadable store aborts startup loudly (docs/plans/gke-module-host.md GKE-MH-1) rather than starting up silently degraded.")
	managedFlag := fs.Bool("managed", false, "this is the operator's managed (launchd/systemd-supervised) daemon (§10.1; plist schema 2 passes it): it obeys ~/.fathom/daemon/disabled.json and paused.json (`fathom daemon stop|pause`) and is the only daemon that watches its own binary for self-restart. A launchd job under the default label or an installed fathom agent's Label is detected as managed without it. Unmanaged daemons are bounded: --exit-with-parent, --idle-exit, --ttl, no auto-refresh by default, switch files ignored, and the managed daemon's store and call log refused")
	exitWithParent := fs.Bool("exit-with-parent", true, "unmanaged only: exit within 10 s of the parent process's death, SIGKILL included (RT-20)")
	idleExit := fs.Duration("idle-exit", tenancy.DefaultUnmanagedBounds().IdleExit, "unmanaged only: exit after this long with no request in flight; 0 disables (RT-21)")
	ttl := fs.Duration("ttl", tenancy.DefaultUnmanagedBounds().TTL, "unmanaged only: exit after this long regardless; 0 disables (RT-21)")
	allowOperatorState := fs.Bool("allow-operator-state", false, "unmanaged only: allow --store/--call-log to name the managed daemon's store or call log (RT-22)")
	lifecycleIDFlag := fs.String("lifecycle-id", "", "unmanaged only: keep the lifecycle ledger (and the crash-loop breaker) under ~/.fathom/daemon/<id>/ across relaunches — the stress harness's (RT-158); never a managed label. Default: a per-process addr-<port>-<pid> ledger")
	modulesTokenEnv := fs.String("modules-token-env", "", "name of an environment variable holding the bearer token required for POST /rpc/modules (GKE-MH-7); unset = auth disabled (default — matches the loopback-only daemon's existing unauthenticated trust boundary). The token itself is never a flag or config value, only this env var's NAME.")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *storePath == "" {
		flog.Error("--store is required")
		fs.Usage()
		return 1
	}

	// §10.1: managed or unmanaged decides every tenancy default below.
	managed, managedLabel := detectManaged(*managedFlag, os.Getenv, installedAgentLabels)
	// RT-6: the managed daemon re-reads paused.json every 2 s and moves to
	// and from safe mode. XPC_SERVICE_NAME covers a schema-1 plist that
	// predates --managed.
	if managed {
		if dir, dirErr := daemonStateDir(); dirErr == nil {
			go watchPauseSwitch(dir, 2*time.Second, time.Now, flog, nil)
		}
	}
	explicit := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	if !managed {
		// A scratch daemon never re-ingests real repositories unless asked.
		if !explicit["auto-refresh"] {
			*autoRefresh = false
		}
		if !*allowOperatorState {
			// The managed daemon's paths from its record and its plist,
			// under HOME and the account's own home (a swapped HOME holds
			// neither); an unreadable managed record refuses (I38).
			homes := operatorStateHomes()
			addrHome := ""
			if len(homes) > 0 {
				addrHome = homes[len(homes)-1] // the account's home when HOME is swapped
			}
			warns, err := checkOperatorState(*storePath, *callLogPath, homes, managedDaemonAddrIn(addrHome, loaded.Config.Daemon.Addr))
			for _, w := range warns {
				flog.Warn(w)
			}
			if err != nil {
				flog.Error(err.Error())
				return 1
			}
		}
		outLog.Info(fmt.Sprintf("serve-http: unmanaged (idle-exit %s, ttl %s, exit-with-parent %v, auto-refresh %v) — pass --managed for a supervised daemon", *idleExit, *ttl, *exitWithParent, *autoRefresh))
	}

	// RT-29/RT-30/RT-177 (design §1, §2): the budget — user-scope
	// [tenancy] and env only; an unmanaged daemon takes its own
	// [tenancy.unmanaged] budget. Every mark is derived and named.
	tenancyCfg, tenancyNotes, tenancyErr := config.ResolveTenancyConfig(loaded, os.Getenv)
	if tenancyErr != nil {
		flog.Error(tenancyErr.Error())
		return 1
	}
	for _, n := range tenancyNotes {
		flog.Warn("tenancy: " + n)
	}
	budget, budgetNotes := tenancy.ResolveBudget(tenancyCfg, tenancy.HostFacts{RAM: hostRAM(), NumCPU: runtime.NumCPU(), Managed: managed})
	outLog.Info(tenancyStartupLine(budget, budgetNotes))
	// The lifecycle modes (safe mode, warm-up, store distress) are
	// process-wide; an in-process caller (a test) never inherits them.
	defer tenancy.ResetLifecycleState()
	serveWarmupGate.mode = func() string { return budget.Admission }
	serveWarmupGate.logf = func(m string) { flog.Info(m) }

	// RT-37/RT-38/RT-158 (design §2.3, §8.4 step 2): the lifecycle ledger
	// and the crash-loop breaker — after the kill switch and the
	// [tenancy] resolution (the breaker's thresholds and the ledger id come
	// from them), before the store and the bind. An unmanaged daemon
	// without --lifecycle-id gets its per-process ledger after the bind
	// (its id names the bound port).
	lcID, lcErr := lifecycleID(*lifecycleIDFlag, managed, managedLabel, installedAgentLabels())
	if lcErr != nil {
		flog.Error(lcErr.Error())
		return 1
	}
	pid := os.Getpid()
	commit := statuszBuildInfo().BinaryCommit
	lcIn := lifecycleStart{
		managed: managed, addr: *addr, pid: pid, commit: commit, version: Version,
		breaker:  budget.Breaker,
		alive:    func(p int) bool { return !pidGone(p) },
		evidence: func(since time.Time) []tenancy.EvidenceFile { return scanUncleanEvidence(evidenceDirs(), since) },
		logf:     func(m string) { flog.Warn(m) },
	}
	if managed {
		if dir, dirErr := daemonStateDir(); dirErr == nil {
			lcIn.switchDir = dir
		}
		lcIn.relaunch = func(rec tenancy.SwitchRecord) []string { return relaunchClause(realDaemonOps{}, managedLabel, rec) }
		if _, _, post, ok := installedPosture(managedLabel); ok {
			lcIn.plistSchema = post.Schema
		}
	}
	begin := func(id string) (lifecycleOutcome, bool) {
		home, hErr := os.UserHomeDir()
		if hErr != nil || home == "" || id == "" {
			return lifecycleOutcome{}, false
		}
		p, pErr := ledgerPathFor(home, id)
		if pErr != nil {
			flog.Warn(fmt.Sprintf("lifecycle: no ledger (%v)", pErr))
			return lifecycleOutcome{}, false
		}
		lcIn.ledger, lcIn.id, lcIn.now = p, id, time.Now()
		return beginLifecycle(lcIn), true
	}
	// lcOut is the breaker's verdict for this start: only its safe mode
	// (never paused.json's) steps down after 30 min at normal.
	var lcOut lifecycleOutcome
	if lcID != "" {
		if out, ok := begin(lcID); ok {
			lcOut = out
			if out.stop {
				return out.code
			}
			if out.safe != nil {
				applySafeMode(out.safe, func(m string) { flog.Warn(m) })
			}
		}
	}
	// Warm-up after any start (§2.3, RT-40): heavy classes get one slot
	// each and auto-refresh runs no inline ingest.
	tenancy.SetWarmup(startedAt, budget.Breaker.Warmup)
	if ws, wd := tenancy.Warmup(); !ws.IsZero() {
		flog.Info(fmt.Sprintf("tenancy: warm-up %s — one slot per heavy class and no inline auto-refresh ingest until %s", wd, ws.Add(wd).Format(time.TimeOnly)))
	}
	exitRow := func(reason, detail string, code int) {
		if lcIn.ledger == "" {
			return
		}
		row := tenancy.NewLifecycleRow(tenancy.RowExit, time.Now(), pid, Version)
		row.Code, row.Reason, row.Detail = tenancy.IntPtr(code), reason, detail
		row.Uptime = time.Since(startedAt).Round(time.Second).String()
		if err := appendLifecycle(lcIn.ledger, row); err != nil {
			flog.Warn(fmt.Sprintf("lifecycle: exit row not written: %v", err))
		}
	}

	// GKE-MH-7: resolve the modules bearer token BEFORE opening anything —
	// a --modules-token-env naming an empty/unset environment variable is a
	// misconfiguration that must fail loudly, never silently leave auth
	// disabled on what the operator believes is a protected path.
	modulesToken := ""
	if *modulesTokenEnv != "" {
		modulesToken = os.Getenv(*modulesTokenEnv)
		if modulesToken == "" {
			msg := fmt.Sprintf("--modules-token-env=%s is set but that environment variable is empty or unset", *modulesTokenEnv)
			flog.Error(msg)
			exitRow(tenancy.ExitStartFailed, msg, 1)
			return 1
		}
	}

	// GKE-MH-1: mount every --module store read-only, conformance-checked
	// against LevelL0 (the module host's floor — sufficient for
	// internal/semantic.ObligationsFor and the staleness-report shape; see
	// store.ConformanceLevel's doc for why Server conformance itself has no
	// separate schema footprint to check). A refusal aborts startup loudly
	// — never a partial mount, never a silently-degraded module. It is
	// configuration, checked before the bind.
	var moduleHost *modulehost.Host
	if len(modulePaths) > 0 {
		moduleHost = modulehost.NewHost(store.LevelL0)
		for _, p := range modulePaths {
			if _, mErr := moduleHost.Mount(p, ""); mErr != nil {
				msg := fmt.Sprintf("mounting module %s: %v", p, mErr)
				flog.Error(msg)
				exitRow(tenancy.ExitStartFailed, msg, 1)
				return 1
			}
			flog.Info(fmt.Sprintf("serve-http: mounted module %s (%s)", p, modulehost.ModuleNameFromPath(p)))
		}
	}

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
	defer func() {
		debug.SetMemoryLimit(prevLimit)
		runtime.GOMAXPROCS(prevProcs)
	}()
	applyMemoryLimit(budget, stdout, debug.SetMemoryLimit, runtime.GOMAXPROCS, os.Getenv)

	// RT-42 (design §8.4 step 3): bind BEFORE the store. DG-19 (owner
	// amendment c): a self-restart or `task deploy` exits the OLD daemon
	// so launchd starts the new one — if the old process is still tearing
	// down when the new one races up, the bind can lose to EADDRINUSE even
	// though nothing is permanently wrong. Retry the BIND ONLY, with
	// backoff, for up to bindRetryMaxWait; then RT-43: a managed fathom of
	// the same label already serves the port (exit 0), any other holder
	// is named (exit 75).
	listener, listenErr := listenTCPWithRetry(*addr, flog)
	if listenErr != nil {
		flog.Error(fmt.Sprintf("http serve: %v", listenErr))
		if isAddrInUse(listenErr) {
			code, reason, detail := resolvePortHolder(managed, managedLabel, gatherPortHolder(*addr, realDaemonOps{}))
			flog.Error("serve-http: " + detail)
			exitRow(reason, detail, code)
			return code
		}
		exitRow(tenancy.ExitStartFailed, "bind "+*addr+": "+listenErr.Error(), 1)
		return 1
	}
	outLog.Info(fmt.Sprintf("serve-http: listening on %s", *addr))
	if lcIn.ledger == "" && !managed {
		// The unmanaged per-process ledger: addr-<port>-<pid>, beside the
		// record; removed with it on a clean exit.
		if out, ok := begin(daemonID(daemonMeta{}, listener.Addr().String(), pid)); ok {
			lcOut = out
			if out.stop {
				_ = listener.Close()
				return out.code
			}
		}
	}
	setCrashOutput(lcIn.ledger, func(m string) { flog.Warn(m) })

	// OC-2: record this daemon (its absolute --call-log, store, pid,
	// commit) only now that the listener is bound, so a verb run from any
	// directory reads the log this daemon writes and a serve-http that
	// failed to start never leaves a record behind — and before the store,
	// so `fathom daemon stop` finds a daemon waiting for its store.
	prevRecord, hadPrev := readDaemonRecordFor("http://" + *addr)
	meta := daemonMeta{Managed: managed, Label: managedLabel, PPID: startPPID, LifecycleID: lcID}
	if !managed {
		meta.IdleExit, meta.TTL = *idleExit, *ttl
	}
	rec, recordPath, recErr := recordServingDaemonAs(meta, listener.Addr().String(), *callLogPath, *storePath, commit, time.Now())
	if recErr != nil {
		flog.Warn(fmt.Sprintf("serve-http: daemon record not written (%v) — verbs fall back to config and the legacy call-log path", recErr))
	} else {
		outLog.Info(fmt.Sprintf("serve-http: daemon record: %s (call log %s)", recordPath, rec.CallLog))
	}

	// Serve from now on: the waiting handler until the store opens, then
	// the full one — one listener, one server, swapped in place.
	var waitState atomic.Pointer[storeWaitState]
	swap := newSwapHandler(waitingHandler(&waitState))
	// Activity (the unmanaged idle bound's input) is counted from the
	// bind, the store wait included.
	act := newActivity(time.Now)
	// RT-59 (§3.5): header, idle and per-route write timeouts and the
	// loopback-Host guard; the managed daemon answers a test caller 409
	// (RT-63).
	httpSrv := newDaemonServer(act.middleware(swap), daemonServerOpts{managed: managed, hosted: modulesToken != ""})
	serveErr := make(chan error, 1)
	go func() { serveErr <- httpSrv.Serve(listener) }()
	stopServing := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(ctx)
	}

	// Signals are handled from the bind on (RT-42 moved the bind and the
	// record ahead of the store, so a SIGTERM during a store wait must not
	// take the default action and leave the record behind). Until the
	// lifetime exists the handler removes the record itself.
	perProcessLedger := !managed && lcID == "" && lcIn.ledger != ""
	var lifetimeP atomic.Pointer[daemonLifetime]
	removeRecord := func() {
		if lt := lifetimeP.Load(); lt != nil {
			lt.closeRecord()
			return
		}
		removeDaemonRecord(recordPath)
	}
	// RT-97 (§7.1): the refresh queue's worker lives until the drain (or
	// return) stops it; it is started once the backend exists below.
	refreshCtx, stopRefresh := context.WithCancel(context.Background())
	defer stopRefresh()
	// RT-44 (§8.5): a managed daemon's SIGTERM drains and exits 0.
	dr := &drainer{
		stopRefresh: stopRefresh,
		ledger:      lcIn.ledger, pid: pid, version: Version, startedAt: startedAt, now: time.Now,
		inflight: tenancy.DefaultInFlight, budget: drainBudget, logf: func(m string) { flog.Warn(m) },
		shutdown:     httpSrv.Shutdown,
		removeRecord: removeRecord,
		exit:         os.Exit,
	}
	sigs := make(chan os.Signal, 1)
	if managed {
		signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	} else {
		signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	}
	defer func() {
		signal.Stop(sigs)
		close(sigs) // the handler below returns (an in-process caller)
	}()
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
		if lt := lifetimeP.Load(); lt != nil {
			lt.shutdownWith(reason, code)
			return
		}
		outLog.Info(fmt.Sprintf("serve-http: unmanaged daemon exiting before its store opened: %s", reason))
		exitRow(tenancy.ExitSignal, reason, code)
		if perProcessLedger {
			removePerProcessLedger(lcIn.ledger)
		}
		removeRecord()
		os.Exit(code)
	}()

	// RT-20/RT-21: the lifetime tick (heartbeat for every daemon; exit
	// bounds for an unmanaged one) and the parent-death watch, from the
	// bind — BEFORE the store wait, so an unmanaged daemon waiting for a
	// store that never comes (a mistyped --store) still keeps its --ttl,
	// --idle-exit and --exit-with-parent (I47, I48). Every lifetime exit
	// writes its exit row (not counted by the breaker).
	lifetime := &daemonLifetime{
		managed:    managed,
		bounds:     tenancy.UnmanagedBounds{IdleExit: *idleExit, TTL: *ttl},
		startedAt:  startedAt,
		act:        act,
		store:      identify(absStorePath(*storePath)),
		recordPath: recordPath,
		now:        time.Now,
		logf:       func(m string) { outLog.Info(m) },
		exit:       os.Exit,
		stopped:    make(chan struct{}),
		onExit: func(reason string, code int) {
			exitRow(lifetimeExitReason(reason), reason, code)
			if perProcessLedger {
				removePerProcessLedger(lcIn.ledger)
			}
		},
	}
	if exe, exeErr := os.Executable(); exeErr == nil {
		lifetime.exe = identify(exe)
	}
	lifetimeP.Store(lifetime)
	defer lifetime.stop()
	go lifetime.run(daemonLifetimeTick)
	if !managed && *exitWithParent {
		if !watchParentExit(startPPID, parentExitPoll, os.Getppid, lifetime.shutdown) {
			outLog.Info(fmt.Sprintf("serve-http: parent-exit watch off (started by pid %d)", startPPID))
		}
	}

	// RT-42 (§8.4 step 4): open the store, waiting — never exiting — while
	// it is unavailable: /healthz and /statusz answer with the errno,
	// tools/call is 503, reopen with backoff 5 s → 5 min, one store_wait
	// row (not counted) per distinct errno, a TCC wedge labelled apart from
	// an unmounted volume with the daemon binary's own remedy (RT-165).
	exePath, _ := os.Executable()
	lastWaitErrno := ""
	storeCtx, cancelStoreWait := context.WithCancel(context.Background())
	defer cancelStoreWait()
	c, err := openStoreWithWait(storeCtx, storeWaitDeps[*client.Client]{
		path:  absStorePath(*storePath),
		open:  func() (*client.Client, error) { return client.NewConstantClient(client.ModeLocal, *storePath) },
		stat:  statPath,
		probe: probeStoreOpen,
		sleep: sleepCtx,
		now:   time.Now,
		exe:   exePath,
		codesign: func(exe string) signingIdentity {
			return codesignDescribe(exe)
		},
		onWait: func(s storeWaitState) {
			if s.Open {
				waitState.Store(nil)
				outLog.Info(fmt.Sprintf("serve-http: store opened after %d attempt(s) waiting (%s, %s)", s.Attempts, s.Errno, s.Label))
				return
			}
			cp := s
			waitState.Store(&cp)
			flog.Warn(fmt.Sprintf("serve-http: waiting-for-store: %s (%s, %s) — %s; retry in %s", s.Err, s.Errno, s.Label, s.Remedy, s.NextRetry))
			if s.Errno != lastWaitErrno && lcIn.ledger != "" {
				lastWaitErrno = s.Errno
				row := tenancy.NewLifecycleRow(tenancy.RowStoreWait, time.Now(), pid, Version)
				row.Errno, row.Label, row.Detail = s.Errno, s.Label, s.Err+" — "+s.Remedy
				if werr := appendLifecycle(lcIn.ledger, row); werr != nil {
					flog.Warn(fmt.Sprintf("lifecycle: store_wait row not written: %v", werr))
				}
			}
		},
	})
	if err != nil {
		// A store that opened and is wrong (not unavailability): a failed
		// start — counted by the breaker, so launchd's relaunch loop is
		// bounded — and the record written at the bind goes with it.
		flog.Error(fmt.Sprintf("opening client: %v", err))
		stopServing()
		exitRow(tenancy.ExitStartFailed, "store: "+err.Error(), 1)
		if perProcessLedger {
			removePerProcessLedger(lcIn.ledger)
		}
		removeRecord()
		return 1
	}
	defer c.Close()
	c.WithLogger(flog) // DG-4: the typed-edge pass's own diagnostics gain a timestamp/level too.

	// Semantic-layer compilation over HTTP (the compile_layer tool) needs an
	// LLM. Wire one when a provider is configured EITHER the pre-profile way
	// (ANTHROPIC_API_KEY, or FATHOM_LLM_PROVIDER set at all) OR via a
	// config-file profile ([daemon].default_profile, else the top-level
	// default_profile, else no profile at all — matching
	// resolveActiveProfile's own fallback chain). Without any of these,
	// dry-run estimation still works and spending compiles return a clear
	// error. Model: compileModelForProfile (FATHOM_COMPILE_MODEL beats the
	// profile's exact pin, which beats the cheap-tier default — ADR-016 §6).
	daemonProfileName := loaded.Config.Daemon.DefaultProfile
	if daemonProfileName == "" {
		daemonProfileName = loaded.Config.DefaultProfile
	}
	hasProfileConfig := daemonProfileName != "" || len(loaded.Config.Profiles) > 0
	if os.Getenv("ANTHROPIC_API_KEY") != "" || os.Getenv("FATHOM_LLM_PROVIDER") != "" || hasProfileConfig {
		p, _, profErr := config.ResolveProfile(loaded.Config, loaded.Provenance, daemonProfileName)
		if profErr != nil {
			// DG-4: names the profile and the remedy once per start — this
			// path runs exactly once during startup, so "once per start" is
			// automatic; profErr already names which profile failed to
			// resolve and why.
			flog.Warn(fmt.Sprintf("compile_layer disabled: %v", profErr))
		} else if llmCl, llmErr := buildLLMClientForProfile(p); llmErr == nil {
			model := compileModelForProfile(p)
			c.WithLLM(llmCl, model, knownModelInRatePerKTok, knownModelOutRatePerKTok)
			flog.Info(fmt.Sprintf("serve-http: compile_layer enabled (model %s)", model))
		} else {
			flog.Warn(fmt.Sprintf("compile_layer disabled: %v", llmErr))
		}
	}

	// ADR-042/DP-9: the decision port — the compile pre-filter and the
	// obligations re-rank both read it through the Client. Fail-open by
	// construction (openDecisionPorts never errors outward): a
	// misconfigured or unreachable [decide] provider warns and falls back
	// to decision.None rather than blocking startup (I14). RT-39: closed in
	// safe mode and at refuse and above (gatedDecider).
	ports := openDecisionPorts(loaded, daemonProfileName, *callLogPath, "serve-http", stderr)
	// RT-39: closed in safe mode and at refuse and above; RT-65: the
	// client's decision-port calls (the obligations re-rank, the compile
	// pre-filter) are also admitted as egress once the gate exists. /route
	// admits its own call, so ports.Decider only closes.
	ports.Decider = closingDecider(ports.Decider)
	c.WithDecision(gateDecider(ports.Decider, "client"), ports.Cfg, ports.Recorder)

	// Default: keep the shared index fresh transparently. A query for a service
	// whose git HEAD has drifted from the indexed commit triggers a hash-gated
	// re-ingest before the answer — so the always-on daemon never serves a stale
	// index without anyone noticing.
	// RT-97 (§7.1): one refresh queue for every whole-service ingest the
	// daemon runs — auto-refresh, the ingest tool, POST /refresh-request
	// (the post-commit hook) and /refresh-file — one per service, one heavy
	// slot, background work held 60 s after this start and staggered during
	// warm-up.
	// RT-106 (§7.2): every `go` this daemon execs (the typed pass's go
	// list, go env) runs through a shim at background priority with
	// GOFLAGS=-p=2 and GOMAXPROCS=2.
	if shimDir, err := os.MkdirTemp("", "fathom-toolchain-"); err == nil {
		if undo, ierr := fusion.InstallBackgroundToolchain(shimDir); ierr != nil {
			flog.Warn(fmt.Sprintf("serve-http: toolchain children run at normal priority: %v", ierr))
			_ = os.RemoveAll(shimDir)
		} else {
			defer undo()
		}
	}
	refreshQ := mcp.NewRefreshQueue(c, refresh.Config{Started: startedAt})
	refreshQ.Start(refreshCtx)
	var backend mcp.QueryInterface = c
	if *autoRefresh {
		backend = mcp.NewAutoRefreshQueued(c, flog, refreshQ)
		// RT-109: an in_progress row is an interrupted ingest only once
		// its owner is gone (a live CLI ingest is never duplicated).
		mcp.SetIngestOwnerProbe(ingestOwnerProbe(realDaemonOps{}))
	}

	// Call logging feeds tool-selection telemetry (which tools each surface
	// actually uses); failure to open the log never blocks serving.
	var logger *mcp.CallLogger
	if *callLogPath != "" {
		f, err := openCallLog(*callLogPath)
		if err != nil {
			flog.Warn(fmt.Sprintf("call log disabled: %v", err))
		} else {
			defer f.Close()
			logger = mcp.NewCallLogger(f)
		}
	}

	var srv *mcp.Server
	fleet := &fleetAdapter{c: c, log: flog}
	if moduleHost != nil {
		srv = mcp.NewServerWithModules(backend, fleet, logger, moduleHost)
	} else {
		srv = mcp.NewServerWithFleetAndLogger(backend, fleet, logger)
	}
	// The daemon answers its own service registry in process: a page render
	// never round-trips list_services over loopback HTTP (it timed out under
	// load and rendered project pages as "store unreadable") and never writes
	// a call-log row of its own.
	// RT-53/RT-54 (design §3.2): the admission gate over the budget's
	// slots, in [tenancy].admission's mode (observe until RT-96).
	gate := tenancy.NewGate(tenancy.GateConfig{
		Budget:   budget,
		Mode:     tenancy.ParseMode(budget.Admission),
		SafeMode: func() bool { _, on := tenancy.SafeModeNow(); return on }, // the breaker's or paused.json's (RT-39)
		Log:      func(m string) { flog.Warn(m) },
	})
	srv.SetGate(gate)
	srv.SetRefreshQueue(refreshQ)
	daemonGate.Store(gate)
	defer daemonGate.Store(nil)
	decisionAdmit.Store(&decisionAdmitter{admit: srv.AdmitDecision})
	defer decisionAdmit.Store(nil)
	if gate.Mode() == tenancy.ModeOff {
		flog.Warn("serve-http: ADMISSION OFF (FATHOM_TENANCY_ADMISSION=off or [tenancy].admission = \"off\") — no call is decided or refused by class (class deadlines still apply); doctor's admission_mode row warns on every run")
	} else {
		outLog.Info(fmt.Sprintf("serve-http: admission %s (slots %s)", gate.Mode(), admissionSlotsLine(budget)))
	}
	// RT-61 (§3.3): the registry read goes through admission as
	// console-inprocess.
	listServicesClass, _ := srv.ToolCostClass("list_services", nil)
	// A refused read (any fleet call running) answers the last good
	// projection, disclosed as stale (§3.4).
	registry := newLastGoodRegistry(func(ctx context.Context) (map[string]svcInfo, error) {
		var out map[string]svcInfo
		err := inProcessToolCall(ctx, "list_services", listServicesClass, func(ctx context.Context) error {
			out = svcInfoFromSummaries(mcp.ServiceSummaries(ctx, backend, fleet))
			return nil
		})
		return out, err
	})
	// BA-9: budget defaults are project-settable. The `budgets` block of
	// <cwd>/.fathom/capabilities.cue (the daemon's working repo) and
	// ~/.fathom/capabilities.cue sizes each tool class in tokens, resolved
	// project → user → built-in inside the server. A broken block is
	// reported loudly and the measured built-in stands for THAT scope —
	// never a silent partial read. Restart-to-apply, like config.toml.
	budgets, budgetErrs := capability.LoadBudgetResolution(capability.DefaultUserPath(), capability.DefaultProjectPath())
	for _, e := range budgetErrs {
		flog.Warn(fmt.Sprintf("%v — built-in budget defaults stand for that scope", e))
	}
	srv.SetBudgets(budgets)
	// GKE-MH-7 (critic finding 2): a configured modules token means this is
	// a hosted, network-reachable deployment — /rpc/modules must serve ONLY
	// its own curated allowlist (never GroupShared's fleet-backend reads or
	// write/spend tools), off-surface calls there must hard-gate (never
	// execute-and-flag), and module tools must be unreachable via any other
	// path (bare /rpc, another surface, a batch sub-call). Set BEFORE the
	// handler is swapped in — never toggled while serving.
	if modulesToken != "" {
		srv.SetHostedModules(true)
	}
	// The console's dependencies (vision 35): serve-http owns them so the
	// event feeders below publish to the same bus GET /events streams.
	consoleDeps := newConsoleDeps(c, "http://"+*addr, time.Now())
	consoleDeps.storePath = absStorePath(*storePath)
	inProcessReg.Store(&inProcessRegistry{self: "http://" + *addr, services: registry.services, stale: registry.staleNote})
	handler := newHTTPHandlerWithAuthDeps(srv, c, modulesToken, *addr, ports, flog, consoleDeps)

	// Adopt a new binary without a human (selfrestart.go). launchd
	// relaunches the job after the adoption exit (75, RT-11), which is what
	// lets an agent complete a deploy end to end, rather than building a
	// fix and then waiting for someone to run `launchctl`. Only the managed
	// daemon watches (RT-12): a scratch serve-http never adopts, never
	// hashes its binary and never writes the operator's lifecycle ledger.
	startSelfRestart(outLog, selfRestartStart{
		managedFlag:     *managedFlag,
		xpcServiceName:  os.Getenv("XPC_SERVICE_NAME"),
		installedLabels: installedAgentLabels,
		disabled:        selfRestartDisabled(),
		executable:      os.Executable,
		watch: func(path string, cfg selfRestartConfig) {
			go watchOwnBinary(path, outLog, cfg)
		},
	})

	// Periodic OS-memory return (memscavenge.go, O108): bounds how long any
	// one peak (a large graph build, a burst of concurrent cache misses)
	// can inflate the daemon's OS-visible footprint, instead of leaving it
	// to Go's own lazy background scavenger for a process that runs 24/7.
	// DG-6 piggybacks the daemon's own launchd-piped stdout/stderr bound on
	// this same ticker (rotateDaemonStdio) — an operator who turns
	// scavenging off also pauses log rotation, a deliberate coupling
	// (matching DG-2's dedupe-flush cadence) rather than a second
	// independent ticker for one more periodic housekeeping task.
	// outFile/errFile are the real *os.File only when this IS a genuine
	// os.Stdout/os.Stderr (production); a test's non-*os.File stdout/stderr
	// simply makes rotateDaemonStdio a no-op for those two arguments.
	outFile, _ := stdout.(*os.File)
	errFile, _ := stderr.(*os.File)
	if !managed {
		outLog.Info("scavenge: off (unmanaged daemon, §10.1)")
	} else if scavengeDisabled() {
		outLog.Info("scavenge: OFF (FATHOM_SCAVENGE=off) — the daemon's OS-visible footprint may not shrink after a peak (O108); fathom-http.log/.err rotation (DG-6) is paused too")
	} else {
		go runMemoryScavenger(context.Background(), scavengeIntervalFromEnv(), outLog, func() {
			scavengeTick(outFile, errFile, daemonStdioRotateBytes, outLog, flog, time.Now)
		}, nil)
	}

	// RT-33..RT-36, RT-155, RT-161, RT-172, RT-173: the governor samples
	// every 2 s and bounds memory whatever the cause. Each sample also
	// feeds store distress (the store volume's free space, RT-41) and the
	// breaker's safe mode, which steps down after 30 min at normal.
	sm := newSafeModeController(lcOut, lcIn.ledger, pid, Version, budget.Breaker.Warmup, func(m string) { flog.Warn(m) })
	minStoreFree := budget.Host.MinStoreVolumeFree
	_, stopGovernor := startGovernor(budget, budgetNotes, absStorePath(*storePath), lcIn.ledger, servingLadderActions(srv, c, fleet), flog, func(s *tenancy.Sample, l tenancy.Level) {
		tenancy.NoteStoreVolumeFree(s.StoreFree, minStoreFree, s.At)
		sm.step(s.At, l)
	})
	defer stopGovernor()
	publishDaemonEvent(consoleDeps.bus, prevRecord, hadPrev, commit)
	eventSources := newConsoleEventSources(consoleDeps.bus, c, verifyLedgerPath(), sessionLedgerPath())
	eventSources.warn = func(msg string) { flog.Warn(msg) }
	startFeeders := func() { go runConsoleEventSources(context.Background(), eventSources) }
	if managed {
		startFeeders()
	} else {
		handler = lazyFeeders(handler, startFeeders) // §10.1: feeders lazy
	}

	swap.set(handler)
	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		flog.Error(fmt.Sprintf("http serve: %v", err))
		exitRow("serve_error", err.Error(), 1)
		removeRecord()
		return 1
	}
	return 0
}

// daemonLifetimeTick is the heartbeat and exit-bound cadence (§10.1,
// §10.2); parentExitPoll the getppid poll behind the kqueue watch.
var (
	daemonLifetimeTick = 60 * time.Second
	parentExitPoll     = 5 * time.Second
)

// httpServerState holds lightweight server-level metrics exposed via GET /metrics.
type httpServerState struct {
	startTime time.Time
	reqCount  atomic.Int64 // incremented on every non-/metrics request
}

// firstNonEmpty returns configured if non-empty, else fallback — the small
// helper CF-7 uses to feed a config.toml [daemon] value into a flag's
// default while still falling back to the pre-config hardcoded default when
// no config file (or no such key) is present.
func firstNonEmpty(configured, fallback string) string {
	if configured != "" {
		return configured
	}
	return fallback
}

// daemonAutoRefreshDefault resolves the --auto-refresh flag's default from
// [daemon].auto_refresh: unset (nil, no layer ever set it) keeps the
// pre-config default of true; an explicit false or true in config.toml is
// honored as the new default, still overridable by the flag itself.
func daemonAutoRefreshDefault(d config.Daemon) bool {
	if d.AutoRefresh == nil {
		return true
	}
	return *d.AutoRefresh
}

// openCallLog opens (creating parent directories as needed) the JSONL
// tool-call log for appending.
func openCallLog(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

// newHTTPHandler returns an http.Handler for the serve-http subcommand.
// It is factored out so tests can drive it via httptest without binding a port.
//
// Routes:
//
//	POST /rpc           — single JSON-RPC 2.0 request/response (full surface)
//	POST /rpc/{surface} — same, scoped to a named tool surface (agent,
//	                      companion, coding, investigation, shared, platform);
//	                      the surface filters tools/list and, by default,
//	                      execute-and-flags off-surface tools/call (ADR-014
//	                      amendment) — hard gating is available behind
//	                      Server.strictSurfaces. agent is the primary
//	                      15-tool surface; companion is a compatible alias.
//	GET  /healthz       — liveness probe, returns "ok"
//	GET  /events        — live freshness (OC-13): SSE with IDs, a 25 s
//	                      keepalive, Last-Event-ID replay of the last 64
//	                      events, at most 16 subscribers; names and IDs only
//	GET  /metrics       — simple text exposition: uptime and request count
//	GET  /statusz       — daemon health, per-service freshness, channel
//	                      liveness, last compile per layer (Q-25/Q-26,
//	                      docs/design/observability-surfaces.md §8 item 6);
//	                      JSON via Accept: application/json or ?format=json,
//	                      HTML otherwise; gated inside the /rpc/modules
//	                      bearer-token allowlist when hosted (modulesAuthMiddleware)
//	GET  /findings/summary — repo-wide open finding-kind claim count + oldest
//	                      compiled-at (FL-21, docs/vision/30-findings-loop.md
//	                      §3.4); ?service=NAME required; never an MCP tool
//	POST /refresh-file  — {service, file} re-extracts one file's
//	                      observations/syntax edges and fingerprint,
//	                      leaving typed edges alone (SU-5/SU-6, ADR-041
//	                      decisions 3/4); the pre-edit hook's per-file
//	                      freshness check; never an MCP tool
//	POST /route         — {prompt} suggests which MCP tool's question-shape
//	                      fits a prompt (DP-9, ADR-042, docs/vision/
//	                      32-decision-port.md §2 item 3) — a SUGGESTION
//	                      about the prompt's shape, never a code fact;
//	                      provider "none" or any failure serves a
//	                      none-shaped 200, never a 5xx; the prompt hook's
//	                      route line (DP-10) is this route's only caller
//	                      today; never an MCP tool
//	POST /reload        — 200 OK stub; would reload a PlatformStore in production
//
// newHTTPHandlerWithAuth wraps newHTTPHandler with the /rpc/modules (AND,
// Q-26, /statusz) bearer-token gate (GKE-MH-7). modulesToken == "" (the
// default) disables the check entirely — identical behavior to
// newHTTPHandler(srv, c), so every existing call site/test unaffected by
// this row keeps working unchanged. ports is DP-9's decision port for
// POST /route; pass nil for the pre-decision-port none-shaped behavior
// (routeHandler treats a nil *decide.Ports the same as provider "none").
// log is DG-5's per-request access logger (loggingMiddleware, wrapped
// OUTSIDE the modules-auth gate so even a 401 gets an access-log line).
func newHTTPHandlerWithAuth(srv *mcp.Server, c *client.Client, modulesToken, selfAddr string, ports *decide.Ports, log *dlog.Logger) http.Handler {
	return newHTTPHandlerWithAuthDeps(srv, c, modulesToken, selfAddr, ports, log, nil)
}

// newHTTPHandlerWithAuthDeps is newHTTPHandlerWithAuth with the console's
// dependencies supplied by the caller — serve-http passes the deps whose
// event bus its feeders publish to (console_events.go); nil builds fresh
// ones, as every handler test does.
func newHTTPHandlerWithAuthDeps(srv *mcp.Server, c *client.Client, modulesToken, selfAddr string, ports *decide.Ports, log *dlog.Logger, deps *consoleDeps) http.Handler {
	return loggingMiddleware(modulesAuthMiddleware(newHTTPHandlerDeps(srv, c, selfAddr, ports, deps), modulesToken), log)
}

// fathomRequestIDHeader is the response header loggingMiddleware stamps on
// every request (DG-5) — a correlation token for reading fathom-http.log
// alongside a client's own logs, never a security credential.
const fathomRequestIDHeader = "X-Fathom-Request-Id"

// loggingRequestSeq is the monotonic counter backing loggingMiddleware's
// request ids — a plain incrementing number is sufficient correlation
// (this is a debug log line, not an auth token) and, unlike
// crypto/rand, needs no error handling on every single request.
var loggingRequestSeq atomic.Int64

// loggingResponseWriter wraps http.ResponseWriter to capture the status
// code a handler actually wrote, defaulting to 200 for a handler that
// never calls WriteHeader explicitly (net/http's own convention).
type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// Flush lets loggingResponseWriter satisfy http.Flusher by forwarding to
// the embedded ResponseWriter's own Flush, when it has one. Go does NOT
// promote a method through an embedded INTERFACE field the way it does
// through an embedded concrete type, so without this, wrapping any
// ResponseWriter in loggingResponseWriter silently strips its Flusher-ness
// — exactly the regression that broke GET /events (an SSE stream) once
// loggingMiddleware started wrapping every route (finding [0]/DG-5): the
// handler's `w.(http.Flusher)` type assertion started failing, so its
// Flush() call was skipped and streamed bytes sat buffered forever.
func (w *loggingResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// loggingMiddleware is DG-5: every route in newHTTPHandler's mux (/statusz,
// /findings/summary, /refresh-file, /route, /rpc*, /healthz, /events,
// /metrics, /reload, the observatory pages) previously had structured
// access logging ONLY for POST /rpc's tools/call (internal/mcp/calllog.go)
// — every other route just incremented a bare counter, with no method,
// path, status, duration or client ever recorded anywhere. This wraps the
// whole handler once, at debug level (the daemon's normal operating volume
// is far too high for info), and stamps every response with a short
// request id so a client's own logs can be correlated against
// fathom-http.log for the same request.
func loggingMiddleware(next http.Handler, log *dlog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := loggingRequestSeq.Add(1)
		reqID := fmt.Sprintf("%d", id)
		w.Header().Set(fathomRequestIDHeader, reqID)

		lw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(lw, r)
		dur := time.Since(start)

		log.Debug("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", lw.status,
			"dur_ms", dur.Milliseconds(),
			"req_id", reqID,
		)
	})
}

// modulesAuthMiddleware gates ONLY the /rpc/modules path (and its
// sub-paths) behind a bearer token — every other route (including bare
// /rpc and every other named surface) is untouched: the loopback-only
// daemon's existing unauthenticated trust boundary (docs/mcp-tools.md)
// stays exactly as it is. token == "" disables the check (default OFF —
// auth is a launch requirement for a NETWORK-reachable deployment, which
// does not exist until an operator opts in via --modules-token-env,
// docs/plans/gke-module-host.md's design section).
//
// Comparison is constant-time (crypto/subtle.ConstantTimeCompare) so a
// byte-by-byte mismatch never leaks timing information about how much of
// the token matched. The configured token is never logged or echoed back —
// only a boolean authorized/denied is observable from this middleware, and
// the 401 body never repeats the caller-supplied credential.
func modulesAuthMiddleware(next http.Handler, token string) http.Handler {
	if token == "" {
		return next
	}
	const prefix = "Bearer "
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Q-26 (docs/plans/expert-suite-q.md): /statusz sits INSIDE this
		// same allowlist gate, not outside it — the phase-O critic finding
		// #2 class (GKE-MH-7): a hosted, network-reachable deployment must
		// not leak index/service detail through a route the modules-token
		// gate doesn't cover. Loopback-only default (token == "" above)
		// leaves /statusz fully open, exactly like every other route.
		if r.URL.Path != "/rpc/modules" && !strings.HasPrefix(r.URL.Path, "/rpc/modules/") && r.URL.Path != "/statusz" {
			next.ServeHTTP(w, r)
			return
		}
		got := r.Header.Get("Authorization")
		authorized := strings.HasPrefix(got, prefix) &&
			subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(got, prefix)), []byte(token)) == 1
		if !authorized {
			w.Header().Set("WWW-Authenticate", `Bearer realm="fathom-modules"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func newHTTPHandler(srv *mcp.Server, c *client.Client, selfAddr string, ports *decide.Ports) http.Handler {
	return newHTTPHandlerDeps(srv, c, selfAddr, ports, nil)
}

func newHTTPHandlerDeps(srv *mcp.Server, c *client.Client, selfAddr string, ports *decide.Ports, deps *consoleDeps) http.Handler {
	state := &httpServerState{startTime: time.Now()}
	mux := http.NewServeMux()
	selfURL := "http://" + selfAddr
	if deps == nil {
		deps = newConsoleDeps(c, selfURL, state.startTime)
	}
	state.startTime = deps.startTime

	// GET /statusz (Q-25/Q-26): daemon health, per-service index
	// freshness, channel liveness, last compile per layer. Registered
	// unconditionally here — modulesAuthMiddleware (servehttp.go, wrapped
	// around this handler by newHTTPHandlerWithAuth) is what gates it
	// under hosted mode; this function itself has no notion of a token.
	// GET /findings/summary (FL-21, ADR-040 decision 5): a plain HTTP
	// route, never an MCP tool — the session hook's own repo-wide "open
	// findings: N" brief line.
	mux.Handle("/findings/summary", findingsSummaryHandler(c))
	// GET /indexed?service= (RT-80): the meta-only "is service X indexed"
	// probe verify, the hooks, plans and the console ask.
	mux.Handle("/indexed", indexedHandler(c))
	// POST /refresh-file {service, file} (SU-5/SU-6, ADR-041 decisions 3
	// and 4): the pre-edit hook's per-file freshness refresh.
	mux.Handle("/refresh-file", refreshFileHandler(c, srv, srv.RefreshQueue))
	// POST /refresh-request {service} (RT-111, §12.3): the post-commit
	// hook's refresh — queued (202, coalesced), refused at refuse and for
	// background work in safe mode, never an ingest in the request.
	mux.Handle("/refresh-request", refreshRequestHandler(c, srv.RefreshQueue))
	// POST /route {prompt} (DP-9, ADR-042): suggests which MCP tool's
	// question-shape fits prompt — the prompt hook's route line (DP-10).
	mux.Handle("/route", routeHandler(ports))
	// The Observatory console (vision 35, ADR-045): every registered page
	// (console_registry.go) on the console shell — /, /fleet,
	// /projects/{svc}, /impact, /sessions, /sessions/{id}, /brief, /wiring,
	// /statusz (GET /statusz is Q-25/Q-26's report; the modules gate above
	// still covers it in hosted mode), /search — plus GET /events, the
	// shell's 404, and the legacy /observatory* routes (308 for HTML, the
	// unchanged v1 JSON for ?format=json). The wiring mutations stay at
	// /observatory/wiring/{preview,apply,undo} behind requireLocalMutation
	// (SH-7/SH-8, law 13; observatory_apply.go).
	mux.Handle("/assets/app.css", assetsHandler())
	mountConsole(mux, deps, wiringRoutes(clientWiringRegistry(c), selfURL, time.Now))

	// GET /debug/pprof/* (O108, docs/guides/fleet-index.md daemon ops,
	// docs/mcp-tools.md HTTP surfaces): the stdlib pprof handlers, mounted
	// on this handler's OWN mux rather than relying on net/http/pprof's
	// init()-time registration onto http.DefaultServeMux (which this
	// server never serves from). Loopback-only by the same trust boundary
	// as every other route here (ADR-014 amendment: surfaces are not a
	// security boundary) — no separate gate, same as /statusz outside
	// hosted mode. /debug/pprof/heap and /debug/pprof/goroutine are the
	// two profiles O108's synthetic driver reads across an auto-refresh
	// loop; /debug/pprof/profile (30s CPU) and /debug/pprof/allocs are
	// mounted too since they cost nothing idle.
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	// serveRPC handles one JSON-RPC 2.0 message on the named surface.
	serveRPC := func(w http.ResponseWriter, r *http.Request, surface string) {
		// RT-179: the call's clock and class deadline run from here.
		arrived := time.Now()
		state.reqCount.Add(1)
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		const maxBody = 4 * 1024 * 1024 // 4 MB
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		raw := make([]byte, 0, 512)
		buf := make([]byte, 4096)
		for {
			n, err := r.Body.Read(buf)
			if n > 0 {
				raw = append(raw, buf[:n]...)
			}
			if err != nil {
				break
			}
		}

		// RT-58 (design §3.5): the request's own context, so a client that
		// leaves cancels read work (counted abandoned); ingest-class calls
		// run detached in dispatch, so a disconnect never cancels an ingest
		// midway. X-Fathom-Deadline bounds it; the admission ticket adds
		// the class deadline. "?contracts=true" (O101/TD-2) opts this
		// request's tools/list back into outputSchema.
		ctx, cancelRPC := rpcContext(r, surface)
		defer cancelRPC()
		ctx = mcp.WithArrival(ctx, arrived)
		// An ingest-class call runs detached with no class deadline; the
		// route's 150 s write deadline would cut its client off mid-ingest
		// (the work completes, the answer is lost), so it lifts it.
		ctx = mcp.WithDetachHook(ctx, func() { _ = http.NewResponseController(w).SetWriteDeadline(time.Time{}) })
		ctx, doneSources := rpcSourcesScope(ctx, r)
		defer doneSources()
		// RT-39/RT-40/RT-41/RT-44: safe mode, warm-up, store distress and
		// a drain refuse before dispatch (503 + Retry-After + -32050).
		releaseGate, admitted := lifecycleGate(ctx, w, raw, r.Header.Get("X-Fathom-Origin"))
		if !admitted {
			return
		}
		defer releaseGate()
		// RT-56 (§3.4): a top-level refusal answers 503 + Retry-After; the
		// admission ticket registers the call in flight by class (RT-172).
		ctx, refusal := mcp.WithRefusalSink(ctx)
		// MCP-Protocol-Version (2025-06-18+ Streamable HTTP): clients send
		// the negotiated revision on every post-initialize request. It is
		// TOLERATED here (RP-12) — never a 400 — and answered with the
		// revision this server would negotiate for it, so a client can see
		// the server's choice on every response, not only on initialize.
		w.Header().Set("MCP-Protocol-Version", mcp.NegotiateProtocolVersion(r.Header.Get("MCP-Protocol-Version")))
		resp, hasResp := srv.HandleRequest(ctx, raw)
		if !hasResp {
			// Notification — no response body.
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if ref := refusal(); ref != nil {
			writeRefusal(w, ref)
			w.Write(resp) //nolint:errcheck
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(resp) //nolint:errcheck
	}

	// POST /rpc — JSON-RPC 2.0 single-message endpoint, full surface.
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		serveRPC(w, r, "")
	})

	// POST /rpc/{surface} — the same endpoint scoped to a named surface.
	mux.HandleFunc("/rpc/{surface}", func(w http.ResponseWriter, r *http.Request) {
		surface := r.PathValue("surface")
		if !mcp.ValidSurface(surface) {
			state.reqCount.Add(1)
			http.Error(w, fmt.Sprintf("unknown surface %q; valid: %s (POST /rpc serves the full surface)",
				surface, strings.Join(mcp.SurfaceNames(), ", ")), http.StatusNotFound)
			return
		}
		serveRPC(w, r, surface)
	})

	// Anything else under /rpc/ (trailing slash, extra segments) gets the same
	// diagnostic instead of the mux's bare 404.
	mux.HandleFunc("/rpc/", func(w http.ResponseWriter, r *http.Request) {
		state.reqCount.Add(1)
		http.Error(w, fmt.Sprintf("unknown RPC path %q; valid: POST /rpc (full surface) or POST /rpc/{%s}",
			r.URL.Path, strings.Join(mcp.SurfaceNames(), "|")), http.StatusNotFound)
	})

	// GET /healthz — liveness probe.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		state.reqCount.Add(1)
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	// GET /metrics — simple text exposition of server-level metrics.
	// Format: one "key value\n" line per metric.
	//
	// O108 (.fathom/observations.md): the always-on daemon's memory grew
	// ~0.9 GB/hour with nothing on this endpoint to attribute it from —
	// no memory series, no goroutine count, no pprof. The go_memstats_*/
	// go_goroutines/process_uptime_seconds lines below are the Prometheus-
	// conventional names (github.com/prometheus/client_golang's own
	// process/go collectors use the same three memstats names and
	// go_goroutines) so an operator's existing dashboards/alerts pick them
	// up without translation; the fathom_* gauges are the per-subsystem
	// breakdown O108 asked for — cheap enough to compute on every scrape
	// (an atomic load or a short mutex-guarded scan each, never a
	// recomputation of the thing being measured).
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		uptime := time.Since(state.startTime).Seconds()
		reqCount := state.reqCount.Load()

		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)

		graphEntries, graphHits, graphMisses := srv.GraphCacheStats()

		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "fathom_uptime_seconds %.3f\n", uptime)
		fmt.Fprintf(w, "fathom_requests_total %d\n", reqCount)
		fmt.Fprintf(w, "process_uptime_seconds %.3f\n", uptime)

		fmt.Fprintf(w, "go_memstats_heap_inuse_bytes %d\n", ms.HeapInuse)
		fmt.Fprintf(w, "go_memstats_heap_sys_bytes %d\n", ms.HeapSys)
		fmt.Fprintf(w, "go_memstats_sys_bytes %d\n", ms.Sys)
		fmt.Fprintf(w, "go_goroutines %d\n", runtime.NumGoroutine())

		// process_resident_memory_bytes / process_phys_footprint_bytes /
		// process_max_resident_memory_bytes (DG-16, owner amendment): the
		// go_memstats_* lines above are ALL Go-runtime-level — none of them
		// is OS-visible resident memory, which is exactly the "heap_sys
		// never decreases, RSS freely oscillates" divergence O108's finding
		// was about. getrusage's ru_maxrss is a PEAK that never decreases,
		// so it is named _max_ here rather than exposed as "current RSS" (a
		// prior draft of this fix would have done exactly that). Omitted
		// entirely — never a fabricated zero — on a platform/build with no
		// proven reading (rss_other.go).
		if v, ok := processResidentMemoryBytes(); ok {
			fmt.Fprintf(w, "process_resident_memory_bytes %d\n", v)
		}
		if v, ok := processPhysFootprintBytes(); ok {
			fmt.Fprintf(w, "process_phys_footprint_bytes %d\n", v)
		}
		if v, ok := processMaxResidentMemoryBytes(); ok {
			fmt.Fprintf(w, "process_max_resident_memory_bytes %d\n", v)
		}

		// go_memory_limit_bytes (W-N, docs/plans/daemon-footprint.md): the
		// soft limit runtime/debug.SetMemoryLimit is currently enforcing —
		// read back live (never a cached copy of what serve-http applied
		// at startup) via the negative-input query contract
		// (memorylimit.go's currentMemoryLimitBytes). A value at
		// math.MaxInt64's scale means no limit is set ("off").
		fmt.Fprint(w, memoryLimitMetricLine())
		fmt.Fprint(w, governorMetricLines())     // RT-161, RT-155: the ladder, footprint, children, quarantine
		fmt.Fprint(w, admissionMetricLines(srv)) // RT-53..RT-63: slots, queues, refusals, would-refuse, outcomes, test callers

		// fathom_graph_memo_entries: the watermark-keyed call-graph memo
		// (BP-28, I21, internal/analysis/callgraph/cache.go) — one entry
		// per (service, watermark) currently held, capped at 8 by default.
		// hits/misses are process-lifetime counters, not gauges, but are
		// cheap enough to always include so a flat entries count next to a
		// climbing misses count reads as "evicting correctly", not "empty".
		fmt.Fprintf(w, "fathom_graph_memo_entries %d\n", graphEntries)
		fmt.Fprintf(w, "fathom_graph_memo_hits_total %d\n", graphHits)
		fmt.Fprintf(w, "fathom_graph_memo_misses_total %d\n", graphMisses)

		// fathom_open_stores: SQLiteStore handles currently open in this
		// process (internal/store's openStoreCount) — a daemon that opens
		// one store at startup and never re-opens one should hold this
		// constant for its whole lifetime.
		fmt.Fprintf(w, "fathom_open_stores %d\n", store.OpenStores())

		// fathom_autorefresh_total: auto-refresh re-ingests triggered by
		// git-HEAD drift (internal/mcp/autorefresh.go) since this process
		// started. 0 when --auto-refresh=false.
		fmt.Fprintf(w, "fathom_autorefresh_total %d\n", srv.AutoRefreshTotal())

		// fathom_typed_pass_total: go/packages typed-edge passes actually
		// run (internal/fusion/typedcalls.go) since this process started —
		// every ingest of a Go service, whether triggered by an explicit
		// `ingest` call or an auto-refresh.
		fmt.Fprintf(w, "fathom_typed_pass_total %d\n", c.TypedPassTotal())

		// fathom_typed_pass_inflight / fathom_typed_pass_waited_total (W-N,
		// docs/plans/daemon-footprint.md, O108 reframed): the process-wide
		// typed-pass semaphore that bounds the memory PEAK a typed-edge
		// pass over a large service can leave as idle heap — inflight is 0
		// or 1; waited_total climbs only when a pass actually had to queue
		// behind another one, e.g. two overlapping `ingest` calls.
		fmt.Fprintf(w, "fathom_typed_pass_inflight %d\n", c.TypedPassInflight())
		fmt.Fprintf(w, "fathom_typed_pass_waited_total %d\n", c.TypedPassWaitedTotal())

		// fathom_console_page_views_total{route} (OC-15, UX-M4): HTML views
		// of each console page since this process started, keyed by the
		// route pattern — the durable 30-day record is console-views.jsonl.
		// fathom_console_event_subscribers: live GET /events streams (OC-13).
		deps.views.writeMetrics(w)
		fmt.Fprintf(w, "fathom_console_event_subscribers %d\n", deps.bus.Subscribers())
	})

	// POST /reload — 200 OK stub; in production this would call
	// PlatformStore.Reload() to re-scan the store directory.
	mux.HandleFunc("/reload", func(w http.ResponseWriter, r *http.Request) {
		state.reqCount.Add(1)
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	return mux
}
