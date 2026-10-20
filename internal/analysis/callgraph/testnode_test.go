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
