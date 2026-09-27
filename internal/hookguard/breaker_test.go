package hookguard

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// fakeClock is a settable wall clock shared by every guard in a test —
// the breaker file is cross-process state keyed on wall time.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *fakeClock) Add(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

func newTestGuard(t *testing.T, clk *fakeClock) *Guard {
	t.Helper()
	root := t.TempDir()
	return &Guard{
		HealthDir: filepath.Join(root, "health"),
		DaemonDir: filepath.Join(root, "daemon"),
		Now:       clk.Now,
	}
}

const testURL = "http://127.0.0.1:7599"

func TestBreakerOpensOnTwoFailuresWithinTenSeconds(t *testing.T) {
	clk := &fakeClock{now: time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)}
	g := newTestGuard(t, clk)

	if d := BreakerAllows(g, testURL); d.Verdict != Allow {
		t.Fatalf("fresh breaker: %v, want allow", d.Verdict)
	}
	g.RecordFailure(testURL, errors.New("timeout"))
	if d := BreakerAllows(g, testURL); d.Verdict != Allow {
		t.Fatalf("one failure must not open (one timeout alone would blank every session): %v", d.Verdict)
	}
	// A second failure 11 s later is outside the window: still closed.
	clk.Add(11 * time.Second)
	g.RecordFailure(testURL, errors.New("refused"))
	if d := BreakerAllows(g, testURL); d.Verdict != Allow {
		t.Fatalf("failures 11 s apart must not open: %v", d.Verdict)
	}
	clk.Add(3 * time.Second)
	g.RecordFailure(testURL, errors.New("refused"))
	d := BreakerAllows(g, testURL)
	if d.Verdict != Open {
		t.Fatalf("two failures within 10 s: %v, want open", d.Verdict)
	}
	if want := clk.Now().Add(5 * time.Second); !d.Until.Equal(want) {
		t.Fatalf("first open until %v, want +5s %v", d.Until, want)
	}
	// Another URL is untouched: the breaker is per URL.
	if d := BreakerAllows(g, "http://127.0.0.1:7598"); d.Verdict != Allow {
		t.Fatalf("other URL: %v, want allow", d.Verdict)
	}
	// A success closes and resets.
	g.RecordSuccess(testURL)
	if d := BreakerAllows(g, testURL); d.Verdict != Allow {
		t.Fatalf("after success: %v, want allow", d.Verdict)
	}
}

func TestBreakerBackoffDoublesToCap(t *testing.T) {
	clk := &fakeClock{now: time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)}
	g := newTestGuard(t, clk)
	g.RecordFailure(testURL, errors.New("x"))
	g.RecordFailure(testURL, errors.New("x"))
	want := []time.Duration{10, 20, 40, 80, 160, 300, 300}
	for i, w := range want {
		d := BreakerAllows(g, testURL)
		if d.Verdict != Open {
			t.Fatalf("step %d: %v, want open", i, d.Verdict)
		}
		clk.now = d.Until // lapse: half-open
		if d := BreakerAllows(g, testURL); d.Verdict != HalfOpen {
			t.Fatalf("step %d: lapsed breaker %v, want half-open", i, d.Verdict)
		}
		g.RecordProbeFailure(testURL, errors.New("probe failed"))
		d = BreakerAllows(g, testURL)
		if got := d.Until.Sub(clk.Now()); got != w*time.Second {
			t.Fatalf("step %d: backoff %v, want %v", i, got, w*time.Second)
		}
	}
}

func TestBreakerBusyOn503RetryAfter(t *testing.T) {
	clk := &fakeClock{now: time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)}
	g := newTestGuard(t, clk)
	g.RecordBusy(testURL, 12*time.Second)
	d := BreakerAllows(g, testURL)
	if d.Verdict != Busy || !d.Until.Equal(clk.Now().Add(12*time.Second)) {
		t.Fatalf("503+Retry-After: %v until %v, want busy +12s", d.Verdict, d.Until)
	}
	// Retry-After is capped at 60 s.
	g.RecordBusy(testURL, 10*time.Minute)
	if d := BreakerAllows(g, testURL); !d.Until.Equal(clk.Now().Add(60 * time.Second)) {
		t.Fatalf("Retry-After cap: until %v", d.Until)
	}
	// Busy is not a failure: after it lapses the breaker is closed, and the
	// backoff ladder has not advanced.
	clk.Add(61 * time.Second)
	if d := BreakerAllows(g, testURL); d.Verdict != Allow {
		t.Fatalf("lapsed busy: %v, want allow", d.Verdict)
	}
}

func TestBreakerFailsOpenOnCorruptState(t *testing.T) {
	clk := &fakeClock{now: time.Now()}
	g := newTestGuard(t, clk)
	if err := os.MkdirAll(g.HealthDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(g.statePath(testURL), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if d := BreakerAllows(g, testURL); d.Verdict != Allow {
		t.Fatalf("corrupt state: %v, want allow (fail open)", d.Verdict)
	}
}

// TestHookBreakerSingleProber is the §11.2 proof: when an open breaker
// lapses, 32 concurrent hooks race for the half-open slot and exactly one
// probes /healthz; every other hook fails open at once. Each goroutine
// builds its own Guard and shares nothing in memory — only the breaker and
// lease files, exactly as 32 hook processes would.
func TestHookBreakerSingleProber(t *testing.T) {
	var probes, calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			probes.Add(1)
			time.Sleep(50 * time.Millisecond) // overlap the racers
			w.WriteHeader(http.StatusOK)
			return
		}
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	root := t.TempDir()
	mk := func() *Guard {
		return &Guard{HealthDir: filepath.Join(root, "health"), DaemonDir: filepath.Join(root, "daemon")}
	}
	// Open the breaker, then let it lapse.
	g0 := mk()
	past := time.Now().Add(-time.Minute)
	g0.Now = func() time.Time { return past }
	g0.RecordFailure(srv.URL, errors.New("timeout"))
	g0.RecordFailure(srv.URL, errors.New("timeout"))
	if d := BreakerAllows(mk(), srv.URL); d.Verdict != HalfOpen {
		t.Fatalf("lapsed breaker: %v, want half-open", d.Verdict)
	}

	const n = 32
	var wg sync.WaitGroup
	var skipped atomic.Int32
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g := mk()
			hc := &http.Client{Transport: &Transport{Base: http.DefaultTransport, Active: func() *Guard { return g }}}
			<-start
			resp, err := hc.Get(srv.URL + "/rpc")
			if err != nil {
				var se *SkipError
				if errors.As(err, &se) {
					skipped.Add(1)
				}
				return
			}
			resp.Body.Close()
		}()
	}
	close(start)
	wg.Wait()

	if got := probes.Load(); got != 1 {
		t.Fatalf("probes = %d, want exactly 1 (one half-open prober under the lease)", got)
	}
	if skipped.Load() == 0 {
		t.Fatalf("no hook failed open while the prober held the lease")
	}
	if skipped.Load()+calls.Load() != n {
		t.Fatalf("skipped %d + served %d != %d", skipped.Load(), calls.Load(), n)
	}
	// The probe succeeded, so the breaker closed.
	if d := BreakerAllows(mk(), srv.URL); d.Verdict != Allow {
		t.Fatalf("after a good probe: %v, want allow", d.Verdict)
	}
	release, ok := mk().acquireLease(srv.URL)
	if !ok {
		t.Fatal("probe lease not released")
	}
	release()
}

func TestLeftoverLeaseFileDoesNotBlock(t *testing.T) {
	clk := &fakeClock{now: time.Now()}
	g := newTestGuard(t, clk)
	if err := os.MkdirAll(g.HealthDir, 0o755); err != nil {
		t.Fatal(err)
	}
	lp := g.leasePath(testURL)
	if err := os.WriteFile(lp, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-10 * time.Second)
	if err := os.Chtimes(lp, old, old); err != nil {
		t.Fatal(err)
	}
	release, ok := g.acquireLease(testURL)
	if !ok {
		t.Fatal("a lease file its (dead) holder left behind must not block the next prober")
	}
	release()
}

func TestTransportClassifiesOutcomes(t *testing.T) {
	var status atomic.Int32
	status.Store(200)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get(DeadlineHeader); got == "" {
			t.Errorf("request without %s", DeadlineHeader)
		}
		s := int(status.Load())
		if s == 503 {
			w.Header().Set("Retry-After", "7")
		}
		w.WriteHeader(s)
	}))
	defer srv.Close()
	g := &Guard{HealthDir: filepath.Join(t.TempDir(), "h"), DaemonDir: t.TempDir(), Deadline: HookDeadline(time.Now(), 5)}
	hc := &http.Client{Transport: &Transport{Base: http.DefaultTransport, Active: func() *Guard { return g }}}
	get := func() error {
		resp, err := hc.Get(srv.URL + "/rpc")
		if err == nil {
			resp.Body.Close()
		}
		return err
	}
	status.Store(503)
	if err := get(); err != nil {
		t.Fatalf("503 is returned to the caller: %v", err)
	}
	if d := BreakerAllows(g, srv.URL); d.Verdict != Busy {
		t.Fatalf("after 503+Retry-After: %v, want busy", d.Verdict)
	}
	err := get()
	var se *SkipError
	if !errors.As(err, &se) || se.Verdict != Busy {
		t.Fatalf("busy breaker: err %v, want SkipError busy", err)
	}
}

func TestRetryableOnlyRefusedOrReset(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{syscall.ECONNREFUSED, true},
		{&os.SyscallError{Syscall: "connect", Err: syscall.ECONNREFUSED}, true},
		{syscall.ECONNRESET, true},
		{context.DeadlineExceeded, false},
		{errors.New("list_services: HTTP 500"), false},
		{&SkipError{Verdict: Open}, false},
		{nil, false},
	} {
		if got := Retryable(tc.err); got != tc.want {
			t.Errorf("Retryable(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
