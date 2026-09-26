package main

import (
	"context"
	"crypto/subtle"
	"fmt"
	"io"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"github.com/geoffmilleraz/fathom/internal/capability"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/modulehost"
	"github.com/geoffmilleraz/fathom/internal/store"
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
	// DG-4 (diagnostics sweep): one logger per stream, constructed FIRST so
	// every diagnostic below — including the ones that can fire before a
	// store or client exists — is timestamped and leveled instead of a bare
	// fmt.Fprintf. Two loggers, not one, because launchd pipes stdout and
	// stderr to SEPARATE files (daemon.go's plistBody, DG-6) — collapsing
	// them onto one writer would move today's stdout-only lines into
	// fathom-http.err and vice versa.
	flog := dlog.New(stderr, dlog.LevelFromEnv(os.Getenv), time.Now)
	outLog := dlog.New(stdout, dlog.LevelFromEnv(os.Getenv), time.Now)

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
	modulesTokenEnv := fs.String("modules-token-env", "", "name of an environment variable holding the bearer token required for POST /rpc/modules (GKE-MH-7); unset = auth disabled (default — matches the loopback-only daemon's existing unauthenticated trust boundary). The token itself is never a flag or config value, only this env var's NAME.")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *storePath == "" {
		flog.Error("--store is required")
		fs.Usage()
		return 1
	}

	// GKE-MH-7: resolve the modules bearer token BEFORE opening anything —
	// a --modules-token-env naming an empty/unset environment variable is a
	// misconfiguration that must fail loudly, never silently leave auth
	// disabled on what the operator believes is a protected path.
	modulesToken := ""
	if *modulesTokenEnv != "" {
		modulesToken = os.Getenv(*modulesTokenEnv)
		if modulesToken == "" {
			flog.Error(fmt.Sprintf("--modules-token-env=%s is set but that environment variable is empty or unset", *modulesTokenEnv))
			return 1
		}
	}

	c, err := client.NewConstantClient(client.ModeLocal, *storePath)
	if err != nil {
		flog.Error(fmt.Sprintf("opening client: %v", err))
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
	// to decision.None rather than blocking startup (I14).
	ports := openDecisionPorts(loaded, daemonProfileName, *callLogPath, "serve-http", stderr)
	c.WithDecision(ports.Decider, ports.Cfg, ports.Recorder)

	// Default: keep the shared index fresh transparently. A query for a service
	// whose git HEAD has drifted from the indexed commit triggers a hash-gated
	// re-ingest before the answer — so the always-on daemon never serves a stale
	// index without anyone noticing.
	var backend mcp.QueryInterface = c
	if *autoRefresh {
		backend = mcp.NewAutoRefresh(c, flog)
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

	// GKE-MH-1: mount every --module store read-only, conformance-checked
	// against LevelL0 (the module host's floor — sufficient for
	// internal/semantic.ObligationsFor and the staleness-report shape; see
	// store.ConformanceLevel's doc for why Server conformance itself has no
	// separate schema footprint to check). A refusal aborts startup loudly
	// — never a partial mount, never a silently-degraded module.
	var moduleHost *modulehost.Host
	if len(modulePaths) > 0 {
		moduleHost = modulehost.NewHost(store.LevelL0)
		for _, p := range modulePaths {
			if _, mErr := moduleHost.Mount(p, ""); mErr != nil {
				flog.Error(fmt.Sprintf("mounting module %s: %v", p, mErr))
				return 1
			}
			flog.Info(fmt.Sprintf("serve-http: mounted module %s (%s)", p, modulehost.ModuleNameFromPath(p)))
		}
	}

	var srv *mcp.Server
	if moduleHost != nil {
		srv = mcp.NewServerWithModules(backend, &fleetAdapter{c: c, log: flog}, logger, moduleHost)
	} else {
		srv = mcp.NewServerWithFleetAndLogger(backend, &fleetAdapter{c: c, log: flog}, logger)
	}
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
	// path (bare /rpc, another surface, a batch sub-call). Set BEFORE
	// ListenAndServe — never toggled while serving.
	if modulesToken != "" {
		srv.SetHostedModules(true)
	}
	// The console's dependencies (vision 35): serve-http owns them so the
	// event feeders below publish to the same bus GET /events streams.
	consoleDeps := newConsoleDeps(c, "http://"+*addr, time.Now())
	if abs, absErr := filepath.Abs(*storePath); absErr == nil {
		consoleDeps.storePath = abs
	}
	handler := newHTTPHandlerWithAuthDeps(srv, c, modulesToken, *addr, ports, flog, consoleDeps)

	// Adopt a new binary without a human (selfrestart.go). launchd's
	// KeepAlive is true for this job, so exiting IS a restart — which is
	// what lets an agent complete a deploy end to end, rather than
	// building a fix and then waiting for someone to run `launchctl`.
	if selfRestartDisabled() {
		outLog.Info("self-restart: OFF (FATHOM_SELF_RESTART=off) — a deploy will need a manual restart")
	} else if exe, exeErr := os.Executable(); exeErr != nil {
		outLog.Warn(fmt.Sprintf("self-restart: disabled — cannot resolve this process's own path (%v)", exeErr))
	} else {
		go watchOwnBinary(exe, outLog, selfRestartPoll, selfRestartCooldown, os.Exit)
	}

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
	if scavengeDisabled() {
		outLog.Info("scavenge: OFF (FATHOM_SCAVENGE=off) — the daemon's OS-visible footprint may not shrink after a peak (O108); fathom-http.log/.err rotation (DG-6) is paused too")
	} else {
		go runMemoryScavenger(context.Background(), scavengeIntervalFromEnv(), outLog, func() {
			scavengeTick(outFile, errFile, daemonStdioRotateBytes, outLog, flog, time.Now)
		})
	}

	// Soft memory limit (memorylimit.go, W-N, docs/plans/daemon-footprint.md):
	// bounds how hard the runtime lets the heap grow toward a ceiling in
	// the first place, complementing the scavenger above (which bounds how
	// long a peak's OS-visible footprint survives AFTER the fact).
	if memLimitErr := applyMemoryLimit(loaded.Config.Daemon.MemoryLimit, stdout, debug.SetMemoryLimit); memLimitErr != nil {
		flog.Error(memLimitErr.Error())
		return 1
	}

	// DG-19 (owner amendment c): a self-restart or `task deploy` exits the
	// OLD daemon so launchd starts the new one — if the old process is
	// still tearing down when the new one races up, the bind can lose to
	// EADDRINUSE even though nothing is permanently wrong. Retry the BIND
	// ONLY, with backoff, for up to bindRetryMaxWait before giving up with
	// the same error ListenAndServe would have returned immediately.
	listener, listenErr := listenTCPWithRetry(*addr, flog)
	if listenErr != nil {
		flog.Error(fmt.Sprintf("http serve: %v", listenErr))
		return 1
	}
	outLog.Info(fmt.Sprintf("serve-http: listening on %s", *addr))

	// OC-2: record this daemon (its absolute --call-log, store, pid,
	// commit) only now that the listener is bound, so a verb run from any
	// directory reads the log this daemon writes and a serve-http that
	// failed to start never leaves a record behind. OC-13: announce the
	// daemon and start the event feeders.
	prevRecord, hadPrev := readDaemonRecordFor("http://" + *addr)
	commit := statuszBuildInfo().BinaryCommit
	if rec, recErr := recordServingDaemon(*addr, *callLogPath, *storePath, commit, time.Now()); recErr != nil {
		flog.Warn(fmt.Sprintf("serve-http: daemon record not written (%v) — verbs fall back to config and the legacy call-log path", recErr))
	} else {
		outLog.Info(fmt.Sprintf("serve-http: daemon record: call log %s", rec.CallLog))
	}
	publishDaemonEvent(consoleDeps.bus, prevRecord, hadPrev, commit)
	eventSources := newConsoleEventSources(consoleDeps.bus, c, verifyLedgerPath(), sessionLedgerPath())
	eventSources.warn = func(msg string) { flog.Warn(msg) }
	go runConsoleEventSources(context.Background(), eventSources)

	if err := http.Serve(listener, handler); err != nil {
		flog.Error(fmt.Sprintf("http serve: %v", err))
		return 1
	}
	return 0
}

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
	// POST /refresh-file {service, file} (SU-5/SU-6, ADR-041 decisions 3
	// and 4): the pre-edit hook's per-file freshness refresh.
	mux.Handle("/refresh-file", refreshFileHandler(c, srv))
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

		// Background, not r.Context(): a client disconnect must not cancel a
		// long-running ingest mid-flight. The scope carries the surface.
		// "?contracts=true" (O101/TD-2) opts this request's tools/list back
		// into outputSchema — off by default so the served payload stays
		// diet-sized; a client that validates structured output can ask.
		ctx := mcp.WithContracts(mcp.WithOrigin(mcp.WithRequestScope(context.Background(), surface, true), r.Header.Get("X-Fathom-Origin")), r.URL.Query().Get("contracts") == "true")
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
