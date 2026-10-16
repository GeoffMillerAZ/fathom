package conflict_test

import (
	"reflect"
	"testing"
	"time"

	assertion "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/assertion"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/conflict"
	evidencegrade "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-grade"
	evidencespan "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-span"
)

// helpers

func testimonialGrade(conf float64) *evidencegrade.Grade {
	return &evidencegrade.Grade{Tier: evidencegrade.TierTestimonial, Confidence: conf}
}

func directGrade(conf float64) *evidencegrade.Grade {
	return &evidencegrade.Grade{Tier: evidencegrade.TierDirect, Confidence: conf}
}

func documentaryGrade(conf float64) *evidencegrade.Grade {
	return &evidencegrade.Grade{Tier: evidencegrade.TierDocumentary, Confidence: conf}
}

func mkAssertion(id, propID, declarant string, stance assertion.Stance, content string, grade *evidencegrade.Grade, statedAt time.Time) assertion.Assertion {
	return assertion.Assertion{
		ID:            id,
		PropositionID: propID,
		Declarant:     declarant,
		Stance:        stance,
		Content:       content,
		Grade:         grade,
		StatedAt:      statedAt,
	}
}

func mkSpan(id, entityID, field, value string, grade *evidencegrade.Grade) evidencespan.Span {
	return evidencespan.Span{
		ID:       id,
		EntityID: entityID,
		Field:    field,
		Value:    value,
		Grade:    grade,
	}
}

var (
	t0 = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 = time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
)

// ----------------------------------------------------------------------------
// Test 1: one fixture per ConflictKind → produces exactly that kind.
// ----------------------------------------------------------------------------

func TestDetect_OneFixturePerKind(t *testing.T) {
	cases := []struct {
		name       string
		assertions []assertion.Assertion
		spans      []evidencespan.Span
		wantKind   conflict.ConflictKind
		wantCount  int
	}{
		{
			name: "self_inconsistency",
			assertions: []assertion.Assertion{
				mkAssertion("a1", "prop-p1", "alice", assertion.StanceAffirms, "yes", testimonialGrade(0.8), t0),
				mkAssertion("a2", "prop-p1", "alice", assertion.StanceAffirms, "no", testimonialGrade(0.8), t1),
			},
			wantKind:  conflict.KindSelfInconsistency,
			wantCount: 1,
		},
		{
			name: "recantation",
			assertions: []assertion.Assertion{
				mkAssertion("a1", "prop-p2", "bob", assertion.StanceAffirms, "I was there", testimonialGrade(0.8), t0),
				mkAssertion("a2", "prop-p2", "bob", assertion.StanceRecants, "I was wrong", testimonialGrade(0.6), t1),
			},
			wantKind:  conflict.KindRecantation,
			wantCount: 1,
		},
		{
			name: "claim_vs_claim",
			assertions: []assertion.Assertion{
				mkAssertion("a1", "prop-p3", "alice", assertion.StanceAffirms, "yes", testimonialGrade(0.8), t0),
				mkAssertion("a2", "prop-p3", "bob", assertion.StanceDenies, "no", testimonialGrade(0.8), t0),
			},
			wantKind:  conflict.KindClaimVsClaim,
			wantCount: 1,
		},
		{
			name: "claim_vs_evidence",
			assertions: []assertion.Assertion{
				mkAssertion("a1", "entity-e1", "alice", assertion.StanceAffirms, "Alice signed", testimonialGrade(0.7), t0),
			},
			spans: []evidencespan.Span{
				mkSpan("s1", "entity-e1", "signatory", "Bob signed", directGrade(0.99)),
			},
			wantKind:  conflict.KindClaimVsEvidence,
			wantCount: 1,
		},
		{
			name: "qualification",
			assertions: []assertion.Assertion{
				mkAssertion("a1", "prop-p5", "alice", assertion.StanceAffirms, "payment made", testimonialGrade(0.8), t0),
				mkAssertion("a2", "prop-p5", "carol", assertion.StanceQualifies, "payment made, pending clearance", testimonialGrade(0.6), t0),
			},
			wantKind:  conflict.KindQualification,
			wantCount: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := conflict.Detect(tc.assertions, tc.spans)
			if len(got) != tc.wantCount {
				t.Fatalf("want %d conflict(s), got %d: %+v", tc.wantCount, len(got), got)
			}
			for _, c := range got {
				if c.Kind != tc.wantKind {
					t.Errorf("want kind %q, got %q", tc.wantKind, c.Kind)
				}
			}
		})
	}
}

// ----------------------------------------------------------------------------
// Test 2: severity ordering — claim vs DIRECT span > claim_vs_claim between
// two testimonial assertions.
// ----------------------------------------------------------------------------

func TestDetect_SeverityOrdering(t *testing.T) {
	// claim_vs_evidence: testimonial assertion vs direct span.
	directSpanConflicts := conflict.Detect(
		[]assertion.Assertion{
			mkAssertion("a1", "entity-x", "alice", assertion.StanceAffirms, "contract signed by Alice", testimonialGrade(0.8), t0),
		},
		[]evidencespan.Span{
			mkSpan("s1", "entity-x", "signatory", "contract signed by Bob", directGrade(0.99)),
		},
	)
	if len(directSpanConflicts) != 1 {
		t.Fatalf("want 1 claim_vs_evidence conflict, got %d", len(directSpanConflicts))
	}
	highSev := directSpanConflicts[0].Severity

	// claim_vs_claim: two testimonial assertions.
	twoTestimonialConflicts := conflict.Detect(
		[]assertion.Assertion{
			mkAssertion("a1", "prop-y", "alice", assertion.StanceAffirms, "yes", testimonialGrade(0.8), t0),
			mkAssertion("a2", "prop-y", "bob", assertion.StanceDenies, "no", testimonialGrade(0.8), t0),
		},
		nil,
	)
	if len(twoTestimonialConflicts) != 1 {
		t.Fatalf("want 1 claim_vs_claim conflict, got %d", len(twoTestimonialConflicts))
	}
	lowSev := twoTestimonialConflicts[0].Severity

	if highSev <= lowSev {
		t.Errorf("claim_vs_evidence (direct) severity %g should be > claim_vs_claim (testimonial) severity %g", highSev, lowSev)
	}
}

// ----------------------------------------------------------------------------
// Test 3: empty input → nil.
// ----------------------------------------------------------------------------

func TestDetect_EmptyInput(t *testing.T) {
	if got := conflict.Detect(nil, nil); got != nil {
		t.Errorf("want nil, got %+v", got)
	}
	if got := conflict.Detect([]assertion.Assertion{}, []evidencespan.Span{}); got != nil {
		t.Errorf("want nil for empty slices, got %+v", got)
	}
}

// ----------------------------------------------------------------------------
// Test 4: identical Content / same stance → no conflict.
// ----------------------------------------------------------------------------

func TestDetect_NoConflictOnIdenticalContent(t *testing.T) {
	// Two different declarants, same proposition, identical content and stance.
	got := conflict.Detect(
		[]assertion.Assertion{
			mkAssertion("a1", "prop-q", "alice", assertion.StanceAffirms, "payment received", testimonialGrade(0.8), t0),
			mkAssertion("a2", "prop-q", "bob", assertion.StanceAffirms, "payment received", testimonialGrade(0.8), t0),
		},
		nil,
	)
	// qualification check: alice affirms, bob affirms — no qualifier present.
	for _, c := range got {
		if c.Kind == conflict.KindClaimVsClaim || c.Kind == conflict.KindSelfInconsistency {
			t.Errorf("unexpected conflict %q for identical content", c.Kind)
		}
	}
}

// Same declarant, same content, same stance → no self_inconsistency.
func TestDetect_SameDeclSameContent(t *testing.T) {
	got := conflict.Detect(
		[]assertion.Assertion{
			mkAssertion("a1", "prop-r", "alice", assertion.StanceAffirms, "same", testimonialGrade(0.8), t0),
			mkAssertion("a2", "prop-r", "alice", assertion.StanceAffirms, "same", testimonialGrade(0.8), t1),
		},
		nil,
	)
	for _, c := range got {
		if c.Kind == conflict.KindSelfInconsistency {
			t.Errorf("unexpected self_inconsistency for identical content")
		}
	}
}

// Assertion content matches span value exactly → no claim_vs_evidence.
func TestDetect_NoConflictAssertionMatchesSpan(t *testing.T) {
	got := conflict.Detect(
		[]assertion.Assertion{
			mkAssertion("a1", "entity-z", "alice", assertion.StanceAffirms, "Alice", testimonialGrade(0.8), t0),
		},
		[]evidencespan.Span{
			mkSpan("s1", "entity-z", "name", "Alice", directGrade(0.99)),
		},
	)
	for _, c := range got {
		if c.Kind == conflict.KindClaimVsEvidence {
			t.Errorf("unexpected claim_vs_evidence when content matches span value")
		}
	}
}

// ----------------------------------------------------------------------------
// Test 5: deterministic ordering — shuffled input → identical output order.
// ----------------------------------------------------------------------------

func TestDetect_DeterministicOrdering(t *testing.T) {
	makeInput := func() ([]assertion.Assertion, []evidencespan.Span) {
		as := []assertion.Assertion{
			mkAssertion("a3", "prop-d", "charlie", assertion.StanceAffirms, "c-val", testimonialGrade(0.8), t0),
			mkAssertion("a1", "prop-d", "alice", assertion.StanceAffirms, "a-val", testimonialGrade(0.8), t0),
			mkAssertion("a2", "prop-d", "bob", assertion.StanceAffirms, "b-val", testimonialGrade(0.8), t0),
		}
		sp := []evidencespan.Span{
			mkSpan("s2", "prop-d", "field", "s2-val", documentaryGrade(0.9)),
			mkSpan("s1", "prop-d", "field", "s1-val", documentaryGrade(0.9)),
		}
		return as, sp
	}

	as1, sp1 := makeInput()
	as2, sp2 := makeInput()

	// Shuffle second set.
	as2[0], as2[2] = as2[2], as2[0]
	sp2[0], sp2[1] = sp2[1], sp2[0]

	got1 := conflict.Detect(as1, sp1)
	got2 := conflict.Detect(as2, sp2)

	if !reflect.DeepEqual(got1, got2) {
		t.Errorf("Detect is not deterministic:\nrun1: %+v\nrun2: %+v", got1, got2)
	}
}

// ----------------------------------------------------------------------------
// Test 6: each unordered pair reported exactly once (no A,B + B,A duplicate).
// ----------------------------------------------------------------------------

func TestDetect_NoDuplicatePairs(t *testing.T) {
	got := conflict.Detect(
		[]assertion.Assertion{
			mkAssertion("a1", "prop-e", "alice", assertion.StanceAffirms, "val1", testimonialGrade(0.8), t0),
			mkAssertion("a2", "prop-e", "bob", assertion.StanceAffirms, "val2", testimonialGrade(0.8), t0),
		},
		nil,
	)

	type pairKey struct {
		kind    conflict.ConflictKind
		subject string
		aID     string
		bID     string
	}
	seen := make(map[pairKey]struct{})
	for _, c := range got {
		k := pairKey{c.Kind, c.Subject, c.A.ID, c.B.ID}
		// Also check the reverse.
		rev := pairKey{c.Kind, c.Subject, c.B.ID, c.A.ID}
		if _, dup := seen[k]; dup {
			t.Errorf("duplicate conflict pair: %+v", c)
		}
		if _, dup := seen[rev]; dup {
			t.Errorf("reverse duplicate conflict pair: %+v", c)
		}
		seen[k] = struct{}{}
	}
}

// ----------------------------------------------------------------------------
// Test 7: ConflictKind.Validate covers valid and invalid values.
// ----------------------------------------------------------------------------

func TestConflictKind_Validate(t *testing.T) {
	valid := []conflict.ConflictKind{
		conflict.KindClaimVsClaim,
		conflict.KindClaimVsEvidence,
		conflict.KindSelfInconsistency,
		conflict.KindRecantation,
		conflict.KindQualification,
		conflict.KindPartial,
	}
	for _, k := range valid {
		if err := k.Validate(); err != nil {
			t.Errorf("unexpected error for valid kind %q: %v", k, err)
		}
	}
	if err := conflict.ConflictKind("hearsay").Validate(); err == nil {
		t.Error("expected error for invalid kind 'hearsay'")
	}
}

// ----------------------------------------------------------------------------
// Tests for DetectWithPropositions.
// ----------------------------------------------------------------------------

// mkProposition builds an assertion.Proposition for test use.
func mkProposition(id, canonical, subject string) assertion.Proposition {
	return assertion.Proposition{
		ID:        id,
		Canonical: canonical,
		Subject:   subject,
	}
}

// TestDetectWithPropositions_DisjointIDs is the regression fixture for the
// "cracked-keystone" bug: when PropositionIDs and Span.EntityIDs live in
// different namespaces the legacy Detect join finds nothing silently.
//
// This test mirrors the shape of a real consumer that:
//   - Generates PropositionIDs as "prop-{sha256hex[:16]}" digests (like
//     throughline's investigate.PropositionIDFromKey).
//   - Stores evidence spans keyed by a record ID (e.g. "rec-{uuid}").
//
// The IDs are invented (org-agnostic); the SHAPE is real.
//
// Assertions:
//  1. With disjoint IDs, Coverage.Degenerate() == true even with non-empty inputs.
//  2. With matched IDs (proposition.Subject == span.EntityID), conflicts are found
//     and Coverage.Degenerate() == false.
func TestDetectWithPropositions_DisjointIDs(t *testing.T) {
	// Proposition IDs in the "prop-hex" namespace (no overlap with record IDs).
	const propID = "prop-7f3a8c1d2e4b5f60"

	// Span EntityIDs in the "rec-" namespace.
	const recID = "rec-00000000-dead-beef-cafe-000000000001"

	assertion1 := mkAssertion(
		"asrt-001", propID, "entity:claimant",
		assertion.StanceAffirms, "the unit was habitable",
		testimonialGrade(0.8), t0,
	)
	span1 := mkSpan("span-001", recID, "habitability", "the unit had mold on the walls", directGrade(0.95))

	prop := mkProposition(propID, "The unit was habitable", recID)

	t.Run("disjoint IDs produce degenerate coverage", func(t *testing.T) {
		// Omit propositions → old-style join (PropositionID vs EntityID).
		// propID == "prop-7f3a8c..." does NOT equal recID == "rec-...".
		// DetectWithPropositions with no propositions: degenerate (no propositions).
		result := conflict.DetectWithPropositions(
			[]assertion.Assertion{assertion1},
			nil, // no propositions → subject bridge absent
			[]evidencespan.Span{span1},
		)
		if len(result.Conflicts) != 0 {
			t.Errorf("want 0 conflicts when propositions are absent, got %d", len(result.Conflicts))
		}
		// Degenerate() should NOT fire when propositions are empty (not a namespace
		// collision — just missing input).
		if result.Coverage.Degenerate() {
			t.Error("Degenerate() should be false when PropositionCount == 0")
		}
	})

	t.Run("propositions with mismatched subject yield degenerate coverage", func(t *testing.T) {
		// Proposition.Subject deliberately set to a DIFFERENT value than span.EntityID
		// to simulate the namespace-collision scenario.
		badProp := mkProposition(propID, "The unit was habitable", "entity-subject-not-a-record-id")
		result := conflict.DetectWithPropositions(
			[]assertion.Assertion{assertion1},
			[]assertion.Proposition{badProp},
			[]evidencespan.Span{span1},
		)
		if len(result.Conflicts) != 0 {
			t.Errorf("want 0 conflicts with disjoint IDs, got %d", len(result.Conflicts))
		}
		if !result.Coverage.Degenerate() {
			t.Errorf("want Coverage.Degenerate()==true for disjoint IDs, got coverage: %+v", result.Coverage)
		}
		if result.Coverage.PropositionSubjectsMatched != 0 {
			t.Errorf("want 0 proposition subjects matched, got %d", result.Coverage.PropositionSubjectsMatched)
		}
		if result.Coverage.SpanEntityIDsMatched != 0 {
			t.Errorf("want 0 span entity IDs matched, got %d", result.Coverage.SpanEntityIDsMatched)
		}
		if result.Coverage.PropositionSubjectsUnmatched != 1 {
			t.Errorf("want 1 proposition subject unmatched, got %d", result.Coverage.PropositionSubjectsUnmatched)
		}
		if result.Coverage.SpanEntityIDsUnmatched != 1 {
			t.Errorf("want 1 span entity ID unmatched, got %d", result.Coverage.SpanEntityIDsUnmatched)
		}
	})

	t.Run("correct proposition bridge finds the conflict", func(t *testing.T) {
		// Proposition.Subject == span.EntityID → join succeeds.
		result := conflict.DetectWithPropositions(
			[]assertion.Assertion{assertion1},
			[]assertion.Proposition{prop},
			[]evidencespan.Span{span1},
		)
		if len(result.Conflicts) != 1 {
			t.Fatalf("want 1 claim_vs_evidence conflict, got %d: %+v", len(result.Conflicts), result.Conflicts)
		}
		if result.Conflicts[0].Kind != conflict.KindClaimVsEvidence {
			t.Errorf("want KindClaimVsEvidence, got %q", result.Conflicts[0].Kind)
		}
		if result.Coverage.Degenerate() {
			t.Errorf("want Degenerate()==false when join succeeds, got coverage: %+v", result.Coverage)
		}
		if result.Coverage.PropositionSubjectsMatched != 1 {
			t.Errorf("want 1 proposition subject matched, got %d", result.Coverage.PropositionSubjectsMatched)
		}
		if result.Coverage.SpanEntityIDsMatched != 1 {
			t.Errorf("want 1 span entity ID matched, got %d", result.Coverage.SpanEntityIDsMatched)
		}
	})
}

// TestDetectWithPropositions_CoverageCounters verifies the join-coverage
// statistics across a multi-proposition, multi-span scenario.
func TestDetectWithPropositions_CoverageCounters(t *testing.T) {
	// Two propositions, each about a different entity.
	// entity-alpha has spans; entity-beta has none.
	props := []assertion.Proposition{
		mkProposition("prop-aaa", "Alpha fact", "entity-alpha"),
		mkProposition("prop-bbb", "Beta fact", "entity-beta"),
	}
	assertions2 := []assertion.Assertion{
		mkAssertion("a1", "prop-aaa", "decl-x", assertion.StanceAffirms, "alpha claims A", testimonialGrade(0.7), t0),
		mkAssertion("a2", "prop-bbb", "decl-y", assertion.StanceAffirms, "beta claims B", testimonialGrade(0.7), t0),
	}
	// Only entity-alpha has a span; entity-gamma is extra (no proposition).
	spans2 := []evidencespan.Span{
		mkSpan("s1", "entity-alpha", "status", "alpha actual A", directGrade(0.9)),
		mkSpan("s2", "entity-gamma", "status", "gamma value", directGrade(0.9)), // unmatched entity
	}

	result := conflict.DetectWithPropositions(assertions2, props, spans2)

	if result.Coverage.AssertionCount != 2 {
		t.Errorf("want AssertionCount==2, got %d", result.Coverage.AssertionCount)
	}
	if result.Coverage.PropositionCount != 2 {
		t.Errorf("want PropositionCount==2, got %d", result.Coverage.PropositionCount)
	}
	if result.Coverage.SpanCount != 2 {
		t.Errorf("want SpanCount==2, got %d", result.Coverage.SpanCount)
	}
	// entity-alpha matched; entity-beta did not.
	if result.Coverage.PropositionSubjectsMatched != 1 {
		t.Errorf("want PropositionSubjectsMatched==1, got %d", result.Coverage.PropositionSubjectsMatched)
	}
	if result.Coverage.PropositionSubjectsUnmatched != 1 {
		t.Errorf("want PropositionSubjectsUnmatched==1, got %d", result.Coverage.PropositionSubjectsUnmatched)
	}
	// entity-alpha matched; entity-gamma did not.
	if result.Coverage.SpanEntityIDsMatched != 1 {
		t.Errorf("want SpanEntityIDsMatched==1, got %d", result.Coverage.SpanEntityIDsMatched)
	}
	if result.Coverage.SpanEntityIDsUnmatched != 1 {
		t.Errorf("want SpanEntityIDsUnmatched==1, got %d", result.Coverage.SpanEntityIDsUnmatched)
	}
	if result.Coverage.Degenerate() {
		t.Error("want Degenerate()==false (entity-alpha matched), got true")
	}
	// claim "alpha claims A" vs span "alpha actual A" → conflict expected.
	if len(result.Conflicts) != 1 {
		t.Errorf("want 1 conflict (entity-alpha join), got %d: %+v", len(result.Conflicts), result.Conflicts)
	}
}

// TestDetectWithPropositions_AssertionVsAssertionPassesStillRun verifies that
// assertion-vs-assertion conflict detection (self_inconsistency, recantation,
// claim_vs_claim, qualification) still runs even when propositions are provided.
func TestDetectWithPropositions_AssertionVsAssertionPassesStillRun(t *testing.T) {
	props := []assertion.Proposition{
		mkProposition("prop-p1", "The payment was made", "entity-payment"),
	}
	assertions3 := []assertion.Assertion{
		mkAssertion("a1", "prop-p1", "alice", assertion.StanceAffirms, "yes", testimonialGrade(0.8), t0),
		mkAssertion("a2", "prop-p1", "alice", assertion.StanceAffirms, "no", testimonialGrade(0.8), t1),
	}
	result := conflict.DetectWithPropositions(assertions3, props, nil)
	if len(result.Conflicts) != 1 {
		t.Fatalf("want 1 self_inconsistency conflict, got %d", len(result.Conflicts))
	}
	if result.Conflicts[0].Kind != conflict.KindSelfInconsistency {
		t.Errorf("want KindSelfInconsistency, got %q", result.Conflicts[0].Kind)
	}
}

// TestDetectWithPropositions_AgreementNoConflict verifies that when the
// assertion content matches the span value exactly no conflict is emitted.
func TestDetectWithPropositions_AgreementNoConflict(t *testing.T) {
	props := []assertion.Proposition{
		mkProposition("prop-agree", "The signatory is Alice", "entity-contract"),
	}
	assertions4 := []assertion.Assertion{
		mkAssertion("a1", "prop-agree", "alice", assertion.StanceAffirms, "Alice", testimonialGrade(0.8), t0),
	}
	spans4 := []evidencespan.Span{
		mkSpan("s1", "entity-contract", "signatory", "Alice", directGrade(0.99)),
	}
	result := conflict.DetectWithPropositions(assertions4, props, spans4)
	for _, c := range result.Conflicts {
		if c.Kind == conflict.KindClaimVsEvidence {
			t.Errorf("unexpected claim_vs_evidence when content matches span value")
		}
	}
	if result.Coverage.Degenerate() {
		t.Error("want Degenerate()==false when IDs match, got true")
	}
}

// TestDetectWithPropositions_DegenerateMethod unit-tests Coverage.Degenerate()
// directly across boundary cases.
func TestDetectWithPropositions_DegenerateMethod(t *testing.T) {
	cases := []struct {
		name      string
		cov       conflict.Coverage
		wantDegen bool
	}{
		{
			name:      "all zero — not degenerate",
			cov:       conflict.Coverage{},
			wantDegen: false,
		},
		{
			name: "propositions present, no spans — not degenerate",
			cov: conflict.Coverage{
				PropositionCount:             2,
				SpanCount:                    0,
				PropositionSubjectsMatched:   0,
				PropositionSubjectsUnmatched: 2,
				SpanEntityIDsMatched:         0,
				SpanEntityIDsUnmatched:       0,
			},
			wantDegen: false,
		},
		{
			name: "spans present, no propositions — not degenerate",
			cov: conflict.Coverage{
				PropositionCount:             0,
				SpanCount:                    3,
				PropositionSubjectsMatched:   0,
				PropositionSubjectsUnmatched: 0,
				SpanEntityIDsMatched:         0,
				SpanEntityIDsUnmatched:       3,
			},
			wantDegen: false,
		},
		{
			name: "both present, zero matches — degenerate",
			cov: conflict.Coverage{
				PropositionCount:             2,
				SpanCount:                    3,
				PropositionSubjectsMatched:   0,
				PropositionSubjectsUnmatched: 2,
				SpanEntityIDsMatched:         0,
				SpanEntityIDsUnmatched:       3,
			},
			wantDegen: true,
		},
		{
			name: "partial match — not degenerate",
			cov: conflict.Coverage{
				PropositionCount:             2,
				SpanCount:                    3,
				PropositionSubjectsMatched:   1,
				PropositionSubjectsUnmatched: 1,
				SpanEntityIDsMatched:         1,
				SpanEntityIDsUnmatched:       2,
			},
			wantDegen: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.cov.Degenerate()
			if got != tc.wantDegen {
				t.Errorf("Degenerate(): want %v, got %v (coverage: %+v)", tc.wantDegen, got, tc.cov)
			}
		})
	}
}

// TestDetectWithPropositions_Determinism verifies that shuffled inputs produce
// identical output.
func TestDetectWithPropositions_Determinism(t *testing.T) {
	props := []assertion.Proposition{
		mkProposition("prop-d1", "Fact about alpha", "entity-alpha"),
		mkProposition("prop-d2", "Fact about beta", "entity-beta"),
	}
	assertions5 := []assertion.Assertion{
		mkAssertion("a3", "prop-d1", "charlie", assertion.StanceAffirms, "c-val", testimonialGrade(0.8), t0),
		mkAssertion("a1", "prop-d1", "alice", assertion.StanceAffirms, "a-val", testimonialGrade(0.8), t0),
		mkAssertion("a2", "prop-d2", "bob", assertion.StanceAffirms, "b-val", testimonialGrade(0.8), t0),
	}
	spans5 := []evidencespan.Span{
		mkSpan("s2", "entity-alpha", "field", "s2-val", documentaryGrade(0.9)),
		mkSpan("s1", "entity-alpha", "field", "s1-val", documentaryGrade(0.9)),
	}

	props2 := []assertion.Proposition{props[1], props[0]} // reversed
	assertions5b := []assertion.Assertion{assertions5[2], assertions5[0], assertions5[1]}
	spans5b := []evidencespan.Span{spans5[1], spans5[0]}

	r1 := conflict.DetectWithPropositions(assertions5, props, spans5)
	r2 := conflict.DetectWithPropositions(assertions5b, props2, spans5b)

	if !reflect.DeepEqual(r1.Conflicts, r2.Conflicts) {
		t.Errorf("DetectWithPropositions is not deterministic:\nrun1: %+v\nrun2: %+v", r1.Conflicts, r2.Conflicts)
	}
	if r1.Coverage != r2.Coverage {
		t.Errorf("Coverage is not deterministic:\nrun1: %+v\nrun2: %+v", r1.Coverage, r2.Coverage)
	}
}

// ----------------------------------------------------------------------------
// Test 8: severity formula spot-checks.
// ----------------------------------------------------------------------------

func TestDetect_SeverityFormula(t *testing.T) {
	// direct (rank 5) vs testimonial (rank 2): (5/5)*0.7 + (2/5)*0.3 = 0.70 + 0.12 = 0.82
	const wantHighSev = 0.82
	highConflicts := conflict.Detect(
		[]assertion.Assertion{
			mkAssertion("a1", "entity-sev", "alice", assertion.StanceAffirms, "val-A", testimonialGrade(0.8), t0),
		},
		[]evidencespan.Span{
			mkSpan("s1", "entity-sev", "field", "val-B", directGrade(1.0)),
		},
	)
	if len(highConflicts) != 1 {
		t.Fatalf("want 1 conflict, got %d", len(highConflicts))
	}
	const tol = 1e-9
	if got := highConflicts[0].Severity; got < wantHighSev-tol || got > wantHighSev+tol {
		t.Errorf("severity: want %.4f, got %.4f", wantHighSev, got)
	}

	// Two testimonial (rank 2) assertions: (2/5)*0.7 + (2/5)*0.3 = 0.28 + 0.12 = 0.40
	const wantLowSev = 0.40
	lowConflicts := conflict.Detect(
		[]assertion.Assertion{
			mkAssertion("a1", "prop-sev2", "alice", assertion.StanceAffirms, "yes", testimonialGrade(0.8), t0),
			mkAssertion("a2", "prop-sev2", "bob", assertion.StanceDenies, "no", testimonialGrade(0.8), t0),
		},
		nil,
	)
	if len(lowConflicts) < 1 {
		t.Fatalf("want at least 1 conflict, got %d", len(lowConflicts))
	}
	// Find the claim_vs_claim entry.
	for _, c := range lowConflicts {
		if c.Kind == conflict.KindClaimVsClaim {
			if got := c.Severity; got < wantLowSev-tol || got > wantLowSev+tol {
				t.Errorf("claim_vs_claim severity: want %.4f, got %.4f", wantLowSev, got)
			}
			break
		}
	}
}
