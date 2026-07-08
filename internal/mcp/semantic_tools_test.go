package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// ─── extendedBackend wraps fakeBackend with optional semantic methods ─────────

// semanticFakeBackend adds obligationsBackend + compileBackend + claimsSummaryBackend
// to fakeBackend via composition, for semantic tool tests.
type semanticFakeBackend struct {
	fakeBackend
	obligations      semantic.Obligations
	obligationsErr   error
	indexStale       bool
	compileRun       core.CompileRun
	compileErr       error
	claimsSummary    map[string]any
	claimsSummaryErr error
}

func (s *semanticFakeBackend) Obligations(_ context.Context, service, file string, _ int) (semantic.Obligations, error) {
	return s.obligations, s.obligationsErr
}

func (s *semanticFakeBackend) ObligationsIndexStale(_ context.Context, _, _ string) bool {
	return s.indexStale
}

func (s *semanticFakeBackend) CompileObligations(_ context.Context, _ string, dryRun bool) (core.CompileRun, error) {
	run := s.compileRun
	run.DryRun = dryRun
	return run, s.compileErr
}

func (s *semanticFakeBackend) ClaimsSummary(_ context.Context, _ string) (map[string]any, error) {
	return s.claimsSummary, s.claimsSummaryErr
}

// ─── obligations: empty rules + not stale → compact payload ──────────────────

func TestObligationsTool_EmptyClean(t *testing.T) {
	fake := &semanticFakeBackend{
		obligations: semantic.Obligations{Service: "svc", File: "pkg/foo.go"},
		indexStale:  false,
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "pkg/foo.go"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	if v, _ := payload["index_stale"].(bool); v {
		t.Error("index_stale should be absent/false for a clean file")
	}
	if rules, ok := payload["rules"]; ok && rules != nil {
		t.Errorf("rules should be absent for zero-obligation file, got %v", rules)
	}
}

// ─── obligations: rules + stale flag ────────────────────────────────────────

func TestObligationsTool_WithRules(t *testing.T) {
	fake := &semanticFakeBackend{
		obligations: semantic.Obligations{
			Service: "svc",
			File:    "internal/mcp/tools.go",
			Rules: []semantic.Obligation{
				{Rule: "Sort outputs.", Src: "CLAUDE.md#L10-L10", Tier: "doc-claimed"},
			},
			Watermark: "abc123",
		},
		indexStale: true,
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
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
	if !ok || len(rules) == 0 {
		t.Fatalf("expected rules, got %v", payload["rules"])
	}
	if stale, _ := payload["index_stale"].(bool); !stale {
		t.Error("expected index_stale:true")
	}
	if wm, _ := payload["compiled_at_watermark"].(string); wm == "" {
		t.Error("expected compiled_at_watermark")
	}
}

// ─── obligations: backend missing optional interface → ErrCodeInvalid ────────

func TestObligationsTool_BackendUnsupported(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "foo.go"},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeInvalid) {
		t.Errorf("code = %v, want ErrCodeInvalid (%d)", code, mcp.ErrCodeInvalid)
	}
}

// ─── obligations IS on companion surface ─────────────────────────────────────

func TestObligationsOnCompanionSurface(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	ctx := mcp.WithRequestScope(context.Background(), "companion", true)
	resp := scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 4, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "foo.go"},
		},
	})
	// Should dispatch (not surface-gated), even though this backend lacks the
	// optional interface (returns ErrCodeInvalid, not ErrCodeNotFound).
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		// No error means it dispatched fine (e.g. backed by semanticFakeBackend).
		return
	}
	if code := errObj["code"].(float64); code == float64(mcp.ErrCodeNotFound) {
		t.Errorf("obligations should be on companion surface but got surface-gate error: %v", errObj)
	}
}

// ─── compile_layer NOT on companion surface ───────────────────────────────────

func TestCompileLayerNotOnCompanionSurface(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	ctx := mcp.WithRequestScope(context.Background(), "companion", true)
	resp := scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 5, Method: "tools/call",
		Params: map[string]any{
			"name":      "compile_layer",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("compile_layer on companion: want surface-gate error, got success")
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeNotFound) {
		t.Errorf("compile_layer companion gate: code = %v, want ErrCodeNotFound (%d)", code, mcp.ErrCodeNotFound)
	}
}

// ─── compile_layer: dry_run defaults true ────────────────────────────────────

func TestCompileLayerDryRunDefault(t *testing.T) {
	fake := &semanticFakeBackend{
		compileRun: core.CompileRun{
			ServiceID:     "svc",
			Layer:         "obligations",
			SectionsTotal: 2,
			TokensIn:      500,
		},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 6, Method: "tools/call",
		Params: map[string]any{
			"name":      "compile_layer",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	if dr, _ := payload["dry_run"].(bool); !dr {
		t.Errorf("dry_run should default true, got %v", payload["dry_run"])
	}
}

// ─── compile_layer: wrong layer rejected ─────────────────────────────────────

func TestCompileLayerUnknownLayer(t *testing.T) {
	fake := &semanticFakeBackend{}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 7, Method: "tools/call",
		Params: map[string]any{
			"name":      "compile_layer",
			"arguments": map[string]any{"service": "svc", "layer": "hypotheses"},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error for unknown layer, got %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeInvalid) {
		t.Errorf("unknown layer code = %v, want ErrCodeInvalid", code)
	}
}

// ─── staleness_report appends layers when claimsSummaryBackend present ────────

func TestStalenessReportWithLayers(t *testing.T) {
	fake := &semanticFakeBackend{
		claimsSummary: map[string]any{
			"obligations": map[string]any{"fresh": 3},
		},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 8, Method: "tools/call",
		Params: map[string]any{
			"name":      "staleness_report",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("staleness_report error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	layers, ok := payload["layers"]
	if !ok {
		t.Fatalf("staleness_report: expected layers field, got %v", payload)
	}
	_ = layers
}

// ─── staleness_report: no layers when backend lacks interface ─────────────────

func TestStalenessReportWithoutLayers(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 9, Method: "tools/call",
		Params: map[string]any{
			"name":      "staleness_report",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("staleness_report error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	if _, ok := payload["layers"]; ok {
		t.Error("staleness_report: unexpected layers field on plain fakeBackend")
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func extractPayload(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result missing: %v", resp)
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("content missing: %v", result)
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v (text=%q)", err, text)
	}
	return payload
}
