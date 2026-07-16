package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
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
	gotCompileLayer  string // records the layer name CompileLayer was called with
}

func (s *semanticFakeBackend) Obligations(_ context.Context, service, file string, _ int) (semantic.Obligations, error) {
	return s.obligations, s.obligationsErr
}

func (s *semanticFakeBackend) ObligationsIndexStale(_ context.Context, _, _ string) bool {
	return s.indexStale
}

func (s *semanticFakeBackend) CompileLayer(_ context.Context, _, layer string, dryRun bool) (core.CompileRun, error) {
	s.gotCompileLayer = layer
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

// ─── compile_layer off the companion surface: execute-and-flag ────────────────

func TestCompileLayerNotOnCompanionSurface(t *testing.T) {
	fake := &semanticFakeBackend{
		compileRun: core.CompileRun{ServiceID: "svc", Layer: "obligations", SectionsTotal: 2, TokensIn: 500},
	}
	var buf bytes.Buffer
	srv := mcp.NewServerWithLogger(fake, mcp.NewCallLogger(&buf))
	ctx := mcp.WithRequestScope(context.Background(), "companion", true)

	// compile_layer is off the companion allowlist (full surface only, per
	// docs/mcp-tools.md) but execute-and-flags rather than being rejected
	// (ADR-014 amendment): it runs, and the call log carries off_surface:true.
	resp := scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 5, Method: "tools/call",
		Params: map[string]any{
			"name":      "compile_layer",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("compile_layer off companion should execute-and-flag, got error: %v", resp["error"])
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &entry); err != nil {
		t.Fatalf("log line parse: %v (buf=%q)", err, buf.String())
	}
	if off, _ := entry["off_surface"].(bool); !off {
		t.Errorf("compile_layer off-surface call should log off_surface:true, got %v", entry)
	}

	// Hard gating stays available behind strictSurfaces for any future
	// multi-tenant shape.
	srv.SetStrictSurfaces(true)
	strict := scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 6, Method: "tools/call",
		Params: map[string]any{
			"name":      "compile_layer",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	errObj, ok := strict["error"].(map[string]any)
	if !ok {
		t.Fatalf("compile_layer under strictSurfaces: want surface-gate error, got success")
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeNotFound) {
		t.Errorf("compile_layer strictSurfaces gate: code = %v, want ErrCodeNotFound (%d)", code, mcp.ErrCodeNotFound)
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
			// F5: a budget-stopped run returns nil error — the tool result
			// must carry the stop, or it is indistinguishable from success.
			BudgetStopped: true,
			RefusalReason: "token total 700 exceeds layer \"obligations\" budget max_tokens_per_run=600",
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
	if bs, _ := payload["budget_stopped"].(bool); !bs {
		t.Errorf("budget_stopped should surface true, got %v", payload["budget_stopped"])
	}
	if rr, _ := payload["refusal_reason"].(string); rr == "" {
		t.Errorf("refusal_reason should surface, got %v", payload["refusal_reason"])
	}
	if _, present := payload["refused"]; present {
		t.Errorf("refused should be omitted when false, got %v", payload["refused"])
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

// ─── compile_layer: the requested layer name actually reaches the backend ────

// TestCompileLayerRoutesRequestedLayerName is the F7 regression (docs/plans/
// findings-tracer.md FT-1): compile_layer validated the requested layer name
// via semantic.LoadLayer but then discarded it, always compiling
// "obligations" regardless of what was asked for. The backend must now
// receive the exact layer the caller named.
func TestCompileLayerRoutesRequestedLayerName(t *testing.T) {
	fake := &semanticFakeBackend{compileRun: core.CompileRun{ServiceID: "svc", Layer: "findings"}}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "compile_layer",
			"arguments": map[string]any{"service": "svc", "layer": "findings"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	if fake.gotCompileLayer != "findings" {
		t.Errorf("backend received layer %q, want %q (F7 regression)", fake.gotCompileLayer, "findings")
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

// ─── regression O7: capabilities must survive the auto-refresh wrapper ───────

// TestSemanticTools_ThroughAutoRefreshWrapper exercises the semantic tools the
// way the DAEMON actually serves them — behind mcp.NewAutoRefresh. Interface
// embedding does not forward methods outside QueryInterface's method set, so
// without explicit forwards on the wrapper every semantic capability was
// silently stripped in production while direct-backend tests stayed green
// (and the edit-hook's silent-on-error contract hid it — dogfood O7).
func TestSemanticTools_ThroughAutoRefreshWrapper(t *testing.T) {
	fake := &semanticFakeBackend{
		obligations: semantic.Obligations{
			Service: "svc", File: "pkg/foo.go",
			Rules: []semantic.Obligation{{Rule: "Sort outputs.", Src: "d.md#L1-L2", Tier: "doc-claimed"}},
		},
		compileRun:    core.CompileRun{ServiceID: "svc", Layer: "obligations", SectionsTotal: 3},
		claimsSummary: map[string]any{"obligations": map[string]any{"fresh": 1}},
	}
	srv := mcp.NewServer(mcp.NewAutoRefresh(fake, nil))

	// obligations
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "pkg/foo.go"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("obligations through wrapper: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	if rules, ok := payload["rules"].([]any); !ok || len(rules) != 1 {
		t.Errorf("obligations rules through wrapper = %v", payload["rules"])
	}

	// compile_layer (dry run default)
	resp = roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: map[string]any{
			"name":      "compile_layer",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("compile_layer through wrapper: %v", resp["error"])
	}

	// staleness_report layers section
	resp = roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3, Method: "tools/call",
		Params: map[string]any{
			"name":      "staleness_report",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("staleness_report through wrapper: %v", resp["error"])
	}
	payload = extractPayload(t, resp)
	if payload["layers"] == nil {
		t.Error("staleness_report through wrapper lost the layers summary")
	}
}

// ─── ingest tool: onboarding a new service by path ────────────────────────────

type ingestAtFake struct {
	fakeBackend
	gotService, gotPath string
}

func (f *ingestAtFake) IngestServiceAt(_ context.Context, service, path string) error {
	f.gotService, f.gotPath = service, path
	return nil
}

// TestIngestTool_WithPath_ThroughWrapper: the ingest tool's path argument must
// reach the backend through the auto-refresh wrapper (same O7 class as the
// semantic capabilities).
func TestIngestTool_WithPath_ThroughWrapper(t *testing.T) {
	fake := &ingestAtFake{}
	srv := mcp.NewServer(mcp.NewAutoRefresh(fake, nil))
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "ingest",
			"arguments": map[string]any{"service": "newrepo", "path": "/abs/corpus"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("ingest with path through wrapper: %v", resp["error"])
	}
	if fake.gotService != "newrepo" || fake.gotPath != "/abs/corpus" {
		t.Errorf("backend got (%q, %q), want (newrepo, /abs/corpus)", fake.gotService, fake.gotPath)
	}
}

// TestIngestTool_WithPath_Unsupported: a backend without the capability gets a
// clear error, not a silent re-ingest of the wrong directory.
func TestIngestTool_WithPath_Unsupported(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "ingest",
			"arguments": map[string]any{"service": "newrepo", "path": "/abs/corpus"},
		},
	})
	if resp["error"] == nil {
		t.Fatal("expected error for path-onboarding on an unsupported backend")
	}
}
