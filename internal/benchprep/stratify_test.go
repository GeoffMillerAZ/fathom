package benchprep

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"reflect"
	"testing"
	"testing/fstest"
)

func TestExtractIdentifiers(t *testing.T) {
	for _, tt := range []struct {
		name, issue string
		want        []string
	}{
		{"forms kept whole", "CacheEntry cache_entry pkg.Reader.Read src/cache.go", []string{"CacheEntry", "cache_entry", "pkg.Reader.Read", "src/cache.go"}},
		{"backticks", "the `lookup` and `cache key` fail", []string{"cache", "key", "lookup"}},
		{"length and stopwords", "`a` `id` `the` `func` `true` `nil` `Fix` `Read`", []string{"Read"}},
		{"deduplicate and sort", "`zebra` CacheEntry `zebra` CacheEntry", []string{"CacheEntry", "zebra"}},
		{"no prose", "the request fails when opening a file", []string{}},
		{"punctuation", "Call `Reader.Read()` or `cache_key`, then CacheEntry.", []string{"CacheEntry", "Reader.Read", "cache_key"}},
		{"unicode", "`读取值` `éé`", []string{"读取值"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractIdentifiers(tt.issue); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("identifiers = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStratifyTask(t *testing.T) {
	tree := fstest.MapFS{
		"a.go":      {Data: []byte("CacheEntry unrelated")},
		"b.go":      {Data: []byte("cacheentry CacheEntry")},
		"gold.go":   {Data: []byte("other identifier")},
		".git/leak": {Data: []byte("CacheEntry")},
		"binary":    {Data: []byte{0, 1, 2}},
		"link":      {Data: []byte("outside"), Mode: fs.ModeSymlink},
	}
	got, err := StratifyTask(tree, "`CacheEntry` fails", []string{"gold.go"})
	if err != nil {
		t.Fatal(err)
	}
	if got.DefinitionA != "non-lexical" || got.DefinitionB != "lexical" || got.LexicalRank != 3 {
		t.Fatalf("strata = %+v", got)
	}
	if len(got.Ranking) != 3 || got.Ranking[0].Path != "b.go" || got.Ranking[1].Path != "a.go" {
		t.Fatalf("ranking = %+v", got.Ranking)
	}
	// N=3, df=2, lengths all 2; k1=1.2, b=0.75, tf=2.
	wantScore := math.Log(1+(3-2+0.5)/(2+0.5)) * (2 * 2.2 / (2 + 1.2))
	if math.Abs(got.Ranking[0].Score-wantScore) > 1e-12 {
		t.Fatalf("BM25 score = %g, want %g", got.Ranking[0].Score, wantScore)
	}
	// Repeated issue identifiers do not reweight the query. Multiple gold
	// files use the earliest rank, and D-a checks every gold file.
	got, err = StratifyTask(tree, "`CacheEntry` `CacheEntry`", []string{"gold.go", "a.go"})
	if err != nil || got.DefinitionA != "lexical" || got.LexicalRank != 2 || math.Abs(got.Ranking[0].Score-wantScore) > 1e-12 {
		t.Fatalf("multi-gold = %+v, %v", got, err)
	}
}

func TestStratifyTaskTopTenAndTies(t *testing.T) {
	tree := fstest.MapFS{}
	for i := range 12 {
		tree[fmt.Sprintf("file%02d.go", i)] = &fstest.MapFile{Data: []byte("CacheEntry")}
	}
	for _, tt := range []struct {
		gold, want string
		rank       int
	}{
		{"file09.go", "lexical", 10},
		{"file10.go", "non-lexical", 11},
	} {
		t.Run(tt.gold, func(t *testing.T) {
			var prior Stratification
			for range 3 {
				got, err := StratifyTask(tree, "`CacheEntry`", []string{tt.gold})
				if err != nil || got.DefinitionB != tt.want || got.LexicalRank != tt.rank || got.DefinitionA != "lexical" {
					t.Fatalf("strata = %+v, %v", got, err)
				}
				if prior.LexicalRank != 0 && !reflect.DeepEqual(got, prior) {
					t.Fatal("ranking changed between identical reads")
				}
				prior = got
			}
		})
	}
}

func TestStratifyTaskLengthNormalizationAndWholeTokens(t *testing.T) {
	tree := fstest.MapFS{
		"a.go":    {Data: []byte("CacheEntry padding padding padding padding padding padding")},
		"b.go":    {Data: []byte("CacheEntry")},
		"gold.go": {Data: []byte("CacheEntrySuffix cache_entry_extra")},
	}
	got, err := StratifyTask(tree, "`CacheEntry` `cache_entry`", []string{"gold.go"})
	if err != nil || got.DefinitionA != "non-lexical" || got.Ranking[0].Path != "b.go" {
		t.Fatalf("normalization / whole tokens = %+v, %v", got, err)
	}
	// N=3, df=2, average length=10/3, b.go length=1, tf=1.
	want := math.Log(1+1.5/2.5) * 2.2 / (1 + 1.2*(0.25+0.75/(10.0/3)))
	if math.Abs(got.Ranking[0].Score-want) > 1e-12 {
		t.Fatalf("length normalization = %g, want %g", got.Ranking[0].Score, want)
	}
	got, err = StratifyTask(tree, "only ordinary prose", []string{"gold.go"})
	if err != nil || got.DefinitionA != "non-lexical" || got.LexicalRank != 3 {
		t.Fatalf("empty query uses path ties = %+v, %v", got, err)
	}
}

func TestStratifyTaskRefusesInvalidTruth(t *testing.T) {
	tree := fstest.MapFS{"gold.go": {Data: []byte("code")}, "binary": {Data: []byte{0}}}
	for _, gold := range [][]string{nil, {"missing.go"}, {"../gold.go"}, {"binary"}} {
		if _, err := StratifyTask(tree, "`code`", gold); err == nil {
			t.Fatalf("accepted gold %q", gold)
		}
	}
	if _, err := StratifyTask(failingFS{}, "`code`", []string{"gold.go"}); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("tree error = %v", err)
	}
}

type failingFS struct{}

func (failingFS) Open(string) (fs.File, error) { return nil, fs.ErrPermission }
