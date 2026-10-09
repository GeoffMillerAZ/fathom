package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/langbench"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// goModuleFixture is a two-package module: main.go imports fmt and the
// module's own package b, and main calls b.Foo; Foo calls bar in its file.
var goModuleFixture = map[string]string{
	"go.mod":  "module example.com/m\n\ngo 1.21\n",
	"main.go": "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/m/b\"\n)\n\nfunc main() {\n\tfmt.Println(b.Foo())\n}\n",
	"b/b.go":  "package b\n\n// Foo is called from main.\nfunc Foo() string { return bar() }\n\nfunc bar() string { return \"x\" }\n",
}

// goFixtureIndex is a scip-go-shaped index of goModuleFixture: a package is
// defined at its package clause and referenced on the import spec's line
// and at each selector; fmt is not in the index.
func goFixtureIndex(t *testing.T) string {
	t.Helper()
	const m = "scip-go gomod example.com/m v0.0.0 "
	const std = "scip-go gomod github.com/golang/go/src go1.22 "
	def := func(sym string, enc []int32, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: rng, EnclosingRange: enc, SymbolRoles: int32(scippb.SymbolRole_Definition)}
	}
	ref := func(sym string, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: rng}
	}
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-go", Version: "0.2.7"}},
		Documents: []*scippb.Document{
			{RelativePath: "b/b.go", Occurrences: []*scippb.Occurrence{
				def(m+"`example.com/m/b`/", nil, 0, 8, 9),
				def(m+"`example.com/m/b`/Foo().", []int32{3, 0, 3, 34}, 3, 5, 8),
				ref(m+"`example.com/m/b`/bar().", 3, 27, 30),
				def(m+"`example.com/m/b`/bar().", []int32{5, 0, 5, 32}, 5, 5, 8),
			}},
			{RelativePath: "main.go", Occurrences: []*scippb.Occurrence{
				def(m+"`example.com/m`/", nil, 0, 8, 12),
				ref(std+"fmt/", 3, 1, 6),
				ref(m+"`example.com/m/b`/", 5, 1, 18), // the import spec
				def(m+"`example.com/m`/main().", []int32{8, 0, 10, 1}, 8, 5, 9),
				ref(std+"fmt/", 9, 1, 4),
				ref(std+"fmt/Println().", 9, 5, 12),
				ref(m+"`example.com/m/b`/", 9, 13, 14),
				ref(m+"`example.com/m/b`/Foo().", 9, 15, 18),
			}},
		}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "go.scip")
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestCmdLangOracleGoFileEdgesAndImports (LB-F58): Go is native (goextract,
// no tags dialect), so its L3 is scored from what ingest stores for it: the
// typed go/packages pass's edges (fusion.EnrichTypedEdges, captured in an
// in-memory store) against scip-go's cross-file edges, and each import spec
// resolved by Go's own loader to its package's measured files against the
// index's references on the import lines.
func TestCmdLangOracleGoFileEdgesAndImports(t *testing.T) {
	subject := t.TempDir()
	writeTree(t, subject, goModuleFixture)
	out := filepath.Join(t.TempDir(), "go.json")
	args := []string{"-dialect", "go", "-subject", subject, "-subject-name", "fixture/go", "-scip", goFixtureIndex(t),
		"-ctags", filepath.Join(t.TempDir(), "none"),
		"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, raw := readValidReading(t, out)
	for k, v := range map[string]int{
		langbench.CntFileEdgesTruth: 1, langbench.CntFileEdgesFathom: 1, langbench.CntFileEdgesAgree: 1,
		langbench.CntImports: 2, langbench.CntImportsInRepo: 1, langbench.CntImportsAgree: 1,
	} {
		if r.Counts[k] != v {
			t.Errorf("%s = %d, want %d", k, r.Counts[k], v)
		}
	}
	for _, m := range []string{langbench.MFileEdgePrecision, langbench.MFileEdgeRecall, langbench.MImportResolution} {
		if v := r.Metrics[m]; v == nil || *v != 1 || r.MetricTiers[m] != "A" {
			t.Errorf("%s = %v tier %q, want 1 at A\n%s", m, v, r.MetricTiers[m], raw)
		}
	}
	if r.Grade.L3 != langbench.GradeExcellent {
		t.Errorf("grade.L3 = %q, want excellent", r.Grade.L3)
	}
	if !bytes.Contains(raw, []byte(noteGoTypedFileEdges)) {
		t.Errorf("the typed-pass basis of Go's file edges is not disclosed:\n%s", raw)
	}
}

// goDispatchFixture is one module whose Run calls Greet through an interface
// two types implement, and whose main calls Run.
var goDispatchFixture = map[string]string{
	"go.mod": "module example.com/m\n\ngo 1.21\n",
	"main.go": "package main\n\n" +
		"type Greeter interface{ Greet() string }\n" + // line 3 (1-based)
		"type A struct{}\n" +
		"func (A) Greet() string { return \"a\" }\n" +
		"type B struct{}\n" +
		"func (B) Greet() string { return \"b\" }\n" +
		"func Run(g Greeter) string { return g.Greet() }\n" +
		"func main() { Run(A{}); println(len(Run(B{}))) }\n",
}

// goDispatchIndex is a scip-go-shaped index of goDispatchFixture: the
// interface method is one term, `Greeter#Greet.`, referenced in Run and
// written as a call; main calls Run and println (no symbol, as in a real
// index it is the predeclared function) and a dependency's function
// (`other.Dep`, defined nowhere in the index).
func goDispatchIndex(t *testing.T) string {
	t.Helper()
	const m = "scip-go gomod example.com/m v0.0.0 "
	const dep = "scip-go gomod example.com/dep v1.0.0 "
	def := func(sym string, enc []int32, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: rng, EnclosingRange: enc, SymbolRoles: int32(scippb.SymbolRole_Definition)}
	}
	ref := func(sym string, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: rng}
	}
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-go", Version: "0.2.7"}},
		Documents: []*scippb.Document{{RelativePath: "main.go", Occurrences: []*scippb.Occurrence{
			def(m+"`example.com/m`/Greeter#Greet.", nil, 2, 24, 29),
			def(m+"`example.com/m`/Run().", []int32{7, 0, 7, 47}, 7, 5, 8),
			ref(m+"`example.com/m`/Greeter#Greet.", 7, 38, 43),
			def(m+"`example.com/m`/main().", []int32{8, 0, 8, 49}, 8, 5, 9),
			ref(m+"`example.com/m`/Run().", 8, 14, 17),
			ref(dep+"`example.com/dep`/Dep().", 8, 14, 17), // a dependency's call, bound by name nowhere
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "go.scip")
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestCmdLangOracleGoTypedCalls (§1.6): Go's L2 grades the calls the product
// serves (the call graph's syntactic Calls that resolve to a node, plus the
// typed pass's edges), not goextract's syntactic Calls alone. A dispatch
// edge to each of N implementers meets scip-go's one interface-method term
// at the method name; a call to a symbol the subject does not define is left
// out of the truth, counted in the notes; option A's numbers sit beside the
// graded set.
func TestCmdLangOracleGoTypedCalls(t *testing.T) {
	subject := t.TempDir()
	writeTree(t, subject, goDispatchFixture)
	out := filepath.Join(t.TempDir(), "go.json")
	unm := filepath.Join(t.TempDir(), "unmatched.json")
	args := []string{"-dialect", "go", "-subject", subject, "-subject-name", "fixture/go", "-scip", goDispatchIndex(t),
		"-ctags", filepath.Join(t.TempDir(), "none"), "-unmatched", unm,
		"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, raw := readValidReading(t, out)
	// Truth: Run->Greet (the interface term) and main->Run; Dep is a
	// dependency's call, named by no node of the subject. Served: the two
	// dispatch edges Run->A.Greet and Run->B.Greet collapse to one pair by
	// name, plus main->Run (direct). goextract writes no Calls entry for the
	// composite literals A{} and B{}, so no reference to a type is served
	// here and the pre-lg C, C and F agree (the classes D and E remove are
	// pinned on a real ingest by callgraph's TestServedCalls_Golden).
	for k, v := range map[string]int{
		langbench.CntCallTruth: 2, langbench.CntCallTruthFound: 2,
		langbench.CntCallFathom: 2, langbench.CntCallFathomInTrue: 2,
	} {
		if r.Counts[k] != v {
			t.Errorf("%s = %d, want %d\n%s", k, r.Counts[k], v, raw)
		}
	}
	for _, want := range []string{
		"Go L2 option numbers (§1.6): A syntactic",
		"2 interface dispatch (2 edges, one per implementer, collapse to 1 pairs by method name",
		"1 of 3 truth pairs name a callee the subject does not define",
		"Go L2 grades the served pairs (§1.6)",
		"D served without the syntactic edges to a type",
		"E served without the syntax-only edges into a method from a file the typed pass resolved",
		"C before wave lg (BuildBoth at 90f0a01f, built here explicitly) served-pre-lg 2 pairs",
		"F D and E together served-no-type-refs-typed-methods 2 pairs",
		"Go L2 graded pairs the truth lacks: 0 name a type",
		"Go L2 truth pairs the graded set lacks: 0 name no node of the service",
	} {
		if !bytes.Contains(raw, []byte(want)) {
			t.Errorf("the reading's notes lack %q:\n%s", want, raw)
		}
	}
	u, err := os.ReadFile(unm)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(u, []byte(`"callee": "Dep"`)) {
		t.Errorf("a call to a dependency is graded, not counted out:\n%s", u)
	}
}

// TestGoFileEdgeKinds (LB-F297): the kinds of the typed edges that draw each
// file edge, sorted and joined, so a fathom-only file edge says which class
// of typed edge drew it (direct, iface-dispatch, func-value, instantiates).
func TestGoFileEdgeKinds(t *testing.T) {
	nodes := []core.SymbolNode{
		{Name: "A", FilePath: "a.go", Kind: core.KindGoMethod},
		{Name: "B", FilePath: "b.go", Kind: core.KindGoMethod},
		{Name: "C", FilePath: "c.go", Kind: core.KindGoType},
	}
	edges := []core.Edge{
		{FromSymbol: nodes[0].Key(), FromFile: "a.go", ToSymbol: nodes[1].Key(), Kind: core.EdgeIfaceDispatch},
		{FromSymbol: nodes[0].Key(), FromFile: "a.go", ToSymbol: nodes[1].Key(), Kind: core.EdgeDirect},
		{FromSymbol: nodes[0].Key(), FromFile: "a.go", ToSymbol: nodes[2].Key(), Kind: core.EdgeInstantiates},
		{FromSymbol: nodes[1].Key(), FromFile: "b.go", ToSymbol: nodes[1].Key(), Kind: core.EdgeDirect}, // same file: no file edge
	}
	got := fileEdgeKinds(edges, nodes)
	want := map[[2]string]string{{"a.go", "b.go"}: "direct+iface-dispatch", {"a.go", "c.go"}: "instantiates"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("fileEdgeKinds = %v, want %v", got, want)
	}
}

// TestCmdLangOracleGoUnmatchedFileEdges (LB-F297): Go's -unmatched carries the
// file edges both ways, like every other language's, each fathom-only edge
// with the typed edge kinds that drew it.
func TestCmdLangOracleGoUnmatchedFileEdges(t *testing.T) {
	subject := t.TempDir()
	writeTree(t, subject, goModuleFixture)
	out := filepath.Join(t.TempDir(), "go.json")
	um := filepath.Join(t.TempDir(), "unmatched.json")
	args := []string{"-dialect", "go", "-subject", subject, "-subject-name", "fixture/go", "-scip", goFixtureIndex(t),
		"-ctags", filepath.Join(t.TempDir(), "none"), "-unmatched", um,
		"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	u := readUnmatched(t, um)
	if u.FileEdges == nil {
		t.Fatalf("no file_edges in Go's -unmatched: %+v", u)
	}
	if len(u.FileEdges.FathomOnly) != 0 || len(u.FileEdges.OracleOnly) != 0 {
		t.Errorf("file_edges = %+v, want both sides empty (main.go -> b/b.go agrees)", *u.FileEdges)
	}
}

// TestGoNamedEdges (LB-F297): Go's file edges are drawn by the edges whose
// target the source names at a site (a call, a function value), not by the
// facts the typed pass also stores: an interface dispatch edge to each
// implementer, an instantiation fact. A file edge only those kinds draw is
// left out of the file-edge set and counted.
func TestGoNamedEdges(t *testing.T) {
	nodes := []core.SymbolNode{
		{Name: "A", FilePath: "a.go", Kind: core.KindGoMethod},
		{Name: "B", FilePath: "b.go", Kind: core.KindGoMethod},
		{Name: "C", FilePath: "c.go", Kind: core.KindGoType},
		{Name: "D", FilePath: "d.go", Kind: core.KindGoMethod},
	}
	k := func(i int) string { return nodes[i].Key() }
	edges := []core.Edge{
		{FromSymbol: k(0), FromFile: "a.go", ToSymbol: k(1), Kind: core.EdgeDirect},
		{FromSymbol: k(0), FromFile: "a.go", ToSymbol: k(1), Kind: core.EdgeIfaceDispatch},
		{FromSymbol: k(0), FromFile: "a.go", ToSymbol: k(2), Kind: core.EdgeInstantiates},
		{FromSymbol: k(0), FromFile: "a.go", ToSymbol: k(3), Kind: core.EdgeIfaceDispatch},
		{FromSymbol: k(1), FromFile: "b.go", ToSymbol: k(3), Kind: core.EdgeFuncValue},
	}
	named, left := goNamedEdges(edges, nodes)
	pairs := fileEdgePairs(named, nodes)
	want := map[[2]string]bool{{"a.go", "b.go"}: true, {"b.go", "d.go"}: true}
	if !reflect.DeepEqual(pairs, want) {
		t.Errorf("named file edges = %v, want %v", pairs, want)
	}
	wantLeft := map[[2]string]bool{{"a.go", "c.go"}: true, {"a.go", "d.go"}: true}
	if !reflect.DeepEqual(left, wantLeft) {
		t.Errorf("left out = %v, want %v", left, wantLeft)
	}
}

// TestGoNativeCoverage (LB-F300): Go's typed_recall is graded on the SCIP
// import (design §1.2), and the native go/types pass's coverage of the same
// edges is disclosed beside it: of the unique (from, to) pairs the import
// stores between mapped definitions, how many the native pass also draws, by
// the kinds that draw them.
func TestGoNativeCoverage(t *testing.T) {
	imported := []core.Edge{
		{FromSymbol: "a", ToSymbol: "b", Kind: core.EdgeDirect},
		{FromSymbol: "a", ToSymbol: "b", Kind: core.EdgeDirect}, // one pair, two references
		{FromSymbol: "a", ToSymbol: "c", Kind: core.EdgeDirect},
		{FromSymbol: "a", ToSymbol: "d", Kind: core.EdgeDirect},
		{FromSymbol: "e", ToSymbol: "e", Kind: core.EdgeDirect}, // a self pair
	}
	native := []core.Edge{
		{FromSymbol: "a", ToSymbol: "b", Kind: core.EdgeDirect},
		{FromSymbol: "a", ToSymbol: "c", Kind: core.EdgeInstantiates},
		{FromSymbol: "a", ToSymbol: "z", Kind: core.EdgeDirect}, // native only
	}
	got := goNativeCoverage("s/x", imported, native)
	for _, want := range []string{"4 unique (from, to) pairs", "2 are edges of the native typed pass (.5000;", "direct 1, instantiates 1", "3 non-self pairs, 2 of them native (.6667)"} {
		if !strings.Contains(got, want) {
			t.Errorf("note lacks %q:\n%s", want, got)
		}
	}
}

// TestGoSCIPNotes (LB-F299): every class Go's L3 and L4 leave out or miss is
// counted in the reading's notes, with what def_mapping and typed_recall
// would read without the classes only an owner decision can move.
func TestGoSCIPNotes(t *testing.T) {
	counts := map[string]int{
		langbench.CntScipGoDocsOutside: 3, langbench.CntScipFileEdgesMultiDef: 7,
		langbench.CntScipDefs: 100, langbench.CntScipDefsMapped: 40,
		"scip_go_defs_unmapped_field": 40, "scip_go_defs_unmapped_package": 20,
		langbench.CntScipEdges: 1000, langbench.CntScipEdgesTyped: 400,
		"scip_go_unmapped_to_field": 450, "scip_go_unmapped_to_package": 100, "scip_go_unmapped_from_field": 50,
	}
	notes := strings.Join(goSCIPNotes("s/x", counts), "\n")
	for _, want := range []string{
		"3 documents of the scip-go index lie outside the subject",
		"7 file edges",
		"unmapped definitions by class: field 40, package 20",
		"def_mapping would read .6667 without the field class and 1.0000 without the field and package classes",
		"unmapped endpoints by class: from_field 50, to_field 450, to_package 100",
		"typed_recall would read .7273 without the references to a field and .8889 without those to a field or a package",
	} {
		if !strings.Contains(notes, want) {
			t.Errorf("notes lack %q:\n%s", want, notes)
		}
	}
}
