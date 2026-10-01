package main

// admission_http.go — the daemon's HTTP side of admission (design §3.3–
// §3.5, §10.4; RT-58, RT-59, RT-61, RT-63). /rpc work runs under the
// request's context bounded by X-Fathom-Deadline (rpcContext), so a client
// that leaves cancels it; serve-http runs an http.Server with header, idle
// and per-route write timeouts and a loopback-Host / same-origin guard on
// every route (newDaemonServer); the managed daemon answers a test caller
// 409 (refuseTestCaller); and the daemon's own in-process console calls go
// through the same gate as console-inprocess (inProcessToolCall).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/internal/hookguard"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// daemonGate is serve-http's admission gate; nil in every CLI process.
var daemonGate atomic.Pointer[tenancy.Gate]

// rpcContext is one /rpc request's context (RT-58, §3.5): the request's
// own (a client disconnect cancels read work; ingest-class calls detach in
// dispatch), the surface scope, the declared origin and caller, and — when
// the client sent X-Fathom-Deadline (whole milliseconds left) — that
// deadline. The class deadline is the admission ticket's.
func rpcContext(r *http.Request, surface string) (context.Context, context.CancelFunc) {
	origin := r.Header.Get(originHeaderName)
	ctx := mcp.WithContracts(mcp.WithOrigin(mcp.WithRequestScope(r.Context(), surface, true), origin), r.URL.Query().Get("contracts") == "true")
	ctx = markHookOrigin(ctx, origin)
	if c, ok := client.ParseCallerHeader(r.Header.Get(client.CallerHeaderName)); ok {
		ctx = mcp.WithCaller(ctx, mcp.CallerInfo{PID: c.PID, PPID: c.PPID, Session: c.Session, Test: c.Test, Cwd: c.Cwd})
	}
	if v := strings.TrimSpace(r.Header.Get(hookguard.DeadlineHeader)); v != "" {
		if ms, err := strconv.ParseInt(v, 10, 64); err == nil && ms > 0 {
			return context.WithTimeout(ctx, time.Duration(ms)*time.Millisecond)
		}
	}
	return context.WithCancel(ctx)
}

// writeRefusal completes a refused /rpc response's HTTP status (§3.4): 503
// with Retry-After. The JSON-RPC body is the -32050 error.
func writeRefusal(w http.ResponseWriter, ref *tenancy.Refusal) {
	status, retry := mcp.RefusalHTTP(ref)
	w.Header().Set("Retry-After", retry)
	// How far the refusal reaches (§11.2): one class's refusal must not
	// open the hooks' shared busy breaker for every other call.
	scope := hookguard.BusyScopeDaemon
	switch ref.Code {
	case tenancy.RefusalSlots, tenancy.RefusalDeadline, tenancy.RefusalCallerGone:
		scope = hookguard.BusyScopeClass
	}
	w.Header().Set(hookguard.BusyScopeHeader, scope)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
}

// daemonServerOpts shapes newDaemonServer.
type daemonServerOpts struct {
	managed bool // answer test callers 409 (§10.4)
	// rpcWriteTimeout overrides daemonRPCWriteTimeout (zero: the default);
	// a test shortens it to reproduce the long-ingest empty answer (EW-36).
	rpcWriteTimeout time.Duration
}

// Server timeouts (§3.5).
const (
	daemonReadHeaderTimeout = 5 * time.Second
	daemonIdleTimeout       = 120 * time.Second
	daemonMaxHeaderBytes    = 64 << 10
	// daemonRPCWriteTimeout is the fleet ceiling (120 s) plus margin.
	daemonRPCWriteTimeout = 150 * time.Second
)

// newDaemonServer is serve-http's http.Server (RT-59): header, idle and
// per-route write timeouts, a loopback Host and a same-origin Origin on
// every route (requireLocalMutation's check, generalized: DNS rebinding
// cannot reach /rpc or the console), and the managed daemon's 409 for a
// test caller. Every route requires the loopback Host: hosted-modules mode,
// the one network-reachable exception, was removed by SR-17.
func newDaemonServer(h http.Handler, o daemonServerOpts) *http.Server {
	h = routeWriteDeadline(h, o.rpcWriteTimeout)
	if o.managed {
		inner := h
		h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if refuseTestCaller(w, r) {
				return
			}
			inner.ServeHTTP(w, r)
		})
	}
	h = requireLoopbackHost(h)
	return &http.Server{
		Handler:           h,
		ReadHeaderTimeout: daemonReadHeaderTimeout,
		IdleTimeout:       daemonIdleTimeout,
		MaxHeaderBytes:    daemonMaxHeaderBytes,
	}
}

// routeWriteDeadline sets each route's write deadline: none for the
// /events SSE stream and the operator's pprof, 150 s for everything else.
func routeWriteDeadline(next http.Handler, rpcTimeout time.Duration) http.Handler {
	if rpcTimeout <= 0 {
		rpcTimeout = daemonRPCWriteTimeout
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := http.NewResponseController(w)
		switch {
		case r.URL.Path == "/events" || strings.HasPrefix(r.URL.Path, "/debug/pprof/"):
			_ = rc.SetWriteDeadline(time.Time{})
		default:
			_ = rc.SetWriteDeadline(time.Now().Add(rpcTimeout))
		}
		next.ServeHTTP(w, r)
	})
}

// liftRPCWriteDeadline removes the route's write deadline for a call that
// runs detached and may outlast it (an ingest-class tools/call, EW-36). It
// reaches the connection through http.ResponseController, so every writer
// wrapped around it must Unwrap (loggingResponseWriter does); the error is
// http.ErrNotSupported when one does not, which a test pins.
func liftRPCWriteDeadline(w http.ResponseWriter) error {
	return http.NewResponseController(w).SetWriteDeadline(time.Time{})
}

// requireLoopbackHost refuses a request whose Host is not loopback, or
// whose Origin (when present) is not the page's own origin.
func requireLoopbackHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hostIsLoopback(r.Host) {
			http.Error(w, fmt.Sprintf("refused: Host %q is not loopback — fathom serves this machine only", r.Host), http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && !sameOrigin(r, origin) {
			http.Error(w, fmt.Sprintf("refused: Origin %q is not a page this daemon served (Host %q)", origin, r.Host), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// testCallersRejected counts test callers the managed daemon refused.
var testCallersRejected atomic.Int64

// refuseTestCaller answers a test caller (X-Fathom-Caller test=1) 409 at
// the managed daemon and counts it (RT-63, §10.4): a test binary that
// reached the operator's daemon is a leak, named, never served.
func refuseTestCaller(w http.ResponseWriter, r *http.Request) bool {
	c, ok := client.ParseCallerHeader(r.Header.Get(client.CallerHeaderName))
	if !ok || !c.Test {
		return false
	}
	testCallersRejected.Add(1)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"reason": "test binary reached the operator daemon",
		"caller": map[string]any{"pid": c.PID, "ppid": c.PPID, "origin": c.Origin},
		"remedy": "tests run under testisolate.Main with FATHOM_HTTP unroutable; start a scratch serve-http for a test that needs a daemon",
	})
	return true
}

// testCallerMetricLine is the /metrics line for refused test callers.
func testCallerMetricLine() string {
	return fmt.Sprintf("fathom_test_callers_rejected_total %d\n", testCallersRejected.Load())
}

// inProcessToolCall runs one of the daemon's own in-process console calls
// (the registry read list_services answers, the project index, conserve)
// through admission as origin console-inprocess (RT-61, §3.3): registered
// in flight, metered, refused by name like any request — excluded from
// agent traffic (mcp.IsConsoleRead) and never an auto-refresh trigger.
// With no gate (a CLI process) fn simply runs.
func inProcessToolCall(ctx context.Context, tool string, class tenancy.Class, fn func(context.Context) error) (err error) {
	ctx = mcp.WithOrigin(ctx, mcp.OriginConsoleInProcess)
	g := daemonGate.Load()
	if g == nil {
		return fn(ctx)
	}
	tk, ref := g.Admit(ctx, tenancy.Request{Tool: tool, Class: class, Origin: tenancy.OriginConsoleInProcess})
	if ref != nil {
		return fmt.Errorf("fathom busy: %s", ref.Reason)
	}
	defer func() {
		// A panicking body releases its ticket and answers a named error:
		// a dead call never holds the class's slot.
		if v := recover(); v != nil {
			err = fmt.Errorf("internal error: %s panicked: %v", tool, v)
		}
		switch {
		case tk.Context().Err() != nil:
			tk.Release(tenancy.OutcomeAbandoned)
		case err != nil:
			tk.Release(tenancy.OutcomeError)
		default:
			tk.Release(tenancy.OutcomeOK)
		}
	}()
	return fn(tk.Context())
}

// lastGoodRegistry is the in-process registry read with its last good
// projection (§3.4: the console's consumer of a refusal renders "the last
// good projection with truth state `stale: fathom shedding`"). The read is
// admitted as fleet with the console's refuse-wait, so any running fleet
// call refuses it; the pages then keep the registry they last had, named
// stale, instead of "service registry unreachable".
type lastGoodRegistry struct {
	read func(context.Context) (map[string]svcInfo, error)

	mu    sync.Mutex
	good  map[string]svcInfo
	asOf  time.Time
	stale string
}

func newLastGoodRegistry(read func(context.Context) (map[string]svcInfo, error)) *lastGoodRegistry {
	return &lastGoodRegistry{read: read}
}

// services reads the registry; a refusal with a last good projection
// answers that projection (staleNote names it), otherwise the error.
func (l *lastGoodRegistry) services(ctx context.Context) (map[string]svcInfo, error) {
	m, err := l.read(ctx)
	l.mu.Lock()
	defer l.mu.Unlock()
	if err == nil {
		l.good, l.asOf, l.stale = m, time.Now(), ""
		return m, nil
	}
	if l.good == nil || !strings.HasPrefix(err.Error(), "fathom busy") {
		return nil, err
	}
	l.stale = fmt.Sprintf("stale: fathom shedding — the registry as of %s (%s)", l.asOf.Format("15:04:05"), strings.TrimPrefix(err.Error(), "fathom busy: "))
	return l.good, nil
}

// staleNote is "" when the last read was fresh, else the stale disclosure.
func (l *lastGoodRegistry) staleNote() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.stale
}

// registryStaleNote is the in-process registry's stale disclosure when
// baseURL is this daemon; "" otherwise.
func registryStaleNote(baseURL string) string {
	if r := inProcessReg.Load(); r != nil && r.stale != nil && sameDaemonURL(baseURL, r.self) {
		return r.stale()
	}
	return ""
}

// admissionMetricLines is the gate's /metrics block and the test-caller
// count.
func admissionMetricLines(srv *mcp.Server) string {
	out := testCallerMetricLine()
	if g := srv.Gate(); g != nil {
		out += g.MetricLines()
	}
	return out
}

// admissionSlotsLine renders the budget's slots for the startup line.
func admissionSlotsLine(b tenancy.Budget) string {
	parts := make([]string, 0, len(tenancy.Classes))
	for _, c := range tenancy.Classes {
		parts = append(parts, fmt.Sprintf("%s %d", c, b.Slots[c]))
	}
	return strings.Join(parts, ", ")
}

// decisionAdmitter admits one decision-port call (mcp.Server.AdmitDecision).
type decisionAdmitter struct {
	admit func(ctx context.Context, site string) (context.Context, func(), string)
}

// decisionAdmit is serve-http's decision-port admission (RT-65); nil in a
// CLI process, where every call proceeds.
var decisionAdmit atomic.Pointer[decisionAdmitter]

// admitDecisionCall admits one decision-port call as the egress class: skip
// is the disclosure when it must not be made (refuse rung, safe mode).
func admitDecisionCall(ctx context.Context, site string) (context.Context, func(), string) {
	if a := decisionAdmit.Load(); a != nil && a.admit != nil {
		return a.admit(ctx, site)
	}
	return ctx, func() {}, ""
}

// gatedDecider closes the decision port in safe mode and at refuse and
// above (RT-39, decisionPortClosed), and — with a site — admits every
// Decide through the egress class (RT-65, vision 36 §8): the obligations
// re-rank and the compile pre-filter reach the provider only when the
// daemon admits it; a skip is the call's error, which those paths already
// disclose fail-open. Without a site it only closes: /route admits its own
// call (admitDecisionCall), so it must not take a second egress slot.
type gatedDecider struct {
	inner decision.Decider
	site  string
}

func (g gatedDecider) Name() string { return g.inner.Name() }

func (g gatedDecider) Decide(ctx context.Context, req decision.Request) (decision.Response, error) {
	if err := decisionPortClosed(); err != nil {
		return decision.Response{}, err
	}
	if g.site == "" {
		return g.inner.Decide(ctx, req)
	}
	ctx, release, skip := admitDecisionCall(ctx, g.site)
	defer release()
	if skip != "" {
		return decision.Response{}, errors.New(skip)
	}
	return g.inner.Decide(ctx, req)
}

// gateDecider wraps an active decider; decision.None stays None (callers
// type-check it to mean "no provider").
func gateDecider(d decision.Decider, site string) decision.Decider {
	if d == nil {
		return nil
	}
	if _, isNone := d.(decision.None); isNone {
		return d
	}
	return gatedDecider{inner: d, site: site}
}
