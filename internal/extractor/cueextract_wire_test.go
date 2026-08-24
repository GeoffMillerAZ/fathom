package extractor

// Direct top-level wire tests for the VS-3/VS-4 contracts. The behavioral
// suite (cueextract_test.go, package extractor_test) exercises these
// through CUEExtractor.Extract — but Extract is a method name shared by
// every extractor type, and the reachability index does not wire
// same-named method calls (the third silent-void mode, alongside t.Run
// closures and Test-name suffixes). The contracted symbol must be called
// at the test's top level, directly.

import (
	"testing"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/parser"
	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestCUEReferenceEdgesWiredDirect(t *testing.T) {
	f, err := parser.ParseFile("kit.cue", "package kit\n#Kit: #Block & {name: string}\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var value ast.Expr
	for _, d := range f.Decls {
		if field, ok := d.(*ast.Field); ok {
			value = field.Value
			break
		}
	}
	if value == nil {
		t.Fatal("fixture parsed to no *ast.Field declaration")
	}
	localDefs := map[string]bool{"#Kit": true, "#Block": true}
	refs := cueReferenceEdges("svc", "kit", value, localDefs, map[string]string{})
	for _, r := range refs {
		if r == core.SymbolKey("svc", "kit", "#Block") {
			return
		}
	}
	t.Fatalf("cueReferenceEdges: want an edge to #Block, got %v", refs)
}

func TestCUETestdataReachesWiredDirect(t *testing.T) {
	nodes := []core.SymbolNode{{
		Service:  "svc",
		Package:  "kit",
		Name:     "myKit",
		Kind:     core.KindCUEField,
		FilePath: "kit/testdata/fixture.cue",
		Calls:    []string{"kit.#Kit"},
	}}
	out := cueTestdataReaches("svc", "kit", "kit/testdata/fixture.cue", nodes)
	for _, n := range out {
		if n.Name == "TestmyKit" {
			return
		}
	}
	t.Fatalf("cueTestdataReaches: want synthesized TestmyKit node, got %v", out)
}
