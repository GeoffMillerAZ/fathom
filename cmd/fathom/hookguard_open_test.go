package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/hookguard"
)

// openBreakerFor opens the shared breaker for url the way two failing
// hooks would, and returns a daemon-hit counter's server.
func openBreakerFor(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	g0 := &hookguard.Guard{}
	g0.RecordFailure(srv.URL, errors.New("timeout"))
	g0.RecordFailure(srv.URL, errors.New("timeout"))
	if d := hookguard.BreakerAllows(&hookguard.Guard{}, srv.URL); d.Verdict != hookguard.Open {
		t.Fatalf("setup: breaker %v, want open", d.Verdict)
	}
	return srv, &hits
}

// §11.2: while the breaker is open hooks are silent (I9, P18). The session
// primer used to print "not responding … Restart it with …" to every new
// session the breaker was shielding the daemon from. The skip is recorded
// as a breaker_open beat, as the design names it.
func TestSessionContextSilentWhileBreakerOpen(t *testing.T) {
	isolateHookRun(t)
	beats := filepath.Join(t.TempDir(), "hooks.jsonl")
	t.Setenv("FATHOM_HOOKS_LOG", beats)
	srv, hits := openBreakerFor(t)

	end, silent := beginHookGuard("session-hook", "fathom session-hook", "", srv.URL)
	if silent {
		t.Fatal("an open breaker is not the kill switch")
	}
	got := buildSessionContext(srv.URL, t.TempDir())
	end()
	if got != "" {
		t.Fatalf("primer while the breaker is open = %q, want silent", got)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("%d RPCs reached a daemon behind an open breaker", n)
	}
	raw, err := os.ReadFile(beats)
	if err != nil {
		t.Fatalf("no beat recorded: %v", err)
	}
	if !strings.Contains(string(raw), `"breaker_open"`) {
		t.Fatalf("beat log lacks a breaker_open beat: %s", raw)
	}
}

// §11.2: while the breaker is open a hook serves the identity cache stale
// where it can (the DG-12 path: the guard's skip is an outright failure).
func TestIdentityServedStaleWhileBreakerOpen(t *testing.T) {
	isolateHookRun(t)
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))
	srv, hits := openBreakerFor(t)
	proj := t.TempDir()
	writeIdentCache(proj, srv.URL, serviceCandidate{name: "svc", root: proj}, true, time.Now().Add(-time.Hour))

	g := &hookguard.Guard{Deadline: hookguard.HookDeadline(time.Now(), 5)}
	restore := activateHookGuard(g)
	defer restore()
	cand, found, stale := resolveIndexedCandidateCached(&http.Client{Timeout: 2 * time.Second}, srv.URL, proj)
	if !found || !stale || cand.name != "svc" {
		t.Fatalf("identity behind an open breaker = %+v found=%v stale=%v, want the stale svc", cand, found, stale)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("%d RPCs reached a daemon behind an open breaker", n)
	}
}
