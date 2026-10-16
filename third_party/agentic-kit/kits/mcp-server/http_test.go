// http_test.go — Streamable HTTP transport tests: cross-transport
// conformance (the same registry through Serve and NewHTTPHandler must yield
// byte-identical JSON-RPC responses), the HTTP-only spec obligations
// (Origin, MCP-Protocol-Version, notification 202-empty-body), and inbound
// traceparent extraction. The stdio tests in mcpserver_test.go are
// deliberately untouched; this file only adds coverage.

package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	toolregistry "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/agent/tool-registry"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/event"
)

// postJSON drives one POST through the handler with optional extra headers.
func postJSON(t *testing.T, h http.Handler, body string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestStreamableHTTPConformance drives the SAME registry and config through
// both transports — Serve over in-memory pipes and NewHTTPHandler over
// httptest — and asserts equivalent observables per case:
//
//   - request-shaped messages (initialize, ping, tools/list, tools/call
//     success and isError, unknown method, batch, oversized): the stdio
//     response line and the HTTP 200 application/json body are BYTE-
//     IDENTICAL JSON-RPC response objects;
//   - notifications: NO JSON-RPC response on either side — stdio emits
//     nothing, HTTP returns 202 Accepted with an EMPTY body (the body
//     assertion, not just the status, is the equivalence);
//   - plus the HTTP-only obligations: MCP-Protocol-Version (present /
//     absent → documented 2025-03-26 fallback / unknown → 400) and Origin
//     validation (allowed / disallowed / absent / fail-closed empty list).
func TestStreamableHTTPConformance(t *testing.T) {
	oversized := `{"jsonrpc":"2.0","id":42,"method":"tools/call","params":{"name":"echo","arguments":{"text":"` +
		strings.Repeat("x", 300) + `"}}}`

	cases := []struct {
		name         string
		cfg          Config
		message      string
		wantResponse bool // false = notification semantics (stdio: nothing; HTTP: 202 empty)
	}{
		{
			name:         "initialize",
			message:      `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"conformance","version":"0.0.1"}}}`,
			wantResponse: true,
		},
		{
			name:         "ping",
			message:      `{"jsonrpc":"2.0","id":2,"method":"ping"}`,
			wantResponse: true,
		},
		{
			name:         "tools list",
			message:      `{"jsonrpc":"2.0","id":3,"method":"tools/list"}`,
			wantResponse: true,
		},
		{
			name:         "tools call success",
			message:      `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"add","arguments":{"a":2,"b":3}}}`,
			wantResponse: true,
		},
		{
			name:         "tools call in-band error",
			message:      `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"fail","arguments":{}}}`,
			wantResponse: true,
		},
		{
			name:         "unknown method",
			message:      `{"jsonrpc":"2.0","id":6,"method":"frobnicate"}`,
			wantResponse: true,
		},
		{
			name:         "batch rejected",
			message:      `[{"jsonrpc":"2.0","id":1,"method":"ping"}]`,
			wantResponse: true,
		},
		{
			name:         "malformed json",
			message:      `{not json`,
			wantResponse: true,
		},
		{
			name:         "oversized",
			cfg:          Config{MaxMessageBytes: 96},
			message:      oversized,
			wantResponse: true,
		},
		{
			name:         "notification",
			message:      `{"jsonrpc":"2.0","method":"notifications/initialized"}`,
			wantResponse: false,
		},
		{
			name:         "notification null id",
			message:      `{"jsonrpc":"2.0","id":null,"method":"ping"}`,
			wantResponse: false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			// Stdio observable: fresh server, one message, Serve to EOF.
			stdioLines := runServe(t, newTestServer(t, tt.cfg), tt.message+"\n")

			// HTTP observable: fresh server with the SAME config, one POST.
			h := NewHTTPHandler(newTestServer(t, tt.cfg), HTTPOptions{})
			rec := postJSON(t, h, tt.message, nil)

			if !tt.wantResponse {
				if len(stdioLines) != 0 {
					t.Errorf("stdio emitted %d response lines for a notification, want 0: %q", len(stdioLines), stdioLines)
				}
				if rec.Code != http.StatusAccepted {
					t.Errorf("HTTP status = %d, want 202", rec.Code)
				}
				if got := rec.Body.String(); got != "" {
					t.Errorf("HTTP notification response body = %q, want EMPTY", got)
				}
				return
			}

			if len(stdioLines) != 1 {
				t.Fatalf("stdio emitted %d response lines, want 1: %q", len(stdioLines), stdioLines)
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("HTTP status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
			if got, want := rec.Body.String(), stdioLines[0]; got != want {
				t.Errorf("transport responses diverge:\nhttp:  %s\nstdio: %s", got, want)
			}
		})
	}

	// HTTP-only: MCP-Protocol-Version request-header handling (2025-11-25
	// Streamable HTTP §Protocol Version Header, verified against the
	// published spec — see docs/designs/mcp-streamable-http.md).
	t.Run("MCP-Protocol-Version", func(t *testing.T) {
		ping := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
		pingResp := `{"jsonrpc":"2.0","id":1,"result":{}}`
		versionCases := []struct {
			name       string
			header     string // "" = absent
			wantStatus int
			wantBody   string // asserted only for 200
		}{
			{"known version proceeds", "2025-11-25", http.StatusOK, pingResp},
			{"older known version proceeds", "2025-03-26", http.StatusOK, pingResp},
			{"absent header assumes 2025-03-26 and proceeds", "", http.StatusOK, pingResp},
			{"unknown version rejected 400", "1999-01-01", http.StatusBadRequest, ""},
			{"malformed version rejected 400", "not-a-version", http.StatusBadRequest, ""},
		}
		for _, tt := range versionCases {
			t.Run(tt.name, func(t *testing.T) {
				h := NewHTTPHandler(newTestServer(t, Config{}), HTTPOptions{})
				hdr := map[string]string{}
				if tt.header != "" {
					hdr[mcpProtocolVersionHeader] = tt.header
				}
				rec := postJSON(t, h, ping, hdr)
				if rec.Code != tt.wantStatus {
					t.Fatalf("status = %d, want %d (body %q)", rec.Code, tt.wantStatus, rec.Body.String())
				}
				if tt.wantStatus == http.StatusOK && rec.Body.String() != tt.wantBody {
					t.Errorf("body = %q, want %q", rec.Body.String(), tt.wantBody)
				}
			})
		}
	})

	// HTTP-only: Origin validation (DNS-rebinding protection MUST).
	t.Run("Origin", func(t *testing.T) {
		ping := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
		originCases := []struct {
			name       string
			allowed    []string
			origin     string // "" = absent
			wantStatus int
		}{
			{"allowed origin passes", []string{"http://localhost:8080"}, "http://localhost:8080", http.StatusOK},
			{"disallowed origin rejected", []string{"http://localhost:8080"}, "http://evil.example", http.StatusForbidden},
			{"absent origin passes (non-browser client)", []string{"http://localhost:8080"}, "", http.StatusOK},
			{"empty allowlist rejects all origin-bearing requests", nil, "http://localhost:8080", http.StatusForbidden},
			{"empty allowlist still passes absent origin", nil, "", http.StatusOK},
		}
		for _, tt := range originCases {
			t.Run(tt.name, func(t *testing.T) {
				h := NewHTTPHandler(newTestServer(t, Config{}), HTTPOptions{AllowedOrigins: tt.allowed})
				hdr := map[string]string{}
				if tt.origin != "" {
					hdr["Origin"] = tt.origin
				}
				rec := postJSON(t, h, ping, hdr)
				if rec.Code != tt.wantStatus {
					t.Fatalf("status = %d, want %d (body %q)", rec.Code, tt.wantStatus, rec.Body.String())
				}
				if tt.wantStatus == http.StatusForbidden && rec.Body.Len() != 0 {
					t.Errorf("403 body = %q, want empty", rec.Body.String())
				}
			})
		}

		t.Run("origin checked before method and body", func(t *testing.T) {
			// A disallowed Origin on a non-POST request is still 403: the
			// security check runs first, before any body read.
			h := NewHTTPHandler(newTestServer(t, Config{}), HTTPOptions{})
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Origin", "http://evil.example")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", rec.Code)
			}
		})
	})

	// HTTP-only: no GET SSE stream in stateless mode — 405 per spec.
	t.Run("non-POST rejected 405", func(t *testing.T) {
		h := NewHTTPHandler(newTestServer(t, Config{}), HTTPOptions{})
		for _, method := range []string{http.MethodGet, http.MethodDelete, http.MethodPut} {
			req := httptest.NewRequest(method, "/", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s status = %d, want 405", method, rec.Code)
			}
		}
	})

	// HTTP-only: MaxBodyBytes override wins over the server's MaxMessageBytes.
	t.Run("MaxBodyBytes precedence", func(t *testing.T) {
		// Server allows 4 MiB; handler narrows to 64 bytes.
		h := NewHTTPHandler(newTestServer(t, Config{}), HTTPOptions{MaxBodyBytes: 64})
		big := `{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"echo","arguments":{"text":"` +
			strings.Repeat("y", 200) + `"}}}`
		rec := postJSON(t, h, big, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		want := `{"jsonrpc":"2.0","id":9,"error":{"code":-32700,"message":"message exceeds maximum size of 64 bytes"}}`
		if got := rec.Body.String(); got != want {
			t.Errorf("body = %s, want %s", got, want)
		}
	})
}

// newTraceCaptureHandler builds a handler around a single "capture" tool
// whose handler records the event.TraceContext visible in its context.
func newTraceCaptureHandler(t *testing.T, opts HTTPOptions) (http.Handler, func() (event.TraceContext, bool)) {
	t.Helper()
	var mu sync.Mutex
	var got event.TraceContext
	var ok bool
	reg := toolregistry.New(toolregistry.Options{})
	reg.MustRegister(toolregistry.Tool{
		Def: llmclient.ToolDef{
			Name:        "capture",
			Description: "records the ambient trace context",
			InputSchema: toolregistry.ObjectSchema(),
		},
		Handler: func(ctx context.Context, _ json.RawMessage) (string, error) {
			mu.Lock()
			defer mu.Unlock()
			got, ok = event.TraceFrom(ctx)
			return "captured", nil
		},
	})
	s := New(Config{Name: "trace-test", Version: "0.0.1"}, reg, fixedClock(testEpoch))
	return NewHTTPHandler(s, opts), func() (event.TraceContext, bool) {
		mu.Lock()
		defer mu.Unlock()
		return got, ok
	}
}

// TestHTTPHandlerTraceExtraction proves the server-side trace seam: a valid
// inbound traceparent header roots the tool-call context as a CHILD of the
// caller's span (same RunID, fresh SpanID from the injected randomness,
// ParentSpanID = the caller's SpanID); an absent, malformed, or all-zero
// header degrades to no trace — never an error, never a bogus trace.
func TestHTTPHandlerTraceExtraction(t *testing.T) {
	const (
		traceID  = "0123456789abcdef0123456789abcdef" // 32 hex
		parentID = "00f067aa0ba902b7"                 // 16 hex
	)
	fixedRandHex := func(n int) string { return strings.Repeat("ab", n) }
	wantChildSpan := fixedRandHex(8) // 16 chars: "abababababababab"
	callBody := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"capture","arguments":{}}}`

	tests := []struct {
		name      string
		header    string // "" = absent
		wantTrace bool
	}{
		{"valid version-00 header", "00-" + traceID + "-" + parentID + "-01", true},
		{"future version accepted per W3C", "cc-" + traceID + "-" + parentID + "-01", true},
		{"absent header", "", false},
		{"malformed header", "not-a-traceparent", false},
		{"all-zero trace-id rejected", "00-" + strings.Repeat("0", 32) + "-" + parentID + "-01", false},
		{"version ff rejected", "ff-" + traceID + "-" + parentID + "-01", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, captured := newTraceCaptureHandler(t, HTTPOptions{RandHex: fixedRandHex})
			hdr := map[string]string{}
			if tt.header != "" {
				hdr[event.TraceparentHeader] = tt.header
			}
			rec := postJSON(t, h, callBody, hdr)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %q) — trace handling must never fail a request", rec.Code, rec.Body.String())
			}
			tc, ok := captured()
			if ok != tt.wantTrace {
				t.Fatalf("tool saw trace = %v, want %v (tc %+v)", ok, tt.wantTrace, tc)
			}
			if !tt.wantTrace {
				return
			}
			if tc.RunID != traceID {
				t.Errorf("RunID = %q, want %q (caller's trace-id must survive the boundary)", tc.RunID, traceID)
			}
			if tc.ParentSpanID != parentID {
				t.Errorf("ParentSpanID = %q, want %q (caller's span becomes the parent)", tc.ParentSpanID, parentID)
			}
			if tc.SpanID != wantChildSpan {
				t.Errorf("SpanID = %q, want %q (fresh child span from injected randomness)", tc.SpanID, wantChildSpan)
			}

			// The stamp proof: event.Apply on the captured context carries
			// the caller's RunID onto emitted telemetry.
			var ev event.Event
			event.Apply(tc, &ev)
			if ev.Attributes[event.AttrRunID] != traceID {
				t.Errorf("Apply stamped run_id %q, want %q", ev.Attributes[event.AttrRunID], traceID)
			}
			if ev.Attributes[event.AttrParentSpanID] != parentID {
				t.Errorf("Apply stamped parent_span_id %q, want %q", ev.Attributes[event.AttrParentSpanID], parentID)
			}
		})
	}
}

// TestHTTPHandlerConcurrentRequests exercises the handler's stated
// concurrency contract under -race: parallel requests through one handler
// (shared registry, shared call log, shared tool_use counter) must all
// succeed with correct bodies.
func TestHTTPHandlerConcurrentRequests(t *testing.T) {
	var log syncBuffer
	cfg := Config{Name: "demo-tools", Version: "1.2.3", CallLog: &log}
	h := NewHTTPHandler(New(cfg, newTestRegistry(t), fixedClock(testEpoch)), HTTPOptions{})

	const n = 16
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"add","arguments":{"a":%d,"b":1}}}`, i, i)
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			want := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":{"content":[{"type":"text","text":"%d"}],"isError":false}}`, i, i+1)
			if rec.Code != http.StatusOK || rec.Body.String() != want {
				errs <- fmt.Errorf("request %d: status %d body %q", i, rec.Code, rec.Body.String())
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if got := strings.Count(log.String(), "\n"); got != n {
		t.Errorf("call log has %d entries, want %d", got, n)
	}
}

// syncBuffer is a mutex-guarded byte buffer: the call-log WRITER serializes
// Append calls, but the test's final read must also synchronize with them.
type syncBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestNewHTTPHandlerPanicsOnNilServer(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewHTTPHandler with nil server did not panic")
		}
	}()
	NewHTTPHandler(nil, HTTPOptions{})
}
