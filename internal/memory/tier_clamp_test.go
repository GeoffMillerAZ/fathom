package memory

// tier_clamp_test.go — W3 (review): the TierCeiling half of capConfidence
// had ZERO coverage. The reviewer deleted the clamp and the whole package
// stayed green — in the branch whose entire thesis is law 14, the only
// bound stopping a caller asserting confidence 9 on a 0–5 scale was
// unprotected by any test.
//
// The asymmetry that makes this the load-bearing half: agentic-kit's
// ApplyCeiling deliberately caps only doc-claimed/sme-stated/assumed;
// verified and pre-scan pass through it UNTOUCHED, their ceilings living in
// TierCeiling instead. So the untested branch was exactly the one covering
// the two highest-trust tiers — the ones a lying caller would claim.

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestCapConfidenceBoundsEveryTier(t *testing.T) {
	cases := []struct {
		tier    core.EvidenceTier
		claimed int
		want    int
	}{
		// The two tiers ApplyCeiling deliberately does NOT cap — the
		// branch the reviewer proved unprotected.
		{core.TierVerified, 9, 5},
		{core.TierPreScan, 9, 4},
		// In-range confidence survives untouched.
		{core.TierVerified, 3, 3},
		{core.TierPreScan, 2, 2},
		// The tiers ApplyCeiling does govern still land at their caps.
		{core.TierDocClaimed, 9, 2},
		{core.TierSMEStated, 9, 2},
		{core.TierAssumed, 9, 1},
		// A negative claim floors at zero rather than going through.
		{core.TierVerified, -3, 0},
	}
	for _, tc := range cases {
		if got := capConfidence(tc.claimed, tc.tier); got != tc.want {
			t.Errorf("capConfidence(%d, %s) = %d, want %d — a caller must not be able to assert "+
				"confidence above what the tier's verification depth supports (law 14)",
				tc.claimed, tc.tier, got, tc.want)
		}
	}
}
