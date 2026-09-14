package scipexport

import (
	"context"
	"testing"

	"github.com/sourcegraph/scip/bindings/go/scip"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// fixtureNodes' FilePaths are deliberately root-level (not under a "pkg/"
// subdirectory matching the package name) so core.SymbolNode.Key()
// (QualifiedPackage: path.Dir(filePath) == "." leaves the bare package name
// unqualified) yields the simple "svc::pkg.A" keys this test's fixtureTypedEdges
// assume — a nested "pkg/a.go" would instead directory-qualify to
// "svc::pkg/pkg.A", which is Key()'s correct (if unintuitive) contract, not
// a bug, but would make these fixtures harder to read.
func fixtureNodes() []core.SymbolNode {
	return []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "A", Kind: core.KindGoFunc, FilePath: "a.go", Line: 10, EndLine: 12, Signature: "func A()", Calls: []string{"svc::pkg.B"}},
		{Service: "svc", Package: "pkg", Name: "B", Kind: core.KindGoFunc, FilePath: "a.go", Line: 20, Signature: "func B()"},
		{Service: "svc", Package: "pkg", Name: "MyType", Kind: core.KindGoType, FilePath: "types.go", Line: 1},
	}
}

func fixtureTypedEdges() []core.Edge {
	return []core.Edge{
		{FromSymbol: "svc::pkg.A", ToSymbol: "svc::pkg.B", Kind: core.EdgeDirect},
		{FromSymbol: "svc::pkg.A", ToSymbol: "svc::pkg.Nowhere", Kind: core.EdgeDirect}, // unresolved target: skipped
	}
}

func TestSymbolID_Deterministic(t *testing.T) {
	a := SymbolID("svc", "pkg", "A")
	b := SymbolID("svc", "pkg", "A")
	if a != b {
		t.Fatalf("SymbolID not deterministic: %q vs %q", a, b)
	}
	if a == SymbolID("svc", "pkg", "B") {
		t.Fatalf("SymbolID collided for different names")
	}
	if a == SymbolID("other-svc", "pkg", "A") {
		t.Fatalf("SymbolID collided across services")
	}
}

func TestSymbolID_RoundTripsThroughParser(t *testing.T) {
	id := SymbolID("svc", "pkg", "A")
	parsed, err := scip.ParseSymbol(id)
	if err != nil {
		t.Fatalf("scip.ParseSymbol(%q): %v", id, err)
	}
	if parsed.Package == nil || parsed.Package.Name != "svc" {
		t.Errorf("parsed package name = %+v, want svc", parsed.Package)
	}
	if len(parsed.Descriptors) != 2 || parsed.Descriptors[0].Name != "pkg" || parsed.Descriptors[1].Name != "A" {
		t.Errorf("parsed descriptors = %+v, want [pkg A]", parsed.Descriptors)
	}
}

func TestBuildFromNodes_DocumentsAndOccurrences(t *testing.T) {
	idx := BuildFromNodes("svc", fixtureNodes(), nil, nil)

	if idx.Metadata == nil || idx.Metadata.ToolInfo == nil || idx.Metadata.ToolInfo.Name != "fathom" {
		t.Fatalf("Metadata.ToolInfo.Name = %+v, want fathom", idx.Metadata)
	}
	if len(idx.Documents) != 2 {
		t.Fatalf("got %d documents, want 2 (a.go, types.go): %+v", len(idx.Documents), idx.Documents)
	}
	// Sorted by RelativePath.
	if idx.Documents[0].RelativePath != "a.go" || idx.Documents[1].RelativePath != "types.go" {
		t.Fatalf("documents not sorted by path: %q, %q", idx.Documents[0].RelativePath, idx.Documents[1].RelativePath)
	}

	docA := idx.Documents[0]
	if len(docA.Occurrences) != 2 {
		t.Fatalf("a.go: got %d occurrences, want 2 (A, B)", len(docA.Occurrences))
	}
	if len(docA.Symbols) != 2 {
		t.Fatalf("a.go: got %d symbols, want 2 (A, B)", len(docA.Symbols))
	}

	// A's occurrence spans Line..EndLine (10..12), 0-based.
	var aOcc *scip.Occurrence
	wantA := SymbolID("svc", "pkg", "A")
	for _, o := range docA.Occurrences {
		if o.Symbol == wantA {
			aOcc = o
		}
	}
	if aOcc == nil {
		t.Fatalf("no occurrence found for A's symbol %q", wantA)
	}
	if got, want := aOcc.Range, []int32{9, 0, 11, 1}; !int32SliceEqual(got, want) {
		t.Errorf("A occurrence range = %v, want %v", got, want)
	}
	if aOcc.SymbolRoles != int32(scip.SymbolRole_Definition) {
		t.Errorf("A occurrence SymbolRoles = %d, want Definition", aOcc.SymbolRoles)
	}
}

func TestBuildFromNodes_TypedEdgesBecomeRelationshipsWithTier(t *testing.T) {
	idx := BuildFromNodes("svc", fixtureNodes(), fixtureTypedEdges(), nil)

	var aInfo *scip.SymbolInformation
	wantA := SymbolID("svc", "pkg", "A")
	for _, d := range idx.Documents {
		for _, s := range d.Symbols {
			if s.Symbol == wantA {
				aInfo = s
			}
		}
	}
	if aInfo == nil {
		t.Fatalf("no SymbolInformation found for A")
	}

	// Exactly one relationship: A -> B. A -> Nowhere is skipped (unresolved
	// target, not among this service's own indexed nodes).
	if len(aInfo.Relationships) != 1 {
		t.Fatalf("A.Relationships = %+v, want exactly 1 (A->B; A->Nowhere must be skipped)", aInfo.Relationships)
	}
	wantB := SymbolID("svc", "pkg", "B")
	rel := aInfo.Relationships[0]
	if rel.Symbol != wantB || !rel.IsReference {
		t.Errorf("relationship = %+v, want {Symbol:%q IsReference:true}", rel, wantB)
	}

	// The resolution tier travels as a Documentation line keyed by the
	// target symbol string (package doc comment's documented contract).
	found := false
	wantLine := "fathom:resolution " + wantB + " = " + string(core.EdgeConfidenceOf(core.Edge{Kind: core.EdgeDirect}))
	for _, d := range aInfo.Documentation {
		if d == wantLine {
			found = true
		}
	}
	if !found {
		t.Errorf("A.Documentation = %+v, want a line %q", aInfo.Documentation, wantLine)
	}
}

func TestBuildFromNodes_HeuristicEdgeTier(t *testing.T) {
	// A syntax-only edge (Kind == "") must be labeled heuristic, never
	// resolved — core.EdgeConfidenceOf's own contract.
	edges := []core.Edge{{FromSymbol: "svc::pkg.A", ToSymbol: "svc::pkg.B", Kind: ""}}
	idx := BuildFromNodes("svc", fixtureNodes(), edges, nil)

	var aInfo *scip.SymbolInformation
	wantA := SymbolID("svc", "pkg", "A")
	for _, d := range idx.Documents {
		for _, s := range d.Symbols {
			if s.Symbol == wantA {
				aInfo = s
			}
		}
	}
	if aInfo == nil {
		t.Fatal("no SymbolInformation for A")
	}
	wantB := SymbolID("svc", "pkg", "B")
	wantLine := "fathom:resolution " + wantB + " = heuristic"
	if len(aInfo.Documentation) == 0 || aInfo.Documentation[len(aInfo.Documentation)-1] != wantLine {
		t.Errorf("A.Documentation = %+v, want to end with %q", aInfo.Documentation, wantLine)
	}
}

func TestBuildFromNodes_LanguageFromSources(t *testing.T) {
	srcs := []core.SourceFile{{Path: "a.go", Lang: "go"}}
	idx := BuildFromNodes("svc", fixtureNodes(), nil, srcs)
	for _, d := range idx.Documents {
		if d.RelativePath == "a.go" && d.Language != "go" {
			t.Errorf("a.go Language = %q, want go", d.Language)
		}
		if d.RelativePath == "types.go" && d.Language != "" {
			t.Errorf("types.go Language = %q, want empty (no Sources entry)", d.Language)
		}
	}
}

func TestBuildFromNodes_Deterministic(t *testing.T) {
	nodes := fixtureNodes()
	edges := fixtureTypedEdges()
	first := BuildFromNodes("svc", nodes, edges, nil)
	for i := 0; i < 10; i++ {
		again := BuildFromNodes("svc", nodes, edges, nil)
		if !indexesEqual(first, again) {
			t.Fatalf("BuildFromNodes not deterministic across repeated calls (iteration %d)", i)
		}
	}
}

func TestSymbolSet(t *testing.T) {
	idx := BuildFromNodes("svc", fixtureNodes(), nil, nil)
	set := SymbolSet(idx)
	if len(set) != 3 {
		t.Fatalf("SymbolSet = %v, want 3 entries (A, B, MyType)", set)
	}
	for _, name := range []string{"A", "B", "MyType"} {
		id := SymbolID("svc", "pkg", name)
		if !set[id] {
			t.Errorf("SymbolSet missing %q", id)
		}
	}
}

// ---------------------------------------------------------------------
// Build (backend-driven)
// ---------------------------------------------------------------------

type fakeBackend struct {
	nodes   []core.SymbolNode
	typed   []core.Edge
	sources []core.SourceFile
	err     error
}

func (f fakeBackend) ListAllSymbols(_ context.Context, _ string) ([]core.SymbolNode, error) {
	return f.nodes, f.err
}

func (f fakeBackend) TypedEdges(_ context.Context, _ string) ([]core.Edge, error) {
	return f.typed, nil
}

func (f fakeBackend) Sources(_ context.Context, _ string) ([]core.SourceFile, error) {
	return f.sources, nil
}

func TestBuild_UsesBackend(t *testing.T) {
	backend := fakeBackend{nodes: fixtureNodes(), typed: fixtureTypedEdges()}
	idx, err := Build(context.Background(), backend, "svc")
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(idx.Documents) != 2 {
		t.Fatalf("Build: got %d documents, want 2", len(idx.Documents))
	}
}

func TestBuild_PropagatesListError(t *testing.T) {
	backend := fakeBackend{err: errBoom}
	if _, err := Build(context.Background(), backend, "svc"); err == nil {
		t.Fatal("Build: want error when ListAllSymbols fails")
	}
}

var errBoom = &boomError{}

type boomError struct{}

func (*boomError) Error() string { return "boom" }

// ---------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------

func int32SliceEqual(a, b []int32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func indexesEqual(a, b *scip.Index) bool {
	if len(a.Documents) != len(b.Documents) {
		return false
	}
	for i := range a.Documents {
		da, db := a.Documents[i], b.Documents[i]
		if da.RelativePath != db.RelativePath || len(da.Occurrences) != len(db.Occurrences) || len(da.Symbols) != len(db.Symbols) {
			return false
		}
		for j := range da.Symbols {
			if da.Symbols[j].Symbol != db.Symbols[j].Symbol {
				return false
			}
		}
	}
	return true
}
