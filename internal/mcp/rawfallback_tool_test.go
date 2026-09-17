package mcp_test

// rawfallback_tool_test.go — wave O1 (ADR-037), the wire-envelope proof: the
// obligations MCP tool must merge a rawFallbackBackend's raw rows AHEAD of
// the compiled ones (a raw row exists specifically because it is NEARER
// than what compiled), tag them tier:"raw" with a "why" note, and leave a
// backend without the capability (or a call that finds no gap) byte-shape
// unchanged — the existing goldens (TestObligationsTool_WithRules and the
// L1 payload replay) must keep passing.

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// rawFallbackFakeBackend adds rawFallbackBackend to semanticFakeBackend.
type rawFallbackFakeBackend struct {
	semanticFakeBackend
	rawRows      []semantic.Obligation
	rawWhy       string
	rawOK        bool
	rawErr       error
	gotRawFile   string
	gotRawMax    int
	rawCallCount int
}

func (r *rawFallbackFakeBackend) RawFallbackObligations(_ context.Context, _, file string, max int) ([]semantic.Obligation, string, bool, error) {
	r.gotRawFile = file
	r.gotRawMax = max
	r.rawCallCount++
	return r.rawRows, r.rawWhy, r.rawOK, r.rawErr
}

// TestObligationsTool_RawFallback_NestedRankedAheadOfCompiledRoot is test
// (b) at the wire level: a compiled ROOT-tier rule already answers the
// file; a raw NESTED-tier row (the fallback backend's own answer) must be
// prepended ahead of it, never displacing the compiled row, never coming
// after it.
func TestObligationsTool_RawFallback_NestedRankedAheadOfCompiledRoot(t *testing.T) {
	fake := &rawFallbackFakeBackend{
		semanticFakeBackend: semanticFakeBackend{
			obligations: semantic.Obligations{
				Service: "svc", File: "internal/extractor/goextract.go",
				Rules: []semantic.Obligation{
					{Rule: "Root compiled rule.", Src: "CLAUDE.md#L1-L2", Tier: "doc-claimed", Layer: "obligations"},
				},
			},
		},
		rawRows: []semantic.Obligation{
			{Rule: "Register every extractor.", Src: "internal/CLAUDE.md#L1-L3", Tier: "raw", Layer: "obligations", Why: "compiled layer incomplete: budget_stopped; raw nearest guide served"},
		},
		rawOK: true,
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "internal/extractor/goextract.go"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	rules, ok := payload["rules"].([]any)
	if !ok || len(rules) != 2 {
		t.Fatalf("expected 2 rules (1 raw + 1 compiled), got %v", payload["rules"])
	}
	first, _ := rules[0].(map[string]any)
	if tier, _ := first["tier"].(string); tier != "raw" {
		t.Errorf("rules[0].tier = %q, want %q (raw must be prepended, not displaced below the compiled root rule)", tier, "raw")
	}
	if why, _ := first["why"].(string); why == "" {
		t.Error("rules[0].why should be set on the raw row")
	}
	second, _ := rules[1].(map[string]any)
	if tier, _ := second["tier"].(string); tier != "doc-claimed" {
		t.Errorf("rules[1].tier = %q, want the compiled root rule preserved second", tier)
	}
	if _, present := second["why"]; present {
		t.Errorf("rules[1] (compiled) should carry no why field, got %v", second["why"])
	}
	if fake.gotRawFile != "internal/extractor/goextract.go" {
		t.Errorf("RawFallbackObligations called with file %q", fake.gotRawFile)
	}
	if fake.gotRawMax != semantic.RawFallbackReserve {
		t.Errorf("RawFallbackObligations called with max %d, want semantic.RawFallbackReserve (%d)", fake.gotRawMax, semantic.RawFallbackReserve)
	}
}

// TestObligationsTool_RawFallback_NoGapLeavesPayloadUnchanged is test (c) at
// the wire level: when the backend reports ok=false (no real gap), the
// obligations payload must be byte-shape identical to a backend that never
// implemented rawFallbackBackend at all.
func TestObligationsTool_RawFallback_NoGapLeavesPayloadUnchanged(t *testing.T) {
	compiled := []semantic.Obligation{{Rule: "Sort outputs.", Src: "CLAUDE.md#L10-L10", Tier: "doc-claimed", Layer: "obligations"}}
	fake := &rawFallbackFakeBackend{
		semanticFakeBackend: semanticFakeBackend{
			obligations: semantic.Obligations{Service: "svc", File: "pkg/foo.go", Rules: compiled},
		},
		rawOK: false, // no gap
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "pkg/foo.go"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	rules, ok := payload["rules"].([]any)
	if !ok || len(rules) != 1 {
		t.Fatalf("expected exactly the 1 compiled rule, got %v", payload["rules"])
	}
	if fake.rawCallCount != 1 {
		t.Errorf("RawFallbackObligations call count = %d, want 1 (it must still be consulted)", fake.rawCallCount)
	}
	first, _ := rules[0].(map[string]any)
	if _, present := first["why"]; present {
		t.Errorf("compiled-only payload should carry no why field, got %v", first)
	}
}

// TestObligationsTool_RawFallback_BackendWithoutCapabilityUnchanged proves
// the existing (pre-wave-O1) TestObligationsTool_WithRules golden path is
// untouched: a plain semanticFakeBackend never implements rawFallbackBackend
// at all, so the type assertion in renderObligationReceipt simply skips the
// whole mechanism.
func TestObligationsTool_RawFallback_BackendWithoutCapabilityUnchanged(t *testing.T) {
	fake := &semanticFakeBackend{
		obligations: semantic.Obligations{
			Service: "svc", File: "internal/mcp/tools.go",
			Rules: []semantic.Obligation{{Rule: "Sort outputs.", Src: "CLAUDE.md#L10-L10", Tier: "doc-claimed"}},
		},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "internal/mcp/tools.go"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	rules, ok := payload["rules"].([]any)
	if !ok || len(rules) != 1 {
		t.Fatalf("expected exactly the 1 compiled rule, got %v", payload["rules"])
	}
}

// TestObligationsTool_RawFallback_RootScopeSkipped: a fileless (root-scope)
// call never consults rawFallbackBackend — the mechanism is keyed on a
// single edited path, out of RC-22's "root conventions" shape (mirroring
// modserve.Bind's own root-scope skip).
func TestObligationsTool_RawFallback_RootScopeSkipped(t *testing.T) {
	fake := &rawFallbackFakeBackend{
		semanticFakeBackend: semanticFakeBackend{
			obligations: semantic.Obligations{Rules: []semantic.Obligation{{Rule: "Repo-wide rule.", Src: "CLAUDE.md#L1-L1", Tier: "doc-claimed", Layer: "obligations"}}},
		},
		rawOK: true,
		rawRows: []semantic.Obligation{
			{Rule: "Should never appear.", Src: "x/CLAUDE.md#L1-L1", Tier: "raw", Layer: "obligations"},
		},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 4, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	if fake.rawCallCount != 0 {
		t.Errorf("RawFallbackObligations call count = %d, want 0 for a root-scope call", fake.rawCallCount)
	}
	payload := extractPayload(t, resp)
	rules, ok := payload["rules"].([]any)
	if !ok || len(rules) != 1 {
		t.Fatalf("expected exactly the 1 root-scope compiled rule, got %v", payload["rules"])
	}
}

// TestObligationsTool_RawFallback_ThroughAutoRefreshWrapper proves the O7
// forwarding class this wave adds to autorefresh.go actually works: without
// the explicit RawFallbackObligations forward, wrapping the fake in
// mcp.NewAutoRefresh would silently strip the capability, exactly like the
// dogfood O7 bug this file's own doc comment documents for every sibling
// semantic capability.
func TestObligationsTool_RawFallback_ThroughAutoRefreshWrapper(t *testing.T) {
	fake := &rawFallbackFakeBackend{
		semanticFakeBackend: semanticFakeBackend{
			obligations: semantic.Obligations{Service: "svc", File: "internal/extractor/goextract.go"},
		},
		rawOK:   true,
		rawRows: []semantic.Obligation{{Rule: "Register every extractor.", Src: "internal/CLAUDE.md#L1-L3", Tier: "raw", Layer: "obligations", Why: "why"}},
	}
	srv := mcp.NewServer(mcp.NewAutoRefresh(fake, nil))
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 5, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "internal/extractor/goextract.go"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("obligations through wrapper: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	rules, ok := payload["rules"].([]any)
	if !ok || len(rules) != 1 {
		t.Fatalf("raw fallback stripped by the auto-refresh wrapper: rules=%v", payload["rules"])
	}
	first, _ := rules[0].(map[string]any)
	if tier, _ := first["tier"].(string); tier != "raw" {
		t.Errorf("rules[0].tier = %q through the wrapper, want raw", tier)
	}
}
