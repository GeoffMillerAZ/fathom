package review_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
)

// TestSupersedeSuccessor_MatchesSameSubjectDifferentContent (SU-20, ADR-041
// decision 7): a re-run that re-emits the SAME dim/subject (same file,
// same symbol) under a DIFFERENT message (so AttachClaims mints a
// DIFFERENT content-addressed ClaimID) is the genuine successor —
// SupersedeSuccessor must resolve the predecessor's own Subject to this
// run's new ClaimID.
func TestSupersedeSuccessor_MatchesSameSubjectDifferentContent(t *testing.T) {
	v := review.Verdict{Service: "svc", ChangedFiles: []string{"api/handler.go"}}
	v.Dimensions = []review.Dimension{
		{
			Name:    review.DimArch,
			Checked: true,
			Findings: []review.Finding{
				{Kind: "arch_violation", Symbol: "api.Handler", File: "api/handler.go", Citation: "api/handler.go#L7-L7", Message: "reworded violation text"},
			},
		},
	}
	claims, _ := review.AttachClaims(&v, review.PersistInput{
		Now:              fixedNow,
		Watermark:        "wm",
		ScopeFingerprint: fakeFP,
	})
	if len(claims) != 1 {
		t.Fatalf("AttachClaims: got %d claims, want 1", len(claims))
	}
	newID := v.Dimensions[0].Findings[0].ClaimID
	if newID == "" {
		t.Fatal("AttachClaims left ClaimID empty")
	}

	predecessorSubject := review.DimArch + "#api.Handler"
	got, ok := review.SupersedeSuccessor(v, predecessorSubject)
	if !ok {
		t.Fatal("SupersedeSuccessor: ok = false, want true (same subject re-emitted)")
	}
	if got != newID {
		t.Errorf("SupersedeSuccessor = %q, want the newly-minted %q", got, newID)
	}
}

// TestSupersedeSuccessor_NoSuccessorWhenSubjectAbsent pins the OTHER case:
// a finding that simply stopped firing (fixed, nothing re-emitted for that
// subject) has no successor — ok must be false, never a guess.
func TestSupersedeSuccessor_NoSuccessorWhenSubjectAbsent(t *testing.T) {
	v := review.Verdict{Service: "svc"}
	v.Dimensions = []review.Dimension{
		{Name: review.DimArch, Checked: true, Findings: nil},
	}
	_, ok := review.SupersedeSuccessor(v, review.DimArch+"#api.Handler")
	if ok {
		t.Error("SupersedeSuccessor: ok = true, want false (no finding shares that subject)")
	}
}
