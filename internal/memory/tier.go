package memory

import (
	"github.com/geoffmilleraz/agentic-kit/foundations/review/scoring"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// tier.go — the law-14 arithmetic, in one place.
//
// Two scales are in play and they must not be conflated (agentic-kit's
// scoring package says the same thing at more length): a TIER is a position
// on the evidence ladder, a CONFIDENCE is a 0–5 number bounded by that
// position. This file owns the ladder walk; scoring owns the bound.

// tierLadder is the evidence ladder, strongest first. It is the ordering
// dropOne walks and the one weaker() compares on. It deliberately does not
// reuse scoring.TierRank for ordering: TierRank collapses "assumed" and
// "unrecognized" to the same 0, which would make an unknown tier
// indistinguishable from the floor exactly where this package must refuse.
var tierLadder = []core.EvidenceTier{
	core.TierVerified,
	core.TierPreScan,
	core.TierDocClaimed,
	core.TierSMEStated,
	core.TierAssumed,
}

// ladderIndex returns t's position on the ladder (0 = strongest). An
// unrecognized tier sorts at the floor.
func ladderIndex(t core.EvidenceTier) int {
	for i, x := range tierLadder {
		if x == t {
			return i
		}
	}
	return len(tierLadder) - 1
}

// weaker returns whichever of a and b is lower on the ladder.
func weaker(a, b core.EvidenceTier) core.EvidenceTier {
	if ladderIndex(b) > ladderIndex(a) {
		return b
	}
	return a
}

// dropOne returns the tier one step down the ladder. The floor
// (core.TierAssumed) drops to itself — there is nowhere lower to go, and
// inventing a sixth tier to express "suspect assumed" would be a new ladder,
// which §2.2 explicitly refuses.
func dropOne(t core.EvidenceTier) core.EvidenceTier {
	i := ladderIndex(t)
	if i+1 < len(tierLadder) {
		return tierLadder[i+1]
	}
	return tierLadder[len(tierLadder)-1]
}

// DeriveTier computes the tier a set of evidence STRUCTURALLY supports: the
// strongest kind present wins, because one machine check is enough to make a
// claim verified regardless of how much weaker material sits beside it. ok is
// false when ev is empty or contains an unrecognized kind — both are write
// refusals, never a silent fallback.
func DeriveTier(ev []Evidence) (core.EvidenceTier, bool) {
	if len(ev) == 0 {
		return "", false
	}
	best := core.TierAssumed
	for i, e := range ev {
		t, ok := TierForEvidenceKind(e.Kind)
		if !ok {
			return "", false
		}
		if i == 0 || ladderIndex(t) < ladderIndex(best) {
			best = t
		}
	}
	return best, true
}

// corroborated reports whether evidence comes from at least two DISTINCT
// refs. The same source cited twice is not independent — the rule
// scoring.DeriveConfidenceFromTier already encodes, restated here over
// evidence refs rather than file paths.
func corroborated(ev []Evidence) bool {
	seen := make(map[string]bool, len(ev))
	for _, e := range ev {
		seen[e.Ref] = true
		if len(seen) > 1 {
			return true
		}
	}
	return false
}

// capConfidence bounds a confidence by a tier, through agentic-kit's
// scoring.ApplyCeiling plus the tier's own hard ceiling.
//
// ApplyCeiling alone is not sufficient and the asymmetry is deliberate in the
// kit: it caps doc-claimed/sme-stated at 2 and assumed at 1, but leaves
// verified and pre-scan UNCAPPED because their ceilings (5 and 4) are
// reported by TierCeiling instead. A caller claiming confidence 9 on pre-scan
// evidence would sail through ApplyCeiling untouched, so both are applied
// here — ApplyCeiling for the tiers it governs, TierCeiling for the rest.
func capConfidence(confidence int, tier core.EvidenceTier) int {
	capped := scoring.ApplyCeiling(confidence, scoring.Tier(tier))
	if ceiling := scoring.TierCeiling(scoring.Tier(tier)); capped > ceiling {
		capped = ceiling
	}
	if capped < 0 {
		return 0
	}
	return capped
}

// defaultConfidence derives a confidence for a caller that did not state one:
// the tier's ceiling when the evidence is corroborated, one below it
// otherwise (scoring.DeriveConfidenceFromTier's rule, unchanged).
func defaultConfidence(tier core.EvidenceTier, ev []Evidence) int {
	return scoring.DeriveConfidenceFromTier(scoring.Tier(tier), corroborated(ev))
}
