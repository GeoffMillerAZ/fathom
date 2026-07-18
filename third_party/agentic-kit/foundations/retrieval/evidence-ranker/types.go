// Package evidenceranker is the LLM-free deterministic feature reranker for
// the evidence-spine RAG layer. It ranks retrieved candidates by a weighted
// linear combination of signature-derived signals (pattern strength, path
// specificity, hit density, ref stability, grade tier) and produces a
// deterministic total order with an explicit tie-break.
//
// # Design principles
//
// Determinism first: the default ranking path is fully deterministic. Given
// the same candidates and weights, Rank always returns the same order. There
// are no hidden sources of randomness, no LLM calls, and no clock reads.
//
// Optional similarity mode: when an Embedder is wired and Rank is called with
// a non-empty query, similarity scores are folded into the ranking. The result
// is marked non-deterministic (RankedResult.Deterministic == false) so
// downstream callers know they must not use it in evidence-graded flows that
// require reproducibility.
//
// ABSENT_PROVEN refusal: similarity mode is explicitly refused when the
// request archetype is ArchetypeAbsence. An absence verdict depends on a
// deterministic total-coverage search; a similarity-influenced reranking could
// suppress findings near the decision boundary and flip the verdict. If a
// caller attempts to use similarity mode for an absence concern, Rank returns
// an error.
//
// Tie-break: when two candidates have equal weighted scores, the tie is broken
// by the candidate's Ref string (ascending lexicographic order). This makes
// the total order unique and stable across runs.
//
// # Feature definitions
//
//   - PatternStrength [0,1]: caller-computed. 1.0 for a narrow, specific regexp
//     or exact predicate; 0.0 for the broadest possible match (e.g. ".*").
//   - PathSpecificity [0,1]: caller-computed. 1.0 for an exact single-file
//     scope; 0.0 for a root-level "**/*" wildcard. Computed from glob depth
//     and wildcard count; PathSpecificityFromGlob is a convenience helper.
//   - HitDensity [0,1]: decisive findings / scope file count, capped at 1.0.
//     Prefer candidates with many hits in a small scope.
//   - RefStability [0,1]: 1.0 when AnchorProofRef is non-empty (hash-proven);
//     0.5 when only a ContentHash is known; 0.0 when neither is known.
//   - GradeTierRank [0,1]: evidencegrade.Tier.Rank() / maxTierRank (= 5).
//     TierDirect → 1.0; TierInferred → 0.2.
//
// # Non-guarantees
//
// Rank does not validate the semantic content of candidates. A caller that
// computes incorrect feature values will get a valid but meaningless ranking.
// Feature computation helpers (PathSpecificityFromGlob, HitDensityFrom,
// RefStabilityFrom) are provided to reduce common mistakes.
//
// Concurrency: Ranker is safe for concurrent use after construction (it holds
// no mutable state after NewRanker returns). Rank is safe for concurrent use.
// The optional Embedder must itself be concurrency-safe; if it is not, the
// caller must guard external calls.
//
// CUE contracts live in schema.cue; this file mirrors those definitions.
package evidenceranker

import (
	"context"
	"fmt"
	"sort"
	"strings"

	embeddingclient "github.com/geoffmilleraz/agentic-kit/foundations/embedding/embedding-client"
	evidencegrade "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-grade"
	evidencesignature "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-signature"
)

// maxTierRank is the maximum Rank() value for any evidencegrade.Tier (TierDirect).
// Used to normalise GradeTierRank to [0, 1].
const maxTierRank = 5

// RankWeights declares the relative importance of each feature signal.
// All fields must be >= 0; negative weights are rejected by NewRanker.
// The ranker normalises internally so callers need not ensure the sum is 1.
//
// CUE mirror: #RankWeights in schema.cue.
type RankWeights struct {
	// PatternStrength: how specific/precise the matched predicate is.
	PatternStrength float64

	// PathSpecificity: how deep or tight the glob scope is.
	PathSpecificity float64

	// HitDensity: density of decisive findings relative to scope size.
	HitDensity float64

	// RefStability: hash-backed references outrank unanchored ones.
	RefStability float64

	// GradeTier: epistemic strength of the evidence grade.
	GradeTier float64
}

// DefaultWeights is the EXAMPLE default weight set. It is not normative;
// callers should tune weights to their corpus.
//
// Documented as EXAMPLE per CLAUDE.md: these are starting values, not spec.
//
// CUE mirror: #DefaultWeights in schema.cue.
var DefaultWeights = RankWeights{
	PatternStrength: 0.3,
	PathSpecificity: 0.2,
	HitDensity:      0.2,
	RefStability:    0.2,
	GradeTier:       0.1,
}

// Candidate is one input to the ranker: a retrieved chunk with its evidence
// features pre-computed by the caller (typically from a ChunkLink + search result).
//
// CUE mirror: #RankedCandidate in schema.cue.
type Candidate struct {
	// Ref is the stable string identifier (chunk_id from a ChunkLink or
	// vectorstore.Entry.ID). Used as the final tie-break key. Must be non-empty.
	Ref string

	// PatternStrength in [0, 1]: specificity of the matched predicate.
	// See package doc for definition. Caller-computed.
	PatternStrength float64

	// PathSpecificity in [0, 1]: depth/tightness of the scope path.
	// See PathSpecificityFromGlob for a ready-made computation helper.
	// Caller-computed.
	PathSpecificity float64

	// HitDensity in [0, 1]: decisive findings / scope file count, capped to 1.
	// See HitDensityFrom for a computation helper.
	// Caller-computed.
	HitDensity float64

	// RefStability in [0, 1]: stability tier of the reference.
	// See RefStabilityFrom for a computation helper.
	// Caller-computed.
	RefStability float64

	// GradeTierRank in [0, 1]: normalised epistemic tier.
	// See GradeTierRankFrom for a computation helper.
	// Caller-computed.
	GradeTierRank float64
}

// RankedResult is the output of the ranker for one candidate.
//
// CUE mirror: #RankedResult in schema.cue.
type RankedResult struct {
	// Ref echoes the input candidate's Ref for correlation.
	Ref string

	// Score is the weighted linear combination of feature scores, in [0, 1].
	Score float64

	// Rank is the 1-based position in the sorted output (1 = highest score).
	Rank int

	// Deterministic is true when no similarity mode was used. False means an
	// embedding similarity score influenced the result — callers must not use
	// this result in ABSENT_PROVEN flows.
	Deterministic bool

	// SimilarityScore is the embedding similarity score used in non-deterministic
	// mode (cosine similarity against the query). 0 when Deterministic is true.
	SimilarityScore float64
}

// RankRequest is the input to Ranker.Rank.
type RankRequest struct {
	// Archetype is the concern archetype for this ranking request.
	// If Archetype is ArchetypeAbsence and an Embedder is wired, Rank
	// returns ErrSimilarityRefusedForAbsence.
	Archetype evidencesignature.Archetype

	// Candidates is the list of candidates to rank. Must be non-empty.
	Candidates []Candidate

	// Query is the natural-language query for similarity-mode ranking.
	// When empty or when the Embedder is nil, deterministic mode is used.
	Query string

	// SimilarityWeight is how much to blend the similarity score into the
	// final score in similarity mode: score = (1-w)*featureScore + w*simScore.
	// Must be in [0, 1]. Ignored in deterministic mode.
	SimilarityWeight float64
}

// ErrSimilarityRefusedForAbsence is returned when Rank is asked to use
// similarity mode (non-empty Query + wired Embedder) for an absence archetype.
// Absence verdicts require deterministic search coverage; similarity-influenced
// reranking may suppress findings and flip the verdict.
var ErrSimilarityRefusedForAbsence = fmt.Errorf("evidenceranker: similarity mode refused for absence archetype — use deterministic mode only")

// Ranker is the evidence feature ranker. It holds caller-configured weights
// and an optional Embedder. After construction with NewRanker it is immutable.
//
// Concurrency: Ranker is safe for concurrent use. Rank may be called from
// multiple goroutines simultaneously.
type Ranker struct {
	weights   RankWeights
	weightSum float64
	embedder  embeddingclient.Embedder // nil → always deterministic
}

// NewRanker constructs a Ranker with the given weights and optional Embedder.
//
// Returns an error if any weight is negative or if all weights are zero (a
// zero-weight ranker cannot produce a meaningful order).
//
// embedder may be nil. When nil, Rank always uses deterministic mode regardless
// of the Query field in RankRequest.
func NewRanker(weights RankWeights, embedder embeddingclient.Embedder) (*Ranker, error) {
	if err := validateWeights(weights); err != nil {
		return nil, err
	}
	sum := weights.PatternStrength + weights.PathSpecificity +
		weights.HitDensity + weights.RefStability + weights.GradeTier
	if sum == 0 {
		return nil, fmt.Errorf("evidenceranker: all weights are zero; at least one must be positive")
	}
	return &Ranker{
		weights:   weights,
		weightSum: sum,
		embedder:  embedder,
	}, nil
}

// Rank reranks the candidates in req and returns a sorted slice of RankedResult
// (rank 1 = best). The slice has the same length as req.Candidates.
//
// Deterministic mode (default): all results have Deterministic == true and
// SimilarityScore == 0. The order is: feature score descending; ties broken by
// Ref ascending (lexicographic). This order is deterministic and reproducible.
//
// Similarity mode: activated when req.Query is non-empty AND the Ranker was
// constructed with a non-nil Embedder. Each candidate's feature score is
// blended with its cosine similarity to the query using req.SimilarityWeight.
// Results have Deterministic == false and SimilarityScore set. Tie-break is
// still by Ref ascending.
//
// ABSENT_PROVEN guard: if req.Archetype == ArchetypeAbsence and similarity
// mode would be activated (non-empty Query + non-nil Embedder), Rank returns
// ErrSimilarityRefusedForAbsence with a nil slice. The caller must either
// clear the Query or nil the Embedder to use deterministic mode.
//
// Returns an error when:
//   - req.Candidates is empty.
//   - req.SimilarityWeight is outside [0, 1].
//   - The absence guard is triggered.
//   - The Embedder fails (embedder errors propagate directly).
func (r *Ranker) Rank(ctx context.Context, req RankRequest) ([]RankedResult, error) {
	if len(req.Candidates) == 0 {
		return nil, fmt.Errorf("evidenceranker: Rank: candidates must not be empty")
	}
	if req.SimilarityWeight < 0 || req.SimilarityWeight > 1 {
		return nil, fmt.Errorf("evidenceranker: Rank: SimilarityWeight %g is outside [0, 1]", req.SimilarityWeight)
	}

	// Determine mode.
	useSimilarity := req.Query != "" && r.embedder != nil

	// ABSENT_PROVEN guard: refuse similarity mode for absence archetypes.
	if useSimilarity && req.Archetype == evidencesignature.ArchetypeAbsence {
		return nil, ErrSimilarityRefusedForAbsence
	}

	// Build intermediate records.
	type intermediate struct {
		cand     Candidate
		feature  float64
		simScore float64
		final    float64
	}
	items := make([]intermediate, len(req.Candidates))
	for i, c := range req.Candidates {
		items[i] = intermediate{
			cand:    c,
			feature: r.featureScore(c),
		}
	}

	if useSimilarity {
		// Embed the query and all candidate texts via their Ref as a proxy text.
		// (The Ref is the chunk ID; callers provide the full text at index time,
		// but the ranker only has the Ref. Use Ref as the similarity target since
		// the full text is not carried in Candidate — see doc for rationale.)
		texts := make([]string, len(req.Candidates))
		for i, c := range req.Candidates {
			texts[i] = c.Ref
		}
		qVec, err := r.embedder.Embed(ctx, req.Query)
		if err != nil {
			return nil, fmt.Errorf("evidenceranker: Rank: embedding query: %w", err)
		}
		docVecs, err := r.embedder.EmbedBatch(ctx, texts)
		if err != nil {
			return nil, fmt.Errorf("evidenceranker: Rank: embedding candidates: %w", err)
		}
		for i := range items {
			var sim float64
			if i < len(docVecs) {
				sim = cosineSimilarity(qVec, docVecs[i])
			}
			items[i].simScore = sim
			w := req.SimilarityWeight
			items[i].final = (1-w)*items[i].feature + w*sim
		}
	} else {
		for i := range items {
			items[i].final = items[i].feature
		}
	}

	// Sort: descending final score; ties broken by Ref ascending.
	sort.SliceStable(items, func(a, b int) bool {
		if items[a].final != items[b].final {
			return items[a].final > items[b].final
		}
		return items[a].cand.Ref < items[b].cand.Ref
	})

	out := make([]RankedResult, len(items))
	for i, item := range items {
		det := !useSimilarity
		out[i] = RankedResult{
			Ref:             item.cand.Ref,
			Score:           item.final,
			Rank:            i + 1,
			Deterministic:   det,
			SimilarityScore: item.simScore,
		}
	}
	return out, nil
}

// featureScore computes the normalised weighted linear feature score for c.
// Returns a value in [0, 1].
func (r *Ranker) featureScore(c Candidate) float64 {
	raw := r.weights.PatternStrength*c.PatternStrength +
		r.weights.PathSpecificity*c.PathSpecificity +
		r.weights.HitDensity*c.HitDensity +
		r.weights.RefStability*c.RefStability +
		r.weights.GradeTier*c.GradeTierRank
	// Normalise by weightSum so the score is in [0, 1] when all inputs are in [0,1].
	return raw / r.weightSum
}

// ---------------------------------------------------------------------------
// Feature computation helpers.
// ---------------------------------------------------------------------------

// PathSpecificityFromGlob computes a PathSpecificity value in [0, 1] for a
// single glob pattern. The heuristic is:
//
//   - depth: number of path segments (slash-separated parts), normalised to a
//     max of 10 (beyond 10 the increment is negligible).
//   - wildcard penalty: each '*' in the pattern reduces specificity by 0.05,
//     each '**' (double star) reduces by 0.15. Penalties are cumulative.
//
// The final value is max(0, depthScore - wildcardPenalty).
// An empty glob returns 0.
func PathSpecificityFromGlob(glob string) float64 {
	if glob == "" {
		return 0
	}
	parts := strings.Split(strings.Trim(glob, "/"), "/")
	depth := float64(len(parts))
	if depth > 10 {
		depth = 10
	}
	depthScore := depth / 10.0

	// Count wildcards.
	var penalty float64
	i := 0
	for i < len(glob) {
		if i+1 < len(glob) && glob[i] == '*' && glob[i+1] == '*' {
			penalty += 0.15
			i += 2
			continue
		}
		if glob[i] == '*' {
			penalty += 0.05
		}
		i++
	}

	score := depthScore - penalty
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return score
}

// HitDensityFrom computes a HitDensity value in [0, 1].
//
//   - decisiveHits: count of decisive findings for this candidate.
//   - scopeFileCount: total files in the candidate's scope.
//
// Returns 0 when scopeFileCount <= 0. Returns 1 when hits >= scopeFileCount.
func HitDensityFrom(decisiveHits, scopeFileCount int) float64 {
	if scopeFileCount <= 0 {
		return 0
	}
	d := float64(decisiveHits) / float64(scopeFileCount)
	if d > 1 {
		return 1
	}
	if d < 0 {
		return 0
	}
	return d
}

// RefStabilityFrom computes a RefStability value in [0, 1]:
//
//   - anchorProofRef non-empty: 1.0 (anchor-proof-backed — highest stability).
//   - contentHash non-empty: 0.5 (content-hash-backed — intermediate).
//   - both empty: 0.0 (unanchored — lowest stability).
//
// These discrete levels reflect the trust hierarchy: an anchor proof provides
// a time-stamped, independently-verifiable witness; a content hash ensures
// content integrity but provides no external witness; no hash provides no
// integrity guarantee at all.
func RefStabilityFrom(contentHash, anchorProofRef string) float64 {
	if anchorProofRef != "" {
		return 1.0
	}
	if contentHash != "" {
		return 0.5
	}
	return 0.0
}

// GradeTierRankFrom computes a GradeTierRank value in [0, 1] for a given
// evidencegrade.Tier. The normalisation factor is maxTierRank (5).
//
//   - TierDirect → 1.0
//   - TierDocumentary → 0.8
//   - TierCircumstantial → 0.6
//   - TierTestimonial → 0.4
//   - TierInferred → 0.2
//   - unknown tier → 0.0
func GradeTierRankFrom(tier evidencegrade.Tier) float64 {
	r := tier.Rank()
	if r <= 0 {
		return 0
	}
	return float64(r) / float64(maxTierRank)
}

// ---------------------------------------------------------------------------
// Private helpers.
// ---------------------------------------------------------------------------

// validateWeights returns an error if any weight is negative.
func validateWeights(w RankWeights) error {
	if w.PatternStrength < 0 {
		return fmt.Errorf("evidenceranker: PatternStrength weight must be >= 0, got %g", w.PatternStrength)
	}
	if w.PathSpecificity < 0 {
		return fmt.Errorf("evidenceranker: PathSpecificity weight must be >= 0, got %g", w.PathSpecificity)
	}
	if w.HitDensity < 0 {
		return fmt.Errorf("evidenceranker: HitDensity weight must be >= 0, got %g", w.HitDensity)
	}
	if w.RefStability < 0 {
		return fmt.Errorf("evidenceranker: RefStability weight must be >= 0, got %g", w.RefStability)
	}
	if w.GradeTier < 0 {
		return fmt.Errorf("evidenceranker: GradeTier weight must be >= 0, got %g", w.GradeTier)
	}
	return nil
}

// cosineSimilarity computes the cosine similarity between two vectors.
// Returns 0 if either vector is nil or has zero magnitude.
func cosineSimilarity(a, b embeddingclient.Vector) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n == 0 {
		return 0
	}
	var dot, magA, magB float64
	for i := 0; i < n; i++ {
		dot += a[i] * b[i]
		magA += a[i] * a[i]
		magB += b[i] * b[i]
	}
	if magA == 0 || magB == 0 {
		return 0
	}
	return dot / (sqrt(magA) * sqrt(magB))
}

// sqrt computes the square root of x using Newton's method (stdlib-only,
// avoids importing math for just this function).
func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// Initial estimate.
	z := x
	for i := 0; i < 50; i++ {
		z1 := z - (z*z-x)/(2*z)
		if z1 == z {
			break
		}
		z = z1
	}
	return z
}
