package sensitivityclassifier_test

import (
	"testing"

	dst "github.com/geoffmilleraz/agentic-kit/foundations/security/data-sensitivity-types"
	sc "github.com/geoffmilleraz/agentic-kit/foundations/security/sensitivity-classifier"
)

func genericCatalog() sc.Catalog {
	return sc.Catalog{
		Name:    "test-v1",
		Version: "1.0.0",
		Rules: []sc.ClassRule{
			{ID: "CR-001", Pattern: `\b(ssn|social.security)\b`, Level: dst.SensitivityRestricted, Description: "SSN pattern"},
			{ID: "CR-002", Pattern: `\b(password|api.key)\b`, Level: dst.SensitivityConfidential, Description: "credential pattern"},
			{ID: "CR-003", Pattern: `\binternal.use.only\b`, Level: dst.SensitivityInternal, Description: "internal marker"},
		},
	}
}

func TestClassifyHighestLevelWins(t *testing.T) {
	t.Parallel()

	// text matches both internal and restricted rules — must return restricted
	text := "internal use only — also contains an SSN field"
	c, err := sc.NewClassifier(genericCatalog())
	if err != nil {
		t.Fatalf("NewClassifier: %v", err)
	}
	result := c.Classify(text)
	if result.Level != dst.SensitivityRestricted {
		t.Errorf("expected restricted, got %q (reason: %s)", result.Level, result.Reason)
	}
}

func TestClassifyNoMatchIsPublic(t *testing.T) {
	t.Parallel()

	c, err := sc.NewClassifier(genericCatalog())
	if err != nil {
		t.Fatalf("NewClassifier: %v", err)
	}
	result := c.Classify("the quick brown fox jumps over the lazy dog")
	if result.Level != dst.SensitivityPublic {
		t.Errorf("expected public for no-match, got %q", result.Level)
	}
}

func TestClassifyIsDeterministic(t *testing.T) {
	t.Parallel()

	text := "your api.key should not be shared"
	c, err := sc.NewClassifier(genericCatalog())
	if err != nil {
		t.Fatalf("NewClassifier: %v", err)
	}
	r1 := c.Classify(text)
	r2 := c.Classify(text)
	if r1.Level != r2.Level || r1.Reason != r2.Reason {
		t.Errorf("Classify is not deterministic: %v vs %v", r1, r2)
	}
}

func TestClassifyCaseInsensitive(t *testing.T) {
	t.Parallel()

	c, err := sc.NewClassifier(genericCatalog())
	if err != nil {
		t.Fatalf("NewClassifier: %v", err)
	}
	for _, text := range []string{"SSN", "ssn", "Ssn"} {
		result := c.Classify(text)
		if result.Level != dst.SensitivityRestricted {
			t.Errorf("expected restricted for %q, got %q", text, result.Level)
		}
	}
}

func TestClassifyPackageLevelFunc(t *testing.T) {
	t.Parallel()

	result, err := sc.Classify("password reset required", genericCatalog())
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if result.Level != dst.SensitivityConfidential {
		t.Errorf("expected confidential, got %q", result.Level)
	}
}

func TestNewClassifierRejectsInvalidPattern(t *testing.T) {
	t.Parallel()

	bad := sc.Catalog{
		Name: "bad", Version: "0.0.1",
		Rules: []sc.ClassRule{
			{ID: "CR-BAD", Pattern: `(?P<unclosed`, Level: dst.SensitivityInternal, Description: "bad"},
		},
	}
	_, err := sc.NewClassifier(bad)
	if err == nil {
		t.Error("expected error for invalid pattern, got nil")
	}
}

func TestClassifyReasonIsNonEmpty(t *testing.T) {
	t.Parallel()

	c, err := sc.NewClassifier(genericCatalog())
	if err != nil {
		t.Fatalf("NewClassifier: %v", err)
	}
	for _, text := range []string{"ssn value here", "harmless content"} {
		result := c.Classify(text)
		if result.Reason == "" {
			t.Errorf("Reason must not be empty for text %q", text)
		}
	}
}
