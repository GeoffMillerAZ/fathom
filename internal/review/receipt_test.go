package review

import (
	"reflect"
	"sort"
	"testing"
)

// TestRiskReceiptFactorsCited proves the receipt's own contract (design doc
// §2, CX-1): all eight named factors are always present, in the design
// doc's fixed order, and every factor that fires carries a non-empty Reason
// — "every reason cited", never a bare weight with no explanation.
func TestRiskReceiptFactorsCited(t *testing.T) {
	in := Input{
		ChangedFiles:      []string{"internal/widget/widget.go"},
		TransitiveCallers: 120,
		Links: []LinkRef{
			{Provider: "widget.Build", Consumer: "other.Caller", ConsumerService: "otherservice", Tier: LinkTierTyped},
			{Provider: "widget.Build", Consumer: "legacy.Caller", Tier: LinkTierSpec},
			{Provider: "widget.Build", Consumer: "guess.Caller", Tier: "heuristic"},
		},
		CertificatesAffected:  []CertificateRef{{Subject: "widget.Build", State: "broken"}},
		ConservationUnexcused: true,
		ConservationDetail:    "internal/widget/widget.go: 3 unattested lines",
		ClaimedRows:           []RowVerdict{{ID: "CX-1", Verdict: RowUnproven}},
		RulesTouched:          []RuleRef{{File: "internal/widget/widget.go", Rule: "no-bare-goroutines", Level: RuleLevelError}},
		ZoneInterestRate:      3.5,
		UncertifiedContracts:  []LinkRef{{Provider: "widget.Build", Consumer: "legacy.Caller", Tier: LinkTierSpec}},
	}

	r := BuildRiskReceipt(in, DefaultRiskTable())

	wantNames := []string{
		FactorBlastRadius,
		FactorLinksTouched,
		FactorCertificates,
		FactorConservation,
		FactorRowsProven,
		FactorRulesTouched,
		FactorZoneInterest,
		FactorUncertified,
	}
	if len(r.Factors) != len(wantNames) {
		t.Fatalf("got %d factors, want %d", len(r.Factors), len(wantNames))
	}
	for i, name := range wantNames {
		if r.Factors[i].Name != name {
			t.Errorf("factor[%d] = %q, want %q (order must be the design doc's own fixed order)", i, r.Factors[i].Name, name)
		}
		if r.Factors[i].Reason == "" {
			t.Errorf("factor %q has no cited reason", name)
		}
	}

	// Every factor above fires in this fixture — each should carry
	// evidence a human can check, not a bare claim.
	for _, f := range r.Factors {
		if f.Weight != WeightNone && len(f.Evidence) == 0 && f.Name != FactorConservation && f.Name != FactorRowsProven && f.Name != FactorRulesTouched {
			t.Errorf("factor %q fired (%s) with no evidence", f.Name, f.Weight)
		}
	}

	// The heuristic-tier link never becomes a counted factor input — it is
	// listed as a hint (CX-7) instead, proven fully by links_test.go.
	if len(r.LinkHints) != 1 || r.LinkHints[0].Consumer != "guess.Caller" {
		t.Errorf("LinkHints = %+v, want exactly the heuristic-tier link", r.LinkHints)
	}

	// The band is the strongest reason, never a summed score: several
	// factors are "high" here (blast radius, links, certificates,
	// conservation, rows, rules) — the receipt cites exactly ONE of them,
	// the first in fixed order, as BandReason.
	if r.Band != BandHigh {
		t.Errorf("Band = %q, want high", r.Band)
	}
	if r.BandReason != r.Factors[0].Reason {
		t.Errorf("BandReason = %q, want the first high factor's reason %q", r.BandReason, r.Factors[0].Reason)
	}

	if r.Authority == "" {
		t.Error("Authority must always be set (ADR-040 boundary, CX-8)")
	}

	if got, want := r.ChangedFiles, []string{"internal/widget/widget.go"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ChangedFiles = %v, want %v", got, want)
	}
}

// TestRiskReceiptLowBand proves the "all clean" path: every factor at
// WeightNone bands to low, with a reason that does not fabricate a citation
// to a factor that never fired.
func TestRiskReceiptLowBand(t *testing.T) {
	in := Input{
		ChangedFiles:      []string{"internal/widget/widget.go"},
		TransitiveCallers: 3,
		CertificatesAffected: []CertificateRef{
			{Subject: "widget.Build", State: "live"},
		},
		ClaimedRows:      []RowVerdict{{ID: "CX-1", Verdict: RowProven}},
		ZoneInterestRate: 0.4,
	}

	r := BuildRiskReceipt(in, DefaultRiskTable())
	if r.Band != BandLow {
		t.Fatalf("Band = %q, want low; factors=%+v", r.Band, r.Factors)
	}
	for _, f := range r.Factors {
		if f.Weight != WeightNone {
			t.Errorf("factor %q unexpectedly fired at %s in an all-clean fixture", f.Name, f.Weight)
		}
	}
}

// TestRiskReceiptDeterministic proves the determinism contract
// (internal/AGENTS.md): identical Input always produces byte-identical
// output, regardless of slice input order (every internal sort is applied).
func TestRiskReceiptDeterministic(t *testing.T) {
	in := Input{
		ChangedFiles: []string{"b.go", "a.go"},
		Links: []LinkRef{
			{Provider: "z.Fn", Consumer: "y.Caller", ConsumerService: "svcB", Tier: LinkTierTyped},
			{Provider: "a.Fn", Consumer: "b.Caller", ConsumerService: "svcA", Tier: LinkTierTyped},
		},
		CertificatesAffected: []CertificateRef{
			{Subject: "z.Fn", State: "broken"},
			{Subject: "a.Fn", State: "broken"},
		},
	}
	table := DefaultRiskTable()

	r1 := BuildRiskReceipt(in, table)
	r2 := BuildRiskReceipt(in, table)
	if !reflect.DeepEqual(r1, r2) {
		t.Fatalf("BuildRiskReceipt is not deterministic:\n%+v\n%+v", r1, r2)
	}

	if !sort.StringsAreSorted(r1.ChangedFiles) {
		t.Errorf("ChangedFiles not sorted: %v", r1.ChangedFiles)
	}
}
