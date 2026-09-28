package review

import "testing"

// TestReceiptHeuristicLinksNeverMoveBand proves I57 end to end (CX-7): a
// diff whose only touched links sit below the typed tier never rises above
// low, no matter how many such links there are or how "cross-service" they
// look — they are listed as hints, never counted.
func TestReceiptHeuristicLinksNeverMoveBand(t *testing.T) {
	in := Input{
		ChangedFiles: []string{"internal/widget/widget.go"},
		Links: []LinkRef{
			{Provider: "widget.Build", Consumer: "other.Caller", ConsumerService: "otherservice", Tier: "heuristic"},
			{Provider: "widget.Build", Consumer: "another.Caller", ConsumerService: "thirdservice", Tier: "literal"},
			{Provider: "widget.Build", Consumer: "yetanother.Caller", ConsumerService: "fourthservice", Tier: "template"},
		},
	}

	r := BuildRiskReceipt(in, DefaultRiskTable())

	if r.Band != BandLow {
		t.Fatalf("Band = %q, want low — heuristic-tier links must never move the band (I57)", r.Band)
	}

	var linksFactor RiskFactor
	for _, f := range r.Factors {
		if f.Name == FactorLinksTouched {
			linksFactor = f
		}
	}
	if linksFactor.Weight != WeightNone {
		t.Errorf("links_touched factor weight = %s, want none — no spec/typed link was touched", linksFactor.Weight)
	}

	if len(r.LinkHints) != 3 {
		t.Fatalf("LinkHints = %d, want all 3 sub-typed links listed", len(r.LinkHints))
	}
}

// TestReceiptLinkTiersPartition proves receiptLinkTiers' own split in
// isolation: spec and typed are counted, everything else is a hint, and the
// function never drops or duplicates an input link.
func TestReceiptLinkTiersPartition(t *testing.T) {
	links := []LinkRef{
		{Provider: "a", Consumer: "b", Tier: LinkTierSpec},
		{Provider: "c", Consumer: "d", Tier: LinkTierTyped},
		{Provider: "e", Consumer: "f", Tier: "literal"},
		{Provider: "g", Consumer: "h", Tier: "template"},
		{Provider: "i", Consumer: "j", Tier: ""},
	}

	counted, hints := receiptLinkTiers(links)
	if len(counted) != 2 {
		t.Errorf("counted = %d, want 2 (spec + typed)", len(counted))
	}
	if len(hints) != 3 {
		t.Errorf("hints = %d, want 3 (literal + template + unlabeled)", len(hints))
	}
	if len(counted)+len(hints) != len(links) {
		t.Errorf("partition dropped or duplicated a link: %d + %d != %d", len(counted), len(hints), len(links))
	}
}
