package main

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/pkg/client"
)

// O1151 gap: hand-built hook requests (/identity, /indexed, /refresh-request,
// the session live-example) go through CallerTransport, not originHeader, and
// must name the same rpcSession. Plain CLI use (no session) sends none.
func TestHandBuiltRequests_CarrySessionViaCallerTransport(t *testing.T) {
	oldT, oldS, oldO := http.DefaultTransport, rpcSession, rpcOrigin
	t.Cleanup(func() { http.DefaultTransport, rpcSession, rpcOrigin = oldT, oldS, oldO })

	var mu sync.Mutex
	seen := map[string]string{} // path -> session=
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, ok := client.ParseCallerHeader(r.Header.Get(client.CallerHeaderName)); ok {
			mu.Lock()
			seen[r.URL.Path] = c.Session
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	t.Setenv("FATHOM_HTTP", srv.URL)

	http.DefaultTransport = &http.Transport{}
	rpcOrigin = "session-hook"
	installCallerTransport()

	call := func() {
		hc := &http.Client{Timeout: 2 * time.Second}
		lookupIdentity(hc, srv.URL, t.TempDir())
		probeIndexed(hc, srv.URL, "svc")
		enqueueViaDaemon(srv.URL, "svc", time.Second)
		liveExampleLine(hc, srv.URL, "svc")
	}
	paths := []string{"/identity", "/indexed", "/refresh-request", "/rpc/agent"}

	rpcSession = "sess-hook-1"
	call()
	mu.Lock()
	for _, p := range paths {
		if got, ok := seen[p]; !ok || got != "sess-hook-1" {
			t.Errorf("%s: session = %q (seen %v), want sess-hook-1", p, got, ok)
		}
	}
	seen = map[string]string{}
	mu.Unlock()

	rpcSession = ""
	call()
	mu.Lock()
	defer mu.Unlock()
	for _, p := range paths {
		if got := seen[p]; got != "" {
			t.Errorf("%s: plain CLI session = %q, want empty", p, got)
		}
	}
}
