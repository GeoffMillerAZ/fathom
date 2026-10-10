package scip

import (
	"testing"

	scippb "github.com/geoffmilleraz/fathom/third_party/scip"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// TestModuleDefinitionTypeScript (LB-F381, K18): scip-typescript names a
// file's module by its path's namespace descriptors, the file's own name
// backtick-quoted last (src/v4/core/`api.ts`/); that symbol is the
// module, and a namespace inside it, a member, or a namespace with no file
// name is not.
func TestModuleDefinitionTypeScript(t *testing.T) {
	const pre = "scip-typescript npm zod 3.25.0 "
	for sym, want := range map[string]bool{
		pre + "src/v4/core/`api.ts`/":               true,
		pre + "`index.ts`/":                         true,
		pre + "src/`types.d.ts`/":                   true,
		pre + "lib/`Compiler.js`/":                  true,
		pre + "src/`app.module.tsx`/":               true,
		pre + "src/`x.mjs`/":                        true,
		pre + "src/`x.cts`/":                        true,
		pre + "src/v4/core/`api.ts`/TimePrecision.": false,
		pre + "src/v4/core/`api.ts`/util/":          false,
		pre + "src/v4/core/`api.ts`/Foo#":           false,
		pre + "src/v4/core/":                        false,
		pre + "`zod/v4/core`/":                      false,
		pre + "src/`data.json`/":                    false,
		"scip-python python x 1.0 `a.ts`/":          false,
		"local 7":                                   false,
	} {
		if got := ModuleDefinition(sym); got != want {
			t.Errorf("ModuleDefinition(%q) = %v, want %v", sym, got, want)
		}
	}
}

// TestImportIndex_K18_TypeScriptModule (LB-F381): a scip-typescript module
// definition (0:0, its enclosing range the whole document) maps to its file's
// module symbol, so the references the file's top level makes — an import,
// a call — are made from it, and an import naming a module targets that
// module's symbol.
func TestImportIndex_K18_TypeScriptModule(t *testing.T) {
	e, err := extractor.NewTSExtractor("javascript")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	files := map[string]string{
		"src/a.ts": "import { helper } from \"./b\";\n\nhelper();\n",
		"src/b.ts": "export function helper() {\n  return 1;\n}\n",
	}
	var nodes []core.SymbolNode
	for _, p := range []string{"src/a.ts", "src/b.ts"} {
		ns, err := e.Extract("svc", p, []byte(files[p]))
		if err != nil {
			t.Fatal(err)
		}
		nodes = append(nodes, ns...)
	}
	const pre = "scip-typescript npm proj 1.0 "
	def := int32(scippb.SymbolRole_Definition)
	imp := int32(scippb.SymbolRole_Import)
	idx := &scippb.Index{
		Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-typescript", Version: "0.4.0"}},
		Documents: []*scippb.Document{
			{RelativePath: "src/a.ts", Language: "typescript", Occurrences: []*scippb.Occurrence{
				{Range: []int32{0, 0, 0}, Symbol: pre + "src/`a.ts`/", SymbolRoles: def, EnclosingRange: []int32{0, 0, 3, 0}},
				{Range: []int32{0, 9, 15}, Symbol: pre + "src/`b.ts`/helper().", SymbolRoles: imp},
				{Range: []int32{0, 23, 28}, Symbol: pre + "src/`b.ts`/"},
				{Range: []int32{2, 0, 6}, Symbol: pre + "src/`b.ts`/helper()."},
			}},
			{RelativePath: "src/b.ts", Language: "typescript", Occurrences: []*scippb.Occurrence{
				{Range: []int32{0, 0, 0}, Symbol: pre + "src/`b.ts`/", SymbolRoles: def, EnclosingRange: []int32{0, 0, 3, 0}},
				{Range: []int32{0, 16, 22}, Symbol: pre + "src/`b.ts`/helper().", SymbolRoles: def, EnclosingRange: []int32{0, 0, 2, 1}},
			}},
		},
	}
	modA := core.SymbolKey("svc", "src/a", core.ModuleSymbolName)
	modB := core.SymbolKey("svc", "src/b", core.ModuleSymbolName)
	helper := core.SymbolKey("svc", "src/b", "helper")
	got := importOf(t, idx, nodes)
	pairs := map[edgePair]int{}
	for _, p := range edgePairs(got) {
		pairs[p]++
	}
	want := map[edgePair]int{
		{modA, helper, core.EdgeDirect}: 2, // the import's name and the top-level call
		{modA, modB, core.EdgeDirect}:   1, // the import's module
	}
	if len(pairs) != len(want) {
		t.Errorf("edges = %v, want %v", pairs, want)
	}
	for p, n := range want {
		if pairs[p] != n {
			t.Errorf("edge %+v x%d, want x%d (all: %v)", p, pairs[p], n, pairs)
		}
	}
	if r := got.Report; r.Unmapped != 0 || r.Skipped["unmapped-endpoint"] != 0 || r.Skipped["no-enclosing-definition"] != 0 {
		t.Errorf("report = %+v, want every definition mapped and every reference made from one", r)
	}
}
