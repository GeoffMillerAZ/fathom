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

// TestRelevanceStrength_TracksStemAndPackageOnly pins the boundary
// crossTierCloser depends on: a hit at stemWeight/packageWeight (or above)
// is "strong"; a hit confined to ancestorWeight/extensionWeight is not,
// even though both contribute to the plain score.
func TestRelevanceStrength_TracksStemAndPackageOnly(t *testing.T) {
	stemOnly := []weightedToken{{tok: "goextract", weight: stemWeight}}
	packageOnly := []weightedToken{{tok: "extractor", weight: packageWeight}}
	ancestorOnly := []weightedToken{{tok: "internal", weight: ancestorWeight}}
	extOnly := []weightedToken{{tok: "go", weight: extensionWeight}}

	text := "## Some rule\n\ngoextract extractor internal go"
	if _, strong := relevanceStrength(text, stemOnly); !strong {
		t.Error("a stemWeight hit must be strong")
	}
	if _, strong := relevanceStrength(text, packageOnly); !strong {
		t.Error("a packageWeight hit must be strong")
	}
	if _, strong := relevanceStrength(text, ancestorOnly); strong {
		t.Error("an ancestorWeight-only hit must NOT be strong")
	}
	if _, strong := relevanceStrength(text, extOnly); strong {
		t.Error("an extensionWeight-only hit must NOT be strong")
	}
	if score, strong := relevanceStrength("no tokens present here", stemOnly); score != 0 || strong {
		t.Errorf("no hit at all: score=%v strong=%v, want 0/false", score, strong)
	}
	if score, strong := relevanceStrength(text, nil); score != 0 || strong {
		t.Errorf("nil tokens: score=%v strong=%v, want 0/false", score, strong)
	}
}

// TestRelevanceStrength_MatchesRelevanceScore: the refactor (relevanceScore
// now delegates to relevanceStrength) must never change the plain score.
func TestRelevanceStrength_MatchesRelevanceScore(t *testing.T) {
	tokens := []weightedToken{
		{tok: "extractor", weight: packageWeight},
		{tok: "internal", weight: ancestorWeight},
		{tok: "goextract", weight: stemWeight},
	}
	text := "## Adding an extractor\n\nRegister it in internal/extractor, then write goextract's golden test."
	score, _ := relevanceStrength(text, tokens)
	if got := relevanceScore(text, tokens); got != score {
		t.Errorf("relevanceScore = %v, relevanceStrength score = %v, want equal", got, score)
	}
}

// TestBehaviorTokens (wave O3, item 2) pins the small, deterministic
// behaviour -> convention-keyword table: a file whose own source writes
// output gets "workspace"; one that shells out to the test/build gate gets
// "gate"/"validate"; a file with neither signal (or no text at all) gets
// nothing.
func TestBehaviorTokens(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []string
	}{
		{"os.WriteFile", `os.WriteFile(path, data, 0o644)`, []string{"workspace"}},
		{"os.MkdirAll", `os.MkdirAll(dir, 0o755)`, []string{"workspace"}},
		{"go test via exec.Command", `exec.Command("go", "test", "./...")`, []string{"gate", "validate"}},
		{"shell via exec.Command", `exec.Command("sh", "-c", cmd)`, []string{"gate", "validate"}},
		{"both signals present", "os.Create(f)\n" + `exec.Command("go", "vet", "./...")`, []string{"workspace", "gate", "validate"}},
		{"neither signal", "package main\n\nfunc main() { fmt.Println(\"hi\") }", nil},
		{"empty text", "", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := BehaviorTokens(c.text)
			if len(got) != len(c.want) {
				t.Fatalf("BehaviorTokens(%q) = %v, want %v", c.text, got, c.want)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Errorf("BehaviorTokens(%q) = %v, want %v", c.text, got, c.want)
					break
				}
			}
		})
	}
}

func TestBehaviorTokens_Deduplicates(t *testing.T) {
	text := `os.WriteFile(a, b, 0o644); os.Create(c); os.MkdirAll(d, 0o755)`
	got := BehaviorTokens(text)
	if len(got) != 1 || got[0] != "workspace" {
		t.Errorf("BehaviorTokens with three matching signals from the same row = %v, want exactly [\"workspace\"] once", got)
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
