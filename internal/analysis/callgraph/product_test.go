package callgraph_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// productFixture is a service with one binary and a library the binary
// only partly uses:
//
//	cmd/app main -> lib.Run -> lib.helper ; main -> reg.Use
//	reg init -> reg.Handler                  (reg is linked: main calls reg.Use)
//	orphan init -> orphan.Registered         (orphan is linked by nothing)
//	TestLibOnly -> lib.LibOnly               (a test is its only caller)
//	testdata main -> lib.Fixture             (a fixture binary is not the product)
func productFixture() []core.SymbolNode {
	fn := func(file, pkg, name string, calls ...string) core.SymbolNode {
		return core.SymbolNode{Service: "svc", Package: pkg, Name: name, Kind: core.KindGoFunc, FilePath: file, Calls: calls}
	}
	return []core.SymbolNode{
		fn("cmd/app/main.go", "main", "main", "lib.Run", "reg.Use"),
		fn("internal/lib/lib.go", "lib", "Run", "helper"),
		fn("internal/lib/lib.go", "lib", "helper"),
		fn("internal/lib/lib.go", "lib", "LibOnly"),
		fn("internal/lib/lib.go", "lib", "Fixture"),
		fn("internal/lib/lib_test.go", "lib", "TestLibOnly", "LibOnly"),
		fn("internal/reg/reg.go", "reg", "Use"),
		fn("internal/reg/reg.go", "reg", "init", "Handler"),
		fn("internal/reg/reg.go", "reg", "Handler"),
		fn("internal/orphan/orphan.go", "orphan", "init", "Registered"),
		fn("internal/orphan/orphan.go", "orphan", "Registered"),
		fn("internal/lib/testdata/fix/main.go", "main", "main", "lib.Fixture"),
	}
}

// productTyped is productFixture's every call as a typed edge: since RF-5
// the product walk follows edges of resolution tier typed only
// (typedreach_test.go covers the heuristic ones).
func productTyped() []core.Edge {
	var out []core.Edge
	g := callgraph.Build(productFixture())
	for _, n := range productFixture() {
		for _, c := range callgraph.CalleesDetail(g, n.Key()) {
			out = append(out, core.Edge{FromService: "svc", FromSymbol: n.Key(), ToSymbol: c.Key, Kind: core.EdgeDirect})
		}
	}
	return out
}

func productGraph() *callgraph.Graph { return callgraph.BuildBoth(productFixture(), productTyped()) }

func TestProductReach_EntriesAreMainsAndLinkedInits(t *testing.T) {
	r := callgraph.ProductEntryReach(productGraph())
	want := []string{"svc::cmd/app/main.main", "svc::internal/reg/reg.init"}
	if got := r.Entries(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Entries = %v, want %v (a testdata main and an unlinked package's init are not product entry points)", got, want)
	}
}

func TestProductReach_TestOnlyIsNotWired(t *testing.T) {
	r := callgraph.ProductEntryReach(productGraph())
	cases := []struct {
		symbol string
		entry  string // "" = not reached by any product entry point
	}{
		{"Run", "svc::cmd/app/main.main"},
		{"svc::internal/lib/lib.helper", "svc::cmd/app/main.main"},
		{"Handler", "svc::internal/reg/reg.init"},
		{"LibOnly", ""},    // reached only by TestLibOnly
		{"Fixture", ""},    // reached only by a testdata binary
		{"Registered", ""}, // registered by an init no binary links
		{"NoSuchSymbol", ""},
	}
	for _, c := range cases {
		entry, ok := r.ReachedBy(c.symbol)
		if ok != (c.entry != "") || entry != c.entry {
			t.Errorf("ReachedBy(%q) = (%q, %v), want (%q, %v)", c.symbol, entry, ok, c.entry, c.entry != "")
		}
	}
}

func TestProductReach_NoEntryPointsMeansALibraryService(t *testing.T) {
	var lib []core.SymbolNode
	for _, n := range productFixture() {
		if n.Name != "main" {
			lib = append(lib, n)
		}
	}
	r := callgraph.ProductEntryReach(callgraph.Build(lib))
	if r.HasEntries() {
		t.Fatalf("a service with no main package has no product entry point, got %v", r.Entries())
	}
	if _, ok := r.ReachedBy("Run"); ok {
		t.Fatal("nothing is product-reached in a service with no entry point")
	}
}

func TestProductReach_Deterministic(t *testing.T) {
	a := callgraph.ProductEntryReach(productGraph())
	b := callgraph.ProductEntryReach(productGraph())
	if !reflect.DeepEqual(a.Entries(), b.Entries()) || a.Len() != b.Len() {
		t.Fatalf("two builds differ: %v/%d vs %v/%d", a.Entries(), a.Len(), b.Entries(), b.Len())
	}
	for _, s := range []string{"Run", "helper", "Handler", "Use"} {
		ea, _ := a.ReachedBy(s)
		eb, _ := b.ReachedBy(s)
		if ea != eb {
			t.Errorf("ReachedBy(%q): %q vs %q", s, ea, eb)
		}
	}
}

// TestAddTypedEdges_PackageInitVertex: the typed pass keys a package's
// variable initializers to "<pkg>.init"; a package with such initializers
// and no init() function gets that one vertex, so what its initializers
// reference is reached when the package is linked. Nothing else gets an
// invented vertex: a method-shaped key, or a file outside the key's
// directory, is dropped as before.
func TestAddTypedEdges_PackageInitVertex(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "main", Name: "main", Kind: core.KindGoFunc, FilePath: "cmd/app/main.go", Calls: []string{"reg.Use"}},
		{Service: "svc", Package: "reg", Name: "Use", Kind: core.KindGoFunc, FilePath: "internal/reg/reg.go"},
		{Service: "svc", Package: "reg", Name: "handler", Kind: core.KindGoFunc, FilePath: "internal/reg/reg.go"},
		{Service: "svc", Package: "reg", Name: "other", Kind: core.KindGoFunc, FilePath: "internal/reg/reg.go"},
	}
	g := callgraph.Build(nodes)
	callgraph.AddTypedEdges(g, []core.Edge{
		{FromSymbol: "svc::cmd/app/main.main", FromFile: "cmd/app/main.go", ToSymbol: "svc::internal/reg/reg.Use", Kind: core.EdgeDirect},
		{FromSymbol: "svc::internal/reg/reg.init", FromFile: "internal/reg/table.go", ToSymbol: "svc::internal/reg/reg.handler", Kind: core.EdgeFuncValue},
		{FromSymbol: "svc::internal/reg/T.T.init", FromFile: "internal/reg/table.go", ToSymbol: "svc::internal/reg/reg.other", Kind: core.EdgeDirect},
		{FromSymbol: "svc::internal/elsewhere/reg.init", FromFile: "internal/reg/table.go", ToSymbol: "svc::internal/reg/reg.other", Kind: core.EdgeDirect},
	})
	if g.NodeCount() != len(nodes)+1 {
		t.Fatalf("NodeCount = %d, want %d (one package-init vertex, nothing else invented)", g.NodeCount(), len(nodes)+1)
	}
	r := callgraph.ProductEntryReach(g)
	if e, ok := r.ReachedBy("handler"); !ok || e != "svc::internal/reg/reg.init" {
		t.Errorf("handler: ReachedBy = (%q, %v), want the package's init", e, ok)
	}
	if _, ok := r.ReachedBy("other"); ok {
		t.Error("other is reached only by edges from keys that are not package initialization")
	}
}
