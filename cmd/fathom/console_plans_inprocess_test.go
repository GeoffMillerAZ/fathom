package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// TestConsolePlansVerifyInProcessNoRequeue (RT-116, design §14): inside the
// daemon the plans portfolio verifies a plan through the verify_plan handler
// in process — no loopback request — under a deadline, and a plan whose key
// moves away and back while its first verification still runs rejoins that
// verification instead of queueing the key again.
func TestConsolePlansVerifyInProcessNoRequeue(t *testing.T) {
	f := newConsoleFixture(t)
	var loopbacks atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loopbacks.Add(1)
		http.Error(w, "loopback", http.StatusTeapot)
	}))
	t.Cleanup(ts.Close)
	srv := mcp.NewServerWithFleetAndLogger(f.client, newFleetAdapter(f.client, testDiscardLogger()), nil)
	var calls atomic.Int32
	prev := inProcessReg.Load()
	inProcessReg.Store(&inProcessRegistry{
		self:    ts.URL,
		statusz: func(context.Context) StatuszReport { return StatuszReport{} },
		rpc: func(ctx context.Context, raw []byte) ([]byte, bool) {
			calls.Add(1)
			return srv.HandleRequest(ctx, raw)
		},
	})
	t.Cleanup(func() { inProcessReg.Store(prev) })

	root := t.TempDir()
	plansDir := filepath.Join(root, "docs", "plans")
	mustWrite(t, filepath.Join(root, "README.md"), "# Runbook\n\n## Window\n")
	planPath := filepath.Join(plansDir, "window.md")
	planA := "# Window\n\n- [x] The runbook names the window {id: W-1, oracle: lexical, path: README.md, pattern: \"^## Window$\"}\n"
	planB := planA + "- [ ] Another row {id: W-2}\n"
	mustWrite(t, planPath, planA)

	deps := newConsoleDeps(f.client, ts.URL, f.now)
	p := plansFor(deps)
	sr := serviceRoot{Service: "proof-service", Root: root}
	gate := make(chan struct{})
	setConsolePlansGate(gate)
	t.Cleanup(func() { setConsolePlansGate(nil) })

	_, first := p.plan(context.Background(), sr, planPath)
	if first == nil {
		t.Fatal("the plan was not submitted")
	}
	waitUntil(t, func() bool { return deps.jobPool().Stats().Running == 1 })
	mustWrite(t, planPath, planB) // the key moves away…
	_, second := p.plan(context.Background(), sr, planPath)
	waitUntil(t, func() bool { return deps.jobPool().Stats().Running == 2 })
	mustWrite(t, planPath, planA) // …and back while the first still runs
	_, again := p.plan(context.Background(), sr, planPath)
	if again != first {
		t.Fatal("a key still being verified was queued again; it must rejoin its flight")
	}
	if st := deps.jobPool().Stats(); st.Running+st.Queued != 2 {
		t.Fatalf("pool = %+v; want the two flights only", st)
	}
	close(gate)
	for _, e := range []*planEntry{first, second} {
		select {
		case <-e.done:
		case <-time.After(30 * time.Second):
			t.Fatal("a verification never finished")
		}
	}
	if first.err != "" {
		t.Fatalf("in-process verification failed: %s", first.err)
	}
	if calls.Load() == 0 {
		t.Error("verify_plan was not called in process")
	}
	if len(first.result.Report.Tasks) != 1 || first.result.Report.Tasks[0].TaskID != "W-1" {
		t.Errorf("verdicts = %+v", first.result.Report.Tasks)
	}
	if n := loopbacks.Load(); n != 0 {
		t.Errorf("%d loopback request(s) to the daemon's own URL", n)
	}

	// Under a deadline: a verify_plan that does not answer is cut off and
	// named, never waited on.
	inProcessReg.Store(&inProcessRegistry{
		self:    ts.URL,
		statusz: func(context.Context) StatuszReport { return StatuszReport{} },
		rpc: func(ctx context.Context, raw []byte) ([]byte, bool) {
			<-ctx.Done()
			return nil, false
		},
	})
	prevDeadline := consolePlanVerifyDeadline
	consolePlanVerifyDeadline = 100 * time.Millisecond
	t.Cleanup(func() { consolePlanVerifyDeadline = prevDeadline })
	raw, _ := os.ReadFile(planPath)
	start := time.Now()
	_, err := verifyPlanInProcess(deps, "proof-service", raw, parsePlanForVerify("proof-service", root, planPath, raw))
	if err == nil || !strings.Contains(err.Error(), "deadline") || time.Since(start) > 5*time.Second {
		t.Errorf("a hung verify_plan = %v after %s; want the deadline named", err, time.Since(start))
	}
}

// TestConsolePlanVerifyPanicFinishes (RT-116, RT-117): a plan verification
// that panics finishes as a failure the page names — its waiters are
// released and its key leaves the flights — never a key stuck "verifying"
// that every later render rejoins.
func TestConsolePlanVerifyPanicFinishes(t *testing.T) {
	f := newConsoleFixture(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "loopback", http.StatusTeapot)
	}))
	t.Cleanup(ts.Close)
	prev := inProcessReg.Load()
	inProcessReg.Store(&inProcessRegistry{
		self:    ts.URL,
		statusz: func(context.Context) StatuszReport { return StatuszReport{} },
		rpc:     func(context.Context, []byte) ([]byte, bool) { panic("the handler broke") },
	})
	t.Cleanup(func() { inProcessReg.Store(prev) })
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "README.md"), "# Runbook\n")
	planPath := filepath.Join(root, "docs", "plans", "p.md")
	mustWrite(t, planPath, "# P\n\n- [x] A row {id: P-1, symbol: Nothing}\n")
	deps := newConsoleDeps(f.client, ts.URL, f.now)
	p := plansFor(deps)
	_, e := p.plan(context.Background(), serviceRoot{Service: "proof-service", Root: root}, planPath)
	if e == nil {
		t.Fatal("the plan was not submitted")
	}
	select {
	case <-e.done:
	case <-time.After(10 * time.Second):
		t.Fatal("a panicking verification never released its waiters")
	}
	p.mu.Lock()
	finished, errText, flights := e.finished, e.err, len(p.flights)
	p.mu.Unlock()
	if !finished || !strings.Contains(errText, "panicked") || flights != 0 {
		t.Fatalf("finished=%v err=%q flights=%d; want a finished failure naming the panic and no flight left", finished, errText, flights)
	}
}

// TestLiveDaemonStaleReadsOwnBuildInProcess (RT-115): inside the daemon,
// the plans page's measured-stale reading is this process's own build
// stamp — never a full-fleet statusz (every service's meta, typed-edge
// status and compile runs) per service per headTTL.
func TestLiveDaemonStaleReadsOwnBuildInProcess(t *testing.T) {
	prev := inProcessReg.Load()
	t.Cleanup(func() { inProcessReg.Store(prev) })
	var built atomic.Int32
	self := "http://127.0.0.1:1/"
	inProcessReg.Store(&inProcessRegistry{self: self,
		services: func(context.Context) (map[string]svcInfo, error) { return nil, nil },
		statusz: func(context.Context) StatuszReport {
			built.Add(1)
			return StatuszReport{}
		}})
	liveDaemonStale(nil, self, t.TempDir())
	if n := built.Load(); n != 0 {
		t.Fatalf("the measured-stale reading built the full statusz %d time(s); want the daemon's own build stamp", n)
	}
}
