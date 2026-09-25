package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testOrigin = "http://127.0.0.1:18766"
	testHost   = "127.0.0.1:18766"
	testToken  = "test-control-token"
)

type recordingHandler struct {
	mu             sync.Mutex
	bodies         [][]byte
	formWasSet     []bool
	postFormWasSet []bool
}

func (h *recordingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	h.mu.Lock()
	h.bodies = append(h.bodies, body)
	h.formWasSet = append(h.formWasSet, r.Form != nil)
	h.postFormWasSet = append(h.postFormWasSet, r.PostForm != nil)
	h.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (h *recordingHandler) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.bodies)
}

func (h *recordingHandler) last() []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	return bytes.Clone(h.bodies[len(h.bodies)-1])
}

func (h *recordingHandler) lastFormsWereSet() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	last := len(h.formWasSet) - 1
	return h.formWasSet[last] || h.postFormWasSet[last]
}

func newFixture(t *testing.T, delay time.Duration) (*probeHandler, *recordingHandler) {
	t.Helper()
	downstream := &recordingHandler{}
	probe, err := newProbeHandler(downstream, testOrigin, testToken, delay)
	if err != nil {
		t.Fatal(err)
	}
	return probe, downstream
}

func controlRequest(method, path, token string) *http.Request {
	r := httptest.NewRequest(method, testOrigin+path, nil)
	r.Host = testHost
	r.RemoteAddr = "127.0.0.1:43123"
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	return r
}

func arm(t *testing.T, probe *probeHandler) {
	t.Helper()
	w := httptest.NewRecorder()
	probe.ServeHTTP(w, controlRequest(http.MethodPost, "/__probe/arm", testToken))
	if w.Code != http.StatusOK {
		t.Fatalf("arm status = %d, body = %q", w.Code, w.Body.String())
	}
}

func previewRequest(ctx context.Context, body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, testOrigin+"/theme-lab", strings.NewReader(body)).WithContext(ctx)
	r.Host = testHost
	r.RemoteAddr = "127.0.0.1:43124"
	r.Header.Set("Origin", testOrigin)
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func state(t *testing.T, probe *probeHandler) probeRecord {
	t.Helper()
	w := httptest.NewRecorder()
	probe.ServeHTTP(w, controlRequest(http.MethodGet, "/__probe/state", testToken))
	if w.Code != http.StatusOK {
		t.Fatalf("state status = %d", w.Code)
	}
	var got probeRecord
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func waitStarted(t *testing.T, probe *probeHandler) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		probe.mu.Lock()
		started := probe.record.Started
		probe.mu.Unlock()
		if started {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("preview did not enter delay")
}

func TestCanceledPreviewIsNotDelegatedAndRetryBodyIsUntouched(t *testing.T) {
	probe, downstream := newFixture(t, time.Second)
	arm(t, probe)
	body := "action=preview&specimen=forms&scheme=dark&viewport=wide&fonts.sans=system-sans"
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		probe.ServeHTTP(httptest.NewRecorder(), previewRequest(ctx, body))
		close(done)
	}()
	waitStarted(t, probe)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("canceled preview did not return")
	}
	got := state(t, probe)
	if !got.Canceled || got.Delegated || got.Live || got.BodySHA256 != hashBytes([]byte(body)) || downstream.count() != 0 {
		t.Fatalf("unexpected canceled state: %+v, downstream=%d", got, downstream.count())
	}

	probe.ServeHTTP(httptest.NewRecorder(), previewRequest(context.Background(), body))
	if downstream.count() != 1 || !bytes.Equal(downstream.last(), []byte(body)) || downstream.lastFormsWereSet() {
		t.Fatalf("retry was not delegated byte-for-byte: %q", downstream.last())
	}
}

func TestIneligibleRequestsDelegateWithoutConsumingBody(t *testing.T) {
	oversize := "action=preview&padding=" + strings.Repeat("x", maxFormBytes)
	tests := []struct {
		name   string
		method string
		path   string
		origin string
		body   string
		mutate func(*http.Request)
	}{
		{name: "export", method: http.MethodPost, path: "/theme-lab", origin: testOrigin, body: "action=export"},
		{name: "get", method: http.MethodGet, path: "/theme-lab", origin: testOrigin, body: "action=preview"},
		{name: "wrong origin", method: http.MethodPost, path: "/theme-lab", origin: "http://127.0.0.1:9", body: "action=preview"},
		{name: "oversize", method: http.MethodPost, path: "/theme-lab", origin: testOrigin, body: oversize},
		{name: "malformed form", method: http.MethodPost, path: "/theme-lab", origin: testOrigin, body: "action=preview&bad=%ZZ"},
		{name: "duplicate action", method: http.MethodPost, path: "/theme-lab", origin: testOrigin, body: "action=preview&action=preview"},
		{name: "wrong host", method: http.MethodPost, path: "/theme-lab", origin: testOrigin, body: "action=preview", mutate: func(r *http.Request) { r.Host = "localhost:18766" }},
		{name: "wrong peer", method: http.MethodPost, path: "/theme-lab", origin: testOrigin, body: "action=preview", mutate: func(r *http.Request) { r.RemoteAddr = "192.0.2.1:9" }},
		{name: "duplicate origin", method: http.MethodPost, path: "/theme-lab", origin: testOrigin, body: "action=preview", mutate: func(r *http.Request) { r.Header.Add("Origin", testOrigin) }},
		{name: "cross-site metadata", method: http.MethodPost, path: "/theme-lab", origin: testOrigin, body: "action=preview", mutate: func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			probe, downstream := newFixture(t, time.Second)
			arm(t, probe)
			r := httptest.NewRequest(tc.method, testOrigin+tc.path, strings.NewReader(tc.body))
			r.Host = testHost
			r.RemoteAddr = "127.0.0.1:43124"
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Sec-Fetch-Site", "same-origin")
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if tc.mutate != nil {
				tc.mutate(r)
			}
			probe.ServeHTTP(httptest.NewRecorder(), r)
			if downstream.count() != 1 || !bytes.Equal(downstream.last(), []byte(tc.body)) || downstream.lastFormsWereSet() {
				t.Fatalf("delegated body = %q", downstream.last())
			}
			if got := state(t, probe); !got.Armed || got.Started {
				t.Fatalf("ineligible request consumed arm: %+v", got)
			}
		})
	}
}

func TestBusyRearmIsRefused(t *testing.T) {
	probe, _ := newFixture(t, time.Second)
	arm(t, probe)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		probe.ServeHTTP(httptest.NewRecorder(), previewRequest(ctx, "action=preview"))
		close(done)
	}()
	waitStarted(t, probe)
	w := httptest.NewRecorder()
	probe.ServeHTTP(w, controlRequest(http.MethodPost, "/__probe/arm", testToken))
	if w.Code != http.StatusConflict {
		t.Fatalf("busy arm status = %d", w.Code)
	}
	cancel()
	<-done
}

func TestTimeoutIsRetainedAndDelegatesOriginalRequest(t *testing.T) {
	probe, downstream := newFixture(t, 5*time.Millisecond)
	arm(t, probe)
	body := "action=preview&specimen=dashboard"
	probe.ServeHTTP(httptest.NewRecorder(), previewRequest(context.Background(), body))
	got := state(t, probe)
	if !got.TimedOut || !got.Delegated || got.Canceled || got.Live || downstream.count() != 1 || !bytes.Equal(downstream.last(), []byte(body)) || downstream.lastFormsWereSet() {
		t.Fatalf("unexpected timeout: %+v, downstream=%d", got, downstream.count())
	}
	if got.TimedOutAt == "" || got.DelegatedAt == "" {
		t.Fatalf("timeout timestamps missing: %+v", got)
	}
}

func TestControlRequiresTokenExactHostAndLoopbackPeer(t *testing.T) {
	probe, _ := newFixture(t, time.Second)
	for _, tc := range []struct {
		name   string
		token  string
		host   string
		remote string
		want   int
	}{
		{"missing token", "", testHost, "127.0.0.1:1", http.StatusUnauthorized},
		{"wrong token", "wrong", testHost, "127.0.0.1:1", http.StatusUnauthorized},
		{"wrong host", testToken, "localhost:18766", "127.0.0.1:1", http.StatusForbidden},
		{"non-loopback", testToken, testHost, "192.0.2.1:1", http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := controlRequest(http.MethodPost, "/__probe/arm", tc.token)
			r.Host, r.RemoteAddr = tc.host, tc.remote
			w := httptest.NewRecorder()
			probe.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d", w.Code, tc.want)
			}
		})
	}
}
