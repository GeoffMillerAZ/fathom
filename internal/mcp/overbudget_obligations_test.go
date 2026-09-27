package mcp_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

type overBudgetObligationsBackend struct {
	semanticFakeBackend
}

func (b *overBudgetObligationsBackend) ObligationsLayerCoverage(context.Context, string) (semantic.LayerCoverage, bool, error) {
	return semantic.LayerCoverage{}, false, fmt.Errorf("layer corpus: %w", tenancy.ProducerAllows(tenancy.ClassTree))
}

func (b *overBudgetObligationsBackend) RawFallbackObligations(context.Context, string, string, int) ([]semantic.Obligation, string, bool, error) {
	return nil, "", false, tenancy.ProducerAllows(tenancy.ClassTree)
}

// RT-153 review finding 2: obligations names a governor-halted corpus read
// (coverage_skipped, layer_coverage status skipped) — never an absent
// coverage block or a silent "no gap".
func TestObligationsTool_OverBudgetIsNamed(t *testing.T) {
	tenancy.PublishLevel(tenancy.LevelRefuse)
	t.Cleanup(func() { tenancy.PublishLevel(tenancy.LevelNormal) })
	fake := &overBudgetObligationsBackend{semanticFakeBackend{
		obligations: semantic.Obligations{Service: "svc", File: "a/b.go"},
	}}
	resp := roundtrip(t, mcp.NewServer(fake), request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{"name": "obligations", "arguments": map[string]any{"service": "svc", "file": "a/b.go"}},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	skipped, _ := payload["coverage_skipped"].(string)
	if !strings.Contains(skipped, "over budget") {
		t.Fatalf("coverage_skipped = %q, want the over-budget halt named", skipped)
	}
	cov, ok := payload["layer_coverage"].(map[string]any)
	if !ok || cov["status"] != "skipped" {
		t.Fatalf("layer_coverage = %#v, want status skipped", payload["layer_coverage"])
	}
}
