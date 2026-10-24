package extractor

import (
	"reflect"
	"strings"
	"testing"
)

// TestSplitTSCallSites (wave lf5): calls.split "whitespace" names each token
// of a set of IDs at the site's range, splitting on HTML's ASCII whitespace
// only; a blank site names nothing.
func TestSplitTSCallSites(t *testing.T) {
	got := splitTSCallSites([]tsCallSite{
		{text: "a  b\tc\nd\fe\rf", start: 1, end: 9},
		{text: "solo", start: 10, end: 14},
		{text: " \t ", start: 15, end: 18},
		{text: "x y", start: 19, end: 22}, // NO-BREAK SPACE is no ASCII whitespace
	})
	want := []tsCallSite{
		{text: "a", start: 1, end: 9}, {text: "b", start: 1, end: 9}, {text: "c", start: 1, end: 9},
		{text: "d", start: 1, end: 9}, {text: "e", start: 1, end: 9}, {text: "f", start: 1, end: 9},
		{text: "solo", start: 10, end: 14},
		{text: "x y", start: 19, end: 22},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitTSCallSites\n got %+v\nwant %+v", got, want)
	}
}

// TestCheckCallsRules (wave lf5): owner "module" needs the module symbol that
// owns the calls; owner and split take only their one value.
func TestCheckCallsRules(t *testing.T) {
	ok := Dialect{ModuleSymbol: "document", Calls: &DialectCalls{Owner: "module", Split: "whitespace"}}
	if err := checkCalls(ok); err != nil {
		t.Fatalf("a module owner with module_symbol must load: %v", err)
	}
	scope := Dialect{ModuleSymbol: "module", Calls: &DialectCalls{Owner: "scope"}}
	if err := checkCalls(scope); err != nil {
		t.Fatalf("a scope owner with module_symbol must load: %v", err)
	}
	if err := checkCalls(Dialect{}); err != nil {
		t.Fatalf("no calls block: %v", err)
	}
	for name, tc := range map[string]struct {
		d    Dialect
		want string
	}{
		"owner without module_symbol": {Dialect{Calls: &DialectCalls{Owner: "module"}}, "needs module_symbol"},
		"scope without module_symbol": {Dialect{Calls: &DialectCalls{Owner: "scope"}}, "needs module_symbol"},
		"owner off the set":           {Dialect{ModuleSymbol: "document", Calls: &DialectCalls{Owner: "file"}}, "is not \"module\" or \"scope\""},
		"split off the set":           {Dialect{Calls: &DialectCalls{Split: "comma"}}, "is not \"whitespace\""},
	} {
		err := checkCalls(tc.d)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want one naming %q", name, err, tc.want)
		}
	}
}

// TestCallsOwnerOnlyWhereTheManifestNamesIt (wave lf5): the html dialect
// names calls.owner and calls.split, scss calls.owner "scope" (LB-Q-scss-rest: a reference is its
// enclosing @mixin or @function's, else the module's), and astro calls.owner (wave lf7, LB-F544:
// a component tag is its template's module symbol's call), so every other
// dialect's calls are attached as before.
func TestCallsOwnerOnlyWhereTheManifestNamesIt(t *testing.T) {
	ds, err := LoadDialects()
	if err != nil {
		t.Fatal(err)
	}
	var named []string
	for _, d := range ds {
		if d.Calls != nil && (d.Calls.Owner != "" || d.Calls.Split != "") {
			named = append(named, d.ID)
		}
	}
	if !reflect.DeepEqual(named, []string{"astro", "html", "jinja2", "scss"}) {
		t.Fatalf("dialects naming calls.owner or calls.split = %q, want [astro html jinja2 scss]", named)
	}
}
