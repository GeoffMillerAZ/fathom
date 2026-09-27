package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// RT-123 (design §13.5): pprof stays on TCP behind the loopback Host
// check, `seconds` is capped at 30, one profile runs at a time, and
// /debug/inflight lists the in-flight table beside it.
func TestPprofLoopbackCappedSingleProfile(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan string, 4)
	stub := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- r.URL.Query().Get("seconds")
		<-release
		w.WriteHeader(http.StatusOK)
	})
	h := guardPprof(stub)

	// A foreign Host is refused before any profile runs.
	req := loopbackReq(http.MethodGet, "http://evil.example:7581/debug/pprof/profile", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-loopback Host: %d, want 403", rec.Code)
	}

	// seconds=600 is capped to 30; while it runs, a second profile is refused 429.
	done := make(chan int, 1)
	go func() {
		r := loopbackReq(http.MethodGet, "http://127.0.0.1:7581/debug/pprof/profile?seconds=600", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		done <- w.Code
	}()
	if got := <-entered; got != "30" {
		t.Fatalf("seconds reached the profiler as %q, want 30", got)
	}
	r2 := loopbackReq(http.MethodGet, "http://localhost:7581/debug/pprof/heap", nil)
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, r2)
	if w2.Code != http.StatusTooManyRequests || !strings.Contains(w2.Body.String(), "one profile at a time") {
		t.Fatalf("a concurrent profile: %d %q, want 429 naming one profile at a time", w2.Code, w2.Body.String())
	}
	close(release)
	if c := <-done; c != http.StatusOK {
		t.Fatalf("the first profile: %d", c)
	}
	// Once it finished, the next profile runs.
	w3 := httptest.NewRecorder()
	h.ServeHTTP(w3, loopbackReq(http.MethodGet, "http://[::1]:7581/debug/pprof/heap?seconds=5", nil))
	if w3.Code != http.StatusOK || <-entered != "5" {
		t.Fatalf("the next profile after release: %d", w3.Code)
	}

	// The real mux: the routes are mounted guarded.
	mux := http.NewServeMux()
	mountPprof(mux)
	w4 := httptest.NewRecorder()
	mux.ServeHTTP(w4, loopbackReq(http.MethodGet, "http://evil.example/debug/pprof/goroutine", nil))
	if w4.Code != http.StatusForbidden {
		t.Fatalf("mounted pprof accepts a foreign Host: %d", w4.Code)
	}

	// /debug/inflight: the in-flight table, oldest first, loopback only.
	now := time.Date(2026, 9, 27, 3, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	inf := tenancy.NewInFlight(clock)
	_, d1 := inf.Begin(context.Background(), tenancy.ClassTree, "references", "svc", "agent")
	defer d1()
	now = now.Add(3 * time.Second)
	ih := inflightHandler(inf, clock)
	w5 := httptest.NewRecorder()
	ih.ServeHTTP(w5, loopbackReq(http.MethodGet, "http://127.0.0.1/debug/inflight", nil))
	var got struct {
		InFlight []inflightView `json:"in_flight"`
	}
	if err := json.Unmarshal(w5.Body.Bytes(), &got); err != nil || len(got.InFlight) != 1 || got.InFlight[0].Tool != "references" || got.InFlight[0].AgeSec != 3 {
		t.Fatalf("/debug/inflight: %v %s", err, w5.Body.String())
	}
	w6 := httptest.NewRecorder()
	ih.ServeHTTP(w6, loopbackReq(http.MethodGet, "http://evil.example/debug/inflight", nil))
	if w6.Code != http.StatusForbidden {
		t.Fatalf("/debug/inflight accepts a foreign Host: %d", w6.Code)
	}
}

// A hosted daemon drops the server-wide Host guard, and Host is what the
// client says: pprof and /debug/inflight also require a loopback peer, so
// a remote caller spoofing Host: 127.0.0.1 is refused.
// loopbackReq is httptest.NewRequest from a loopback peer (httptest's
// default RemoteAddr, 192.0.2.1, is refused like any remote peer).
func loopbackReq(method, target string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, target, body)
	r.RemoteAddr = "127.0.0.1:50000"
	return r
}

func TestPprofRefusesRemotePeerSpoofingLoopbackHost(t *testing.T) {
	mux := http.NewServeMux()
	mountPprof(mux)
	for _, path := range []string{"/debug/pprof/cmdline", "/debug/pprof/", "/debug/inflight"} {
		req := loopbackReq(http.MethodGet, "http://127.0.0.1:7581"+path, nil)
		req.RemoteAddr = "203.0.113.9:4444"
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("GET %s from a remote peer with a loopback Host: %d, want 403", path, rec.Code)
		}
		req = loopbackReq(http.MethodGet, "http://127.0.0.1:7581"+path, nil)
		req.RemoteAddr = "127.0.0.1:50000"
		rec = httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s from a loopback peer: %d, want 200", path, rec.Code)
		}
	}
}
