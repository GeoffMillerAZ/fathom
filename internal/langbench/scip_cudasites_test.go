package langbench

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/core"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// TestScoreSCIPCUDACompilerSiteRules (round 3 C): two classes of cuda-samples'
// L2 pairs, read as C reads them. A call written inside a #define body is a
// call of no definition: scip-clang references no token of a macro's body, so
// C's truth never holds one, and the compiler's site there (the build's
// clang-calls) is left out (clang_calls_macro_body). And scip-clang 0.4.0
// writes no definition for a function in an `extern "C"` linkage
// specification, so fathom's definition there stands in, its calls scored
// against the compiler's sites (clang_stand_in_defs).
func TestScoreSCIPCUDACompilerSiteRules(t *testing.T) {
	src := "#define CHECK(x) \\\n" + // 1
		"  if (x) abort();\n" + // 2
		"extern \"C\" void launch(int n) {\n" + // 3
		"  int g = iDivUp(n, 4);\n" + // 4
		"}\n" // 5
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-clang", Version: "0.4.0"}},
		Documents: []*scippb.Document{{RelativePath: "k.cu", Occurrences: []*scippb.Occurrence{
			scipCallsiteDef("cxx . . $ `k.cu:1:9`!", nil, 0, 8, 13),
			scipCallsiteDef("local 0", nil, 2, 26, 27),
			scipCallsiteRef("cxx . . $ iDivUp(1111111111111111).", 3, 10, 16),
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{
		{Service: "s", Name: "CHECK", Kind: "CudaMacro", FilePath: "k.cu", Line: 1, EndLine: 2},
		{Service: "s", Name: "launch", Kind: "CudaFunc", FilePath: "k.cu", Line: 3, EndLine: 5, Calls: []string{"iDivUp"}},
	}
	opts := SCIPOptionsFor("cuda", func(string) []byte { return []byte(src) })
	opts.ClangSites = map[string][]ClangSite{"k.cu": {
		{Line: 2, Col: 11, Name: "abort", Class: ClangResolved},
		{Line: 4, Col: 11, Name: "iDivUp", Class: ClangResolved},
	}}
	res, err := ScoreSCIPWith(raw, "s", nodes, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !res.CallTruth[CallPair{"k.cu", "launch", "iDivUp"}] || len(res.CallTruth) != 1 {
		t.Errorf("call truth = %v, want only launch -> iDivUp", res.CallTruth)
	}
	if res.Counts[CntClangMacroBody] != 1 || res.Counts[CntClangStandInDefs] != 1 {
		t.Errorf("clang_calls_macro_body %d, clang_stand_in_defs %d; want 1, 1", res.Counts[CntClangMacroBody], res.Counts[CntClangStandInDefs])
	}
}
