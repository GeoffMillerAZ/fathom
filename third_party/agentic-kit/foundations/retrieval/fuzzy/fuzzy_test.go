package fuzzy_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/retrieval/fuzzy"
)

// --- Match ---

func TestMatchEmptyNeedle(t *testing.T) {
	t.Parallel()

	score, pos, ok := fuzzy.Match("", "hello")
	if !ok {
		t.Fatal("empty needle must match")
	}
	if score != 0 {
		t.Errorf("empty needle score = %d, want 0", score)
	}
	if len(pos) != 0 {
		t.Errorf("expected empty positions, got %v", pos)
	}
}

func TestMatchNoMatch(t *testing.T) {
	t.Parallel()

	_, _, ok := fuzzy.Match("xyz", "abcdef")
	if ok {
		t.Error("non-subsequence must return ok=false")
	}
}

func TestMatchExactReturnsOK(t *testing.T) {
	t.Parallel()

	score, pos, ok := fuzzy.Match("hello", "hello")
	if !ok {
		t.Fatal("exact match must return ok=true")
	}
	if score <= 0 {
		t.Errorf("exact match score = %d, want >0", score)
	}
	if len(pos) != 5 {
		t.Errorf("exact match positions = %v, want 5 entries", pos)
	}
}

func TestMatchCaseInsensitive(t *testing.T) {
	t.Parallel()

	_, _, ok := fuzzy.Match("HELLO", "hello world")
	if !ok {
		t.Error("match must be case-insensitive")
	}
}

func TestMatchPositionsCorrect(t *testing.T) {
	t.Parallel()

	// needle "ac" in "abcde" → positions [0, 2]
	_, pos, ok := fuzzy.Match("ac", "abcde")
	if !ok {
		t.Fatal("expected match")
	}
	if len(pos) != 2 || pos[0] != 0 || pos[1] != 2 {
		t.Errorf("positions = %v, want [0 2]", pos)
	}
}

func TestMatchExactBeatsSparse(t *testing.T) {
	t.Parallel()

	// "test" contiguous at start of "testing" (boundary + 3 continuations, no gaps)
	// vs scattered mid-word in "taxeasytail" (no boundaries after first, 3 multi-char gaps).
	scoreContiguous, _, ok1 := fuzzy.Match("test", "testing")
	scoreSparse, _, ok2 := fuzzy.Match("test", "taxeasytail")
	if !ok1 || !ok2 {
		t.Fatal("both must match")
	}
	if scoreContiguous <= scoreSparse {
		t.Errorf("contiguous (%d) should beat sparse (%d)", scoreContiguous, scoreSparse)
	}
}

func TestMatchWordBoundaryBonus(t *testing.T) {
	t.Parallel()

	// "nt" in "new thread": 'n' at boundary (pos 0), 't' at boundary (pos 4 after space)
	// "nt" in "intent":    'n' at pos 2 (mid-word), 't' at pos 3 (continuation)
	scoreBoundary, _, ok1 := fuzzy.Match("nt", "new thread")
	scoreMid, _, ok2 := fuzzy.Match("nt", "intent")
	if !ok1 || !ok2 {
		t.Fatal("both must match")
	}
	if scoreBoundary <= scoreMid {
		t.Errorf("boundary match (%d) should beat mid-word match (%d)", scoreBoundary, scoreMid)
	}
}

func TestMatchContiguousRunBonus(t *testing.T) {
	t.Parallel()

	// "ab" at start of "abcde": both adjacent → continuation bonus on second rune.
	// "ab" scattered in "axbyz": gap between a and b → no continuation, gap penalty.
	scoreAdj, _, ok1 := fuzzy.Match("ab", "abcde")
	scoreGap, _, ok2 := fuzzy.Match("ab", "axbyz")
	if !ok1 || !ok2 {
		t.Fatal("both must match")
	}
	if scoreAdj <= scoreGap {
		t.Errorf("adjacent (%d) should beat gap (%d)", scoreAdj, scoreGap)
	}
}

func TestMatchUnicode(t *testing.T) {
	t.Parallel()

	// "café": runes c, a, f, é
	score, pos, ok := fuzzy.Match("café", "café au lait")
	if !ok {
		t.Fatal("unicode match must succeed")
	}
	if score <= 0 {
		t.Errorf("unicode score = %d, want >0", score)
	}
	if len(pos) != 4 {
		t.Errorf("positions = %v, want 4 entries", pos)
	}
}

func TestMatchUnicodeCaseInsensitive(t *testing.T) {
	t.Parallel()

	_, _, ok := fuzzy.Match("CAFÉ", "café au lait")
	if !ok {
		t.Error("unicode case-insensitive match must succeed")
	}
}

// --- MatchTerms ---

func TestMatchTermsAllTermsMustMatch(t *testing.T) {
	t.Parallel()

	_, _, ok := fuzzy.MatchTerms("hello world", "hello there")
	if ok {
		t.Error("AND semantics: both terms must match; 'world' not in haystack")
	}
}

func TestMatchTermsBothTermsMatch(t *testing.T) {
	t.Parallel()

	score, _, ok := fuzzy.MatchTerms("hello world", "hello world")
	if !ok {
		t.Fatal("both terms present, must match")
	}
	if score <= 0 {
		t.Errorf("score = %d, want >0", score)
	}
}

func TestMatchTermsPositionsUnionSorted(t *testing.T) {
	t.Parallel()

	// "ab cd" in "abcd": term "ab" → [0,1], term "cd" → [2,3], union sorted = [0,1,2,3]
	_, pos, ok := fuzzy.MatchTerms("ab cd", "abcd")
	if !ok {
		t.Fatal("must match")
	}
	want := []int{0, 1, 2, 3}
	if len(pos) != len(want) {
		t.Fatalf("positions = %v, want %v", pos, want)
	}
	for i, p := range pos {
		if p != want[i] {
			t.Errorf("positions[%d] = %d, want %d", i, p, want[i])
		}
	}
}

func TestMatchTermsEmptyNeedle(t *testing.T) {
	t.Parallel()

	score, pos, ok := fuzzy.MatchTerms("", "hello")
	if !ok {
		t.Fatal("empty needle must match")
	}
	if score != 0 {
		t.Errorf("score = %d, want 0", score)
	}
	if len(pos) != 0 {
		t.Errorf("positions = %v, want empty", pos)
	}
}

func TestMatchTermsScoreIsSum(t *testing.T) {
	t.Parallel()

	scoreA, _, _ := fuzzy.Match("hello", "hello world")
	scoreB, _, _ := fuzzy.Match("world", "hello world")
	scoreAB, _, ok := fuzzy.MatchTerms("hello world", "hello world")
	if !ok {
		t.Fatal("must match")
	}
	if scoreAB != scoreA+scoreB {
		t.Errorf("MatchTerms score %d != sum of term scores %d+%d=%d",
			scoreAB, scoreA, scoreB, scoreA+scoreB)
	}
}
