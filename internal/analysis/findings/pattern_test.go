package findings

import (
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestNormalizeRuleShape(t *testing.T) {
	cases := []struct {
		name, rule, want string
	}{
		{
			"arch violation keeps only the stable reason clause",
			"svc::api.Handler → svc::store.Query: layer api may not call store directly",
			"layer api may not call store directly",
		},
		{
			"a second arch violation of the same rule normalizes identically",
			"svc::api.Other → svc::store.Write: layer api may not call store directly",
			"layer api may not call store directly",
		},
		{
			"spec_uncovered strips the bracketed spec ID",
			"spec [US-777] is referenced in this diff but has no covering symbol",
			"spec <spec> is referenced in this diff but has no covering symbol",
		},
		{
			"a different spec ID normalizes to the same shape",
			"spec [US-42] is referenced in this diff but has no covering symbol",
			"spec <spec> is referenced in this diff but has no covering symbol",
		},
		{
			"quoted tokens and numbers are placeholder'd",
			`Always close "db.Conn" within 30 lines of acquiring it`,
			"always close <id> within <n> lines of acquiring it",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeRuleShape(tc.rule); got != tc.want {
				t.Errorf("NormalizeRuleShape(%q) = %q, want %q", tc.rule, got, tc.want)
			}
		})
	}
}

func TestNormalizeRuleShape_SameRuleDifferentSites_Identical(t *testing.T) {
	a := NormalizeRuleShape("svc::api.Handler → svc::store.Query: layer api may not call store directly")
	b := NormalizeRuleShape("svc::api.Other → svc::store.Write: layer api may not call store directly")
	if a != b {
		t.Errorf("two violations of the SAME rule normalized differently: %q vs %q", a, b)
	}
}

func mkClaim(id, subject, rule string, class core.FindingClass, status core.ClaimStatus, at time.Time, watermark string) core.SemanticClaim {
	return core.SemanticClaim{
		ClaimID: id, ServiceID: "svc", Layer: "findings", Kind: "finding",
		Subject: subject, Rule: rule, Class: class, Status: status,
		Scope: []string{subject}, CompiledAt: at, CompiledAtWatermark: watermark,
	}
}

// TestDerivePatterns_GroupsAcrossDistinctSites is the grouping key's own
// stability proof: three arch_conformance findings at three distinct sites,
// all violating the SAME rule (identical normalized shape), form ONE
// pattern with three members — even though each claim's raw Rule text
// names a different symbol pair.
func TestDerivePatterns_GroupsAcrossDistinctSites(t *testing.T) {
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	claims := []core.SemanticClaim{
		mkClaim("c1", "arch_conformance#api.A", "svc::api.A → svc::store.X: layer api may not call store directly", core.FindingClassConformance, core.ClaimFresh, t0, "wm1"),
		mkClaim("c2", "arch_conformance#api.B", "svc::api.B → svc::store.Y: layer api may not call store directly", core.FindingClassConformance, core.ClaimFresh, t0.Add(24*time.Hour), "wm2"),
		mkClaim("c3", "arch_conformance#api.C", "svc::api.C → svc::store.Z: layer api may not call store directly", core.FindingClassConformance, core.ClaimSuperseded, t0.Add(48*time.Hour), "wm3"),
	}
	patterns := DerivePatterns(claims, nil)
	if len(patterns) != 1 {
		t.Fatalf("patterns = %d, want 1: %+v", len(patterns), patterns)
	}
	p := patterns[0]
	if p.Class != core.FindingClassConformance {
		t.Errorf("Class = %q, want conformance", p.Class)
	}
	if len(p.Members) != 3 {
		t.Fatalf("Members = %d, want 3", len(p.Members))
	}
	if p.SiteCount() != 3 {
		t.Errorf("SiteCount = %d, want 3", p.SiteCount())
	}
	if !p.FirstSeenAt.Equal(t0) || p.FirstSeenWatermark != "wm1" {
		t.Errorf("FirstSeenAt/Watermark = %v/%q, want %v/wm1", p.FirstSeenAt, p.FirstSeenWatermark, t0)
	}
	if !p.LastSeenAt.Equal(t0.Add(48*time.Hour)) || p.LastSeenWatermark != "wm3" {
		t.Errorf("LastSeenAt/Watermark = %v/%q, want %v/wm3", p.LastSeenAt, p.LastSeenWatermark, t0.Add(48*time.Hour))
	}
}

// TestDerivePatterns_SingleSite_NoPattern proves the >=2-distinct-sites
// admission bar: one finding, however many times re-derived, is not
// recurrence.
func TestDerivePatterns_SingleSite_NoPattern(t *testing.T) {
	claims := []core.SemanticClaim{
		mkClaim("c1", "arch_conformance#api.A", "svc::api.A → svc::store.X: layer api may not call store directly", core.FindingClassConformance, core.ClaimFresh, time.Now(), ""),
	}
	patterns := DerivePatterns(claims, nil)
	if len(patterns) != 0 {
		t.Fatalf("patterns = %d, want 0 (single site)", len(patterns))
	}
}

// TestDerivePatterns_DifferentClassNeverMerges proves the grouping key
// includes Class: two findings with IDENTICAL rule text but different
// classes never merge into one pattern.
func TestDerivePatterns_DifferentClassNeverMerges(t *testing.T) {
	claims := []core.SemanticClaim{
		mkClaim("c1", "site#1", "identical text", core.FindingClassSecurity, core.ClaimFresh, time.Now(), ""),
		mkClaim("c2", "site#2", "identical text", core.FindingClassPerformance, core.ClaimFresh, time.Now(), ""),
	}
	patterns := DerivePatterns(claims, nil)
	if len(patterns) != 0 {
		t.Fatalf("patterns = %d, want 0 (each class has only 1 site)", len(patterns))
	}
}

// TestDerivePatterns_ResolutionAndStatus_MemberFieldsPopulated proves
// per-member status and resolving-commit population from claim_events.
func TestDerivePatterns_ResolutionAndStatus_MemberFieldsPopulated(t *testing.T) {
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	claims := []core.SemanticClaim{
		mkClaim("c1", "site#1", "shared rule text here", core.FindingClassSecurity, core.ClaimFresh, t0, "wm1"),
		mkClaim("c2", "site#2", "shared rule text here", core.FindingClassSecurity, core.ClaimSuperseded, t0.Add(time.Hour), "wm2"),
		mkClaim("c3", "site#3", "shared rule text here", core.FindingClassSecurity, core.ClaimRefuted, t0.Add(2*time.Hour), "wm3"),
	}
	events := []core.ClaimEvent{
		{ClaimID: "c2", Verdict: "superseded", At: t0.Add(90 * time.Minute), ResolutionKind: core.ResolutionCodeChange, ResolutionCommit: "abc123"},
		{ClaimID: "c3", Verdict: "refuted", At: t0.Add(3 * time.Hour)}, // no ResolutionKind — refutation is self-describing
	}
	patterns := DerivePatterns(claims, events)
	if len(patterns) != 1 {
		t.Fatalf("patterns = %d, want 1", len(patterns))
	}
	byID := map[string]PatternMember{}
	for _, m := range patterns[0].Members {
		byID[m.ClaimID] = m
	}
	if byID["c1"].Status != MemberLive {
		t.Errorf("c1 status = %q, want live", byID["c1"].Status)
	}
	if byID["c2"].Status != MemberSuperseded || byID["c2"].ResolutionKind != core.ResolutionCodeChange || byID["c2"].ResolutionCommit != "abc123" {
		t.Errorf("c2 = %+v, want superseded/code-change/abc123", byID["c2"])
	}
	if !byID["c2"].HasIndependentCodeChangeResolution() {
		t.Error("c2 should count as an independent code-change resolution")
	}
	if byID["c3"].Status != MemberRefuted || byID["c3"].ResolutionKind != "" {
		t.Errorf("c3 = %+v, want refuted with empty ResolutionKind", byID["c3"])
	}
	if patterns[0].ResolvedCodeChangeCount() != 1 {
		t.Errorf("ResolvedCodeChangeCount = %d, want 1", patterns[0].ResolvedCodeChangeCount())
	}
}

// TestDerivePatterns_Deterministic proves re-derivation over an unchanged
// input always regroups identically (§4 honesty rule 2: never cached, never
// drifts).
func TestDerivePatterns_Deterministic(t *testing.T) {
	claims := []core.SemanticClaim{
		mkClaim("c2", "site#2", "shared text", core.FindingClassSecurity, core.ClaimFresh, time.Now(), ""),
		mkClaim("c1", "site#1", "shared text", core.FindingClassSecurity, core.ClaimFresh, time.Now(), ""),
	}
	p1 := DerivePatterns(claims, nil)
	p2 := DerivePatterns(claims, nil)
	if len(p1) != 1 || len(p2) != 1 {
		t.Fatalf("expected 1 pattern each run, got %d/%d", len(p1), len(p2))
	}
	if p1[0].Members[0].ClaimID != "c1" || p2[0].Members[0].ClaimID != "c1" {
		t.Errorf("members not deterministically sorted: %+v / %+v", p1[0].Members, p2[0].Members)
	}
}

// TestDerivePatterns_EmptyInput_EmptyOutput.
func TestDerivePatterns_EmptyInput_EmptyOutput(t *testing.T) {
	if got := DerivePatterns(nil, nil); len(got) != 0 {
		t.Errorf("got %+v, want empty", got)
	}
}
