package findings

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func patternWithResolvedCount(n int) Pattern {
	var members []PatternMember
	for i := 0; i < n; i++ {
		members = append(members, PatternMember{
			ClaimID: "resolved", Status: MemberSuperseded, ResolutionKind: core.ResolutionCodeChange,
		})
	}
	// Two extra live members so SiteCount would be plausible (not asserted
	// here — PromoteTier doesn't check site count itself, DerivePatterns
	// already enforced it upstream).
	members = append(members, PatternMember{ClaimID: "live1", Status: MemberLive}, PatternMember{ClaimID: "live2", Status: MemberLive})
	return Pattern{Class: core.FindingClassSecurity, Members: members}
}

func TestPromoteTier_LayerNotEvaled_NeverPromotes(t *testing.T) {
	p := patternWithResolvedCount(5) // well above the bar
	tier, note := PromoteTier(core.TierSMEStated, p, false)
	if tier != core.TierSMEStated {
		t.Errorf("tier = %q, want unchanged sme-stated", tier)
	}
	if !strings.Contains(note, "eval gate") {
		t.Errorf("note = %q, want it to name the eval-gate reason", note)
	}
}

func TestPromoteTier_BelowThreeResolved_NotPromoted(t *testing.T) {
	p := patternWithResolvedCount(2)
	tier, note := PromoteTier(core.TierSMEStated, p, true)
	if tier != core.TierSMEStated {
		t.Errorf("tier = %q, want unchanged sme-stated", tier)
	}
	if !strings.Contains(note, "2/3") {
		t.Errorf("note = %q, want it to name the 2/3 count", note)
	}
}

func TestPromoteTier_ThreeResolved_LayerEvaled_Promotes(t *testing.T) {
	p := patternWithResolvedCount(3)
	tier, note := PromoteTier(core.TierSMEStated, p, true)
	if tier != core.TierVerified {
		t.Errorf("tier = %q, want promoted to verified", tier)
	}
	if !strings.Contains(note, "promoted") {
		t.Errorf("note = %q, want it to say promoted", note)
	}
}

func TestPromoteTier_AlreadyAtCeiling_NoRungAbove(t *testing.T) {
	p := patternWithResolvedCount(5)
	tier, note := PromoteTier(core.TierVerified, p, true)
	if tier != core.TierVerified {
		t.Errorf("tier = %q, want unchanged verified (already the ceiling)", tier)
	}
	if !strings.Contains(note, "no rung above") {
		t.Errorf("note = %q, want it to say there's no rung above", note)
	}
}

// TestPromoteTier_Reversible proves §4 honesty rule 3 directly: the SAME
// pattern, recomputed after a member flips from code-change-resolved to
// refuted (dropping the count below 3), no longer promotes — nothing
// "stuck" from the prior call.
func TestPromoteTier_Reversible(t *testing.T) {
	p := patternWithResolvedCount(3)
	tier1, _ := PromoteTier(core.TierSMEStated, p, true)
	if tier1 != core.TierVerified {
		t.Fatalf("expected initial promotion, got %q", tier1)
	}

	// One member refuted (docs/design/findings-as-evidence.md §3.4's third
	// case) — the pattern is RE-DERIVED (in real use, via a fresh
	// DerivePatterns call over refreshed claims) with only 2 resolved
	// members left.
	p.Members[0].Status = MemberRefuted
	p.Members[0].ResolutionKind = ""
	tier2, note2 := PromoteTier(core.TierSMEStated, p, true)
	if tier2 != core.TierSMEStated {
		t.Errorf("tier2 = %q, want dropped back to sme-stated", tier2)
	}
	if !strings.Contains(note2, "2/3") {
		t.Errorf("note2 = %q, want it to name the new 2/3 count", note2)
	}
}

// TestPromoteTier_IndependenceRequiresSupersededStatus proves a LIVE
// member's ResolutionKind (which should never be set, but defends against
// a future caller mistake) is never counted — only a genuinely superseded,
// code-change-resolved member counts, matching
// PatternMember.HasIndependentCodeChangeResolution's own contract.
func TestPromoteTier_IndependenceRequiresSupersededStatus(t *testing.T) {
	p := Pattern{
		Class: core.FindingClassSecurity,
		Members: []PatternMember{
			{ClaimID: "c1", Status: MemberLive, ResolutionKind: core.ResolutionCodeChange}, // must not count
			{ClaimID: "c2", Status: MemberSuperseded, ResolutionKind: core.ResolutionCodeChange},
			{ClaimID: "c3", Status: MemberSuperseded, ResolutionKind: core.ResolutionCodeChange},
		},
	}
	tier, note := PromoteTier(core.TierSMEStated, p, true)
	if tier != core.TierSMEStated {
		t.Errorf("tier = %q, want unchanged (only 2 genuine resolutions)", tier)
	}
	if !strings.Contains(note, "2/3") {
		t.Errorf("note = %q, want 2/3", note)
	}
}
