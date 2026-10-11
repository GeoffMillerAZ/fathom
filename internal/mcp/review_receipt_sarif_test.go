package mcp_test

// review_receipt_sarif_test.go — EVX-3 (docs/plans/portable-receipts.md):
// review_diff's receipt_sarif renders through the one risk converter with
// the run's provenance. Its key is unchanged (P4); its content grows.

import (
	"encoding/json"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

func TestReviewDiffReceiptSARIFCarriesProvenance(t *testing.T) {
	fake := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}},
		obligations: map[string]semantic.Obligations{
			"pkg/a.go": {
				Service: "svc", File: "pkg/a.go",
				Rules: []semantic.Obligation{{Rule: "Sort outputs.", Src: "CLAUDE.md#L1-L1", Tier: "doc-claimed", Status: "contradicted"}},
			},
		},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 5, Method: "tools/call",
		Params: map[string]any{
			"name":      "review_diff",
			"arguments": map[string]any{"service": "svc", "changed_files": []string{"pkg/a.go"}},
		},
	})
	p := extToolPayload(t, resp)
	raw, ok := p["receipt_sarif"]
	if !ok {
		t.Fatal("review_diff payload missing \"receipt_sarif\" (its key must not change)")
	}
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Runs []struct {
			AutomationDetails struct {
				ID string `json:"id"`
			} `json:"automationDetails"`
			Properties struct {
				Fathom struct {
					ExtractorEpoch string `json:"extractor_epoch"`
					Producer       struct {
						Name string `json:"name"`
					} `json:"producer"`
				} `json:"fathom"`
			} `json:"properties"`
			Results []struct {
				RuleID string `json:"ruleId"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(b, &doc); err != nil || len(doc.Runs) != 1 {
		t.Fatalf("receipt_sarif is not a one-run SARIF log (%v): %s", err, b)
	}
	run := doc.Runs[0]
	// The fake backend has no corpus root: no commit, so the id is the
	// category alone.
	if run.AutomationDetails.ID != "fathom-review-diff/" {
		t.Errorf("automationDetails.id = %q, want \"fathom-review-diff/\"", run.AutomationDetails.ID)
	}
	if run.Properties.Fathom.ExtractorEpoch != fusion.ExtractorEpoch || run.Properties.Fathom.Producer.Name != "fathom" {
		t.Errorf("run.properties.fathom = %+v, want the epoch and producer", run.Properties.Fathom)
	}
	if len(run.Results) == 0 || run.Results[0].RuleID != "rules_touched" {
		t.Errorf("results = %+v, want the fired rules_touched factor", run.Results)
	}
}
