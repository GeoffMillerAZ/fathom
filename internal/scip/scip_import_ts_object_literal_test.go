package scip

import (
	"os"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// scip-typescript0.4.0 names a property assignment's definition value0:,
// value1:, etc (the existing D2 fixture in langbench/scip_ts_test.go).
// Those are distinct from first./second. even on the same source line.
// This fixture is synthetic SCIP with real syntax extraction, not a retained
// compiler run or a new language-grade measurement.
func TestImportIndex_TSObjectLiteralDoesNotAliasItsContainer(t *testing.T) {
	const path = "ts-object-literal-identity.js"
	src, err := os.ReadFile("testdata/" + path)
	if err != nil {
		t.Fatal(err)
	}
	e, err := extractor.NewTSExtractor("javascript")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	nodes, err := e.Extract("s", path, src)
	if err != nil {
		t.Fatal(err)
	}
	const pre = "scip-typescript npm fixture 1.0 `ts-object-literal-identity.js`/"
	def := func(sym string, enc []int32, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: pre + sym, Range: rng, EnclosingRange: enc, SymbolRoles: int32(scippb.SymbolRole_Definition)}
	}
	ref := func(sym string, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: pre + sym, Range: rng}
	}
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-typescript", Version: "0.4.0"}},
		Documents: []*scippb.Document{{RelativePath: path, Language: "javascript", Occurrences: []*scippb.Occurrence{
			def("", []int32{0, 0, 5, 0}, 0, 0, 0),
			def("first.", nil, 0, 13, 18),
			def("value0:", nil, 0, 23, 28),
			def("second.", nil, 1, 13, 19),
			def("value1:", nil, 1, 24, 29),
			def("read().", []int32{2, 0, 4, 1}, 2, 16, 20),
			ref("first.", 3, 9, 14), ref("value0:", 3, 15, 20),
			ref("second.", 3, 23, 29), ref("value1:", 3, 30, 35),
		}}}}
	imp := importOf(t, idx, nodes)
	if imp.Report.Unmapped != 2 || imp.Report.Direct != 2 || imp.Report.Unresolved != 0 || imp.Report.Skipped["unmapped-endpoint"] != 2 {
		t.Fatalf("object properties must stay internal-unmapped while their real containers map: %+v", imp.Report)
	}
	if got := len(edgePairs(imp)); got != 2 {
		t.Fatalf("%d edges: an unsupported property fabricated a duplicate container edge", got)
	}
}

func TestResolveDefinition_TSObjectLiteralCannotUseLineOrLeafFallback(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "s", Package: "a", Name: "cfg", Kind: "JSConst", FilePath: "a.js", Line: 1, EndLine: 1},
		{Service: "s", Package: "a", Name: "value0", Kind: "JSConst", FilePath: "a.js", Line: 2, EndLine: 2},
	}
	byLoc, byLeaf := indexNodesByLocation(nodes), indexNodesByFileAndLeaf(nodes)
	for _, line := range []int32{0, 99} {
		if n, mapped, _ := resolveDefinition(byLoc, byLeaf, "a.js", line, "scip-typescript npm fixture 1 a/value0:", false); mapped {
			t.Fatalf("property joined %s via line/leaf at source line %d", n.Name, line)
		}
	}
	// Real globals/fields/methods retain the existing position join. In
	// particular C# constructor spelling differs from the extracted type
	// name; no general same-name condition may be added for this repair.
	for _, tc := range []struct{ sym, name, kind string }{
		{"scip-typescript npm fixture 1 a/cfg.", "cfg", "JSConst"},
		{"scip-typescript npm fixture 1 a/Box#value.", "Box.value", "JSField"},
		{"scip-typescript npm fixture 1 a/Box#read().", "Box.read", "JSMethod"},
		{"scip-dotnet nuget fixture 1 Box#Value.", "Box.Value", "CSharpProperty"},
		{"scip-dotnet nuget fixture 1 Box#`#ctor`().", "Box.Box", "CSharpMethod"},
		{"scip-dotnet nuget fixture 1 Box#Read().", "Box.Read", "CSharpMethod"},
	} {
		node := core.SymbolNode{Service: "s", Package: "a", Name: tc.name, Kind: core.SourceKind(tc.kind), FilePath: "a.js", Line: 1, EndLine: 1}
		n, mapped, _ := resolveDefinition(indexNodesByLocation([]core.SymbolNode{node}), indexNodesByFileAndLeaf([]core.SymbolNode{node}), "a.js", 0, tc.sym, false)
		if !mapped || n.Key() != node.Key() {
			t.Errorf("legitimate producer-specific position join changed for %s", tc.sym)
		}
	}
}
