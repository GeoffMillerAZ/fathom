package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/scope"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// TestOriginHeaderCarriesCallerTestFlag (RT-164): every hook/CLI RPC built
// by originHeader carries X-Fathom-Caller, marked test=1 under test.
func TestOriginHeaderCarriesCallerTestFlag(t *testing.T) {
	req, err := originHeader("http://127.0.0.1:1", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	c, ok := client.ParseCallerHeader(req.Header.Get(client.CallerHeaderName))
	if !ok || !c.Test {
		t.Fatalf("X-Fathom-Caller = %q, want a parsed header with test=1", req.Header.Get(client.CallerHeaderName))
	}
}

// TestEveryDaemonRequestCarriesCaller (RT-164 review, §10.4 "every client
// sends X-Fathom-Caller"): the header was set only by originHeader, so
// the session primer, the PreToolUse /refresh-file, /route, /metrics,
// /statusz, /findings/summary and the custodian's stanza read went out
// unmarked. The binary's process setup now installs a transport that
// stamps every loopback (daemon) request that lacks it; a non-daemon host
// never receives it.
func TestEveryDaemonRequestCarriesCaller(t *testing.T) {
	t.Setenv(scope.EnvVar, filepath.Join(t.TempDir(), "suppressions.json"))
	saved := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = saved })
	prepareProcess()

	var mu sync.Mutex
	seen := map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen[r.URL.Path] = r.Header.Get(client.CallerHeaderName)
		mu.Unlock()
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	hc := &http.Client{Timeout: 5 * time.Second}

	_ = liveExampleLine(hc, srv.URL, "svc")
	_ = ensureFileFresh(srv.URL, "svc", "a.go")
	_, _ = fetchMetricsText(hc, srv.URL)
	_, _ = fetchStatusz(hc, srv.URL)
	_ = findingsBriefLine(hc, srv.URL, "svc")

	mu.Lock()
	defer mu.Unlock()
	for _, path := range []string{"/rpc/agent", "/refresh-file", "/metrics", "/statusz", "/findings/summary"} {
		h, ok := seen[path]
		if !ok {
			t.Errorf("%s: no request reached the fake daemon", path)
			continue
		}
		c, parsed := client.ParseCallerHeader(h)
		if !parsed || !c.Test || c.PID != os.Getpid() {
			t.Errorf("%s: X-Fathom-Caller = %q, want this process with test=1", path, h)
		}
	}

	// Only daemon hosts: an outbound request elsewhere is left alone.
	for host, want := range map[string]bool{"127.0.0.1:7575": true, "localhost:7575": true, "[::1]:7575": true, "api.example.com": false} {
		if got := client.IsDaemonHost(host, ""); got != want {
			t.Errorf("IsDaemonHost(%q) = %v, want %v", host, got, want)
		}
	}
	if !client.IsDaemonHost("fathom.internal:7575", "http://fathom.internal:7575") {
		t.Error("the FATHOM_HTTP host is a daemon host")
	}
}
