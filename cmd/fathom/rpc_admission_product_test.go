package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/hookguard"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// admissionProductBackend is the store client with a ServiceMeta the test
// steers by service name: "block" waits in the handler until the call's
// context ends (search reads it inside the handler), "boom" is an
// ordinary handler error, and "panic" panics once (symbol_context reads it
// in dispatch's setup, after admission: GDEV-08's S1 path).
type admissionProductBackend struct {
	*client.Client
	entered   chan struct{}
	panicOnce atomic.Bool
}

func (b *admissionProductBackend) ServiceMeta(ctx context.Context, svc string) (string, string, bool, error) {
	switch svc {
	case "block":
		b.entered <- struct{}{}
		<-ctx.Done()
		return "", "", false, ctx.Err()
	case "boom":
		return "", "", false, errors.New("boom")
	case "panic":
		if b.panicOnce.CompareAndSwap(false, true) {
			panic("setup boom")
		}
	}
	return b.Client.ServiceMeta(ctx, svc)
}

// callLogBuf is the call log, read only after the call that wrote it
// returned.
type callLogBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *callLogBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *callLogBuf) last() map[string]any {
	l.mu.Lock()
	defer l.mu.Unlock()
	lines := strings.Split(strings.TrimSpace(l.b.String()), "\n")
	var row map[string]any
	_ = json.Unmarshal([]byte(lines[len(lines)-1]), &row)
	return row
}

func (l *callLogBuf) rows() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	if s := strings.TrimSpace(l.b.String()); s != "" {
		return len(strings.Split(s, "\n"))
	}
	return 0
}

// steppedClock is the gate's injected clock.
type steppedClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *steppedClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *steppedClock) Add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// expiringCtx is a parent whose deadline the test expires by closing done:
// the call's context reports DeadlineExceeded, as an X-Fathom-Deadline that
// ran out does, with no wall-clock wait.
type expiringCtx struct {
	context.Context
	done chan struct{}
	dl   time.Time
}

func (e *expiringCtx) Deadline() (time.Time, bool) { return e.dl, true }
func (e *expiringCtx) Done() <-chan struct{}       { return e.done }
func (e *expiringCtx) Err() error {
	select {
	case <-e.done:
		return context.DeadlineExceeded
	default:
		return nil
	}
}

type productRig struct {
	h      http.Handler
	gate   *tenancy.Gate
	be     *admissionProductBackend
	log    *callLogBuf
	clk    *steppedClock
	level  atomic.Uint32
	timers chan chan time.Time
}

// newProductRig is serve-http's /rpc handler (newHTTPHandlerLogged) over
// real tools and the steerable backend, with an enforcing gate at a 1 GiB
// managed budget on the process's in-flight registry (the product gate's,
// which the lifecycle refusal's in_flight reads), an injected clock, level
// and queue-wait timer.
func newProductRig(t *testing.T) *productRig {
	t.Helper()
	resetLifecycleModes(t)
	if n := tenancy.DefaultInFlight.Len(); n != 0 {
		t.Fatalf("rig premise: %d in-flight entries before the test: %+v", n, tenancy.DefaultInFlight.Snapshot())
	}
	c, err := client.NewConstantClient(client.ModeLocal, filepath.Join(t.TempDir(), "rpc.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	r := &productRig{be: &admissionProductBackend{Client: c, entered: make(chan struct{}, 1)}, log: &callLogBuf{},
		clk: &steppedClock{t: time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)}, timers: make(chan chan time.Time, 1)}
	srv := mcp.NewServerWithLogger(r.be, mcp.NewCallLogger(r.log))
	budget, _ := tenancy.ResolveBudget(tenancy.Config{Budget: tenancy.GiB}, tenancy.HostFacts{RAM: 16 * tenancy.GiB, NumCPU: 4, Managed: true})
	if budget.Slots[tenancy.ClassTree] != 1 {
		t.Fatalf("rig premise: tree has %d slots at 1 GiB, want 1", budget.Slots[tenancy.ClassTree])
	}
	r.gate = tenancy.NewGate(tenancy.GateConfig{Budget: budget, Mode: tenancy.ModeEnforce, Now: r.clk.Now,
		Level: func() tenancy.Level { return tenancy.Level(r.level.Load()) },
		Timer: func(time.Duration) (<-chan time.Time, func() bool) {
			ch := make(chan time.Time, 1)
			r.timers <- ch
			return ch, func() bool { return true }
		}})
	srv.SetGate(r.gate)
	r.h = newHTTPHandlerLogged(srv, c, "127.0.0.1:0", nil, testDiscardLogger())
	return r
}

type rpcAnswer struct {
	rec      *httptest.ResponseRecorder
	panicked any
	Error    *struct {
		Code    int            `json:"code"`
		Message string         `json:"message"`
		Data    map[string]any `json:"data"`
	} `json:"error"`
}

// do posts one tools/call to /rpc; a panic out of the handler is caught
// and returned, so a regression fails by name.
func (r *productRig) do(t *testing.T, ctx context.Context, tool string, args map[string]any, hdr map[string]string) rpcAnswer {
	body := rpcBody(t, map[string]any{"jsonrpc": "2.0", "id": 7, "method": "tools/call", "params": map[string]any{"name": tool, "arguments": args}})
	req := httptest.NewRequest(http.MethodPost, "/rpc", body).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	a := rpcAnswer{rec: httptest.NewRecorder()}
	func() {
		defer func() { a.panicked = recover() }()
		r.h.ServeHTTP(a.rec, req)
	}()
	if a.panicked == nil {
		_ = json.Unmarshal(a.rec.Body.Bytes(), &a)
	}
	return a
}

// async runs do on its own goroutine; the answer arrives on the channel.
func (r *productRig) async(t *testing.T, ctx context.Context, tool string, args map[string]any, hdr map[string]string) <-chan rpcAnswer {
	out := make(chan rpcAnswer, 1)
	go func() { out <- r.do(t, ctx, tool, args, hdr) }()
	return out
}

func awaitAnswer(t *testing.T, ch <-chan rpcAnswer) rpcAnswer {
	t.Helper()
	select {
	case a := <-ch:
		return a
	case <-time.After(waitSafety):
		t.Fatal("the call never answered")
		return rpcAnswer{}
	}
}

func (r *productRig) entered(t *testing.T) {
	t.Helper()
	select {
	case <-r.be.entered:
	case <-time.After(waitSafety):
		t.Fatal("the call never reached its handler")
	}
}

func (r *productRig) queued(t *testing.T) chan time.Time {
	t.Helper()
	select {
	case ch := <-r.timers:
		return ch
	case <-time.After(waitSafety):
		t.Fatal("the call never queued")
		return nil
	}
}

var searchArgs = map[string]any{"service": "nope", "pattern": "x"}
var hookOrigin = map[string]string{originHeaderName: "edit-hook"}

// assertRefused checks §3.4's refusal: 503, Retry-After, the busy scope,
// and -32050 with every data key the compatibility surface lists, named.
func assertRefused(t *testing.T, a rpcAnswer, scope string, inFlight []string) {
	t.Helper()
	if a.panicked != nil {
		t.Fatalf("panic escaped the handler: %v", a.panicked)
	}
	if a.rec.Code != http.StatusServiceUnavailable || a.rec.Header().Get("Retry-After") == "" {
		t.Fatalf("want 503 with Retry-After, got %d %q: %s", a.rec.Code, a.rec.Header().Get("Retry-After"), a.rec.Body)
	}
	if got := a.rec.Header().Get(hookguard.BusyScopeHeader); got != scope {
		t.Fatalf("busy scope %q, want %q", got, scope)
	}
	if a.Error == nil || a.Error.Code != mcp.ErrCodeBusy {
		t.Fatalf("want -32050: %s", a.rec.Body)
	}
	d := a.Error.Data
	for _, k := range []string{"reason", "class", "level", "retry_after_ms", "in_flight", "outcome"} {
		if _, ok := d[k]; !ok {
			t.Fatalf("data lacks %q: %s", k, a.rec.Body)
		}
	}
	if reason, _ := d["reason"].(string); reason == "" || !strings.HasSuffix(a.Error.Message, reason) {
		t.Fatalf("a refusal names its reason: %s", a.rec.Body)
	}
	if d["outcome"] != "refused" {
		t.Fatalf("outcome %v, want refused", d["outcome"])
	}
	raw, ok := d["in_flight"].([]any)
	if !ok {
		t.Fatalf("in_flight must be a list: %s", a.rec.Body)
	}
	got := make([]string, 0, len(raw))
	for _, v := range raw {
		got = append(got, v.(string))
	}
	if strings.Join(got, ",") != strings.Join(inFlight, ",") {
		t.Fatalf("in_flight %v, want %v", got, inFlight)
	}
}

// assertAbandoned checks an abandonment named in the response and in the
// call log's outcome.
func assertAbandoned(t *testing.T, r *productRig, a rpcAnswer, outcome, cause string) {
	t.Helper()
	if a.panicked != nil {
		t.Fatalf("panic escaped the handler: %v", a.panicked)
	}
	if a.Error == nil || a.Error.Data["outcome"] != outcome || !strings.Contains(a.Error.Message, cause) {
		t.Fatalf("want an abandonment named %q (%q): %s", outcome, cause, a.rec.Body)
	}
	if row := r.log.last(); row["outcome"] != outcome {
		t.Fatalf("call-log outcome %v, want %q", row["outcome"], outcome)
	}
}

// assertNoLeak is (c): nothing in flight, no running count, and the
// one-slot tree class admits the next call.
func assertNoLeak(t *testing.T, r *productRig) {
	t.Helper()
	if n := tenancy.DefaultInFlight.Len(); n != 0 {
		t.Fatalf("%d in-flight entries left behind: %+v", n, tenancy.DefaultInFlight.Snapshot())
	}
	if lines := r.gate.MetricLines(); strings.Contains(lines, "fathom_inflight{") {
		t.Fatalf("a running count outlived its call:\n%s", lines)
	}
	a := r.do(t, context.Background(), "search", searchArgs, hookOrigin)
	if a.panicked != nil || a.rec.Code == http.StatusServiceUnavailable || (a.Error != nil && a.Error.Code == mcp.ErrCodeBusy) {
		t.Fatalf("the tree class did not admit the next call: %d %s", a.rec.Code, a.rec.Body)
	}
}

// TestRPCAdmissionNamesEveryRefusalAndLeaksNoSlot (GDEV-08, design §3.3 to
// §3.5): through serve-http's /rpc handler with real tools, every refusal
// (admission's slots, level, deadline and caller-gone; the lifecycle gate's
// safe mode, drain and store distress; the store-unavailable handler) is a
// named 503 with every §3.4 data key; every abandonment (a client that
// left, a deadline that passed, the refuse rung's cancel) is named in the
// response and the call log; and after each, and after an ordinary error
// and a setup panic, no slot, in-flight entry or running count is left.
func TestRPCAdmissionNamesEveryRefusalAndLeaksNoSlot(t *testing.T) {
	bg := context.Background()
	admitTree := func(t *testing.T, r *productRig, tool string) *tenancy.Ticket {
		t.Helper()
		tk, ref := r.gate.Admit(bg, tenancy.Request{Tool: tool, Class: tenancy.ClassTree, Origin: tenancy.OriginAgent})
		if ref != nil {
			t.Fatalf("hold: %+v", ref)
		}
		t.Cleanup(func() { tk.Release(tenancy.OutcomeOK) }) // a failed case never leaks into the next
		return tk
	}
	holdTree := func(t *testing.T, r *productRig) *tenancy.Ticket { return admitTree(t, r, "holder") }

	t.Run("slots full", func(t *testing.T) {
		r := newProductRig(t)
		tk := holdTree(t, r)
		assertRefused(t, r.do(t, bg, "search", searchArgs, hookOrigin), hookguard.BusyScopeClass, []string{"holder"})
		if row := r.log.last(); row["outcome"] != "refused" {
			t.Fatalf("call-log outcome %v, want refused", row["outcome"])
		}
		ch := r.async(t, bg, "search", searchArgs, nil)
		r.queued(t) <- time.Time{} // an agent waits, then the wait ends
		assertRefused(t, awaitAnswer(t, ch), hookguard.BusyScopeClass, []string{"holder"})
		tk.Release(tenancy.OutcomeOK)
		assertNoLeak(t, r)
	})

	t.Run("caller gone", func(t *testing.T) {
		r := newProductRig(t)
		tk := holdTree(t, r)
		ctx, cancel := context.WithCancel(bg)
		ch := r.async(t, ctx, "search", searchArgs, nil)
		r.queued(t)
		cancel() // the caller leaves while it waits
		assertRefused(t, awaitAnswer(t, ch), hookguard.BusyScopeClass, []string{})
		tk.Release(tenancy.OutcomeOK)
		gone, cancelGone := context.WithCancel(bg)
		cancelGone() // and one already gone at admission
		assertRefused(t, r.do(t, gone, "search", searchArgs, nil), hookguard.BusyScopeClass, []string{})
		assertNoLeak(t, r)
	})

	t.Run("level at refuse", func(t *testing.T) {
		r := newProductRig(t)
		r.level.Store(uint32(tenancy.LevelRefuse))
		assertRefused(t, r.do(t, bg, "search", searchArgs, nil), hookguard.BusyScopeDaemon, []string{})
		r.level.Store(uint32(tenancy.LevelNormal))
		assertNoLeak(t, r)
	})

	t.Run("deadline refusal", func(t *testing.T) {
		r := newProductRig(t)
		for range 8 { // search's recent p50 becomes a minute on the gate's clock
			tk := admitTree(t, r, "search")
			r.clk.Add(time.Minute)
			tk.Release(tenancy.OutcomeOK)
		}
		a := r.do(t, bg, "search", searchArgs, map[string]string{hookguard.DeadlineHeader: "30000"})
		assertRefused(t, a, hookguard.BusyScopeClass, []string{})
		if !strings.Contains(a.Error.Data["reason"].(string), "cannot finish before your deadline") {
			t.Fatalf("reason: %v", a.Error.Data["reason"])
		}
		assertNoLeak(t, r)
	})

	t.Run("safe mode", func(t *testing.T) {
		r := newProductRig(t)
		tk := holdTree(t, r)
		tenancy.SetSafeMode(&tenancy.SafeModeInfo{Since: r.clk.Now(), Reason: "test"})
		before := r.log.rows()
		a := r.do(t, bg, "search", searchArgs, nil)
		assertRefused(t, a, hookguard.BusyScopeDaemon, []string{"holder"})
		if a.Error.Data["level"] != tenancy.CurrentLevel().String() {
			t.Fatalf("level %v, want the ladder's %s", a.Error.Data["level"], tenancy.CurrentLevel())
		}
		if r.log.rows() != before {
			t.Fatal("a lifecycle refusal is answered before dispatch; it writes no call-log row")
		}
		tenancy.SetSafeMode(nil)
		tk.Release(tenancy.OutcomeOK)
		assertNoLeak(t, r)
	})

	t.Run("drain", func(t *testing.T) {
		r := newProductRig(t)
		tk := holdTree(t, r)
		draining.Store(true)
		assertRefused(t, r.do(t, bg, "search", searchArgs, nil), hookguard.BusyScopeDaemon, []string{"holder"})
		draining.Store(false)
		tk.Release(tenancy.OutcomeOK)
		assertNoLeak(t, r)
	})

	t.Run("store distress", func(t *testing.T) {
		r := newProductRig(t)
		tenancy.NoteStoreVolumeFree(1, 2, time.Time{})
		assertRefused(t, r.do(t, bg, "ingest", map[string]any{"service": "nope"}, nil), hookguard.BusyScopeClass, []string{})
		tenancy.ResetLifecycleStateForTest()
		assertNoLeak(t, r)
	})

	t.Run("store unavailable", func(t *testing.T) {
		var st atomic.Pointer[storeWaitState]
		st.Store(&storeWaitState{Errno: "EPERM", Label: storeWaitTCC, Err: "operation not permitted", Remedy: "grant access"})
		sw := newSwapHandler(waitingHandler(&st))
		body := rpcBody(t, map[string]any{"jsonrpc": "2.0", "id": 9, "method": "tools/call", "params": map[string]any{"name": "search", "arguments": searchArgs}})
		a := rpcAnswer{rec: httptest.NewRecorder()}
		sw.ServeHTTP(a.rec, httptest.NewRequest(http.MethodPost, "/rpc", body))
		_ = json.Unmarshal(a.rec.Body.Bytes(), &a)
		assertRefused(t, a, hookguard.BusyScopeDaemon, []string{})
	})

	t.Run("client disconnect", func(t *testing.T) {
		r := newProductRig(t)
		ctx, cancel := context.WithCancel(bg)
		ch := r.async(t, ctx, "search", map[string]any{"service": "block", "pattern": "x"}, nil)
		r.entered(t)
		cancel()
		assertAbandoned(t, r, awaitAnswer(t, ch), "abandoned", "the client went away")
		assertNoLeak(t, r)
	})

	t.Run("deadline passed", func(t *testing.T) {
		r := newProductRig(t)
		ex := &expiringCtx{Context: bg, done: make(chan struct{}), dl: r.clk.Now().Add(time.Hour)}
		ch := r.async(t, ex, "search", map[string]any{"service": "block", "pattern": "x"}, map[string]string{hookguard.DeadlineHeader: "600000"})
		r.entered(t)
		close(ex.done)
		assertAbandoned(t, r, awaitAnswer(t, ch), "abandoned", "deadline passed")
		assertNoLeak(t, r)
	})

	t.Run("refuse rung cancels", func(t *testing.T) {
		r := newProductRig(t)
		ch := r.async(t, bg, "search", map[string]any{"service": "block", "pattern": "x"}, nil)
		r.entered(t)
		tenancy.DefaultInFlight.Cancel(tenancy.AbandonedBy(tenancy.Cause{}), tenancy.ClassTree, tenancy.ClassFleet)
		assertAbandoned(t, r, awaitAnswer(t, ch), "abandoned-by-governor", "")
		assertNoLeak(t, r)
	})

	t.Run("handler error", func(t *testing.T) {
		r := newProductRig(t)
		a := r.do(t, bg, "search", map[string]any{"service": "boom", "pattern": "x"}, nil)
		if a.panicked != nil || a.Error == nil {
			t.Fatalf("want a handler error: %v %s", a.panicked, a.rec.Body)
		}
		if row := r.log.last(); row["outcome"] != "error" {
			t.Fatalf("call-log outcome %v, want error", row["outcome"])
		}
		assertNoLeak(t, r)
	})

	t.Run("setup panic", func(t *testing.T) {
		r := newProductRig(t)
		a := r.do(t, bg, "symbol_context", map[string]any{"service": "panic", "symbol": "x"}, nil)
		if a.panicked != nil {
			t.Fatalf("the panic escaped the handler: %v", a.panicked)
		}
		if a.Error == nil || a.Error.Data["outcome"] != "error" || !strings.Contains(a.Error.Message, "panicked: setup boom") {
			t.Fatalf("want the named panic error: %s", a.rec.Body)
		}
		if row := r.log.last(); row["outcome"] != "error" || row["class"] != "tree" {
			t.Fatalf("call-log row %v, want outcome error, class tree", row)
		}
		assertNoLeak(t, r)
	})
}

// TestLifecycleBusyInFlightIsSorted (GDEV-08): the lifecycle body's in_flight is
// the refused class's tools in flight, sorted, whatever the registry's order.
func TestLifecycleBusyInFlightIsSorted(t *testing.T) {
	resetLifecycleModes(t)
	g := tenancy.NewGate(tenancy.GateConfig{Budget: func() tenancy.Budget {
		b, _ := tenancy.ResolveBudget(tenancy.Config{Budget: 8 * tenancy.GiB}, tenancy.HostFacts{RAM: 64 * tenancy.GiB, NumCPU: 16, Managed: true})
		return b
	}(), Mode: tenancy.ModeEnforce})
	var tks []*tenancy.Ticket
	for _, tool := range []string{"zeta", "alpha", "mid"} {
		tk, ref := g.Admit(context.Background(), tenancy.Request{Tool: tool, Class: tenancy.ClassTree, Origin: tenancy.OriginAgent})
		if ref != nil {
			t.Fatalf("admit %s: %+v", tool, ref)
		}
		tks = append(tks, tk)
	}
	defer func() {
		for _, tk := range tks {
			tk.Release(tenancy.OutcomeOK)
		}
	}()
	rec := httptest.NewRecorder()
	writeBusy(rec, json.RawMessage("1"), "test", tenancy.ClassTree.String(), "", time.Second)
	var body struct {
		Error struct {
			Data map[string]any `json:"data"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, v := range body.Error.Data["in_flight"].([]any) {
		got = append(got, v.(string))
	}
	if !sort.StringsAreSorted(got) || len(got) != 3 {
		t.Fatalf("in_flight %v, want the three tools sorted", got)
	}
}
