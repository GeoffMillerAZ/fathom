package packagedeps_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/packagedeps"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// fixture symbols
var symbols = []core.SymbolNode{
	// auth calls storage and crypto
	{
		Package: "auth", Name: "Login", Kind: core.KindGoFunc,
		Calls: []string{"storage.Get", "crypto.Hash"},
	},
	{
		Package: "auth", Name: "Logout", Kind: core.KindGoFunc,
		Calls: []string{"storage.Delete"},
	},
	// storage has no outgoing deps
	{Package: "storage", Name: "Get", Kind: core.KindGoFunc},
	{Package: "storage", Name: "Delete", Kind: core.KindGoFunc},
	// crypto has no outgoing deps
	{Package: "crypto", Name: "Hash", Kind: core.KindGoFunc},
	// api calls auth (creates auth→storage, auth→crypto edges but api→auth edge too)
	{
		Package: "api", Name: "Handle", Kind: core.KindGoFunc,
		Calls: []string{"auth.Login"},
	},
}

func TestBuild_Deps(t *testing.T) {
	g := packagedeps.Build(symbols)

	cases := []struct {
		pkg  string
		want []string
	}{
		{"auth", []string{"crypto", "storage"}},
		{"api", []string{"auth"}},
		{"storage", []string{}},
		{"crypto", []string{}},
	}
	for _, tc := range cases {
		got := packagedeps.Deps(g, tc.pkg)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Deps(%q) = %v; want %v", tc.pkg, got, tc.want)
		}
	}
}

func TestBuild_UnknownPkg(t *testing.T) {
	g := packagedeps.Build(symbols)
	if got := packagedeps.Deps(g, "nonexistent"); got != nil {
		t.Errorf("Deps for unknown pkg should be nil, got %v", got)
	}
}

func TestCycles_NoCycle(t *testing.T) {
	g := packagedeps.Build(symbols)
	got := packagedeps.Cycles(g)
	if len(got) != 0 {
		t.Errorf("expected no cycles, got %v", got)
	}
}

func TestCycles_WithCycle(t *testing.T) {
	// a → b → c → a
	cycleSymbols := []core.SymbolNode{
		{Package: "a", Name: "F", Kind: core.KindGoFunc, Calls: []string{"b.G"}},
		{Package: "b", Name: "G", Kind: core.KindGoFunc, Calls: []string{"c.H"}},
		{Package: "c", Name: "H", Kind: core.KindGoFunc, Calls: []string{"a.F"}},
	}
	g := packagedeps.Build(cycleSymbols)
	cycles := packagedeps.Cycles(g)
	if len(cycles) != 1 {
		t.Fatalf("expected 1 cycle, got %d: %v", len(cycles), cycles)
	}
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(cycles[0], want) {
		t.Errorf("cycle = %v; want %v", cycles[0], want)
	}
}

func TestCycles_SelfLoop(t *testing.T) {
	selfLoop := []core.SymbolNode{
		{Package: "alone", Name: "F", Kind: core.KindGoFunc, Calls: []string{"alone.F"}},
	}
	g := packagedeps.Build(selfLoop)
	cycles := packagedeps.Cycles(g)
	if len(cycles) != 1 {
		t.Fatalf("expected 1 self-loop cycle, got %d: %v", len(cycles), cycles)
	}
	if !reflect.DeepEqual(cycles[0], []string{"alone"}) {
		t.Errorf("self-loop cycle = %v; want [alone]", cycles[0])
	}
}

func TestCycles_MultipleCycles(t *testing.T) {
	// Two independent cycles: {a,b} and {x,y}
	multi := []core.SymbolNode{
		{Package: "a", Name: "F", Kind: core.KindGoFunc, Calls: []string{"b.G"}},
		{Package: "b", Name: "G", Kind: core.KindGoFunc, Calls: []string{"a.F"}},
		{Package: "x", Name: "P", Kind: core.KindGoFunc, Calls: []string{"y.Q"}},
		{Package: "y", Name: "Q", Kind: core.KindGoFunc, Calls: []string{"x.P"}},
	}
	g := packagedeps.Build(multi)
	cycles := packagedeps.Cycles(g)
	if len(cycles) != 2 {
		t.Fatalf("expected 2 cycles, got %d: %v", len(cycles), cycles)
	}
	if !reflect.DeepEqual(cycles[0], []string{"a", "b"}) {
		t.Errorf("cycle[0] = %v; want [a b]", cycles[0])
	}
	if !reflect.DeepEqual(cycles[1], []string{"x", "y"}) {
		t.Errorf("cycle[1] = %v; want [x y]", cycles[1])
	}
}

func TestBuild_ServiceScopedCallKey(t *testing.T) {
	// Calls in "svc::pkg.Name" format.
	svcSymbols := []core.SymbolNode{
		{
			Package: "handler", Name: "Serve", Kind: core.KindGoFunc,
			Calls: []string{"mysvc::db.Query"},
		},
		{Package: "db", Name: "Query", Kind: core.KindGoFunc},
	}
	g := packagedeps.Build(svcSymbols)
	got := packagedeps.Deps(g, "handler")
	want := []string{"db"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Deps(handler) = %v; want %v", got, want)
	}
}

func TestBuild_BareCallKeyIgnored(t *testing.T) {
	// Bare "Name" entries (no package) must not create edges.
	bare := []core.SymbolNode{
		{
			Package: "foo", Name: "Bar", Kind: core.KindGoFunc,
			Calls: []string{"someBareFunc"},
		},
	}
	g := packagedeps.Build(bare)
	got := packagedeps.Deps(g, "foo")
	if len(got) != 0 {
		t.Errorf("bare call key should produce no dep, got %v", got)
	}
}

func TestDeterminism(t *testing.T) {
	// Build and query twice; results must be identical.
	g1 := packagedeps.Build(symbols)
	g2 := packagedeps.Build(symbols)
	pkgs := []string{"auth", "api", "storage", "crypto"}
	for _, p := range pkgs {
		d1 := packagedeps.Deps(g1, p)
		d2 := packagedeps.Deps(g2, p)
		if !reflect.DeepEqual(d1, d2) {
			t.Errorf("non-deterministic Deps(%q): %v vs %v", p, d1, d2)
		}
	}
	c1 := packagedeps.Cycles(g1)
	c2 := packagedeps.Cycles(g2)
	if !reflect.DeepEqual(c1, c2) {
		t.Errorf("non-deterministic Cycles: %v vs %v", c1, c2)
	}
}
