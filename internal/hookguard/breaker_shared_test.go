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
	"testing"
	"time"
)

// A hung daemon whose calls end on the CALLER's http.Client.Timeout —
// shorter than the hook deadline, as edit-hook --pre's 2 s client is under
// its 4 s deadline — must count as failures and open the breaker. Wrapping
// http.DefaultTransport sends Client.Timeout down net/http's fallback
// cancel path, which cancels the request context (context.Canceled), so a
// classifier that trusted only DeadlineExceeded missed it about half the
// time. Every trial must open.
func TestClientTimeoutCountsAsFailure(t *testing.T) {
	rel := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-rel:
		}
	}))
	defer srv.Close()
	defer close(rel)
	for trial := 0; trial < 6; trial++ {
		g := &Guard{HealthDir: filepath.Join(t.TempDir(), "h"), DaemonDir: t.TempDir(), Deadline: HookDeadline(time.Now(), 5)}
		hc := &http.Client{Timeout: 60 * time.Millisecond, Transport: &Transport{Base: http.DefaultTransport, Active: func() *Guard { return g }}}
		for i := 0; i < 2; i++ {
			if resp, err := hc.Get(srv.URL + "/rpc"); err == nil {
				resp.Body.Close()
				t.Fatal("hung daemon answered")
			}
		}
		if d := BreakerAllows(g, srv.URL); d.Verdict != Open {
			t.Fatalf("trial %d: two client timeouts against a hung daemon left the breaker %v (state %+v)", trial, d.Verdict, g.readState(srv.URL))
		}
	}
}

// A caller that cancels its own request before any deadline is not the
// daemon's fault and must not count.
func TestCallerCancelIsNotAFailure(t *testing.T) {
	rel := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-rel:
		}
	}))
	defer srv.Close()
	defer close(rel)
	g := &Guard{HealthDir: filepath.Join(t.TempDir(), "h"), DaemonDir: t.TempDir(), Deadline: HookDeadline(time.Now(), 5)}
	hc := &http.Client{Transport: &Transport{Base: http.DefaultTransport, Active: func() *Guard { return g }}}
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(30*time.Millisecond, cancel)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/rpc", nil)
		if resp, err := hc.Do(req); err == nil {
			resp.Body.Close()
		}
		cancel()
	}
	if s := g.readState(srv.URL); len(s.Failures) != 0 || s.State != "closed" {
		t.Fatalf("caller cancellations were counted: %+v", s)
	}
}

// The incident shape: /healthz is cheap and answers while real RPCs hang.
// A good half-open probe must not reset the backoff ladder — only a real
// RPC success does — or the breaker reopens at 5 s forever and two hooks
// eat their whole deadline every cycle.
func TestBackoffClimbsWhenHealthzAnswersButRPCsFail(t *testing.T) {
	clk := &fakeClock{now: time.Now()}
	g := newTestGuard(t, clk)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer srv.Close()
	var got []time.Duration
	for cycle := 0; cycle < 4; cycle++ {
		g.RecordFailure(srv.URL, errors.New("timeout"))
		g.RecordFailure(srv.URL, errors.New("timeout"))
		s := g.readState(srv.URL)
		got = append(got, s.Until.Sub(clk.Now()))
		clk.now = s.Until.Add(time.Millisecond)
		if err := g.halfOpen(http.DefaultTransport, srv.URL, t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	want := []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("backoffs %v, want %v", got, want)
		}
	}
	// A real RPC success resets the ladder.
	g.RecordSuccessSince(srv.URL, clk.Now())
	g.RecordFailure(srv.URL, errors.New("timeout"))
	g.RecordFailure(srv.URL, errors.New("timeout"))
	if s := g.readState(srv.URL); s.Until.Sub(clk.Now()) != 5*time.Second {
		t.Fatalf("after a real success the ladder restarts at 5 s, got %v", s.Until.Sub(clk.Now()))
	}
}

// Two hooks failing at the same moment each count: the read-modify-write
// of the shared file is serialized, so "2 failures in 10 s" holds across
// processes (each goroutine shares only the files).
func TestConcurrentFailuresOpenBreaker(t *testing.T) {
	lost := 0
	for trial := 0; trial < 200; trial++ {
		dir := filepath.Join(t.TempDir(), "h")
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				g := &Guard{HealthDir: dir, DaemonDir: t.TempDir()}
				<-start
				g.RecordFailure(testURL, errors.New("timeout"))
			}()
		}
		close(start)
		wg.Wait()
		if BreakerAllows(&Guard{HealthDir: dir, DaemonDir: t.TempDir()}, testURL).Verdict != Open {
			lost++
		}
	}
	if lost > 0 {
		t.Fatalf("%d/200 trials: two concurrent failures did not open the breaker (lost update)", lost)
	}
}

// An RPC that started before the breaker opened and answers late must not
// close the fresher "open" (and reset the ladder) other hooks just wrote.
func TestInFlightSuccessDoesNotCloseFresherOpen(t *testing.T) {
	clk := &fakeClock{now: time.Now()}
	g := newTestGuard(t, clk)
	started := clk.Now()
	clk.Add(time.Second)
	g.RecordFailure(testURL, errors.New("timeout"))
	g.RecordFailure(testURL, errors.New("timeout"))
	clk.Add(100 * time.Millisecond)
	g.RecordSuccessSince(testURL, started)
	if d := BreakerAllows(g, testURL); d.Verdict != Open {
		t.Fatalf("a late success from before the open closed it: %v", d.Verdict)
	}
	// A success that started after the open does close it.
	g.RecordSuccessSince(testURL, clk.Now())
	if d := BreakerAllows(g, testURL); d.Verdict != Allow {
		t.Fatalf("a fresh success: %v, want allow", d.Verdict)
	}
}

// A leftover lease file (its holder died mid-probe) must admit exactly one
// new prober even when many hooks find it at once — the old "stat, remove
// if stale, O_EXCL again" let a second breaker delete the lease the first
// had just re-created, and both probed.
func TestLeftoverLeaseAdmitsOneProber(t *testing.T) {
	for trial := 0; trial < 50; trial++ {
		root := t.TempDir()
		mk := func() *Guard {
			return &Guard{HealthDir: filepath.Join(root, "health"), DaemonDir: filepath.Join(root, "daemon")}
		}
		lp := mk().leasePath(testURL)
		if err := os.MkdirAll(filepath.Dir(lp), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(lp, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-time.Minute)
		if err := os.Chtimes(lp, old, old); err != nil {
			t.Fatal(err)
		}
		const n = 32
		var held atomic.Int32
		var wg, tried sync.WaitGroup
		start := make(chan struct{})
		tried.Add(n)
		releases := make(chan func(), n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				release, ok := mk().acquireLease(testURL)
				if ok {
					held.Add(1)
					releases <- release
				}
				tried.Done()
			}()
		}
		close(start)
		tried.Wait()
		wg.Wait()
		close(releases)
		for r := range releases {
			r()
		}
		if got := held.Load(); got != 1 {
			t.Fatalf("trial %d: %d hooks held the probe lease at once, want exactly 1", trial, got)
		}
	}
}
