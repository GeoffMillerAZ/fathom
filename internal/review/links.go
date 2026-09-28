package review

// receiptLinkTiers partitions the diff's touched links by evidence tier
// (I57, docs/intent/invariants.md: "a cross-boundary link below the typed
// tier never gates"): counted holds spec/typed-tier links, which the
// links-touched and uncertified-contracts factors may weigh; hints holds
// every lower tier (literal, template, or anything else the caller names),
// which the receipt lists for a human to see but NEVER lets move the band.
//
// Both return slices are in receiptLinkTiers' own input order — callers
// that need a stable rendering order sort the result themselves
// (linksTouchedFactor and uncertifiedContractsFactor already do).
func receiptLinkTiers(links []LinkRef) (counted, hints []LinkRef) {
	for _, l := range links {
		if l.Tier == LinkTierSpec || l.Tier == LinkTierTyped {
			counted = append(counted, l)
		} else {
			hints = append(hints, l)
		}
	}
	return counted, hints
}
