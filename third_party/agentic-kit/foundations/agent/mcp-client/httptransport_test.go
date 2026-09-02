// httptransport_test.go — tests for the Streamable HTTP client transport.
//
// All HTTP is httptest-backed; no network. The scripted MCP handler here
// mirrors the pipeServer fake in mcpclient_test.go, translated to the
// single-POST Streamable HTTP shape: requests get a JSON body back,
// notifications get 202 with an empty body.

package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/event"
)

// mcpHTTPHandler is a minimal scripted MCP server over Streamable HTTP:
// it echoes the request id and answers initialize / tools/list / tools/call
// with the same response builders the stdio tests use. Notifications
// (no id) get 202 Accepted with an empty body.
func mcpHTTPHandler(t *testing.T, tools []toolInfoJSON, callText string, callIsError bool) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("handler: read body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("handler: parse request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if len(req.ID) == 0 || string(req.ID) == "null" {
			// Notification: acknowledge, no body.
			w.WriteHeader(http.StatusAccepted)
			return
		}

		var resp string
		switch req.Method {
		case "initialize":
			resp = fmt.Sprintf(
				`{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":"2025-11-25","capabilities":{"tools":{}},"serverInfo":{"name":"fake-http","version":"1"}}}`,
				req.ID,
			)
		case "tools/list":
			b, _ := json.Marshal(struct {
				Tools []toolInfoJSON `json:"tools"`
			}{Tools: tools})
			resp = fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":%s}`, req.ID, b)
		case "tools/call":
			type item struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			b, _ := json.Marshal(struct {
				Content []item `json:"content"`
				IsError bool   `json:"isError"`
			}{
				Content: []item{{Type: "text", Text: callText}},
				IsError: callIsError,
			})
			resp = fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":%s}`, req.ID, b)
		default:
			resp = fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"error":{"code":-32601,"message":"method not found"}}`, req.ID)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, resp)
	}
}

// ─── transport unit tests ─────────────────────────────────────────────────────

func TestHTTPTransportRoundtrip(t *testing.T) {
	const respBody = `{"jsonrpc":"2.0","id":1,"result":{}}`
	var gotContentType, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, respBody)
	}))
	defer srv.Close()

	tr := NewHTTPTransport(srv.URL, srv.Client(), 0)
	if err := tr.Send(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)); err != nil {
		t.Fatalf("Send: %v", err)
	}
	got, err := tr.Receive(context.Background())
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if string(got) != respBody {
		t.Errorf("Receive = %q, want %q", got, respBody)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotAccept != "application/json, text/event-stream" {
		t.Errorf("Accept = %q, want \"application/json, text/event-stream\"", gotAccept)
	}
}

func TestHTTPTransportNotificationQueuesNothing(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"202 Accepted", http.StatusAccepted},
		{"204 No Content", http.StatusNoContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			tr := NewHTTPTransport(srv.URL, srv.Client(), 0)
			if err := tr.Send(context.Background(), []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)); err != nil {
				t.Fatalf("Send: %v", err)
			}
			_, err := tr.Receive(context.Background())
			if err == nil {
				t.Fatal("expected error from Receive on empty queue, got nil")
			}
			if !strings.Contains(err.Error(), "no queued response") {
				t.Errorf("error %q does not mention 'no queued response'", err)
			}
		})
	}
}

func TestHTTPTransportStatusError(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"403 Forbidden", http.StatusForbidden},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			tr := NewHTTPTransport(srv.URL, srv.Client(), 0)
			err := tr.Send(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var statusErr *HTTPStatusError
			if !errors.As(err, &statusErr) {
				t.Fatalf("expected *HTTPStatusError, got %T: %v", err, err)
			}
			if statusErr.StatusCode != tt.status {
				t.Errorf("StatusCode = %d, want %d", statusErr.StatusCode, tt.status)
			}
		})
	}
}

func TestHTTPTransportOversizedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, strings.Repeat("x", 64))
	}))
	defer srv.Close()

	tr := NewHTTPTransport(srv.URL, srv.Client(), 16)
	err := tr.Send(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	if err == nil {
		t.Fatal("expected error for oversized response, got nil")
	}
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Errorf("expected ErrResponseTooLarge, got: %v", err)
	}
}

func TestHTTPTransportSSEUnsupported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {}\n\n")
	}))
	defer srv.Close()

	tr := NewHTTPTransport(srv.URL, srv.Client(), 0)
	err := tr.Send(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	if err == nil {
		t.Fatal("expected error for text/event-stream response, got nil")
	}
	if !strings.Contains(err.Error(), "event-stream") {
		t.Errorf("error %q does not mention event-stream", err)
	}
}

func TestHTTPTransportCloseIdempotent(t *testing.T) {
	tr := NewHTTPTransport("http://unused.invalid/rpc", nil, 0)
	if err := tr.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := tr.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if err := tr.Send(context.Background(), []byte(`{}`)); err == nil {
		t.Fatal("expected error on Send after Close")
	}
	if _, err := tr.Receive(context.Background()); err == nil {
		t.Fatal("expected error on Receive after Close")
	}
}

func TestHTTPTransportContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tr := NewHTTPTransport(srv.URL, srv.Client(), 0)
	if err := tr.Send(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)); !errors.Is(err, context.Canceled) {
		t.Errorf("Send with cancelled ctx: got %v, want context.Canceled", err)
	}
	if _, err := tr.Receive(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Receive with cancelled ctx: got %v, want context.Canceled", err)
	}
}

// ─── trace injection ──────────────────────────────────────────────────────────

func TestHTTPTransportTraceInjection(t *testing.T) {
	tc := event.TraceContext{
		RunID:  "0123456789abcdef0123456789abcdef",
		SpanID: "0123456789abcdef",
	}
	tests := []struct {
		name       string
		ctx        context.Context
		wantHeader string // "" means the header must be absent
	}{
		{
			name:       "trace in context injects traceparent",
			ctx:        event.WithTrace(context.Background(), tc),
			wantHeader: "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01",
		},
		{
			name:       "no trace in context sends no header",
			ctx:        context.Background(),
			wantHeader: "",
		},
		{
			name:       "zero trace in context sends no header",
			ctx:        event.WithTrace(context.Background(), event.TraceContext{}),
			wantHeader: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			var present bool
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.Header.Get(event.TraceparentHeader)
				_, present = r.Header[http.CanonicalHeaderKey(event.TraceparentHeader)]
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{}`)
			}))
			defer srv.Close()

			tr := NewHTTPTransport(srv.URL, srv.Client(), 0)
			if err := tr.Send(tt.ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)); err != nil {
				t.Fatalf("Send: %v", err)
			}
			if tt.wantHeader == "" {
				if present {
					t.Errorf("traceparent header present (%q), want absent", got)
				}
				return
			}
			if got != tt.wantHeader {
				t.Errorf("traceparent = %q, want %q", got, tt.wantHeader)
			}
		})
	}
}

// ─── Client over HTTP transport ───────────────────────────────────────────────

func TestClientOverHTTPTransport(t *testing.T) {
	srv := httptest.NewServer(mcpHTTPHandler(t, defaultTools, "hello", false))
	defer srv.Close()

	tr := NewHTTPTransport(srv.URL, srv.Client(), 0)
	c, err := New(context.Background(), tr, Options{})
	if err != nil {
		t.Fatalf("New over HTTP transport: %v", err)
	}
	defer c.Close()

	// Handshake recorded the negotiated version.
	if c.ProtocolVersion != "2025-11-25" {
		t.Errorf("ProtocolVersion = %q, want 2025-11-25", c.ProtocolVersion)
	}

	// Tool pins recorded at the initial tools/list, same as over stdio.
	for _, tool := range defaultTools {
		h, pinned := c.VerifyPin(tool.Name)
		if !pinned {
			t.Errorf("tool %q not pinned after New", tool.Name)
			continue
		}
		want := toolHash(tool.Name, tool.Description, tool.InputSchema, nil)
		if h != want {
			t.Errorf("tool %q: pin = %s, want %s", tool.Name, h, want)
		}
	}

	// Re-list: unchanged tools, no drift error.
	if _, err := c.ListTools(context.Background()); err != nil {
		t.Errorf("ListTools over HTTP: %v", err)
	}

	// tools/call round-trip.
	result, err := c.CallTool(context.Background(), "echo", json.RawMessage(`{"text":"hello"}`))
	if err != nil {
		t.Fatalf("CallTool over HTTP: %v", err)
	}
	if result != "hello" {
		t.Errorf("CallTool result = %q, want %q", result, "hello")
	}
}

func TestClientOverHTTPTransportInBandError(t *testing.T) {
	srv := httptest.NewServer(mcpHTTPHandler(t, defaultTools, "boom", true))
	defer srv.Close()

	tr := NewHTTPTransport(srv.URL, srv.Client(), 0)
	c, err := New(context.Background(), tr, Options{})
	if err != nil {
		t.Fatalf("New over HTTP transport: %v", err)
	}
	defer c.Close()

	_, err = c.CallTool(context.Background(), "echo", json.RawMessage(`{"text":"x"}`))
	if err == nil {
		t.Fatal("expected ToolCallError, got nil")
	}
	var tce *ToolCallError
	if !errors.As(err, &tce) {
		t.Fatalf("expected *ToolCallError, got %T: %v", err, err)
	}
	if tce.Content != "boom" {
		t.Errorf("ToolCallError.Content = %q, want %q", tce.Content, "boom")
	}
}
