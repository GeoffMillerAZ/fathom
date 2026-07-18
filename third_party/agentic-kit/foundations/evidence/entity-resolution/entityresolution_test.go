package entityresolution_test

import (
	"math"
	"testing"
	"time"

	er "github.com/geoffmilleraz/agentic-kit/foundations/evidence/entity-resolution"
	evidencegrade "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-grade"
	aiprovenance "github.com/geoffmilleraz/agentic-kit/foundations/provenance/ai-provenance"
)

// ── Levenshtein ───────────────────────────────────────────────────────────────

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"kitten", "sitting", 3},
		{"sitting", "kitten", 3},
		{"", "", 0},
		{"abc", "abc", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"a", "b", 1},
		{"saturday", "sunday", 3},
		{"flaw", "lawn", 2},
	}
	for _, tc := range cases {
		t.Run(tc.a+"→"+tc.b, func(t *testing.T) {
			got := er.Levenshtein(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("Levenshtein(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// ── NormalizedLevenshtein ─────────────────────────────────────────────────────

func TestNormalizedLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want float64
	}{
		{"", "", 1.0},
		{"abc", "abc", 1.0},
		{"abc", "xyz", 0.0},
		{"kitten", "sitting", 1.0 - float64(3)/float64(7)},
	}
	for _, tc := range cases {
		t.Run(tc.a+"→"+tc.b, func(t *testing.T) {
			got := er.NormalizedLevenshtein(tc.a, tc.b)
			if math.Abs(got-tc.want) > 1e-9 {
				t.Errorf("NormalizedLevenshtein(%q, %q) = %f, want %f", tc.a, tc.b, got, tc.want)
			}
			// result must be in [0,1]
			if got < 0 || got > 1 {
				t.Errorf("NormalizedLevenshtein(%q, %q) = %f outside [0,1]", tc.a, tc.b, got)
			}
		})
	}
}

// ── JaccardSimilarity ─────────────────────────────────────────────────────────

func TestJaccardSimilarity(t *testing.T) {
	cases := []struct {
		a, b string
		want float64
	}{
		{"", "", 1.0},
		{"apple", "apple", 1.0},
		{"apple", "banana", 0.0},
		// {"john doe", "doe john"} — same tokens, Jaccard = 1
		{"john doe", "doe john", 1.0},
		// {"john doe", "jane doe"} — intersection={doe} union={john,jane,doe} → 1/3
		{"john doe", "jane doe", 1.0 / 3.0},
	}
	for _, tc := range cases {
		t.Run(tc.a+"→"+tc.b, func(t *testing.T) {
			got := er.JaccardSimilarity(tc.a, tc.b)
			if math.Abs(got-tc.want) > 1e-9 {
				t.Errorf("JaccardSimilarity(%q, %q) = %f, want %f", tc.a, tc.b, got, tc.want)
			}
			if got < 0 || got > 1 {
				t.Errorf("JaccardSimilarity(%q, %q) = %f outside [0,1]", tc.a, tc.b, got)
			}
		})
	}
}

// ── Similarity ────────────────────────────────────────────────────────────────

func TestSimilarityRange(t *testing.T) {
	pairs := [][2]string{
		{"", ""},
		{"alice", "alice"},
		{"alice", "bob"},
		{"John Smith", "Jon Smith"},
		{"Jane Doe", "J. Doe"},
	}
	for _, p := range pairs {
		s := er.Similarity(p[0], p[1])
		if s < 0 || s > 1 {
			t.Errorf("Similarity(%q, %q) = %f outside [0,1]", p[0], p[1], s)
		}
	}
}

func TestSimilaritySymmetric(t *testing.T) {
	pairs := [][2]string{
		{"kitten", "sitting"},
		{"John Smith", "Jon Smyth"},
		{"Alice", ""},
	}
	for _, p := range pairs {
		ab := er.Similarity(p[0], p[1])
		ba := er.Similarity(p[1], p[0])
		if math.Abs(ab-ba) > 1e-9 {
			t.Errorf("Similarity not symmetric: Similarity(%q,%q)=%f != Similarity(%q,%q)=%f",
				p[0], p[1], ab, p[1], p[0], ba)
		}
	}
}

// ── Entity.Validate ───────────────────────────────────────────────────────────

func TestEntityValidate(t *testing.T) {
	cases := []struct {
		name    string
		e       er.Entity
		wantErr bool
	}{
		{"valid", er.Entity{ID: "e1", Name: "Alice"}, false},
		{"empty-id", er.Entity{ID: "", Name: "Alice"}, true},
		{"empty-name", er.Entity{ID: "e1", Name: ""}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.e.Validate()
			if tc.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ── Candidate.Validate ────────────────────────────────────────────────────────

// validAuthor returns a well-formed human Author for use in tests.
func validAuthor() aiprovenance.Author {
	return aiprovenance.NewHuman("analyst-1")
}

// validGrade returns a well-formed Grade for use in tests.
func validGrade() evidencegrade.Grade {
	return evidencegrade.Grade{Tier: evidencegrade.TierDocumentary, Confidence: 0.9}
}

func validCandidate() er.Candidate {
	return er.Candidate{
		LeftID:        "e1",
		RightID:       "e2",
		Score:         0.8,
		Method:        "name-similarity",
		Author:        validAuthor(),
		Grade:         validGrade(),
		SourceSpanIDs: []string{"span-1"},
	}
}

func TestCandidateValidate(t *testing.T) {
	cases := []struct {
		name    string
		mutFn   func(*er.Candidate)
		wantErr bool
	}{
		{"valid", func(*er.Candidate) {}, false},
		{"empty-left", func(c *er.Candidate) { c.LeftID = "" }, true},
		{"empty-right", func(c *er.Candidate) { c.RightID = "" }, true},
		{"same-ids", func(c *er.Candidate) { c.RightID = c.LeftID }, true},
		// Finding 3: canonical ordering must be enforced by Validate.
		{"left-gt-right", func(c *er.Candidate) { c.LeftID = "z-entity"; c.RightID = "a-entity" }, true},
		{"score-neg", func(c *er.Candidate) { c.Score = -0.01 }, true},
		{"score-above1", func(c *er.Candidate) { c.Score = 1.01 }, true},
		{"score-zero", func(c *er.Candidate) { c.Score = 0 }, false},
		{"score-one", func(c *er.Candidate) { c.Score = 1 }, false},
		{"empty-method", func(c *er.Candidate) { c.Method = "" }, true},
		// Finding 5: invalid Author must be rejected.
		{"invalid-author", func(c *er.Candidate) { c.Author = aiprovenance.Author{} }, true},
		// Finding 5: invalid Grade must be rejected.
		{"invalid-grade", func(c *er.Candidate) { c.Grade = evidencegrade.Grade{Tier: "bogus", Confidence: 0.5} }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validCandidate()
			tc.mutFn(&c)
			err := c.Validate()
			if tc.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestCandidateValidate_CanonicalOrderEnforced is the regression test for Finding 3:
// Candidate.Validate() must reject LeftID > RightID.
func TestCandidateValidate_CanonicalOrderEnforced(t *testing.T) {
	c := validCandidate()
	c.LeftID = "z-entity"
	c.RightID = "a-entity"
	if err := c.Validate(); err == nil {
		t.Error("expected error for LeftID > RightID, got nil")
	}
}

// ── GenerateCandidates ────────────────────────────────────────────────────────

func TestGenerateCandidatesThreshold(t *testing.T) {
	entities := []er.Entity{
		{ID: "e1", Name: "Alice Smith"},
		{ID: "e2", Name: "Alice Smyth"}, // very similar
		{ID: "e3", Name: "Bob Johnson"}, // unrelated
	}
	author := validAuthor()
	grade := validGrade()

	// High threshold: only the very-similar pair survives.
	highCands := er.GenerateCandidates(entities, 0.8, author, grade)
	for _, c := range highCands {
		if c.Score < 0.8 {
			t.Errorf("candidate %v has score %f below threshold 0.8", c, c.Score)
		}
	}

	// Zero threshold: all 3 pairs should appear.
	allCands := er.GenerateCandidates(entities, 0.0, author, grade)
	if len(allCands) != 3 {
		t.Errorf("expected 3 candidates at threshold 0, got %d", len(allCands))
	}
}

func TestGenerateCandidatesNoSelfPairs(t *testing.T) {
	entities := []er.Entity{
		{ID: "e1", Name: "Alice"},
		{ID: "e2", Name: "Alice"}, // identical names
	}
	cands := er.GenerateCandidates(entities, 0.0, validAuthor(), validGrade())
	for _, c := range cands {
		if c.LeftID == c.RightID {
			t.Errorf("self-pair found: %v", c)
		}
	}
}

func TestGenerateCandidatesCanonicalOrder(t *testing.T) {
	// e2 < e1 lexicographically — LeftID must still be the smaller one.
	entities := []er.Entity{
		{ID: "z-entity", Name: "Alice"},
		{ID: "a-entity", Name: "Alice"},
	}
	cands := er.GenerateCandidates(entities, 0.0, validAuthor(), validGrade())
	if len(cands) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(cands))
	}
	c := cands[0]
	if c.LeftID > c.RightID {
		t.Errorf("canonical ordering violated: LeftID=%q > RightID=%q", c.LeftID, c.RightID)
	}
}

func TestGenerateCandidatesDeterministicOrder(t *testing.T) {
	entities := []er.Entity{
		{ID: "e1", Name: "Alice Smith"},
		{ID: "e2", Name: "Alice Smyth"},
		{ID: "e3", Name: "Alicia Smith"},
		{ID: "e4", Name: "Bob Jones"},
	}
	author := validAuthor()
	grade := validGrade()
	a := er.GenerateCandidates(entities, 0.0, author, grade)
	b := er.GenerateCandidates(entities, 0.0, author, grade)
	if len(a) != len(b) {
		t.Fatalf("lengths differ: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].LeftID != b[i].LeftID || a[i].RightID != b[i].RightID || a[i].Score != b[i].Score {
			t.Errorf("index %d: %v != %v", i, a[i], b[i])
		}
	}
	// Verify descending score order.
	for i := 1; i < len(a); i++ {
		if a[i].Score > a[i-1].Score {
			t.Errorf("score order violated at %d: %f > %f", i, a[i].Score, a[i-1].Score)
		}
	}
}

func TestGenerateCandidatesMethod(t *testing.T) {
	entities := []er.Entity{
		{ID: "e1", Name: "Alice"},
		{ID: "e2", Name: "Alice"},
	}
	cands := er.GenerateCandidates(entities, 0.0, validAuthor(), validGrade())
	for _, c := range cands {
		if c.Method != "name-similarity" {
			t.Errorf("expected Method %q, got %q", "name-similarity", c.Method)
		}
	}
}

// ── Review workflow ───────────────────────────────────────────────────────────

var (
	testClock    = time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	testReviewAt = testClock.Add(time.Hour)
)

func proposedMatch() er.Match {
	return er.Propose(validCandidate(), testClock)
}

func TestProposeState(t *testing.T) {
	m := proposedMatch()
	if m.State != er.StateProposed {
		t.Errorf("Propose: expected state %q, got %q", er.StateProposed, m.State)
	}
	if m.ReviewedBy != "" || m.Note != "" {
		t.Errorf("Propose: expected empty ReviewedBy/Note, got %q/%q", m.ReviewedBy, m.Note)
	}
	// Finding 1: CreatedAt must be set; ReviewedAt must be zero.
	if m.CreatedAt.IsZero() {
		t.Error("Propose: CreatedAt must not be zero")
	}
	if !m.ReviewedAt.IsZero() {
		t.Error("Propose: ReviewedAt must be zero on a proposed match")
	}
}

func TestAcceptFromProposed(t *testing.T) {
	m := proposedMatch()
	got, err := m.Accept("reviewer-1", "looks correct", testReviewAt)
	if err != nil {
		t.Fatalf("Accept: unexpected error: %v", err)
	}
	if got.State != er.StateAccepted {
		t.Errorf("expected state %q, got %q", er.StateAccepted, got.State)
	}
	if got.ReviewedBy != "reviewer-1" {
		t.Errorf("ReviewedBy: got %q, want %q", got.ReviewedBy, "reviewer-1")
	}
	if got.Note != "looks correct" {
		t.Errorf("Note: got %q, want %q", got.Note, "looks correct")
	}
	// Finding 1: ReviewedAt must be set after Accept.
	if got.ReviewedAt.IsZero() {
		t.Error("Accept: ReviewedAt must not be zero after accept")
	}
	if !got.ReviewedAt.Equal(testReviewAt) {
		t.Errorf("Accept: ReviewedAt = %v, want %v", got.ReviewedAt, testReviewAt)
	}
	// Receiver must be unchanged (append-only).
	if m.State != er.StateProposed {
		t.Error("Accept mutated the receiver")
	}
}

func TestRejectFromProposed(t *testing.T) {
	m := proposedMatch()
	got, err := m.Reject("reviewer-2", "different people", testReviewAt)
	if err != nil {
		t.Fatalf("Reject: unexpected error: %v", err)
	}
	if got.State != er.StateRejected {
		t.Errorf("expected state %q, got %q", er.StateRejected, got.State)
	}
	if got.ReviewedBy != "reviewer-2" {
		t.Errorf("ReviewedBy: got %q, want %q", got.ReviewedBy, "reviewer-2")
	}
	// Finding 1: ReviewedAt must be set after Reject.
	if got.ReviewedAt.IsZero() {
		t.Error("Reject: ReviewedAt must not be zero after reject")
	}
}

func TestAcceptFromAcceptedErrors(t *testing.T) {
	m := proposedMatch()
	accepted, _ := m.Accept("r1", "ok", testReviewAt)
	_, err := accepted.Accept("r2", "again", testReviewAt)
	if err == nil {
		t.Error("expected error when accepting an already-accepted match, got nil")
	}
}

func TestRejectFromAcceptedErrors(t *testing.T) {
	m := proposedMatch()
	accepted, _ := m.Accept("r1", "ok", testReviewAt)
	_, err := accepted.Reject("r2", "nope", testReviewAt)
	if err == nil {
		t.Error("expected error when rejecting an accepted match, got nil")
	}
}

func TestAcceptFromRejectedErrors(t *testing.T) {
	m := proposedMatch()
	rejected, _ := m.Reject("r1", "nope", testReviewAt)
	_, err := rejected.Accept("r2", "changed mind", testReviewAt)
	if err == nil {
		t.Error("expected error when accepting a rejected match, got nil")
	}
}

func TestAcceptEmptyByErrors(t *testing.T) {
	m := proposedMatch()
	_, err := m.Accept("", "some note", testReviewAt)
	if err == nil {
		t.Error("expected error for empty reviewer, got nil")
	}
}

func TestRejectEmptyByErrors(t *testing.T) {
	m := proposedMatch()
	_, err := m.Reject("", "some note", testReviewAt)
	if err == nil {
		t.Error("expected error for empty reviewer, got nil")
	}
}

// ── Match.Validate ────────────────────────────────────────────────────────────

func TestMatchValidate(t *testing.T) {
	m := proposedMatch()
	if err := m.Validate(); err != nil {
		t.Errorf("valid match: unexpected error: %v", err)
	}

	bad := m
	bad.State = "unknown-state"
	if err := bad.Validate(); err == nil {
		t.Error("invalid state: expected error, got nil")
	}
}

// TestMatchValidate_ReviewedByRequiredWhenDecided is the regression test for Finding 4:
// Validate() must reject an accepted/rejected Match with empty ReviewedBy.
func TestMatchValidate_ReviewedByRequiredWhenDecided(t *testing.T) {
	m := proposedMatch()
	accepted, err := m.Accept("r1", "ok", testReviewAt)
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	// Manually zero out ReviewedBy to simulate a hand-constructed broken record.
	accepted.ReviewedBy = ""
	if err := accepted.Validate(); err == nil {
		t.Error("expected error for accepted match with empty ReviewedBy, got nil")
	}

	rejected, err := m.Reject("r2", "nope", testReviewAt)
	if err != nil {
		t.Fatalf("Reject: %v", err)
	}
	rejected.ReviewedBy = ""
	if err := rejected.Validate(); err == nil {
		t.Error("expected error for rejected match with empty ReviewedBy, got nil")
	}
}

// TestMatchValidate_ReviewedAtRequiredWhenDecided is the regression test for Finding 1:
// Validate() must reject an accepted/rejected Match with zero ReviewedAt.
func TestMatchValidate_ReviewedAtRequiredWhenDecided(t *testing.T) {
	m := proposedMatch()
	accepted, _ := m.Accept("r1", "ok", testReviewAt)
	accepted.ReviewedAt = time.Time{}
	if err := accepted.Validate(); err == nil {
		t.Error("expected error for accepted match with zero ReviewedAt, got nil")
	}
}

// TestGenerateCandidates_DuplicateEntityIDs is the regression test for Finding 2:
// GenerateCandidates must skip pairs where the two entities share the same ID.
func TestGenerateCandidates_DuplicateEntityIDs(t *testing.T) {
	// Two entries share ID "e1": the swap logic would produce LeftID==RightID.
	entities := []er.Entity{
		{ID: "e1", Name: "Alice"},
		{ID: "e1", Name: "Bob"},
		{ID: "e2", Name: "Carol"},
	}
	cands := er.GenerateCandidates(entities, 0.0, validAuthor(), validGrade())
	for _, c := range cands {
		if c.LeftID == c.RightID {
			t.Errorf("self-pair found: %v", c)
		}
		if err := c.Validate(); err != nil {
			t.Errorf("invalid candidate generated: %v — %v", c, err)
		}
	}
}

// TestCandidateSourceSpanIDsCopy_DefensiveCopy verifies that SourceSpanIDsCopy returns
// a defensive copy: mutating the returned slice does not affect the next call.
func TestCandidateSourceSpanIDsCopy_DefensiveCopy(t *testing.T) {
	c := validCandidate()
	first := c.SourceSpanIDsCopy()
	if len(first) == 0 {
		t.Skip("no SourceSpanIDs to test defensive copy")
	}
	// Mutate the returned copy.
	first[0] = "mutated"
	// A second call must return the original value, not the mutated one.
	second := c.SourceSpanIDsCopy()
	if len(second) == 0 || second[0] == "mutated" {
		t.Errorf("SourceSpanIDsCopy is not returning an independent copy: second[0] = %q", second[0])
	}
}
