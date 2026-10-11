package review

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/receipt"
	riskreview "github.com/geoffmilleraz/fathom/internal/review"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// TestChangeRiskReceiptGolden is SR-49's golden test (docs/plans/
// surface-reckoning.md, design §5 row SR-49): changeRiskReceipt renders a
// riskreview.RiskReceipt — the SAME change risk receipt (U7: blast radius,
// certificates, conservation, rules touched, suggested reviewers)
// review_diff already builds via internal/mcp/review_receipt.go's
// reviewDiffReceipt — as SARIF 2.1.0 and a GitHub check-run payload: one
// SARIF result / check annotation per FIRED factor (WeightNone factors are
// not findings), located at the receipt's own first changed file, and the
// check-run's fixed "neutral" conclusion (CX-8/ADR-040: fathom cites
// evidence, never scores pass/fail on its own authority). Both renderings
// are byte-identical on a second render over the same receipt — the
// determinism contract, internal/AGENTS.md.
func TestChangeRiskReceiptGolden(t *testing.T) {
	in := riskreview.Input{
		ChangedFiles:          []string{"internal/widget/widget.go"},
		TransitiveCallers:     120,
		CertificatesAffected:  []riskreview.CertificateRef{{Subject: "widget.Build", State: "broken"}},
		ConservationUnexcused: true,
		ConservationDetail:    "internal/widget/widget.go: 3 unattested lines",
		RulesTouched:          []riskreview.RuleRef{{File: "internal/widget/widget.go", Rule: "no-bare-goroutines", Level: riskreview.RuleLevelError}},
	}
	r := riskreview.BuildRiskReceipt(in, riskreview.DefaultRiskTable())

	sarif, checkRun := changeRiskReceipt(context.Background(), "fathom review_diff", receipt.Run{}, r)

	// SARIF: valid JSON, carries every fired factor, located at the
	// receipt's own first changed file, and omits a factor that never
	// fired (zone_interest has no rate in this fixture).
	var sarifDoc map[string]any
	if err := json.Unmarshal(sarif, &sarifDoc); err != nil {
		t.Fatalf("SARIF is not valid JSON: %v\n%s", err, sarif)
	}
	if !strings.Contains(string(sarif), riskreview.FactorBlastRadius) {
		t.Errorf("SARIF missing fired factor %q: %s", riskreview.FactorBlastRadius, sarif)
	}
	if !strings.Contains(string(sarif), "internal/widget/widget.go") {
		t.Errorf("SARIF missing the receipt's changed-file location: %s", sarif)
	}
	if strings.Contains(string(sarif), riskreview.FactorZoneInterest) {
		t.Errorf("SARIF must not carry a never-fired factor (zone_interest, WeightNone here): %s", sarif)
	}

	// Check-run: valid JSON, "neutral" conclusion always, summary names
	// the band and the fixed authority statement.
	var checkDoc struct {
		Conclusion string `json:"conclusion"`
		Output     struct {
			Summary     string `json:"summary"`
			Annotations []any  `json:"annotations"`
		} `json:"output"`
	}
	if err := json.Unmarshal(checkRun, &checkDoc); err != nil {
		t.Fatalf("check-run is not valid JSON: %v\n%s", err, checkRun)
	}
	if checkDoc.Conclusion != "neutral" {
		t.Errorf("check-run conclusion = %q, want %q (CX-8: fathom never scores pass/fail)", checkDoc.Conclusion, "neutral")
	}
	if !strings.Contains(checkDoc.Output.Summary, string(r.Band)) || !strings.Contains(checkDoc.Output.Summary, r.Authority) {
		t.Errorf("check-run summary = %q, want the band %q and the fixed authority statement %q", checkDoc.Output.Summary, r.Band, r.Authority)
	}
	if len(checkDoc.Output.Annotations) == 0 {
		t.Error("check-run carries no annotations for a receipt with fired factors")
	}

	// Determinism: the SAME receipt renders byte-identical SARIF and
	// check-run on a second call.
	sarif2, checkRun2 := changeRiskReceipt(context.Background(), "fathom review_diff", receipt.Run{}, r)
	if string(sarif) != string(sarif2) {
		t.Error("SARIF rendering is not deterministic across renders")
	}
	if string(checkRun) != string(checkRun2) {
		t.Error("check-run rendering is not deterministic across renders")
	}
}

// TestChangeRiskReceiptSARIF_OneConverterI57 is EVX-3's merged converter:
// changeRiskReceiptSARIF is the one risk-to-SARIF converter the CLI's
// `review --receipt` and review_diff's receipt_sarif both render through,
// with the run's provenance. A cross-boundary link below the typed tier is
// listed as a note with its tier named and never reads level error (I57),
// while a typed cross-service link still fires links_touched at error.
func TestChangeRiskReceiptSARIF_OneConverterI57(t *testing.T) {
	in := riskreview.Input{
		ChangedFiles: []string{"api/handler.go"},
		Links: []riskreview.LinkRef{
			{Provider: "api.Handle", Consumer: "web.call", ConsumerService: "web", Tier: "literal", Citation: "web/call.go#L3-L3"},
			{Provider: "api.Handle", Consumer: "jobs.call", ConsumerService: "jobs", Tier: "template"},
			{Provider: "api.Serve", Consumer: "cli.main", ConsumerService: "cli", Tier: riskreview.LinkTierTyped},
		},
	}
	r := riskreview.BuildRiskReceipt(in, riskreview.DefaultRiskTable())
	results := changeRiskReceiptSARIF(r)

	var hints, typed int
	for _, res := range results {
		switch {
		case strings.Contains(res.Message, "literal") || strings.Contains(res.Message, "template"):
			hints++
			if res.Level == "error" {
				t.Errorf("a result built on a %q link reads level error (I57): %+v", res.Message, res)
			}
			if res.Level != "note" {
				t.Errorf("a sub-typed link renders as a note, got %q", res.Level)
			}
		case res.RuleID == riskreview.FactorLinksTouched:
			typed++
			if res.Level != "error" {
				t.Errorf("a typed cross-service link fires links_touched at error, got %q", res.Level)
			}
			if strings.Contains(res.Message, "web.call") || strings.Contains(res.Message, "jobs.call") {
				t.Errorf("links_touched counts a sub-typed link: %s", res.Message)
			}
		}
	}
	if hints != 2 || typed != 1 {
		t.Fatalf("want 2 tier-named link notes and 1 links_touched result, got %d and %d: %+v", hints, typed, results)
	}

	prov := verify.RunProvenance{Commit: "abc", Clean: true, Producer: verify.Producer{Name: "fathom"}, ExtractorEpoch: "60"}
	sarif := verify.RenderSARIFRun("fathom review_diff", prov, results)
	var doc struct {
		Runs []struct {
			Properties struct {
				Fathom map[string]any `json:"fathom"`
			} `json:"properties"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(sarif, &doc); err != nil || doc.Runs[0].Properties.Fathom["commit"] != "abc" {
		t.Fatalf("RenderSARIFRun over the merged converter's results lost the provenance (%v): %s", err, sarif)
	}
	got, _ := changeRiskReceipt(context.Background(), "fathom review_diff", receipt.Run{}, r)
	if !strings.Contains(string(got), `"extractor_epoch"`) || !strings.Contains(string(got), "link_below_typed_tier") {
		t.Fatalf("changeRiskReceipt does not render through RenderSARIFRun with the merged converter: %s", got)
	}
}
