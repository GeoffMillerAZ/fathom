package assertion_test

import (
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/evidence/assertion"
	evidencegrade "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-grade"
	es "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-span"
	aiprovenance "github.com/geoffmilleraz/agentic-kit/foundations/provenance/ai-provenance"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func validLocator() es.Locator {
	return es.Locator{
		Kind:  "line",
		Value: "depo-alice.txt:142",
		Start: 142,
		End:   145,
	}
}

func validGrade() *evidencegrade.Grade {
	g := evidencegrade.Grade{Tier: evidencegrade.TierTestimonial, Confidence: 0.80}
	return &g
}

// fullyProvenancedAssertion returns an Assertion that passes ValidateAsEvidence.
// Declarant is "entity:alice" (the witness); Author is an AI extractor — they
// are explicitly distinct, validating the core design invariant.
func fullyProvenancedAssertion() assertion.Assertion {
	return assertion.Assertion{
		ID:            "assert-001",
		PropositionID: "prop-001",
		Declarant:     "entity:alice",
		Stance:        assertion.StanceAffirms,
		Content:       "I was at the bar at 9pm",
		AsOf:          time.Date(2024, 1, 15, 21, 0, 0, 0, time.UTC),
		StatedAt:      time.Date(2024, 3, 10, 9, 30, 0, 0, time.UTC),
		StatedIn:      "rec-depo-alice-20240310",
		Locator:       validLocator(),
		Via:           evidencegrade.TierTestimonial,
		Author: aiprovenance.NewAI(
			"claude-sonnet-4-6",
			"20250514",
			"sha256:promptabc123",
			"sha256:paramsxyz789",
			"sha256:inputdigest000",
			time.Date(2024, 3, 10, 9, 31, 0, 0, time.UTC),
		),
		Grade: validGrade(),
	}
}

// minimalValidAssertion returns a minimal Assertion that passes Validate (not ValidateAsEvidence).
func minimalValidAssertion() assertion.Assertion {
	return assertion.Assertion{
		ID:            "assert-min",
		PropositionID: "prop-001",
		Declarant:     "entity:bob",
		Stance:        assertion.StanceDenies,
		Content:       "I never saw her there",
	}
}

// ---------------------------------------------------------------------------
// 1. Stance tests
// ---------------------------------------------------------------------------

func TestStanceValidateKnown(t *testing.T) {
	known := []assertion.Stance{
		assertion.StanceAffirms,
		assertion.StanceDenies,
		assertion.StanceQualifies,
		assertion.StanceWithdraws,
		assertion.StanceRecants,
		assertion.StanceDoesNotRecall,
		assertion.StanceDeclines,
	}
	for _, s := range known {
		t.Run(string(s), func(t *testing.T) {
			if err := s.Validate(); err != nil {
				t.Errorf("known stance %q should be valid, got: %v", s, err)
			}
		})
	}
}

func TestStanceValidateUnknown(t *testing.T) {
	unknown := []assertion.Stance{"", "speculates", "hedges", "affirm"}
	for _, s := range unknown {
		t.Run(string(s), func(t *testing.T) {
			if err := s.Validate(); err == nil {
				t.Errorf("unknown stance %q should be rejected, got nil", s)
			}
		})
	}
}

func TestStanceIsRetraction(t *testing.T) {
	cases := []struct {
		stance  assertion.Stance
		wantRet bool
	}{
		{assertion.StanceWithdraws, true},
		{assertion.StanceRecants, true},
		{assertion.StanceAffirms, false},
		{assertion.StanceDenies, false},
		{assertion.StanceQualifies, false},
		{assertion.StanceDoesNotRecall, false},
		{assertion.StanceDeclines, false},
	}
	for _, tc := range cases {
		t.Run(string(tc.stance), func(t *testing.T) {
			got := tc.stance.IsRetraction()
			if got != tc.wantRet {
				t.Errorf("Stance(%q).IsRetraction() = %v, want %v", tc.stance, got, tc.wantRet)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 2. Proposition.Validate tests
// ---------------------------------------------------------------------------

func TestPropositionValidateHappyPath(t *testing.T) {
	p := assertion.Proposition{
		ID:        "prop-001",
		Canonical: "Alice was at The Tap Room on 2024-01-15 at 21:00 local time",
		Subject:   "entity:alice",
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestPropositionValidateEmptyFields(t *testing.T) {
	base := assertion.Proposition{
		ID:        "prop-001",
		Canonical: "Alice was at the bar",
		Subject:   "entity:alice",
	}
	cases := []struct {
		name  string
		mutFn func(*assertion.Proposition)
	}{
		{"empty-ID", func(p *assertion.Proposition) { p.ID = "" }},
		{"empty-Canonical", func(p *assertion.Proposition) { p.Canonical = "" }},
		{"empty-Subject", func(p *assertion.Proposition) { p.Subject = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := base
			tc.mutFn(&p)
			if err := p.Validate(); err == nil {
				t.Errorf("%s: expected validation error, got nil", tc.name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 3. Assertion.Validate tests (lenient)
// ---------------------------------------------------------------------------

func TestAssertionValidateHappyPath(t *testing.T) {
	if err := minimalValidAssertion().Validate(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestAssertionValidateEmptyRequiredFields(t *testing.T) {
	cases := []struct {
		name  string
		mutFn func(*assertion.Assertion)
	}{
		{"empty-ID", func(a *assertion.Assertion) { a.ID = "" }},
		{"empty-PropositionID", func(a *assertion.Assertion) { a.PropositionID = "" }},
		{"empty-Declarant", func(a *assertion.Assertion) { a.Declarant = "" }},
		{"empty-Content", func(a *assertion.Assertion) { a.Content = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := minimalValidAssertion()
			tc.mutFn(&a)
			if err := a.Validate(); err == nil {
				t.Errorf("%s: expected validation error, got nil", tc.name)
			}
		})
	}
}

func TestAssertionValidateUnknownStance(t *testing.T) {
	a := minimalValidAssertion()
	a.Stance = assertion.Stance("speculates")
	if err := a.Validate(); err == nil {
		t.Fatal("unknown Stance should be rejected by Validate, got nil")
	}
}

func TestAssertionValidateInvalidVia(t *testing.T) {
	a := minimalValidAssertion()
	a.Via = evidencegrade.Tier("hearsay") // not a valid tier
	if err := a.Validate(); err == nil {
		t.Fatal("invalid Via tier should be rejected by Validate, got nil")
	}
}

func TestAssertionValidateValidViaOptional(t *testing.T) {
	// Via is optional in lenient Validate — empty string is OK.
	a := minimalValidAssertion()
	a.Via = ""
	if err := a.Validate(); err != nil {
		t.Fatalf("empty Via should be allowed by Validate: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 4. Assertion.ValidateAsEvidence tests (strict)
// ---------------------------------------------------------------------------

func TestValidateAsEvidenceHappyPath(t *testing.T) {
	if err := fullyProvenancedAssertion().ValidateAsEvidence(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateAsEvidenceRejectsMissingFields(t *testing.T) {
	cases := []struct {
		name  string
		mutFn func(*assertion.Assertion)
	}{
		{"missing-StatedIn", func(a *assertion.Assertion) { a.StatedIn = "" }},
		{"missing-AsOf", func(a *assertion.Assertion) { a.AsOf = time.Time{} }},
		{"missing-StatedAt", func(a *assertion.Assertion) { a.StatedAt = time.Time{} }},
		{"missing-Author", func(a *assertion.Assertion) { a.Author = aiprovenance.Author{} }},
		{"missing-Locator", func(a *assertion.Assertion) { a.Locator = es.Locator{} }},
		{"missing-Grade", func(a *assertion.Assertion) { a.Grade = nil }},
		{"missing-Via", func(a *assertion.Assertion) { a.Via = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := fullyProvenancedAssertion()
			tc.mutFn(&a)
			if err := a.ValidateAsEvidence(); err == nil {
				t.Errorf("%s: expected ValidateAsEvidence error, got nil", tc.name)
			}
		})
	}
}

// TestDeclarantDistinctFromAuthor is the CRITICAL design-point test.
// Declarant (the real-world witness "Alice") must be distinct from Author
// (the system recorder — here an AI extractor). ValidateAsEvidence must pass.
func TestDeclarantDistinctFromAuthor(t *testing.T) {
	a := fullyProvenancedAssertion()

	// Declarant is the witness — a real-world entity ID.
	a.Declarant = "entity:alice"

	// Author is the AI extractor who logged the statement — NOT Alice.
	// Author.AI.ModelID is "claude-sonnet-4-6", not "alice".
	// Confirm they are distinct.
	if a.Declarant == a.Author.AI.ModelID {
		t.Fatal("test setup error: Declarant should differ from Author AI model")
	}

	if err := a.ValidateAsEvidence(); err != nil {
		t.Fatalf("Declarant≠Author should still pass ValidateAsEvidence: %v", err)
	}
}

// TestDeclarantDistinctFromHumanAuthor verifies the same invariant when Author
// is a human recorder (e.g. a paralegal), not an AI.
func TestDeclarantDistinctFromHumanAuthor(t *testing.T) {
	a := fullyProvenancedAssertion()
	a.Declarant = "entity:alice"
	a.Author = aiprovenance.NewHuman("paralegal@lawfirm.example.com")

	// Declarant is not the Author's human ID.
	if a.Declarant == a.Author.Human.ID {
		t.Fatal("test setup error: Declarant should differ from Author.Human.ID")
	}

	if err := a.ValidateAsEvidence(); err != nil {
		t.Fatalf("Declarant≠Author.Human.ID should pass ValidateAsEvidence: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 5. Locator.Kind=="none" rejected by ValidateAsEvidence
// ---------------------------------------------------------------------------

func TestValidateAsEvidenceRejectsNoneLocator(t *testing.T) {
	a := fullyProvenancedAssertion()
	a.Locator = es.Locator{Kind: "none"}

	// Strict gate must reject "none" locator.
	if err := a.ValidateAsEvidence(); err == nil {
		t.Fatal("expected error for Locator.Kind='none' at evidence gate, got nil")
	}
	// Lenient Validate() must still accept it (back-compat: "none" is recognised).
	if err := a.Validate(); err != nil {
		t.Fatalf("Validate() should accept Locator.Kind='none': %v", err)
	}
}

// ---------------------------------------------------------------------------
// 6. Knowledge enum — Validate and IsHearsay
// ---------------------------------------------------------------------------

func TestKnowledgeValidateKnown(t *testing.T) {
	known := []assertion.Knowledge{
		assertion.KnowledgeFirstHand,
		assertion.KnowledgeReported,
		assertion.KnowledgeBelief,
	}
	for _, k := range known {
		t.Run(string(k), func(t *testing.T) {
			if err := k.Validate(); err != nil {
				t.Errorf("known Knowledge %q should be valid, got: %v", k, err)
			}
		})
	}
}

func TestKnowledgeValidateUnknown(t *testing.T) {
	unknown := []assertion.Knowledge{"", "hearsay", "direct", "first_hand_ish"}
	for _, k := range unknown {
		t.Run(string(k), func(t *testing.T) {
			if err := k.Validate(); err == nil {
				t.Errorf("unknown Knowledge %q should be rejected, got nil", k)
			}
		})
	}
}

func TestIsHearsay(t *testing.T) {
	cases := []struct {
		knowledge assertion.Knowledge
		want      bool
	}{
		{assertion.KnowledgeReported, true},
		{assertion.KnowledgeFirstHand, false},
		{assertion.KnowledgeBelief, false},
		{"", false}, // zero value: not hearsay
	}
	for _, tc := range cases {
		t.Run(string(tc.knowledge), func(t *testing.T) {
			a := minimalValidAssertion()
			a.Knowledge = tc.knowledge
			if got := a.IsHearsay(); got != tc.want {
				t.Errorf("IsHearsay() with Knowledge=%q = %v, want %v", tc.knowledge, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 7. Knowledge field on Assertion — Validate rejects invalid Knowledge
// ---------------------------------------------------------------------------

func TestAssertionValidateRejectsInvalidKnowledge(t *testing.T) {
	a := minimalValidAssertion()
	a.Knowledge = assertion.Knowledge("unknown_value")
	if err := a.Validate(); err == nil {
		t.Fatal("invalid Knowledge should be rejected by Validate, got nil")
	}
}

func TestAssertionValidateAcceptsEmptyKnowledge(t *testing.T) {
	// Knowledge is optional: zero value ("") must be accepted by Validate.
	a := minimalValidAssertion()
	a.Knowledge = ""
	if err := a.Validate(); err != nil {
		t.Fatalf("empty (zero) Knowledge should be accepted by Validate: %v", err)
	}
}

func TestAssertionValidateAcceptsValidKnowledge(t *testing.T) {
	for _, k := range []assertion.Knowledge{
		assertion.KnowledgeFirstHand,
		assertion.KnowledgeReported,
		assertion.KnowledgeBelief,
	} {
		t.Run(string(k), func(t *testing.T) {
			a := minimalValidAssertion()
			a.Knowledge = k
			if err := a.Validate(); err != nil {
				t.Errorf("valid Knowledge %q should pass Validate: %v", k, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 8. Role field on Assertion — unconstrained, any string passes
// ---------------------------------------------------------------------------

func TestAssertionValidateAcceptsAnyRole(t *testing.T) {
	roles := []string{"", "witness", "defendant", "expert-witness", "paralegal", "some-future-role"}
	for _, r := range roles {
		t.Run(r, func(t *testing.T) {
			a := minimalValidAssertion()
			a.Role = r
			if err := a.Validate(); err != nil {
				t.Errorf("Role %q should pass Validate: %v", r, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 9. Full assertion with Knowledge + Role passes ValidateAsEvidence
// ---------------------------------------------------------------------------

func TestValidateAsEvidenceWithKnowledgeAndRole(t *testing.T) {
	a := fullyProvenancedAssertion()
	a.Knowledge = assertion.KnowledgeReported
	a.Role = "witness"

	if err := a.ValidateAsEvidence(); err != nil {
		t.Fatalf("fully-provenanced assertion with Knowledge+Role should pass ValidateAsEvidence: %v", err)
	}
	if !a.IsHearsay() {
		t.Fatal("KnowledgeReported assertion should be flagged as hearsay")
	}
}

func TestValidateAsEvidenceRejectsInvalidKnowledgeWhenSet(t *testing.T) {
	a := fullyProvenancedAssertion()
	a.Knowledge = assertion.Knowledge("bad_value")

	if err := a.ValidateAsEvidence(); err == nil {
		t.Fatal("invalid Knowledge should be rejected by ValidateAsEvidence, got nil")
	}
}
