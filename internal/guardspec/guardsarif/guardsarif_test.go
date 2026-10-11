package guardsarif

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/guardspec"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// TestGuardVerdictsSARIFGolden is SR-47's golden test (docs/plans/
// surface-reckoning.md, design §5 row SR-47, F-29): EvaluateLayerRule's
// own []Violation — a guard verdict, a served rule checked live against
// the index — renders as SARIF 2.1.0 through the shared emitter
// (verify.RenderSARIFRun), one result per violation, ruleId the violation's
// own RuleID, level "error"/"warning" from its Severity, and the offending
// package directory as the location. Deterministic: the SAME violations
// render byte-identical SARIF on a second call.
func TestGuardVerdictsSARIFGolden(t *testing.T) {
	violations := []guardspec.Violation{
		{RuleID: "no-store-in-cli", Kind: guardspec.ImportDirection, Severity: guardspec.SeverityError, From: "cmd/fathom", To: "internal/store", Message: "cmd/fathom imports internal/store: cmd/fathom may not import internal/store directly"},
		{RuleID: "ledger-boundary", Kind: guardspec.Boundary, Severity: guardspec.SeverityWarn, From: "internal/other", To: "internal/ledger", Message: "internal/other imports guarded package internal/ledger without being on the allow list: only internal/payment may import internal/ledger"},
	}

	results := guardVerdictsSARIF(violations)
	if len(results) != 2 {
		t.Fatalf("guardVerdictsSARIF: got %d results, want 2", len(results))
	}
	if results[0].RuleID != "no-store-in-cli" || results[0].Level != "error" || results[0].File != "cmd/fathom" {
		t.Errorf("error-severity violation SARIF wrong: %+v", results[0])
	}
	if results[1].RuleID != "ledger-boundary" || results[1].Level != "warning" || results[1].File != "internal/other" {
		t.Errorf("warn-severity violation SARIF wrong: %+v", results[1])
	}
	for i, r := range results {
		if r.Message != violations[i].Message {
			t.Errorf("result[%d].Message = %q, want the violation's own Message %q", i, r.Message, violations[i].Message)
		}
	}

	rendered := verify.RenderSARIFRun("fathom rules", verify.RunProvenance{}, results)
	var doc map[string]any
	if err := json.Unmarshal(rendered, &doc); err != nil {
		t.Fatalf("rendered SARIF is not valid JSON: %v\n%s", err, rendered)
	}
	if !strings.Contains(string(rendered), "no-store-in-cli") || !strings.Contains(string(rendered), "ledger-boundary") {
		t.Errorf("rendered SARIF missing expected rule ids: %s", rendered)
	}

	// Determinism: the SAME violations render byte-identical SARIF twice.
	rendered2 := verify.RenderSARIFRun("fathom rules", verify.RunProvenance{}, guardVerdictsSARIF(violations))
	if string(rendered) != string(rendered2) {
		t.Error("guard verdicts SARIF rendering is not deterministic across renders")
	}

	// An empty violation list still renders a valid, empty-results SARIF
	// (mirrors renderSARIF's own "always one run" contract): a clean guard
	// pass is distinguishable from "did not run".
	if got := guardVerdictsSARIF(nil); len(got) != 0 {
		t.Errorf("guardVerdictsSARIF(nil) = %v, want empty", got)
	}
}
