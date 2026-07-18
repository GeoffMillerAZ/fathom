// Package evidenceranker defines the CUE-first contract for the deterministic
// feature reranker over signature-derived signals. Go types in types.go mirror
// these definitions. CUE is authoritative for shape; Go is authoritative for
// behaviour.
//
// Design invariants (see types.go for full rationale):
//   1. Deterministic total order by default (no similarity, no LLM).
//   2. Similarity mode: optional, marked non-deterministic in result.
//   3. ABSENT_PROVEN flows: similarity mode is refused (not just discouraged).
//   4. Tie-break: explicit, by candidate ref string (ascending lexicographic).
package evidenceranker

// #RankWeights declares the relative importance of each feature signal.
// All fields are >= 0. The ranker normalises internally; callers do not need
// to ensure they sum to 1.
//
// Defaults are documented as EXAMPLE data; callers must supply their own
// weights via NewRanker.
//
// CUE mirror: #RankWeights in schema.cue.
#RankWeights: {
	// PatternStrength: how specific/precise the matched predicate is.
	// A narrow regexp outscores a broad substring match.
	pattern_strength: >=0

	// PathSpecificity: how deep or tight the glob scope is.
	// A deep, narrow path outscores a root-level wildcard.
	path_specificity: >=0

	// HitDensity: density of decisive findings relative to scope size.
	// More decisive hits per scoped file scores higher.
	hit_density: >=0

	// RefStability: hash-backed references outrank unanchored ones.
	// An anchored proof ref (non-empty AnchorProofRef) scores highest;
	// a content-hash-only ref scores intermediate; no hash scores 0.
	ref_stability: >=0

	// GradeTier: epistemic strength of the evidence grade.
	// TierDirect > TierDocumentary > … > TierInferred.
	grade_tier: >=0
}

// #DefaultWeights is the EXAMPLE default weight set shipped with the package.
// It is not normative; callers should tune weights to their corpus.
#DefaultWeights: #RankWeights & {
	pattern_strength: 0.3
	path_specificity: 0.2
	hit_density:      0.2
	ref_stability:    0.2
	grade_tier:       0.1
}

// #RankedCandidate is one input to the ranker: a retrieved chunk with its
// evidence features pre-computed by the caller (typically from a ChunkLink +
// search result).
//
// CUE mirror: #RankedCandidate in schema.cue.
#RankedCandidate: {
	// Ref is the stable string identifier (typically the chunk_id from a
	// ChunkLink). Used as the final tie-break key (ascending lexicographic).
	// Must be non-empty.
	ref: string & !=""

	// PatternStrength in [0, 1]: caller-computed specificity of the matched
	// predicate (e.g. 1.0 for a narrow regexp, 0.2 for a broad glob match).
	pattern_strength: >=0 & <=1

	// PathSpecificity in [0, 1]: caller-computed depth/tightness of the path
	// (e.g. 1.0 for an exact file path, 0.1 for "**/*").
	path_specificity: >=0 & <=1

	// HitDensity in [0, 1]: decisive findings / scope file count, capped to 1.
	hit_density: >=0 & <=1

	// RefStability in [0, 1]: stability tier of the reference.
	// 1.0 = anchor-proof-backed, 0.5 = content-hash-only, 0.0 = unanchored.
	ref_stability: >=0 & <=1

	// GradeTierRank in [0, 1]: normalised epistemic tier (evidencegrade.Tier.Rank() / maxRank).
	grade_tier_rank: >=0 & <=1
}

// #RankedResult is the output of the ranker for one candidate.
//
// CUE mirror: #RankedResult in schema.cue.
#RankedResult: {
	// Ref echoes the input candidate's Ref for correlation.
	ref: string & !=""

	// Score is the weighted linear combination of feature scores, in [0, 1].
	// Scores are normalised by the sum of weights; ties broken by Ref.
	score: >=0 & <=1

	// Rank is the 1-based position in the sorted output (1 = highest score).
	rank: >=1

	// Deterministic is true when no similarity mode was used. When false, the
	// result was influenced by an embedding similarity score — callers must not
	// use it in ABSENT_PROVEN flows.
	deterministic: bool

	// SimilarityScore is the embedding similarity used in non-deterministic mode.
	// 0 when deterministic is true.
	similarity_score: >=0 & <=1
}
