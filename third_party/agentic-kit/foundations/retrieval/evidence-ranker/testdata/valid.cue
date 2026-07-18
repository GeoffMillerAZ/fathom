// Positive CUE fixture for evidence-ranker schema conformance.
// cue vet schema.cue testdata/valid.cue must pass.
package evidenceranker

// Example default weight configuration.
_weights: #RankWeights & {
	pattern_strength: 0.3
	path_specificity: 0.2
	hit_density:      0.2
	ref_stability:    0.2
	grade_tier:       0.1
}

// Example candidate for a well-anchored, highly-specific finding.
_bestCandidate: #RankedCandidate & {
	ref:              "chunk-001"
	pattern_strength: 0.95
	path_specificity: 0.8
	hit_density:      0.6
	ref_stability:    1.0
	grade_tier_rank:  1.0
}

// Example candidate that is unanchored and inferred.
_weakCandidate: #RankedCandidate & {
	ref:              "chunk-002"
	pattern_strength: 0.1
	path_specificity: 0.1
	hit_density:      0.0
	ref_stability:    0.0
	grade_tier_rank:  0.2
}

// Example ranked result (deterministic, rank 1).
_topResult: #RankedResult & {
	ref:              "chunk-001"
	score:            0.87
	rank:             1
	deterministic:    true
	similarity_score: 0.0
}

// Example ranked result (non-deterministic, similarity mode).
_simResult: #RankedResult & {
	ref:              "chunk-003"
	score:            0.72
	rank:             2
	deterministic:    false
	similarity_score: 0.65
}
