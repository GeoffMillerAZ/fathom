package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	mcpclient "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/agent/mcp-client"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// kitClientSentence is the adoption docs' promise (docs/adoption/codex.md
// §2 and the capability table; RP-13, docs/plans/reach-parity.md): it is
// the exact text this test's failure prints, so a red test and the doc
// never disagree about what was proven.
const kitClientSentence = "fathom's /rpc/agent negotiates with a Streamable-HTTP MCP client fathom did not write (agentic-kit's mcp-client, 2025-11-25 stateless mode): initialize → tools/list → tools/call succeed end to end under httptest"

// TestServeHTTPNegotiatesWithKitClient (RP-13) drives POST /rpc/agent
// through agentic-kit's mcp-client — the third party: its HTTP transport
// speaks MCP Streamable HTTP (single POST, Accept json+SSE, 202/204 for
// notifications) and its Client performs the initialize handshake offering
// 2025-11-25, pins tools/list hashes, and calls a tool. Every step below
// is what an adopter's Codex/Gemini/Cursor client does before the first
// answer arrives; a failure here means the adoption doc's sentence is
// false, so that sentence is the failure text.
func TestServeHTTPNegotiatesWithKitClient(t *testing.T) {
	ts := httptest.NewServer(newTestHandler(t))
	defer ts.Close()

	ctx := context.Background()
	tr := mcpclient.NewHTTPTransport(ts.URL+"/rpc/agent", ts.Client(), 0)
	c, err := mcpclient.New(ctx, tr, mcpclient.Options{ClientName: "reach-parity-test", ClientVersion: "0"})
	if err != nil {
		t.Fatalf("NOT PROVEN: %s — initialize (or the initial tools/list pin) failed: %v", kitClientSentence, err)
	}
	defer c.Close()

	// The server echoed the client's offered revision (2025-11-25 is in the
	// supported table), not a hard-coded older one.
	if c.ProtocolVersion != "2025-11-25" {
		t.Errorf("NOT PROVEN: %s — server negotiated %q for a client offering 2025-11-25", kitClientSentence, c.ProtocolVersion)
	}

	tools, err := c.ListTools(ctx)
	if err != nil {
		t.Fatalf("NOT PROVEN: %s — tools/list: %v", kitClientSentence, err)
	}
	var names []string
	for _, tl := range tools {
		names = append(names, tl.Name)
	}
	// SR-33 (surface reckoning F-86): /rpc/agent now serves the eight-tool
	// default, not the wider allowlist AgentSurfaceTools still names.
	want := mcp.DefaultSurfaceTools()
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("NOT PROVEN: %s — /rpc/agent listed %v through the kit client, want the pinned allowlist %v", kitClientSentence, names, want)
	}
	// The second ListTools re-verified every pin: a byte-stable tools/list
	// (RP-12's deterministic order) is what lets a pinning client trust it.
	if _, err := c.ListTools(ctx); err != nil {
		t.Errorf("NOT PROVEN: %s — tools/list changed between two calls: %v", kitClientSentence, err)
	}

	// SR-23: capabilities merged into status(section:"tools"), byte-
	// compatible with its own {active_surface, tool_count, schema_bytes, ...}.
	text, err := c.CallTool(ctx, "status", json.RawMessage(`{"section":"tools"}`))
	if err != nil {
		t.Fatalf("NOT PROVEN: %s — tools/call status: %v", kitClientSentence, err)
	}
	var caps struct {
		ActiveSurface string `json:"active_surface"`
		ToolCount     int    `json:"tool_count"`
		SchemaBytes   int    `json:"schema_bytes"`
	}
	if err := json.Unmarshal([]byte(text), &caps); err != nil {
		t.Fatalf("NOT PROVEN: %s — status text block is not JSON: %v (%q)", kitClientSentence, err, text)
	}
	if caps.ActiveSurface != mcp.SurfaceAgent || caps.ToolCount != len(want) || caps.SchemaBytes == 0 {
		t.Errorf("NOT PROVEN: %s — status(section:tools) over the kit client = %+v", kitClientSentence, caps)
	}
	t.Logf("agent surface schema budget: %d bytes per session (tools/list, %d tools)", caps.SchemaBytes, caps.ToolCount)
}

// TestServeHTTPToleratesProtocolVersionHeader (RP-12): the MCP-Protocol-
// Version header a 2025-06-18+ client sends on every request is never
// rejected; the response carries the revision the server negotiates for it.
func TestServeHTTPToleratesProtocolVersionHeader(t *testing.T) {
	handler := newTestHandler(t)
	for hdr, want := range map[string]string{"2025-06-18": "2025-06-18", "2026-07-28": "2025-11-25", "": "2025-11-25"} {
		req := httptest.NewRequest("POST", "/rpc/agent", rpcBody(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"}))
		req.Header.Set("Content-Type", "application/json")
		if hdr != "" {
			req.Header.Set("MCP-Protocol-Version", hdr)
		}
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != 200 {
			t.Errorf("header %q: status %d, want 200 (tolerated, never rejected)", hdr, rr.Code)
		}
		if got := rr.Header().Get("MCP-Protocol-Version"); got != want {
			t.Errorf("header %q: response MCP-Protocol-Version = %q, want %q", hdr, got, want)
		}
	}
}
