package findings

import (
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// buildResolvedClaims returns n finding-claims, each superseded with a
// recorded code-change resolution — the fixture shape BuildStatus's
// ResolvedFindingCount counts.
func buildResolvedClaims(n int, class core.FindingClass) ([]core.SemanticClaim, []core.ClaimEvent) {
	var claims []core.SemanticClaim
	var events []core.ClaimEvent
	for i := 0; i < n; i++ {
		id := "resolved-" + string(rune('a'+i))
		claims = append(claims, core.SemanticClaim{
			ClaimID: id, Layer: "findings", Kind: "finding", Class: class,
			Subject: "site#" + string(rune('a'+i)), Status: core.ClaimSuperseded,
			CompiledAt: time.Now(),
		})
		events = append(events, core.ClaimEvent{
			ClaimID: id, Verdict: "superseded", At: time.Now(),
			ResolutionKind: core.ResolutionCodeChange, ResolutionCommit: "commit-" + id,
		})
	}
	return claims, events
}

func TestBuildStatus_ResolvedFindingCount_OnlyCodeChangeResolutionsCount(t *testing.T) {
	claims, events := buildResolvedClaims(3, core.FindingClassSecurity)
	// Add a refuted claim and a scope-disappearance-resolved claim —
	// neither should count as "resolved" precedent.
	claims = append(claims,
		core.SemanticClaim{ClaimID: "refuted-1", Layer: "findings", Kind: "finding", Class: core.FindingClassSecurity, Status: core.ClaimRefuted},
		core.SemanticClaim{ClaimID: "gone-1", Layer: "findings", Kind: "finding", Class: core.FindingClassSecurity, Status: core.ClaimSuperseded},
	)
	events = append(events, core.ClaimEvent{ClaimID: "gone-1", Verdict: "superseded", ResolutionKind: core.ResolutionScopeDisappearance})

	r := BuildStatus(claims, events)
	if r.ResolvedFindingCount != 3 {
		t.Errorf("ResolvedFindingCount = %d, want 3 (refuted and scope-disappearance must not count)", r.ResolvedFindingCount)
	}
}

func TestBuildStatus_DistinctClasses_Sorted(t *testing.T) {
	claims := []core.SemanticClaim{
		{ClaimID: "1", Layer: "findings", Kind: "finding", Class: core.FindingClassSecurity, Status: core.ClaimFresh},
		{ClaimID: "2", Layer: "findings", Kind: "finding", Class: core.FindingClassPerformance, Status: core.ClaimFresh},
		{ClaimID: "3", Layer: "findings", Kind: "finding", Class: core.FindingClassSecurity, Status: core.ClaimFresh}, // dup
	}
	r := BuildStatus(claims, nil)
	want := []core.FindingClass{core.FindingClassPerformance, core.FindingClassSecurity}
	if len(r.DistinctClasses) != len(want) {
		t.Fatalf("DistinctClasses = %v, want %v", r.DistinctClasses, want)
	}
	for i := range want {
		if r.DistinctClasses[i] != want[i] {
			t.Errorf("DistinctClasses[%d] = %q, want %q", i, r.DistinctClasses[i], want[i])
		}
	}
}

func TestBuildStatus_EntryConditionThresholds(t *testing.T) {
	cases := []struct {
		n        int
		wantMet  bool
		wantNote string
	}{
		{14, false, "not yet met"},
		{15, true, "below its top"},
		{19, true, "below its top"},
		{20, true, "at or above the top"},
	}
	for _, tc := range cases {
		claims, events := buildResolvedClaims(tc.n, core.FindingClassSecurity)
		r := BuildStatus(claims, events)
		if r.ResolvedFindingCount != tc.n {
			t.Fatalf("n=%d: ResolvedFindingCount = %d, want %d", tc.n, r.ResolvedFindingCount, tc.n)
		}
		if r.EntryConditionMet != tc.wantMet {
			t.Errorf("n=%d: EntryConditionMet = %v, want %v", tc.n, r.EntryConditionMet, tc.wantMet)
		}
		if !strings.Contains(r.Note, tc.wantNote) {
			t.Errorf("n=%d: Note = %q, want it to contain %q", tc.n, r.Note, tc.wantNote)
		}
		if r.EntryBarLow != EntryBarLow || r.EntryBarHigh != EntryBarHigh {
			t.Errorf("n=%d: bar = %d-%d, want %d-%d", tc.n, r.EntryBarLow, r.EntryBarHigh, EntryBarLow, EntryBarHigh)
		}
	}
}

func TestBuildStatus_PatternsWithThreeResolvedMembers(t *testing.T) {
	// A pattern needs >=2 distinct sites to exist at all (DerivePatterns'
	// own bar) and >=3 resolved members to count here — build 3 resolved
	// members at 3 distinct sites, all sharing rule text/class.
	t0 := time.Now()
	claims := []core.SemanticClaim{
		mkClaim("p1", "site#1", "shared", core.FindingClassSecurity, core.ClaimSuperseded, t0, "wm"),
		mkClaim("p2", "site#2", "shared", core.FindingClassSecurity, core.ClaimSuperseded, t0, "wm"),
		mkClaim("p3", "site#3", "shared", core.FindingClassSecurity, core.ClaimSuperseded, t0, "wm"),
	}
	events := []core.ClaimEvent{
		{ClaimID: "p1", Verdict: "superseded", At: t0, ResolutionKind: core.ResolutionCodeChange, ResolutionCommit: "c1"},
		{ClaimID: "p2", Verdict: "superseded", At: t0, ResolutionKind: core.ResolutionCodeChange, ResolutionCommit: "c2"},
		{ClaimID: "p3", Verdict: "superseded", At: t0, ResolutionKind: core.ResolutionCodeChange, ResolutionCommit: "c3"},
	}
	r := BuildStatus(claims, events)
	if r.PatternsWithThreeResolvedMembers != 1 {
		t.Errorf("PatternsWithThreeResolvedMembers = %d, want 1", r.PatternsWithThreeResolvedMembers)
	}
}

func TestBuildStatus_EmptyInput_HonestZeros(t *testing.T) {
	r := BuildStatus(nil, nil)
	if r.ResolvedFindingCount != 0 || len(r.DistinctClasses) != 0 || r.PatternsWithThreeResolvedMembers != 0 {
		t.Errorf("got %+v, want all zeros", r)
	}
	if r.EntryConditionMet {
		t.Error("EntryConditionMet should be false with zero findings")
	}
	if !strings.Contains(r.Note, "not yet met") {
		t.Errorf("Note = %q, want it to say the entry condition is not yet met", r.Note)
	}
}
