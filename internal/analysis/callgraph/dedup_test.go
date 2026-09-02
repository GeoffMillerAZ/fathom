package callgraph_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// dedupFixture models an index polluted by stale worktree copies: the same
// symbol observed multiple times with an IDENTICAL SymbolKey. Build dedupes
// g.nodes by key, so the ambiguity counts must be computed over the
// deduplicated key set — otherwise a symbol that is genuinely unique looks
// ambiguous (count 3 > 1), drops out of byName/byPkgName, and the
// cross-package "pkg.Name" callee falls to suffix matching where "."+raw can
// never match across the "::" boundary of "svc::pkg.Name". The dropped edge
// class is exactly the external "_test"-package -> package-under-test edge
// that TestImpact needs.
//
//	widget.Frobnicate   — duplicated 3x (identical keys; snapshot-covered)
//	widget.Helper       — observed once (post-snapshot package; must resolve
//	                      regardless, guarding against "every symbol drops")
//	widget_test.TestFrobnicate -> "widget.Frobnicate"  (cross-package selector)
//	widget_test.TestHelper     -> "widget.Helper"
func dedupFixture() []core.SymbolNode {
	frob := core.SymbolNode{
		Service: "svc", Package: "widget", Name: "Frobnicate",
		Kind: core.KindGoFunc, FilePath: "widget/widget.go",
	}
	return []core.SymbolNode{
		frob,
		frob, // duplicate observation (stale worktree copy)
		frob, // duplicate observation (stale worktree copy)
		{
			Service: "svc", Package: "widget", Name: "Helper",
			Kind: core.KindGoFunc, FilePath: "widget/helper.go",
		},
		{
			Service: "svc", Package: "widget_test", Name: "TestFrobnicate",
			Kind: core.KindGoFunc, FilePath: "widget/widget_test.go",
			Calls: []string{"widget.Frobnicate"},
		},
		{
			Service: "svc", Package: "widget_test", Name: "TestHelper",
			Kind: core.KindGoFunc, FilePath: "widget/helper_test.go",
			Calls: []string{"widget.Helper"},
		},
	}
}

// TestBuild_DuplicateObservationsDoNotPoisonResolution pins the dedup bug:
// duplicate observations of the SAME key must not make the symbol ambiguous.
func TestBuild_DuplicateObservationsDoNotPoisonResolution(t *testing.T) {
	g := callgraph.Build(dedupFixture())

	cases := []struct {
		name      string
		symbol    string
		wantTests []string
		test      string // test node key, for Reachable/CallPath
	}{
		{
			name:      "triplicated symbol still resolves",
			symbol:    "svc::widget/widget.Frobnicate",
			wantTests: []string{"svc::widget/widget_test.TestFrobnicate"},
			test:      "svc::widget/widget_test.TestFrobnicate",
		},
		{
			name:      "non-duplicated symbol resolves alongside duplicates",
			symbol:    "svc::widget/widget.Helper",
			wantTests: []string{"svc::widget/widget_test.TestHelper"},
			test:      "svc::widget/widget_test.TestHelper",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := callgraph.TestImpact(g, tc.symbol); !reflect.DeepEqual(got, tc.wantTests) {
				t.Errorf("TestImpact(%s) = %v, want %v", tc.symbol, got, tc.wantTests)
			}
			if !callgraph.Reachable(g, tc.test, tc.symbol) {
				t.Errorf("Reachable(%s, %s) = false, want true", tc.test, tc.symbol)
			}
			if got := callgraph.RenameImpact(g, tc.symbol); !reflect.DeepEqual(got, tc.wantTests) {
				t.Errorf("RenameImpact(%s) = %v, want %v", tc.symbol, got, tc.wantTests)
			}
			wantPath := []string{tc.test, tc.symbol}
			if got := callgraph.CallPath(g, tc.test, tc.symbol); !reflect.DeepEqual(got, wantPath) {
				t.Errorf("CallPath(%s, %s) = %v, want %v", tc.test, tc.symbol, got, wantPath)
			}
		})
	}
}

// TestBuild_DuplicateObservationsBareNameLookup: after dedup, the bare name of
// a triplicated-but-unique symbol must be usable for tolerant lookup too.
func TestBuild_DuplicateObservationsBareNameLookup(t *testing.T) {
	g := callgraph.Build(dedupFixture())
	want := []string{"svc::widget/widget_test.TestFrobnicate"}
	if got := callgraph.TestImpact(g, "Frobnicate"); !reflect.DeepEqual(got, want) {
		t.Errorf("TestImpact(bare Frobnicate) = %v, want %v", got, want)
	}
}

// TestBuild_DuplicateObservationsNoDuplicateEdges: dedup must also apply to
// edges — three identical caller observations must not triple the edge list.
func TestBuild_DuplicateObservationsNoDuplicateEdges(t *testing.T) {
	caller := core.SymbolNode{
		Service: "svc", Package: "pkg", Name: "Caller",
		Kind: core.KindGoFunc, Calls: []string{"svc::pkg.Leaf"},
	}
	nodes := []core.SymbolNode{
		caller, caller, caller,
		{Service: "svc", Package: "pkg", Name: "Leaf", Kind: core.KindGoFunc},
	}
	g := callgraph.Build(nodes)
	want := []string{"svc::pkg.Caller"}
	if got := callgraph.RenameImpact(g, "svc::pkg.Leaf"); !reflect.DeepEqual(got, want) {
		t.Errorf("RenameImpact(Leaf) = %v, want %v (edges must be deduplicated)", got, want)
	}
	if got := g.EdgeCount(); got != 1 {
		t.Errorf("EdgeCount() = %d, want 1", got)
	}
}

// TestBuild_GenuineAmbiguityStaysAmbiguous: two DIFFERENT symbols sharing a
// bare name (distinct keys) must keep resolving to nothing — dedup must not
// turn honest ambiguity into first-match-wins.
func TestBuild_GenuineAmbiguityStaysAmbiguous(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "alpha", Name: "New", Kind: core.KindGoFunc},
		{Service: "svc", Package: "beta", Name: "New", Kind: core.KindGoFunc},
		{
			Service: "svc", Package: "gamma", Name: "TestNew",
			Kind: core.KindGoFunc, Calls: []string{"New"},
		},
	}
	g := callgraph.Build(nodes)
	if got := callgraph.TestImpact(g, "New"); len(got) != 0 {
		t.Errorf("TestImpact(ambiguous New) = %v, want []", got)
	}
	if g.EdgeCount() != 0 {
		t.Errorf("EdgeCount() = %d, want 0 (ambiguous bare callee must not resolve)", g.EdgeCount())
	}
}
