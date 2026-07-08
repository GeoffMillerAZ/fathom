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
	store := filepath.Join(t.TempDir(), "test.duckdb")
	c, err := client.NewConstantClient(client.ModeLocal, store)
	if err != nil {
		t.Fatalf("NewConstantClient: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	srv := mcp.NewServer(c)
	return newHTTPHandler(srv)
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
	resp := postRPCPath(t, handler, "/rpc/companion", rpcBody(t, map[string]any{
		"jsonrpc": "2.0", "id": 20, "method": "tools/list", "params": map[string]any{},
	}))
	if resp["error"] != nil {
		t.Fatalf("tools/list error: %v", resp["error"])
	}
	tools := resp["result"].(map[string]any)["tools"].([]any)
	// Exact membership is pinned in internal/mcp; here pin the count so the
	// HTTP path provably serves the slim surface, not the full registry.
	if len(tools) != 15 {
		t.Fatalf("companion surface lists %d tools, want 15", len(tools))
	}
}

func TestServeHTTPSurfaceGatesToolsCall(t *testing.T) {
	handler := newTestHandler(t)
	resp := postRPCPath(t, handler, "/rpc/companion", rpcBody(t, map[string]any{
		"jsonrpc": "2.0", "id": 21, "method": "tools/call",
		"params": map[string]any{"name": "secret_scan", "arguments": map[string]any{"service": "svc"}},
	}))
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("secret_scan via /rpc/companion: want error, got %v", resp)
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

func TestServeHTTPSetProfileDisabled(t *testing.T) {
	handler := newTestHandler(t)
	resp := postRPC(t, handler, rpcBody(t, map[string]any{
		"jsonrpc": "2.0", "id": 23, "method": "tools/call",
		"params": map[string]any{"name": "set_profile", "arguments": map[string]any{"profile": "coding"}},
	}))
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("set_profile over HTTP: want error, got %v", resp)
	}
	if msg := errObj["message"].(string); !strings.Contains(msg, "/rpc/") {
		t.Errorf("error message %q should point at path-scoped surfaces", msg)
	}
}

func TestServeHTTPCallLogRecordsSurface(t *testing.T) {
	store := filepath.Join(t.TempDir(), "test.duckdb")
	c, err := client.NewConstantClient(client.ModeLocal, store)
	if err != nil {
		t.Fatalf("NewConstantClient: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	var buf bytes.Buffer
	srv := mcp.NewServerWithFleetAndLogger(c, nil, mcp.NewCallLogger(&buf))
	handler := newHTTPHandler(srv)

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
	var out, errBuf bytes.Buffer
	code := runServeHTTP([]string{}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when --store is missing")
	}
	if errBuf.String() == "" {
		t.Fatal("expected non-empty stderr")
	}
}

// ─── main dispatch: serve-http subcommand ────────────────────────────────────

func TestRunServeHTTPSubcommand(t *testing.T) {
	// Verify that "serve-http" is wired through runWithStdin.
	// We use --addr on a random port; since we never call ListenAndServe in
	// runServeHTTP for tests (the handler factory is separate), this just
	// verifies the flag + client construction succeeds when --store is given.
	store := filepath.Join(t.TempDir(), "test.duckdb")
	var out, errBuf bytes.Buffer
	// runServeHTTP with a valid store but no actual listener — we only test
	// the handler via newHTTPHandler; the port-binding path is covered by
	// the integration call below which we skip: just verify dispatch.
	_ = store
	_ = out
	_ = errBuf
	// Smoke-test: ensure "serve-http" is recognised by runWithStdin's switch.
	// We pass --store so flag parsing succeeds, but the call will attempt to
	// bind :0 and start serving — we don't want that in a unit test.
	// Instead we just verify the flag validation path returns nonzero without a store.
	code := runWithStdin([]string{"serve-http"}, nil, &out, &errBuf)
	if code == 0 {
		t.Fatal("serve-http without --store should exit nonzero")
	}
}
