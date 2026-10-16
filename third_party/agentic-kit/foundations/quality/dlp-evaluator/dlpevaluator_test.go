package dlpevaluator_test

import (
	"testing"

	dlpevaluator "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/dlp-evaluator"
)

// genericRules is a small, deterministic rule set for use across tests.
// These patterns are generic and carry no org-specific literals.
func genericRules() []dlpevaluator.DLPRule {
	return []dlpevaluator.DLPRule{
		{
			ID:          "DLP-001",
			Pattern:     `(?i)\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`,
			RiskBand:    dlpevaluator.RiskMedium,
			Description: "Email address pattern.",
		},
		{
			ID:          "DLP-002",
			Pattern:     `(?i)\b(password|passwd|secret|api[_\-]?key|token)\s*[:=]\s*\S+`,
			RiskBand:    dlpevaluator.RiskHigh,
			Description: "Credential or secret assignment.",
		},
		{
			ID:          "DLP-003",
			Pattern:     `(?i)\bconfidential\b`,
			RiskBand:    dlpevaluator.RiskLow,
			Description: "Document marked confidential.",
		},
	}
}

func TestEvaluateCleanTextReturnsNoneRisk(t *testing.T) {
	t.Parallel()

	report, err := dlpevaluator.Evaluate("This text has no sensitive content.", genericRules())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if report.MaxRisk != dlpevaluator.RiskNone {
		t.Errorf("expected MaxRisk=none, got %q", report.MaxRisk)
	}
	if len(report.Findings) != 0 {
		t.Errorf("expected no findings, got %d", len(report.Findings))
	}
	if report.Score != 0.0 {
		t.Errorf("expected Score=0.0, got %f", report.Score)
	}
}

func TestEvaluateEmailDetected(t *testing.T) {
	t.Parallel()

	report, err := dlpevaluator.Evaluate("Contact alice@example.com for help.", genericRules())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if report.MaxRisk != dlpevaluator.RiskMedium {
		t.Errorf("expected MaxRisk=medium, got %q", report.MaxRisk)
	}
	if len(report.Findings) == 0 {
		t.Fatal("expected at least one finding")
	}
	if report.Findings[0].RuleID != "DLP-001" {
		t.Errorf("expected finding RuleID=DLP-001, got %q", report.Findings[0].RuleID)
	}
	if report.Score != 0.67 {
		t.Errorf("expected Score=0.67 for medium, got %f", report.Score)
	}
}

func TestEvaluateCredentialDetected(t *testing.T) {
	t.Parallel()

	report, err := dlpevaluator.Evaluate("password=supersecret123", genericRules())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if report.MaxRisk != dlpevaluator.RiskHigh {
		t.Errorf("expected MaxRisk=high, got %q", report.MaxRisk)
	}
	if report.Score != 1.0 {
		t.Errorf("expected Score=1.0 for high, got %f", report.Score)
	}
}

func TestEvaluateHighRiskBeatsLower(t *testing.T) {
	t.Parallel()

	// Text contains both a confidential marker (low) and a credential (high).
	text := "This is confidential. secret: abc123"
	report, err := dlpevaluator.Evaluate(text, genericRules())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if report.MaxRisk != dlpevaluator.RiskHigh {
		t.Errorf("expected MaxRisk=high when high and low both fire, got %q", report.MaxRisk)
	}
	if len(report.Findings) < 2 {
		t.Errorf("expected at least 2 findings, got %d", len(report.Findings))
	}
}

func TestEvaluateIsDeterministic(t *testing.T) {
	t.Parallel()

	text := "token=ghp_abc123 sent to user@corp.io"
	rules := genericRules()

	r1, err := dlpevaluator.Evaluate(text, rules)
	if err != nil {
		t.Fatalf("Evaluate first call: %v", err)
	}
	r2, err := dlpevaluator.Evaluate(text, rules)
	if err != nil {
		t.Fatalf("Evaluate second call: %v", err)
	}
	if r1.Score != r2.Score {
		t.Errorf("Score is non-deterministic: %f vs %f", r1.Score, r2.Score)
	}
	if r1.MaxRisk != r2.MaxRisk {
		t.Errorf("MaxRisk is non-deterministic: %q vs %q", r1.MaxRisk, r2.MaxRisk)
	}
	if len(r1.Findings) != len(r2.Findings) {
		t.Errorf("Findings count is non-deterministic: %d vs %d", len(r1.Findings), len(r2.Findings))
	}
}

func TestEvaluateInvalidPatternReturnsError(t *testing.T) {
	t.Parallel()

	bad := []dlpevaluator.DLPRule{
		{ID: "DLP-BAD", Pattern: `(?P<unclosed`, RiskBand: dlpevaluator.RiskLow, Description: "bad"},
	}
	_, err := dlpevaluator.Evaluate("anything", bad)
	if err == nil {
		t.Error("expected error for invalid regexp, got nil")
	}
}

func TestRiskGTE(t *testing.T) {
	t.Parallel()

	ordered := []dlpevaluator.RiskBand{
		dlpevaluator.RiskNone,
		dlpevaluator.RiskLow,
		dlpevaluator.RiskMedium,
		dlpevaluator.RiskHigh,
	}
	for i := 0; i < len(ordered); i++ {
		if !dlpevaluator.RiskGTE(ordered[i], ordered[i]) {
			t.Errorf("RiskGTE(%q, %q) should be true (reflexive)", ordered[i], ordered[i])
		}
		for j := i + 1; j < len(ordered); j++ {
			if dlpevaluator.RiskGTE(ordered[i], ordered[j]) {
				t.Errorf("RiskGTE(%q, %q) should be false (lower must not be GTE higher)", ordered[i], ordered[j])
			}
			if !dlpevaluator.RiskGTE(ordered[j], ordered[i]) {
				t.Errorf("RiskGTE(%q, %q) should be true (higher must be GTE lower)", ordered[j], ordered[i])
			}
		}
	}
}

func TestNewEvaluatorReusable(t *testing.T) {
	t.Parallel()

	ev, err := dlpevaluator.NewEvaluator(genericRules())
	if err != nil {
		t.Fatalf("NewEvaluator: %v", err)
	}
	r1 := ev.Evaluate("user@example.com")
	r2 := ev.Evaluate("clean text here")
	if r1.MaxRisk != dlpevaluator.RiskMedium {
		t.Errorf("first call: expected medium, got %q", r1.MaxRisk)
	}
	if r2.MaxRisk != dlpevaluator.RiskNone {
		t.Errorf("second call: expected none, got %q", r2.MaxRisk)
	}
}
