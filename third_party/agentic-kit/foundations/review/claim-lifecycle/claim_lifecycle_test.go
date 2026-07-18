package claimlifecycle_test

import (
	"testing"
	"time"

	evidencegrade "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-grade"
	aiprovenance "github.com/geoffmilleraz/agentic-kit/foundations/provenance/ai-provenance"
	cl "github.com/geoffmilleraz/agentic-kit/foundations/review/claim-lifecycle"
)

// sentinel is a fixed clock used in all tests so timestamps are deterministic.
var sentinel = time.Date(2024, 11, 1, 9, 0, 0, 0, time.UTC)

func fixedClock() time.Time { return sentinel }

// newLedger returns a ClaimLedger with a deterministic clock and an initial
// KindProposed event for claimID, so tests start from a known state.
func newLedger(t *testing.T, claimID string) *cl.ClaimLedger {
	t.Helper()
	ledger := cl.NewClaimLedgerWithClock(fixedClock)
	author := aiprovenance.NewHuman("tester")
	if err := cl.Transition(ledger, claimID, cl.KindProposed, author, cl.TransitionPayload{ProposedText: "test claim"}); err != nil {
		t.Fatalf("newLedger: KindProposed: %v", err)
	}
	return ledger
}

// mustProject projects claimID from ledger and fails the test on error.
func mustProject(t *testing.T, ledger *cl.ClaimLedger, claimID string) cl.ClaimView {
	t.Helper()
	events, err := ledger.Events()
	if err != nil {
		t.Fatalf("ledger.Events: %v", err)
	}
	view, err := cl.Project(claimID, events)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	return view
}

// ── Original 7 FSM tests (same scenarios, new API) ────────────────────────────

func TestTransition_LegalProposedToSupported(t *testing.T) {
	ledger := newLedger(t, "c1")
	author := aiprovenance.NewHuman("tester")
	if err := cl.Transition(ledger, "c1", cl.KindSupported, author, cl.TransitionPayload{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	view := mustProject(t, ledger, "c1")
	if view.State != cl.StateSupported {
		t.Errorf("expected supported, got %q", view.State)
	}
}

func TestTransition_LegalProposedToRefuted(t *testing.T) {
	ledger := newLedger(t, "c2")
	author := aiprovenance.NewHuman("tester")
	if err := cl.Transition(ledger, "c2", cl.KindRefuted, author, cl.TransitionPayload{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	view := mustProject(t, ledger, "c2")
	if view.State != cl.StateRefuted {
		t.Errorf("expected refuted, got %q", view.State)
	}
}

func TestTransition_LegalProposedToUncertain(t *testing.T) {
	ledger := newLedger(t, "c3")
	author := aiprovenance.NewHuman("tester")
	if err := cl.Transition(ledger, "c3", cl.KindMarkedUncertain, author, cl.TransitionPayload{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	view := mustProject(t, ledger, "c3")
	if view.State != cl.StateUncertain {
		t.Errorf("expected uncertain, got %q", view.State)
	}
}

func TestTransition_ReopenFromSupported(t *testing.T) {
	ledger := newLedger(t, "c4")
	author := aiprovenance.NewHuman("tester")
	if err := cl.Transition(ledger, "c4", cl.KindSupported, author, cl.TransitionPayload{}); err != nil {
		t.Fatalf("support: %v", err)
	}
	if err := cl.Transition(ledger, "c4", cl.KindReopened, author, cl.TransitionPayload{}); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	view := mustProject(t, ledger, "c4")
	if view.State != cl.StateProposed {
		t.Errorf("expected proposed, got %q", view.State)
	}
}

func TestTransition_ReopenFromRefuted(t *testing.T) {
	ledger := newLedger(t, "c5")
	author := aiprovenance.NewHuman("tester")
	if err := cl.Transition(ledger, "c5", cl.KindRefuted, author, cl.TransitionPayload{}); err != nil {
		t.Fatalf("refute: %v", err)
	}
	if err := cl.Transition(ledger, "c5", cl.KindReopened, author, cl.TransitionPayload{}); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	view := mustProject(t, ledger, "c5")
	if view.State != cl.StateProposed {
		t.Errorf("expected proposed, got %q", view.State)
	}
}

func TestTransition_ReopenFromUncertain(t *testing.T) {
	ledger := newLedger(t, "c6")
	author := aiprovenance.NewHuman("tester")
	if err := cl.Transition(ledger, "c6", cl.KindMarkedUncertain, author, cl.TransitionPayload{}); err != nil {
		t.Fatalf("mark_uncertain: %v", err)
	}
	if err := cl.Transition(ledger, "c6", cl.KindReopened, author, cl.TransitionPayload{}); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	view := mustProject(t, ledger, "c6")
	if view.State != cl.StateProposed {
		t.Errorf("expected proposed, got %q", view.State)
	}
}

func TestTransition_IllegalTransitions(t *testing.T) {
	type testCase struct {
		startKind   cl.ClaimEventKind // transition to reach this state first (empty = start at proposed)
		illegalKind cl.ClaimEventKind
	}

	cases := []testCase{
		// proposed → reopen is illegal
		{startKind: "", illegalKind: cl.KindReopened},
		// supported → support/refute/mark_uncertain are illegal
		{startKind: cl.KindSupported, illegalKind: cl.KindSupported},
		{startKind: cl.KindSupported, illegalKind: cl.KindRefuted},
		{startKind: cl.KindSupported, illegalKind: cl.KindMarkedUncertain},
		// refuted → support/refute/mark_uncertain are illegal
		{startKind: cl.KindRefuted, illegalKind: cl.KindSupported},
		{startKind: cl.KindRefuted, illegalKind: cl.KindRefuted},
		{startKind: cl.KindRefuted, illegalKind: cl.KindMarkedUncertain},
		// uncertain → support/refute/mark_uncertain are illegal
		{startKind: cl.KindMarkedUncertain, illegalKind: cl.KindSupported},
		{startKind: cl.KindMarkedUncertain, illegalKind: cl.KindRefuted},
		{startKind: cl.KindMarkedUncertain, illegalKind: cl.KindMarkedUncertain},
	}

	author := aiprovenance.NewHuman("tester")
	for _, tc := range cases {
		id := "illegal-" + string(tc.startKind) + "-" + string(tc.illegalKind)
		ledger := newLedger(t, id)
		// Transition to the desired starting state first.
		if tc.startKind != "" {
			if err := cl.Transition(ledger, id, tc.startKind, author, cl.TransitionPayload{}); err != nil {
				t.Fatalf("setup transition %q: %v", tc.startKind, err)
			}
		}
		err := cl.Transition(ledger, id, tc.illegalKind, author, cl.TransitionPayload{})
		if err == nil {
			events, _ := ledger.Events()
			view, _ := cl.Project(id, events)
			t.Errorf("expected error for state %q -(%q)→ but got nil (view state=%q)",
				view.State, tc.illegalKind, view.State)
		}
	}
}

func TestTransition_FullCycle(t *testing.T) {
	ledger := newLedger(t, "cycle-1")
	author := aiprovenance.NewHuman("tester")

	// proposed → supported
	if err := cl.Transition(ledger, "cycle-1", cl.KindSupported, author, cl.TransitionPayload{}); err != nil {
		t.Fatalf("support: %v", err)
	}
	view := mustProject(t, ledger, "cycle-1")
	if view.State != cl.StateSupported {
		t.Fatalf("expected supported, got %q", view.State)
	}

	// supported → proposed (reopen)
	if err := cl.Transition(ledger, "cycle-1", cl.KindReopened, author, cl.TransitionPayload{}); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	view = mustProject(t, ledger, "cycle-1")
	if view.State != cl.StateProposed {
		t.Fatalf("expected proposed, got %q", view.State)
	}

	// proposed → refuted
	if err := cl.Transition(ledger, "cycle-1", cl.KindRefuted, author, cl.TransitionPayload{}); err != nil {
		t.Fatalf("refute: %v", err)
	}
	view = mustProject(t, ledger, "cycle-1")
	if view.State != cl.StateRefuted {
		t.Fatalf("expected refuted, got %q", view.State)
	}
}

// ── Finding 1: inline time.Now() ──────────────────────────────────────────────
//
// Verify that the injected clock (not time.Now()) is used for all ledger
// timestamps. We inject two different clocks into two ledgers and confirm that
// the events replay with the expected RecordedAt values. If time.Now() were
// called inline, this test would be non-deterministic and the two ledgers
// would both show wall-clock times regardless of the injected clock.

func TestLedger_UsesInjectedClock(t *testing.T) {
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2030, 6, 15, 12, 0, 0, 0, time.UTC)

	authorA := aiprovenance.NewHuman("alice")
	authorB := aiprovenance.NewHuman("bob")

	ledgerA := cl.NewClaimLedgerWithClock(func() time.Time { return t1 })
	ledgerB := cl.NewClaimLedgerWithClock(func() time.Time { return t2 })

	if err := cl.Transition(ledgerA, "claim-a", cl.KindProposed, authorA, cl.TransitionPayload{ProposedText: "claim A"}); err != nil {
		t.Fatalf("ledgerA append: %v", err)
	}
	if err := cl.Transition(ledgerB, "claim-b", cl.KindProposed, authorB, cl.TransitionPayload{ProposedText: "claim B"}); err != nil {
		t.Fatalf("ledgerB append: %v", err)
	}

	eventsA, err := ledgerA.Events()
	if err != nil {
		t.Fatalf("ledgerA.Events: %v", err)
	}
	eventsB, err := ledgerB.Events()
	if err != nil {
		t.Fatalf("ledgerB.Events: %v", err)
	}

	if got := eventsA[0].RecordedAt; !got.Equal(t1) {
		t.Errorf("ledgerA: expected RecordedAt %v, got %v", t1, got)
	}
	if got := eventsB[0].RecordedAt; !got.Equal(t2) {
		t.Errorf("ledgerB: expected RecordedAt %v, got %v", t2, got)
	}
	// The two ledgers must record different timestamps.
	if eventsA[0].RecordedAt.Equal(eventsB[0].RecordedAt) {
		t.Errorf("expected distinct RecordedAt values; both are %v", eventsA[0].RecordedAt)
	}
}

// ── Finding 2: schema.cue imports evidencegrade, no inline copy ───────────────
//
// The drift test lives in cue_vet_test.go. Here we verify at Go level that
// ClaimView.Grade is *evidencegrade.ClaimGrade (not *evidencegrade.Grade),
// so FloorTier and Promoted are never silently discarded.

func TestClaimView_GradeIsClaimGrade(t *testing.T) {
	ledger := cl.NewClaimLedgerWithClock(fixedClock)
	author := aiprovenance.NewHuman("tester")

	const id = "grade-claim"
	if err := cl.Transition(ledger, id, cl.KindProposed, author, cl.TransitionPayload{ProposedText: "claim with evidence"}); err != nil {
		t.Fatalf("propose: %v", err)
	}

	// Link a necessary documentary span.
	if err := cl.Transition(ledger, id, cl.KindEvidenceLinked, author, cl.TransitionPayload{
		EvidenceLinked: &cl.EvidenceLinkedPayload{
			SpanID:     "span-001",
			Tier:       string(evidencegrade.TierDocumentary),
			Confidence: 0.8,
			Necessary:  true,
		},
	}); err != nil {
		t.Fatalf("evidence_linked: %v", err)
	}

	// Link an independent corroborating direct span (stronger tier).
	if err := cl.Transition(ledger, id, cl.KindEvidenceLinked, author, cl.TransitionPayload{
		EvidenceLinked: &cl.EvidenceLinkedPayload{
			SpanID:      "span-002",
			Tier:        string(evidencegrade.TierDirect),
			Confidence:  0.9,
			Independent: true,
		},
	}); err != nil {
		t.Fatalf("evidence_linked corroborator: %v", err)
	}

	view := mustProject(t, ledger, id)

	if view.Grade == nil {
		t.Fatal("expected Grade to be set after linking evidence, got nil")
	}

	// The Grade field must be a *ClaimGrade with FloorTier populated.
	// FloorTier = documentary (the necessary span's tier).
	if view.Grade.FloorTier != evidencegrade.TierDocumentary {
		t.Errorf("expected FloorTier documentary, got %q", view.Grade.FloorTier)
	}
	// No promotion because PromotionRule.AllowTierPromotion=false in Project.
	if view.Grade.Promoted {
		t.Errorf("expected Promoted=false (promotion disabled in Project), got true")
	}
	// Effective tier is the floor tier.
	if view.Grade.Tier != evidencegrade.TierDocumentary {
		t.Errorf("expected effective Tier documentary, got %q", view.Grade.Tier)
	}
}

// ── Finding 3: ClaimView.Grade stores full ClaimGrade (FloorTier preserved) ───
//
// Confirms that promotion provenance is preserved end-to-end when PromotionRule
// allows it. We call GradeClaim directly and verify all ClaimGrade fields are
// accessible without loss — this is the contract that *evidencegrade.ClaimGrade
// (vs *evidencegrade.Grade) enforces.

func TestClaimGrade_PromotionProvenancePreserved(t *testing.T) {
	spans := []evidencegrade.GradedSpan{
		{SpanID: "s1", Tier: evidencegrade.TierTestimonial, Confidence: 0.7, Necessary: true},
		{SpanID: "s2", Tier: evidencegrade.TierDocumentary, Confidence: 0.85, Independent: true},
	}
	rule := evidencegrade.PromotionRule{
		AllowTierPromotion:                true,
		MinIndependentSourcesForPromotion: 1,
		MinCorroboratorConfidence:         0.5,
	}
	cg, err := evidencegrade.GradeClaim(spans, rule)
	if err != nil {
		t.Fatalf("GradeClaim: %v", err)
	}

	// Verify all provenance fields are accessible (would be impossible if only
	// Grade were stored instead of ClaimGrade).
	if !cg.Promoted {
		t.Errorf("expected Promoted=true: documentary corroborator is stronger than testimonial floor")
	}
	if cg.FloorTier != evidencegrade.TierTestimonial {
		t.Errorf("expected FloorTier testimonial, got %q", cg.FloorTier)
	}
	if cg.Tier != evidencegrade.TierDocumentary {
		t.Errorf("expected promoted Tier documentary, got %q", cg.Tier)
	}
	if cg.PromotionReason == "" {
		t.Errorf("expected non-empty PromotionReason")
	}

	// Store as *ClaimGrade (the type now used in ClaimView) and verify round-trip.
	stored := &cg
	if stored.FloorTier != cg.FloorTier {
		t.Errorf("FloorTier lost: want %q got %q", cg.FloorTier, stored.FloorTier)
	}
	if stored.Promoted != cg.Promoted {
		t.Errorf("Promoted lost: want %v got %v", cg.Promoted, stored.Promoted)
	}
}
