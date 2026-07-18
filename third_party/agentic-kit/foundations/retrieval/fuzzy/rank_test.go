package fuzzy_test

import (
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/retrieval/fuzzy"
)

// --- Rank ---

func TestRankEmptyNeedleReturnsAll(t *testing.T) {
	t.Parallel()

	candidates := []string{"alpha", "beta", "gamma"}
	results := fuzzy.Rank("", candidates)
	if len(results) != 3 {
		t.Fatalf("expected all 3 candidates, got %d", len(results))
	}
	for i, r := range results {
		if r.Score != 0 {
			t.Errorf("results[%d].Score = %d, want 0", i, r.Score)
		}
		if r.Index != i {
			t.Errorf("results[%d].Index = %d, want %d", i, r.Index, i)
		}
	}
}

func TestRankWhitespaceNeedleReturnsAll(t *testing.T) {
	t.Parallel()

	results := fuzzy.Rank("   ", []string{"foo", "bar"})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestRankNoMatchExcluded(t *testing.T) {
	t.Parallel()

	results := fuzzy.Rank("zzz", []string{"alpha", "beta", "gamma"})
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d: %v", len(results), results)
	}
}

func TestRankSortedByScoreDesc(t *testing.T) {
	t.Parallel()

	// "new thread" should score higher than "intent" for needle "nt"
	// because boundary hits outweigh mid-word hits.
	candidates := []string{"intent", "new thread", "nothing"}
	results := fuzzy.Rank("nt", candidates)

	// Find "new thread" and "intent" in results.
	var newThreadScore, intentScore int
	newThreadFound, intentFound := false, false
	for _, r := range results {
		switch r.Candidate {
		case "new thread":
			newThreadScore = r.Score
			newThreadFound = true
		case "intent":
			intentScore = r.Score
			intentFound = true
		}
	}
	if !newThreadFound || !intentFound {
		t.Fatalf("both candidates must match; results: %v", results)
	}
	if newThreadScore <= intentScore {
		t.Errorf("'new thread' score %d should exceed 'intent' score %d (boundary bonus)", newThreadScore, intentScore)
	}
	// Results must be in descending score order.
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted desc at index %d: %d > %d", i, results[i].Score, results[i-1].Score)
		}
	}
}

func TestRankTieBreakByIndex(t *testing.T) {
	t.Parallel()

	// "ab" is a subsequence of all three; tie → original index order.
	candidates := []string{"ab", "ab", "ab"}
	results := fuzzy.Rank("ab", candidates)
	if len(results) != 3 {
		t.Fatalf("expected 3, got %d", len(results))
	}
	if results[0].Index != 0 || results[1].Index != 1 || results[2].Index != 2 {
		t.Errorf("tie-break order wrong: got indices %d %d %d", results[0].Index, results[1].Index, results[2].Index)
	}
}

func TestRankDeterministic(t *testing.T) {
	t.Parallel()

	candidates := []string{"foo bar", "bar foo", "foobar", "baz", "fo", "bar"}
	needle := "fo"

	var first []fuzzy.Result
	for run := 0; run < 50; run++ {
		res := fuzzy.Rank(needle, candidates)
		if first == nil {
			first = res
			continue
		}
		if len(res) != len(first) {
			t.Fatalf("run %d: length mismatch %d vs %d", run, len(res), len(first))
		}
		for i := range res {
			if res[i].Index != first[i].Index || res[i].Score != first[i].Score {
				t.Fatalf("run %d: non-deterministic at index %d", run, i)
			}
		}
	}
}

func TestRankIndexPreserved(t *testing.T) {
	t.Parallel()

	candidates := []string{"alpha", "match me", "beta", "match also", "gamma"}
	results := fuzzy.Rank("match", candidates)
	// Only index 1 and 3 contain "match".
	if len(results) != 2 {
		t.Fatalf("expected 2, got %d: %v", len(results), results)
	}
	indexSet := map[int]bool{results[0].Index: true, results[1].Index: true}
	if !indexSet[1] || !indexSet[3] {
		t.Errorf("expected indices 1 and 3, got %v", results)
	}
}

func TestRankMultiTermAND(t *testing.T) {
	t.Parallel()

	candidates := []string{
		"hello world",  // both terms match
		"hello there",  // only "hello" matches
		"world class",  // only "world" matches
		"hello world!", // both terms match (extra punct)
	}
	results := fuzzy.Rank("hello world", candidates)
	// Only indices 0 and 3 satisfy AND.
	if len(results) != 2 {
		t.Fatalf("expected 2, got %d: %v", len(results), results)
	}
	for _, r := range results {
		if r.Index != 0 && r.Index != 3 {
			t.Errorf("unexpected candidate index %d: %q", r.Index, r.Candidate)
		}
	}
}

func TestRankPositionsNonNilOnMatch(t *testing.T) {
	t.Parallel()

	results := fuzzy.Rank("hi", []string{"hi there"})
	if len(results) != 1 {
		t.Fatal("expected 1 result")
	}
	if results[0].Positions == nil {
		t.Error("Positions must not be nil on a match")
	}
}

func TestRankUnicode(t *testing.T) {
	t.Parallel()

	candidates := []string{"café au lait", "coffee shop", "café noir"}
	results := fuzzy.Rank("café", candidates)
	if len(results) != 2 {
		t.Fatalf("expected 2 unicode matches, got %d: %v", len(results), results)
	}
}
