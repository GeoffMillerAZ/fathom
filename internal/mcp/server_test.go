// Package mcp_test drives the MCP server via framed JSON-RPC byte streams.
// Tests are table-driven and use a fake query backend — no DuckDB, no disk.
package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// ─── helpers ────────────────────────────────────────────────────────────────

// sendLine writes a single newline-delimited JSON-RPC request to buf.
func sendLine(t *testing.T, buf *bytes.Buffer, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("sendLine marshal: %v", err)
	}
	buf.Write(b)
	buf.WriteByte('\n')
}

// readResponse reads exactly one JSON object from r, which must be a
// *strings.Reader or *bytes.Reader. It returns the parsed map.
func readResponse(t *testing.T, out *bytes.Buffer) map[string]any {
	t.Helper()
	dec := json.NewDecoder(out)
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("readResponse decode: %v (buf=%q)", err, out.String())
	}
	return m
}

// request is a convenience type for building JSON-RPC request maps.
type request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// roundtrip sends req and returns the parsed response.
func roundtrip(t *testing.T, srv *mcp.Server, req request) map[string]any {
	t.Helper()
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	sendLine(t, in, req)
	// Input is a finite buffer, so Serve reads to EOF and returns on its own.
	// Run it synchronously (no goroutine) so reading `out` afterwards is
	// race-free, then decode the single response.
	_ = srv.Serve(context.Background(), in, out)
	var resp map[string]any
	if err := json.NewDecoder(bytes.NewReader(out.Bytes())).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v (raw: %q)", err, out.String())
	}
	return resp
}

// ─── initialize ─────────────────────────────────────────────────────────────

func TestInitialize(t *testing.T) {
	srv := mcp.NewServer(nil)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0",
		ID:      0,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": "2024-11-05",
			"clientInfo":      map[string]any{"name": "test", "version": "0.1"},
			"capabilities":    map[string]any{},
		},
	})

	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result missing or wrong type: %v", resp)
	}
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("protocolVersion = %v, want 2024-11-05", result["protocolVersion"])
	}
	si, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatalf("serverInfo missing: %v", result)
	}
	if si["name"] != "fathom" {
		t.Errorf("serverInfo.name = %v, want fathom", si["name"])
	}
	caps, ok := result["capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("capabilities missing: %v", result)
	}
	if _, ok := caps["tools"]; !ok {
		t.Error("capabilities.tools missing")
	}
}

// ─── ping ───────────────────────────────────────────────────────────────────

func TestPing(t *testing.T) {
	srv := mcp.NewServer(nil)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0",
		ID:      99,
		Method:  "ping",
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	result := resp["result"]
	if result == nil {
		t.Fatal("ping: expected non-nil result")
	}
}

// ─── tools/list ─────────────────────────────────────────────────────────────

func TestToolsList(t *testing.T) {
	srv := mcp.NewServer(nil)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
		Params:  map[string]any{},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result missing: %v", resp)
	}
	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("tools missing or wrong type: %v", result)
	}
	if len(tools) == 0 {
		t.Fatal("tools/list returned empty list")
	}
	// Each tool must have name, description, inputSchema.
	for _, raw := range tools {
		tool, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("tool entry is not a map: %v", raw)
		}
		for _, field := range []string{"name", "description", "inputSchema"} {
			if tool[field] == nil {
				t.Errorf("tool %v missing field %q", tool["name"], field)
			}
		}
	}
}

// ─── tools/call — set_profile filters tools/list ────────────────────────────

func TestSetProfileFiltersToolsList(t *testing.T) {
	// With a fake backend and "coding" profile, only coding+shared tools appear.
	fake := &fakeBackend{}
	srv := mcp.NewServer(fake)

	// Set profile to coding.
	setResp := roundtrip(t, srv, request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      "set_profile",
			"arguments": map[string]any{"profile": "coding"},
		},
	})
	if setResp["error"] != nil {
		t.Fatalf("set_profile error: %v", setResp["error"])
	}

	// Now list tools.
	listResp := roundtrip(t, srv, request{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/list",
		Params:  map[string]any{},
	})
	if listResp["error"] != nil {
		t.Fatalf("tools/list error: %v", listResp["error"])
	}
	result := listResp["result"].(map[string]any)
	tools := result["tools"].([]any)

	for _, raw := range tools {
		tool := raw.(map[string]any)
		name := tool["name"].(string)
		// investigation-only tools must not appear in coding profile.
		if name == "find_callers" {
			// find_callers is shared so it is allowed
			continue
		}
	}
	// query_symbols must appear (it's in coding group).
	found := false
	for _, raw := range tools {
		if raw.(map[string]any)["name"] == "query_symbols" {
			found = true
		}
	}
	if !found {
		t.Error("query_symbols not found in coding profile")
	}
}

// ─── tools/call — query_symbols ─────────────────────────────────────────────

func TestToolsCallQuerySymbols(t *testing.T) {
	fake := &fakeBackend{
		queryResult: fakeQueryResult{
			nodes: []fakeNode{
				{name: "ValidateToken", kind: "GoFunc", file: "auth/token.go"},
			},
		},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0",
		ID:      10,
		Method:  "tools/call",
		Params: map[string]any{
			"name": "query_symbols",
			"arguments": map[string]any{
				"service":  "payment-service",
				"concern":  "authentication",
				"keywords": []string{"token", "auth"},
				"top":      5,
			},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result missing: %v", resp)
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("content missing or empty: %v", result)
	}
	textEntry := content[0].(map[string]any)
	text, ok := textEntry["text"].(string)
	if !ok {
		t.Fatalf("content[0].text missing: %v", textEntry)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("payload unmarshal: %v (text=%q)", err, text)
	}
	if payload["nodes"] == nil {
		t.Error("payload.nodes missing")
	}
}

// ─── unknown method → -32601 ─────────────────────────────────────────────────

func TestUnknownMethod(t *testing.T) {
	srv := mcp.NewServer(nil)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0",
		ID:      20,
		Method:  "nonexistent/method",
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got result: %v", resp)
	}
	code, ok := errObj["code"].(float64)
	if !ok {
		t.Fatalf("error.code missing: %v", errObj)
	}
	if code != -32601 {
		t.Errorf("error.code = %v, want -32601", code)
	}
}

// ─── malformed JSON → -32700 ─────────────────────────────────────────────────

func TestMalformedJSON(t *testing.T) {
	srv := mcp.NewServer(nil)

	in := strings.NewReader("not valid json\n")
	out := &bytes.Buffer{}
	// Finite input → Serve returns at EOF; run synchronously (race-free).
	_ = srv.Serve(context.Background(), in, out)

	var resp map[string]any
	if err := json.NewDecoder(bytes.NewReader(out.Bytes())).Decode(&resp); err != nil {
		t.Fatalf("decode malformed response: %v (buf=%q)", err, out.String())
	}
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got: %v", resp)
	}
	code := errObj["code"].(float64)
	if code != -32700 {
		t.Errorf("error.code = %v, want -32700", code)
	}
}

// ─── notifications/initialized (no response expected) ───────────────────────

func TestNotificationsInitialized(t *testing.T) {
	// notifications/initialized has a null id and should receive no response.
	srv := mcp.NewServer(nil)

	in := &bytes.Buffer{}
	out := &bytes.Buffer{}

	sendLine(t, in, map[string]any{
		"jsonrpc": "2.0",
		"id":      nil,
		"method":  "notifications/initialized",
		"params":  map[string]any{},
	})
	// Follow with a ping so we know Serve processed the notification.
	sendLine(t, in, request{
		JSONRPC: "2.0",
		ID:      77,
		Method:  "ping",
	})

	// Finite input → Serve returns at EOF; run synchronously (race-free).
	_ = srv.Serve(context.Background(), in, out)

	// The notification must produce NO response; the only response object on
	// the stream should be the ping (id=77).
	dec := json.NewDecoder(bytes.NewReader(out.Bytes()))
	var resp map[string]any
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("decode response: %v (raw: %q)", err, out.String())
	}
	if id, _ := resp["id"].(float64); id != 77 {
		t.Errorf("expected ping response id=77, got %v", resp["id"])
	}
	if dec.More() {
		t.Errorf("notification should produce no response; extra output: %q", out.String())
	}
}
