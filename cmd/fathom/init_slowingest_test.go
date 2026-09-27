package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A first ingest of a large repository legitimately outlasts init's
// request: wave D runs the type-checker children niced at -p=2, and the
// daemon's RPC write deadline is 150 s. The ingest itself carries on
// detached (RT-149), so a request that times out is not "daemon down" —
// init waits for the service to read as indexed. Found by the stranger
// check on 2026-09-27 ("fathom init exited 2" on a fathom-sized tree).
func TestInitIngest_SlowFirstIngestWaitsForIndexed(t *testing.T) {
	var polls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/indexed"):
			n := polls.Add(1)
			_, _ = io.WriteString(w, map[bool]string{true: `{"indexed":true}`, false: `{"indexed":false}`}[n >= 2])
		case r.URL.Path == "/rpc":
			time.Sleep(400 * time.Millisecond) // outlasts the request timeout below
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{}"}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	var out strings.Builder
	ok, err := initIngestWith(ts.URL, "svc", "/repo", &out, initIngestTiming{request: 100 * time.Millisecond, pollEvery: 20 * time.Millisecond, pollFor: 5 * time.Second})
	if err != nil {
		t.Fatalf("a slow ingest read as daemon-down: %v\n%s", err, out.String())
	}
	if !ok {
		t.Fatalf("init gave up before the service read as indexed:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "still running") {
		t.Errorf("init did not say it was waiting:\n%s", out.String())
	}
}

// A daemon that is not listening is still daemon-down.
func TestInitIngest_RefusedConnectionIsDaemonDown(t *testing.T) {
	ts := httptest.NewServer(http.NotFoundHandler())
	url := ts.URL
	ts.Close()
	var out strings.Builder
	if _, err := initIngestWith(url, "svc", "/repo", &out, initIngestTiming{request: time.Second, pollEvery: 10 * time.Millisecond, pollFor: 100 * time.Millisecond}); err == nil {
		t.Fatalf("a refused connection must be reported as daemon-down:\n%s", out.String())
	}
}
