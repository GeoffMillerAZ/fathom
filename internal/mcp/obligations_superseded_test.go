package mcp_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// TestSupersededWithheld_RenderedOnThePayload: RS-2's withheld count (rules
// citing a fully superseded decision record) rides the wire response, so the
// withhold is disclosed, never silent (I7).
func TestSupersededWithheld_RenderedOnThePayload(t *testing.T) {
	fake := &semanticFakeBackend{
		obligations: semantic.Obligations{
			Service:    "svc",
			File:       "a.go",
			Rules:      []semantic.Obligation{{Rule: "A rule.", Src: "ADR.md#L1-L2"}},
			Superseded: 2,
		},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "a.go"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	if got := extractPayload(t, resp)["superseded_withheld"]; got != float64(2) {
		t.Errorf("superseded_withheld = %v, want 2", got)
	}
}
