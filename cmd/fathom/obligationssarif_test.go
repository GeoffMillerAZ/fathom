package main

// obligationssarif_test.go — GL-20 (docs/plans/governance-learning.md,
// design doc §8 "SARIF"): obligations and review findings render through
// verify.RenderSARIF with their own rule vocabularies, reusing the PC-21
// emitter rather than a second one.

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
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
	if out[0].RuleID != "compiled" || out[0].File != "internal/CLAUDE.md" || out[0].Line != 10 {
		t.Fatalf("compiled obligation SARIF wrong: %+v", out[0])
	}
	if out[1].RuleID != "raw" || !strings.Contains(out[1].Message, "coverage gap") {
		t.Fatalf("raw obligation SARIF wrong: %+v", out[1])
	}

	findings := []review.Finding{
		{Kind: "arch_violation", File: "internal/foo/foo.go", Line: 42, Message: "layer skip", Severity: "error"},
		{Kind: "spec_uncovered", File: "internal/bar/bar.go", Line: 7, Message: "no test reaches it", Severity: "warn"},
	}
	rout := reviewFindingsSARIF(findings)
	if len(rout) != 2 {
		t.Fatalf("reviewFindingsSARIF: got %d results, want 2", len(rout))
	}
	if rout[0].RuleID != "arch_violation" || rout[0].Level != "error" || rout[0].File != "internal/foo/foo.go" || rout[0].Line != 42 {
		t.Fatalf("arch finding SARIF wrong: %+v", rout[0])
	}
	if rout[1].RuleID != "spec_uncovered" || rout[1].Level != "warning" {
		t.Fatalf("spec finding SARIF wrong: %+v", rout[1])
	}

	rendered := verify.RenderSARIF("fathom obligations", append(out, rout...))
	if !strings.Contains(string(rendered), "arch_violation") || !strings.Contains(string(rendered), "compiled") {
		t.Fatalf("combined SARIF missing expected rule ids: %s", rendered)
	}
}
