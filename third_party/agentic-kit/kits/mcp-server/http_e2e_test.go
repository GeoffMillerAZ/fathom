package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mcpclient "github.com/geoffmilleraz/agentic-kit/foundations/agent/mcp-client"
	toolregistry "github.com/geoffmilleraz/agentic-kit/foundations/agent/tool-registry"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/event"
)

// seqHex returns a deterministic randHex: call n yields the (n+1)-th hex
// digit repeated to width, so successive IDs are distinct, non-zero,
// lowercase hex — valid under the W3C traceparent grammar.
func seqHex() func(nBytes int) string {
	digits := "123456789abcdef"
	n := 0
	return func(nBytes int) string {
		d := digits[n%len(digits)]
		n++
		return strings.Repeat(string(d), nBytes*2)
	}
}

// TestStreamableHTTPTraceRoundTrip is the H6 end-to-end canary: a kit MCP
// client speaking the streamable HTTP transport to the kit MCP server, with
// the W3C trace context crossing the process boundary. It proves the full
// composition — event.FormatTraceparent on the client Send, the traceparent
// header on the wire, event.ParseTraceparent + Child in the HTTP handler,
// and the tool handler observing the propagated context — which no single
// package's tests can prove alone.
func TestStreamableHTTPTraceRoundTrip(t *testing.T) {
	reg := toolregistry.New(toolregistry.Options{})
	reg.MustRegister(toolregistry.Tool{
		Def: llmclient.ToolDef{
			Name:        "trace-echo",
			Description: "echo the trace context visible to the tool handler",
			InputSchema: toolregistry.ObjectSchema(),
		},
		Handler: func(ctx context.Context, _ json.RawMessage) (string, error) {
			tc, ok := event.TraceFrom(ctx)
			return fmt.Sprintf(`{"ok":%t,"run_id":%q,"span_id":%q,"parent_span_id":%q}`,
				ok, tc.RunID, tc.SpanID, tc.ParentSpanID), nil
		},
	})
	now := func() time.Time { return time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC) }
	serverHex := seqHex()
	h := NewHTTPHandler(New(Config{}, reg, now), HTTPOptions{RandHex: serverHex})
	srv := httptest.NewServer(h)
	defer srv.Close()

	tc := event.NewTrace(seqHex()) // RunID = 32×'1', SpanID = 16×'2'
	ctx := event.WithTrace(context.Background(), tc)

	cli, err := mcpclient.New(ctx, mcpclient.NewHTTPTransport(srv.URL, srv.Client(), 0), mcpclient.Options{})
	if err != nil {
		t.Fatalf("client handshake over HTTP: %v", err)
	}
	defer cli.Close()

	out, err := cli.CallTool(ctx, "trace-echo", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	var got struct {
		OK           bool   `json:"ok"`
		RunID        string `json:"run_id"`
		SpanID       string `json:"span_id"`
		ParentSpanID string `json:"parent_span_id"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("tool output not JSON: %v (out=%q)", err, out)
	}
	if !got.OK {
		t.Fatalf("tool handler saw no trace context; want propagated trace")
	}
	if got.RunID != tc.RunID {
		t.Errorf("RunID not propagated: server saw %q, client sent %q", got.RunID, tc.RunID)
	}
	if got.ParentSpanID != tc.SpanID {
		t.Errorf("ParentSpanID: server saw %q, want client's wire SpanID %q", got.ParentSpanID, tc.SpanID)
	}
	if got.SpanID == tc.SpanID || got.SpanID == "" {
		t.Errorf("server-side SpanID must be a fresh child span, got %q (client sent %q)", got.SpanID, tc.SpanID)
	}

	t.Run("no trace on ctx → no header → handler sees none", func(t *testing.T) {
		out, err := cli.CallTool(context.Background(), "trace-echo", json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if !strings.Contains(out, `"ok":false`) {
			t.Errorf("want no trace visible to handler without ctx trace, got %s", out)
		}
	})
}
