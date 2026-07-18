package redactionexecutor_test

import (
	"strings"
	"testing"

	re "github.com/geoffmilleraz/agentic-kit/foundations/security/redaction-executor"
)

// ---- helpers ----------------------------------------------------------------

func emailRule() re.RedactionRule {
	return re.RedactionRule{
		ID:          "REDACT-PII-001",
		Pattern:     `[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`,
		Replacement: "[EMAIL]",
	}
}

func phoneRule() re.RedactionRule {
	return re.RedactionRule{
		ID:          "REDACT-PII-002",
		Pattern:     `\b\d{3}[-.\s]?\d{3}[-.\s]?\d{4}\b`,
		Replacement: "[PHONE]",
	}
}

func ssnRule() re.RedactionRule {
	return re.RedactionRule{
		ID:          "REDACT-PII-003",
		Pattern:     `\b\d{3}-\d{2}-\d{4}\b`,
		Replacement: "[SSN]",
	}
}

func piiPolicy(rules ...re.RedactionRule) re.RedactionPolicy {
	return re.RedactionPolicy{Name: "pii-v1", Version: "1.0.0", Rules: rules}
}

// ---- NewExecutor: pattern validation ----------------------------------------

func TestNewExecutor_InvalidPatternRejected(t *testing.T) {
	policy := piiPolicy(re.RedactionRule{
		ID:          "BAD-001",
		Pattern:     `(?P<unclosed`,
		Replacement: "[X]",
	})
	_, err := re.NewExecutor(policy)
	if err == nil {
		t.Error("expected error for invalid regexp pattern, got nil")
	}
}

func TestNewExecutor_ValidPolicyAccepted(t *testing.T) {
	_, err := re.NewExecutor(piiPolicy(emailRule()))
	if err != nil {
		t.Fatalf("unexpected error for valid policy: %v", err)
	}
}

// ---- Redact: basic substitution --------------------------------------------

func TestRedact_SingleRule_EmailReplaced(t *testing.T) {
	exec, err := re.NewExecutor(piiPolicy(emailRule()))
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	text := "Contact alice@example.com for details."
	redacted, applied := exec.Redact(text)

	if strings.Contains(redacted, "alice@example.com") {
		t.Errorf("email must be redacted, got: %q", redacted)
	}
	if !strings.Contains(redacted, "[EMAIL]") {
		t.Errorf("expected [EMAIL] in output, got: %q", redacted)
	}
	if len(applied) != 1 {
		t.Fatalf("expected 1 applied rule, got %d", len(applied))
	}
	if applied[0].RuleID != "REDACT-PII-001" {
		t.Errorf("RuleID: want REDACT-PII-001, got %q", applied[0].RuleID)
	}
	if applied[0].Occurrences != 1 {
		t.Errorf("Occurrences: want 1, got %d", applied[0].Occurrences)
	}
}

func TestRedact_MultipleOccurrences(t *testing.T) {
	exec, err := re.NewExecutor(piiPolicy(emailRule()))
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	text := "From: alice@example.com To: bob@corp.org CC: carol@firm.net"
	redacted, applied := exec.Redact(text)

	if strings.Contains(redacted, "@") {
		t.Errorf("all emails must be redacted, got: %q", redacted)
	}
	if len(applied) != 1 {
		t.Fatalf("expected 1 applied rule entry, got %d", len(applied))
	}
	if applied[0].Occurrences != 3 {
		t.Errorf("Occurrences: want 3, got %d", applied[0].Occurrences)
	}
}

// ---- Redact: no matches ----------------------------------------------------

func TestRedact_NoMatches_EmptyApplied(t *testing.T) {
	exec, err := re.NewExecutor(piiPolicy(emailRule()))
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	text := "There are no emails in this text."
	redacted, applied := exec.Redact(text)

	if redacted != text {
		t.Errorf("text without matches must be unchanged: got %q", redacted)
	}
	if len(applied) != 0 {
		t.Errorf("expected 0 applied rules, got %d", len(applied))
	}
}

func TestRedact_EmptyText(t *testing.T) {
	exec, err := re.NewExecutor(piiPolicy(emailRule()))
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	redacted, applied := exec.Redact("")
	if redacted != "" {
		t.Errorf("empty text must stay empty, got %q", redacted)
	}
	if len(applied) != 0 {
		t.Errorf("expected 0 applied, got %d", len(applied))
	}
}

// ---- Redact: multiple rules in order ----------------------------------------

func TestRedact_MultipleRulesAppliedInOrder(t *testing.T) {
	exec, err := re.NewExecutor(piiPolicy(emailRule(), phoneRule()))
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	text := "Email: alice@example.com, Phone: 555-867-5309"
	redacted, applied := exec.Redact(text)

	if strings.Contains(redacted, "@") {
		t.Errorf("email must be redacted, got: %q", redacted)
	}
	if strings.Contains(redacted, "555") {
		t.Errorf("phone must be redacted, got: %q", redacted)
	}
	if len(applied) != 2 {
		t.Fatalf("expected 2 applied rules, got %d", len(applied))
	}
	if applied[0].RuleID != "REDACT-PII-001" {
		t.Errorf("first applied rule: want REDACT-PII-001, got %q", applied[0].RuleID)
	}
	if applied[1].RuleID != "REDACT-PII-002" {
		t.Errorf("second applied rule: want REDACT-PII-002, got %q", applied[1].RuleID)
	}
}

func TestRedact_OnlyMatchingRulesAppearInApplied(t *testing.T) {
	// text has an email but no phone
	exec, err := re.NewExecutor(piiPolicy(emailRule(), phoneRule()))
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	text := "Send report to alice@example.com"
	_, applied := exec.Redact(text)

	if len(applied) != 1 {
		t.Fatalf("expected 1 applied rule (only email), got %d", len(applied))
	}
	if applied[0].RuleID != "REDACT-PII-001" {
		t.Errorf("expected REDACT-PII-001, got %q", applied[0].RuleID)
	}
}

// ---- Redact: empty replacement (removal) ------------------------------------

func TestRedact_EmptyReplacementRemovesText(t *testing.T) {
	rule := re.RedactionRule{
		ID:          "REDACT-TOKEN-001",
		Pattern:     `Bearer\s+[A-Za-z0-9._\-]+`,
		Replacement: "",
	}
	exec, err := re.NewExecutor(piiPolicy(rule))
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	text := "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.payload.sig"
	redacted, applied := exec.Redact(text)

	if strings.Contains(redacted, "Bearer") {
		t.Errorf("Bearer token must be removed, got: %q", redacted)
	}
	if len(applied) != 1 || applied[0].Occurrences != 1 {
		t.Errorf("expected 1 applied rule with 1 occurrence, got %v", applied)
	}
}

// ---- Redact: determinism ---------------------------------------------------

func TestRedact_Deterministic(t *testing.T) {
	exec, err := re.NewExecutor(piiPolicy(emailRule(), phoneRule(), ssnRule()))
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	text := "Alice: alice@corp.com, 800-555-1234, SSN 123-45-6789"

	first, appliedFirst := exec.Redact(text)
	second, appliedSecond := exec.Redact(text)

	if first != second {
		t.Errorf("non-deterministic: first=%q, second=%q", first, second)
	}
	if len(appliedFirst) != len(appliedSecond) {
		t.Errorf("applied count differs: %d vs %d", len(appliedFirst), len(appliedSecond))
	}
	for i := range appliedFirst {
		if appliedFirst[i] != appliedSecond[i] {
			t.Errorf("applied[%d] differs: %+v vs %+v", i, appliedFirst[i], appliedSecond[i])
		}
	}
}

// ---- Redact: idempotency ---------------------------------------------------

func TestRedact_Idempotent(t *testing.T) {
	exec, err := re.NewExecutor(piiPolicy(emailRule()))
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	text := "Contact alice@example.com for details."

	// First pass: redacts the email.
	once, _ := exec.Redact(text)

	// Second pass: [EMAIL] does not match the email pattern, so text unchanged.
	twice, appliedSecond := exec.Redact(once)

	if once != twice {
		t.Errorf("idempotency violated: first=%q, second=%q", once, twice)
	}
	if len(appliedSecond) != 0 {
		t.Errorf("second pass should produce no applied rules, got %d", len(appliedSecond))
	}
}

// ---- Redact: org-agnostic (rules are data) ----------------------------------

func TestRedact_OrgAgnosticCustomRules(t *testing.T) {
	// A completely custom rule set with no org-specific hard-coding inside the
	// executor.
	custom := re.RedactionPolicy{
		Name:    "custom-v1",
		Version: "0.1.0",
		Rules: []re.RedactionRule{
			{ID: "CUSTOM-001", Pattern: `PROJECT-\d+`, Replacement: "[TICKET]"},
			{ID: "CUSTOM-002", Pattern: `SECRET_KEY=\S+`, Replacement: "SECRET_KEY=[REDACTED]"},
		},
	}
	exec, err := re.NewExecutor(custom)
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}

	text := "Working on PROJECT-1234. Config: SECRET_KEY=abc123xyz"
	redacted, applied := exec.Redact(text)

	if strings.Contains(redacted, "PROJECT-1234") {
		t.Errorf("PROJECT ticket must be redacted, got: %q", redacted)
	}
	if strings.Contains(redacted, "abc123xyz") {
		t.Errorf("secret key value must be redacted, got: %q", redacted)
	}
	if len(applied) != 2 {
		t.Fatalf("expected 2 applied rules, got %d", len(applied))
	}
}
