package mcp_test

// sourcesbusy_tool_test.go — RT-146's named skip on the hook-reachable MCP
// paths (docs/design/responsible-tenant.md §4.6, I38): when a hook-origin
// whole-tree read is refused (core.ErrSourcesBusy), obligations names the
// skip instead of rendering an absent coverage block or dropping its raw
// fallback rows, and verify_plan refuses by name instead of failing every
// spec-backed row against an empty matrix.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

type busyObligationsBackend struct {
	semanticFakeBackend
}

func (b *busyObligationsBackend) ObligationsLayerCoverage(context.Context, string) (semantic.LayerCoverage, bool, error) {
	return semantic.LayerCoverage{}, false, fmt.Errorf("layer coverage: %w", core.ErrSourcesBusy)
}

func (b *busyObligationsBackend) RawFallbackObligations(context.Context, string, string, int) ([]semantic.Obligation, string, bool, error) {
	return nil, "", false, core.ErrSourcesBusy
}

func TestObligationsTool_SourcesBusyIsNamed(t *testing.T) {
	fake := &busyObligationsBackend{semanticFakeBackend{
		obligations: semantic.Obligations{Service: "svc", File: "a/b.go"},
	}}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "a/b.go"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	const want = "coverage skipped: whole-tree read in progress"
	if payload["coverage_skipped"] != want {
		t.Fatalf("coverage_skipped = %v, want %q", payload["coverage_skipped"], want)
	}
	cov, ok := payload["layer_coverage"].(map[string]any)
	if !ok || cov["status"] != "skipped" || cov["note"] != want {
		t.Fatalf("layer_coverage = %#v, want {status: skipped, note: %q}", payload["layer_coverage"], want)
	}
}

type busySourcesVerifyBackend struct {
	*verifyFakeBackend
}

func (b busySourcesVerifyBackend) Sources(context.Context, string) ([]core.SourceFile, error) {
	return nil, core.ErrSourcesBusy
}

func TestVerifyPlan_SourcesBusyRefusesByName(t *testing.T) {
	srv := mcp.NewServer(busySourcesVerifyBackend{newVerifyBackend()})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name": "verify_plan",
			"arguments": map[string]any{
				"service": "svc",
				"tasks": []any{
					map[string]any{"id": "T-1", "title": "spec", "symbol": "DoWork", "spec_ids": []any{"SPEC-1"}},
				},
			},
		},
	})
	errObj, _ := resp["error"].(map[string]any)
	if errObj == nil {
		t.Fatalf("verify_plan with a skipped source read = %v, want a named refusal (an empty spec matrix would fail the row falsely)", resp["result"])
	}
	if msg, _ := errObj["message"].(string); !strings.Contains(msg, "coverage skipped: whole-tree read in progress") {
		t.Fatalf("error message = %q, want the named skip", msg)
	}

	// Without spec IDs the source read is never needed: the call answers.
	resp = roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: map[string]any{
			"name": "verify_plan",
			"arguments": map[string]any{
				"service": "svc",
				"tasks":   []any{map[string]any{"id": "T-1", "title": "do", "symbol": "DoWork"}},
			},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("verify_plan without specs = %v, want an answer", resp["error"])
	}
}
