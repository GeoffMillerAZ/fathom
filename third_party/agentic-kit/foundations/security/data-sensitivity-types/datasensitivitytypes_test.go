package datasensitivitytypes_test

import (
	"testing"

	dst "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/data-sensitivity-types"
)

func TestSensitivityLevelValues(t *testing.T) {
	t.Parallel()

	levels := []dst.SensitivityLevel{
		dst.SensitivityPublic,
		dst.SensitivityInternal,
		dst.SensitivityConfidential,
		dst.SensitivityRestricted,
	}
	for _, l := range levels {
		if l == "" {
			t.Errorf("SensitivityLevel constant must not be empty")
		}
	}
}

func TestLevelGTEOrder(t *testing.T) {
	t.Parallel()

	ordered := []dst.SensitivityLevel{
		dst.SensitivityPublic,
		dst.SensitivityInternal,
		dst.SensitivityConfidential,
		dst.SensitivityRestricted,
	}

	for i, a := range ordered {
		// Reflexive: a >= a
		if !dst.LevelGTE(a, a) {
			t.Errorf("LevelGTE(%q, %q) should be true (reflexive)", a, a)
		}
		for j, b := range ordered {
			if i < j {
				// a is lower than b: a should NOT be >= b
				if dst.LevelGTE(a, b) {
					t.Errorf("LevelGTE(%q, %q) should be false (lower level must not be GTE higher)", a, b)
				}
				// b is higher than a: b should be >= a
				if !dst.LevelGTE(b, a) {
					t.Errorf("LevelGTE(%q, %q) should be true (higher level must be GTE lower)", b, a)
				}
			}
		}
	}
}

func TestClassificationFields(t *testing.T) {
	t.Parallel()

	c := dst.Classification{
		Level:  dst.SensitivityConfidential,
		Reason: "contains PII",
	}
	if c.Level != dst.SensitivityConfidential {
		t.Errorf("expected Level=%q, got %q", dst.SensitivityConfidential, c.Level)
	}
	if c.Reason == "" {
		t.Error("Reason must not be empty")
	}
}
