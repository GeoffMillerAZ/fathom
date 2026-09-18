package findings

import (
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestDisagreementsByRule_ExternalClosureVsFresh_DifferentProducers(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	claims := []core.SemanticClaim{
		{ClaimID: "fresh1", Layer: "findings", Kind: "finding", Rule: "Do X.", Subject: "arch_conformance#api.A", Scope: []string{"internal/api/a.go"}, Status: core.ClaimFresh, CompiledAt: t0},
		{ClaimID: "closed1", Layer: "findings", Kind: "finding", Rule: "Do X.", Subject: "ext:reviewkit.security-review:f1", Scope: []string{"internal/api/a.go"}, Status: core.ClaimSuperseded, CompiledAt: t0},
	}
	events := []core.ClaimEvent{
		{ClaimID: "closed1", Verdict: "superseded", ResolutionKind: "external-closure", At: t0},
	}
	if got := disagreementsByRule(claims, events); got != 1 {
		t.Errorf("disagreementsByRule = %d, want 1", got)
	}
}

func TestDisagreementsByRule_SameProducer_NotADisagreement(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	claims := []core.SemanticClaim{
		{ClaimID: "fresh1", Layer: "findings", Kind: "finding", Rule: "Do X.", Subject: "ext:reviewkit.security-review:f1", Scope: []string{"a.go"}, Status: core.ClaimFresh, CompiledAt: t0},
		{ClaimID: "closed1", Layer: "findings", Kind: "finding", Rule: "Do X.", Subject: "ext:reviewkit.security-review:f2", Scope: []string{"a.go"}, Status: core.ClaimSuperseded, CompiledAt: t0},
	}
	events := []core.ClaimEvent{
		{ClaimID: "closed1", Verdict: "superseded", ResolutionKind: "external-closure", At: t0},
	}
	if got := disagreementsByRule(claims, events); got != 0 {
		t.Errorf("disagreementsByRule = %d, want 0 (same producer on both sides)", got)
	}
}

func TestDisagreementsByRule_CodeChangeResolution_NotADisagreement(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	claims := []core.SemanticClaim{
		{ClaimID: "fresh1", Layer: "findings", Kind: "finding", Rule: "Do X.", Subject: "arch_conformance#api.A", Scope: []string{"a.go"}, Status: core.ClaimFresh, CompiledAt: t0},
		{ClaimID: "closed1", Layer: "findings", Kind: "finding", Rule: "Do X.", Subject: "ext:reviewkit.security-review:f1", Scope: []string{"a.go"}, Status: core.ClaimSuperseded, CompiledAt: t0},
	}
	events := []core.ClaimEvent{
		{ClaimID: "closed1", Verdict: "superseded", ResolutionKind: core.ResolutionCodeChange, At: t0},
	}
	if got := disagreementsByRule(claims, events); got != 0 {
		t.Errorf("disagreementsByRule = %d, want 0 (a code-change resolution is agreement, not disagreement)", got)
	}
}

func TestDisagreementsByRule_DifferentPathOrShape_NotGrouped(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	claims := []core.SemanticClaim{
		{ClaimID: "fresh1", Layer: "findings", Kind: "finding", Rule: "Do X.", Subject: "arch_conformance#api.A", Scope: []string{"a.go"}, Status: core.ClaimFresh, CompiledAt: t0},
		{ClaimID: "closed1", Layer: "findings", Kind: "finding", Rule: "Do X.", Subject: "ext:reviewkit.security-review:f1", Scope: []string{"b.go"}, Status: core.ClaimSuperseded, CompiledAt: t0},
	}
	events := []core.ClaimEvent{
		{ClaimID: "closed1", Verdict: "superseded", ResolutionKind: "external-closure", At: t0},
	}
	if got := disagreementsByRule(claims, events); got != 0 {
		t.Errorf("disagreementsByRule = %d, want 0 (different anchor path)", got)
	}
}

func TestDisagreementsByRule_EmptyInput(t *testing.T) {
	if got := disagreementsByRule(nil, nil); got != 0 {
		t.Errorf("disagreementsByRule(nil, nil) = %d, want 0", got)
	}
}

func TestBuildStatus_CandidatesAndDisagreements(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	claims := []core.SemanticClaim{
		mkClaim("c1", "a", "Do a thing.", core.FindingClassCorrectness, core.ClaimFresh, t0, "wm"),
		mkClaim("c2", "b", "Do a thing.", core.FindingClassCorrectness, core.ClaimFresh, t0, "wm"),
		mkClaim("c3", "c", "Do a thing.", core.FindingClassCorrectness, core.ClaimFresh, t0, "wm"),
		{ClaimID: "fresh1", Layer: "findings", Kind: "finding", Rule: "Do X.", Subject: "arch_conformance#api.A", Scope: []string{"a.go"}, Status: core.ClaimFresh, CompiledAt: t0},
		{ClaimID: "closed1", Layer: "findings", Kind: "finding", Rule: "Do X.", Subject: "ext:reviewkit.security-review:f1", Scope: []string{"a.go"}, Status: core.ClaimSuperseded, CompiledAt: t0},
	}
	events := []core.ClaimEvent{
		{ClaimID: "closed1", Verdict: "superseded", ResolutionKind: "external-closure", At: t0},
	}
	r := BuildStatus(claims, events)
	if r.Candidates != 1 {
		t.Errorf("Candidates = %d, want 1 (the 3-site 'Do a thing.' pattern)", r.Candidates)
	}
	if r.DisagreementsByRule != 1 {
		t.Errorf("DisagreementsByRule = %d, want 1", r.DisagreementsByRule)
	}
}
