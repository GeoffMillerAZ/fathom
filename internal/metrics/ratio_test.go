package metrics

import "testing"

// TestRatio_ZeroDenominator_NeverFabricates100Percent is the exact bug
// class phase P's critic finding F1 fixed once (buildDiscoverySection) —
// numerator > 0, denominator == 0 must never compute 100%.
func TestRatio_ZeroDenominator_NeverFabricates100Percent(t *testing.T) {
	pct, note := Ratio(42, 0)
	if pct != nil {
		t.Fatalf("Ratio(42, 0) pct = %v, want nil", *pct)
	}
	if note == "" {
		t.Fatal("Ratio(42, 0) note is empty, want an honest zero-denominator note")
	}
}

func TestRatio_ZeroNumerator_ZeroPercent(t *testing.T) {
	pct, note := Ratio(0, 10)
	if pct == nil || *pct != 0 {
		t.Fatalf("Ratio(0, 10) = %v, want 0", pct)
	}
	if note != "" {
		t.Errorf("note = %q, want empty on a real computation", note)
	}
}

func TestRatio_ComputesExpectedPercentage(t *testing.T) {
	pct, note := Ratio(1, 4)
	if pct == nil || *pct != 25 {
		t.Fatalf("Ratio(1, 4) = %v, want 25", pct)
	}
	if note != "" {
		t.Errorf("note = %q, want empty", note)
	}
}

func TestRatio_BothZero_NeverMeasurable(t *testing.T) {
	pct, note := Ratio(0, 0)
	if pct != nil {
		t.Fatalf("Ratio(0, 0) pct = %v, want nil", *pct)
	}
	if note == "" {
		t.Fatal("Ratio(0, 0) note is empty")
	}
}
