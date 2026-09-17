package semantic

import "testing"

func TestPathRelevanceTokens(t *testing.T) {
	toks := pathRelevanceTokens("internal/extractor/goextract.go")
	has := func(tok string, weight float64) {
		t.Helper()
		for _, w := range toks {
			if w.tok == tok {
				if w.weight != weight {
					t.Errorf("token %q weight = %v, want %v", tok, w.weight, weight)
				}
				return
			}
		}
		t.Errorf("token %q not found in %+v", tok, toks)
	}
	has("goextract", stemWeight)
	has("extractor", packageWeight) // the file's own immediate directory
	has("internal", ancestorWeight) // a shallower ancestor
	has("go", extensionWeight)      // bare extension
}

func TestPathRelevanceTokens_NoDirectory(t *testing.T) {
	toks := pathRelevanceTokens("emit.go")
	if len(toks) == 0 {
		t.Fatal("expected at least the stem and extension tokens")
	}
	for _, w := range toks {
		if w.tok == "emit" && w.weight != stemWeight {
			t.Errorf("stem weight = %v, want %v", w.weight, stemWeight)
		}
	}
}

func TestSplitIdent(t *testing.T) {
	cases := map[string][]string{
		"goExtract":  {"go", "extract"},
		"go_extract": {"go", "extract"},
		"goextract":  nil, // no boundary at all — caller keeps the whole token separately
		"CamelCase":  {"camel", "case"},
	}
	for in, want := range cases {
		got := splitIdent(in)
		if len(got) != len(want) {
			t.Errorf("splitIdent(%q) = %v, want %v", in, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("splitIdent(%q) = %v, want %v", in, got, want)
				break
			}
		}
	}
}

func TestCountWordHits_WordBoundary(t *testing.T) {
	cases := []struct {
		text, tok string
		want      int
	}{
		{"pure go and go test", "go", 2},
		{"cgo and golden and argo", "go", 0}, // "go" never stands alone here
		{"internal/store is internal", "internal", 2},
		{"extractor.extractor bindings", "extractor", 2},
	}
	for _, c := range cases {
		if got := countWordHits(c.text, c.tok); got != c.want {
			t.Errorf("countWordHits(%q, %q) = %d, want %d", c.text, c.tok, got, c.want)
		}
	}
}

func TestRelevanceScore_HeadingWeighsMore(t *testing.T) {
	tokens := []weightedToken{{tok: "extractor", weight: packageWeight}}
	headingHit := "## Adding an extractor\n\nUnrelated body text."
	bodyHit := "## Unrelated heading\n\nSomething about an extractor here."
	if s1, s2 := relevanceScore(headingHit, tokens), relevanceScore(bodyHit, tokens); s1 <= s2 {
		t.Errorf("heading hit score %v should exceed body hit score %v", s1, s2)
	}
}

func TestRelevanceScore_CappedRepetitionNeverDominatesDistinctTokens(t *testing.T) {
	tokens := []weightedToken{
		{tok: "extractor", weight: packageWeight},
		{tok: "internal", weight: ancestorWeight},
		{tok: "goextract", weight: stemWeight},
	}
	repeated := "## Repeats\n\n" +
		"internal internal internal internal internal internal internal internal internal internal"
	distinct := "## Distinct\n\nextractor goextract internal"
	if s1, s2 := relevanceScore(repeated, tokens), relevanceScore(distinct, tokens); s2 <= s1 {
		t.Errorf("three distinct on-topic tokens (%v) should outscore one repeated common token (%v)", s2, s1)
	}
}

func TestRelevanceScore_EmptyInputsScoreZero(t *testing.T) {
	if s := relevanceScore("", []weightedToken{{tok: "x", weight: 1}}); s != 0 {
		t.Errorf("empty text scored %v, want 0", s)
	}
	if s := relevanceScore("some text", nil); s != 0 {
		t.Errorf("no tokens scored %v, want 0", s)
	}
}

func TestSymbolRelevanceTokens_DedupesAndWeighsAtStem(t *testing.T) {
	toks := symbolRelevanceTokens([]string{"Edge", "edge", "EdgeKind", ""})
	if len(toks) != 2 {
		t.Fatalf("got %d tokens, want 2 (Edge/edge dedupe, EdgeKind separate, empty dropped): %+v", len(toks), toks)
	}
	for _, w := range toks {
		if w.weight != stemWeight {
			t.Errorf("symbol token %q weight = %v, want stemWeight", w.tok, w.weight)
		}
	}
}
