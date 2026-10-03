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

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestCUEReferenceEdgesWiredDirect(t *testing.T) {
	res := extractCUE("svc", "kit.cue", []byte("package kit\n#Kit: #Block & {name: string}\n#Block: {}\n"))
	for _, n := range res.Nodes {
		if n.Name != "#Kit" {
			continue
		}
		for _, c := range n.Calls {
			if c == core.SymbolKey("svc", "kit", "#Block") {
				return
			}
		}
		t.Fatalf("#Kit.Calls: want an edge to #Block, got %v", n.Calls)
	}
	t.Fatal("extractCUE: no #Kit entity")
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
