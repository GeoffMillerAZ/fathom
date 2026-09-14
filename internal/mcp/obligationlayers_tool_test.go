package mcp_test

// obligationlayers_tool_test.go — VB-9 (docs/plans/verbs.md, ADR-035
// decision 2): the obligations tool's "layer" request param, end to end
// through the MCP surface.

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// layeredFakeBackend adds obligationLayerBackend + rootObligationsLayerBackend
// to semanticFakeBackend, recording which layer value each call carried.
type layeredFakeBackend struct {
	semanticFakeBackend
	gotLayer     semantic.ObligationLayerFilter
	gotRootLayer semantic.ObligationLayerFilter
}

func (l *layeredFakeBackend) ObligationsLayer(_ context.Context, _, _ string, _ int, layer semantic.ObligationLayerFilter) (semantic.Obligations, error) {
	l.gotLayer = layer
	return l.obligations, l.obligationsErr
}

func (l *layeredFakeBackend) RootObligationsLayer(_ context.Context, _ string, _ int, layer semantic.ObligationLayerFilter) (semantic.Obligations, error) {
	l.gotRootLayer = layer
	return l.obligations, l.obligationsErr
}

func TestObligationsTool_LayerDefaultsToRules(t *testing.T) {
	fake := &layeredFakeBackend{semanticFakeBackend: semanticFakeBackend{
		obligations: semantic.Obligations{Service: "svc", File: "a.go"},
	}}
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
	if fake.gotLayer != semantic.ObligationLayerRules {
		t.Errorf("backend saw layer %q, want %q (the default)", fake.gotLayer, semantic.ObligationLayerRules)
	}
	payload := extractPayload(t, resp)
	if payload["layer"] != "rules" {
		t.Errorf(`payload["layer"] = %v, want "rules"`, payload["layer"])
	}
}

func TestObligationsTool_LayerClaimsPassesThrough(t *testing.T) {
	fake := &layeredFakeBackend{semanticFakeBackend: semanticFakeBackend{
		obligations: semantic.Obligations{Service: "svc", File: "a.go"},
	}}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "a.go", "layer": "claims"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	if fake.gotLayer != semantic.ObligationLayerClaims {
		t.Errorf("backend saw layer %q, want %q", fake.gotLayer, semantic.ObligationLayerClaims)
	}
}

func TestObligationsTool_LayerRoot(t *testing.T) {
	fake := &layeredFakeBackend{semanticFakeBackend: semanticFakeBackend{
		obligations: semantic.Obligations{Service: "svc"},
	}}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "layer": "all"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	if fake.gotRootLayer != semantic.ObligationLayerAll {
		t.Errorf("backend saw root layer %q, want %q", fake.gotRootLayer, semantic.ObligationLayerAll)
	}
	payload := extractPayload(t, resp)
	if payload["root"] != true {
		t.Errorf(`payload["root"] = %v, want true`, payload["root"])
	}
}

func TestObligationsTool_LayerInvalidValueRefused(t *testing.T) {
	fake := &layeredFakeBackend{semanticFakeBackend: semanticFakeBackend{
		obligations: semantic.Obligations{Service: "svc", File: "a.go"},
	}}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "a.go", "layer": "bogus"},
		},
	})
	if resp["error"] == nil {
		t.Fatal("expected an error for an unrecognized layer value")
	}
}

// TestObligationsTool_LegacyBackendDegradesToAll: a backend that predates
// VB-9 (semanticFakeBackend alone) never sees a "layer" argument at all —
// the tool must still answer, reporting the union it actually served.
func TestObligationsTool_LegacyBackendDegradesToAll(t *testing.T) {
	fake := &semanticFakeBackend{
		obligations: semantic.Obligations{Service: "svc", File: "a.go"},
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
	payload := extractPayload(t, resp)
	if payload["layer"] != "all" {
		t.Errorf(`payload["layer"] = %v, want "all" (the legacy-backend degrade)`, payload["layer"])
	}
}

// TestDisplacedByTier_RenderedOnThePayload: displaced_by_tier rides the
// wire response byte-for-byte from semantic.Obligations.
func TestDisplacedByTier_RenderedOnThePayload(t *testing.T) {
	fake := &semanticFakeBackend{
		obligations: semantic.Obligations{
			Service:         "svc",
			File:            "a.go",
			Displaced:       3,
			DisplacedByTier: map[string]int{"root": 2, "package": 1},
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
	payload := extractPayload(t, resp)
	tiers, ok := payload["displaced_by_tier"].(map[string]any)
	if !ok {
		t.Fatalf("displaced_by_tier missing or wrong shape: %v", payload["displaced_by_tier"])
	}
	if tiers["root"] != float64(2) || tiers["package"] != float64(1) {
		t.Errorf("displaced_by_tier = %v, want root:2 package:1", tiers)
	}
}
