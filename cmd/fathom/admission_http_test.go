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
// under the request's context, so a client that has gone away cancels it
// and the call is counted abandoned — never computed on for nobody; and
// X-Fathom-Deadline bounds the call's context.
func TestClientDisconnectCancelsReadWork(t *testing.T) {
	h, g := gatedTestHandler(t, tenancy.ModeObserve)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // the client is already gone
	req := httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(toolsCall("capabilities"))).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `"outcome":"abandoned"`) {
		t.Fatalf("a disconnected client's call was not abandoned: %s", rec.Body.String())
	}
	if m := g.MetricLines(); !strings.Contains(m, `fathom_calls_total{class="point",outcome="abandoned"} 1`) {
		t.Fatalf("abandoned not counted:\n%s", m)
	}

	// rpcContext: the request's ctx, bounded by X-Fathom-Deadline.
	r := httptest.NewRequest(http.MethodPost, "/rpc", nil)
	r.Header.Set("X-Fathom-Deadline", "250")
	rctx, done := rpcContext(r, "")
	defer done()
	dl, ok := rctx.Deadline()
	if !ok || time.Until(dl) > 250*time.Millisecond || time.Until(dl) < 100*time.Millisecond {
		t.Fatalf("X-Fathom-Deadline 250 ms: ctx deadline %v", dl)
	}
	rc, rcancel := context.WithCancel(context.Background())
	r2 := httptest.NewRequest(http.MethodPost, "/rpc", nil).WithContext(rc)
	r2.Header.Set(client.CallerHeaderName, "pid=7;ppid=1;origin=edit-hook;session=s;test=0")
	r2ctx, done2 := rpcContext(r2, "agent")
	defer done2()
	rcancel()
	select {
	case <-r2ctx.Done():
	case <-time.After(time.Second):
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
	for _, host := range []string{"127.0.0.1:7575", "localhost:7575", "[::1]:7575"} {
		if code := serve(host, "", "/rpc"); code != http.StatusOK {
			t.Fatalf("loopback Host %s: %d", host, code)
		}
	}
	if code := serve("evil.example:7575", "", "/rpc"); code != http.StatusForbidden {
		t.Fatalf("DNS-rebinding Host: %d, want 403", code)
	}
	if code := serve("evil.example", "", "/"); code != http.StatusForbidden {
		t.Fatalf("console route with a foreign Host: %d, want 403", code)
	}
	if code := serve("127.0.0.1:7575", "http://evil.example", "/rpc"); code != http.StatusForbidden {
		t.Fatalf("cross-origin Origin: %d, want 403", code)
	}
	if code := serve("127.0.0.1:7575", "http://localhost:7575", "/rpc"); code != http.StatusOK {
		t.Fatalf("same-origin Origin: %d", code)
	}

	mu.Lock()
	seen = nil
	mu.Unlock()
	serve("127.0.0.1:7575", "", "/rpc/agent")
	serve("127.0.0.1:7575", "", "/events")
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 2 {
		t.Fatalf("write deadlines set %d times, want 2", len(seen))
	}
	if d := time.Until(seen[0]); d < 140*time.Second || d > 150*time.Second {
		t.Fatalf("/rpc write deadline in %s, want 150 s", d)
	}
	if !seen[1].IsZero() {
		t.Fatalf("/events (SSE) has a write deadline %v; it must have none", seen[1])
	}

	// Hosted mode (a modules token) serves a network Host.
	hosted := newDaemonServer(inner, daemonServerOpts{hosted: true})
	r := httptest.NewRequest(http.MethodPost, "/rpc/modules", nil)
	r.Host = "fathom.internal:443"
	w := httptest.NewRecorder()
	hosted.Handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("hosted Host refused: %d", w.Code)
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
		r.Host = "127.0.0.1:7575"
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

func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition never held")
		}
		time.Sleep(time.Millisecond)
	}
}
