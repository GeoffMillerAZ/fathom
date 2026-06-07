package core

import "testing"

func TestSymbolKey(t *testing.T) {
	tests := []struct {
		svc, pkg, name, want string
	}{
		{"payment-service", "auth", "ValidateToken", "payment-service::auth.ValidateToken"},
		{"svc", "", "Top", "svc::Top"},
	}
	for _, tc := range tests {
		if got := SymbolKey(tc.svc, tc.pkg, tc.name); got != tc.want {
			t.Errorf("SymbolKey(%q,%q,%q) = %q, want %q", tc.svc, tc.pkg, tc.name, got, tc.want)
		}
	}
}

func TestSymbolNodeKeyDeterministic(t *testing.T) {
	n := SymbolNode{Service: "s", Package: "p", Name: "F"}
	if n.Key() != "s::p.F" {
		t.Fatalf("Key() = %q", n.Key())
	}
	if n.Key() != n.Key() {
		t.Fatal("Key() not deterministic")
	}
}

func TestEvidenceTierValid(t *testing.T) {
	for _, tier := range []EvidenceTier{TierVerified, TierPreScan, TierDocClaimed, TierSMEStated, TierAssumed} {
		if !tier.Valid() {
			t.Errorf("%q should be valid", tier)
		}
	}
	if EvidenceTier("bogus").Valid() {
		t.Error("bogus tier should be invalid")
	}
}
