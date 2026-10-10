package scip

import (
	"testing"

	scippb "github.com/geoffmilleraz/fathom/third_party/scip"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// moduleFixture is a two-file Python project as the real python extractor
// reads it (each file with its module symbol) and as scip-python 0.6.6 indexes
// it: one `<module path>/__init__:` definition per document at 0:0 with no
// enclosing_range, an import line referencing the imported module's, and a
// module-level call. pkg/a.py's class has an __init__ method, the leaf the
// module definition's descriptor ends in (O527); pkg/b.py's function sits on
// line 1 beside its module symbol.
func moduleFixture(t *testing.T) (*scippb.Index, []core.SymbolNode) {
	t.Helper()
	e, err := extractor.NewTSExtractor("python")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	files := map[string]string{
		"pkg/a.py": "import pkg.b\n\n\nclass Widget:\n    def __init__(self):\n        self.x = 1\n\n\npkg.b.helper()\n",
		"pkg/b.py": "def helper():\n    return 1\n",
	}
	var nodes []core.SymbolNode
	for _, p := range []string{"pkg/a.py", "pkg/b.py"} {
		ns, err := e.Extract("svc", p, []byte(files[p]))
		if err != nil {
			t.Fatal(err)
		}
		nodes = append(nodes, ns...)
	}
	const pre = "scip-python python proj 1.0 "
	def := int32(scippb.SymbolRole_Definition)
	imp := int32(scippb.SymbolRole_Import)
	idx := &scippb.Index{
		Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-python", Version: "0.6.6"}},
		Documents: []*scippb.Document{
			{RelativePath: "pkg/a.py", Language: "python", Occurrences: []*scippb.Occurrence{
				{Range: []int32{0, 0, 0}, Symbol: pre + "`pkg.a`/__init__:", SymbolRoles: def},
				{Range: []int32{0, 7, 12}, Symbol: pre + "`pkg.b`/__init__:", SymbolRoles: imp},
				{Range: []int32{3, 6, 12}, Symbol: pre + "`pkg.a`/Widget#", SymbolRoles: def, EnclosingRange: []int32{3, 0, 5, 18}},
				{Range: []int32{4, 8, 16}, Symbol: pre + "`pkg.a`/Widget#__init__().", SymbolRoles: def, EnclosingRange: []int32{4, 4, 5, 18}},
				{Range: []int32{8, 0, 5}, Symbol: pre + "`pkg.b`/__init__:"},
				{Range: []int32{8, 6, 12}, Symbol: pre + "`pkg.b`/helper()."},
			}},
			{RelativePath: "pkg/b.py", Language: "python", Occurrences: []*scippb.Occurrence{
				{Range: []int32{0, 0, 0}, Symbol: pre + "`pkg.b`/__init__:", SymbolRoles: def},
				{Range: []int32{0, 4, 10}, Symbol: pre + "`pkg.b`/helper().", SymbolRoles: def, EnclosingRange: []int32{0, 0, 1, 12}},
			}},
		},
	}
	return idx, nodes
}

// TestImportIndex_K18_ModuleDefinitionIsTheModuleSymbol (LB-F348, O527): a
// scip-python module definition maps to its file's module symbol, never by
// its `__init__` leaf to a class's __init__ nor by position to a definition on
// line 1; the file's top-level references are made from it, and a reference
// to an imported module targets that module's symbol.
func TestImportIndex_K18_ModuleDefinitionIsTheModuleSymbol(t *testing.T) {
	idx, nodes := moduleFixture(t)
	modA := core.SymbolKey("svc", "pkg/a", core.ModuleSymbolName)
	modB := core.SymbolKey("svc", "pkg/b", core.ModuleSymbolName)
	helper := core.SymbolKey("svc", "pkg/b", "helper")
	initKey := core.SymbolKey("svc", "pkg/a", "Widget.__init__")

	got := importOf(t, idx, nodes)
	pairs := map[edgePair]int{}
	for _, p := range edgePairs(got) {
		pairs[p]++
		if p.from == initKey {
			t.Errorf("an edge from %s, the class's __init__ (O527's mis-join): %+v", initKey, p)
		}
	}
	want := map[edgePair]int{
		{modA, modB, core.EdgeDirect}:   2, // the import line and `pkg.b` in the call's chain
		{modA, helper, core.EdgeDirect}: 1,
	}
	if len(pairs) != len(want) {
		t.Errorf("edges = %v, want %v", pairs, want)
	}
	for p, n := range want {
		if pairs[p] != n {
			t.Errorf("edge %+v x%d, want x%d (all: %v)", p, pairs[p], n, pairs)
		}
	}
	if r := got.Report; r.Unmapped != 0 || r.ResolvedViaFallback != 0 || r.Skipped["unmapped-endpoint"] != 0 || r.Skipped["no-enclosing-definition"] != 0 {
		t.Errorf("report = %+v, want every definition mapped, none by fallback, and every reference made from a definition", r)
	}

	// Without module symbols (a dialect that mints none) the module
	// definitions map to nothing: never to Widget.__init__ by leaf, never to
	// helper by position, and the top-level references are made from no
	// definition rather than from the wrong one.
	var plain []core.SymbolNode
	for _, n := range nodes {
		if !n.IsModuleSymbol() {
			plain = append(plain, n)
		}
	}
	got = importOf(t, idx, plain)
	for _, p := range edgePairs(got) {
		t.Errorf("without module symbols, an edge %+v", p)
	}
	if r := got.Report; r.Unmapped != 2 || r.Skipped["no-enclosing-definition"] != 3 {
		t.Errorf("without module symbols: Unmapped %d (want 2, the module definitions), Skipped %v (want 3 no-enclosing-definition)", r.Unmapped, r.Skipped)
	}
}

// TestModuleDefinition: scip-python's module symbol is one namespace
// descriptor followed by the `__init__:` meta descriptor; a method, a term or
// a nested meta named __init__ is not one, nor is another producer's.
func TestModuleDefinition(t *testing.T) {
	const pre = "scip-python python flask 3.1.3 "
	for sym, want := range map[string]bool{
		pre + "flask/__init__:":                      true,
		pre + "`flask.views`/__init__:":              true,
		pre + "`flask.views`/View#__init__().":       false,
		pre + "`flask.views`/__init__.":              false,
		pre + "`flask.views`/View#__init__:":         false,
		pre + "`flask.views`/sub/__init__:":          false,
		"scip-typescript npm x 1.0 `a.ts`/__init__:": false,
		"local 3":                      false,
		pre + "/__init__:":             false,
		pre + "`flask.views/__init__:": false,
		"scip-python python python-stdlib 3.11 os/__init__:": true,
	} {
		if got := ModuleDefinition(sym); got != want {
			t.Errorf("ModuleDefinition(%q) = %v, want %v", sym, got, want)
		}
	}
}
