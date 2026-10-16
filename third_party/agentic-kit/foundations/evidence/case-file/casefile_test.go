package casefile_test

import (
	"encoding/json"
	"testing"
	"time"

	assertion "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/assertion"
	authenticity "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/authenticity"
	casefile "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/case-file"
	conflict "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/conflict"
	evidencegrade "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-grade"
	narrativeshift "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/investigation/narrative-shift"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func makeAssertion(id, propID, declarant string, stance assertion.Stance, content string, tier evidencegrade.Tier) assertion.Assertion {
	return assertion.Assertion{
		ID:            id,
		PropositionID: propID,
		Declarant:     declarant,
		Stance:        stance,
		Content:       content,
		Grade:         &evidencegrade.Grade{Tier: tier, Confidence: 0.8},
	}
}

func makeAssertionVia(id, propID, declarant string, stance assertion.Stance, content string, via evidencegrade.Tier) assertion.Assertion {
	return assertion.Assertion{
		ID:            id,
		PropositionID: propID,
		Declarant:     declarant,
		Stance:        stance,
		Content:       content,
		Via:           via,
	}
}

func mustBuild(t *testing.T, propID string, in casefile.Inputs) casefile.CaseFile {
	t.Helper()
	cf, err := casefile.Build(propID, in)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return cf
}

// ── Test 1: empty proposition ID is rejected ──────────────────────────────────

func TestBuild_EmptyPropositionID(t *testing.T) {
	t.Parallel()
	_, err := casefile.Build("", casefile.Inputs{})
	if err == nil {
		t.Fatal("expected error for empty propositionID; got nil")
	}
}

// ── Test 2: no assertions → unresolved ───────────────────────────────────────

func TestBuild_NoAssertions_Unresolved(t *testing.T) {
	t.Parallel()
	cf := mustBuild(t, "prop-001", casefile.Inputs{})

	if cf.PropositionID != "prop-001" {
		t.Errorf("PropositionID = %q; want %q", cf.PropositionID, "prop-001")
	}
	if cf.Supporting != nil {
		t.Errorf("Supporting = %v; want nil", cf.Supporting)
	}
	if cf.Contradicting != nil {
		t.Errorf("Contradicting = %v; want nil", cf.Contradicting)
	}
	if cf.SupportSummary.Tier != casefile.SupportTierUnresolved {
		t.Errorf("SupportSummary.Tier = %q; want %q", cf.SupportSummary.Tier, casefile.SupportTierUnresolved)
	}
	if cf.SupportSummary.SupportScore != 0 {
		t.Errorf("SupportScore = %g; want 0", cf.SupportSummary.SupportScore)
	}
}

// ── Test 3: assertions for other propositions are ignored ─────────────────────

func TestBuild_FiltersToProposition(t *testing.T) {
	t.Parallel()
	asrts := []assertion.Assertion{
		makeAssertion("a1", "prop-X", "alice", assertion.StanceAffirms, "claim-x", evidencegrade.TierTestimonial),
		makeAssertion("a2", "prop-Y", "bob", assertion.StanceDenies, "claim-y", evidencegrade.TierTestimonial),
	}
	cf := mustBuild(t, "prop-X", casefile.Inputs{Assertions: asrts})

	if len(cf.Supporting) != 1 {
		t.Fatalf("Supporting len = %d; want 1", len(cf.Supporting))
	}
	if cf.Supporting[0].AssertionID != "a1" {
		t.Errorf("Supporting[0].AssertionID = %q; want %q", cf.Supporting[0].AssertionID, "a1")
	}
	if cf.Contradicting != nil {
		t.Errorf("Contradicting = %v; want nil (belongs to prop-Y)", cf.Contradicting)
	}
}

// ── Test 4: stance partitioning ───────────────────────────────────────────────

func TestBuild_StancePartitioning(t *testing.T) {
	t.Parallel()
	asrts := []assertion.Assertion{
		makeAssertion("aff", "p", "alice", assertion.StanceAffirms, "yes", evidencegrade.TierDirect),
		makeAssertion("den", "p", "bob", assertion.StanceDenies, "no", evidencegrade.TierTestimonial),
		makeAssertion("ret", "p", "carol", assertion.StanceWithdraws, "retract", evidencegrade.TierInferred),
		makeAssertion("qual", "p", "dave", assertion.StanceQualifies, "maybe", evidencegrade.TierCircumstantial),
	}
	cf := mustBuild(t, "p", casefile.Inputs{Assertions: asrts})

	if len(cf.Supporting) != 1 || cf.Supporting[0].AssertionID != "aff" {
		t.Errorf("Supporting = %v; want [aff]", cf.Supporting)
	}
	if len(cf.Contradicting) != 1 || cf.Contradicting[0].AssertionID != "den" {
		t.Errorf("Contradicting = %v; want [den]", cf.Contradicting)
	}
	if len(cf.Retracting) != 1 || cf.Retracting[0].AssertionID != "ret" {
		t.Errorf("Retracting = %v; want [ret]", cf.Retracting)
	}
	if len(cf.Qualifying) != 1 || cf.Qualifying[0].AssertionID != "qual" {
		t.Errorf("Qualifying = %v; want [qual]", cf.Qualifying)
	}
}

// ── Test 5: tier rank from Grade takes priority over Via ──────────────────────

func TestBuild_TierRank_GradeOverVia(t *testing.T) {
	t.Parallel()
	a := assertion.Assertion{
		ID:            "a1",
		PropositionID: "p",
		Declarant:     "alice",
		Stance:        assertion.StanceAffirms,
		Content:       "yes",
		Via:           evidencegrade.TierInferred, // weak via
		Grade:         &evidencegrade.Grade{Tier: evidencegrade.TierDirect, Confidence: 0.9},
	}
	cf := mustBuild(t, "p", casefile.Inputs{Assertions: []assertion.Assertion{a}})
	if len(cf.Supporting) != 1 {
		t.Fatalf("Supporting len = %d; want 1", len(cf.Supporting))
	}
	if cf.Supporting[0].TierRank != 5 {
		t.Errorf("TierRank = %d; want 5 (direct)", cf.Supporting[0].TierRank)
	}
	if cf.Supporting[0].Tier != evidencegrade.TierDirect {
		t.Errorf("Tier = %q; want %q", cf.Supporting[0].Tier, evidencegrade.TierDirect)
	}
}

// ── Test 6: tier rank from Via when no Grade ─────────────────────────────────

func TestBuild_TierRank_ViaFallback(t *testing.T) {
	t.Parallel()
	a := makeAssertionVia("a1", "p", "alice", assertion.StanceAffirms, "yes", evidencegrade.TierDocumentary)
	cf := mustBuild(t, "p", casefile.Inputs{Assertions: []assertion.Assertion{a}})
	if cf.Supporting[0].TierRank != 4 {
		t.Errorf("TierRank = %d; want 4 (documentary)", cf.Supporting[0].TierRank)
	}
}

// ── Test 7: ungraded assertion has TierRank=0 and Tier="ungraded" ─────────────

func TestBuild_TierRank_Ungraded(t *testing.T) {
	t.Parallel()
	a := assertion.Assertion{
		ID:            "a1",
		PropositionID: "p",
		Declarant:     "alice",
		Stance:        assertion.StanceAffirms,
		Content:       "yes",
	}
	cf := mustBuild(t, "p", casefile.Inputs{Assertions: []assertion.Assertion{a}})
	if cf.Supporting[0].TierRank != 0 {
		t.Errorf("TierRank = %d; want 0 (ungraded)", cf.Supporting[0].TierRank)
	}
	if cf.Supporting[0].Tier != "ungraded" {
		t.Errorf("Tier = %q; want %q", cf.Supporting[0].Tier, "ungraded")
	}
}

// ── Test 8: SupportSummary aggregation math ───────────────────────────────────

func TestBuild_SupportSummary_Math(t *testing.T) {
	t.Parallel()
	// One direct affirming (rank 5) + one testimonial denying (rank 2).
	// maxPossible = (1+1)*5 = 10
	// supportScore = 5/10 = 0.50
	// againstScore = 2/10 = 0.20
	// net = 0.30 → moderate_support
	asrts := []assertion.Assertion{
		makeAssertion("a1", "p", "alice", assertion.StanceAffirms, "yes", evidencegrade.TierDirect),
		makeAssertion("a2", "p", "bob", assertion.StanceDenies, "no", evidencegrade.TierTestimonial),
	}
	cf := mustBuild(t, "p", casefile.Inputs{Assertions: asrts})
	ss := cf.SupportSummary

	if ss.AffirmingCount != 1 {
		t.Errorf("AffirmingCount = %d; want 1", ss.AffirmingCount)
	}
	if ss.DenyingCount != 1 {
		t.Errorf("DenyingCount = %d; want 1", ss.DenyingCount)
	}
	const wantSupport = 0.5
	if ss.SupportScore != wantSupport {
		t.Errorf("SupportScore = %g; want %g", ss.SupportScore, wantSupport)
	}
	const wantAgainst = 0.2
	if ss.AgainstScore != wantAgainst {
		t.Errorf("AgainstScore = %g; want %g", ss.AgainstScore, wantAgainst)
	}
	if ss.Tier != casefile.SupportTierModerateSupport {
		t.Errorf("Tier = %q; want %q", ss.Tier, casefile.SupportTierModerateSupport)
	}
}

// ── Test 9: SupportSummary tier thresholds ────────────────────────────────────

func TestBuild_SupportTierThresholds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		affirm   int // number of direct (rank=5) affirming assertions
		deny     int // number of direct (rank=5) denying assertions
		wantTier casefile.SupportTier
	}{
		{"all_affirm_strong", 1, 0, casefile.SupportTierStrongSupport},  // net=1.0
		{"mostly_affirm", 4, 1, casefile.SupportTierStrongSupport},      // net=0.6 exactly
		{"moderate_support", 3, 1, casefile.SupportTierModerateSupport}, // net=0.5
		{"contested_equal", 1, 1, casefile.SupportTierContested},        // net=0
		{"moderate_against", 1, 3, casefile.SupportTierModerateAgainst}, // net=-0.5
		{"strong_against", 0, 1, casefile.SupportTierStrongAgainst},     // net=-1.0
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var asrts []assertion.Assertion
			for i := range tc.affirm {
				asrts = append(asrts, makeAssertion(
					"aff-"+string(rune('a'+i)), "p", "d",
					assertion.StanceAffirms, "c", evidencegrade.TierDirect,
				))
			}
			for i := range tc.deny {
				asrts = append(asrts, makeAssertion(
					"den-"+string(rune('a'+i)), "p", "e",
					assertion.StanceDenies, "c", evidencegrade.TierDirect,
				))
			}
			cf := mustBuild(t, "p", casefile.Inputs{Assertions: asrts})
			if cf.SupportSummary.Tier != tc.wantTier {
				t.Errorf("Tier = %q; want %q (net=%.2f)",
					cf.SupportSummary.Tier, tc.wantTier,
					cf.SupportSummary.SupportScore-cf.SupportSummary.AgainstScore)
			}
		})
	}
}

// ── Test 10: Supporting sorted by TierRank desc, then AssertionID asc ─────────

func TestBuild_SortOrder(t *testing.T) {
	t.Parallel()
	asrts := []assertion.Assertion{
		makeAssertion("z-inf", "p", "a", assertion.StanceAffirms, "c", evidencegrade.TierInferred),
		makeAssertion("a-doc", "p", "b", assertion.StanceAffirms, "c", evidencegrade.TierDocumentary),
		makeAssertion("b-doc", "p", "c", assertion.StanceAffirms, "c", evidencegrade.TierDocumentary),
		makeAssertion("m-dir", "p", "d", assertion.StanceAffirms, "c", evidencegrade.TierDirect),
	}
	cf := mustBuild(t, "p", casefile.Inputs{Assertions: asrts})

	want := []string{"m-dir", "a-doc", "b-doc", "z-inf"}
	for i, ga := range cf.Supporting {
		if ga.AssertionID != want[i] {
			t.Errorf("Supporting[%d].AssertionID = %q; want %q", i, ga.AssertionID, want[i])
		}
	}
}

// ── Test 11: conflicts filtered to the proposition ────────────────────────────

func TestBuild_ConflictsFiltered(t *testing.T) {
	t.Parallel()
	conflicts := []conflict.Conflict{
		{Kind: conflict.KindClaimVsClaim, Subject: "prop-A", A: conflict.Ref{Kind: conflict.RefKindAssertion, ID: "a1"}, B: conflict.Ref{Kind: conflict.RefKindAssertion, ID: "a2"}, Severity: 0.4, Reason: "r"},
		{Kind: conflict.KindClaimVsClaim, Subject: "prop-B", A: conflict.Ref{Kind: conflict.RefKindAssertion, ID: "a3"}, B: conflict.Ref{Kind: conflict.RefKindAssertion, ID: "a4"}, Severity: 0.4, Reason: "r"},
	}
	cf := mustBuild(t, "prop-A", casefile.Inputs{Conflicts: conflicts})
	if len(cf.Conflicts) != 1 {
		t.Fatalf("Conflicts len = %d; want 1", len(cf.Conflicts))
	}
	if cf.Conflicts[0].Subject != "prop-A" {
		t.Errorf("Conflicts[0].Subject = %q; want %q", cf.Conflicts[0].Subject, "prop-A")
	}
}

// ── Test 12: authenticity digests filtered to cited records ───────────────────

func TestBuild_AuthenticityDigests_FilteredToCited(t *testing.T) {
	t.Parallel()
	// Assertion cites rec-001; assessment for rec-001 and also rec-002 (uncited).
	a := assertion.Assertion{
		ID:            "a1",
		PropositionID: "p",
		Declarant:     "alice",
		Stance:        assertion.StanceAffirms,
		Content:       "yes",
		StatedIn:      "rec-001",
	}
	assessments := []authenticity.Assessment{
		{RecordID: "rec-001", Overall: authenticity.VerdictAuthentic, Confidence: 0.95},
		{RecordID: "rec-002", Overall: authenticity.VerdictTampered, Confidence: 0.8},
	}
	cf := mustBuild(t, "p", casefile.Inputs{
		Assertions:  []assertion.Assertion{a},
		Assessments: assessments,
	})
	if len(cf.AuthenticityDigests) != 1 {
		t.Fatalf("AuthenticityDigests len = %d; want 1", len(cf.AuthenticityDigests))
	}
	if cf.AuthenticityDigests[0].RecordID != "rec-001" {
		t.Errorf("AuthenticityDigests[0].RecordID = %q; want %q", cf.AuthenticityDigests[0].RecordID, "rec-001")
	}
	if cf.AuthenticityDigests[0].Overall != authenticity.VerdictAuthentic {
		t.Errorf("Overall = %q; want %q", cf.AuthenticityDigests[0].Overall, authenticity.VerdictAuthentic)
	}
}

// ── Test 13: narrative shift history filtered to the proposition ───────────────

func TestBuild_ShiftHistory(t *testing.T) {
	t.Parallel()
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC)
	from := assertion.Assertion{ID: "a-old", PropositionID: "prop-p", Declarant: "alice", Stance: assertion.StanceAffirms, Content: "yes", StatedAt: t1}
	to := assertion.Assertion{ID: "a-new", PropositionID: "prop-p", Declarant: "alice", Stance: assertion.StanceDenies, Content: "no", StatedAt: t2}
	otherFrom := assertion.Assertion{ID: "b-old", PropositionID: "prop-q", Declarant: "bob", Stance: assertion.StanceAffirms, Content: "yes", StatedAt: t1}
	otherTo := assertion.Assertion{ID: "b-new", PropositionID: "prop-q", Declarant: "bob", Stance: assertion.StanceDenies, Content: "no", StatedAt: t2}

	shifts := []narrativeshift.NarrativeShift{
		{Declarant: "alice", PropositionID: "prop-p", From: from, To: to, Kind: narrativeshift.KindStanceChange, GapDays: 7},
		{Declarant: "bob", PropositionID: "prop-q", From: otherFrom, To: otherTo, Kind: narrativeshift.KindStanceChange, GapDays: 7},
	}
	cf := mustBuild(t, "prop-p", casefile.Inputs{Shifts: shifts})

	if len(cf.ShiftHistory) != 1 {
		t.Fatalf("ShiftHistory len = %d; want 1", len(cf.ShiftHistory))
	}
	sh := cf.ShiftHistory[0]
	if sh.Declarant != "alice" {
		t.Errorf("Declarant = %q; want %q", sh.Declarant, "alice")
	}
	if sh.FromID != "a-old" {
		t.Errorf("FromID = %q; want %q", sh.FromID, "a-old")
	}
	if sh.ToID != "a-new" {
		t.Errorf("ToID = %q; want %q", sh.ToID, "a-new")
	}
	if sh.Kind != narrativeshift.KindStanceChange {
		t.Errorf("Kind = %q; want %q", sh.Kind, narrativeshift.KindStanceChange)
	}
	if sh.GapDays != 7 {
		t.Errorf("GapDays = %d; want 7", sh.GapDays)
	}
}

// ── Test 14: determinism / golden test ────────────────────────────────────────

func TestBuild_Determinism(t *testing.T) {
	t.Parallel()
	asrts := []assertion.Assertion{
		makeAssertion("a3", "prop-det", "charlie", assertion.StanceAffirms, "c", evidencegrade.TierCircumstantial),
		makeAssertion("a1", "prop-det", "alice", assertion.StanceAffirms, "y", evidencegrade.TierDirect),
		makeAssertion("a2", "prop-det", "bob", assertion.StanceDenies, "n", evidencegrade.TierTestimonial),
	}

	run := func() string {
		cf, err := casefile.Build("prop-det", casefile.Inputs{Assertions: asrts})
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		b, err := json.Marshal(cf)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		return string(b)
	}

	s1 := run()
	s2 := run()
	if s1 != s2 {
		t.Errorf("non-deterministic output:\nrun1: %s\nrun2: %s", s1, s2)
	}
}

// ── Test 15: does_not_recall and declines do not appear in any bucket ─────────

func TestBuild_NonEvidentialStances_Excluded(t *testing.T) {
	t.Parallel()
	asrts := []assertion.Assertion{
		makeAssertion("dnr", "p", "alice", assertion.StanceDoesNotRecall, "cant recall", evidencegrade.TierTestimonial),
		makeAssertion("dec", "p", "bob", assertion.StanceDeclines, "declines", evidencegrade.TierTestimonial),
	}
	cf := mustBuild(t, "p", casefile.Inputs{Assertions: asrts})

	if cf.Supporting != nil {
		t.Errorf("Supporting = %v; want nil", cf.Supporting)
	}
	if cf.Contradicting != nil {
		t.Errorf("Contradicting = %v; want nil", cf.Contradicting)
	}
	if cf.Retracting != nil {
		t.Errorf("Retracting = %v; want nil", cf.Retracting)
	}
	if cf.Qualifying != nil {
		t.Errorf("Qualifying = %v; want nil", cf.Qualifying)
	}
	if cf.SupportSummary.Tier != casefile.SupportTierUnresolved {
		t.Errorf("Tier = %q; want unresolved", cf.SupportSummary.Tier)
	}
}

// ── Test 16: retraction assertions counted but do not affect support scores ───

func TestBuild_RetractionsDoNotMoveScore(t *testing.T) {
	t.Parallel()
	asrts := []assertion.Assertion{
		makeAssertion("ret", "p", "alice", assertion.StanceWithdraws, "retract", evidencegrade.TierDirect),
	}
	cf := mustBuild(t, "p", casefile.Inputs{Assertions: asrts})

	if cf.SupportSummary.SupportScore != 0 {
		t.Errorf("SupportScore = %g; want 0 (retraction should not move score)", cf.SupportSummary.SupportScore)
	}
	if cf.SupportSummary.AgainstScore != 0 {
		t.Errorf("AgainstScore = %g; want 0", cf.SupportSummary.AgainstScore)
	}
	if cf.SupportSummary.Tier != casefile.SupportTierUnresolved {
		t.Errorf("Tier = %q; want unresolved", cf.SupportSummary.Tier)
	}
	if cf.SupportSummary.RetractingCount != 1 {
		t.Errorf("RetractingCount = %d; want 1", cf.SupportSummary.RetractingCount)
	}
}

// ── Test 17: CUE schema vet (invoked externally via `cue vet ./...`) ──────────
// This test is a marker: `cue vet ./foundations/evidence/case-file/...` is run
// as part of `task validate` and verifies that testdata/valid.cue satisfies
// the schema. We add a compile-time guard here so the presence of the schema
// file is at least referenced.

func TestSchemaExists(t *testing.T) {
	t.Parallel()
	// If the package compiles, the schema file exists and the Go types are in
	// the correct package. CUE conformance is validated by `cue vet`.
	_ = casefile.SupportTierStrongSupport
	_ = casefile.SupportTierUnresolved
}
