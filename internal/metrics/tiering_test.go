package metrics

import "testing"

func TestResolveMetricsTier_Unset_DefaultsBasic(t *testing.T) {
	t.Setenv("FATHOM_METRICS", "")
	tier, note := ResolveMetricsTier()
	if tier != MetricsTierBasic {
		t.Fatalf("tier = %q, want basic", tier)
	}
	if note != "" {
		t.Errorf("note = %q, want empty on the default", note)
	}
}

func TestResolveMetricsTier_Off(t *testing.T) {
	t.Setenv("FATHOM_METRICS", "off")
	tier, note := ResolveMetricsTier()
	if tier != MetricsTierOff {
		t.Fatalf("tier = %q, want off", tier)
	}
	if note != "" {
		t.Errorf("note = %q, want empty", note)
	}
}

func TestResolveMetricsTier_Full(t *testing.T) {
	t.Setenv("FATHOM_METRICS", "full")
	tier, note := ResolveMetricsTier()
	if tier != MetricsTierFull {
		t.Fatalf("tier = %q, want full", tier)
	}
	if note != "" {
		t.Errorf("note = %q, want empty", note)
	}
}

func TestResolveMetricsTier_InvalidValue_FallsBackToBasicWithNote(t *testing.T) {
	t.Setenv("FATHOM_METRICS", "extreme")
	tier, note := ResolveMetricsTier()
	if tier != MetricsTierBasic {
		t.Fatalf("tier = %q, want basic (fallback)", tier)
	}
	if note == "" {
		t.Error("note is empty, want an observable fallback note naming the invalid value")
	}
}
