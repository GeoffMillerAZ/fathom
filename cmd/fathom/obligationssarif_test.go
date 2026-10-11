package main

// obligationssarif_test.go — GL-20 (docs/plans/governance-learning.md,
// design doc §8 "SARIF"): obligations and review findings render through
// the shared emitter (verify.RenderSARIFRun over PC-21's renderSARIF) with
// their own rule vocabularies, never a second emitter. Review findings take
// the change risk receipt's one converter (`fathom review --receipt`,
// EVX-3); the CLI-only reviewFindingsSARIF is gone (EVX-4).

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/receipt"
	riskreview "github.com/geoffmilleraz/fathom/internal/review"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

func TestObligationsAndReviewSARIF(t *testing.T) {
	obligations := []semantic.Obligation{
		{Rule: "no bare goroutines", Src: "internal/CLAUDE.md#L10-L12", Tier: "compiled"},
		{Rule: "raw fallback rule", Src: "internal/semantic/CLAUDE.md#L1-L3", Tier: "raw", Why: "coverage gap"},
	}
	out := obligationsSARIF(obligations)
	if len(out) != 2 {
		t.Fatalf("obligationsSARIF: got %d results, want 2", len(out))
	}
	if out[0].RuleID != "compiled" || out[0].File != "internal/CLAUDE.md" || out[0].Line != 10 || out[0].EndLine != 12 {
		t.Fatalf("compiled obligation SARIF wrong: %+v", out[0])
	}
	if out[1].RuleID != "raw" || !strings.Contains(out[1].Message, "coverage gap") {
		t.Fatalf("raw obligation SARIF wrong: %+v", out[1])
	}
	if out[0].Help == "" {
		t.Error("an obligation result carries no rule help")
	}
	rendered := verify.RenderSARIFRun("fathom obligations", verify.RunProvenance{}, out)
	if !strings.Contains(string(rendered), `"compiled"`) || !strings.Contains(string(rendered), `"raw"`) {
		t.Fatalf("obligations SARIF missing its tier rule ids: %s", rendered)
	}

	// Review findings: the change risk receipt's fired factors, through
	// the one converter, under their own vocabulary (the factor names).
	r := riskreview.BuildRiskReceipt(riskreview.Input{
		ChangedFiles:          []string{"internal/foo/foo.go"},
		CertificatesAffected:  []riskreview.CertificateRef{{Subject: "foo.Build", State: "broken"}},
		RulesTouched:          []riskreview.RuleRef{{File: "internal/foo/foo.go", Rule: "layer skip", Level: riskreview.RuleLevelWarn}},
		ConservationUnexcused: false,
	}, riskreview.DefaultRiskTable())
	var buf bytes.Buffer
	if code := cmdReviewReceipt(&buf, "sarif", receipt.Run{}, r); code != 0 {
		t.Fatalf("cmdReviewReceipt exit %d", code)
	}
	var doc struct {
		Runs []struct {
			Results []struct {
				RuleID string `json:"ruleId"`
				Level  string `json:"level"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil || len(doc.Runs) != 1 {
		t.Fatalf("review receipt SARIF (%v): %s", err, buf.Bytes())
	}
	levels := map[string]string{}
	for _, res := range doc.Runs[0].Results {
		levels[res.RuleID] = res.Level
	}
	if levels[riskreview.FactorCertificates] != "error" || levels[riskreview.FactorRulesTouched] != "warning" {
		t.Fatalf("review findings SARIF = %v, want certificates_affected at error, rules_touched at warning", levels)
	}
}
