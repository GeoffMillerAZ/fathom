package review

import (
	"encoding/json"
	"strings"
	"testing"

	riskreview "github.com/geoffmilleraz/fathom/internal/review"
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

	sarif, checkRun := changeRiskReceipt("fathom review_diff", r)

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
	sarif2, checkRun2 := changeRiskReceipt("fathom review_diff", r)
	if string(sarif) != string(sarif2) {
		t.Error("SARIF rendering is not deterministic across renders")
	}
	if string(checkRun) != string(checkRun2) {
		t.Error("check-run rendering is not deterministic across renders")
	}
}
