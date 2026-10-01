package main

import (
	"context"
	"fmt"
	"os"

	"time"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/refresh"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// openStore opens the store, waiting — never exiting — while it is
// unavailable.
func (r *serveRun) openStore() (int, bool) {
	flog, outLog := r.flog, r.outLog
	storePath := r.storePath
	// RT-42 (§8.4 step 4): open the store, waiting — never exiting — while
	// it is unavailable: /healthz and /statusz answer with the errno,
	// tools/call is 503, reopen with backoff 5 s → 5 min, one store_wait
	// row (not counted) per distinct errno, a TCC wedge labelled apart from
	// an unmounted volume with the daemon binary's own remedy (RT-165).
	exePath, _ := os.Executable()
	lastWaitErrno := ""
	//fathom:tenancy-allow bg-ctx daemon lifetime context in serve-http startup, cancelled by the drain; not a request
	storeCtx, cancelStoreWait := context.WithCancel(context.Background())
	r.deferred(cancelStoreWait)
	c, err := openStoreWithWait(storeCtx, storeWaitDeps[*client.Client]{
		path:  absStorePath(storePath),
		open:  func() (*client.Client, error) { return client.NewConstantClient(client.ModeLocal, storePath) },
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
				r.waitState.Store(nil)
				outLog.Info(fmt.Sprintf("serve-http: store opened after %d attempt(s) waiting (%s, %s)", s.Attempts, s.Errno, s.Label))
				return
			}
			cp := s
			r.waitState.Store(&cp)
			flog.Warn(fmt.Sprintf("serve-http: waiting-for-store: %s (%s, %s) — %s; retry in %s", s.Err, s.Errno, s.Label, s.Remedy, s.NextRetry))
			if s.Errno != lastWaitErrno && r.lcIn.ledger != "" {
				lastWaitErrno = s.Errno
				row := tenancy.NewLifecycleRow(tenancy.RowStoreWait, time.Now(), r.pid, Version)
				row.Errno, row.Label, row.Detail = s.Errno, s.Label, s.Err+" — "+s.Remedy
				if werr := appendLifecycle(r.lcIn.ledger, row); werr != nil {
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
		r.stopServing()
		r.exitRow(tenancy.ExitStartFailed, "store: "+err.Error(), 1)
		if r.perProcessLedger {
			removePerProcessLedger(r.lcIn.ledger)
		}
		r.removeRecord()
		return 1, true
	}
	r.c = c
	r.deferred(func() { c.Close() })
	c.WithLogger(flog) // DG-4: the typed-edge pass's own diagnostics gain a timestamp/level too.
	return 0, false
}

// wireLLM wires the compile_layer LLM when a provider is configured.
func (r *serveRun) wireLLM() {
	flog, loaded, c := r.flog, r.loaded, r.c
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
	r.daemonProfileName = daemonProfileName
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
}

// wirePorts opens the decision ports and hands them to the client.
func (r *serveRun) wirePorts() {
	c := r.c
	// ADR-042/DP-9: the decision port — the compile pre-filter and the
	// obligations re-rank both read it through the Client. Fail-open by
	// construction (openDecisionPorts never errors outward): a
	// misconfigured or unreachable [decide] provider warns and falls back
	// to decision.None rather than blocking startup (I14). RT-39: closed in
	// safe mode and at refuse and above (gatedDecider).
	ports := openDecisionPorts(r.loaded, r.daemonProfileName, r.callLogPath, "serve-http", r.stderr)
	// RT-39: closed in safe mode and at refuse and above; RT-65: the
	// client's decision-port calls (the obligations re-rank, the compile
	// pre-filter) are also admitted as egress once the gate exists. /route
	// admits its own call, so ports.Decider only closes. JF-6: each gate
	// sits on the provider inside the port's fallback to decision.None
	// (Ports.Gated), so a closure degrades like a provider failure: one
	// line, the deterministic path.
	ports.Decider = ports.Gated(closingDecider)
	c.WithDecision(ports.Gated(func(d decision.Decider) decision.Decider { return gateDecider(d, "client") }), ports.Cfg, ports.Recorder)
	r.ports = ports
}

// buildBackend installs the toolchain shim, starts the refresh queue and
// picks the query backend.
func (r *serveRun) buildBackend() {
	flog, c := r.flog, r.c
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
	// RT-106: <tmp>/fathom-toolchain-<pid>; a killed predecessor's is
	// scavenged here.
	if undo, ierr := fusion.InstallProcessToolchain(""); ierr != nil {
		flog.Warn(fmt.Sprintf("serve-http: toolchain children run at normal priority: %v", ierr))
	} else {
		r.deferred(undo)
	}
	r.refreshQ = mcp.NewRefreshQueueLogged(c, refresh.Config{Started: r.startedAt}, flog)
	r.refreshQ.Start(r.refreshCtx)
	var backend mcp.QueryInterface = c
	if r.autoRefresh {
		backend = mcp.NewAutoRefreshQueued(c, flog, r.refreshQ)
		// RT-109: an in_progress row is an interrupted ingest only once
		// its owner is gone (a live CLI ingest is never duplicated).
		mcp.SetIngestOwnerProbe(ingestOwnerProbe(realDaemonOps{}))
	}
	r.backend = backend
}

// buildServer builds the MCP server over the backend, call log, fleet and
// module host, and the admission gate.
func (r *serveRun) buildServer() {
	flog, outLog, c, budget := r.flog, r.outLog, r.c, r.budget
	var srv *mcp.Server
	fleet := newFleetAdapter(c, flog)
	if r.moduleHost != nil {
		srv = mcp.NewServerWithModules(r.backend, fleet, r.logger, r.moduleHost)
	} else {
		srv = mcp.NewServerWithFleetAndLogger(r.backend, fleet, r.logger)
	}
	r.srv, r.fleet = srv, fleet
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
	r.gate = gate
	srv.SetGate(gate)
	srv.SetRefreshQueue(r.refreshQ)
	daemonGate.Store(gate)
	r.deferred(func() { daemonGate.Store(nil) })
	decisionAdmit.Store(&decisionAdmitter{admit: srv.AdmitDecision})
	r.deferred(func() { decisionAdmit.Store(nil) })
	if gate.Mode() == tenancy.ModeOff {
		flog.Warn("serve-http: ADMISSION OFF (FATHOM_TENANCY_ADMISSION=off or [tenancy].admission = \"off\") — no call is decided or refused by class (class deadlines still apply); doctor's admission_mode row warns on every run")
	} else {
		outLog.Info(fmt.Sprintf("serve-http: admission %s (slots %s)", gate.Mode(), admissionSlotsLine(budget)))
	}
}
