package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/hookguard"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// testGate is a 1 GiB managed-shape gate over its own in-flight registry.
func testGate(t *testing.T, mode tenancy.Mode) (*tenancy.Gate, *tenancy.InFlight) {
	t.Helper()
	b, _ := tenancy.ResolveBudget(tenancy.Config{Budget: tenancy.GiB}, tenancy.HostFacts{RAM: 16 * tenancy.GiB, NumCPU: 4, Managed: true})
	reg := tenancy.NewInFlight(nil)
	return tenancy.NewGate(tenancy.GateConfig{Budget: b, Mode: mode, InFlight: reg, Level: func() tenancy.Level { return tenancy.LevelNormal }}), reg
}

func gatedTestHandler(t *testing.T, mode tenancy.Mode) (http.Handler, *tenancy.Gate) {
	t.Helper()
	c, err := client.NewConstantClient(client.ModeLocal, filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	srv := mcp.NewServer(c)
	g, _ := testGate(t, mode)
	srv.SetGate(g)
	return newHTTPHandler(srv, c, "127.0.0.1:0", nil), g
}

func toolsCall(tool string) string {
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": tool, "arguments": map[string]any{}}})
	return string(b)
}

// TestClientDisconnectCancelsReadWork (RT-58, design §3.5): /rpc work runs
// under the request's context, so a client that has gone away is never
// computed on — a caller already gone is refused by name before its
// handler runs (a mid-call disconnect stopping the handler is
// internal/mcp's TestDisconnectStopsTheHandler) — and X-Fathom-Deadline
// bounds the call's context.
func TestClientDisconnectCancelsReadWork(t *testing.T) {
	h, g := gatedTestHandler(t, tenancy.ModeObserve)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // the client is already gone
	readSourceCall, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "read_source", "arguments": map[string]any{"service": "svc", "file": "a.go"}}})
	req := httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(string(readSourceCall))).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "caller went away") {
		t.Fatalf("a disconnected client's call was computed on: %s", rec.Body.String())
	}
	if m := g.MetricLines(); !strings.Contains(m, `fathom_rejected_total{class="point",reason="caller_gone",origin="agent"} 1`) {
		t.Fatalf("caller_gone not counted:\n%s", m)
	}

	// rpcContext: the request's ctx, bounded by X-Fathom-Deadline.
	r := httptest.NewRequest(http.MethodPost, "/rpc", nil)
	r.Header.Set("X-Fathom-Deadline", "250")
	// The deadline is 250 ms from the instant rpcContext read the clock, so
	// it lies between the readings either side of the call: a stalled machine
	// only widens the bracket, never breaks it.
	lo := time.Now()
	rctx, done := rpcContext(r, "")
	hi := time.Now()
	defer done()
	dl, ok := rctx.Deadline()
	if !ok || dl.Before(lo.Add(250*time.Millisecond)) || dl.After(hi.Add(250*time.Millisecond)) {
		t.Fatalf("X-Fathom-Deadline 250 ms: ctx deadline %v, want within [%v, %v]", dl, lo.Add(250*time.Millisecond), hi.Add(250*time.Millisecond))
	}
	rc, rcancel := context.WithCancel(context.Background())
	r2 := httptest.NewRequest(http.MethodPost, "/rpc", nil).WithContext(rc)
	r2.Header.Set(client.CallerHeaderName, "pid=7;ppid=1;origin=edit-hook;session=s;test=0")
	r2ctx, done2 := rpcContext(r2, "agent")
	defer done2()
	rcancel()
	select {
	case <-r2ctx.Done():
	case <-time.After(waitSafety):
		t.Fatal("the rpc ctx outlived its request")
	}
}

// TestDaemonServerTimeoutsAndLoopbackHost (RT-59, §3.5): serve-http runs an
// http.Server with header, idle and per-route write timeouts, and every
// route requires a loopback Host and, when present, a same-origin Origin.
func TestDaemonServerTimeoutsAndLoopbackHost(t *testing.T) {
	var seen []time.Time
	var mu sync.Mutex
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	s := newDaemonServer(inner, daemonServerOpts{})
	if s.ReadHeaderTimeout != 5*time.Second || s.IdleTimeout != 120*time.Second || s.MaxHeaderBytes != 64<<10 {
		t.Fatalf("server timeouts: header %s idle %s maxHeader %d", s.ReadHeaderTimeout, s.IdleTimeout, s.MaxHeaderBytes)
	}
	serve := func(host, origin, path string) int {
		r := httptest.NewRequest(http.MethodPost, path, nil)
		r.Host = host
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder(), mu: &mu, seen: &seen}
		s.Handler.ServeHTTP(w, r)
		return w.Code
	}
	for _, host := range []string{"127.0.0.1:7581", "localhost:7581", "[::1]:7581"} {
		if code := serve(host, "", "/rpc"); code != http.StatusOK {
			t.Fatalf("loopback Host %s: %d", host, code)
		}
	}
	if code := serve("evil.example:7581", "", "/rpc"); code != http.StatusForbidden {
		t.Fatalf("DNS-rebinding Host: %d, want 403", code)
	}
	if code := serve("evil.example", "", "/"); code != http.StatusForbidden {
		t.Fatalf("console route with a foreign Host: %d, want 403", code)
	}
	if code := serve("127.0.0.1:7581", "http://evil.example", "/rpc"); code != http.StatusForbidden {
		t.Fatalf("cross-origin Origin: %d, want 403", code)
	}
	if code := serve("127.0.0.1:7581", "http://localhost:7581", "/rpc"); code != http.StatusOK {
		t.Fatalf("same-origin Origin: %d", code)
	}

	mu.Lock()
	seen = nil
	mu.Unlock()
	lo := time.Now()
	serve("127.0.0.1:7581", "", "/rpc/agent")
	hi := time.Now()
	serve("127.0.0.1:7581", "", "/events")
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 2 {
		t.Fatalf("write deadlines set %d times, want 2", len(seen))
	}
	// 150 s from the instant the handler read the clock: between the
	// readings either side of the request.
	if seen[0].Before(lo.Add(150*time.Second)) || seen[0].After(hi.Add(150*time.Second)) {
		t.Fatalf("/rpc write deadline %v, want 150 s out within [%v, %v]", seen[0], lo.Add(150*time.Second), hi.Add(150*time.Second))
	}
	if !seen[1].IsZero() {
		t.Fatalf("/events (SSE) has a write deadline %v; it must have none", seen[1])
	}

	// SR-17 removed hosted-modules mode: a network Host is refused on every
	// path, /rpc/modules included — no configuration opts out of the guard.
	r := httptest.NewRequest(http.MethodPost, "/rpc/modules", nil)
	r.Host = "fathom.example.com:443"
	w := httptest.NewRecorder()
	s.Handler.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("network Host served: %d, want 403", w.Code)
	}
}

// deadlineRecorder records SetWriteDeadline calls (http.ResponseController).
type deadlineRecorder struct {
	*httptest.ResponseRecorder
	mu   *sync.Mutex
	seen *[]time.Time
}

func (d *deadlineRecorder) SetWriteDeadline(t time.Time) error {
	d.mu.Lock()
	*d.seen = append(*d.seen, t)
	d.mu.Unlock()
	return nil
}

// TestTestCallerRefusedByManagedDaemon (RT-63, §10.4): the managed daemon
// answers a test caller 409 and counts it; an unmanaged daemon serves it.
func TestTestCallerRefusedByManagedDaemon(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	before := testCallersRejected.Load()
	managed := newDaemonServer(inner, daemonServerOpts{managed: true})
	call := func(s *http.Server, caller string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(toolsCall("capabilities")))
		r.Host = "127.0.0.1:7581"
		if caller != "" {
			r.Header.Set(client.CallerHeaderName, caller)
		}
		w := httptest.NewRecorder()
		s.Handler.ServeHTTP(w, r)
		return w
	}
	w := call(managed, "pid=9;ppid=8;origin=;session=;test=1")
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "test binary reached the operator daemon") {
		t.Fatalf("test caller at the managed daemon: %d %s", w.Code, w.Body.String())
	}
	if n := testCallersRejected.Load() - before; n != 1 {
		t.Fatalf("rejections counted %d, want 1", n)
	}
	if !strings.Contains(testCallerMetricLine(), "fathom_test_callers_rejected_total") {
		t.Fatal("the count is not exposed")
	}
	for _, caller := range []string{"pid=9;ppid=8;origin=edit-hook;session=;test=0", ""} {
		if w := call(managed, caller); w.Code != http.StatusOK {
			t.Fatalf("non-test caller %q refused: %d", caller, w.Code)
		}
	}
	if w := call(newDaemonServer(inner, daemonServerOpts{}), "pid=9;ppid=8;origin=;session=;test=1"); w.Code != http.StatusOK {
		t.Fatalf("an unmanaged daemon must serve tests: %d", w.Code)
	}
}

// TestInProcessCallsMeteredAsConsole (RT-61, §3.3): the daemon's own
// in-process console calls go through admission as console-inprocess —
// registered in flight, metered, refused by name — and carry the console
// origin that keeps auto-refresh off.
func TestInProcessCallsMeteredAsConsole(t *testing.T) {
	g, reg := testGate(t, tenancy.ModeEnforce)
	daemonGate.Store(g)
	defer daemonGate.Store(nil)

	var during []tenancy.InFlightEntry
	var origin string
	err := inProcessToolCall(context.Background(), "list_services", tenancy.ClassFleet, func(ctx context.Context) error {
		during = reg.Snapshot()
		origin = mcp.OriginOf(ctx)
		if tk := tenancy.TicketFrom(ctx); tk == nil || tk.Origin() != tenancy.OriginConsoleInProcess {
			t.Errorf("no console-inprocess ticket in the call's ctx")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(during) != 1 || during[0].Origin != "console-inprocess" || during[0].Tool != "list_services" || during[0].Class != tenancy.ClassFleet {
		t.Fatalf("in flight during the call: %+v", during)
	}
	if origin != mcp.OriginConsoleInProcess || !mcp.IsConsoleRead(origin) {
		t.Fatalf("origin %q: not a console read (auto-refresh and agent traffic exclude it)", origin)
	}
	if !strings.Contains(g.MetricLines(), `fathom_calls_total{class="fleet",outcome="ok"} 1`) {
		t.Fatalf("in-process call not metered:\n%s", g.MetricLines())
	}

	// The fleet slot held: an in-process console call is refused by name,
	// never queued (the console's fleet wait is refuse).
	held, _ := g.Admit(context.Background(), tenancy.Request{Tool: "list_services", Class: tenancy.ClassFleet, Origin: tenancy.OriginAgent})
	defer held.Release(tenancy.OutcomeOK)
	ran := false
	err = inProcessToolCall(context.Background(), "list_services", tenancy.ClassFleet, func(context.Context) error { ran = true; return nil })
	if ran || err == nil || !strings.Contains(err.Error(), "fleet-class slots full") {
		t.Fatalf("refused in-process call: ran %v err %v", ran, err)
	}

	// With no gate (a CLI process) the call simply runs.
	daemonGate.Store(nil)
	if err := inProcessToolCall(context.Background(), "list_services", tenancy.ClassFleet, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
}

// TestInProcessPanicReleasesItsSlot (review finding, RT-61): an in-process
// console call whose body panics releases its ticket and answers a named
// error — the fleet class's single slot is never held by a dead call.
func TestInProcessPanicReleasesItsSlot(t *testing.T) {
	g, reg := testGate(t, tenancy.ModeEnforce)
	daemonGate.Store(g)
	defer daemonGate.Store(nil)
	var err error
	func() {
		defer func() {
			if v := recover(); v != nil {
				t.Errorf("the panic escaped inProcessToolCall: %v", v)
			}
		}()
		err = inProcessToolCall(context.Background(), "list_services", tenancy.ClassFleet, func(context.Context) error { panic("boom") })
	}()
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("want a named error, got %v", err)
	}
	if n := len(reg.Snapshot()); n != 0 {
		t.Fatalf("%d in-flight entries left behind", n)
	}
	if err := inProcessToolCall(context.Background(), "list_services", tenancy.ClassFleet, func(context.Context) error { return nil }); err != nil {
		t.Fatalf("the panicked call kept its fleet slot: %v", err)
	}
}

// TestIngestClassLiftsTheWriteDeadline (review finding, RT-59/§3.4): an
// ingest-class /rpc call runs detached with no class deadline, so the
// route's 150 s write deadline must not cut its client off mid-ingest: the
// call lifts it. A read keeps it.
func TestIngestClassLiftsTheWriteDeadline(t *testing.T) {
	h, _ := gatedTestHandler(t, tenancy.ModeEnforce)
	h = routeWriteDeadline(h, 0)
	serve := func(body string) []time.Time {
		var mu sync.Mutex
		var seen []time.Time
		r := httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(body))
		w := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder(), mu: &mu, seen: &seen}
		h.ServeHTTP(w, r)
		return seen
	}
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "remove_service", "arguments": map[string]any{"service": "nope"}}})
	seen := serve(string(b))
	if len(seen) < 2 || !seen[len(seen)-1].IsZero() {
		t.Fatalf("ingest-class call: write deadlines %v, want the route's then none", seen)
	}
	seen = serve(toolsCall("capabilities"))
	if len(seen) != 1 || seen[0].IsZero() {
		t.Fatalf("read call: write deadlines %v, want the route's 150 s only", seen)
	}
}

// TestRefusalNamesItsScope (review finding, RT-56 + §11.2): a class-scoped
// refusal (slots full) says so in X-Fathom-Busy-Scope, so the hooks' shared
// breaker does not hold every other call off; the refuse rung's refusal is
// daemon-scoped.
func TestRefusalNamesItsScope(t *testing.T) {
	for _, tc := range []struct {
		ref  tenancy.Refusal
		want string
	}{
		{tenancy.Refusal{Code: tenancy.RefusalSlots, RetryAfter: 30 * time.Second}, hookguard.BusyScopeClass},
		{tenancy.Refusal{Code: tenancy.RefusalDeadline, RetryAfter: time.Second}, hookguard.BusyScopeClass},
		{tenancy.Refusal{Code: tenancy.RefusalLevel, RetryAfter: 30 * time.Second}, hookguard.BusyScopeDaemon},
		{tenancy.Refusal{Code: tenancy.RefusalSafeMode, RetryAfter: 60 * time.Second}, hookguard.BusyScopeDaemon},
	} {
		rec := httptest.NewRecorder()
		writeRefusal(rec, &tc.ref)
		if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" {
			t.Fatalf("%s: %d Retry-After %q", tc.ref.Code, rec.Code, rec.Header().Get("Retry-After"))
		}
		if got := rec.Header().Get(hookguard.BusyScopeHeader); got != tc.want {
			t.Fatalf("%s: scope %q, want %q", tc.ref.Code, got, tc.want)
		}
	}
}

// TestRegistryServesLastGoodWhenShedding (review finding, RT-61 + §3.4):
// the in-process registry read is admitted as fleet with the console's
// refuse-wait, so it is refused whenever any fleet call runs; the console
// then serves the last good projection named `stale: fathom shedding`,
// never "registry unreachable" — and only a registry never read fails.
func TestRegistryServesLastGoodWhenShedding(t *testing.T) {
	g, _ := testGate(t, tenancy.ModeEnforce)
	daemonGate.Store(g)
	defer daemonGate.Store(nil)
	calls := 0
	reg := newLastGoodRegistry(func(ctx context.Context) (map[string]svcInfo, error) {
		var out map[string]svcInfo
		err := inProcessToolCall(ctx, "list_services", tenancy.ClassFleet, func(context.Context) error {
			calls++
			out = map[string]svcInfo{"demo": {symbols: calls}}
			return nil
		})
		return out, err
	})

	held, _ := g.Admit(context.Background(), tenancy.Request{Tool: "review_diff", Class: tenancy.ClassFleet, Origin: tenancy.OriginAgent})
	if _, err := reg.services(context.Background()); err == nil || !strings.Contains(err.Error(), "fathom busy") {
		t.Fatalf("never read: want the busy error, got %v", err)
	}
	held.Release(tenancy.OutcomeOK)

	got, err := reg.services(context.Background())
	if err != nil || got["demo"].symbols != 1 || reg.staleNote() != "" {
		t.Fatalf("fresh read: %v %v note %q", got, err, reg.staleNote())
	}
	held, _ = g.Admit(context.Background(), tenancy.Request{Tool: "review_diff", Class: tenancy.ClassFleet, Origin: tenancy.OriginAgent})
	defer held.Release(tenancy.OutcomeOK)
	got, err = reg.services(context.Background())
	if err != nil || got["demo"].symbols != 1 {
		t.Fatalf("shedding: want the last good projection, got %v %v", got, err)
	}
	if note := reg.staleNote(); !strings.HasPrefix(note, "stale: fathom shedding") || !strings.Contains(note, "review_diff") {
		t.Fatalf("shedding note %q", note)
	}
}
