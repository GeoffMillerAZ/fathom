package langbench

import (
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestCSSCtagsRendition (K19, LB-F423): the tags universal-ctags 6.2.1's
// CSS parser (parsers/css.c, v6.2.1 9463f1c6) writes for one selector: its
// tokens joined by one space, an attribute selector glued to the token before
// it, a leading attribute selector's brackets dropped, and a tag ended by any
// comma, including one inside a functional pseudo-class's selector list. The
// line of each tag is its last selector token's, as an offset in the text.
func TestCSSCtagsRendition(t *testing.T) {
	for _, tc := range []struct {
		sel  string
		want []ctagsTag
	}{
		{".a", []ctagsTag{{".a", 0}}},
		{"[hidden]", []ctagsTag{{"hidden", 0}}},
		{"audio:not([controls])", []ctagsTag{{"audio:not([controls] )", 0}}},
		{`input[type="search"]::-webkit-search-cancel-button`, []ctagsTag{{`input[type="search"] ::-webkit-search-cancel-button`, 0}}},
		{"input:not([type=submit],\n[type=button])", []ctagsTag{{"input:not([type=submit]", 0}, {"type", 1}, {"button", 1}, {")", 1}}},
		{":where(select, textarea)", []ctagsTag{{":where(select", 0}, {"textarea)", 0}}},
		{"details\n  summary", []ctagsTag{{"details summary", 1}}},
		{"a > b ~ c + d", []ctagsTag{{"a > b ~ c + d", 0}}},
		{"a>b", []ctagsTag{{"a>b", 0}}},
		{`[title="a,b"]`, []ctagsTag{{"title", 0}}},
		{`a[title="a, b"]`, []ctagsTag{{`a[title="a, b"]`, 0}}},
		{"a[ href = 'x' ]", []ctagsTag{{"a[href='x']", 0}}},
		{"&:hover", []ctagsTag{{":hover", 0}}},
		{"/* note */ .x", []ctagsTag{{".x", 0}}},
		{`.sm\:flex`, []ctagsTag{{".sm", 0}, {":flex", 0}}},
		{".café", []ctagsTag{{".caf", 0}}},
		{"[a][b=c]::before", []ctagsTag{{"a", 0}, {"b", 0}, {"c", 0}, {"::before", 0}}},
		{"", nil},
	} {
		if got := cssCtagsRendition(tc.sel); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("cssCtagsRendition(%q) = %v, want %v", tc.sel, got, tc.want)
		}
	}
}

// TestScoreL1CSSCtagsRendition (K19, LB-F423): against universal-ctags, a
// CSS selector is compared in the tags ctags writes for it. The first tag is
// the selector's match key, at the line of its last token; the tags ctags
// writes for the rest of a selector split inside a functional pseudo-class
// are the same definition, consumed when the first tag matches and left out
// of the oracle count (tallied as ctags:selector-fragment). Another lang tag
// or another oracle keeps the identity comparison.
func TestScoreL1CSSCtagsRendition(t *testing.T) {
	const src = `{"id":"css","kind":"language","lang_tag":%q,"name_match":"identity",
		"kinds":{"selector":{"source_kind":"CSSSelector","canonical":"selector"}},
		"required_kinds":["selector"],"oracle_kinds":{"ctags":{"class":"selector","id":"selector","selector":"selector"},"tool":{"selector":"selector"}}}`
	nodes := []core.SymbolNode{
		{Name: "audio:not([controls])", Kind: "CSSSelector", FilePath: "x.css", Line: 41},
		{Name: "[hidden]", Kind: "CSSSelector", FilePath: "x.css", Line: 51},
		{Name: "input:not([type=submit],\n[type=button],\n[type=reset])", Kind: "CSSSelector", FilePath: "x.css", Line: 155},
		{Name: ".a\n.b\n.c\n.d", Kind: "CSSSelector", FilePath: "x.css", Line: 200}, // one descendant selector over four lines
		{Name: ".gone", Kind: "CSSSelector", FilePath: "x.css", Line: 300},
	}
	oracle := []OracleDef{
		{Path: "x.css", Name: "audio:not([controls] )", Kind: "selector", Line: 41},
		{Path: "x.css", Name: "hidden", Kind: "selector", Line: 51},
		{Path: "x.css", Name: "input:not([type=submit]", Kind: "selector", Line: 155},
		{Path: "x.css", Name: "type", Kind: "selector", Line: 156},
		{Path: "x.css", Name: "button", Kind: "selector", Line: 156},
		{Path: "x.css", Name: "type", Kind: "selector", Line: 157},
		{Path: "x.css", Name: "reset", Kind: "selector", Line: 157},
		{Path: "x.css", Name: ")", Kind: "selector", Line: 157},
		{Path: "x.css", Name: ".a .b .c .d", Kind: "class", Line: 203},
		{Path: "x.css", Name: ".other", Kind: "class", Line: 400},
	}
	m, err := ParseManifest([]byte(strings.Replace(src, "%q", `"css"`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	r := ScoreL1(m, "ctags", nodes, oracle)
	want := map[string]int{CntFathomDefs: 5, CntOracleDefs: 5, CntMatched: 4, CntKindEqual: 4}
	for k, v := range want {
		if r.Counts[k] != v {
			t.Errorf("count %s = %d, want %d (all: %v)", k, r.Counts[k], v, r.Counts)
		}
	}
	if got := r.Excluded["ctags:selector-fragment"]; got != 5 {
		t.Errorf("excluded ctags:selector-fragment = %d, want 5 (%v)", got, r.Excluded)
	}
	if len(r.DialectOnly) != 1 || r.DialectOnly[0].Name != ".gone" || r.DialectOnly[0].Line != 300 {
		t.Errorf("dialect only = %+v, want the selector as written at its own line", r.DialectOnly)
	}
	if len(r.OracleOnly) != 1 || r.OracleOnly[0].Name != ".other" {
		t.Errorf("oracle only = %+v, want .other", r.OracleOnly)
	}

	// The rule is ctags' CSS parser's: another language's ctags tags, or another
	// oracle's CSS definitions, are compared whole as before.
	other, err := ParseManifest([]byte(strings.Replace(src, "%q", `"html"`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	if got := ScoreL1(other, "ctags", nodes, oracle).Counts[CntMatched]; got != 0 {
		t.Errorf("lang_tag html: matched %d, want 0 (identity)", got)
	}
	var tool []OracleDef
	for _, n := range nodes {
		tool = append(tool, OracleDef{Path: n.FilePath, Name: n.Name, Kind: "selector", Line: n.Line})
	}
	if got := ScoreL1(m, "tool", nodes, tool); got.Counts[CntMatched] != 5 || got.Excluded["tool:selector-fragment"] != 0 {
		t.Errorf("tool oracle: matched %d, excluded %v; want 5 whole names and no fragment", got.Counts[CntMatched], got.Excluded)
	}
}

// TestScoreL1CSSFragmentNeedsMatch (K19): a selector whose first tag finds no
// oracle tag consumes nothing; the oracle's tags near it stay in the count.
func TestScoreL1CSSFragmentNeedsMatch(t *testing.T) {
	m, err := ParseManifest([]byte(`{"id":"css","kind":"language","lang_tag":"css","name_match":"identity",
		"kinds":{"selector":{"source_kind":"CSSSelector","canonical":"selector"}},
		"required_kinds":["selector"],"oracle_kinds":{"ctags":{"selector":"selector"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{{Name: ":is(a, b)", Kind: "CSSSelector", FilePath: "y.css", Line: 10}}
	oracle := []OracleDef{{Path: "y.css", Name: "b)", Kind: "selector", Line: 10}}
	r := ScoreL1(m, "ctags", nodes, oracle)
	if r.Counts[CntMatched] != 0 || r.Counts[CntOracleDefs] != 1 || r.Excluded["ctags:selector-fragment"] != 0 {
		t.Errorf("counts %v excluded %v: want no match, the oracle tag kept", r.Counts, r.Excluded)
	}
}
