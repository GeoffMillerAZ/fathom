package findings

// tier.go — the §3.5 promotion rule (docs/design/findings-as-evidence.md):
// "A pattern may be promoted exactly one tier when it has >=3 members with
// recorded, independent code-change resolutions... Promotion is
// deterministic and recomputed, never sticky... The findings layer must
// pass an eval gate before any pattern serves anywhere."
//
// Deterministic and reversible: PromoteTier is a pure function of (base
// tier, pattern, layer-evaled flag) — nothing here is written anywhere, so
// a pattern that drops below 3 resolved members (a refutation flips a
// member) simply stops promoting on the NEXT call. There is no persisted
// "promoted" state to un-set.
//
// Findings-layer claims only ever carry TierSMEStated (the LLM-compiled
// findings/** corpus) or TierVerified (the deterministic review-verb
// bridge, already at the ceiling — ADR-016 M1's five-tier ranking's own
// top rung). This is therefore a two-rung ladder specific to findings, not
// a reuse of the full five-tier ranking (pre-scan/assumed never appear on
// a finding-kind claim) — PromoteTier documents that scope explicitly
// rather than pretending to generalize past it.

import (
	"fmt"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// promotionCeiling is the ONE step PromoteTier ever takes: sme-stated (a
// reviewer's restated opinion) rises to verified (resolved-evidence-backed)
// once the promotion bar is met. TierDocClaimed is included for
// completeness — a finding-kind claim compiled under an unusual layer
// override could in principle carry it — but no shipped findings pack
// does.
var promotionCeiling = map[core.EvidenceTier]core.EvidenceTier{
	core.TierSMEStated:  core.TierVerified,
	core.TierDocClaimed: core.TierVerified,
}

// PromoteTier applies §3.5's promotion rule to one Pattern. layerEvaled
// gates the WHOLE mechanism structurally — "must pass an eval gate before
// any pattern serves anywhere" is enforced here, not left to a caller to
// remember to check: when the findings layer has not passed its eval gate,
// PromoteTier always returns base unchanged, regardless of resolved-member
// count. tier is always base or its one-rung-up promotion; note explains
// which, and why not, in every case (never a silent no-op).
func PromoteTier(base core.EvidenceTier, p Pattern, layerEvaled bool) (tier core.EvidenceTier, note string) {
	if !layerEvaled {
		return base, "not promoted: the findings layer has not passed its eval gate (docs/layers.md) — no pattern serves at a promoted tier until it does"
	}
	n := p.ResolvedCodeChangeCount()
	if n < 3 {
		return base, fmt.Sprintf("not promoted: %d/3 members have an independent, recorded code-change resolution", n)
	}
	promoted, ok := promotionCeiling[base]
	if !ok {
		return base, fmt.Sprintf("not promoted: tier %q has no rung above it in the findings promotion ladder", base)
	}
	return promoted, fmt.Sprintf("promoted from %q to %q: %d members have independent code-change resolutions (recomputed every call — reverses if resolutions drop below 3)", base, promoted, n)
}
