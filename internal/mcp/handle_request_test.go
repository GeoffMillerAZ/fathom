// Package mcp_test — tests for Server.HandleRequest.
package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// handleReq is a helper that calls HandleRequest and decodes the response.
func handleReq(t *testing.T, srv *mcp.Server, v any) (map[string]any, bool) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("handleReq marshal: %v", err)
	}
	resp, hasResp := srv.HandleRequest(context.Background(), raw)
	if !hasResp {
		return nil, false
	}
	var m map[string]any
	if err := json.Unmarshal(resp, &m); err != nil {
		t.Fatalf("handleReq unmarshal response: %v (raw=%q)", err, resp)
	}
	return m, true
}

func TestHandleRequestInitialize(t *testing.T) {
	srv := mcp.NewServer(nil)
	resp, hasResp := handleReq(t, srv, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"clientInfo":      map[string]any{"name": "test", "version": "0.1"},
			"capabilities":    map[string]any{},
		},
	})
	if !hasResp {
		t.Fatal("HandleRequest returned hasResp=false for initialize")
	}
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
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

func TestHandleRequestToolsList(t *testing.T) {
	srv := mcp.NewServer(nil)
	resp, hasResp := handleReq(t, srv, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]any{},
	})
	if !hasResp {
		t.Fatal("HandleRequest returned hasResp=false for tools/list")
	}
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
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

func TestHandleRequestNotificationNoResponse(t *testing.T) {
	srv := mcp.NewServer(nil)
	_, hasResp := handleReq(t, srv, map[string]any{
		"jsonrpc": "2.0",
		"id":      nil,
		"method":  "notifications/initialized",
		"params":  map[string]any{},
	})
	if hasResp {
		t.Error("HandleRequest should return hasResp=false for notification (null id)")
	}
}

func TestHandleRequestMalformedJSON(t *testing.T) {
	srv := mcp.NewServer(nil)
	resp, hasResp := srv.HandleRequest(context.Background(), []byte("not valid json"))
	if !hasResp {
		t.Fatal("HandleRequest should return hasResp=true for a parse error")
	}
	var m map[string]any
	if err := json.Unmarshal(resp, &m); err != nil {
		t.Fatalf("unmarshal parse-error response: %v", err)
	}
	errObj, ok := m["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error field: %v", m)
	}
	if code, _ := errObj["code"].(float64); code != -32700 {
		t.Errorf("error.code = %v, want -32700", code)
	}
}

func TestHandleRequestUnknownMethod(t *testing.T) {
	srv := mcp.NewServer(nil)
	resp, hasResp := handleReq(t, srv, map[string]any{
		"jsonrpc": "2.0",
		"id":      99,
		"method":  "no/such/method",
	})
	if !hasResp {
		t.Fatal("HandleRequest should return hasResp=true for unknown method")
	}
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error: %v", resp)
	}
	if code, _ := errObj["code"].(float64); code != -32601 {
		t.Errorf("error.code = %v, want -32601", code)
	}
}

func TestHandleRequestEmptyInput(t *testing.T) {
	srv := mcp.NewServer(nil)
	_, hasResp := srv.HandleRequest(context.Background(), []byte{})
	if hasResp {
		t.Error("HandleRequest should return hasResp=false for empty input")
	}
}

// TestHandleRequestInitializeReportsBuildVersion pins serverInfo.version to
// the build identity (buildinfo.Version), never a hard-coded "1.0.0".
func TestHandleRequestInitializeReportsBuildVersion(t *testing.T) {
	srv := mcp.NewServer(nil)
	resp, _ := handleReq(t, srv, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{}},
	})
	si := resp["result"].(map[string]any)["serverInfo"].(map[string]any)
	if got := si["version"]; got != buildinfo.Version() || got == "1.0.0" {
		t.Errorf("serverInfo.version = %v, want %q", got, buildinfo.Version())
	}
}
