// A1-1 (docs/plans/activation.md): status(section:"daemon")'s
// surface_traffic block (the retired server_stats tool's field, unchanged —
// SR-23) — per-surface {calls, last_ts} — answers "is anything flowing on
// /rpc/agent?" from the daemon itself, without tailing the JSONL call log.
// Follows the IngestStats/ClassifiedZeroStats capability-block convention
// (statsbackend_test.go): present + non-empty only when a call logger is
// wired in, omitted entirely otherwise.
package mcp_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

func TestServerStats_SurfaceTraffic_ReportsPerSurfaceCounts(t *testing.T) {
	var buf bytes.Buffer
	srv := mcp.NewServerWithLogger(&fakeBackend{}, mcp.NewCallLogger(&buf))

	agentCtx := mcp.WithRequestScope(context.Background(), mcp.SurfaceAgent, true)
	bareCtx := mcp.WithRequestScope(context.Background(), "", true)

	// Two calls on /rpc/agent, one on bare /rpc.
	scopedRoundtrip(t, srv, agentCtx, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{"name": "find_symbol", "arguments": map[string]any{"service": "svc", "name": "Foo"}},
	})
	scopedRoundtrip(t, srv, agentCtx, request{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: map[string]any{"name": "find_symbol", "arguments": map[string]any{"service": "svc", "name": "Foo"}},
	})
	scopedRoundtrip(t, srv, bareCtx, request{
		JSONRPC: "2.0", ID: 3, Method: "tools/call",
		Params: map[string]any{"name": "find_symbol", "arguments": map[string]any{"service": "svc", "name": "Foo"}},
	})

	resp := scopedRoundtrip(t, srv, agentCtx, request{
		JSONRPC: "2.0", ID: 4, Method: "tools/call",
		Params: map[string]any{"name": "status", "arguments": map[string]any{"section": "daemon"}},
	})
	p := extToolPayload(t, resp)
	traffic, ok := p["surface_traffic"].(map[string]any)
	if !ok {
		t.Fatalf("surface_traffic missing or wrong shape: %v", p)
	}
	agent, ok := traffic["agent"].(map[string]any)
	if !ok {
		t.Fatalf("surface_traffic[agent] missing or wrong shape: %v", traffic)
	}
	// Counts reflect COMPLETED calls: the call log entry for this very
	// server_stats invocation is written after its handler returns (it needs
	// the final latency/error outcome), so the snapshot the handler body
	// reads never includes the in-flight call itself — only the two prior
	// find_symbol calls on the agent surface.
	if got := agent["calls"]; got != float64(2) {
		t.Errorf("surface_traffic[agent].calls = %v, want 2 (the 2 prior find_symbol calls; this server_stats call isn't logged until after it returns)", got)
	}
	if lastTS, _ := agent["last_ts"].(string); lastTS == "" {
		t.Error("surface_traffic[agent].last_ts is empty, want a timestamp")
	}
	bare, ok := traffic[""].(map[string]any)
	if !ok {
		t.Fatalf("surface_traffic[\"\"] (bare /rpc) missing or wrong shape: %v", traffic)
	}
	if got := bare["calls"]; got != float64(1) {
		t.Errorf("surface_traffic[\"\"].calls = %v, want 1", got)
	}
}

// TestServerStats_SurfaceTraffic_OmittedWithoutCallLogger: no call logger
// wired in (mcp.NewServer, not NewServerWithLogger) means the instrument
// itself is absent — the block must be omitted, not present-empty.
func TestServerStats_SurfaceTraffic_OmittedWithoutCallLogger(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{"name": "status", "arguments": map[string]any{"section": "daemon"}},
	})
	p := extToolPayload(t, resp)
	if _, ok := p["surface_traffic"]; ok {
		t.Errorf("surface_traffic should be omitted with no call logger wired in, got %v", p)
	}
}

// TestServerStats_SurfaceTraffic_ZeroCallsEmitsEmptyBlock (bounce finding
// 3): a WIRED logger with zero completed calls must emit surface_traffic
// as a present-but-empty object — a freshly restarted daemon reads as
// "instrument live, zero traffic", distinguishable from "instrument
// missing" (the omitted case above) by any existence check (doctrine 7).
// The first-ever call being server_stats itself exercises exactly this:
// its own log entry is written after the handler returns.
func TestServerStats_SurfaceTraffic_ZeroCallsEmitsEmptyBlock(t *testing.T) {
	var buf bytes.Buffer
	srv := mcp.NewServerWithLogger(&fakeBackend{}, mcp.NewCallLogger(&buf))
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{"name": "status", "arguments": map[string]any{"section": "daemon"}},
	})
	p := extToolPayload(t, resp)
	traffic, ok := p["surface_traffic"].(map[string]any)
	if !ok {
		t.Fatalf("surface_traffic must be PRESENT (as an empty object) when a logger is wired with zero calls, got %v", p)
	}
	if len(traffic) != 0 {
		t.Errorf("surface_traffic at zero completed calls = %v, want empty object", traffic)
	}
}

// TestServerStats_SurfaceTraffic_StdioSurfaceCounted: a plain roundtrip
// (Serve, no WithRequestScope) resolves to the zero-value ("") surface —
// proving stdio/bare-/rpc traffic is tracked like any named surface, not
// silently dropped for lacking a surface tag. A preceding find_symbol call
// establishes the completed-call this test asserts on (see the "counts
// reflect COMPLETED calls" note above — server_stats can't see itself).
func TestServerStats_SurfaceTraffic_StdioSurfaceCounted(t *testing.T) {
	var buf bytes.Buffer
	srv := mcp.NewServerWithLogger(&fakeBackend{}, mcp.NewCallLogger(&buf))
	roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{"name": "find_symbol", "arguments": map[string]any{"service": "svc", "name": "Foo"}},
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: map[string]any{"name": "status", "arguments": map[string]any{"section": "daemon"}},
	})
	p := extToolPayload(t, resp)
	traffic, ok := p["surface_traffic"].(map[string]any)
	if !ok {
		t.Fatalf("surface_traffic missing or wrong shape: %v", p)
	}
	bare, ok := traffic[""].(map[string]any)
	if !ok {
		t.Fatalf("surface_traffic missing the stdio/bare (\"\") surface: %v", traffic)
	}
	if got := bare["calls"]; got != float64(1) {
		t.Errorf("surface_traffic[\"\"].calls = %v, want 1", got)
	}
}
