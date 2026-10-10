package langbench

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/core"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// TestScoreSCIP_K18_ModuleDefinitionMapsToModuleSymbol (LB-F348, O527):
// def_mapping names a scip-python module definition (`<path>/__init__:`) as
// the file's module symbol, so it maps to that symbol at line 1 and never to
// a class's __init__ within two lines of it, and a module-level call is a
// call made from the module symbol.
func TestScoreSCIP_K18_ModuleDefinitionMapsToModuleSymbol(t *testing.T) {
	const pre = "scip-python python proj 1.0 "
	def := int32(scippb.SymbolRole_Definition)
	idx := &scippb.Index{
		Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-python", Version: "0.6.6"}},
		Documents: []*scippb.Document{{RelativePath: "a.py", Language: "python", Occurrences: []*scippb.Occurrence{
			{Range: []int32{0, 0, 0}, Symbol: pre + "a/__init__:", SymbolRoles: def},
			{Range: []int32{0, 6, 7}, Symbol: pre + "a/W#", SymbolRoles: def, EnclosingRange: []int32{0, 0, 2, 12}},
			{Range: []int32{1, 8, 16}, Symbol: pre + "a/W#__init__().", SymbolRoles: def, EnclosingRange: []int32{1, 4, 2, 12}},
			{Range: []int32{3, 4, 7}, Symbol: pre + "a/run().", SymbolRoles: def, EnclosingRange: []int32{3, 0, 4, 8}},
			{Range: []int32{6, 0, 3}, Symbol: pre + "a/run()."},
		}}},
	}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	src := "class W:\n    def __init__(self):\n        pass\ndef run():\n    pass\n\nrun()\n"
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "a", Name: core.ModuleSymbolName, Kind: "PythonModule", FilePath: "a.py", Line: 1, EndLine: 7, Calls: []string{"run"}},
		{Service: "svc", Package: "a", Name: "W", Kind: "PythonClass", FilePath: "a.py", Line: 1, EndLine: 3},
		{Service: "svc", Package: "a", Name: "W.__init__", Kind: "PythonFunc", FilePath: "a.py", Line: 2, EndLine: 3},
		{Service: "svc", Package: "a", Name: "run", Kind: "PythonFunc", FilePath: "a.py", Line: 4, EndLine: 5},
	}
	res, err := ScoreSCIPWith(raw, "svc", nodes, SCIPOptionsFor("python", func(string) []byte { return []byte(src) }))
	if err != nil {
		t.Fatal(err)
	}
	if res.Counts[CntScipDefs] != 4 || res.Counts[CntScipDefsMapped] != 4 {
		t.Errorf("def_mapping %d of %d, want 4 of 4 (the module definition is the module symbol)", res.Counts[CntScipDefsMapped], res.Counts[CntScipDefs])
	}
	if !res.CallTruth[CallPair{"a.py", core.ModuleSymbolName, "run"}] || !res.CallFound[CallPair{"a.py", core.ModuleSymbolName, "run"}] {
		t.Errorf("the module-level call run() is not the module symbol's on both sides: truth %v, found %v", res.CallTruth, res.CallFound)
	}
	for p := range res.CallTruth {
		if p.Caller == "W.__init__" || p.Caller == "__init__" {
			t.Errorf("a call given to the class's __init__: %+v", p)
		}
	}
}
