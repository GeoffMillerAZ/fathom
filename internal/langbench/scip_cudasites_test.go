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
// specification, so the compiler's own definition there (clang-ast-defs'
// range, round 4 C4-R1) stands in, its calls scored against the compiler's
// sites (clang_stand_in_defs, clang_stand_in_extern_c).
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
	opts.ClangLinkage.Function("k.cu", "launch", 3, 5, true)
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
	if res.Counts[CntClangMacroBody] != 1 || res.Counts[CntClangStandInDefs] != 1 || res.Counts[CntClangStandInExternC] != 1 {
		t.Errorf("clang_calls_macro_body %d, clang_stand_in_defs %d, clang_stand_in_extern_c %d; want 1, 1, 1",
			res.Counts[CntClangMacroBody], res.Counts[CntClangStandInDefs], res.Counts[CntClangStandInExternC])
	}
}

// TestScoreSCIPCUDAStandInsFromCompiler (round 4 C4-R1): a stand-in's range is
// the compiler's (clang-ast-defs), never fathom's. scip-clang defines outer()
// over lines 1-6 and fathom also extracts a nested inner() over lines 2-4 that
// the index lacks; the compiler's one call site, g at line 3, is outer's call
// with or without ClangCUDA: a fathom node never shapes the truth.
func TestScoreSCIPCUDAStandInsFromCompiler(t *testing.T) {
	src := "void outer(int n) {\n" + // 1
		"  struct L { void inner() {\n" + // 2
		"    g();\n" + // 3
		"  } };\n" + // 4
		"  (void)n;\n" + // 5
		"}\n" // 6
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-clang", Version: "0.4.0"}},
		Documents: []*scippb.Document{{RelativePath: "k.cu", Occurrences: []*scippb.Occurrence{
			scipCallsiteDef("cxx . . $ outer(2222222222222222).", nil, 0, 5, 10),
			scipCallsiteRef("cxx . . $ g(1111111111111111).", 2, 4, 5),
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{
		{Service: "s", Name: "outer", Kind: "CudaFunc", FilePath: "k.cu", Line: 1, EndLine: 6, Calls: []string{"g"}},
		{Service: "s", Name: "inner", Kind: "CudaFunc", FilePath: "k.cu", Line: 2, EndLine: 4, Calls: []string{"g"}},
	}
	for _, cuda := range []bool{false, true} {
		opts := SCIPOptionsFor("cuda", func(string) []byte { return []byte(src) })
		opts.ClangCUDA = cuda
		opts.ClangLinkage.Function("k.cu", "outer", 1, 6, false)
		opts.ClangSites = map[string][]ClangSite{"k.cu": {{Line: 3, Col: 5, Name: "g", Class: ClangResolved}}}
		res, err := ScoreSCIPWith(raw, "s", nodes, opts)
		if err != nil {
			t.Fatal(err)
		}
		if !res.CallTruth[CallPair{"k.cu", "outer", "g"}] || len(res.CallTruth) != 1 {
			t.Errorf("ClangCUDA=%v: call truth = %v, want only outer -> g", cuda, res.CallTruth)
		}
		if res.Counts[CntClangStandInDefs] != 0 {
			t.Errorf("ClangCUDA=%v: clang_stand_in_defs %d, want 0 (the compiler defines nothing the index lacks)", cuda, res.Counts[CntClangStandInDefs])
		}
	}
}
