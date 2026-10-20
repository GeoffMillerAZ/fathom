package callgraph

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestTestReachNamesNonGoTestEntries (TQ-28): the reverse walks that feed
// test_impact, related_tests and tests select name a jest block and a
// pytest function that call a changed symbol, and still name Go Test*.
func TestTestReachNamesNonGoTestEntries(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "s", Package: "src", Name: "add", Kind: "JSFunc", FilePath: "src/calc.ts"},
		{Service: "s", Package: "src", Name: `"adds"`, Kind: "JSTest", FilePath: "src/calc.test.ts", Signature: `it("adds", () =>`, Calls: []string{"add"}},
		{Service: "s", Package: "src", Name: "helper", Kind: "JSFunc", FilePath: "src/calc.test.ts", Calls: []string{"add"}},
		{Service: "s", Package: "py", Name: "py_add", Kind: "PythonFunc", FilePath: "py/calc.py"},
		{Service: "s", Package: "py", Name: "test_add", Kind: "PythonFunc", FilePath: "py/test_calc.py", Signature: "def test_add()", Calls: []string{"py_add"}},
	}
	g := Build(nodes)
	got := TestImpactDetail(g, nodes[0].Key())
	if len(got) != 1 || got[0].Key != nodes[1].Key() {
		t.Fatalf("js reach = %+v, want only the it block", got)
	}
	if !IsTestNode(nodes[4]) || IsTestNode(nodes[2]) || IsTestNode(nodes[0]) {
		t.Fatalf("IsTestNode: pytest function must be a test, helper and production must not be")
	}
}

// TestIsTestNodeKeepsGoRule (TQ-28): a Go symbol keeps Go's own rule, the
// Test prefix on its name. A fake's method named TestReaches in a _test.go
// file is not a test entry, so Go reach is what it was before TQ-28.
func TestIsTestNodeKeepsGoRule(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "s", Package: "v", Name: "Check", Kind: core.KindGoFunc, FilePath: "v/check.go"},
		{Service: "s", Package: "v", Name: "fakeEvidence.TestReaches", Kind: core.KindGoMethod, FilePath: "v/check_test.go", Signature: "func TestReaches(symbol string) []string", Calls: []string{"Check"}},
		{Service: "s", Package: "v", Name: "TestCheck", Kind: core.KindGoFunc, FilePath: "v/check_test.go", Calls: []string{"fakeEvidence.TestReaches"}},
	}
	if IsTestNode(nodes[1]) {
		t.Fatalf("Go method %q in a _test.go file read as a test entry", nodes[1].Name)
	}
	g := Build(nodes)
	if got := TestImpact(g, nodes[0].Key()); len(got) != 1 || got[0] != nodes[2].Key() {
		t.Fatalf("Go reach = %v, want only TestCheck", got)
	}
}
