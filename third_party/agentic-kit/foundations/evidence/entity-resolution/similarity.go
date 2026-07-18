// similarity.go — pure-stdlib string similarity primitives for entity-resolution.
//
// Blend decision for Similarity(a, b):
//
//	average of NormalizedLevenshtein and JaccardSimilarity.
//	- NormalizedLevenshtein captures character-level edits (typos, abbreviations).
//	- Jaccard captures token-set overlap (reordered words, middle-name variants).
//	- Equal weight keeps the blend symmetric and simple; no tuning parameters.
//	- Both components are in [0,1] so the average is also in [0,1].
//
// Concurrency contract: all functions in this file are pure with no shared
// mutable state and are safe to call concurrently from multiple goroutines.
package entityresolution

import "strings"

// Levenshtein returns the edit distance between strings a and b (insertions,
// deletions, substitutions each cost 1). Result is 0 for identical strings.
// Pure stdlib; allocates one row of length len(b)+1.
func Levenshtein(a, b string) int {
	ra := []rune(a)
	rb := []rune(b)
	la, lb := len(ra), len(rb)

	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	// prev[j] = edit distance between ra[:i] and rb[:j]
	prev := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	curr := make([]int, lb+1)
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			curr[j] = min(del, min(ins, sub))
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

// NormalizedLevenshtein returns 1 - Levenshtein(a,b)/max(len(a),len(b)).
// Result is in [0, 1]. Returns 1.0 for two empty strings.
func NormalizedLevenshtein(a, b string) float64 {
	ra := []rune(a)
	rb := []rune(b)
	maxLen := len(ra)
	if len(rb) > maxLen {
		maxLen = len(rb)
	}
	if maxLen == 0 {
		return 1.0
	}
	dist := Levenshtein(a, b)
	return 1.0 - float64(dist)/float64(maxLen)
}

// JaccardSimilarity returns the Jaccard index of the token sets of a and b.
// Tokens are whitespace-split lowercased words. Duplicate words within a single
// string are deduplicated before computing the index (set semantics): for
// example, "alice alice" and "alice" have Jaccard = 1.0 because both produce
// the token set {"alice"}.
// Result is in [0, 1]. Returns 1.0 when both strings are empty.
func JaccardSimilarity(a, b string) float64 {
	setA := tokenSet(a)
	setB := tokenSet(b)

	if len(setA) == 0 && len(setB) == 0 {
		return 1.0
	}

	intersection := 0
	for tok := range setA {
		if setB[tok] {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 1.0
	}
	return float64(intersection) / float64(union)
}

// Similarity returns a combined similarity score in [0, 1] for two strings.
//
// Blend: average of NormalizedLevenshtein and JaccardSimilarity.
// Character-level edits (NL) + token-set overlap (Jaccard) at equal weight.
// Symmetric: Similarity(a, b) == Similarity(b, a).
func Similarity(a, b string) float64 {
	nl := NormalizedLevenshtein(a, b)
	jac := JaccardSimilarity(a, b)
	return (nl + jac) / 2.0
}

// tokenSet splits s on whitespace, lowercases each token, and returns the set.
func tokenSet(s string) map[string]bool {
	tokens := strings.Fields(strings.ToLower(s))
	if len(tokens) == 0 {
		return nil
	}
	m := make(map[string]bool, len(tokens))
	for _, t := range tokens {
		m[t] = true
	}
	return m
}

// minInt was the hand-rolled minimum function used before Go 1.21 introduced the
// builtin min. Replaced with the builtin in Go 1.26; this comment remains as a
// tombstone so reviewers understand the history.
