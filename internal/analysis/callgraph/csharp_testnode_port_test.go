package callgraph

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestCSharpCandidateBranchPreservesGenericTestEntries(t *testing.T) {
	for _, tc := range []struct {
		name string
		node core.SymbolNode
		want bool
	}{
		{"attributed method outside filename convention", core.SymbolNode{Name: "Cases.KeepsStock", Kind: core.KindCSharpMethod, FilePath: "Specs/Cases.cs", Signature: "[Test] public void KeepsStock()"}, true},
		{"unannotated Test prefix", core.SymbolNode{Name: "TestWithoutAttribute", Kind: core.KindCSharpMethod, FilePath: "Specs/CasesTests.cs", Signature: "public void TestWithoutAttribute()"}, false},
		{"unrelated qualified attribute", core.SymbolNode{Name: "Cases.Other", Kind: core.KindCSharpMethod, FilePath: "Specs/CasesTests.cs", Signature: "[Other.Test] public void Other()"}, false},
		{"lifecycle is not test", core.SymbolNode{Name: "TestSetup", Kind: core.KindCSharpMethod, FilePath: "Specs/CasesTests.cs", Signature: "[SetUp] public void TestSetup()"}, false},
		{"type name is not test method", core.SymbolNode{Name: "TestContainer", Kind: core.KindCSharpClass, FilePath: "Specs/CasesTests.cs"}, false},
		{"upstream pytest entry", core.SymbolNode{Name: "test_add", Kind: "PythonFunc", FilePath: "py/test_calc.py", Signature: "def test_add()"}, true},
		{"upstream jest entry", core.SymbolNode{Name: `"adds"`, Kind: "JSTest", FilePath: "src/calc.test.ts", Signature: `it("adds", () =>`}, true},
		{"upstream Go test", core.SymbolNode{Name: "TestAdd", Kind: core.KindGoFunc, FilePath: "calc_test.go"}, true},
		{"upstream Go helper method", core.SymbolNode{Name: "fakeEvidence.TestReaches", Kind: core.KindGoMethod, FilePath: "calc_test.go"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsTestNode(tc.node); got != tc.want {
				t.Fatalf("test candidate=%v, want %v for %+v", got, tc.want, tc.node)
			}
		})
	}
}
