package callgraph_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestCrossPackageSelectorResolves covers the very common Go pattern where a
// test lives in an external "<pkg>_test" package and calls the symbol under
// test as a qualified selector ("pkg.Name"). The callee is recorded as
// "pkg.Name" while the node key is "svc::pkg.Name" (boundary "::", not "."), so
// the resolver must match the "::"-boundary or the edge is silently dropped and
// test_impact wrongly reports tested code as untested.
func TestCrossPackageSelectorResolves(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "foo", Name: "Foo", FilePath: "foo/foo.go"},
		{Service: "svc", Package: "foo_test", Name: "TestFoo", FilePath: "foo/foo_test.go", Calls: []string{"foo.Foo"}},
	}
	g := callgraph.Build(nodes)

	// The test must be recognized as reaching Foo.
	tests := callgraph.TestImpact(g, "Foo")
	if len(tests) != 1 || tests[0] != "svc::foo/foo_test.TestFoo" {
		t.Fatalf("TestImpact(Foo) = %v, want [svc::foo/foo_test.TestFoo]", tests)
	}
	// Reachability and rename-impact must see the cross-package edge too.
	if !callgraph.Reachable(g, "TestFoo", "Foo") {
		t.Fatal("Reachable(TestFoo, Foo) = false, want true")
	}
	if callers := callgraph.RenameImpact(g, "Foo"); len(callers) != 1 || callers[0] != "svc::foo/foo_test.TestFoo" {
		t.Fatalf("RenameImpact(Foo) = %v, want [svc::foo/foo_test.TestFoo]", callers)
	}
}
