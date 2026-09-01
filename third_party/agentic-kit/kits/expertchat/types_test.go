package expertchat

import "testing"

// TestEvidenceItem_Citable pins the Citable() status gate directly (see
// chat_test.go's Run-level TestRun_RefusesNoEvidence_AllNonCitableStatus for
// the end-to-end proof).
func TestEvidenceItem_Citable(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{StatusFresh, true},
		{StatusStale, true},
		{StatusContradicted, false},
		{StatusRefuted, false},
		{"", false}, // the domain type documents Status as never empty for a
		// well-formed EvidenceItem; the wire layer (wireClaim.toEvidenceItem)
		// is responsible for normalizing "" to StatusFresh BEFORE an
		// EvidenceItem is ever constructed — Citable() itself makes no such
		// allowance, by design.
		{"unknown-status", false},
	}
	for _, tc := range cases {
		got := EvidenceItem{Status: tc.status}.Citable()
		if got != tc.want {
			t.Errorf("Citable() with Status=%q = %v, want %v", tc.status, got, tc.want)
		}
	}
}

// TestValidCiteIdentity_AllowsEmptyAnchorAndModuleVersion proves the two
// module_query v0 fields that are legitimately empty on the wire — Anchor
// (legacy hash-only claims) and ModuleVersion (the manifest loader is
// unbuilt, so it is ALWAYS empty via HTTPRetriever today) — do not make an
// otherwise well-formed item unsafe to cite. Only Module/ClaimID/Tier are
// required.
func TestValidCiteIdentity_AllowsEmptyAnchorAndModuleVersion(t *testing.T) {
	item := EvidenceItem{
		Module: "m", ClaimID: "c1", Tier: "verified", Status: StatusFresh,
		Anchor:        "",
		ModuleVersion: "",
	}
	if !validCiteIdentity(item) {
		t.Fatalf("validCiteIdentity(%+v) = false, want true — Anchor/ModuleVersion are not required fields", item)
	}
	if !item.Citable() {
		t.Fatal("Citable() = false, want true")
	}
}

// TestValidCiteIdentity_RequiresModuleClaimIDTier proves the three fields
// that ARE required.
func TestValidCiteIdentity_RequiresModuleClaimIDTier(t *testing.T) {
	cases := []struct {
		name string
		item EvidenceItem
	}{
		{"empty Module", EvidenceItem{ClaimID: "c1", Tier: "verified"}},
		{"empty ClaimID", EvidenceItem{Module: "m", Tier: "verified"}},
		{"empty Tier", EvidenceItem{Module: "m", ClaimID: "c1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if validCiteIdentity(tc.item) {
				t.Fatalf("validCiteIdentity(%+v) = true, want false", tc.item)
			}
		})
	}
}

// TestHasForbiddenTokenChars pins the exact character classes that make a
// field unsafe to embed in a citeToken/CITES list.
func TestHasForbiddenTokenChars(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"clean-id-42", false},
		{"", false},
		{"has#hash", true},
		{"has,comma", true},
		{"has space", true},
		{"has\ttab", true},
		{"has\nnewline", true},
		{"has\x00control", true},
	}
	for _, tc := range cases {
		if got := hasForbiddenTokenChars(tc.s); got != tc.want {
			t.Errorf("hasForbiddenTokenChars(%q) = %v, want %v", tc.s, got, tc.want)
		}
	}
}
