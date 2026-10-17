package mcp_test

// stale_withheld_layer_test.go — O1122 (I38): the stale-section withhold
// judges each served rule against the coverage of the layer that compiled
// it, so an adr-rules claim from a section the adr-rules layer reports
// stale is withheld even when the obligations layer's coverage is silent,
// and a rule fresh in its own layer is not withheld by another layer's
// staleness.

import (
	"context"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// perLayerCovFakeBackend reports a coverage per layer name; a layer absent
// from covs answers found:false. layersRead records every per-layer read.
type perLayerCovFakeBackend struct {
	rawFallbackFakeBackend
	covs       map[string]semantic.LayerCoverage
	layersRead []string
}

func (p *perLayerCovFakeBackend) ObligationsLayerCoverage(context.Context, string) (semantic.LayerCoverage, bool, error) {
	cov, ok := p.covs[semantic.LayerObligations]
	return cov, ok, nil
}

func (p *perLayerCovFakeBackend) LayerCoverage(_ context.Context, _, layer string) (semantic.LayerCoverage, bool, error) {
	p.layersRead = append(p.layersRead, layer)
	cov, ok := p.covs[layer]
	return cov, ok, nil
}

func obligationsCall(t *testing.T, backend mcp.QueryInterface) map[string]any {
	t.Helper()
	return roundtrip(t, mcp.NewServer(backend), request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{"name": "obligations", "arguments": map[string]any{"service": "svc", "file": "go.mod"}},
	})
}

func servedRuleTexts(payload map[string]any) []string {
	rules, _ := payload["rules"].([]any)
	var out []string
	for _, r := range rules {
		m, _ := r.(map[string]any)
		s, _ := m["rule"].(string)
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func staleWithheld(payload map[string]any) any {
	cov, _ := payload["layer_coverage"].(map[string]any)
	return cov["stale_withheld"]
}

func freshObligationsCov() semantic.LayerCoverage {
	return semantic.LayerCoverage{SectionsTotal: 1, SectionsCompiled: 1}
}

// (a) a docs/design section is in the adr-rules corpus only, so the
// obligations coverage never names it: the adr-rules coverage must.
func TestObligationsTool_AdrRulesDesignSectionStaleWithheld(t *testing.T) {
	fake := &perLayerCovFakeBackend{
		rawFallbackFakeBackend: rawFallbackFakeBackend{semanticFakeBackend: semanticFakeBackend{
			obligations: semantic.Obligations{Service: "svc", File: "go.mod", Rules: []semantic.Obligation{
				{Rule: "Design rule.", Src: "docs/design/x.md#L4-L6", Tier: "doc-claimed", Layer: "adr-rules", Subject: "docs/design/x.md#sec"},
				{Rule: "Fresh rule.", Src: "AGENTS.md#L5-L6", Tier: "doc-claimed", Layer: "obligations", Subject: "AGENTS.md#gate"},
			}},
		}},
		covs: map[string]semantic.LayerCoverage{
			semantic.LayerObligations: freshObligationsCov(),
			"adr-rules":               {SectionsTotal: 2, SectionsCompiled: 1, UncompiledReasons: map[string]string{"docs/design/x.md#sec": core.UncompiledStale}},
		},
	}
	payload := extractPayload(t, obligationsCall(t, fake))
	if got := servedRuleTexts(payload); len(got) != 1 || got[0] != "Fresh rule." {
		t.Fatalf("served %v, want only the fresh rule: the adr-rules design claim is stale in its own layer", got)
	}
	if n, _ := staleWithheld(payload).(float64); n != 1 {
		t.Errorf("layer_coverage.stale_withheld = %v, want 1", staleWithheld(payload))
	}
}

// (b) ADR.md is in both corpora: an adr-rules claim is judged against the
// adr-rules compile of that section, not the obligations compile.
func TestObligationsTool_AdrRulesStaleObligationsFreshWithheld(t *testing.T) {
	fake := &perLayerCovFakeBackend{
		rawFallbackFakeBackend: rawFallbackFakeBackend{semanticFakeBackend: semanticFakeBackend{
			obligations: semantic.Obligations{Service: "svc", File: "go.mod", Rules: []semantic.Obligation{
				{Rule: "ADR rule.", Src: "ADR.md#L650-L651", Tier: "doc-claimed", Layer: "adr-rules", Subject: "ADR.md#adr-019"},
				{Rule: "Fresh rule.", Src: "AGENTS.md#L5-L6", Tier: "doc-claimed", Layer: "obligations", Subject: "AGENTS.md#gate"},
			}},
		}},
		covs: map[string]semantic.LayerCoverage{
			semantic.LayerObligations: freshObligationsCov(),
			"adr-rules":               {SectionsTotal: 1, UncompiledReasons: map[string]string{"ADR.md#adr-019": core.UncompiledStale}},
		},
	}
	payload := extractPayload(t, obligationsCall(t, fake))
	if got := servedRuleTexts(payload); len(got) != 1 || got[0] != "Fresh rule." {
		t.Fatalf("served %v, want only the fresh rule", got)
	}
	if n, _ := staleWithheld(payload).(float64); n != 1 {
		t.Errorf("layer_coverage.stale_withheld = %v, want 1", staleWithheld(payload))
	}
}

// (c) no over-withhold: a rule fresh in its own layer is served even when
// another layer reports the same section stale; the other layer's own stale
// rule is still withheld.
func TestObligationsTool_RuleFreshInOwnLayerServed(t *testing.T) {
	fake := &perLayerCovFakeBackend{
		rawFallbackFakeBackend: rawFallbackFakeBackend{semanticFakeBackend: semanticFakeBackend{
			obligations: semantic.Obligations{Service: "svc", File: "go.mod", Rules: []semantic.Obligation{
				{Rule: "ADR rule.", Src: "ADR.md#L650-L651", Tier: "doc-claimed", Layer: "adr-rules", Subject: "ADR.md#adr-019"},
				{Rule: "Stale obligations rule.", Src: "ADR.md#L650-L651", Tier: "doc-claimed", Layer: "obligations", Subject: "ADR.md#adr-019"},
			}},
		}},
		covs: map[string]semantic.LayerCoverage{
			semantic.LayerObligations: {SectionsTotal: 1, UncompiledReasons: map[string]string{"ADR.md#adr-019": core.UncompiledStale}},
			"adr-rules":               {SectionsTotal: 1, SectionsCompiled: 1},
		},
	}
	payload := extractPayload(t, obligationsCall(t, fake))
	if got := servedRuleTexts(payload); len(got) != 1 || got[0] != "ADR rule." {
		t.Fatalf("served %v, want only the adr-rules rule (fresh in adr-rules)", got)
	}
	if n, _ := staleWithheld(payload).(float64); n != 1 {
		t.Errorf("layer_coverage.stale_withheld = %v, want 1", staleWithheld(payload))
	}
}

// (d) with no stale coverage anywhere, reading the adr-rules coverage
// changes nothing: the receipt is byte-identical to the one a backend that
// reports only the obligations coverage renders.
func TestObligationsTool_NoStaleCoverageReceiptBytesUnchanged(t *testing.T) {
	rules := []semantic.Obligation{
		{Rule: "ADR rule.", Src: "ADR.md#L650-L651", Tier: "doc-claimed", Layer: "adr-rules", Subject: "ADR.md#adr-019"},
		{Rule: "Fresh rule.", Src: "AGENTS.md#L5-L6", Tier: "doc-claimed", Layer: "obligations", Subject: "AGENTS.md#gate"},
	}
	oblCov := semantic.LayerCoverage{SectionsTotal: 3, SectionsCompiled: 3}
	perLayer := &perLayerCovFakeBackend{
		rawFallbackFakeBackend: rawFallbackFakeBackend{semanticFakeBackend: semanticFakeBackend{
			obligations: semantic.Obligations{Service: "svc", File: "go.mod", Rules: rules},
		}},
		covs: map[string]semantic.LayerCoverage{
			semantic.LayerObligations: oblCov,
			"adr-rules":               {SectionsTotal: 2, SectionsCompiled: 2, UncompiledReasons: map[string]string{"ADR.md#later": core.UncompiledBudget}},
		},
	}
	oneLayer := &staleCovFakeBackend{
		rawFallbackFakeBackend: rawFallbackFakeBackend{semanticFakeBackend: semanticFakeBackend{
			obligations: semantic.Obligations{Service: "svc", File: "go.mod", Rules: rules},
		}},
		cov: oblCov,
	}
	got := receiptText(t, obligationsCall(t, perLayer))
	want := receiptText(t, obligationsCall(t, oneLayer))
	if len(perLayer.layersRead) != 1 || perLayer.layersRead[0] != "adr-rules" {
		t.Fatalf("per-layer coverage reads = %v, want exactly [adr-rules] (the one non-obligations layer served)", perLayer.layersRead)
	}
	if got != want {
		t.Errorf("receipt bytes changed with no stale coverage anywhere:\n got: %s\nwant: %s", got, want)
	}
}

func receiptText(t *testing.T, resp map[string]any) string {
	t.Helper()
	result, _ := resp["result"].(map[string]any)
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("content missing: %v", resp)
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	return text
}
