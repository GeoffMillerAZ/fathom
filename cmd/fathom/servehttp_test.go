package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// newTestHandler builds a real mcp.Server backed by a faux client pointing at
// a temp store, then wraps it in the HTTP handler under test.
func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	store := filepath.Join(t.TempDir(), "test.sqlite")
	c, err := client.NewConstantClient(client.ModeLocal, store)
	if err != nil {
		t.Fatalf("NewConstantClient: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	srv := mcp.NewServer(c)
	return newHTTPHandler(srv, c, "127.0.0.1:0", nil)
}

// rpcBody serialises v as compact JSON.
func rpcBody(t *testing.T, v any) io.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("rpcBody marshal: %v", err)
	}
	return bytes.NewReader(b)
}

// postRPC sends a POST /rpc request to handler and returns the decoded response.
func postRPC(t *testing.T, handler http.Handler, body io.Reader) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/rpc", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /rpc status = %d, want 200 (body=%q)", rr.Code, rr.Body.String())
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var m map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode /rpc response: %v (body=%q)", err, rr.Body.String())
	}
	return m
}

// ─── /healthz ───────────────────────────────────────────────────────────────

func TestServeHTTPHealthz(t *testing.T) {
	handler := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /healthz status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "ok") {
		t.Errorf("GET /healthz body = %q, want \"ok\"", rr.Body.String())
	}
}

// ─── POST /rpc: initialize ───────────────────────────────────────────────────

func TestServeHTTPInitialize(t *testing.T) {
	handler := newTestHandler(t)
	resp := postRPC(t, handler, rpcBody(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"clientInfo":      map[string]any{"name": "test", "version": "0.1"},
			"capabilities":    map[string]any{},
		},
	}))
	if resp["error"] != nil {
		t.Fatalf("initialize error: %v", resp["error"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result missing: %v", resp)
	}
	si, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatalf("serverInfo missing: %v", result)
	}
	if si["name"] != "fathom" {
		t.Errorf("serverInfo.name = %v, want fathom", si["name"])
	}
}

// ─── POST /rpc: tools/list ───────────────────────────────────────────────────

func TestServeHTTPToolsList(t *testing.T) {
	handler := newTestHandler(t)
	resp := postRPC(t, handler, rpcBody(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]any{},
	}))
	if resp["error"] != nil {
		t.Fatalf("tools/list error: %v", resp["error"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result missing: %v", resp)
	}
	tools, ok := result["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Fatalf("tools missing or empty: %v", result)
	}
}

// ─── POST /rpc: notification (null id) → 204 no content ─────────────────────

func TestServeHTTPNotificationNoContent(t *testing.T) {
	handler := newTestHandler(t)
	body := rpcBody(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      nil,
		"method":  "notifications/initialized",
		"params":  map[string]any{},
	})
	req := httptest.NewRequest(http.MethodPost, "/rpc", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("notification POST /rpc status = %d, want 204", rr.Code)
	}
}

// ─── GET /events: SSE stream returns hello event ─────────────────────────────

func TestServeHTTPEventsHello(t *testing.T) {
	handler := newTestHandler(t)

	// Use a real httptest.Server so we can close the client connection and
	// unblock the keep-alive goroutine inside the /events handler.
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	resp, err := ts.Client().Get(ts.URL + "/events")
	if err != nil {
		t.Fatalf("GET /events: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /events status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	// Read just enough bytes to see the hello event without blocking.
	buf := make([]byte, 256)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])
	if !strings.Contains(body, "hello") {
		t.Errorf("GET /events body = %q, want to contain \"hello\"", body)
	}
}

// ─── GET /metrics ────────────────────────────────────────────────────────────

func TestServeHTTPMetrics(t *testing.T) {
	handler := newTestHandler(t)

	// Make one /healthz request so the counter is non-zero.
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Now hit /metrics.
	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /metrics status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "fathom_uptime_seconds") {
		t.Errorf("GET /metrics body = %q, want fathom_uptime_seconds", body)
	}
	if !strings.Contains(body, "fathom_requests_total") {
		t.Errorf("GET /metrics body = %q, want fathom_requests_total", body)
	}
	// O108: the memstats/goroutine/subsystem gauges must all be present —
	// this is the instrument the observation asked for, and its whole
	// value is that every one of these lines is actually served, not just
	// the two pre-existing ones above.
	for _, want := range []string{
		"process_uptime_seconds",
		"go_memstats_heap_inuse_bytes",
		"go_memstats_heap_sys_bytes",
		"go_memstats_sys_bytes",
		"go_goroutines",
		"fathom_graph_memo_entries",
		"fathom_graph_memo_hits_total",
		"fathom_graph_memo_misses_total",
		"fathom_open_stores",
		"fathom_autorefresh_total",
		"fathom_typed_pass_total",
		// W-N (docs/plans/daemon-footprint.md): the process-wide typed-pass
		// semaphore's gauge/counter.
		"fathom_typed_pass_inflight",
		"fathom_typed_pass_waited_total",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("GET /metrics body missing %q; body=%q", want, body)
		}
	}
}

func TestServeHTTPMetricsMethodNotAllowed(t *testing.T) {
	handler := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/metrics", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /metrics status = %d, want 405", rr.Code)
	}
}

// ─── GET /debug/pprof ─────────────────────────────────────────────────────────

// TestServeHTTPPprofMounted pins O108's instrumentation: the daemon must
// serve the stdlib pprof handlers on its own mux (not rely on
// net/http/pprof's init()-time registration onto http.DefaultServeMux,
// which this server never serves from) so a synthetic driver — or an
// operator chasing a live leak — can pull /debug/pprof/heap without a
// rebuild.
func TestServeHTTPPprofMounted(t *testing.T) {
	handler := newTestHandler(t)
	for _, path := range []string{
		"/debug/pprof/",
		"/debug/pprof/heap",
		"/debug/pprof/goroutine",
		"/debug/pprof/allocs",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, want 200 (body=%q)", path, rr.Code, rr.Body.String())
		}
	}
}

// ─── POST /reload ─────────────────────────────────────────────────────────────

func TestServeHTTPReload(t *testing.T) {
	handler := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/reload", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /reload status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "ok") {
		t.Errorf("POST /reload body = %q, want \"ok\"", rr.Body.String())
	}
}

func TestServeHTTPReloadMethodNotAllowed(t *testing.T) {
	handler := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/reload", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /reload status = %d, want 405", rr.Code)
	}
}

// ─── path-scoped surfaces: POST /rpc/{surface} ───────────────────────────────

// postRPCPath is postRPC against an arbitrary /rpc path.
func postRPCPath(t *testing.T, handler http.Handler, path string, body io.Reader) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST %s status = %d, want 200 (body=%q)", path, rr.Code, rr.Body.String())
	}
	var m map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode %s response: %v (body=%q)", path, err, rr.Body.String())
	}
	return m
}

func TestServeHTTPCompanionSurfaceToolsList(t *testing.T) {
	handler := newTestHandler(t)
	// /rpc/agent is the primary surface; /rpc/companion is a compatible
	// alias — both must serve the identical allowlist (16 tools since T5-4
	// added read_source, docs/plans/trust-and-tokenomics.md). Exact
	// membership is pinned in internal/mcp; here pin the count so the HTTP
	// path provably serves the slim surface, not the full registry.
	for _, path := range []string{"/rpc/agent", "/rpc/companion"} {
		resp := postRPCPath(t, handler, path, rpcBody(t, map[string]any{
			"jsonrpc": "2.0", "id": 20, "method": "tools/list", "params": map[string]any{},
		}))
		if resp["error"] != nil {
			t.Fatalf("tools/list error on %s: %v", path, resp["error"])
		}
		tools := resp["result"].(map[string]any)["tools"].([]any)
		if len(tools) != 25 {
			t.Fatalf("%s lists %d tools, want 25 (19 + the four ADR-035 verbs + safe_delete_check + localize)", path, len(tools))
		}
	}
}

// TestServeHTTPOffSurfaceExecutesAndFlags: an off-surface tools/call still
// executes (ADR-014 amendment, docs/plans/surfaces-week1.md SW-4) — a
// rejection only teaches an agent to abandon the surface. The call log
// records the pass-through with off_surface:true. Hard gating (the
// pre-amendment behavior) stays available behind Server.strictSurfaces.
func TestServeHTTPOffSurfaceExecutesAndFlags(t *testing.T) {
	store := filepath.Join(t.TempDir(), "test.sqlite")
	c, err := client.NewConstantClient(client.ModeLocal, store)
	if err != nil {
		t.Fatalf("NewConstantClient: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	var buf bytes.Buffer
	srv := mcp.NewServerWithFleetAndLogger(c, nil, mcp.NewCallLogger(&buf))
	handler := newHTTPHandler(srv, c, "127.0.0.1:0", nil)

	// secret_scan is off the companion allowlist. It still reaches the
	// handler — any error it returns is its own backend error (no checkout
	// for "svc" on disk), never the surface-gate message.
	resp := postRPCPath(t, handler, "/rpc/companion", rpcBody(t, map[string]any{
		"jsonrpc": "2.0", "id": 21, "method": "tools/call",
		"params": map[string]any{"name": "secret_scan", "arguments": map[string]any{"service": "svc"}},
	}))
	if errObj, ok := resp["error"].(map[string]any); ok {
		if msg, _ := errObj["message"].(string); strings.Contains(msg, "is not on the") {
			t.Fatalf("off-surface secret_scan should execute, not be gated: %v", msg)
		}
	}
	line := strings.TrimSpace(buf.String())
	if !strings.Contains(line, `"off_surface":true`) {
		t.Errorf("call-log line %q should flag off_surface", line)
	}

	// Hard gating stays available behind strictSurfaces.
	srv.SetStrictSurfaces(true)
	strictResp := postRPCPath(t, handler, "/rpc/companion", rpcBody(t, map[string]any{
		"jsonrpc": "2.0", "id": 22, "method": "tools/call",
		"params": map[string]any{"name": "secret_scan", "arguments": map[string]any{"service": "svc"}},
	}))
	errObj, ok := strictResp["error"].(map[string]any)
	if !ok {
		t.Fatalf("secret_scan under strictSurfaces: want error, got %v", strictResp)
	}
	if msg := errObj["message"].(string); !strings.Contains(msg, "surface") {
		t.Errorf("error message %q should name the surface", msg)
	}
}

func TestServeHTTPUnknownSurface(t *testing.T) {
	handler := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/rpc/bogus", rpcBody(t, map[string]any{
		"jsonrpc": "2.0", "id": 22, "method": "tools/list", "params": map[string]any{},
	}))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("POST /rpc/bogus status = %d, want 404", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, "companion") {
		t.Errorf("404 body %q should list the valid surfaces", body)
	}
}

func TestServeHTTPTrailingSlashDiagnostic(t *testing.T) {
	handler := newTestHandler(t)
	for _, path := range []string{"/rpc/", "/rpc/companion/extra"} {
		req := httptest.NewRequest(http.MethodPost, path, rpcBody(t, map[string]any{
			"jsonrpc": "2.0", "id": 25, "method": "tools/list", "params": map[string]any{},
		}))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("POST %s status = %d, want 404", path, rr.Code)
		}
		if body := rr.Body.String(); !strings.Contains(body, "companion") {
			t.Errorf("POST %s 404 body %q should list the valid endpoints", path, body)
		}
	}
}

// set_profile no longer exists (removed entirely, SW-2): there is no
// HTTP-specific rejection to test any more — calling it returns the same
// generic tool-not-found error as any other unregistered name.

func TestServeHTTPCallLogRecordsSurface(t *testing.T) {
	store := filepath.Join(t.TempDir(), "test.sqlite")
	c, err := client.NewConstantClient(client.ModeLocal, store)
	if err != nil {
		t.Fatalf("NewConstantClient: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	var buf bytes.Buffer
	srv := mcp.NewServerWithFleetAndLogger(c, nil, mcp.NewCallLogger(&buf))
	handler := newHTTPHandler(srv, c, "127.0.0.1:0", nil)

	_ = postRPCPath(t, handler, "/rpc/companion", rpcBody(t, map[string]any{
		"jsonrpc": "2.0", "id": 24, "method": "tools/call",
		"params": map[string]any{"name": "list_services", "arguments": map[string]any{}},
	}))
	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("no call-log line written")
	}
	if !strings.Contains(line, `"surface":"companion"`) {
		t.Errorf("call-log line %q should record the surface", line)
	}
	if !strings.Contains(line, `"tool":"list_services"`) {
		t.Errorf("call-log line %q should record the tool", line)
	}
}

// ─── call-log file helper ─────────────────────────────────────────────────────

func TestOpenCallLogCreatesParents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "nested", "calls.jsonl")
	f, err := openCallLog(path)
	if err != nil {
		t.Fatalf("openCallLog: %v", err)
	}
	if _, err := f.WriteString("x\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	// Reopen appends rather than truncates.
	f2, err := openCallLog(path)
	if err != nil {
		t.Fatalf("openCallLog reopen: %v", err)
	}
	if _, err := f2.WriteString("y\n"); err != nil {
		t.Fatalf("write 2: %v", err)
	}
	f2.Close()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(b) != "x\ny\n" {
		t.Errorf("call log content = %q, want appended \"x\\ny\\n\"", b)
	}
}

// ─── flag validation ─────────────────────────────────────────────────────────

func TestServeHTTPMissingStore(t *testing.T) {
	// CF-7 made runServeHTTP load user+project config unconditionally, so
	// this test MUST NOT see the operator's real ~/.fathom/config.toml — a
	// real [daemon].store would defeat the missing-store assertion (and a
	// real [daemon].addr would send the test to ListenAndServe on a real
	// port). withIsolatedConfigHome isolates HOME and the cwd (project
	// config resolves from os.Getwd()).
	withIsolatedConfigHome(t)
	var out, errBuf bytes.Buffer
	code := runServeHTTP([]string{}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when --store is missing")
	}
	if errBuf.String() == "" {
		t.Fatal("expected non-empty stderr")
	}
	if !strings.Contains(errBuf.String(), "--store is required") {
		t.Fatalf("stderr = %q, want the missing-store error specifically", errBuf.String())
	}
}

// ─── main dispatch: serve-http subcommand ────────────────────────────────────

func TestRunServeHTTPSubcommand(t *testing.T) {
	// Config isolation is load-bearing (CF-7): with the operator's real
	// ~/.fathom/config.toml visible, [daemon].store would default --store
	// and this invocation would proceed to open a store, wire an LLM, and
	// ListenAndServe on the real [daemon].addr instead of failing fast.
	withIsolatedConfigHome(t)
	var out, errBuf bytes.Buffer
	// Smoke-test: ensure "serve-http" is recognised by runWithStdin's switch.
	// We deliberately omit --store: with config isolated there is no
	// [daemon].store default, so the flag-validation path returns nonzero
	// before any port binding.
	code := runWithStdin([]string{"serve-http"}, nil, &out, &errBuf)
	if code == 0 {
		t.Fatal("serve-http without --store should exit nonzero")
	}
}
