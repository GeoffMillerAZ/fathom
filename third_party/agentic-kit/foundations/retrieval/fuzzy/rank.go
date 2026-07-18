package fuzzy

import (
	"sort"
	"strings"
)

// Result pairs a candidate with its match score and matched positions.
type Result struct {
	// Index is the position of the candidate in the original input slice.
	Index int

	// Candidate is the original candidate string.
	Candidate string

	// Score is the fuzzy match score. Higher is better.
	Score int

	// Positions contains the haystack rune indices that were matched,
	// suitable for highlight rendering.
	Positions []int
}

// Rank returns matching candidates sorted by score descending, then by
// Index ascending (stable, deterministic tie-breaking).
//
// Matching uses MatchTerms semantics: all whitespace-separated terms in
// needle must appear as subsequences of the candidate (AND).
//
// When needle is empty or all whitespace, all candidates are returned in
// input order with Score 0 and nil Positions.
func Rank(needle string, candidates []string) []Result {
	if strings.TrimSpace(needle) == "" {
		results := make([]Result, len(candidates))
		for i, c := range candidates {
			results[i] = Result{Index: i, Candidate: c, Score: 0, Positions: nil}
		}
		return results
	}

	results := make([]Result, 0, len(candidates))
	for i, c := range candidates {
		s, pos, ok := MatchTerms(needle, c)
		if !ok {
			continue
		}
		results = append(results, Result{
			Index:     i,
			Candidate: c,
			Score:     s,
			Positions: pos,
		})
	}

	// Sort: score descending, Index ascending for deterministic tie-break.
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Index < results[j].Index
	})

	return results
}
