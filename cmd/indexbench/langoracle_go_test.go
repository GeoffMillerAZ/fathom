package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"

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
