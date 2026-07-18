package evidenceranker_test

import (
	"context"
	"errors"
	"testing"

	embeddingclient "github.com/geoffmilleraz/agentic-kit/foundations/embedding/embedding-client"
	evidencegrade "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-grade"
	evidencesignature "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-signature"
	evidenceranker "github.com/geoffmilleraz/agentic-kit/foundations/retrieval/evidence-ranker"
)

// equalish reports whether a and b are equal within a small epsilon.
func equalish(a, b, epsilon float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < epsilon
}

// ---- NewRanker tests --------------------------------------------------------

func TestNewRanker_RejectsNegativeWeight(t *testing.T) {
	bad := evidenceranker.DefaultWeights
	bad.HitDensity = -1
	_, err := evidenceranker.NewRanker(bad, nil)
	if err == nil {
		t.Error("expected error for negative weight")
	}
}

func TestNewRanker_RejectsAllZeroWeights(t *testing.T) {
	_, err := evidenceranker.NewRanker(evidenceranker.RankWeights{}, nil)
	if err == nil {
		t.Error("expected error for all-zero weights")
	}
}

func TestNewRanker_AcceptsValidWeights(t *testing.T) {
	r, err := evidenceranker.NewRanker(evidenceranker.DefaultWeights, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r == nil {
		t.Error("expected non-nil ranker")
	}
}

// ---- Deterministic ranking tests -------------------------------------------

func TestRank_DeterministicOrder(t *testing.T) {
	r, _ := evidenceranker.NewRanker(evidenceranker.DefaultWeights, nil)

	candidates := []evidenceranker.Candidate{
		{Ref: "c1", PatternStrength: 0.1, PathSpecificity: 0.1, HitDensity: 0.1, RefStability: 0.0, GradeTierRank: 0.2},
		{Ref: "c2", PatternStrength: 0.9, PathSpecificity: 0.8, HitDensity: 0.7, RefStability: 1.0, GradeTierRank: 1.0},
		{Ref: "c3", PatternStrength: 0.5, PathSpecificity: 0.5, HitDensity: 0.5, RefStability: 0.5, GradeTierRank: 0.6},
	}

	req := evidenceranker.RankRequest{
		Archetype:  evidencesignature.ArchetypeExistence,
		Candidates: candidates,
	}

	got, err := r.Rank(context.Background(), req)
	if err != nil {
		t.Fatalf("Rank error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len(got) = %d; want 3", len(got))
	}
	// c2 should rank first (highest all-round features).
	if got[0].Ref != "c2" {
		t.Errorf("rank 1: Ref=%q; want c2", got[0].Ref)
	}
	if got[2].Ref != "c1" {
		t.Errorf("rank 3: Ref=%q; want c1", got[2].Ref)
	}
	// All results must be deterministic.
	for _, res := range got {
		if !res.Deterministic {
			t.Errorf("result %q: Deterministic=false in deterministic mode", res.Ref)
		}
		if res.SimilarityScore != 0 {
			t.Errorf("result %q: SimilarityScore=%g; want 0 in deterministic mode", res.Ref, res.SimilarityScore)
		}
	}
}

// TestRank_TieBreakByRef verifies that equal-score candidates are broken by
// Ref ascending.
func TestRank_TieBreakByRef(t *testing.T) {
	// Single-weight ranker so any two identical-feature candidates tie.
	w := evidenceranker.RankWeights{PatternStrength: 1.0}
	r, _ := evidenceranker.NewRanker(w, nil)

	candidates := []evidenceranker.Candidate{
		{Ref: "zebra", PatternStrength: 0.5},
		{Ref: "apple", PatternStrength: 0.5},
		{Ref: "mango", PatternStrength: 0.5},
	}

	got, err := r.Rank(context.Background(), evidenceranker.RankRequest{
		Archetype:  evidencesignature.ArchetypeExistence,
		Candidates: candidates,
	})
	if err != nil {
		t.Fatalf("Rank error: %v", err)
	}
	if got[0].Ref != "apple" {
		t.Errorf("rank 1: Ref=%q; want apple (tie-break by Ref asc)", got[0].Ref)
	}
	if got[1].Ref != "mango" {
		t.Errorf("rank 2: Ref=%q; want mango", got[1].Ref)
	}
	if got[2].Ref != "zebra" {
		t.Errorf("rank 3: Ref=%q; want zebra", got[2].Ref)
	}
}

// TestRank_RankNumbersSequential verifies Rank field is 1-based and sequential.
func TestRank_RankNumbersSequential(t *testing.T) {
	r, _ := evidenceranker.NewRanker(evidenceranker.DefaultWeights, nil)
	candidates := []evidenceranker.Candidate{
		{Ref: "a", PatternStrength: 0.9},
		{Ref: "b", PatternStrength: 0.5},
		{Ref: "c", PatternStrength: 0.1},
	}
	got, _ := r.Rank(context.Background(), evidenceranker.RankRequest{
		Archetype:  evidencesignature.ArchetypeExistence,
		Candidates: candidates,
	})
	for i, res := range got {
		if res.Rank != i+1 {
			t.Errorf("got[%d].Rank = %d; want %d", i, res.Rank, i+1)
		}
	}
}

// TestRank_ScoreInRange verifies all scores are in [0, 1].
func TestRank_ScoreInRange(t *testing.T) {
	r, _ := evidenceranker.NewRanker(evidenceranker.DefaultWeights, nil)
	candidates := []evidenceranker.Candidate{
		{Ref: "x", PatternStrength: 1.0, PathSpecificity: 1.0, HitDensity: 1.0, RefStability: 1.0, GradeTierRank: 1.0},
		{Ref: "y", PatternStrength: 0.0, PathSpecificity: 0.0, HitDensity: 0.0, RefStability: 0.0, GradeTierRank: 0.0},
	}
	got, _ := r.Rank(context.Background(), evidenceranker.RankRequest{
		Archetype:  evidencesignature.ArchetypeExistence,
		Candidates: candidates,
	})
	for _, res := range got {
		if res.Score < 0 || res.Score > 1 {
			t.Errorf("Score %g out of [0,1] for Ref=%q", res.Score, res.Ref)
		}
	}
}

// ---- ABSENT_PROVEN guard tests ---------------------------------------------

// stubEmbedder is a minimal Embedder for testing similarity-mode refusal.
type stubEmbedder struct{}

func (s *stubEmbedder) Embed(_ context.Context, _ string) (embeddingclient.Vector, error) {
	return embeddingclient.Vector{1.0, 0.0}, nil
}

func (s *stubEmbedder) EmbedBatch(_ context.Context, texts []string) ([]embeddingclient.Vector, error) {
	vecs := make([]embeddingclient.Vector, len(texts))
	for i := range texts {
		vecs[i] = embeddingclient.Vector{1.0, 0.0}
	}
	return vecs, nil
}

func TestRank_AbsenceRefusesSimilarityMode(t *testing.T) {
	r, _ := evidenceranker.NewRanker(evidenceranker.DefaultWeights, &stubEmbedder{})

	req := evidenceranker.RankRequest{
		Archetype: evidencesignature.ArchetypeAbsence,
		Candidates: []evidenceranker.Candidate{
			{Ref: "c1", PatternStrength: 0.5},
		},
		Query:            "find the thing",
		SimilarityWeight: 0.3,
	}

	_, err := r.Rank(context.Background(), req)
	if err == nil {
		t.Fatal("expected ErrSimilarityRefusedForAbsence, got nil")
	}
	if !errors.Is(err, evidenceranker.ErrSimilarityRefusedForAbsence) {
		t.Errorf("error = %v; want ErrSimilarityRefusedForAbsence", err)
	}
}

// TestRank_AbsenceDeterministicOK verifies that absence archetype with no
// query / no embedder works fine deterministically.
func TestRank_AbsenceDeterministicOK(t *testing.T) {
	r, _ := evidenceranker.NewRanker(evidenceranker.DefaultWeights, nil)
	req := evidenceranker.RankRequest{
		Archetype: evidencesignature.ArchetypeAbsence,
		Candidates: []evidenceranker.Candidate{
			{Ref: "c1", PatternStrength: 0.5},
		},
	}
	got, err := r.Rank(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got[0].Deterministic {
		t.Error("expected Deterministic=true for absence archetype without query/embedder")
	}
}

// ---- Similarity mode tests -------------------------------------------------

func TestRank_SimilarityModeMarksDeterministicFalse(t *testing.T) {
	r, _ := evidenceranker.NewRanker(evidenceranker.DefaultWeights, &stubEmbedder{})
	req := evidenceranker.RankRequest{
		Archetype: evidencesignature.ArchetypeExistence,
		Candidates: []evidenceranker.Candidate{
			{Ref: "c1", PatternStrength: 0.5},
			{Ref: "c2", PatternStrength: 0.3},
		},
		Query:            "some query",
		SimilarityWeight: 0.3,
	}
	got, err := r.Rank(context.Background(), req)
	if err != nil {
		t.Fatalf("Rank error: %v", err)
	}
	for _, res := range got {
		if res.Deterministic {
			t.Errorf("result %q: Deterministic=true in similarity mode; want false", res.Ref)
		}
	}
}

func TestRank_EmptyCandidatesError(t *testing.T) {
	r, _ := evidenceranker.NewRanker(evidenceranker.DefaultWeights, nil)
	_, err := r.Rank(context.Background(), evidenceranker.RankRequest{
		Archetype: evidencesignature.ArchetypeExistence,
	})
	if err == nil {
		t.Error("expected error for empty candidates")
	}
}

func TestRank_InvalidSimilarityWeight(t *testing.T) {
	r, _ := evidenceranker.NewRanker(evidenceranker.DefaultWeights, nil)
	_, err := r.Rank(context.Background(), evidenceranker.RankRequest{
		Archetype:        evidencesignature.ArchetypeExistence,
		Candidates:       []evidenceranker.Candidate{{Ref: "c1", PatternStrength: 0.5}},
		SimilarityWeight: 1.5, // invalid
	})
	if err == nil {
		t.Error("expected error for SimilarityWeight > 1")
	}
}

// ---- Feature helper tests --------------------------------------------------

func TestPathSpecificityFromGlob_Empty(t *testing.T) {
	if evidenceranker.PathSpecificityFromGlob("") != 0 {
		t.Error("expected 0 for empty glob")
	}
}

func TestPathSpecificityFromGlob_ExactFile(t *testing.T) {
	// Exact single-file path → high specificity.
	s := evidenceranker.PathSpecificityFromGlob("foundations/retrieval/truth-bridge/types.go")
	if s <= 0.3 {
		t.Errorf("exact path specificity = %g; expected > 0.3", s)
	}
}

func TestPathSpecificityFromGlob_BroadWildcard(t *testing.T) {
	// "**/*" → very low specificity.
	s := evidenceranker.PathSpecificityFromGlob("**/*")
	if s > 0.3 {
		t.Errorf("broad wildcard specificity = %g; expected <= 0.3", s)
	}
}

func TestPathSpecificityFromGlob_InRange(t *testing.T) {
	globs := []string{
		"**/*",
		"foundations/*.go",
		"foundations/retrieval/**/*.go",
		"foundations/retrieval/truth-bridge/types.go",
		"*",
	}
	for _, g := range globs {
		s := evidenceranker.PathSpecificityFromGlob(g)
		if s < 0 || s > 1 {
			t.Errorf("PathSpecificityFromGlob(%q) = %g; want in [0,1]", g, s)
		}
	}
}

func TestHitDensityFrom_ZeroScope(t *testing.T) {
	if evidenceranker.HitDensityFrom(5, 0) != 0 {
		t.Error("expected 0 for zero scope")
	}
}

func TestHitDensityFrom_Capped(t *testing.T) {
	if evidenceranker.HitDensityFrom(100, 1) != 1.0 {
		t.Errorf("expected 1.0 for hits >> scope, got %g", evidenceranker.HitDensityFrom(100, 1))
	}
}

func TestHitDensityFrom_Partial(t *testing.T) {
	d := evidenceranker.HitDensityFrom(1, 4)
	if !equalish(d, 0.25, 1e-9) {
		t.Errorf("HitDensityFrom(1,4) = %g; want 0.25", d)
	}
}

func TestRefStabilityFrom_Levels(t *testing.T) {
	cases := []struct {
		hash   string
		anchor string
		want   float64
	}{
		{"", "", 0.0},
		{"abc123def", "", 0.5},
		{"abc123def", "proofref", 1.0},
		{"", "proofref", 1.0},
	}
	for _, tc := range cases {
		got := evidenceranker.RefStabilityFrom(tc.hash, tc.anchor)
		if got != tc.want {
			t.Errorf("RefStabilityFrom(%q,%q) = %g; want %g", tc.hash, tc.anchor, got, tc.want)
		}
	}
}

func TestGradeTierRankFrom_AllTiers(t *testing.T) {
	cases := []struct {
		tier evidencegrade.Tier
		want float64
	}{
		{evidencegrade.TierDirect, 1.0},
		{evidencegrade.TierDocumentary, 0.8},
		{evidencegrade.TierCircumstantial, 0.6},
		{evidencegrade.TierTestimonial, 0.4},
		{evidencegrade.TierInferred, 0.2},
		{"unknown-tier", 0.0},
	}
	for _, tc := range cases {
		got := evidenceranker.GradeTierRankFrom(tc.tier)
		if !equalish(got, tc.want, 1e-9) {
			t.Errorf("GradeTierRankFrom(%q) = %g; want %g", tc.tier, got, tc.want)
		}
	}
}

// ---- Determinism property test ---------------------------------------------

// TestRank_Deterministic verifies that calling Rank twice with the same inputs
// returns byte-identical results.
func TestRank_Deterministic(t *testing.T) {
	r, _ := evidenceranker.NewRanker(evidenceranker.DefaultWeights, nil)
	candidates := []evidenceranker.Candidate{
		{Ref: "c1", PatternStrength: 0.7, PathSpecificity: 0.4, HitDensity: 0.3, RefStability: 0.5, GradeTierRank: 0.8},
		{Ref: "c2", PatternStrength: 0.3, PathSpecificity: 0.9, HitDensity: 0.6, RefStability: 1.0, GradeTierRank: 0.4},
		{Ref: "c3", PatternStrength: 0.5, PathSpecificity: 0.5, HitDensity: 0.5, RefStability: 0.5, GradeTierRank: 0.5},
	}
	req := evidenceranker.RankRequest{
		Archetype:  evidencesignature.ArchetypeExistence,
		Candidates: candidates,
	}

	got1, _ := r.Rank(context.Background(), req)
	got2, _ := r.Rank(context.Background(), req)

	for i := range got1 {
		if got1[i].Ref != got2[i].Ref || got1[i].Score != got2[i].Score || got1[i].Rank != got2[i].Rank {
			t.Errorf("result[%d] differs between runs: %+v vs %+v", i, got1[i], got2[i])
		}
	}
}
