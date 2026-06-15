package platformlink_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/platformlink"
)

// TestBuildFromSymbols_MultiLanguage proves cross-repo edges are detected for
// NON-Go languages. The data source is reconstructed SymbolNodes (exactly as
// they come back from the store), and matching is purely by Package/Name/Calls
// — no Go AST, no exported-name capitalization rule. A Go-only path would find
// zero of these edges.
func TestBuildFromSymbols_MultiLanguage(t *testing.T) {
	nodesByService := map[string][]core.SymbolNode{
		// Python "lib" defines helpers.validate — snake_case, which the old Go
		// capitalization gate would have wrongly excluded from the index.
		"py-lib": {
			{Service: "py-lib", Package: "helpers", Name: "validate", FilePath: "helpers.py", Tier: core.TierVerified},
		},
		// Python "api" calls helpers.validate.
		"py-api": {
			{Service: "py-api", Package: "app", Name: "handler", FilePath: "app.py", Calls: []string{"helpers.validate"}, Tier: core.TierVerified},
		},
		// JS pair: utils.format defined in js-ui, called from js-web.
		"js-ui": {
			{Service: "js-ui", Package: "utils", Name: "format", FilePath: "utils.js", Tier: core.TierVerified},
		},
		"js-web": {
			{Service: "js-web", Package: "page", Name: "render", FilePath: "page.js", Calls: []string{"utils.format"}, Tier: core.TierVerified},
		},
	}

	g := platformlink.BuildFromSymbols(nodesByService)

	want := []core.CrossRepoEdge{
		{FromService: "js-web", FromFile: "page.js", ToService: "js-ui", ToKey: "format", ToFile: "utils.js"},
		{FromService: "py-api", FromFile: "app.py", ToService: "py-lib", ToKey: "validate", ToFile: "helpers.py"},
	}
	if got := g.Edges(); !reflect.DeepEqual(got, want) {
		t.Fatalf("multi-language edges mismatch:\n got %#v\nwant %#v", got, want)
	}

	// Blast radius and callers work for the non-Go services.
	if br := g.BlastRadius("py-lib"); !reflect.DeepEqual(br, []string{"py-api"}) {
		t.Fatalf("BlastRadius(py-lib) = %v, want [py-api]", br)
	}
	if c := g.Callers("validate"); len(c) != 1 || c[0].FromService != "py-api" {
		t.Fatalf("Callers(validate) = %v, want one edge from py-api", c)
	}
}

// TestBuildFromSymbols_NoIntraServiceEdges ensures a call resolved within the
// same service never produces a cross-repo edge.
func TestBuildFromSymbols_NoIntraServiceEdges(t *testing.T) {
	nodes := map[string][]core.SymbolNode{
		"svc": {
			{Service: "svc", Package: "a", Name: "Foo", FilePath: "a.go"},
			{Service: "svc", Package: "b", Name: "Bar", FilePath: "b.go", Calls: []string{"a.Foo"}},
		},
	}
	if g := platformlink.BuildFromSymbols(nodes); len(g.Edges()) != 0 {
		t.Fatalf("expected no intra-service edges, got %v", g.Edges())
	}
}

// TestBuildFromSymbols_BareCalleesNotLinked verifies the conservative policy:
// an unqualified callee ("validate", no package) is NOT cross-linked, because
// bare names collide fleet-wide.
func TestBuildFromSymbols_BareCalleesNotLinked(t *testing.T) {
	nodes := map[string][]core.SymbolNode{
		"def":    {{Service: "def", Package: "helpers", Name: "validate", FilePath: "h.py"}},
		"caller": {{Service: "caller", Package: "app", Name: "run", FilePath: "a.py", Calls: []string{"validate"}}},
	}
	if g := platformlink.BuildFromSymbols(nodes); len(g.Edges()) != 0 {
		t.Fatalf("bare callee should not be cross-linked, got %v", g.Edges())
	}
}

// TestBuildFromSymbols_FanOut: a qualified call matching symbols in multiple
// services fans out to all of them — the true blast radius.
func TestBuildFromSymbols_FanOut(t *testing.T) {
	nodes := map[string][]core.SymbolNode{
		"def1":   {{Service: "def1", Package: "shared", Name: "Helper", FilePath: "h1.go"}},
		"def2":   {{Service: "def2", Package: "shared", Name: "Helper", FilePath: "h2.go"}},
		"caller": {{Service: "caller", Package: "c", Name: "Use", FilePath: "c.go", Calls: []string{"shared.Helper"}}},
	}
	g := platformlink.BuildFromSymbols(nodes)
	if got := len(g.Edges()); got != 2 {
		t.Fatalf("expected 2 fan-out edges, got %d: %v", got, g.Edges())
	}
	if br := g.BlastRadius("def1"); !reflect.DeepEqual(br, []string{"caller"}) {
		t.Fatalf("BlastRadius(def1) = %v, want [caller]", br)
	}
}

// TestBuildFromSymbols_Determinism asserts identical input → identical output.
func TestBuildFromSymbols_Determinism(t *testing.T) {
	nodes := map[string][]core.SymbolNode{
		"a": {{Service: "a", Package: "p", Name: "X", FilePath: "x.go"}},
		"b": {{Service: "b", Package: "q", Name: "Y", FilePath: "y.go", Calls: []string{"p.X"}}},
	}
	g1 := platformlink.BuildFromSymbols(nodes)
	g2 := platformlink.BuildFromSymbols(nodes)
	if !reflect.DeepEqual(g1.Edges(), g2.Edges()) {
		t.Fatalf("non-deterministic: %v vs %v", g1.Edges(), g2.Edges())
	}
}

// TestBuildFromSymbols_Empty returns an empty graph, never nil-panics.
func TestBuildFromSymbols_Empty(t *testing.T) {
	if g := platformlink.BuildFromSymbols(nil); g == nil || len(g.Edges()) != 0 {
		t.Fatalf("empty input should yield an empty graph, got %v", g)
	}
}
