package prosepolicy_test

import (
	"testing"

	pp "github.com/geoffmilleraz/agentic-kit/foundations/prose/prose-policy"
)

// openPolicy returns a valid open-posture policy with no rules.
func openPolicy() pp.Policy {
	return pp.Policy{Posture: pp.PostureOpen, Rules: nil}
}

// ---- Policy.Validate -------------------------------------------------------

func TestPolicy_ValidateHappyPath(t *testing.T) {
	if err := openPolicy().Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestPolicy_ValidateAllPostures(t *testing.T) {
	for _, pos := range []pp.Posture{
		pp.PostureOpen,
		pp.PostureInternal,
		pp.PostureRestricted,
		pp.PostureSecret,
	} {
		p := pp.Policy{Posture: pos}
		if err := p.Validate(); err != nil {
			t.Errorf("posture %q: unexpected error: %v", pos, err)
		}
	}
}

func TestPolicy_ValidateUnknownPosture(t *testing.T) {
	p := pp.Policy{Posture: "classified"}
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for unknown posture, got nil")
	}
}

func TestPolicy_ValidateEmptyTermInRule(t *testing.T) {
	p := pp.Policy{
		Posture: pp.PostureSecret,
		Rules: []pp.PostureRule{
			{Threshold: pp.PostureSecret, Terms: []string{"ok-term", ""}},
		},
	}
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for empty term in rule, got nil")
	}
}

func TestPolicy_ValidateUnknownRuleThreshold(t *testing.T) {
	p := pp.Policy{
		Posture: pp.PostureInternal,
		Rules: []pp.PostureRule{
			{Threshold: "ultra-secret", Terms: []string{"term"}},
		},
	}
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for unknown rule threshold, got nil")
	}
}

// ---- Policy.Evaluate -------------------------------------------------------

func TestEvaluate_OpenPostureNoRedactions(t *testing.T) {
	p := pp.Policy{
		Posture: pp.PostureOpen,
		Rules: []pp.PostureRule{
			{Threshold: pp.PostureInternal, Terms: []string{"INTERNAL"}},
		},
	}
	d := p.Evaluate("This document is INTERNAL use only.")
	// The rule's threshold is "internal" but posture is "open" (<= internal),
	// so the rule should NOT fire.
	if !d.Allowed {
		t.Errorf("open posture: expected Allowed=true, got false; redactions=%v", d.Redactions)
	}
	if len(d.Redactions) != 0 {
		t.Errorf("open posture: expected no redactions, got %v", d.Redactions)
	}
}

func TestEvaluate_InternalPostureActivatesInternalRule(t *testing.T) {
	p := pp.Policy{
		Posture: pp.PostureInternal,
		Rules: []pp.PostureRule{
			{Threshold: pp.PostureInternal, Terms: []string{"project-x"}},
		},
	}
	d := p.Evaluate("We are working on project-x.")
	if d.Allowed {
		t.Error("expected Allowed=false, got true")
	}
	if len(d.Redactions) != 1 || d.Redactions[0] != "project-x" {
		t.Errorf("expected [project-x], got %v", d.Redactions)
	}
}

func TestEvaluate_SecretPostureActivatesAllLowerRules(t *testing.T) {
	p := pp.Policy{
		Posture: pp.PostureSecret,
		Rules: []pp.PostureRule{
			{Threshold: pp.PostureInternal, Terms: []string{"INTERNAL"}},
			{Threshold: pp.PostureRestricted, Terms: []string{"RESTRICTED"}},
			{Threshold: pp.PostureSecret, Terms: []string{"TOP-SECRET"}},
		},
	}
	text := "INTERNAL RESTRICTED TOP-SECRET content."
	d := p.Evaluate(text)
	if d.Allowed {
		t.Error("expected Allowed=false")
	}
	want := map[string]bool{"INTERNAL": true, "RESTRICTED": true, "TOP-SECRET": true}
	for _, r := range d.Redactions {
		if !want[r] {
			t.Errorf("unexpected redaction %q", r)
		}
		delete(want, r)
	}
	for term := range want {
		t.Errorf("expected redaction %q not found", term)
	}
}

func TestEvaluate_NoMatchReturnsAllowed(t *testing.T) {
	p := pp.Policy{
		Posture: pp.PostureRestricted,
		Rules: []pp.PostureRule{
			{Threshold: pp.PostureRestricted, Terms: []string{"CONFIDENTIAL"}},
		},
	}
	d := p.Evaluate("This text has no sensitive content.")
	if !d.Allowed {
		t.Errorf("expected Allowed=true, got false; redactions=%v", d.Redactions)
	}
}

func TestEvaluate_NoDuplicateRedactions(t *testing.T) {
	// Same term appears in two rules at different thresholds.
	p := pp.Policy{
		Posture: pp.PostureSecret,
		Rules: []pp.PostureRule{
			{Threshold: pp.PostureInternal, Terms: []string{"term-a"}},
			{Threshold: pp.PostureSecret, Terms: []string{"term-a"}},
		},
	}
	d := p.Evaluate("Mentions term-a twice via two rules.")
	count := 0
	for _, r := range d.Redactions {
		if r == "term-a" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected term-a to appear exactly once in redactions, got %d", count)
	}
}

func TestEvaluate_OpenPolicyNoRules(t *testing.T) {
	p := openPolicy()
	d := p.Evaluate("Any content whatsoever.")
	if !d.Allowed {
		t.Error("open policy with no rules: expected Allowed=true")
	}
}
