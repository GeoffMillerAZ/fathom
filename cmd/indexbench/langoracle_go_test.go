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
