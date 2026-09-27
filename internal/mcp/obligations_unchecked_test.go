package mcp_test

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// uncheckedFakeBackend reports the edited file's freshness as unchecked,
// the way pkg/client does for a file over the hot-path read cap (RT-26).
type uncheckedFakeBackend struct {
	semanticFakeBackend
}

func (u *uncheckedFakeBackend) ObligationsIndexCheck(_ context.Context, _, _ string) (bool, string) {
	return false, "index freshness not checked: file is 3.0 MiB, over the 2.0 MiB read cap; not refreshed"
}

// TestObligationsTool_IndexUncheckedOnWire: a freshness check that skipped
// its read is carried as index_unchecked — never dropped into a bare
// index_stale:false that reads as "current" (I38, RT-26).
func TestObligationsTool_IndexUncheckedOnWire(t *testing.T) {
	fake := &uncheckedFakeBackend{semanticFakeBackend{
		obligations: semantic.Obligations{Service: "svc", File: "big.md"},
	}}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "big.md"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	got, _ := payload["index_unchecked"].(string)
	if got == "" {
		t.Fatalf("index_unchecked absent; payload=%v", payload)
	}
	if v, _ := payload["index_stale"].(bool); v {
		t.Error("index_stale must not be claimed for an unread file")
	}
}
