package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/memo"
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

// serveRun is runServeHTTP's state: the locals the phases share, each phase
// a method in servehttp_<phase>.go. Cleanups registered with deferred run
// LIFO in the single defer runServeHTTP installs, which is exactly the
// order the inline defers ran in before the split (a defer inside a phase
// method would run when the phase returned).
type serveRun struct {
	args           []string
	stdout, stderr io.Writer

	// config and flags
	flog, outLog       *dlog.Logger
	startPPID          int
	startedAt          time.Time
	loaded             config.Loaded
	storePath, addr    string
	callLogPath        string
	autoRefresh        bool
	modulePaths        stringSliceFlag
	managedFlag        bool
	exitWithParent     bool
	allowOperatorState bool
	idleExit, ttl      time.Duration
	lifecycleIDFlag    string
	managed            bool
	managedLabel       string
	supervised         *bool // managed only: launchd/systemd runs it (O1363)
	budget             tenancy.Budget
	budgetNotes        []tenancy.Note

	// lifecycle
	lcID             string
	pid              int
	commit           string
	lcIn             lifecycleStart
	lcOut            lifecycleOutcome
	moduleHost       *modulehost.Host
	listener         net.Listener
	prevRecord       console.DaemonRecord
	hadPrev          bool
	recordPath       string
	waitState        atomic.Pointer[storeWaitState]
	swap             *swapHandler
	act              *activity
	httpSrv          *http.Server
	serveErr         chan error
	perProcessLedger bool
	lifetimeP        atomic.Pointer[daemonLifetime]
	refreshCtx       context.Context
	stopRefresh      context.CancelFunc
	lifetime         *daemonLifetime

	// store and tenancy setup
	c                 *client.Client
	daemonProfileName string
	ports             *decide.Ports
	refreshQ          *refresh.Queue
	backend           mcp.QueryInterface
	logger            *mcp.CallLogger
	srv               *mcp.Server
	fleet             *fleetAdapter
	gate              *tenancy.Gate

	// routes
	registry    *lastGoodRegistry
	consoleDeps *consoleDeps
	handler     http.Handler

	cleanups []func()
}

// deferred registers a cleanup to run when runServeHTTP returns, LIFO.
func (r *serveRun) deferred(f func()) { r.cleanups = append(r.cleanups, f) }

// runCleanups runs the registered cleanups newest first, each as a real
// defer so a panicking cleanup still lets the older ones run.
func (r *serveRun) runCleanups() {
	if len(r.cleanups) == 0 {
		return
	}
	n := len(r.cleanups) - 1
	f := r.cleanups[n]
	r.cleanups = r.cleanups[:n]
	defer r.runCleanups()
	f()
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
	r := &serveRun{args: args, stdout: stdout, stderr: stderr}
	defer r.runCleanups()
	if code, done := r.configure(); done {
		return code
	}
	if code, done := r.startLedger(); done {
		return code
	}
	if code, done := r.mountModules(); done {
		return code
	}
	r.limitMemory()
	if code, done := r.bind(); done {
		return code
	}
	r.serveWaiting()
	r.handleSignals()
	r.startLifetime()
	if code, done := r.openStore(); done {
		return code
	}
	r.wireLLM()
	r.wirePorts()
	r.buildBackend()
	r.openCallLog()
	r.buildServer()
	r.buildRoutes()
	r.startLoops()
	return r.serveUntilDone()
}

// openCallLog opens the rotating call log and builds the MCP call logger.
func (r *serveRun) openCallLog() {
	// Call logging feeds tool-selection telemetry (which tools each surface
	// actually uses); failure to open the log never blocks serving.
	if r.callLogPath != "" {
		// RT-126 (§12.1): rotated at 64 MiB; the previous generation is
		// rolled up and archived, never deleted.
		f, err := openRotatingCallLog(r.callLogPath, callLogRotateBytes, func(m string) { r.flog.Warn(m) })
		if err != nil {
			r.flog.Warn(fmt.Sprintf("call log disabled: %v", err))
		} else {
			r.deferred(func() { f.Close() })
			r.logger = mcp.NewCallLogger(f)
		}
	}
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
//	                      extended, companion; mcp.SurfaceNames); the
//	                      surface filters tools/list and, by default,
//	                      execute-and-flags off-surface tools/call (ADR-014
//	                      amendment) — hard gating is available behind
//	                      Server.strictSurfaces. agent is the eight-tool
//	                      default; extended the opt-in superset, companion
//	                      its compatible alias. Any other name, the retired
//	                      group surfaces included, answers 404.
//	GET  /healthz       — liveness probe, returns "ok"
//	GET  /events        — live freshness (OC-13): SSE with IDs, a 25 s
//	                      keepalive, Last-Event-ID replay of the last 64
//	                      events, at most 16 subscribers; names and IDs only
//	GET  /metrics       — simple text exposition: uptime and request count
//	GET  /statusz       — daemon health, per-service freshness, channel
//	                      liveness, last compile per layer (Q-25/Q-26,
//	                      docs/design/observability-surfaces.md §8 item 6);
//	                      JSON via Accept: application/json or ?format=json,
//	                      HTML otherwise
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
// newHTTPHandlerLogged wraps newHTTPHandler in DG-5's per-request access
// logger (loggingMiddleware). ports is DP-9's decision port for POST /route;
// pass nil for the none-shaped behavior (routeHandler treats a nil
// *decide.Ports the same as provider "none"). The /rpc/modules bearer-token
// gate that once sat inside it (GKE-MH-7, Q-26) went with hosted-modules
// mode (SR-17, surface reckoning F-68): every route is loopback-only
// (newDaemonServer's Host guard), /statusz included.
func newHTTPHandlerLogged(srv *mcp.Server, c *client.Client, selfAddr string, ports *decide.Ports, log *dlog.Logger) http.Handler {
	return newHTTPHandlerLoggedDeps(srv, c, selfAddr, ports, log, nil)
}

// newHTTPHandlerLoggedDeps is newHTTPHandlerLogged with the console's
// dependencies supplied by the caller — serve-http passes the deps whose
// event bus its feeders publish to (console_events.go); nil builds fresh
// ones, as every handler test does.
func newHTTPHandlerLoggedDeps(srv *mcp.Server, c *client.Client, selfAddr string, ports *decide.Ports, log *dlog.Logger, deps *consoleDeps) http.Handler {
	return loggingMiddleware(newHTTPHandlerDeps(srv, c, selfAddr, ports, deps), log)
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

// Unwrap lets http.ResponseController reach the connection through this
// wrapper (EW-36, the O128 recurrence): without it, the ingest-class detach
// hook's SetWriteDeadline(zero) and every other ResponseController call
// returned ErrNotSupported, silently, and the route's 150 s write deadline
// cut a long compile_layer/ingest off with an empty answer.
func (w *loggingResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

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
	// freshness, channel liveness, last compile per layer.
	// GET /findings/summary (FL-21, ADR-040 decision 5): a plain HTTP
	// route, never an MCP tool — the session hook's own repo-wide "open
	// findings: N" brief line.
	mux.Handle("/findings/summary", findingsSummaryHandler(c))
	// GET /indexed?service= (RT-80): the meta-only "is service X indexed"
	// probe verify, the hooks, plans and the console ask.
	mux.Handle("/indexed", indexedHandler(c))
	// GET /identity?path=<dir>[&path=<toplevel>] (RT-133): which indexed
	// service a directory is, from the in-process registry.
	mux.Handle("/identity", handleIdentity(daemonIdentityServices))
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
	// /statusz (GET /statusz is Q-25/Q-26's report), /search — plus GET /events, the
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
	// RT-123 (§13.5): guarded — loopback Host, seconds ≤ 30, one profile
	// at a time — with GET /debug/inflight beside it (pprof_guard.go).
	mountPprof(mux)

	// serveRPC handles one JSON-RPC 2.0 message on the named surface.
	serveRPC := func(w http.ResponseWriter, r *http.Request, surface string) {
		// RT-179: the call's clock and class deadline run from here.
		arrived := time.Now()
		state.reqCount.Add(1)
		rpcActivity.Add(1) // RT-128: the WAL checkpoint's idle signal
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
		ctx = mcp.WithDetachHook(ctx, func() { _ = liftRPCWriteDeadline(w) })
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
		// HeapSys is virtual size, not memory held (§13.3): labelled so,
		// never read as the footprint (fathom_footprint_bytes is).
		fmt.Fprint(w, "# HELP go_memstats_heap_sys_bytes virtual address space reserved for the heap (never decreases; not resident memory — see fathom_footprint_bytes)\n")
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
		// RT-124 (§13.3): every tenancy family — the ladder, footprint,
		// children and quarantine (RT-161, RT-155), admission's slots,
		// queues, refusals and outcomes (RT-53..RT-63), and volumes,
		// released memory, oldest in-flight, abandoned and bytes read by
		// tool, cache bytes, starts by reason and SQLite's memory.
		writeTenancyMetrics(w, srv)
		// fathom_cache_events_total{group,event}: the counters of every
		// memo group, abandoned, refuse-cancelled and declined apart.
		writeCacheEvents(w, memo.Stats())

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
