package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/hookguard"
)

// O1449 (1): the edit-time refresh's 400 ms client timeouts open the shared
// breaker; the post-commit /refresh-request is queued (202) and nothing waits
// on it, so the open breaker must not skip it.
func TestPostCommitRefreshBypassesTheEditBreaker(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path != "/refresh-request" {
			http.Error(w, "unexpected", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"service":"svc","state":"queued"}`))
	}))
	defer srv.Close()

	g := &hookguard.Guard{HealthDir: filepath.Join(t.TempDir(), "h"), DaemonDir: t.TempDir(), Deadline: hookguard.HookDeadline(time.Now(), 5)}
	g.RecordFailure(srv.URL, errors.New("request canceled while awaiting headers"))
	g.RecordFailure(srv.URL, errors.New("request canceled while awaiting headers"))
	if d := hookguard.BreakerAllows(g, srv.URL); d.Verdict != hookguard.Open {
		t.Fatalf("setup: breaker %v, want open", d.Verdict)
	}
	restore := activateHookGuard(g)
	defer restore()

	what, handled, err := enqueueViaDaemon(srv.URL, "svc", 2*time.Second)
	if err != nil || !handled || what == "" {
		t.Fatalf("post-commit refresh under an open breaker: what=%q handled=%v err=%v", what, handled, err)
	}
	if hits != 1 {
		t.Fatalf("daemon saw %d requests, want 1", hits)
	}
	// Other hook RPCs stay shed, and the bypass neither closed nor deepened the breaker.
	if d := hookguard.BreakerAllows(g, srv.URL); d.Verdict != hookguard.Open {
		t.Fatalf("breaker after the bypass: %v, want still open", d.Verdict)
	}
	resp, err := http.Get(srv.URL + "/rpc")
	if err == nil {
		resp.Body.Close()
		t.Fatal("an edit-time RPC passed the open breaker")
	}
}
