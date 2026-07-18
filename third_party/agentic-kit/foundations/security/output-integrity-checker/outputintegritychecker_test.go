package outputintegritychecker_test

import (
	"testing"

	oic "github.com/geoffmilleraz/agentic-kit/foundations/security/output-integrity-checker"
)

func simplePolicy() oic.IntegrityPolicy {
	return oic.IntegrityPolicy{
		Name:    "test-v1",
		Version: "1.0.0",
		RequiredDisclaimers: []oic.RequiredDisclaimer{
			{ID: "RD-001", Substring: "not professional advice", Description: "general disclaimer"},
		},
		BannedTokens: []oic.BannedToken{
			{ID: "BT-001", Pattern: `(?i)\bconfidential\b`, Description: "must not expose confidential label"},
		},
	}
}

func TestCheckPassesWhenCompliant(t *testing.T) {
	t.Parallel()

	output := "This answer is not professional advice. Please consult an expert."
	report, err := oic.Check(output, simplePolicy())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !report.Passed {
		t.Errorf("expected Passed=true, got violations: %v", report.Violations)
	}
}

func TestCheckFailsMissingDisclaimer(t *testing.T) {
	t.Parallel()

	output := "Here is your answer. It is quite thorough."
	report, err := oic.Check(output, simplePolicy())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if report.Passed {
		t.Error("expected Passed=false (missing disclaimer)")
	}
	found := false
	for _, v := range report.Violations {
		if v.RuleID == "RD-001" && v.Kind == "missing_disclaimer" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected RD-001 missing_disclaimer violation, got: %v", report.Violations)
	}
}

func TestCheckFailsBannedToken(t *testing.T) {
	t.Parallel()

	output := "Not professional advice. This document is CONFIDENTIAL and restricted."
	report, err := oic.Check(output, simplePolicy())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if report.Passed {
		t.Error("expected Passed=false (banned token)")
	}
	found := false
	for _, v := range report.Violations {
		if v.RuleID == "BT-001" && v.Kind == "banned_token" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected BT-001 banned_token violation, got: %v", report.Violations)
	}
}

func TestCheckIsDeterministic(t *testing.T) {
	t.Parallel()

	output := "missing disclaimer and CONFIDENTIAL content"
	r1, err := oic.Check(output, simplePolicy())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	r2, _ := oic.Check(output, simplePolicy())
	if r1.Passed != r2.Passed || len(r1.Violations) != len(r2.Violations) {
		t.Errorf("Check not deterministic: %+v vs %+v", r1, r2)
	}
}

func TestCheckCaseInsensitiveDisclaimer(t *testing.T) {
	t.Parallel()

	output := "This is NOT PROFESSIONAL ADVICE for everyone."
	report, err := oic.Check(output, simplePolicy())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !report.Passed {
		t.Errorf("disclaimer check should be case-insensitive, got violations: %v", report.Violations)
	}
}

func TestNewCheckerRejectsInvalidPattern(t *testing.T) {
	t.Parallel()

	bad := oic.IntegrityPolicy{
		Name: "bad", Version: "0.0.1",
		BannedTokens: []oic.BannedToken{
			{ID: "BT-BAD", Pattern: `(?P<unclosed`, Description: "bad"},
		},
	}
	_, err := oic.NewChecker(bad)
	if err == nil {
		t.Error("expected error for invalid pattern, got nil")
	}
}

func TestEmptyPolicyAlwaysPasses(t *testing.T) {
	t.Parallel()

	policy := oic.IntegrityPolicy{Name: "empty", Version: "1.0.0"}
	report, err := oic.Check("any output at all", policy)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !report.Passed {
		t.Errorf("empty policy should always pass, got: %v", report.Violations)
	}
}

func TestBannedTokenExcerptIsPopulated(t *testing.T) {
	t.Parallel()

	output := "Not professional advice. This is CONFIDENTIAL."
	c, err := oic.NewChecker(simplePolicy())
	if err != nil {
		t.Fatalf("NewChecker: %v", err)
	}
	report := c.Check(output)
	for _, v := range report.Violations {
		if v.Kind == "banned_token" && v.Excerpt == "" {
			t.Errorf("banned_token violation must have non-empty Excerpt")
		}
	}
}
