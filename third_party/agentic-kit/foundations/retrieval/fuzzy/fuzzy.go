package fuzzy

import (
	"sort"
	"strings"
	"unicode"
)

// Score weights — documented as constants for caller reference.
const (
	// ScorePerRune is added for every matched rune.
	ScorePerRune = 2

	// ScoreBoundaryBonus is added when a matched rune starts at a word
	// boundary: the beginning of the string, or immediately following a
	// space or ASCII punctuation rune.
	ScoreBoundaryBonus = 3

	// ScoreContinuationBonus is added for each rune that continues an
	// already-started contiguous run (i.e. the second, third, … rune of
	// a back-to-back match sequence).
	ScoreContinuationBonus = 2

	// ScoreGapPenalty is subtracted once for every haystack rune that
	// lies between two matched runes. The cumulative penalty is floored
	// at zero.
	ScoreGapPenalty = 1
)

// Match scores needle against haystack using a case-insensitive subsequence
// match. It returns ok=false when needle is not a subsequence of haystack.
// Positions contains the haystack rune indices that were matched (in order),
// suitable for highlight rendering.
func Match(needle, haystack string) (score int, positions []int, ok bool) {
	nr := []rune(strings.ToLower(needle))
	hr := []rune(strings.ToLower(haystack))
	origHr := []rune(haystack) // for boundary detection (original casing irrelevant — we use unicode)

	if len(nr) == 0 {
		return 0, []int{}, true
	}

	// Greedy left-to-right subsequence match.
	positions = make([]int, 0, len(nr))
	hi := 0
	for _, n := range nr {
		matched := false
		for hi < len(hr) {
			if hr[hi] == n {
				positions = append(positions, hi)
				hi++
				matched = true
				break
			}
			hi++
		}
		if !matched {
			return 0, nil, false
		}
	}

	// Compute score from positions.
	score = computeScore(positions, origHr)
	return score, positions, true
}

// computeScore derives the score from matched positions within the haystack.
func computeScore(positions []int, haystack []rune) int {
	score := 0
	prevPos := -2 // sentinel: no previous match

	for i, pos := range positions {
		score += ScorePerRune

		// Word boundary bonus.
		if isBoundary(pos, haystack) {
			score += ScoreBoundaryBonus
		}

		// Continuation bonus: this rune is adjacent to the previous matched rune.
		if i > 0 && pos == prevPos+1 {
			score += ScoreContinuationBonus
		}

		// Gap penalty: count runes skipped since last match.
		if i > 0 {
			gap := pos - prevPos - 1
			if gap > 0 {
				score -= gap * ScoreGapPenalty
			}
		}

		prevPos = pos
	}

	if score < 0 {
		score = 0
	}
	return score
}

// isBoundary returns true when haystack[pos] starts a word boundary:
// pos == 0, or the preceding rune is a space or ASCII punctuation.
func isBoundary(pos int, haystack []rune) bool {
	if pos == 0 {
		return true
	}
	prev := haystack[pos-1]
	return unicode.IsSpace(prev) || isASCIIPunct(prev)
}

// isASCIIPunct returns true for common ASCII punctuation characters.
func isASCIIPunct(r rune) bool {
	return (r >= '!' && r <= '/') ||
		(r >= ':' && r <= '@') ||
		(r >= '[' && r <= '`') ||
		(r >= '{' && r <= '~')
}

// MatchTerms splits needle on whitespace and requires every term to match
// (AND semantics). Score is the sum of per-term scores. Positions are the
// sorted, deduplicated union of all per-term positions.
// Returns ok=false if any term fails to match.
func MatchTerms(needle, haystack string) (score int, positions []int, ok bool) {
	terms := strings.Fields(needle)
	if len(terms) == 0 {
		return 0, []int{}, true
	}

	totalScore := 0
	posSet := make(map[int]struct{})

	for _, term := range terms {
		s, pos, matched := Match(term, haystack)
		if !matched {
			return 0, nil, false
		}
		totalScore += s
		for _, p := range pos {
			posSet[p] = struct{}{}
		}
	}

	allPos := make([]int, 0, len(posSet))
	for p := range posSet {
		allPos = append(allPos, p)
	}
	sort.Ints(allPos)

	return totalScore, allPos, true
}
