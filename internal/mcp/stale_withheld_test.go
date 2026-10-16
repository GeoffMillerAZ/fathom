package mcp_test

// stale_withheld_test.go — I38: a compiled claim whose source section the
// layer itself reports stale is not served as a rule; it is counted in
// layer_coverage.stale_withheld, and fresh claims are unchanged.

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

type staleCovFakeBackend struct {
	rawFallbackFakeBackend
	cov semantic.LayerCoverage
}

func (s *staleCovFakeBackend) ObligationsLayerCoverage(context.Context, string) (semantic.LayerCoverage, bool, error) {
	return s.cov, true, nil
}

func TestObligationsTool_StaleSectionClaimWithheld(t *testing.T) {
	fake := &staleCovFakeBackend{
		rawFallbackFakeBackend: rawFallbackFakeBackend{
			semanticFakeBackend: semanticFakeBackend{
				obligations: semantic.Obligations{
					Service: "svc", File: "go.mod",
					Rules: []semantic.Obligation{
						{Rule: "go.mod carries replace.", Src: "ADR.md#L650-L651", Tier: "doc-claimed", Layer: "adr-rules", Subject: "ADR.md#adr-019"},
						{Rule: "Fresh rule.", Src: "AGENTS.md#L5-L6", Tier: "doc-claimed", Layer: "obligations", Subject: "AGENTS.md#gate"},
					},
				},
			},
			rawRows: []semantic.Obligation{
				{Rule: "ADR-019 as amended.", Src: "ADR.md#L650-L660", Tier: "raw", Layer: "obligations", Why: "stale section withheld"},
			},
			rawOK: true,
		},
		cov: semantic.LayerCoverage{
			SectionsTotal: 2, SectionsCompiled: 1,
			UncompiledScopes:  []string{"ADR.md"},
			UncompiledReasons: map[string]string{"ADR.md#adr-019": core.UncompiledStale, "ADR.md#new": core.UncompiledCorpusGrew},
		},
	}
	resp := roundtrip(t, mcp.NewServer(fake), request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{"name": "obligations", "arguments": map[string]any{"service": "svc", "file": "go.mod"}},
	})
	payload := extractPayload(t, resp)
	rules, _ := payload["rules"].([]any)
	if len(rules) != 2 {
		t.Fatalf("want raw + fresh = 2 rules, got %v", payload["rules"])
	}
	for _, r := range rules {
		m, _ := r.(map[string]any)
		if m["rule"] == "go.mod carries replace." {
			t.Fatalf("stale-section claim was served as a rule: %v", m)
		}
	}
	if first, _ := rules[0].(map[string]any); first["tier"] != "raw" {
		t.Errorf("rules[0] should be the raw current section, got %v", first)
	}
	cov, _ := payload["layer_coverage"].(map[string]any)
	if n, _ := cov["stale_withheld"].(float64); n != 1 {
		t.Errorf("layer_coverage.stale_withheld = %v, want 1", cov["stale_withheld"])
	}
}

func TestObligationsTool_CorpusGrewClaimNotWithheld(t *testing.T) {
	fake := &staleCovFakeBackend{
		rawFallbackFakeBackend: rawFallbackFakeBackend{
			semanticFakeBackend: semanticFakeBackend{
				obligations: semantic.Obligations{
					Service: "svc", File: "go.mod",
					Rules: []semantic.Obligation{{Rule: "Kept.", Src: "ADR.md#L1-L2", Tier: "doc-claimed", Layer: "adr-rules", Subject: "ADR.md#a"}},
				},
			},
		},
		cov: semantic.LayerCoverage{UncompiledReasons: map[string]string{"ADR.md#a": core.UncompiledBudget}},
	}
	resp := roundtrip(t, mcp.NewServer(fake), request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{"name": "obligations", "arguments": map[string]any{"service": "svc", "file": "go.mod"}},
	})
	payload := extractPayload(t, resp)
	if rules, _ := payload["rules"].([]any); len(rules) != 1 {
		t.Fatalf("budget-reason claim must still be served, got %v", payload["rules"])
	}
	if cov, _ := payload["layer_coverage"].(map[string]any); cov["stale_withheld"] != nil {
		t.Errorf("stale_withheld should be absent, got %v", cov["stale_withheld"])
	}
}
