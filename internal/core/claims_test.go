package core

import "testing"

// TestValidVerdict_Vocabulary is plan FL-3: "annotated" joins the documented
// verdict vocabulary additively, and ValidVerdict is the single source of
// truth for recognizing it (and every existing verdict) plus rejecting an
// unknown one.
func TestValidVerdict_Vocabulary(t *testing.T) {
	good := []string{"refuted", "corroborated", "contradicted", "re-anchored", "superseded", "annotated"}
	for _, v := range good {
		if !ValidVerdict(v) {
			t.Errorf("ValidVerdict(%q) = false, want true", v)
		}
	}
	bad := []string{"", "accepted", "ANNOTATED", "refute"}
	for _, v := range bad {
		if ValidVerdict(v) {
			t.Errorf("ValidVerdict(%q) = true, want false", v)
		}
	}
}

// TestClaimLifecycle_ValidAndEffective is GL-13 (docs/plans/
// governance-learning.md, design doc §6): the promotion/retirement axis
// validates its own vocabulary (including "", a legacy/unset claim — a
// valid state, not a malformed one) and EffectiveLifecycle applies the
// owner decision's stated default ("default active") without mutating the
// stored value itself.
func TestClaimLifecycle_ValidAndEffective(t *testing.T) {
	good := []ClaimLifecycle{"", LifecycleCandidate, LifecycleActive, LifecycleRetired}
	for _, l := range good {
		if !l.Valid() {
			t.Errorf("ClaimLifecycle(%q).Valid() = false, want true", l)
		}
	}
	if ClaimLifecycle("promoted").Valid() {
		t.Error(`ClaimLifecycle("promoted").Valid() = true, want false`)
	}

	cases := []struct {
		in   ClaimLifecycle
		want ClaimLifecycle
	}{
		{"", LifecycleActive},
		{LifecycleCandidate, LifecycleCandidate},
		{LifecycleActive, LifecycleActive},
		{LifecycleRetired, LifecycleRetired},
	}
	for _, tc := range cases {
		if got := tc.in.EffectiveLifecycle(); got != tc.want {
			t.Errorf("ClaimLifecycle(%q).EffectiveLifecycle() = %q, want %q", tc.in, got, tc.want)
		}
	}
	// EffectiveLifecycle never mutates the receiver's own stored form —
	// only a caller that assigns the result changes anything.
	var l ClaimLifecycle
	_ = l.EffectiveLifecycle()
	if l != "" {
		t.Errorf("EffectiveLifecycle mutated the receiver: %q", l)
	}
}

// TestResolutionKind_FindingsLoopValues is FL-3: ResolutionExternalClosure
// and ResolutionRevoked join ResolutionCodeChange/ResolutionScopeDisappearance
// additively, distinct string values, none colliding.
func TestResolutionKind_FindingsLoopValues(t *testing.T) {
	if ResolutionExternalClosure != "external-closure" {
		t.Errorf("ResolutionExternalClosure = %q, want \"external-closure\"", ResolutionExternalClosure)
	}
	if ResolutionRevoked != "revoked" {
		t.Errorf("ResolutionRevoked = %q, want \"revoked\"", ResolutionRevoked)
	}
	seen := map[string]bool{}
	for _, k := range []string{ResolutionCodeChange, ResolutionScopeDisappearance, ResolutionExternalClosure, ResolutionRevoked} {
		if seen[k] {
			t.Errorf("duplicate ResolutionKind value %q", k)
		}
		seen[k] = true
	}
}

// TestExternalFinding_ZeroValueCompiles pins the ExternalFinding shape FL-3
// specifies — a compile-time check disguised as a test: every field the
// findings-loop importer and store package need is present with the right
// type.
func TestExternalFinding_ZeroValueCompiles(t *testing.T) {
	var f ExternalFinding
	f.ServiceID = "svc"
	f.ClaimID = "claim1"
	f.Producer = "reviewkit.system-review"
	f.ProducerVersion = "1.0.0"
	f.SourceID = "rule-1"
	f.SourceVersion = 1
	f.PayloadDigest = "deadbeef"
	f.SourceRevision = "abc123"
	f.RuleID = "R1"
	f.RuleVersion = "1"
	f.Remedy = "do the thing"
	f.AnchorPath = "internal/foo/bar.go"
	f.AnchorStart = 10
	f.AnchorEnd = 20
	f.AnchorSymbol = "Bar"
	f.AnchorDigest = "cafef00d"
	f.Unanchored = false
	if f.ServiceID == "" {
		t.Fatal("unreachable")
	}
}
