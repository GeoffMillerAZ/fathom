package mcp_test

import (
	"context"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// findingsNodes: A (pkg/a.go) calls B (pkg/b.go) calls C (pkg/c.go).
func findingsNodes() []core.SymbolNode {
	return []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "A", Kind: core.KindGoFunc, FilePath: "pkg/a.go", Calls: []string{"svc::pkg.B"}},
		{Service: "svc", Package: "pkg", Name: "B", Kind: core.KindGoFunc, FilePath: "pkg/b.go", Calls: []string{"svc::pkg.C"}},
		{Service: "svc", Package: "pkg", Name: "C", Kind: core.KindGoFunc, FilePath: "pkg/c.go"},
	}
}

// TestFindingsNear_ToolIsOnInvestigationSurfaceNotAgentSurface pins surface
// membership directly (ADR.md's own convention): findings_near must appear
// on the bare/unscoped registry AND the "investigation" named surface, but
// NEVER on the agent/companion surface's allowlist — the mission's own "no
// net tool growth on the agent surface" constraint, verified rather than
// merely claimed.
func TestFindingsNear_ToolIsOnInvestigationSurfaceNotAgentSurface(t *testing.T) {
	srv := mcp.NewServer(&extendedFakeBackend{nodes: findingsNodes(), services: []string{"svc"}})

	bare := roundtrip(t, srv, request{JSONRPC: "2.0", ID: 1, Method: "tools/list"})
	if !containsName(toolNames(t, bare), "findings_near") {
		t.Error("findings_near missing from the bare (full) registry")
	}

	investigationCtx := mcp.WithRequestScope(context.Background(), "investigation", true)
	inv := scopedRoundtrip(t, srv, investigationCtx, request{JSONRPC: "2.0", ID: 2, Method: "tools/list"})
	if !containsName(toolNames(t, inv), "findings_near") {
		t.Error("findings_near missing from the \"investigation\" named surface")
	}

	for _, surface := range []string{mcp.SurfaceAgent, mcp.SurfaceCompanion} {
		ctx := mcp.WithRequestScope(context.Background(), surface, true)
		resp := scopedRoundtrip(t, srv, ctx, request{JSONRPC: "2.0", ID: 3, Method: "tools/list"})
		if containsName(toolNames(t, resp), "findings_near") {
			t.Errorf("findings_near must NOT be on the %q surface (no net agent-surface tool growth)", surface)
		}
	}
}

func containsName(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

func TestFindingsNear_RanksByProximityAndLabelsSuperseded(t *testing.T) {
	t0 := time.Now()
	fake := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: findingsNodes(), services: []string{"svc"}},
		claims: map[string]core.SemanticClaim{
			"near": {ClaimID: "near", Layer: "findings", Kind: "finding", Class: core.FindingClassConformance,
				Subject: "site#b", Rule: "near finding", Scope: []string{"pkg/b.go"}, Status: core.ClaimFresh, CompiledAt: t0},
			"far": {ClaimID: "far", Layer: "findings", Kind: "finding", Class: core.FindingClassTestGap,
				Subject: "site#c", Rule: "far finding", Scope: []string{"pkg/c.go"}, Status: core.ClaimFresh, CompiledAt: t0},
			"fixed": {ClaimID: "fixed", Layer: "findings", Kind: "finding", Class: core.FindingClassConformance,
				Subject: "site#a", Rule: "old finding", Scope: []string{"pkg/a.go"}, Status: core.ClaimSuperseded, CompiledAt: t0},
		},
	}
	srv := mcp.NewServer(fake)

	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3, Method: "tools/call",
		Params: map[string]any{
			"name":      "findings_near",
			"arguments": map[string]any{"service": "svc", "subject": "svc::pkg.A", "hops": 2},
		},
	})
	p := extToolPayload(t, resp)
	if p["found"] != true {
		t.Fatalf("found = %v, want true: %v", p["found"], p)
	}
	findings, _ := p["findings"].([]any)
	if len(findings) != 2 {
		t.Fatalf("findings = %v, want 2 (superseded excluded by default)", findings)
	}
	first := findings[0].(map[string]any)
	if first["claim_id"] != "near" {
		t.Errorf("first result = %v, want claim_id=near (closest)", first)
	}

	// With include_superseded, the fixed finding appears too, labeled.
	resp2 := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 4, Method: "tools/call",
		Params: map[string]any{
			"name":      "findings_near",
			"arguments": map[string]any{"service": "svc", "subject": "svc::pkg.A", "hops": 2, "include_superseded": true},
		},
	})
	p2 := extToolPayload(t, resp2)
	findings2, _ := p2["findings"].([]any)
	if len(findings2) != 3 {
		t.Fatalf("findings2 = %v, want 3", findings2)
	}
	var sawSupersededLabel bool
	for _, raw := range findings2 {
		f := raw.(map[string]any)
		if f["claim_id"] == "fixed" {
			sawSupersededLabel = f["superseded"] == true
		}
	}
	if !sawSupersededLabel {
		t.Error("the fixed/superseded finding must be labeled superseded:true when included")
	}
}

func TestFindingsNear_UnresolvableSubject_HonestNotFound(t *testing.T) {
	fake := &reviewFakeBackend{extendedFakeBackend: extendedFakeBackend{nodes: findingsNodes(), services: []string{"svc"}}}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 5, Method: "tools/call",
		Params: map[string]any{
			"name":      "findings_near",
			"arguments": map[string]any{"service": "svc", "subject": "svc::pkg.NoSuch"},
		},
	})
	p := extToolPayload(t, resp)
	if p["found"] != false {
		t.Errorf("found = %v, want false", p["found"])
	}
	if p["note"] == "" || p["note"] == nil {
		t.Error("expected a non-empty honest note")
	}
}

func TestFindingsNear_MissingRequiredArgs_ParamsError(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 6, Method: "tools/call",
		Params: map[string]any{"name": "findings_near", "arguments": map[string]any{"service": "svc"}},
	})
	if resp["error"] == nil {
		t.Fatal("expected an error for a missing subject")
	}
}
